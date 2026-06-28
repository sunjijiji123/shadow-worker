#ifndef SCREENSHOTCONTROLLER_H
#define SCREENSHOTCONTROLLER_H

#include <QObject>
#include <QString>

class ScreenShotWindow;

// ScreenshotController: QML ↔ QWidget 桥。
//
// QML 无法直接实例化 QWidget（ScreenShotWindow），故由这个 QObject 持有
// ScreenShotWindow 生命周期并转发其 finished/cancelled 信号给 QML。
//
// 暴露为 context property `screenshotController`。QML 调用 capture(saveDir)
// 弹出全屏框选覆盖层；用户 ✓完成时 emit finished(path)，✗/ESC 时 emit
// cancelled()。模态期间 ScreenShotWindow 由本对象持有，关闭后 deleteLater。
class ScreenshotController : public QObject {
  Q_OBJECT
 public:
  explicit ScreenshotController(QObject *parent = nullptr);

  // 弹出截图覆盖层。saveDir 为空时回落到默认目录
  // %APPDATA%\shadow-worker\screenshots（与后端 VLM 截图目录一致）。
  // 若已有截图在进行中，忽略本次调用（防重入）。
  Q_INVOKABLE void capture(const QString &saveDir = QString());

 signals:
  // 用户 ✓ 完成，PNG 已落盘 + 已写剪贴板。path 为绝对路径。
  void finished(const QString &path);
  // 用户 ESC / 右键 / ✗ 取消。
  void cancelled();

 private:
  ScreenShotWindow *m_window = nullptr;  // 当前活动的覆盖层（nullptr = 无）
};

#endif  // SCREENSHOTCONTROLLER_H
