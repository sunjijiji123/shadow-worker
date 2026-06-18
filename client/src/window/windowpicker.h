#pragma once

#include <QObject>
#include <QQmlEngine>
#include <QString>
#include <QTimer>
#include <qqmlintegration.h>

// WindowPicker 提供"选窗口"能力。
// 当前为简化实现：调用 pick() 后等待几秒，取当前前台窗口。
class WindowPicker : public QObject {
  Q_OBJECT
  QML_ELEMENT
  Q_PROPERTY(bool picking READ picking NOTIFY pickingChanged)

public:
  explicit WindowPicker(QObject *parent = nullptr);

  bool picking() const { return m_picking; }

  Q_INVOKABLE void pick();

signals:
  void pickingChanged();
  void picked(const QString &path, const QString &name, const QString &title);
  void cancelled();

private:
  void finishPick();
  bool m_picking = false;
  QTimer *m_timer = nullptr;
};
