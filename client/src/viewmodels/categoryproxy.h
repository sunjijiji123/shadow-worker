// categoryproxy.h - 通用 role 等值过滤代理 model。
//
// 解决 Timeline 切换 catFilter/evFilter 时全量重建 delegate 的卡顿：
//  - QSortFilterProxyModel 的 filterAcceptsRow 是 Qt 增量实现，
//    filter 变化时只增删差异行，不像 beginResetModel 那样全量销毁重建。
//  - source model 保持全量数据，过滤完全在 proxy 层完成。
//  - 同时配合 ListView 的虚拟化：QML 只绑到 proxy，可视区外不实例化 delegate。
//
// 用法：
//   RoleFilterProxyModel { filterRoleName: "category"; filterValue: "coding" }
//   filterValue 为空或 "all" 时不做过滤（全量透传）。

#pragma once

#include <QSortFilterProxyModel>
#include <QString>
#include <qqmlintegration.h>

class RoleFilterProxyModel : public QSortFilterProxyModel {
  Q_OBJECT
  QML_ELEMENT
  // filterRoleName 是用来过滤的 role 名称（如 "category" / "type"）。
  // 对应 sourceModel 的 roleNames()。设置后会重新解析 role id。
  Q_PROPERTY(QString filterRoleName READ filterRoleName WRITE setFilterRoleName
                 NOTIFY filterRoleNameChanged)
  // filterValue 是期望的等值（如 "coding"）。空串或 "all" 表示不过滤。
  Q_PROPERTY(
      QString filterValue READ filterValue WRITE setFilterValue NOTIFY filterValueChanged)

 public:
  explicit RoleFilterProxyModel(QObject *parent = nullptr);

  QString filterRoleName() const { return m_roleName; }
  void setFilterRoleName(const QString &name);

  QString filterValue() const { return m_value; }
  void setFilterValue(const QString &value);

 signals:
  void filterRoleNameChanged();
  void filterValueChanged();

 protected:
  bool filterAcceptsRow(int sourceRow,
                        const QModelIndex &sourceParent) const override;

 private:
  // 把 role 名称解析成 role id，缓存避免每次过滤都查表。
  int resolveRole() const;

  QString m_roleName;
  QString m_value;
  // mutable：resolveRole 在 const filterAcceptsRow 里做懒解析缓存。
  mutable int m_roleId = -1;
  mutable bool m_roleResolved = false;
};
