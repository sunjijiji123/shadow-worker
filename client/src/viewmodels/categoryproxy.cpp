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

  int role = resolveRole();
  if (role < 0) return true;  // role 未配置，放行

  QModelIndex idx = sourceModel()->index(sourceRow, 0, sourceParent);
  if (!idx.isValid()) return true;
  return sourceModel()->data(idx, role).toString() == m_value;
}
