#pragma once

#include <QObject>
#include <QVariantList>
#include <qqmlintegration.h>

class QMediaDevices;

// AudioDeviceManager: exposes the list of audio input devices to QML and
// persists the user's selected device id via QSettings (registry on Windows).
//
// Instantiated from QML (QML_ELEMENT). Listens for device-list changes (hotplug)
// and refreshes automatically. The selected device id defaults to empty
// (= system default).
class AudioDeviceManager : public QObject {
  Q_OBJECT
  QML_ELEMENT
  // [{id, description, isDefault}, ...]
  Q_PROPERTY(QVariantList devices READ devices NOTIFY devicesChanged)
  // selected input device id; empty = system default. Persisted to QSettings.
  Q_PROPERTY(QString selectedDeviceId READ selectedDeviceId WRITE
                 setSelectedDeviceId NOTIFY selectedDeviceIdChanged)

public:
  explicit AudioDeviceManager(QObject *parent = nullptr);
  ~AudioDeviceManager();

  QVariantList devices() const { return m_devices; }
  QString selectedDeviceId() const { return m_selectedId; }
  void setSelectedDeviceId(const QString &id);

  // manual refresh (in addition to automatic hotplug handling)
  Q_INVOKABLE void refresh();

signals:
  void devicesChanged();
  void selectedDeviceIdChanged();

private:
  void rebuildDeviceList();

  QVariantList m_devices;
  QString m_selectedId;
};
