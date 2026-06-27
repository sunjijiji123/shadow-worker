#ifndef BACKENDLAUNCHER_H
#define BACKENDLAUNCHER_H

#include <QObject>
#include <qglobal.h>

// BackendLauncher 负责拉起后端 exe 并在客户端退出时关闭它。
//
// 生命周期：
//   start()  → QProcess::startDetached 拉起后端，记录 PID
//   stop()   → aboutToQuit 时调：先 taskkill /PID（graceful），等 2.5s，
//              仍存活再 taskkill /F /T /PID 兜底
//
// 用 startDetached 而非 start：start 绑定父进程生命周期，父退出子跟着死，
// 但我们就是要主动管理后端生命周期，所以用 startDetached + 记 PID。
class BackendLauncher : public QObject {
  Q_OBJECT

 public:
  explicit BackendLauncher(QObject *parent = nullptr);

  // 探测后端 exe 路径。
  // 候选：1) client exe 同目录（release 布局）
  //       2) ../../build/（dev 布局：client/build/ → repo/build/）
  // 返回空字符串表示未找到（客户端优雅降级）。
  static QString resolveExePath();

  // 拉起后端。成功返回 true 并记录 PID。
  // 不等待 gRPC 就绪——startDetached 异步，后端启动有 1-3s 延迟，
  // 客户端优雅降级，UI 会显示 gRPC error 直到后端起来。
  bool start();

  // 退出时关闭后端。仅当 start() 成功过才执行。
  // 先发 taskkill /PID（不带 /F），让后端走 signal.NotifyContext → GracefulStop。
  // 等 2.5 秒后仍存活则 taskkill /F /T /PID 兜底。
  void stop();

 private:
  qint64 m_pid = 0;
  bool m_startedByUs = false;
};

#endif  // BACKENDLAUNCHER_H
