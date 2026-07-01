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

  // 探测后台服务 exe（shadow-worker.exe）路径。
  // 候选：1) client exe 同目录（release 布局）
  //       2) ../../build/（dev 布局：client/build/ → repo/build/）
  // 返回空字符串表示未找到（客户端优雅降级）。
  // 注意：这个函数返回的是「后台服务」exe，供 start() 拉起。
  // MCP 配置用的是另一个 exe（resolveMcpExePath），见下。
  static QString resolveExePath();

  // 探测 MCP server exe 路径，供「系统设置 → MCP 服务」生成 agent 配置。
  // 优先 shadow-worker-mcp.exe（MCP 拆分后的独立 exe，纯 Go，与主后端文件隔离）；
  // 找不到时 fallback 到 shadow-worker.exe（兼容旧安装 / 开发期未拆分，
  // 此时的 MCP 配置仍会带 ["--mcp"] 参数——由 QML 侧根据 exe 名决定）。
  // 候选目录同 resolveExePath：client 同目录 或 ../../build/。
  static QString resolveMcpExePath();

  // 把 resolveExePath() 返回的路径转成 Windows 8.3 短路径名
  // （如 C:\PROGRA~2\SHADOW~1\shadow-worker.exe）。
  // 用途：work buddy/TRAE 这类 MCP 客户端只接受裸路径 command（不能带引号），
  // 但含空格的路径（"C:\Program Files (x86)\..."）会被当 shell 命令在首个空格
  // 截断。短路径不含空格，裸路径即可用。调用 Win32 GetShortPathNameW 拿权威值
  // （序号 ~N 取决于目录内同前缀文件排序，无法靠启发式可靠推断）。
  // 转换失败（非 Windows 或 API 报错）返回空串，前端回退到带引号配置。
  static QString resolveShortPath();

  // 同 resolveShortPath，但作用于 resolveMcpExePath() 返回的 MCP exe 路径。
  // 用途：work buddy/TRAE 的 MCP 配置（指向 shadow-worker-mcp.exe）。
  static QString resolveMcpShortPath();

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
