// OverviewViewModel 实现

#include "overview_vm.h"

#include <QDebug>
#include <QGrpcCallReply>
#include <QGrpcServerStream>
#include <QGrpcStatus>

#include "collection.qpb.h"
#include "collection_client.grpc.qpb.h"

using shadowworker::PauseRequest;
using shadowworker::Result;
using shadowworker::ResumeRequest;

OverviewViewModel::OverviewViewModel(QObject *parent) : QObject(parent) {}

OverviewViewModel::~OverviewViewModel() { stopWatch(); }

void OverviewViewModel::setChannel(
    std::shared_ptr<QAbstractGrpcChannel> channel) {
  m_channel = std::move(channel);
  m_client.attachChannel(m_channel);
  m_collectionClient.attachChannel(m_channel);
  startWatch();
  refresh();
}

void OverviewViewModel::refresh() {
  if (!m_channel) {
    setError(QStringLiteral("gRPC channel 未初始化"));
    return;
  }
  setLoading(true);
  setError({});

  GetOverviewRequest req;
  std::unique_ptr<QGrpcCallReply> reply = m_client.GetOverview(req);
  auto *replyPtr = reply.get();
  reply.release();

  QObject::connect(
      replyPtr, &QGrpcCallReply::finished, this,
      [this, replyPtr](const QGrpcStatus &status) {
        replyPtr->deleteLater();
        setLoading(false);

        if (!status.isOk()) {
          setError(QStringLiteral("gRPC 错误: ") + status.message());
          return;
        }

        std::optional<OverviewData> opt = replyPtr->read<OverviewData>();
        if (!opt.has_value()) {
          setError(QStringLiteral("解析响应失败"));
          return;
        }
        applyOverviewData(*opt);
      });
}

void OverviewViewModel::pauseCollection() {
  if (!m_channel)
    return;
  PauseRequest req;
  auto reply = m_collectionClient.Pause(req);
  auto *replyPtr = reply.get();
  reply.release();
  QObject::connect(
      replyPtr, &QGrpcCallReply::finished, this,
      [replyPtr](const QGrpcStatus &) { replyPtr->deleteLater(); });
}

void OverviewViewModel::resumeCollection() {
  if (!m_channel)
    return;
  ResumeRequest req;
  auto reply = m_collectionClient.Resume(req);
  auto *replyPtr = reply.get();
  reply.release();
  QObject::connect(
      replyPtr, &QGrpcCallReply::finished, this,
      [replyPtr](const QGrpcStatus &) { replyPtr->deleteLater(); });
}

void OverviewViewModel::applyOverviewData(const OverviewData &data) {
  m_todayMinutes = (int)data.todayMinutes();
  m_activeSegments = (int)data.activeSegments();
  m_collectionStatus = data.collectionStatus();
  m_asrStatus = data.asrStatus();
  m_vlmStatus = data.vlmStatus();
  m_mcpStatus = data.mcpStatus();

  m_appsVariant.clear();
  const auto &apps = data.apps();
  for (const auto &a : apps) {
    QVariantMap m;
    m["name"] = a.name();
    m["category"] = a.category();
    m["todayMinutes"] = (int)a.todayMinutes();
    m_appsVariant.append(m);
  }
  emit dataChanged();
}

void OverviewViewModel::applyOverviewUpdate(const OverviewUpdate &update) {
  m_todayMinutes = (int)update.todayMinutes();
  m_collectionStatus = update.collectionStatus();
  m_activeApp = update.activeApp();
  emit dataChanged();
}

void OverviewViewModel::startWatch() {
  if (!m_channel || m_watchOp)
    return;

  shadowworker::WatchOverviewRequest req;
  std::unique_ptr<QGrpcOperation> op = m_client.WatchOverview(req);
  m_watchOp = op.get();

  auto *stream = qobject_cast<QGrpcServerStream *>(m_watchOp);
  if (!stream) {
    qWarning() << "WatchOverview 无法创建 server stream";
    op.release();
    m_watchOp->deleteLater();
    m_watchOp = nullptr;
    return;
  }

  connect(stream, &QGrpcServerStream::messageReceived, this, [this, stream]() {
    auto result = stream->read<OverviewUpdate>();
    if (!result)
      return;
    applyOverviewUpdate(*result);
  });

  connect(stream, &QGrpcOperation::finished, this,
          [this](const QGrpcStatus &status) {
            m_watchOp = nullptr;
            if (!status.isOk()) {
              qWarning() << "WatchOverview 流结束:" << status.message();
            }
          });

  op.release();
}

void OverviewViewModel::stopWatch() {
  if (m_watchOp) {
    m_watchOp->deleteLater();
    m_watchOp = nullptr;
  }
}

void OverviewViewModel::setLoading(bool v) {
  if (m_loading == v)
    return;
  m_loading = v;
  emit loadingChanged();
}

void OverviewViewModel::setError(const QString &e) {
  if (m_error == e)
    return;
  m_error = e;
  emit errorChanged();
}
