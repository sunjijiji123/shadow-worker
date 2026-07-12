#ifndef SCREENSHOTWINDOW_H
#define SCREENSHOTWINDOW_H

#include <QColor>
#include <QList>
#include <QPixmap>
#include <QPoint>
#include <QRect>
#include <QString>
#include <QWidget>

class QLineEdit;

// ScreenShotWindow: 全屏透明覆盖层，鼠标拖拽框选区域 → 裁剪落盘 + 写剪贴板。
//
// 交互流程：
//   Drag 阶段：鼠标悬停高亮光标下窗口（智能预选）；单击=选窗口进 Confirm；
//              按下拖拽=自由矩形框选。
//   Confirm 阶段：选区已确定，可同时做"调整选区"和"画标注"：
//     - m_currentTool == None 时：8 手柄缩放、选区内拖动移动选区。
//     - m_currentTool != None 时：在选区内画对应标注。
//     选中矩形/椭圆/箭头/画笔时，工具按钮下方弹出颜色+笔窗弹窗；
//     选中文字时弹出颜色+字号弹窗；马赛克无弹窗。
//   ESC / 右键 / ✗ 取消。
//
// 工具条布局（Confirm 阶段）：
//   [矩形][椭圆][箭头][画笔][马赛克][文字] | [撤销][置顶] | [✦识别?] [✗取消] [✓完成]
class ScreenShotWindow : public QWidget {
  Q_OBJECT
 public:
  explicit ScreenShotWindow(const QString &saveDir,
                            bool showRecognizeBtn = false,
                            QWidget *parent = nullptr);

 signals:
  void finished(const QString &path);
  void recognized(const QString &path);
  void pinned(const QString &path);
  void cancelled();

 protected:
  void paintEvent(QPaintEvent *event) override;
  void mousePressEvent(QMouseEvent *event) override;
  void mouseMoveEvent(QMouseEvent *event) override;
  void mouseReleaseEvent(QMouseEvent *event) override;
  void keyPressEvent(QKeyEvent *event) override;
  void wheelEvent(QWheelEvent *event) override;
  void focusOutEvent(QFocusEvent *event) override;

 private:
  enum Phase { Drag, Confirm };
  enum HandlePos {
    TopLeft, Top, TopRight, Right, BottomRight, Bottom, BottomLeft, Left
  };
  enum Tool { None, Rect, Ellipse, Arrow, Pen, Mosaic, Text };
  static constexpr int kHandleSize = 6;

  struct Annotation {
    Tool tool = None;
    QColor color;
    QList<QPoint> points;
    int penWidth = 3;
    int fontSize = 16;
    QString text;
  };

  // 截图/选区
  void initFullscreenPixmap();
  bool saveSelection(QString &outPath);

  // 手柄/工具条几何
  QRect handleRect(HandlePos pos) const;
  HandlePos handleAt(QPoint globalPos) const;
  void setCursorForPos(QPoint globalPos);
  QRect toolbarRect() const;
  QRect toolBtnRect(Tool tool) const;
  QRect undoBtnRect() const;
  QRect pinBtnRect() const;
  QRect recognizeBtnRect() const;
  QRect cancelBtnRect() const;
  QRect confirmBtnRect() const;
  // 弹窗几何（在当前选中工具按钮下方）
  QRect popupRect() const;
  QRect popupColorDotRect(int index) const;
  QRect popupPenRect(int index) const;    // 笔粗档位 0..3
  QRect popupSizeRect(int index) const;   // 字号档位 0..2
  QRect popupVsepRect() const;

  // 窗口预选
  void detectHoverWindow(QPoint globalPos);
  void drawHoverHighlight(QPainter &p, QPoint origin) const;

  // 标注
  void drawAnnotation(QPainter &p, const Annotation &a,
                      QPoint selOrigin) const;
  void paintAnnotationsOnPixmap(QPixmap &pixmap, qreal dpr) const;
  void drawMosaic(QPainter &p, const Annotation &a, const QPixmap &srcPixmap,
                  QPoint selOrigin, qreal dpr) const;

  // 工具图标
  void drawToolIcon(QPainter &p, Tool tool, const QRect &r,
                    const QColor &color) const;

  // 文字输入
  void startTextInput(QPoint globalPos);
  void commitTextInput();

  // 选区视觉
  void drawSelectionBorder(QPainter &p, QPoint localTopLeft) const;

  // 工具条按钮命中检测（返回命中按钮的 rect 是否包含 lpos）
  bool handleToolButtonClick(QPoint lpos, QPoint gpos);
  void handlePopupClick(QPoint lpos);

  QColor annotationColor(int index) const;
  int penWidthForIndex(int index) const;   // 0→2, 1→3, 2→5, 3→8
  int fontSizeForIndex(int index) const;   // 0→12(小), 1→16(中), 2→22(大)

  // 数据
  QString m_saveDir;
  bool m_showRecognizeBtn;
  Phase m_phase;
  QPixmap m_fullPixmap;
  QRect m_selection;
  QPoint m_startPoint;
  QPoint m_dragOffset;
  QPoint m_resizeAnchor;
  bool m_dragging;
  bool m_movingSelection;  // true=整体移动选区
  int m_activeHandle;

  // 窗口预选
  QRect m_hoverRect;
  QString m_hoverTitle;
  bool m_hoverValid;
  static constexpr int kDragThreshold = 4;

  // 标注
  Tool m_currentTool;
  QColor m_drawColor;
  int m_penWidth;        // 当前笔粗（像素）
  int m_textFontSize;    // 当前字号
  QList<Annotation> m_annotations;
  bool m_drawingActive;
  Annotation m_drawingAnno;

  // 文字输入
  QLineEdit *m_textInput;       // 非空=正在输入文字
  QPoint m_textPos;             // 文字输入位置（选区相对）

  // 弹窗（由 m_currentTool 控制：Rect/Ellipse/Arrow/Pen→颜色+笔粗,
  //  Text→颜色+字号, Mosaic/None→无弹窗）
  bool m_popupVisible;

  // hover 状态：工具条按钮 hover 高亮（-1=无 hover）
  int m_hoveredBtn;  // 0..5=工具按钮, 6=撤销, 7=置顶, 8=识别, 9=取消, 10=完成
};

#endif  // SCREENSHOTWINDOW_H
