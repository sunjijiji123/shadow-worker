// timelinemodels.cpp - SegmentListModel / EventListModel 实现。
//
// 重构后：本层只负责持有全量数据 + diff 增量刷新 + 全量统计。
// 过滤已移交给 RoleFilterProxyModel（categoryproxy.h），故 rowCount/data 恒 O(1)。

#include "timelinemodels.h"

// ============================================================
// SegmentListModel
// ============================================================

SegmentListModel::SegmentListModel(QObject *parent)
    : QAbstractListModel(parent) {}

int SegmentListModel::rowCount(const QModelIndex &parent) const {
  if (parent.isValid()) return 0;
  return m_items.size();
}

QVariant SegmentListModel::data(const QModelIndex &index, int role) const {
  if (!index.isValid()) return {};
  int row = index.row();
  if (row < 0 || row >= m_items.size()) return {};
  const SegItem &s = m_items[row];

  switch (role) {
    case StartTsRole: return s.startTs;
    case EndTsRole: return s.endTs;
    case DurationSecRole: return s.durationSec;
    case DurationMinRole: return s.durationMin;
    case DurationTextRole: return s.durationText;
    case AppNameRole: return s.appName;
    case CategoryRole: return s.category;
    case WindowTitleRole: return s.windowTitle;
    case StateRole: return s.state;
    case SummaryRole: return s.summary;
    case AppIconRole: return s.appIcon;
    case StartTimeRole: return s.startTime;
    case EndTimeRole: return s.endTime;
  }
  return {};
}

QHash<int, QByteArray> SegmentListModel::roleNames() const {
  return {
      {StartTsRole, "startTs"},       {EndTsRole, "endTs"},
      {DurationSecRole, "durationSec"}, {DurationMinRole, "durationMin"},
      {DurationTextRole, "durationText"}, {AppNameRole, "appName"},
      {CategoryRole, "category"},     {WindowTitleRole, "windowTitle"},
      {StateRole, "state"},           {SummaryRole, "summary"},
      {AppIconRole, "appIcon"},       {StartTimeRole, "startTime"},
      {EndTimeRole, "endTime"},
  };
}

// get 暴露给 QML JS：按索引取一行，返回 role 名 → 值 的 map。
// TimelineTrack.segmentAtX 用此做命中测试（hover 时根据 x 反查段）。
// 越界返回空 map（调用方判空）。key 名与 roleNames() 一致，QML 端用 s.startTs 访问。
QVariantMap SegmentListModel::get(int i) const {
  QVariantMap m;
  if (i < 0 || i >= m_items.size()) return m;
  const SegItem &s = m_items[i];
  m["startTs"] = s.startTs;
  m["endTs"] = s.endTs;
  m["durationSec"] = s.durationSec;
  m["durationMin"] = s.durationMin;
  m["durationText"] = s.durationText;
  m["appName"] = s.appName;
  m["category"] = s.category;
  m["windowTitle"] = s.windowTitle;
  m["state"] = s.state;
  m["summary"] = s.summary;
  m["appIcon"] = s.appIcon;
  m["startTime"] = s.startTime;
  m["endTime"] = s.endTime;
  return m;
}

