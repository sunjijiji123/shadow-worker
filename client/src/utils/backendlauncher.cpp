#include "backendlauncher.h"

#include <QCoreApplication>
#include <QDir>
#include <QFileInfo>
#include <QProcess>
#include <QThread>

#ifdef Q_OS_WIN
#include <windows.h>
#endif

BackendLauncher::BackendLauncher(QObject *parent) : QObject(parent) {}

QString BackendLauncher::resolveExePath() {
  // 后台服务 exe（gRPC + 采集 + ASR）。
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

QString BackendLauncher::resolveMcpExePath() {
  // MCP server exe。优先独立 exe（shadow-worker-mcp.exe，与主后端文件隔离，
  // 升级主程序不被 MCP 子进程锁文件阻断，见 AGENTS.md 坑 50）；
  // 找不到时 fallback 到主后端 exe（旧安装 / 开发期未拆分，此时 MCP 配置
  // 在 QML 侧会带 ["--mcp"] 参数）。
  const QString clientDir =
      QFileInfo(QCoreApplication::applicationFilePath()).absolutePath();
  const QString mcpName = QStringLiteral("shadow-worker-mcp.exe");
  const QStringList mcpCandidates = {
      QDir(clientDir).absoluteFilePath(mcpName),
      QDir(clientDir).absoluteFilePath(QStringLiteral("../../build/") + mcpName),
  };
  for (const QString &p : mcpCandidates) {
    const QString abs = QDir(p).absolutePath();
    if (QFileInfo::exists(abs)) return abs;
  }
  // fallback：主后端 exe（resolveExePath 已含两个候选目录）。
  return resolveExePath();
}

#ifdef Q_OS_WIN
// GetShortPathNameW 的公共实现（resolveShortPath / resolveMcpShortPath 共用）。
// exe 为空或转换失败返回空串。
static QString toShortPath(const QString &exe) {
  if (exe.isEmpty()) return {};
  // GetShortPathNameW 需要反斜杠原生路径。先转一下。
  const QString native = QDir::toNativeSeparators(exe);
  // 第一次调用拿所需缓冲区长度，第二次写入短路径。
  const std::wstring w = native.toStdWString();
  DWORD len = GetShortPathNameW(w.c_str(), nullptr, 0);
  if (len == 0) return {};  // API 失败（文件不存在 / 卷禁用 8.3）
  std::wstring buf(len, L'\0');
  DWORD written = GetShortPathNameW(w.c_str(), &buf[0], len);
  if (written == 0 || written >= len) return {};  // 失败或截断
  buf.resize(written);
  return QString::fromStdWString(buf);
}
#endif

QString BackendLauncher::resolveShortPath() {
  const QString exe = resolveExePath();
#ifdef Q_OS_WIN
  return toShortPath(exe);
#else
  return exe;  // 非 Windows 无短路径概念，原样返回
#endif
}

QString BackendLauncher::resolveMcpShortPath() {
  const QString exe = resolveMcpExePath();
#ifdef Q_OS_WIN
  return toShortPath(exe);
#else
  return exe;
#endif
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
