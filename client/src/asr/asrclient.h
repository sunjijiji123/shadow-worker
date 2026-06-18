#pragma once

#include <QByteArray>
#include <QGrpcOperation>
#include <QGrpcStatus>
#include <QObject>
#include <QUrl>
#include <qqmlintegration.h>

#include "asr.qpb.h"
#include "asr_client.grpc.qpb.h"

// AsrClient 调用 Go 后台的 AsrService 双向流识别。
// 将录音 PCM 分块发送,接收识别结果。
class AsrClient : public QObject {
  Q_OBJECT
  QML_ELEMENT
  Q_PROPERTY(bool recognizing READ recognizing NOTIFY recognizingChanged)
  Q_PROPERTY(QString partialText READ partialText NOTIFY partialTextChanged)
  Q_PROPERTY(QString finalText READ finalText NOTIFY finalTextChanged)
  Q_PROPERTY(QString error READ error NOTIFY errorChanged)

public:
  explicit AsrClient(QObject *parent = nullptr);

  bool recognizing() const { return m_recognizing; }
  QString partialText() const { return m_partialText; }
  QString finalText() const { return m_finalText; }
  QString error() const { return m_error; }

  // 开始一次识别会话
  Q_INVOKABLE void start();
  // 发送一块 PCM 数据
  Q_INVOKABLE void feed(const QByteArray &pcm);
  // 结束识别会话并返回最终结果
  Q_INVOKABLE void finish();

signals:
  void recognizingChanged();
  void partialTextChanged();
  void finalTextChanged();
  void errorChanged();
  void resultReady(const QString &text);

private:
  void setRecognizing(bool v);
  void setPartialText(const QString &v);
  void setFinalText(const QString &v);
  void setError(const QString &v);

  std::unique_ptr<shadowworker::AsrService::Client> m_client;
  QGrpcOperation *m_operation = nullptr;
  bool m_recognizing = false;
  QString m_partialText;
  QString m_finalText;
  QString m_error;
};
