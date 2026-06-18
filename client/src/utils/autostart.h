// AutostartManager: Windows 开机自启管理

#pragma once

#include <QObject>
#include <QString>

class AutostartManager : public QObject {
  Q_OBJECT
  Q_PROPERTY(bool enabled READ enabled WRITE setEnabled NOTIFY enabledChanged)

public:
  explicit AutostartManager(QObject *parent = nullptr);

  bool enabled() const;
  void setEnabled(bool v);

  Q_INVOKABLE bool isEnabled() const;
  Q_INVOKABLE void enable();
  Q_INVOKABLE void disable();

signals:
  void enabledChanged();

private:
  QString runKeyPath() const;
  QString appName() const { return QStringLiteral("ShadowWorker"); }
  QString executablePath() const;
};
