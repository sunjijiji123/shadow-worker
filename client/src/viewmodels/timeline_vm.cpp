// TimelineViewModel implementation.

#include "timeline_vm.h"

#include <QDate>
#include <QDateTime>
#include <QGrpcCallReply>
#include <QGrpcStatus>

using shadowworker::TimelineRequest;
using shadowworker::TimelineSnapshot;

TimelineViewModel::TimelineViewModel(QObject *parent) : QObject(parent) {
  m_date = QDate::currentDate().toString("yyyy-MM-dd");
}

void TimelineViewModel::setChannel(
    std::shared_ptr<QAbstractGrpcChannel> channel) {
  m_channel = std::move(channel);
  m_client.attachChannel(m_channel);
}

void TimelineViewModel::setDate(const QString &date) {
  if (m_date == date)
    return;
  m_date = date;
  emit dateChanged();
  refresh();
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
        m_segments.clear();
        const auto &segs = snapshot.segments();
        for (const auto &seg : segs) {
          QVariantMap m;
          qint64 startTs = seg.startTs();
          qint64 endTs = seg.endTs();
          m["startTs"] = startTs;
          m["endTs"] = endTs;
          m["durationSec"] = (int)(endTs - startTs);
          m["durationMin"] = (int)((endTs - startTs) / 60);
          m["appName"] = seg.appName();
          m["category"] = seg.category();
          m["windowTitle"] = seg.windowTitle();
          m["state"] = seg.state();
          m["startTime"] =
              QDateTime::fromSecsSinceEpoch(startTs).toString("HH:mm");
          m["endTime"] =
              QDateTime::fromSecsSinceEpoch(endTs).toString("HH:mm");
          m_segments.append(m);
        }

        m_events.clear();
        const auto &events = snapshot.events();
        for (const auto &ev : events) {
          QVariantMap m;
          qint64 ts = ev.ts();
          m["ts"] = ts;
          m["time"] = QDateTime::fromSecsSinceEpoch(ts).toString("HH:mm");
          m["type"] = ev.type();
          m["text"] = ev.text();
          m["appName"] = ev.appName();
          m_events.append(m);
        }

        emit dataChanged();
      });
}

void TimelineViewModel::setLoading(bool v) {
  if (m_loading == v)
    return;
  m_loading = v;
  emit loadingChanged();
}

void TimelineViewModel::setError(const QString &e) {
  if (m_error == e)
    return;
  m_error = e;
  emit errorChanged();
}
