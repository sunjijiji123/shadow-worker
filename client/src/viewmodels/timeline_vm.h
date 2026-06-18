// TimelineViewModel: 时间线页面 ↔ CollectionService gRPC 桥

#pragma once

#include <QObject>
#include <QString>
#include <QVariantList>
#include <memory>

#include "collection.qpb.h"
#include "collection_client.grpc.qpb.h"
#include <QAbstractGrpcChannel>
#include <QGrpcChannelOptions>
#include <QGrpcHttp2Channel>

class TimelineViewModel : public QObject {
  Q_OBJECT
  Q_PROPERTY(QString date READ date WRITE setDate NOTIFY dateChanged)
  Q_PROPERTY(QVariantList segments READ segments NOTIFY dataChanged)
  Q_PROPERTY(QVariantList events READ events NOTIFY dataChanged)
  Q_PROPERTY(bool loading READ loading NOTIFY loadingChanged)
  Q_PROPERTY(QString error READ error NOTIFY errorChanged)

public:
  explicit TimelineViewModel(QObject *parent = nullptr);

  void setChannel(std::shared_ptr<QAbstractGrpcChannel> channel);

  QString date() const { return m_date; }
  void setDate(const QString &date);

  QVariantList segments() const { return m_segments; }
  QVariantList events() const { return m_events; }
  bool loading() const { return m_loading; }
  QString error() const { return m_error; }

  Q_INVOKABLE void refresh();

signals:
  void dateChanged();
  void dataChanged();
  void loadingChanged();
  void errorChanged();

private:
  void setLoading(bool v);
  void setError(const QString &e);

  shadowworker::CollectionService::Client m_client;
  std::shared_ptr<QAbstractGrpcChannel> m_channel;

  QString m_date;
  QVariantList m_segments;
  QVariantList m_events;
  bool m_loading = false;
  QString m_error;
};
