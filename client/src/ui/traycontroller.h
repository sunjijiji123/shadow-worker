#pragma once

#include <QObject>
#include <QSystemTrayIcon>

class QMenu;
class QAction;

// TrayController owns the system tray icon + its context menu and forwards
// user actions to QML via signals.
//
// Menu: Show Main Window / Settings... / Quit.
// Left-click (Trigger) on the tray icon => showMainRequested.
//
// Created in main.cpp and exposed to QML as the `trayController` context
// property. The app must NOT quit on last-window-closed (see
// QApplication::setQuitOnLastWindowClosed(false) in main.cpp) so that hiding
// the main window keeps the process alive with the tray present.
class TrayController : public QObject {
  Q_OBJECT

public:
  explicit TrayController(QObject *parent = nullptr);
  ~TrayController();

  // pop a balloon notification from the tray (optional convenience)
  Q_INVOKABLE void showMessage(const QString &title, const QString &msg,
                               int timeoutMs = 3000);

  // 重新翻译托盘菜单文字。C++ QAction 没有 QML 的 retranslate 机制，菜单文字
  // 在构造时一次性设置（此时翻译器可能还未安装）。故在 Translator 安装后、
  // 或运行时切换语言后，需显式调用本方法重设菜单文字。
  Q_INVOKABLE void retranslateUi();

signals:
  void showMainRequested();
  void settingsRequested();
  void screenshotRequested();
  void quitRequested();

private:
  QSystemTrayIcon *m_tray = nullptr;
  QMenu *m_menu = nullptr;
  QAction *m_actShow = nullptr;
  QAction *m_actSettings = nullptr;
  QAction *m_actScreenshot = nullptr;
  QAction *m_actQuit = nullptr;
};
