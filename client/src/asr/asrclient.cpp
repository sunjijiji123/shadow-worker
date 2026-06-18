#include "asrclient.h"

#include <QDebug>
#include <QGrpcBidiStream>
#include <QGrpcHttp2Channel>

AsrClient::AsrClient(QObject *parent) : QObject(parent) {
  auto channel =
      std::make_shared<QGrpcHttp2Channel>(QUrl("http://127.0.0.1:50051"));
  m_client = std::make_unique<shadowworker::AsrService::Client>(this);
  m_client->attachChannel(channel);
}

void AsrClient::start() {
  if (m_recognizing)
    finish();

  setPartialText(QString());
  setFinalText(QString());
  setError(QString());
  setRecognizing(true);

  shadowworker::AudioChunk init;
  auto op = m_client->StreamRecognize(init);
  m_operation = op.get();
  auto *bidi = qobject_cast<QGrpcBidiStream *>(m_operation);
  if (!bidi) {
    setError("无法创建 ASR 双向流");
    setRecognizing(false);
    return;
  }

  connect(bidi, &QGrpcBidiStream::messageReceived, this, [this, bidi]() {
    auto result = bidi->read<shadowworker::AsrResult>();
    if (!result)
      return;
    if (!result->partialText().isEmpty()) {
      setPartialText(result->partialText());
    }
    if (!result->finalText().isEmpty()) {
      setFinalText(result->finalText());
      emit resultReady(finalText());
    }
  });

  connect(bidi, &QGrpcOperation::finished, this,
          [this, bidi](const QGrpcStatus &status) {
            Q_UNUSED(bidi)
            setRecognizing(false);
            if (!status.isOk()) {
              setError(status.message());
            }
            m_operation = nullptr;
          });

  op.release();
}

void AsrClient::feed(const QByteArray &pcm) {
  if (!m_operation || pcm.isEmpty())
    return;

  shadowworker::AudioChunk chunk;
  chunk.setPcm(pcm);

  auto *bidi = qobject_cast<QGrpcBidiStream *>(m_operation);
  if (bidi)
    bidi->writeMessage(chunk);
}

void AsrClient::finish() {
  if (!m_operation)
    return;

  auto *bidi = qobject_cast<QGrpcBidiStream *>(m_operation);
  if (bidi)
    bidi->writesDone();
}

void AsrClient::setRecognizing(bool v) {
  if (m_recognizing == v)
    return;
  m_recognizing = v;
  emit recognizingChanged();
}

void AsrClient::setPartialText(const QString &v) {
  if (m_partialText == v)
    return;
  m_partialText = v;
  emit partialTextChanged();
}

void AsrClient::setFinalText(const QString &v) {
  if (m_finalText == v)
    return;
  m_finalText = v;
  emit finalTextChanged();
}

void AsrClient::setError(const QString &v) {
  if (m_error == v)
    return;
  m_error = v;
  emit errorChanged();
}
