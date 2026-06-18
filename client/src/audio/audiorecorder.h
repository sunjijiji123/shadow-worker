#pragma once

#include <QAudioFormat>
#include <QAudioSource>
#include <QByteArray>
#include <QIODevice>
#include <QObject>
#include <qqmlintegration.h>

// AudioRecorder 使用 Qt Multimedia 录制麦克风 PCM。
// 输出格式: 16kHz / mono / int16 little-endian,与 ASR 输入对齐。
class AudioRecorder : public QObject {
  Q_OBJECT
  QML_ELEMENT
  Q_PROPERTY(bool recording READ recording NOTIFY recordingChanged)

public:
  explicit AudioRecorder(QObject *parent = nullptr);
  ~AudioRecorder();

  bool recording() const { return m_recording; }

  Q_INVOKABLE void startRecording();
  Q_INVOKABLE void stopRecording();

  // 取出已录制的 PCM 并清空内部缓冲区
  QByteArray takeAudioData();

signals:
  void recordingChanged();
  void errorOccurred(const QString &error);

private:
  void setRecording(bool recording);

  QAudioFormat m_format;
  QAudioSource *m_source = nullptr;
  QIODevice *m_io = nullptr;
  QByteArray m_buffer;
  bool m_recording = false;
};
