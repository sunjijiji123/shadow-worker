#ifndef SCREENSHOTWINDOW_H
#define SCREENSHOTWINDOW_H

#include <QColor>
#include <QList>
#include <QPixmap>
#include <QPoint>
#include <QRect>
#include <QString>
#include <QWidget>

// ScreenShotWindow: 全屏透明覆盖层，鼠标拖拽框选区域 → 裁剪落盘 + 写剪贴板。
//
// 交互流程：
//   Drag 阶段：鼠标悬停高亮光标下窗口（智能预选）；单击=选窗口进 Confirm；
//              按下拖拽=自由矩形框选。
//   Confirm 阶段：选区已确定，可同时做"调整选区"和"画标注"：
//     - m_currentTool == None 时：8 手柄缩放、选区内拖动移动选区。
//     - m_currentTool != None 时：在选区内画对应标注（矩形/箭头/画笔/马赛克）。
//     两者无缝切换——点工具条工具按钮选中工具，再点一次或按 ESC 取消选中
//     回到调选区模式。标注和选区调整在同一个阶段共存，不互相锁定。
//   ESC / 右键 / ✗ 取消。
//
// 构造时抓取所有显示器（虚拟屏并集）的当前画面存到 m_fullPixmap，
// 落盘时按选区在物理像素坐标裁剪（逻辑坐标 × dpr），保证高 DPI 清晰。
//
// 工具条按钮（Confirm 阶段，统一布局）：
//   [颜色×5] [□框][→箭头][✎笔][▦马赛克] | [↶撤销] [✦识别?] [✓完成] [✗取消]
class ScreenShotWindow : public QWidget {
  Q_OBJECT
 public:
  explicit ScreenShotWindow(const QString &saveDir,
                            bool showRecognizeBtn = false,
                            QWidget *parent = nullptr);

 signals:
  // 用户点 ✓ 完成，PNG 已落盘 + 已写剪贴板。path 为绝对路径。
  void finished(const QString &path);
  // 用户点 ✦ 识别，PNG 已落盘 + 已写剪贴板。上层据此强制触发 VLM 分析。
  void recognized(const QString &path);
  // 用户 ESC / 右键 / ✗ 取消。
  void cancelled();

 protected:
  void paintEvent(QPaintEvent *event) override;
  void mousePressEvent(QMouseEvent *event) override;
  void mouseMoveEvent(QMouseEvent *event) override;
  void mouseReleaseEvent(QMouseEvent *event) override;
  void keyPressEvent(QKeyEvent *event) override;

 private:
  // ---- 阶段 ----
  enum Phase { Drag, Confirm };
  // 8 个调整手柄位置（Confirm 阶段，m_currentTool==None 时用）
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
  // 标注工具
  enum Tool { None, Rect, Arrow, Pen, Mosaic };
  static constexpr int kHandleSize = 6;

  // 一条标注。坐标存储为选区相对坐标（减 m_selection.topLeft()），
  // 绘制/落盘时加回选区左上角。这样标注语义绑定到选区内容。
  struct Annotation {
    Tool tool = None;
    QColor color;
    QList<QPoint> points;  // Pen=轨迹点; Rect/Arrow=[start,end]; Mosaic=覆盖区域点集
    int penWidth = 3;
  };

  // ---- 截图/选区 ----
  void initFullscreenPixmap();
  bool saveSelection(QString &outPath);

  // ---- 手柄/工具条几何 ----
  QRect handleRect(HandlePos pos) const;
  HandlePos handleAt(QPoint globalPos) const;
  void setCursorForPos(QPoint globalPos);
  // 工具条总矩形。
  QRect toolbarRect() const;
  // 标注工具按钮。
  QRect toolBtnRect(Tool tool) const;
  // 颜色圆点。
  QRect colorDotRect(int index) const;
  // 撤销按钮。
  QRect undoBtnRect() const;
  // 识别/完成/取消按钮。
  QRect recognizeBtnRect() const;
  QRect confirmBtnRect() const;
  QRect cancelBtnRect() const;

  // ---- 窗口智能预选（Drag 阶段）----
  void detectHoverWindow(QPoint globalPos);
  void drawHoverHighlight(QPainter &p, QPoint origin) const;

  // ---- 标注 ----
  void drawAnnotation(QPainter &p, const Annotation &a,
                      QPoint selOrigin) const;
  void paintAnnotationsOnPixmap(QPixmap &pixmap, qreal dpr) const;
  void drawMosaic(QPainter &p, const Annotation &a, const QPixmap &srcPixmap,
                  QPoint selOrigin, qreal dpr) const;

  // ---- 工具条图标矢量绘制 ----
  // 在 rect 内用 QPainter 矢量绘制标注工具图标。color=图标颜色。
  void drawToolIcon(QPainter &p, Tool tool, const QRect &r,
                    const QColor &color) const;

  // ---- 数据 ----
  QString m_saveDir;
  bool m_showRecognizeBtn;
  Phase m_phase;
  QPixmap m_fullPixmap;
  QRect m_selection;
  QPoint m_startPoint;
  QPoint m_dragOffset;
  QPoint m_resizeAnchor;
  bool m_dragging;       // 选区拖动/缩放中
  bool m_movingSelection;  // true=整体移动选区（区别于手柄缩放）
  int m_activeHandle;

  // ---- 窗口智能预选 ----
  QRect m_hoverRect;
  QString m_hoverTitle;
  bool m_hoverValid;
  static constexpr int kDragThreshold = 4;

  // ---- 标注 ----
  Tool m_currentTool;
  QColor m_drawColor;
  QList<Annotation> m_annotations;
  bool m_drawingActive;  // 正在绘制标注（区别于选区拖动）
  Annotation m_drawingAnno;

  static constexpr int kColorCount = 5;
  QColor annotationColor(int index) const;
};

#endif  // SCREENSHOTWINDOW_H
