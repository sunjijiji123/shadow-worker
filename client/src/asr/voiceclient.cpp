#include "voiceclient.h"

#include <QDebug>
#include <QGrpcCallReply>
#include <QGrpcServerStream>
#include <QGrpcStatus>

VoiceClient::VoiceClient(QObject *parent) : QObject(parent) {}

void VoiceClient::setChannel(std::shared_ptr<QAbstractGrpcChannel> channel) {
  m_channel = std::move(channel);
  m_client.attachChannel(m_channel);
}

void VoiceClient::start(const QString &deviceId) {
  if (!m_channel) {
    setError(QStringLiteral("gRPC channel not initialized"));
    return;
  }
  shadowworker::StartRequest req;
  req.setDeviceId(deviceId);
  auto reply = m_client.StartRecording(req);
  auto *replyPtr = reply.get();
  reply.release();
  QObject::connect(replyPtr, &QGrpcCallReply::finished, this,
                   [this, replyPtr](const QGrpcStatus &status) {
                     replyPtr->deleteLater();
                     if (!status.isOk()) {
                       setError(status.message());
                       setRecording(false);
                       return;
                     }
                     auto opt = replyPtr->read<shadowworker::StartResponse>();
                     if (!opt.has_value() || !opt->ok()) {
                       setError(opt.has_value() ? opt->error()
                                                : QStringLiteral("start failed"));
                       setRecording(false);
                       return;
                     }
                     setRecording(true);
                     setError(QString());
                     // begin the spectrum stream immediately
                     streamLevels();
                   });
}

void VoiceClient::stop() {
  if (!m_channel) return;
  shadowworker::StopRequest req;
  auto reply = m_client.StopRecording(req);
  auto *replyPtr = reply.get();
  reply.release();
  QObject::connect(replyPtr, &QGrpcCallReply::finished, this,
                   [this, replyPtr](const QGrpcStatus &status) {
                     replyPtr->deleteLater();
                     setRecording(false);
                     // stop the levels stream
                     if (m_levelsStream) {
                       m_levelsStream->cancel();
                       m_levelsStream = nullptr;
                     }
                     if (!status.isOk()) {
                       emit resultReady(QString(), status.message());
                       return;
                     }
                     auto opt = replyPtr->read<shadowworker::VoiceResult>();
                     if (!opt.has_value()) {
                       emit resultReady(QString(), QStringLiteral("parse failed"));
                       return;
                     }
                     emit resultReady(opt->text(), opt->error());
                   });
}

void VoiceClient::streamLevels() {
  if (!m_channel) return;
  shadowworker::LevelsRequest req;
  auto stream = m_client.StreamLevels(req);
  // keep a typed QGrpcServerStream* so we can connect its signal directly
  auto *serverStream = qobject_cast<QGrpcServerStream *>(stream.get());
  if (!serverStream) return;
  stream.release();
  m_levelsStream = serverStream;
  QObject::connect(serverStream, &QGrpcServerStream::messageReceived, this,
                   [this, serverStream]() {
                     auto opt = serverStream->read<shadowworker::AudioLevel>();
                     if (!opt.has_value()) return;
                     const auto &bands = opt->bands();
                     QVariantList list;
                     list.reserve(bands.size());
                     for (const auto &b : bands) list.append(b);
                     emit levelsReady(list, opt->rms());
                   });
  QObject::connect(serverStream, &QGrpcOperation::finished, this,
                   [this](const QGrpcStatus &) { m_levelsStream = nullptr; });
}

void VoiceClient::setRecording(bool v) {
  if (m_recording == v) return;
  m_recording = v;
  emit recordingChanged();
}

void VoiceClient::setError(const QString &e) {
  if (m_error == e) return;
  m_error = e;
  emit errorChanged();
}
