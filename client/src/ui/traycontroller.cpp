#include "traycontroller.h"

#include <QAction>
#include <QApplication>
#include <QMenu>

TrayController::TrayController(QObject *parent) : QObject(parent) {
  // Build the context menu (right-click).
  m_menu = new QMenu();
  m_actShow = m_menu->addAction(tr("Show Main Window"));
  m_actSettings = m_menu->addAction(tr("Settings..."));
  m_menu->addSeparator();
  m_actQuit = m_menu->addAction(tr("Quit"));

  // Tray icon: reuse the application window icon (set in main.cpp via
  // setWindowIcon). This is the branded product icon from app.ico.
  m_tray = new QSystemTrayIcon(this);
  m_tray->setIcon(QApplication::windowIcon());
  m_tray->setToolTip(QStringLiteral("Shadow Worker"));
  m_tray->setContextMenu(m_menu);
  m_tray->show();

  // Menu actions -> signals
  connect(m_actShow, &QAction::triggered, this,
          &TrayController::showMainRequested);
  connect(m_actSettings, &QAction::triggered, this,
          &TrayController::settingsRequested);
  connect(m_actQuit, &QAction::triggered, this,
          &TrayController::quitRequested);

  // Single left-click (Trigger) on the tray icon => show main window.
  // DoubleClick / Context / MiddleClick are ignored.
  connect(m_tray, &QSystemTrayIcon::activated, this,
          [this](QSystemTrayIcon::ActivationReason reason) {
            if (reason == QSystemTrayIcon::Trigger)
              emit showMainRequested();
          });
}

TrayController::~TrayController() {
  if (m_tray) m_tray->hide();
}

void TrayController::showMessage(const QString &title, const QString &msg,
                                 int timeoutMs) {
  if (m_tray && QSystemTrayIcon::supportsMessages())
    m_tray->showMessage(title, msg, m_tray->icon(), timeoutMs);
}
