#include "audiodevicemanager.h"

#include <QAudioDevice>
#include <QMediaDevices>
#include <QSettings>

AudioDeviceManager::AudioDeviceManager(QObject *parent) : QObject(parent) {
  // restore last-selected device id from QSettings
  QSettings s;
  m_selectedId = s.value(QStringLiteral("audio/inputDevice")).toString();

  rebuildDeviceList();

  // NOTE: Qt 6.11 removed the QMediaDevices hotplug singleton/signal, so we
  // don't auto-detect device plug/unplug here. QML calls refresh() on demand
  // (e.g. when opening the device dropdown) to pick up new devices.
}

AudioDeviceManager::~AudioDeviceManager() = default;

void AudioDeviceManager::setSelectedDeviceId(const QString &id) {
  if (m_selectedId == id)
    return;
  m_selectedId = id;
  // persist
  QSettings s;
  s.setValue(QStringLiteral("audio/inputDevice"), id);
  emit selectedDeviceIdChanged();
}

void AudioDeviceManager::refresh() { rebuildDeviceList(); }

void AudioDeviceManager::rebuildDeviceList() {
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

  // only emit if the list actually changed
  if (out.size() != m_devices.size()) {
    m_devices = out;
    emit devicesChanged();
    return;
  }
  for (int i = 0; i < out.size(); ++i) {
    if (out[i].toMap()["id"] != m_devices[i].toMap()["id"]) {
      m_devices = out;
      emit devicesChanged();
      return;
    }
  }
}
