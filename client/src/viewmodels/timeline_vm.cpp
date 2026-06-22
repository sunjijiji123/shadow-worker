// TimelineViewModel implementation.

#include "timeline_vm.h"

#include <QDate>
#include <QDateTime>
#include <QDebug>
#include <QDir>
#include <QFile>
#include <QGrpcCallReply>
#include <QGrpcStatus>
#include <QTextStream>
#include <algorithm>

using shadowworker::TimelineRequest;
using shadowworker::TimelineSnapshot;

// formatDuration 把秒数格式化为智能进位的时长文本。
//   < 60s       → "45s"
//   < 3600s     → "12 min"（秒部分舍去）
//   ≥ 3600s     → "1h 23min"
// 避免出现 "0 min"（秒级时长被整数除法截成 0）。
static QString formatDuration(int sec) {
  if (sec < 60) {
    return QString::number(sec < 0 ? 0 : sec) + "s";
  }
  if (sec < 3600) {
    return QString::number(sec / 60) + " min";
  }
  int h = sec / 3600;
  int m = (sec % 3600) / 60;
  // 整点小时不显示 "0min"，如 2h 而非 2h 0min
  return QString::number(h) + "h" +
         (m > 0 ? " " + QString::number(m) + "min" : QString());
}

TimelineViewModel::TimelineViewModel(QObject *parent) : QObject(parent) {
  m_date = QDate::currentDate().toString("yyyy-MM-dd");

  // 串接 source → proxy。filterRoleName 指定按哪个 role 等值过滤。
  // segments 按 category 过滤（catFilter ∈ all/coding/browser/...）。
  m_segProxy.setSourceModel(&m_segModel);
  m_segProxy.setFilterRoleName("category");
  // events 按 type 过滤（evFilter ∈ all/voice/screenshot/...）。
  m_evProxy.setSourceModel(&m_evModel);
  m_evProxy.setFilterRoleName("type");

  // 周期刷新：停留在 timeline 页时，每 30 秒自动拉一次最新采集数据，
  // 让用户新产生的活动段/事件能及时出现在列表里（无需手动点 Today）。
  m_pollTimer.setInterval(30000);
  connect(&m_pollTimer, &QTimer::timeout, this, &TimelineViewModel::refresh);
  m_pollTimer.start();
}

void TimelineViewModel::setChannel(
    std::shared_ptr<QAbstractGrpcChannel> channel) {
  m_channel = std::move(channel);
  m_client.attachChannel(m_channel);
}

void TimelineViewModel::setDate(const QString &date) {
  if (m_date == date) return;
  m_date = date;
  emit dateChanged();
  refresh();
}

void TimelineViewModel::setCatFilter(const QString &f) {
  // 转发给 proxy：proxy 内部 invalidateFilter 增量增/删可见行，
  // 不再触发 beginResetModel 全量重建。
  // 注意：filterValue 相同时 proxy 短路返回，但本层仍需发 catFilterChanged
  // 让 QML Chip 的 checked 绑定保持一致（虽然值没变，绑定本身会幂等）。
  QString old = m_segProxy.filterValue();
  m_segProxy.setFilterValue(f);
  if (old != f) emit catFilterChanged();
}

void TimelineViewModel::setEvFilter(const QString &f) {
  QString old = m_evProxy.filterValue();
  m_evProxy.setFilterValue(f);
  if (old != f) emit evFilterChanged();
}

void TimelineViewModel::refresh() {
  if (!m_channel) {
    setError(QStringLiteral("gRPC channel not initialized"));
    return;
  }
  setLoading(true);
  setError({});

  TimelineRequest req;
  req.setDate(m_date);
  std::unique_ptr<QGrpcCallReply> reply = m_client.QueryTimeline(req);

  auto *replyPtr = reply.get();
  reply.release();

  QObject::connect(
      replyPtr, &QGrpcCallReply::finished, this,
      [this, replyPtr](const QGrpcStatus &status) {
        replyPtr->deleteLater();
        setLoading(false);

        if (!status.isOk()) {
          setError(QStringLiteral("gRPC error: ") + status.message());
          return;
        }

        std::optional<TimelineSnapshot> opt =
            replyPtr->read<TimelineSnapshot>();
        if (!opt.has_value()) {
          setError(QStringLiteral("Failed to parse response"));
          return;
        }

        const TimelineSnapshot &snapshot = *opt;

        // 时间轴可视窗口：直接透传后端算好的整点边界。
        setWindowStartTs(snapshot.windowStartTs());
        setWindowEndTs(snapshot.windowEndTs());

        // 转 SegItem 列表。后端返回已按 start_ts 升序，这里按 endTs 倒序
        // （最新在前，worklog 列表顶部是最近的记录）。
        QList<SegItem> segs;
        const auto &srcSegs = snapshot.segments();
        segs.reserve(srcSegs.size());
        for (const auto &seg : srcSegs) {
          SegItem item;
          item.startTs = seg.startTs();
          item.endTs = seg.endTs();
          int dur = (int)(item.endTs - item.startTs);
          item.durationSec = dur;
          item.durationMin = dur / 60;
          item.durationText = formatDuration(dur);
          item.appName = seg.appName();
          item.category = seg.category();
          item.windowTitle = seg.windowTitle();
          item.state = seg.state();
          item.summary = seg.summary();
          item.appIcon = seg.appName();
          item.startTime =
              QDateTime::fromSecsSinceEpoch(item.startTs).toString("HH:mm");
          item.endTime =
              QDateTime::fromSecsSinceEpoch(item.endTs).toString("HH:mm");
          segs.append(item);
        }
        std::sort(segs.begin(), segs.end(),
                  [](const SegItem &a, const SegItem &b) {
                    return a.endTs > b.endTs;  // 倒序：endTs 大的在前
                  });
        m_segModel.replaceAll(segs);
        // 数据已更新，通知顶部统计 Q_PROPERTY 绑定重算。
        // activeDurationSec/activeSegmentCount 读 m_segModel，replaceAll 后已是新值。
        emit activeDurationSecChanged();
        emit activeSegmentCountChanged();

        // 转 EvItem 列表，按 ts 倒序。
        QList<EvItem> evs;
        const auto &srcEvs = snapshot.events();
        evs.reserve(srcEvs.size());
        for (const auto &ev : srcEvs) {
          EvItem item;
          item.ts = ev.ts();
          item.time = QDateTime::fromSecsSinceEpoch(item.ts).toString("HH:mm");
          item.type = ev.type();
          item.text = ev.text();
          item.appName = ev.appName();
          evs.append(item);
        }
        std::sort(evs.begin(), evs.end(),
                  [](const EvItem &a, const EvItem &b) {
                    return a.ts > b.ts;
                  });
        m_evModel.replaceAll(evs);
      });
}

void TimelineViewModel::setLoading(bool v) {
  if (m_loading == v) return;
  m_loading = v;
  emit loadingChanged();
}

void TimelineViewModel::setError(const QString &e) {
  if (m_error == e) return;
  m_error = e;
  emit errorChanged();
}

void TimelineViewModel::setWindowStartTs(qint64 v) {
  if (m_windowStartTs == v) return;
  m_windowStartTs = v;
  emit windowStartTsChanged();
}

void TimelineViewModel::setWindowEndTs(qint64 v) {
  if (m_windowEndTs == v) return;
  m_windowEndTs = v;
  emit windowEndTsChanged();
}
