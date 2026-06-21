// TimelineViewModel: 时间线页面 ↔ CollectionService gRPC 桥
//
// 数据流（重构后）：
//   m_segModel (source, 全量) ──┬── m_segProxy (按 category 过滤) ── QML ListView
//                              └── allSegments() 直供 TimelineTrack 画轨道（要全量）
//   m_evModel (source) ── m_evProxy (按 type 过滤) ── QML events ListView
//
// 过滤改由 RoleFilterProxyModel 处理（不再 beginResetModel 全量重建），
// 配合 ListView 虚拟化，切 catFilter 时只增删差异行，且只重建可视区 delegate。

#pragma once

#include <QObject>
#include <QString>
#include <QTimer>
#include <memory>

#include "collection.qpb.h"
#include "collection_client.grpc.qpb.h"
#include "timelinemodels.h"
#include "categoryproxy.h"
#include <QAbstractGrpcChannel>
#include <QGrpcChannelOptions>
#include <QGrpcHttp2Channel>

class TimelineViewModel : public QObject {
  Q_OBJECT
  Q_PROPERTY(QString date READ date WRITE setDate NOTIFY dateChanged)
  // segments 返回 proxy（已按 catFilter 过滤），QML ListView 绑这个。
  // 类型用 QAbstractItemModel*：proxy 是 QSortFilterProxyModel（继承
  // QAbstractProxyModel→QAbstractItemModel），不是 QAbstractListModel；
  // 用共同基类 QAbstractItemModel 才能同时容纳 proxy 和 source。
  // QML 不关心具体 C++ 类型，只要是个 model 就能绑 ListView.model / Repeater.model。
  Q_PROPERTY(QAbstractItemModel *segments READ segments CONSTANT)
  // allSegments 返回 source model（全量），供 TimelineTrack 画轨道使用——
  // 轨道必须显示全天的所有段，不能跟着列表过滤变。
  Q_PROPERTY(SegmentListModel *allSegments READ allSegments CONSTANT)
  Q_PROPERTY(QAbstractItemModel *events READ events CONSTANT)
  Q_PROPERTY(QString catFilter READ catFilter WRITE setCatFilter NOTIFY catFilterChanged)
  Q_PROPERTY(QString evFilter READ evFilter WRITE setEvFilter NOTIFY evFilterChanged)
  Q_PROPERTY(bool loading READ loading NOTIFY loadingChanged)
  Q_PROPERTY(QString error READ error NOTIFY errorChanged)

 public:
  explicit TimelineViewModel(QObject *parent = nullptr);

  void setChannel(std::shared_ptr<QAbstractGrpcChannel> channel);

  QString date() const { return m_date; }
  void setDate(const QString &date);

  // segments 返回 proxy（过滤后），供 ListView。QML 看到的就是过滤后的行。
  QAbstractItemModel *segments() { return &m_segProxy; }
  // allSegments 返回 source（全量），供 TimelineTrack。
  SegmentListModel *allSegments() { return &m_segModel; }
  // events 返回 proxy（过滤后）。
  QAbstractItemModel *events() { return &m_evProxy; }

  QString catFilter() const { return m_segProxy.filterValue(); }
  void setCatFilter(const QString &f);
  QString evFilter() const { return m_evProxy.filterValue(); }
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
  SegmentListModel m_segModel;          // 全量 source
  RoleFilterProxyModel m_segProxy;      // 按 category 过滤，QML ListView 绑这个
  EventListModel m_evModel;             // 全量 source
  RoleFilterProxyModel m_evProxy;       // 按 type 过滤
  bool m_loading = false;
  QString m_error;

  // 周期刷新定时器：timeline 页面停留在当天时，自动拉取最新采集数据。
  QTimer m_pollTimer;
};
