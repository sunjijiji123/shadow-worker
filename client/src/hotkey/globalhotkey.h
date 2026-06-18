// GlobalHotkey: Windows 全局热键封装

#pragma once

#include <QAbstractNativeEventFilter>
#include <QMap>
#include <QObject>
#include <QString>

class GlobalHotkey : public QObject, public QAbstractNativeEventFilter {
  Q_OBJECT
public:
  explicit GlobalHotkey(QObject *parent = nullptr);
  ~GlobalHotkey();

  // 注册形如 "F9" / "Ctrl+Shift+P" / "Alt+F10" 的热键
  Q_INVOKABLE bool registerShortcut(const QString &shortcut,
                                    const QString &name = QString());
  Q_INVOKABLE void unregisterAll();

  // 兼容旧接口:仅注册一个虚拟键(固定 Ctrl+Shift)
  Q_INVOKABLE bool registerHotkey(int vk);

  bool nativeEventFilter(const QByteArray &eventType, void *message,
                         qintptr *result) override;

signals:
  void activated();
  void activatedWithName(const QString &name);

private:
  struct Reg {
    int id = 0;
    QString name;
  };

  bool parseShortcut(const QString &shortcut, uint &modifiers, int &vk);
  int keyToVk(const QString &key);
  int m_nextId = 1;
  QMap<int, Reg> m_registrations;
  bool m_installed = false;
};
