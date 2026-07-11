// categoryproxy.cpp - RoleFilterProxyModel 实现。

#include "categoryproxy.h"

RoleFilterProxyModel::RoleFilterProxyModel(QObject *parent)
    : QSortFilterProxyModel(parent) {}

void RoleFilterProxyModel::setFilterRoleName(const QString &name) {
  if (m_roleName == name) return;
  m_roleName = name;
  // role 名变化，强制下次过滤重新解析 id。
  m_roleResolved = false;
  emit filterRoleNameChanged();
  invalidateFilter();
}

void RoleFilterProxyModel::setFilterValue(const QString &value) {
  if (m_value == value) return;
  m_value = value;
  emit filterValueChanged();
  invalidateFilter();
}

void RoleFilterProxyModel::setSpecialFilter(const QString &v) {
  if (m_special == v) return;
  m_special = v;
  emit specialFilterChanged();
  invalidateFilter();
}

int RoleFilterProxyModel::resolveRole() const {
  if (m_roleResolved) return m_roleId;
  m_roleId = -1;
  if (!m_roleName.isEmpty() && sourceModel()) {
    const auto names = sourceModel()->roleNames();
    for (auto it = names.begin(); it != names.end(); ++it) {
      if (it.value() == m_roleName) {
        m_roleId = it.key();
        break;
      }
    }
  }
  m_roleResolved = true;
  return m_roleId;
}

bool RoleFilterProxyModel::filterAcceptsRow(
    int sourceRow, const QModelIndex &sourceParent) const {
  // 空或 "all" = 不过滤，全量透传。
  if (m_value.isEmpty() || m_value == "all") return true;

  QModelIndex idx = sourceModel()->index(sourceRow, 0, sourceParent);
  if (!idx.isValid()) return true;

  // 特殊过滤：如 "failed" = failMeta 非空的行（分析失败的段）。
  // 不走等值匹配，改查 failMeta role 是否非空。
  if (!m_special.isEmpty() && m_value == m_special) {
    // 解析 failMeta role id（懒缓存）。
    static const char *failMetaName = "failMeta";
    int failRole = -1;
    const auto names = sourceModel()->roleNames();
    for (auto it = names.begin(); it != names.end(); ++it) {
      if (it.value() == failMetaName) {
        failRole = it.key();
        break;
      }
    }
    if (failRole < 0) return true;
    return !sourceModel()->data(idx, failRole).toString().isEmpty();
  }

  int role = resolveRole();
  if (role < 0) return true;  // role 未配置，放行
  return sourceModel()->data(idx, role).toString() == m_value;
}
