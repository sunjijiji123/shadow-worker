#include "audiorecorder.h"

#include <QAudioDevice>
#include <QDebug>
#include <QMediaDevices>
#include <QtMath>

#include <cmath>

AudioRecorder::AudioRecorder(QObject *parent) : QObject(parent) {
  m_format.setSampleRate(16000);
  m_format.setChannelCount(1);
  m_format.setSampleFormat(QAudioFormat::Int16);

  // level meter poll: 50ms while recording
  m_levelTimer.setInterval(50);
  m_levelTimer.setSingleShot(false);
  connect(&m_levelTimer, &QTimer::timeout, this, &AudioRecorder::pollLevel);
}

AudioRecorder::~AudioRecorder() { stopRecording(); }

QVariantList AudioRecorder::inputDevices() const {
  QVariantList out;
  const QAudioDevice defaultDev = QMediaDevices::defaultAudioInput();
  const QString defaultId = defaultDev.id();
  for (const QAudioDevice &dev : QMediaDevices::audioInputs()) {
    QVariantMap m;
    m["id"] = dev.id();
    m["description"] = dev.description();
    m["isDefault"] = (dev.id() == defaultId);
    out.append(m);
  }
  return out;
}

void AudioRecorder::setInputDeviceId(const QString &id) {
  if (m_inputDeviceId == id)
    return;
  m_inputDeviceId = id;
  emit inputDeviceIdChanged();
}

QAudioDevice AudioRecorder::resolveDevice() const {
  // selected id wins if it still exists; otherwise fall back to default.
  if (!m_inputDeviceId.isEmpty()) {
    for (const QAudioDevice &dev : QMediaDevices::audioInputs()) {
      if (dev.id() == m_inputDeviceId)
        return dev;
    }
  }
  return QMediaDevices::defaultAudioInput();
}

void AudioRecorder::startRecording(bool testMode) {
  if (m_recording)
    return;

  m_testMode = testMode;
  QAudioDevice inputDevice = resolveDevice();
  if (inputDevice.isNull()) {
    emit errorOccurred(QStringLiteral("No microphone found"));
    return;
  }

  if (!inputDevice.isFormatSupported(m_format)) {
    emit errorOccurred(QStringLiteral(
        "Microphone does not support 16kHz/mono/int16"));
    return;
  }

  m_buffer.clear();
  m_smoothedLevel = 0.0;   // reset the envelope follower on a fresh capture
  m_source = new QAudioSource(inputDevice, m_format, this);
  m_io = m_source->start();
  if (!m_io) {
    emit errorOccurred(QStringLiteral("Unable to start recording"));
    delete m_source;
    m_source = nullptr;
    return;
  }

  connect(m_io, &QIODevice::readyRead, this, [this]() {
    if (m_io)
      m_buffer.append(m_io->readAll());
  });

  setRecording(true);
  m_levelTimer.start();
  qDebug() << "AudioRecorder: start recording (testMode=" << m_testMode << ")";
}

void AudioRecorder::stopRecording() {
  if (!m_recording)
    return;

  m_levelTimer.stop();
  setLevel(0);

  if (m_source) {
    m_source->stop();
    m_source->deleteLater();
    m_source = nullptr;
    m_io = nullptr;
  }

  setRecording(false);
  qDebug() << "AudioRecorder: stop recording, bytes=" << m_buffer.size();
}

QByteArray AudioRecorder::takeAudioData() {
  QByteArray data;
  data.swap(m_buffer);
  return data;
}

void AudioRecorder::setRecording(bool recording) {
  if (m_recording == recording)
    return;
  m_recording = recording;
  emit recordingChanged();
}

void AudioRecorder::setLevel(int level) {
  // clamp + ignore tiny noise so the bar rests at 0 when silent
  if (level < 0)
    level = 0;
  if (level > 100)
    level = 100;
  if (m_level == level)
    return;
  m_level = level;
  emit levelChanged();
}

void AudioRecorder::pollLevel() {
  if (!m_io)
    return;
  // Inspect the tail of the buffer (last ~20ms of 16kHz mono Int16 = ~640 bytes).
  const int bytesPerSample = 2;
  const int samplesToCheck = 320;  // 20ms
  const int bytesNeeded = samplesToCheck * bytesPerSample;

  if (m_buffer.size() < bytesNeeded)
    return;

  const auto *samples =
      reinterpret_cast<const qint16 *>(m_buffer.constData());
  int total = m_buffer.size() / bytesPerSample;
  int start = total - samplesToCheck;
  if (start < 0)
    start = 0;

  // RMS over the window, mapped to 0..100.
  // Full-scale Int16 = 32767; RMS of ~32767 -> 100.
  double sumSq = 0.0;
  int count = 0;
  for (int i = start; i < total; ++i) {
    double v = samples[i] / 32767.0;
    sumSq += v * v;
    ++count;
  }
  if (count == 0) {
    setLevel(0);
    return;
  }
  double rms = std::sqrt(sumSq / count);
  // Map RMS (0..1) to a target level 0..100 for the visualizer.
  // 1) Noise gate: RMS below ~0.012 (~-38dB) is treated as silence so ambient
  //    room noise doesn't make the wave jitter. Speech is well above this.
  // 2) Moderate gain (3x) + soft compression so the wave is clearly reactive
  //    but not jumpy on quiet background.
  double target;
  if (rms < 0.012) {
    target = 0.0;
  } else {
    double scaled = rms * 3.0;
    double compressed = scaled / (scaled + 0.9);
    target = compressed * 100.0;
  }
  // 3) Envelope follower (one-pole smoother) so the displayed level rises fast
  //    but falls slowly — like a real VU/spectrum meter, not a raw twitchy RMS.
  //    Timer ticks every 50ms (dt=0.05s). attack ~40ms, release ~180ms.
  //    alpha = 1 - exp(-dt/tau). Pick alpha by whether target rose or fell.
  double dt = 0.05;
  double tau = (target > m_smoothedLevel) ? 0.040 : 0.180;
  double alpha = 1.0 - std::exp(-dt / tau);
  m_smoothedLevel += alpha * (target - m_smoothedLevel);
  int lvl = static_cast<int>(m_smoothedLevel + 0.5);
  setLevel(lvl);
}
