#include "backendlauncher.h"

#include <QCoreApplication>
#include <QDir>
#include <QFileInfo>
#include <QProcess>
#include <QThread>

BackendLauncher::BackendLauncher(QObject *parent) : QObject(parent) {}

QString BackendLauncher::resolveExePath() {
  const QString exeName = QStringLiteral("shadow-worker.exe");
  const QString clientDir =
      QFileInfo(QCoreApplication::applicationFilePath()).absolutePath();
  const QStringList candidates = {
      QDir(clientDir).absoluteFilePath(exeName),
      QDir(clientDir).absoluteFilePath(
          QStringLiteral("../../build/") + exeName),
  };
  for (const QString &p : candidates) {
    const QString abs = QDir(p).absolutePath();
    if (QFileInfo::exists(abs)) return abs;
  }
  return {};
}

bool BackendLauncher::start() {
  QString exe = resolveExePath();
  if (exe.isEmpty()) return false;  // 找不到，优雅降级

  QString workDir = QFileInfo(exe).absolutePath();
  bool ok = QProcess::startDetached(
      exe, {}, QDir::toNativeSeparators(workDir), &m_pid);
  if (ok) m_startedByUs = true;
  return ok;
}

void BackendLauncher::stop() {
  if (!m_startedByUs || m_pid <= 0) return;

  // 先 graceful（不带 /F），让后端走 signal.NotifyContext
  // → GracefulStop → defer（db.Close 等）正常执行。
  QProcess::startDetached(
      QStringLiteral("taskkill"),
      {QStringLiteral("/PID"), QString::number(m_pid)},
      {}, nullptr);

  // 等 2.5 秒让 graceful 完成
  QThread::msleep(2500);

  // 兜底：仍存活则强杀
  QProcess::startDetached(
      QStringLiteral("taskkill"),
      {QStringLiteral("/F"), QStringLiteral("/T"),
       QStringLiteral("/PID"), QString::number(m_pid)},
      {}, nullptr);
}
