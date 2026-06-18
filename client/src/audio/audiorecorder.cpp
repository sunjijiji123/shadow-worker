#include "audiorecorder.h"

#include <QAudioDevice>
#include <QDebug>
#include <QMediaDevices>

AudioRecorder::AudioRecorder(QObject *parent) : QObject(parent) {
  m_format.setSampleRate(16000);
  m_format.setChannelCount(1);
  m_format.setSampleFormat(QAudioFormat::Int16);
}

AudioRecorder::~AudioRecorder() { stopRecording(); }

void AudioRecorder::startRecording() {
  if (m_recording)
    return;

  QAudioDevice inputDevice = QMediaDevices::defaultAudioInput();
  if (inputDevice.isNull()) {
    emit errorOccurred("未找到麦克风");
    return;
  }

  if (!inputDevice.isFormatSupported(m_format)) {
    emit errorOccurred("麦克风不支持 16kHz/mono/int16 格式");
    return;
  }

  m_buffer.clear();
  m_source = new QAudioSource(inputDevice, m_format, this);
  m_io = m_source->start();
  if (!m_io) {
    emit errorOccurred("无法启动录音");
    delete m_source;
    m_source = nullptr;
    return;
  }

  connect(m_io, &QIODevice::readyRead, this, [this]() {
    if (m_io)
      m_buffer.append(m_io->readAll());
  });

  setRecording(true);
  qDebug() << "AudioRecorder: 开始录音";
}

void AudioRecorder::stopRecording() {
  if (!m_recording)
    return;

  if (m_source) {
    m_source->stop();
    m_source->deleteLater();
    m_source = nullptr;
    m_io = nullptr;
  }

  setRecording(false);
  qDebug() << "AudioRecorder: 停止录音, bytes=" << m_buffer.size();
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
