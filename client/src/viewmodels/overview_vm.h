// OverviewViewModel: bridge between QML overview page and OverviewService gRPC.

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

// Generated classes live in the shadowworker namespace (proto package).
using shadowworker::GetOverviewRequest;
using shadowworker::OverviewData;
using shadowworker::OverviewUpdate;
using shadowworker::OverviewService::Client;

class OverviewViewModel : public QObject {
  Q_OBJECT
  Q_PROPERTY(QString range READ range WRITE setRange NOTIFY rangeChanged)
  Q_PROPERTY(int todayMinutes READ todayMinutes NOTIFY dataChanged)
  Q_PROPERTY(int activeSegments READ activeSegments NOTIFY dataChanged)
  Q_PROPERTY(int interruptCount READ interruptCount NOTIFY dataChanged)
  Q_PROPERTY(int minutesDelta READ minutesDelta NOTIFY dataChanged)
  Q_PROPERTY(int interruptDelta READ interruptDelta NOTIFY dataChanged)
  Q_PROPERTY(int appCount READ appCount NOTIFY dataChanged)
  Q_PROPERTY(QString collectionStatus READ collectionStatus NOTIFY dataChanged)
  Q_PROPERTY(QString asrStatus READ asrStatus NOTIFY dataChanged)
  Q_PROPERTY(QString vlmStatus READ vlmStatus NOTIFY dataChanged)
  Q_PROPERTY(QString mcpStatus READ mcpStatus NOTIFY dataChanged)
  Q_PROPERTY(QString activeApp READ activeApp NOTIFY dataChanged)
  Q_PROPERTY(QString activeCategory READ activeCategory NOTIFY dataChanged)
  Q_PROPERTY(QVariantList apps READ apps NOTIFY dataChanged)
  Q_PROPERTY(QVariantList heatmap READ heatmap NOTIFY heatmapChanged)
  Q_PROPERTY(QVariantList categoryRank READ categoryRank NOTIFY categoryRankChanged)
  Q_PROPERTY(bool loading READ loading NOTIFY loadingChanged)
  Q_PROPERTY(QString error READ error NOTIFY errorChanged)

public:
  explicit OverviewViewModel(QObject *parent = nullptr);
  ~OverviewViewModel();

  void setChannel(std::shared_ptr<QAbstractGrpcChannel> channel);

  QString range() const { return m_range; }
  void setRange(const QString &r);

  Q_INVOKABLE void refresh();
  Q_INVOKABLE void pauseCollection();
  Q_INVOKABLE void resumeCollection();
  // Fetch heatmap (last N months) and category rank for current range.
  Q_INVOKABLE void refreshHeatmap(int monthsBack = 3);
  Q_INVOKABLE void refreshCategoryRank();

  int todayMinutes() const { return m_todayMinutes; }
  int activeSegments() const { return m_activeSegments; }
  int interruptCount() const { return m_interruptCount; }
  int minutesDelta() const { return m_minutesDelta; }
  int interruptDelta() const { return m_interruptDelta; }
  int appCount() const { return m_appCount; }
  QString collectionStatus() const { return m_collectionStatus; }
  QString asrStatus() const { return m_asrStatus; }
  QString vlmStatus() const { return m_vlmStatus; }
  QString mcpStatus() const { return m_mcpStatus; }
  QString activeApp() const { return m_activeApp; }
  QString activeCategory() const { return m_activeCategory; }
  QVariantList apps() const { return m_appsVariant; }
  QVariantList heatmap() const { return m_heatmap; }
  QVariantList categoryRank() const { return m_categoryRank; }
  bool loading() const { return m_loading; }
  QString error() const { return m_error; }

signals:
  void rangeChanged();
  void dataChanged();
  void heatmapChanged();
  void categoryRankChanged();
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

  QString m_range = "day";   // day | week | month
  int m_todayMinutes = 0;
  int m_activeSegments = 0;
  int m_interruptCount = 0;
  int m_minutesDelta = 0;
  int m_interruptDelta = 0;
  int m_appCount = 0;
  QString m_collectionStatus;
  QString m_asrStatus;
  QString m_vlmStatus;
  QString m_mcpStatus;
  QString m_activeApp;
  QString m_activeCategory;
  QVariantList m_appsVariant;
  QVariantList m_heatmap;
  QVariantList m_categoryRank;
  bool m_loading = false;
  QString m_error;
};
