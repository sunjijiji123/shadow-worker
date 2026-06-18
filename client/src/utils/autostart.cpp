// AutostartManager 实现

#include "autostart.h"

#include <QCoreApplication>
#include <QDir>
#include <QSettings>

AutostartManager::AutostartManager(QObject *parent) : QObject(parent) {}

bool AutostartManager::enabled() const { return isEnabled(); }

void AutostartManager::setEnabled(bool v) {
  if (v)
    enable();
  else
    disable();
}

bool AutostartManager::isEnabled() const {
  QSettings settings(runKeyPath(), QSettings::NativeFormat);
  return settings.contains(appName());
}

void AutostartManager::enable() {
  const QString path = executablePath();
  if (path.isEmpty())
    return;

  QSettings settings(runKeyPath(), QSettings::NativeFormat);
  settings.setValue(appName(), QStringLiteral("\"%1\" --autostart").arg(path));
  emit enabledChanged();
}

void AutostartManager::disable() {
  QSettings settings(runKeyPath(), QSettings::NativeFormat);
  settings.remove(appName());
  emit enabledChanged();
}

QString AutostartManager::runKeyPath() const {
  return QStringLiteral(
      "HKEY_CURRENT_USER\\Software\\Microsoft\\Windows\\CurrentVersion\\Run");
}

QString AutostartManager::executablePath() const {
  return QDir::toNativeSeparators(QCoreApplication::applicationFilePath());
}
