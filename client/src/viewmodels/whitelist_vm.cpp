#include "whitelist_vm.h"

#include <QGrpcHttp2Channel>

WhitelistViewModel::WhitelistViewModel(QObject *parent)
    : QAbstractListModel(parent) {
  auto channel =
      std::make_shared<QGrpcHttp2Channel>(QUrl("http://127.0.0.1:50051"));
  m_client = std::make_unique<shadowworker::WhitelistService::Client>(this);
  m_client->attachChannel(channel);
}

int WhitelistViewModel::rowCount(const QModelIndex &parent) const {
  Q_UNUSED(parent)
  return m_apps.size();
}

QVariant WhitelistViewModel::data(const QModelIndex &index, int role) const {
  if (!index.isValid() || index.row() < 0 || index.row() >= m_apps.size())
    return QVariant();

  const auto &app = m_apps.at(index.row());
  switch (role) {
  case PathRole:
    return app.path;
  case NameRole:
    return app.name;
  case CategoryRole:
    return app.category;
  case TodayMinutesRole:
    return app.todayMinutes;
  default:
    return QVariant();
  }
}

QHash<int, QByteArray> WhitelistViewModel::roleNames() const {
  QHash<int, QByteArray> roles;
  roles[PathRole] = "path";
  roles[NameRole] = "name";
  roles[CategoryRole] = "category";
  roles[TodayMinutesRole] = "todayMinutes";
  return roles;
}

void WhitelistViewModel::refresh() {
  setLoading(true);
  setError(QString());

  shadowworker::ListAppsRequest req;
  auto reply = m_client->List(req);
  auto *replyPtr = reply.get();

  connect(replyPtr, &QGrpcOperation::finished, this,
          [this, reply = std::move(reply)](const QGrpcStatus &status) mutable {
            setLoading(false);
            if (!status.isOk()) {
              setError(status.message());
              return;
            }

            auto result = reply->read<shadowworker::AppList>();
            if (!result) {
              setError("解析白名单失败");
              return;
            }

            beginResetModel();
            m_apps.clear();
            for (const auto &app : result->apps()) {
              WhitelistAppItem item;
              item.path = app.path();
              item.name = app.name();
              item.category = app.category();
              item.todayMinutes = app.todayMinutes();
              m_apps.append(item);
            }
            endResetModel();
          });
}

void WhitelistViewModel::addApp(const QString &path, const QString &name,
                                const QString &category) {
  if (path.isEmpty() || name.isEmpty())
    return;

  shadowworker::AddAppRequest req;
  req.setPath(path);
  req.setName(name);
  req.setCategory(category.isEmpty() ? QStringLiteral("other") : category);
  auto reply = m_client->Add(req);
  auto *replyPtr = reply.get();

  connect(replyPtr, &QGrpcOperation::finished, this,
          [this, reply = std::move(reply)](const QGrpcStatus &status) mutable {
            if (!status.isOk()) {
              setError(status.message());
              return;
            }
            refresh();
          });
}

void WhitelistViewModel::removeApp(const QString &path) {
  if (path.isEmpty())
    return;

  shadowworker::RemoveAppRequest req;
  req.setPath(path);
  auto reply = m_client->Remove(req);
  auto *replyPtr = reply.get();

  connect(replyPtr, &QGrpcOperation::finished, this,
          [this, reply = std::move(reply)](const QGrpcStatus &status) mutable {
            if (!status.isOk()) {
              setError(status.message());
              return;
            }
            refresh();
          });
}

void WhitelistViewModel::updateCategory(const QString &path,
                                        const QString &category) {
  if (path.isEmpty() || category.isEmpty())
    return;

  shadowworker::UpdateCategoryRequest req;
  req.setPath(path);
  req.setCategory(category);
  auto reply = m_client->UpdateCategory(req);
  auto *replyPtr = reply.get();

  connect(replyPtr, &QGrpcOperation::finished, this,
          [this, reply = std::move(reply)](const QGrpcStatus &status) mutable {
            if (!status.isOk()) {
              setError(status.message());
              return;
            }
            refresh();
          });
}

void WhitelistViewModel::setLoading(bool loading) {
  if (m_loading == loading)
    return;
  m_loading = loading;
  emit loadingChanged();
}

void WhitelistViewModel::setError(const QString &error) {
  if (m_error == error)
    return;
  m_error = error;
  emit errorChanged();
}
