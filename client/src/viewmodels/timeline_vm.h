// TimelineViewModel: 时间线页面 ↔ CollectionService gRPC 桥

#pragma once

#include <QObject>
#include <QString>
#include <QTimer>
#include <memory>

#include "collection.qpb.h"
#include "collection_client.grpc.qpb.h"
#include "timelinemodels.h"
#include <QAbstractGrpcChannel>
#include <QGrpcChannelOptions>
#include <QGrpcHttp2Channel>

class TimelineViewModel : public QObject {
  Q_OBJECT
  Q_PROPERTY(QString date READ date WRITE setDate NOTIFY dateChanged)
  // segments/events 改为 QAbstractListModel*：QML 直接绑到 Repeater.model，
  // Model 内部做 diff 增量更新，避免每次刷新全量重建 delegate。
  Q_PROPERTY(SegmentListModel *segments READ segments CONSTANT)
  Q_PROPERTY(EventListModel *events READ events CONSTANT)
  Q_PROPERTY(QString catFilter READ catFilter WRITE setCatFilter NOTIFY catFilterChanged)
  Q_PROPERTY(QString evFilter READ evFilter WRITE setEvFilter NOTIFY evFilterChanged)
  Q_PROPERTY(bool loading READ loading NOTIFY loadingChanged)
  Q_PROPERTY(QString error READ error NOTIFY errorChanged)

 public:
  explicit TimelineViewModel(QObject *parent = nullptr);

  void setChannel(std::shared_ptr<QAbstractGrpcChannel> channel);

  QString date() const { return m_date; }
  void setDate(const QString &date);

  SegmentListModel *segments() { return &m_segModel; }
  EventListModel *events() { return &m_evModel; }

  QString catFilter() const { return m_segModel.categoryFilter(); }
  void setCatFilter(const QString &f);
  QString evFilter() const { return m_evModel.typeFilter(); }
  void setEvFilter(const QString &f);

  bool loading() const { return m_loading; }
  QString error() const { return m_error; }

  // 顶部统计上移到 C++：避免 QML 遍历整个 model。只统计 engaged/active 段。
  Q_INVOKABLE int activeDurationSec() const;
  Q_INVOKABLE int activeSegmentCount() const;

  Q_INVOKABLE void refresh();

 signals:
  void dateChanged();
  void catFilterChanged();
  void evFilterChanged();
  void loadingChanged();
  void errorChanged();

 private:
  void setLoading(bool v);
  void setError(const QString &e);

  shadowworker::CollectionService::Client m_client;
  std::shared_ptr<QAbstractGrpcChannel> m_channel;

  QString m_date;
  SegmentListModel m_segModel;
  EventListModel m_evModel;
  bool m_loading = false;
  QString m_error;

  // 周期刷新定时器：timeline 页面停留在当天时，自动拉取最新采集数据。
  QTimer m_pollTimer;
};
