#pragma once

#include <QAudioFormat>
#include <QAudioSource>
#include <QObject>
#include <QTimer>
#include <QVariantList>
#include <qqmlintegration.h>

class QIODevice;

// AudioRecorder: captures microphone audio via Qt Multimedia (16kHz mono Int16,
// the ASR input format). Exposes device enumeration, selectable input device,
// and a live volume level (0..100, RMS of recent PCM) to QML.
//
// Two capture modes:
//   - recording (m_testMode=false): real capture for ASR / hotkey flow
//   - testing   (m_testMode=true):  mic-test only, drives VolumeBar, no ASR
class AudioRecorder : public QObject {
  Q_OBJECT
  QML_ELEMENT
  // true while capturing (either mode)
  Q_PROPERTY(bool recording READ recording NOTIFY recordingChanged)
  // live volume level 0..100 (RMS). 0 when not recording.
  Q_PROPERTY(int level READ level NOTIFY levelChanged)
  // selected input device id (empty = system default)
  Q_PROPERTY(QString inputDeviceId READ inputDeviceId WRITE setInputDeviceId
                 NOTIFY inputDeviceIdChanged)

public:
  explicit AudioRecorder(QObject *parent = nullptr);
  ~AudioRecorder();

  bool recording() const { return m_recording; }
  int level() const { return m_level; }
  QString inputDeviceId() const { return m_inputDeviceId; }
  void setInputDeviceId(const QString &id);

  // Enumerate available audio input devices.
  // Returns [{id, description, isDefault}, ...]
  Q_INVOKABLE QVariantList inputDevices() const;

  // Start capture. testMode=true => mic-test (no ASR pipeline).
  Q_INVOKABLE void startRecording(bool testMode = false);
  Q_INVOKABLE void stopRecording();

  // Pop and clear the accumulated PCM buffer. Returns the raw bytes as a
  // QByteArray (exposed to QML as a byte-array variant; feed it straight to
  // asrClient.feed()). Q_INVOKABLE so QML can call it.
  Q_INVOKABLE QByteArray takeAudioData();

signals:
  void recordingChanged();
  void levelChanged();
  void inputDeviceIdChanged();
  void errorOccurred(const QString &error);

private:
  void setRecording(bool recording);
  void setLevel(int level);
  // resolve the QAudioDevice to use: selected id if set+found, else default.
  QAudioDevice resolveDevice() const;
  // compute RMS level (0..100) from the most recent PCM samples in m_buffer.
  void pollLevel();

  QAudioFormat m_format;
  QAudioSource *m_source = nullptr;
  QIODevice *m_io = nullptr;
  QByteArray m_buffer;
  QTimer m_levelTimer;

  bool m_recording = false;
  bool m_testMode = false;
  int m_level = 0;
  double m_smoothedLevel = 0.0;   // envelope-followed level for the visualizer
  QString m_inputDeviceId;
};
