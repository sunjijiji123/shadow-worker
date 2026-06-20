#include "traycontroller.h"

#include <QAction>
#include <QApplication>
#include <QMenu>
#include <QStyle>

TrayController::TrayController(QObject *parent) : QObject(parent) {
  // Build the context menu (right-click).
  m_menu = new QMenu();
  m_actShow = m_menu->addAction(tr("Show Main Window"));
  m_actSettings = m_menu->addAction(tr("Settings..."));
  m_menu->addSeparator();
  m_actQuit = m_menu->addAction(tr("Quit"));

  // Tray icon. Uses Qt's standard computer icon as a placeholder until a
  // branded .ico is added.
  m_tray = new QSystemTrayIcon(this);
  m_tray->setIcon(QIcon(QApplication::style()->standardIcon(
      QStyle::SP_ComputerIcon)));
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
