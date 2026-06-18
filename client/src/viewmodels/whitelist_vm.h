#pragma once

#include <QAbstractListModel>
#include <QObject>
#include <qqmlintegration.h>

#include "whitelist.qpb.h"
#include "whitelist_client.grpc.qpb.h"
#include <QAbstractItemModel>
#include <QAbstractListModel>

// WhitelistAppItem 是 QML 展示的一条白名单应用。
struct WhitelistAppItem {
  QString path;
  QString name;
  QString category;
  int todayMinutes = 0;
};

// WhitelistViewModel 是 Qt 端白名单管理 VM。
// 通过 gRPC 连接 Go 后台的 WhitelistService。
class WhitelistViewModel : public QAbstractListModel {
  Q_OBJECT
  QML_ELEMENT
  Q_PROPERTY(bool loading READ loading NOTIFY loadingChanged)
  Q_PROPERTY(QString error READ error NOTIFY errorChanged)

public:
  enum Role {
    PathRole = Qt::UserRole + 1,
    NameRole,
    CategoryRole,
    TodayMinutesRole,
  };
  Q_ENUM(Role)

  explicit WhitelistViewModel(QObject *parent = nullptr);

  int rowCount(const QModelIndex &parent = QModelIndex()) const override;
  QVariant data(const QModelIndex &index,
                int role = Qt::DisplayRole) const override;
  QHash<int, QByteArray> roleNames() const override;

  bool loading() const { return m_loading; }
  QString error() const { return m_error; }

  Q_INVOKABLE void refresh();
  Q_INVOKABLE void addApp(const QString &path, const QString &name,
                          const QString &category);
  Q_INVOKABLE void removeApp(const QString &path);
  Q_INVOKABLE void updateCategory(const QString &path, const QString &category);

signals:
  void loadingChanged();
  void errorChanged();
  void appPicked(const QString &path, const QString &name);

private:
  void setLoading(bool loading);
  void setError(const QString &error);
  std::unique_ptr<shadowworker::WhitelistService::Client> m_client;
  QList<WhitelistAppItem> m_apps;
  bool m_loading = false;
  QString m_error;
};