void SegmentListModel::replaceAll(const QList<SegItem> &items) {
  // diff 增量更新：用 (startTs, appName) 复合 key 匹配旧行。
  // 顺序变化（如换日期导致完全不同）会退化为大量 remove+insert，
  // 但单次刷新（同一天数据微变）时绝大多数行命中旧 key，仅更新变化的字段。
  //
  // 简化策略：若新旧数量一致且复合 key 序列相同，只逐行 dataChanged(roles)；
  // 否则 beginResetModel/endResetModel（换日期等场景，低频，可接受）。
  // 这覆盖了轮询刷新（最常见、最需要增量）的场景，又不会在结构剧变时出错。
  //
  // 注意：过滤已交给 proxy，本层 reset 不会引发 QML 全量重建 delegate ——
  // proxy 在 source reset 时会重算过滤并增/删可见行，配合 ListView 虚拟化，
  // 即使 reset 也只重建当前可视区的少数 delegate。

  bool sameStructure =
      items.size() == m_items.size() &&
      std::equal(
          items.begin(), items.end(), m_items.begin(),
          [](const SegItem &a, const SegItem &b) {
            return a.startTs == b.startTs && a.appName == b.appName;
          });

  if (sameStructure) {
    // 增量：逐行比对字段，只通知变化的 role。
    for (int i = 0; i < items.size(); ++i) {
      const SegItem &oldI = m_items[i];
      const SegItem &newI = items[i];
      QVector<int> changed;
      if (oldI.endTs != newI.endTs) {
        changed << EndTsRole << DurationSecRole << DurationMinRole
                << DurationTextRole << EndTimeRole;
      }
      if (oldI.state != newI.state) changed << StateRole;
      if (oldI.summary != newI.summary) changed << SummaryRole;
      if (oldI.windowTitle != newI.windowTitle) changed << WindowTitleRole;
      if (oldI.category != newI.category) changed << CategoryRole;
      if (!changed.isEmpty()) {
        m_items[i] = newI;
        QModelIndex idx = index(i);
        emit dataChanged(idx, idx, changed);
      }
    }
    return;
  }

  // 结构变化：整体 reset（换日期/数据剧变，低频）。
  int oldCount = m_items.size();
  beginResetModel();
  m_items = items;
  endResetModel();
  // count 仅在结构变化路径可能变（增量路径 sameStructure 保证 size 不变）。
  // 用 Q_PROPERTY(count) 的 NOTIFY 驱动 QML 绑定（segmentAtX 遍历用 count）。
  if (m_items.size() != oldCount) emit countChanged();
}

int SegmentListModel::activeDurationSec() const {
  int total = 0;
  for (const auto &s : m_items) {
    if (s.state == "engaged" || s.state == "active") {
      total += s.durationSec;
    }
  }
  return total;
}

int SegmentListModel::activeSegmentCount() const {
  int n = 0;
  for (const auto &s : m_items) {
    if (s.state == "engaged" || s.state == "active") ++n;
  }
  return n;
}

// ============================================================
// EventListModel
// ============================================================

EventListModel::EventListModel(QObject *parent)
    : QAbstractListModel(parent) {}

int EventListModel::rowCount(const QModelIndex &parent) const {
  if (parent.isValid()) return 0;
  return m_items.size();
}

QVariant EventListModel::data(const QModelIndex &index, int role) const {
  if (!index.isValid()) return {};
  int row = index.row();
  if (row < 0 || row >= m_items.size()) return {};
  const EvItem &e = m_items[row];

  switch (role) {
    case TsRole: return e.ts;
    case TimeRole: return e.time;
    case TypeRole: return e.type;
    case TextRole: return e.text;
    case AppNameRole: return e.appName;
  }
  return {};
}

QHash<int, QByteArray> EventListModel::roleNames() const {
  return {
      {TsRole, "ts"},
      {TimeRole, "time"},
      {TypeRole, "type"},
      // 注意：role 名用 "evText" 而非 "text"。"text" 是 QML 内置属性名，
      // delegate 用 required property string text 时会与 QML 的 text 属性
      // 机制冲突，导致 role 值无法绑定到 delegate（实测 text role 永远为空）。
      // 改名后 QML 用 required property string evText 即可正确绑定。
      {TextRole, "evText"},
      {AppNameRole, "appName"},
  };
}

void EventListModel::replaceAll(const QList<EvItem> &items) {
  // 同 SegmentListModel：结构一致走增量 dataChanged，否则 reset。
  bool sameStructure =
      items.size() == m_items.size() &&
      std::equal(items.begin(), items.end(), m_items.begin(),
                 [](const EvItem &a, const EvItem &b) {
                   return a.ts == b.ts && a.type == b.type;
                 });

  if (sameStructure) {
    for (int i = 0; i < items.size(); ++i) {
      const EvItem &oldI = m_items[i];
      const EvItem &newI = items[i];
      QVector<int> changed;
      if (oldI.text != newI.text) changed << TextRole;
      if (oldI.appName != newI.appName) changed << AppNameRole;
      if (!changed.isEmpty()) {
        m_items[i] = newI;
        QModelIndex idx = index(i);
        emit dataChanged(idx, idx, changed);
      }
    }
    return;
  }

  beginResetModel();
  m_items = items;
  endResetModel();
}
