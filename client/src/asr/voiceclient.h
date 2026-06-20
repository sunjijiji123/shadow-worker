#pragma once

#include <QObject>
#include <QVariantList>
#include <qqmlintegration.h>

#include "voice.qpb.h"
#include "voice_client.grpc.qpb.h"
#include <QAbstractGrpcChannel>

// VoiceClient drives the backend recording flow: StartRecording, StopRecording
// (returns the ASR text), and StreamLevels (live 16-band spectrum frames).
//
// Exposed to QML as the `voiceClient` context property. The Qt side calls
// start()/stop() and subscribes to levelsReady (16 floats) + resultReady(text).
class VoiceClient : public QObject {
  Q_OBJECT
  QML_ELEMENT
  Q_PROPERTY(bool recording READ recording NOTIFY recordingChanged)
  Q_PROPERTY(QString error READ error NOTIFY errorChanged)

public:
  explicit VoiceClient(QObject *parent = nullptr);

  void setChannel(std::shared_ptr<QAbstractGrpcChannel> channel);

  bool recording() const { return m_recording; }
  QString error() const { return m_error; }

  // begin capturing on the backend (deviceId empty = default device)
  Q_INVOKABLE void start(const QString &deviceId = QString());
  // stop capturing + run ASR; result arrives via resultReady
  Q_INVOKABLE void stop();
  // start the live spectrum stream (call after start())
  Q_INVOKABLE void streamLevels();
  // test ASR config connectivity (cloud endpoint or local model file).
  // fields: map<string,string> of provider config fields.
  // resultReady is repurposed: on success, text=message; on failure, error=message.
  Q_INVOKABLE void testConnection(const QString &mode, const QVariantMap &fields);
  // polish recognized text via the backend LLM. result arrives via polishReady.
  Q_INVOKABLE void polish(const QString &text);

signals:
  void recordingChanged();
  void errorChanged();
  // 16 normalized band energies [0..1] + overall rms 0..100
  void levelsReady(const QVariantList &bands, int rms);
  // final recognized text after stop()
  void resultReady(const QString &text, const QString &error);
  // test connection result (separate from resultReady to avoid triggering the
  // recording bubble)
  void connectionTested(const QString &message, const QString &error);
  // polish result: originalText 是请求时的原文（供失败时回退显示），
  // polishedText 是润色后文字（失败时为空），error 非空表示失败。
  void polishReady(const QString &originalText, const QString &polishedText, const QString &error);

private:
  void setRecording(bool v);
  void setError(const QString &e);

  shadowworker::VoiceService::Client m_client;
  std::shared_ptr<QAbstractGrpcChannel> m_channel;
  QGrpcOperation *m_levelsStream = nullptr;
  bool m_recording = false;
  QString m_error;
};
