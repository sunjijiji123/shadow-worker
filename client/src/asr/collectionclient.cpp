#include "collectionclient.h"

#include <QGrpcCallReply>
#include <QGrpcStatus>

CollectionClient::CollectionClient(QObject *parent) : QObject(parent) {}

void CollectionClient::setChannel(std::shared_ptr<QAbstractGrpcChannel> channel) {
  m_channel = std::move(channel);
  m_client.attachChannel(m_channel);
}

void CollectionClient::triggerVLM() {
  if (!m_channel) {
    emit vlmSummaryReady(QString(),
                         QStringLiteral("gRPC channel not initialized"));
    return;
  }
  // TriggerVLMRequest 是空 message。
  shadowworker::TriggerVLMRequest req;
  auto reply = m_client.TriggerVLM(req);
  auto *replyPtr = reply.get();
  reply.release();  // 把所有权交给 lambda，回调里 deleteLater
  QObject::connect(replyPtr, &QGrpcCallReply::finished, this,
                   [this, replyPtr](const QGrpcStatus &status) {
                     replyPtr->deleteLater();
                     if (!status.isOk()) {
                       emit vlmSummaryReady(QString(), status.message());
                       return;
                     }
                     auto opt = replyPtr->read<shadowworker::VLMSummary>();
                     if (!opt.has_value()) {
                       emit vlmSummaryReady(QString(),
                                            QStringLiteral("parse failed"));
                       return;
                     }
                     emit vlmSummaryReady(opt->summary(), QString());
                   });
}

void CollectionClient::analyzeImage(const QString &path) {
  if (!m_channel) {
    emit imageAnalyzed(path, QString(),
                       QStringLiteral("gRPC channel not initialized"));
    return;
  }
  shadowworker::AnalyzeImageRequest req;
  req.setPath(path);
  auto reply = m_client.AnalyzeImage(req);
  auto *replyPtr = reply.get();
  reply.release();
  QObject::connect(replyPtr, &QGrpcCallReply::finished, this,
                   [this, replyPtr, path](const QGrpcStatus &status) {
                     replyPtr->deleteLater();
                     if (!status.isOk()) {
                       emit imageAnalyzed(path, QString(), status.message());
                       return;
                     }
                     auto opt = replyPtr->read<shadowworker::VLMSummary>();
                     if (!opt.has_value()) {
                       emit imageAnalyzed(path, QString(),
                                          QStringLiteral("parse failed"));
                       return;
                     }
                     emit imageAnalyzed(path, opt->summary(), QString());
                   });
}
