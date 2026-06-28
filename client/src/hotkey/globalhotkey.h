// GlobalHotkey: Windows 全局热键封装
//
// 支持两种模式：
//   - press（默认）：按键按下时 emit activated，用于 toggle 行为（按一次开始，再按一次结束）
//   - hold：按下时 emit pressed，松开时 emit released，用于按住录音

#pragma once

#include <QAbstractNativeEventFilter>
#include <QMap>
#include <QObject>
#include <QString>
#include <QTimer>

class GlobalHotkey : public QObject, public QAbstractNativeEventFilter {
  Q_OBJECT
public:
  explicit GlobalHotkey(QObject *parent = nullptr);
  ~GlobalHotkey();

  // 注册形如 "F9" / "Ctrl+Shift+P" / "Alt+F10" 的热键
  // mode: "press"（toggle）或 "hold"（按住录音）
  Q_INVOKABLE bool registerShortcut(const QString &shortcut,
                                    const QString &name = QString(),
                                    const QString &mode = QString());
  Q_INVOKABLE void unregisterAll();
  // 仅注销指定 name 的热键（不影响其他 name）。用于多热键共存场景：
  // 改 record 热键时只 unregisterByName("record")，不误杀 screenshot 等其他热键。
  Q_INVOKABLE void unregisterByName(const QString &name);

  // 兼容旧接口:仅注册一个虚拟键(固定 Ctrl+Shift)
  Q_INVOKABLE bool registerHotkey(int vk);

  bool nativeEventFilter(const QByteArray &eventType, void *message,
                         qintptr *result) override;

signals:
  void activated();
  void activatedWithName(const QString &name);
  // hold 模式专用：按下/松开
  void pressed(const QString &name);
  void released(const QString &name);

private:
  struct Reg {
    int id = 0;
    QString name;
    QString mode;       // "press" | "hold"
    uint modifiers = 0;
    int vk = 0;
  };

  bool parseShortcut(const QString &shortcut, uint &modifiers, int &vk);
  int keyToVk(const QString &key);
  int m_nextId = 1;
  QMap<int, Reg> m_registrations;
  bool m_installed = false;

  // hold 模式：轮询 GetAsyncKeyState 检测松开（替代低级键盘钩子）。
  // 低级钩子 WH_KEYBOARD_LL 的回调跑在系统键盘链路里，若回调里同步执行
  // gRPC 等慢操作，会阻塞整个系统的键盘输入，导致电脑卡顿。改为轮询后，
  // 所有处理都在 Qt 主线程事件循环内完成，绝不阻塞系统键盘。
  QTimer m_holdPollTimer;
  int m_holdVk = 0;          // 正在追踪的虚拟键（0 = 未追踪）
  QString m_holdName;
  void stopHoldPolling();
  void onHoldPollTick();
};
