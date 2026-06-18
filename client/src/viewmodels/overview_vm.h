// OverviewViewModel: 概览页的 QML ↔ C++ 桥

#pragma once

#include <QList>
#include <QObject>
#include <QString>
#include <QVariantList>
#include <memory>

#include "collection.qpb.h"
#include "collection_client.grpc.qpb.h"
#include "overview.qpb.h"
#include "overview_client.grpc.qpb.h"
#include <QAbstractGrpcChannel>
#include <QGrpcChannelOptions>
#include <QGrpcHttp2Channel>
#include <QGrpcOperation>

// 生成的类在 shadowworker 命名空间下(proto package)
using shadowworker::GetOverviewRequest;
using shadowworker::OverviewData;
using shadowworker::OverviewUpdate;
using shadowworker::OverviewService::Client;

class OverviewViewModel : public QObject {
  Q_OBJECT
  Q_PROPERTY(int todayMinutes READ todayMinutes NOTIFY dataChanged)
  Q_PROPERTY(int activeSegments READ activeSegments NOTIFY dataChanged)
  Q_PROPERTY(QString collectionStatus READ collectionStatus NOTIFY dataChanged)
  Q_PROPERTY(QString asrStatus READ asrStatus NOTIFY dataChanged)
  Q_PROPERTY(QString vlmStatus READ vlmStatus NOTIFY dataChanged)
  Q_PROPERTY(QString mcpStatus READ mcpStatus NOTIFY dataChanged)
  Q_PROPERTY(QString activeApp READ activeApp NOTIFY dataChanged)
  Q_PROPERTY(QVariantList apps READ apps NOTIFY dataChanged)
  Q_PROPERTY(bool loading READ loading NOTIFY loadingChanged)
  Q_PROPERTY(QString error READ error NOTIFY errorChanged)

public:
  explicit OverviewViewModel(QObject *parent = nullptr);
  ~OverviewViewModel();

  void setChannel(std::shared_ptr<QAbstractGrpcChannel> channel);

  Q_INVOKABLE void refresh();
  Q_INVOKABLE void pauseCollection();
  Q_INVOKABLE void resumeCollection();

  int todayMinutes() const { return m_todayMinutes; }
  int activeSegments() const { return m_activeSegments; }
  QString collectionStatus() const { return m_collectionStatus; }
  QString asrStatus() const { return m_asrStatus; }
  QString vlmStatus() const { return m_vlmStatus; }
  QString mcpStatus() const { return m_mcpStatus; }
  QString activeApp() const { return m_activeApp; }
  QVariantList apps() const { return m_appsVariant; }
  bool loading() const { return m_loading; }
  QString error() const { return m_error; }

signals:
  void dataChanged();
  void loadingChanged();
  void errorChanged();

private:
  void setLoading(bool v);
  void setError(const QString &e);
  void applyOverviewData(const OverviewData &data);
  void applyOverviewUpdate(const OverviewUpdate &update);
  void startWatch();
  void stopWatch();

  Client m_client;
  shadowworker::CollectionService::Client m_collectionClient;
  std::shared_ptr<QAbstractGrpcChannel> m_channel;
  QGrpcOperation *m_watchOp = nullptr;

  int m_todayMinutes = 0;
  int m_activeSegments = 0;
  QString m_collectionStatus;
  QString m_asrStatus;
  QString m_vlmStatus;
  QString m_mcpStatus;
  QString m_activeApp;
  QVariantList m_appsVariant;
  bool m_loading = false;
  QString m_error;
};
