// timelinemodels.cpp - SegmentListModel / EventListModel 实现。

#include "timelinemodels.h"

#include <QHash>

// ============================================================
// SegmentListModel
// ============================================================

SegmentListModel::SegmentListModel(QObject *parent)
    : QAbstractListModel(parent) {}

int SegmentListModel::rowCount(const QModelIndex &parent) const {
  if (parent.isValid()) return 0;
  if (m_filter == "all" || m_filter.isEmpty()) return m_items.size();
  // 过滤态：统计匹配项。filter 是低频操作，rowCount 线性遍历可接受。
  int n = 0;
  for (const auto &s : m_items) {
    if (s.category == m_filter) ++n;
  }
  return n;
}

QVariant SegmentListModel::data(const QModelIndex &index, int role) const {
  if (!index.isValid()) return {};
  int row = index.row();

  // 过滤态下需把可见行号映射回 m_items 的真实下标。
  const SegItem *p = nullptr;
  if (m_filter == "all" || m_filter.isEmpty()) {
    if (row < 0 || row >= m_items.size()) return {};
    p = &m_items[row];
  } else {
    int visible = 0;
    for (const auto &s : m_items) {
      if (s.category != m_filter) continue;
      if (visible == row) { p = &s; break; }
      ++visible;
    }
    if (!p) return {};
  }
  const SegItem &s = *p;

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

void SegmentListModel::replaceAll(const QList<SegItem> &items) {
  // diff 增量更新：用 (startTs, appName) 复合 key 匹配旧行。
  // 顺序变化（如换日期导致完全不同）会退化为大量 remove+insert，
  // 但单次刷新（同一天数据微变）时绝大多数行命中旧 key，仅更新变化的字段。
  //
  // 简化策略：若新旧数量一致且复合 key 序列相同，只逐行 dataChanged(roles)；
  // 否则 beginResetModel/endResetModel（换日期等场景，低频，可接受）。
  // 这覆盖了轮询刷新（最常见、最需要增量）的场景，又不会在结构剧变时出错。

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
  beginResetModel();
  m_items = items;
  endResetModel();
}

void SegmentListModel::setCategoryFilter(const QString &filter) {
  if (m_filter == filter) return;
  beginResetModel();
  m_filter = filter;
  endResetModel();
  emit categoryFilterChanged();
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
  if (m_filter == "all" || m_filter.isEmpty()) return m_items.size();
  int n = 0;
  for (const auto &e : m_items) {
    if (e.type == m_filter) ++n;
  }
  return n;
}

QVariant EventListModel::data(const QModelIndex &index, int role) const {
  if (!index.isValid()) return {};
  int row = index.row();

  const EvItem *p = nullptr;
  if (m_filter == "all" || m_filter.isEmpty()) {
    if (row < 0 || row >= m_items.size()) return {};
    p = &m_items[row];
  } else {
    int visible = 0;
    for (const auto &e : m_items) {
      if (e.type != m_filter) continue;
      if (visible == row) { p = &e; break; }
      ++visible;
    }
    if (!p) return {};
  }
  const EvItem &e = *p;

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
      {TextRole, "text"},
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

void EventListModel::setTypeFilter(const QString &filter) {
  if (m_filter == filter) return;
  beginResetModel();
  m_filter = filter;
  endResetModel();
  emit typeFilterChanged();
}
