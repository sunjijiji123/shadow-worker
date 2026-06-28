#ifndef SCREENSHOTWINDOW_H
#define SCREENSHOTWINDOW_H

#include <QPixmap>
#include <QPoint>
#include <QRect>
#include <QString>
#include <QWidget>

// ScreenShotWindow: 全屏透明覆盖层，鼠标拖拽框选区域 → 裁剪落盘 + 写剪贴板。
//
// 移植自 ai-voice-tool（Qt5 → Qt6）。交互流程：
//   Drag 阶段：鼠标按下 → 拖拽框选矩形 → 松开进入 Confirm 阶段
//   Confirm 阶段：8 个调整手柄可缩放、选区内可整体移动、工具条 ✓完成/✗取消
//   ESC / 右键 / ✗ 取消
//
// 构造时抓取所有显示器（虚拟屏并集）的当前画面存到 m_fullPixmap，
// 落盘时按选区在物理像素坐标裁剪（逻辑坐标 × dpr），保证高 DPI 清晰。
class ScreenShotWindow : public QWidget {
  Q_OBJECT
 public:
  explicit ScreenShotWindow(const QString &saveDir,
                            QWidget *parent = nullptr);

 signals:
  // 用户点 ✓ 完成，PNG 已落盘 + 已写剪贴板。path 为绝对路径。
  void finished(const QString &path);
  // 用户 ESC / 右键 / ✗ 取消。
  void cancelled();

 protected:
  void paintEvent(QPaintEvent *event) override;
  void mousePressEvent(QMouseEvent *event) override;
  void mouseMoveEvent(QMouseEvent *event) override;
  void mouseReleaseEvent(QMouseEvent *event) override;
  void keyPressEvent(QKeyEvent *event) override;

 private:
  enum Phase { Drag, Confirm };
  // 8 个调整手柄位置（Confirm 阶段用）
  enum HandlePos {
    TopLeft,
    Top,
    TopRight,
    Right,
    BottomRight,
    Bottom,
    BottomLeft,
    Left
  };
  static constexpr int kHandleSize = 6;

  void initFullscreenPixmap();
  QRect handleRect(HandlePos pos) const;
  HandlePos handleAt(QPoint globalPos) const;
  void setCursorForPos(QPoint globalPos);
  QRect toolbarRect() const;
  QRect confirmBtnRect() const;
  QRect cancelBtnRect() const;

  QString m_saveDir;
  Phase m_phase;
  QPixmap m_fullPixmap;  // 虚拟屏并集画面（物理像素，已设 devicePixelRatio）
  QRect m_selection;     // 选区，全局虚拟屏坐标
  QPoint m_startPoint;   // Drag 阶段拖拽起点（全局）
  QPoint m_dragOffset;   // Confirm 阶段整体移动偏移
  QPoint m_resizeAnchor;  // Confirm 阶段缩放锚点（对角/对边）
  bool m_dragging;
  int m_activeHandle;  // Confirm 阶段当前拖拽的手柄（-1 = 无）
};

#endif  // SCREENSHOTWINDOW_H
