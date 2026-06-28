#include "screenshotcontroller.h"

#include "screenshotwindow.h"

#include <QDir>
#include <QStandardPaths>

ScreenshotController::ScreenshotController(QObject *parent)
    : QObject(parent) {}

void ScreenshotController::capture(const QString &saveDir) {
  // 防重入：已有覆盖层在活动，忽略。
  if (m_window) return;

  // 默认目录：%APPDATA%\shadow-worker\screenshots（与后端 VLM 截图目录一致）。
  // QStandardPaths::GenericConfigLocation 在 Windows 上 = %APPDATA%。
  QString dir = saveDir;
  if (dir.isEmpty()) {
    dir = QStandardPaths::writableLocation(QStandardPaths::GenericConfigLocation) +
          QStringLiteral("/shadow-worker/screenshots");
  }

  m_window = new ScreenShotWindow(dir);
  // 关闭后由 deleteLater 回收，并清空 m_window（避免悬空指针）。
  m_window->setAttribute(Qt::WA_DeleteOnClose);

  connect(m_window, &ScreenShotWindow::finished, this,
          [this](const QString &path) {
            m_window = nullptr;
            emit finished(path);
          });
  connect(m_window, &ScreenShotWindow::cancelled, this, [this]() {
    m_window = nullptr;
    emit cancelled();
  });

  m_window->show();
  m_window->activateWindow();
  m_window->raise();
}
