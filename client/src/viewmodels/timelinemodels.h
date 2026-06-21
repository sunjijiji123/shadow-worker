// timelinemodels.h - Timeline 段/事件的 source model。
//
// 职责（重构后）：只持有全量数据 + 增量刷新（diff）+ 全量统计。
// 过滤职责已移交给 RoleFilterProxyModel（categoryproxy.h），本类不再做过滤，
// rowCount/data 恒为 O(1)。
//
// 数据流：
//   TimelineViewModel.m_segModel (source, 全量)
//     └── RoleFilterProxyModel (filterValue=catFilter)  ← QML ListView 绑这个
//     └── TimelineTrack 直接绑 source（要全量画轨道）
//
// replaceAll 保留 diff 增量逻辑（轮询刷新的最常见场景，绝大多数行命中旧 key）。

#pragma once

#include <QAbstractListModel>
#include <QString>
#include <qqmlintegration.h>

// SegItem 是一条时间线段（聚合后的活动记录）。
// 字段与 QML delegate 的 role 一一对应。
struct SegItem {
  qint64 startTs = 0;
  qint64 endTs = 0;
  int durationSec = 0;
  int durationMin = 0;
  QString durationText;
  QString appName;
  QString category;
  QString windowTitle;
  QString state;
  QString summary;
  QString appIcon;
  QString startTime;  // "HH:mm"
  QString endTime;    // "HH:mm"
};

// EvItem 是一条时间线事件（语音/截图/VLM 摘要等）。
struct EvItem {
  qint64 ts = 0;
  QString time;  // "HH:mm"
  QString type;
  QString text;
  QString appName;
};

// SegmentListModel 是 worklog 列表与 timeline track 的全量数据源。
// - replaceAll 做 diff 增量更新（用 startTs+appName 复合 key 匹配旧行）。
// - 过滤不在本层做（交给 RoleFilterProxyModel），rowCount/data 恒 O(1)。
// - 数据按 endTs 倒序存储（最新在前），QML 无需 reverse。
class SegmentListModel : public QAbstractListModel {
  Q_OBJECT
  QML_ELEMENT

public:
  enum Role {
    StartTsRole = Qt::UserRole + 1,
    EndTsRole,
    DurationSecRole,
    DurationMinRole,
    DurationTextRole,
    AppNameRole,
    CategoryRole,
    WindowTitleRole,
    StateRole,
    SummaryRole,
    AppIconRole,
    StartTimeRole,
    EndTimeRole,
  };
  Q_ENUM(Role)

  explicit SegmentListModel(QObject *parent = nullptr);

  int rowCount(const QModelIndex &parent = QModelIndex()) const override;
  QVariant data(const QModelIndex &index,
                int role = Qt::DisplayRole) const override;
  QHash<int, QByteArray> roleNames() const override;

  // replaceAll 用新数据增量替换：diff (startTs+appName) 复合 key，
  // 匹配旧行；变化行发 dataChanged(roles)，消失行 removeRows，新行 insertRows。
  // items 必须已按 endTs 倒序排好（由 ViewModel 保证）。
  void replaceAll(const QList<SegItem> &items);

  // 统计全量数据中 engaged/active 段的总秒数/段数。
  // 供顶部"Work Xh · N segments"使用，避免 QML 遍历整个列表。
  // 注意：本类不再做过滤，统计的就是全量（与 proxy 的 catFilter 无关）。
  int activeDurationSec() const;
  int activeSegmentCount() const;

private:
  QList<SegItem> m_items;   // 全量数据（倒序）
};

// EventListModel 是 events 列表的全量数据源。结构与 SegmentListModel 一致。
class EventListModel : public QAbstractListModel {
  Q_OBJECT
  QML_ELEMENT

public:
  enum Role {
    TsRole = Qt::UserRole + 1,
    TimeRole,
    TypeRole,
    TextRole,
    AppNameRole,
  };
  Q_ENUM(Role)

  explicit EventListModel(QObject *parent = nullptr);

  int rowCount(const QModelIndex &parent = QModelIndex()) const override;
  QVariant data(const QModelIndex &index,
                int role = Qt::DisplayRole) const override;
  QHash<int, QByteArray> roleNames() const override;

  // replaceAll 增量替换。items 须已按 ts 倒序排好。
  void replaceAll(const QList<EvItem> &items);

private:
  QList<EvItem> m_items;
};
