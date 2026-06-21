// timelinemodels.h - Timeline 段/事件的 QAbstractListModel 实现。
//
// 用 Model 替代原先的 QVariantList，解决 30s 轮询刷新时全量重建 delegate
// 导致的界面卡顿：replaceAll 做 diff 增量更新，只对变化的行发 dataChanged，
// 未变化的 delegate 不重建。过滤与倒序也在 C++ 侧完成，QML 直接绑 model。

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

// SegmentListModel 是 worklog 列表与 timeline track 的数据源。
// - replaceAll 做 diff 增量更新（用 startTs+appName 复合 key 匹配旧行）。
// - setCategoryFilter 内置过滤（"all" 或具体类别），过滤变化时 reset。
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

  // setCategoryFilter 设置类别过滤（"all"=不过滤，或具体类别如 "coding"）。
  // 过滤变化时整体 reset（filter 是低频操作，reset 可接受）。
  void setCategoryFilter(const QString &filter);
  QString categoryFilter() const { return m_filter; }

  // 统计全量数据（忽略 catFilter）中 engaged/active 段的总秒数/段数。
  // 供顶部"Work Xh · N segments"使用，避免 QML 遍历整个列表。
  int activeDurationSec() const;
  int activeSegmentCount() const;

signals:
  void categoryFilterChanged();

private:
  QList<SegItem> m_items;   // 全量数据（倒序）
  QString m_filter;         // "all" 或具体类别
};

// EventListModel 是 events 列表的数据源。结构与 SegmentListModel 一致。
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

  // setTypeFilter 设置事件类型过滤（"all" 或具体类型如 "voice"）。
  void setTypeFilter(const QString &filter);
  QString typeFilter() const { return m_filter; }

signals:
  void typeFilterChanged();

private:
  QList<EvItem> m_items;
  QString m_filter;
};
