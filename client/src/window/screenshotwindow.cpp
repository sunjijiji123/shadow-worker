#include "screenshotwindow.h"

#include <cmath>

#include <QApplication>
#include <QClipboard>
#include <QDateTime>
#include <QDir>
#include <QFile>
#include <QFontMetrics>
#include <QGuiApplication>
#include <QImage>
#include <QKeyEvent>
#include <QLineEdit>
#include <QMouseEvent>
#include <QPainter>
#include <QPainterPath>
#include <QScreen>
#include <QSvgRenderer>

#include <qt_windows.h>

// ---- 常量 ----
// 工具条按钮尺寸
static constexpr int kBtnW = 36;        // 图标按钮宽
static constexpr int kActionBtnW = 56;  // 识别/取消/完成按钮宽
static constexpr int kBtnH = 32;
static constexpr int kGap = 4;
static constexpr int kSepGap = 10;
static constexpr int kPadding = 4;
// 弹窗
static constexpr int kPopupDotSize = 18;    // 颜色圆点
static constexpr int kPopupPenBtn = 28;     // 笔粗按钮宽
static constexpr int kPopupSizeW = 32;      // 字号按钮宽
static constexpr int kPopupPadding = 8;
// 颜色/笔粗/字号数量（文件级常量，供自由函数 popupContentWidth 使用）
static constexpr int kColorCount = 5;
static constexpr int kPenCount = 4;
static constexpr int kSizeCount = 3;

// 色值（对齐设计稿）
static const QColor kColorBorder = QColor(59, 130, 246);   // #3b82f6 选区蓝
static const QColor kColorToolbarBg = QColor(45, 45, 51);  // #2d2d33
static const QColor kColorConfirm = QColor(16, 185, 129);  // #10b981 完成绿
static const QColor kColorCancel = QColor(244, 67, 54);    // #f44336 取消红
static const QColor kColorVlm = QColor(59, 130, 246);      // #3b82f6 识别蓝
static const QColor kColorActive = QColor(59, 130, 246, 180);  // 工具选中蓝

namespace {
constexpr UINT kGaRoot = 2;

bool isHighlightableWindow(HWND hwnd, HWND selfHwnd) {
  if (!hwnd || hwnd == selfHwnd) return false;
  if (!IsWindowVisible(hwnd)) return false;
  WCHAR title[256] = {0};
  if (GetWindowTextW(hwnd, title, 256) == 0) return false;
  HWND ancestor = GetAncestor(hwnd, kGaRoot);
  if (ancestor == selfHwnd) return false;
  return true;
}
}  // namespace

// ---- 构造 ----

ScreenShotWindow::ScreenShotWindow(const QString &saveDir,
                                   bool showRecognizeBtn,
                                   QWidget *parent)
    : QWidget(parent),
      m_saveDir(saveDir),
      m_showRecognizeBtn(showRecognizeBtn),
      m_phase(Drag),
      m_dragging(false),
      m_movingSelection(false),
      m_activeHandle(-1),
      m_hoverValid(false),
      m_currentTool(None),
      m_drawColor(QColor(239, 68, 68)),
      m_penWidth(3),
      m_textFontSize(16),
      m_drawingActive(false),
      m_textInput(nullptr),
      m_popupVisible(false),
      m_hoveredBtn(-1) {
  setWindowFlags(Qt::FramelessWindowHint | Qt::Tool |
                 Qt::WindowStaysOnTopHint);
  setAttribute(Qt::WA_TranslucentBackground, true);
  setAttribute(Qt::WA_NoSystemBackground, true);
  setMouseTracking(true);

  QRect total;
  for (QScreen *screen : QGuiApplication::screens()) {
    total = total.united(screen->geometry());
  }
  setGeometry(total);

  initFullscreenPixmap();
  setCursor(Qt::CrossCursor);
}

void ScreenShotWindow::initFullscreenPixmap() {
  QRect total = geometry();
  qreal dpr = devicePixelRatioF();
  m_fullPixmap = QPixmap(total.size() * dpr);
  m_fullPixmap.fill(Qt::transparent);

  QPainter p(&m_fullPixmap);
  for (QScreen *screen : QGuiApplication::screens()) {
    QRect geo = screen->geometry();
    QPoint offset = (geo.topLeft() - total.topLeft()) * dpr;
    QPixmap grab = screen->grabWindow(0);
    grab.setDevicePixelRatio(1.0);
    p.drawPixmap(offset, grab);
  }
  p.end();
}

// ---- 颜色/笔粗/字号 ----

QColor ScreenShotWindow::annotationColor(int index) const {
  static const QColor colors[kColorCount] = {
      QColor(239, 68, 68),   // 红
      QColor(245, 158, 11),  // 橙黄
      QColor(16, 185, 129),  // 绿
      QColor(59, 130, 246),  // 蓝
      QColor(255, 255, 255)  // 白
  };
  return colors[qBound(0, index, kColorCount - 1)];
}

int ScreenShotWindow::penWidthForIndex(int index) const {
  static const int widths[kPenCount] = {2, 3, 5, 8};
  return widths[qBound(0, index, kPenCount - 1)];
}

int ScreenShotWindow::fontSizeForIndex(int index) const {
  static const int sizes[kSizeCount] = {12, 16, 22};
  return sizes[qBound(0, index, kSizeCount - 1)];
}

// ---- 手柄几何 ----

QRect ScreenShotWindow::handleRect(HandlePos pos) const {
  QPoint pt;
  switch (pos) {
    case TopLeft: pt = m_selection.topLeft(); break;
    case Top: pt = QPoint(m_selection.center().x(), m_selection.top()); break;
    case TopRight: pt = m_selection.topRight(); break;
    case Right: pt = QPoint(m_selection.right(), m_selection.center().y()); break;
    case BottomRight: pt = m_selection.bottomRight(); break;
    case Bottom: pt = QPoint(m_selection.center().x(), m_selection.bottom()); break;
    case BottomLeft: pt = m_selection.bottomLeft(); break;
    case Left: pt = QPoint(m_selection.left(), m_selection.center().y()); break;
  }
  int hs = kHandleSize / 2;
  return QRect(pt.x() - hs, pt.y() - hs, kHandleSize, kHandleSize);
}

ScreenShotWindow::HandlePos ScreenShotWindow::handleAt(QPoint globalPos) const {
  QPoint local = mapFromGlobal(globalPos);
  for (int i = TopLeft; i <= Left; ++i) {
    if (handleRect(static_cast<HandlePos>(i)).contains(local)) {
      return static_cast<HandlePos>(i);
    }
  }
  return static_cast<HandlePos>(-1);
}

void ScreenShotWindow::setCursorForPos(QPoint globalPos) {
  if (m_phase != Confirm) return;
  HandlePos h = handleAt(globalPos);
  switch (h) {
    case TopLeft: case BottomRight: setCursor(Qt::SizeFDiagCursor); break;
    case TopRight: case BottomLeft: setCursor(Qt::SizeBDiagCursor); break;
    case Top: case Bottom: setCursor(Qt::SizeVerCursor); break;
    case Left: case Right: setCursor(Qt::SizeHorCursor); break;
    default:
      setCursor(Qt::ArrowCursor);
      break;
  }
}

// ---- 工具条几何 ----
//
// 布局：[矩形][椭圆][箭头][画笔][马赛克][文字] | [撤销][置顶] | [✦?] [✗] [✓]

// 标注工具按钮数量（不含 Mosaic/None，但含 Mosaic 按钮本身——它在工具条上只是不弹窗）
static constexpr int kToolBtnCount = 6;  // Rect, Ellipse, Arrow, Pen, Mosaic, Text

QRect ScreenShotWindow::toolbarRect() const {
  int toolW = kToolBtnCount * kBtnW + kGap * (kToolBtnCount - 1);
  int undoPinW = 2 * kBtnW + kGap;  // 撤销+置顶
  int actionCount = m_showRecognizeBtn ? 3 : 2;
  int actionW = actionCount * kActionBtnW + kGap * (actionCount - 1);

  int w = kPadding * 2 + toolW + kSepGap + undoPinW + kSepGap + actionW;
  int h = kBtnH + kPadding * 2;
  int x = m_selection.right() - w + kPadding;
  int y = m_selection.bottom() + 8;

  QRect total = geometry();
  if (y + h > total.bottom()) y = m_selection.top() - h - 8;
  if (x < total.left()) x = total.left() + 4;
  if (x + w > total.right()) x = total.right() - w - 4;
  return QRect(x, y, w, h);
}

QRect ScreenShotWindow::toolBtnRect(Tool tool) const {
  QRect tb = toolbarRect();
  int x = tb.left() + kPadding;
  int idx = 0;
  switch (tool) {
    case Rect: idx = 0; break;
    case Ellipse: idx = 1; break;
    case Arrow: idx = 2; break;
    case Pen: idx = 3; break;
    case Mosaic: idx = 4; break;
    case Text: idx = 5; break;
    default: idx = 0; break;
  }
  x += idx * (kBtnW + kGap);
  return QRect(x, tb.top() + kPadding, kBtnW, kBtnH);
}

QRect ScreenShotWindow::undoBtnRect() const {
  QRect tb = toolbarRect();
  int x = tb.left() + kPadding + kToolBtnCount * (kBtnW + kGap) - kGap + kSepGap;
  return QRect(x, tb.top() + kPadding, kBtnW, kBtnH);
}

QRect ScreenShotWindow::pinBtnRect() const {
  int x = undoBtnRect().right() + kGap;
  return QRect(x, toolbarRect().top() + kPadding, kBtnW, kBtnH);
}

QRect ScreenShotWindow::recognizeBtnRect() const {
  int x = pinBtnRect().right() + kSepGap;
  return QRect(x, toolbarRect().top() + kPadding, kActionBtnW, kBtnH);
}

QRect ScreenShotWindow::cancelBtnRect() const {
  int x;
  if (m_showRecognizeBtn) {
    x = recognizeBtnRect().right() + kGap;
  } else {
    x = pinBtnRect().right() + kSepGap;
  }
  return QRect(x, toolbarRect().top() + kPadding, kActionBtnW, kBtnH);
}

QRect ScreenShotWindow::confirmBtnRect() const {
  int x = cancelBtnRect().right() + kGap;
  return QRect(x, toolbarRect().top() + kPadding, kActionBtnW, kBtnH);
}

// ---- 弹窗几何 ----

// 弹窗内容宽度：颜色组(5) + 分隔 + 第二组(笔粗4 或 字号3)
// 用 int 参数而非 Tool（Tool 是私有嵌套枚举，自由函数无法访问）
static int popupContentWidth(int toolType) {
  int colorW = kColorCount * kPopupDotSize + kGap * (kColorCount - 1);
  int vsepW = 1 + kSepGap;
  int secondW = 0;
  // toolType == 6 是 Text（枚举值），其余为形状/画笔
  if (toolType == 6) {
    secondW = kSizeCount * kPopupSizeW + kGap * (kSizeCount - 1);
  } else {
    secondW = kPenCount * kPopupPenBtn + kGap * (kPenCount - 1);
  }
  return colorW + vsepW + secondW + kPopupPadding * 2;
}

QRect ScreenShotWindow::popupRect() const {
  if (m_currentTool == None || m_currentTool == Mosaic || !m_popupVisible)
    return QRect();
  QRect btn = toolBtnRect(m_currentTool);
  int w = popupContentWidth(m_currentTool);
  int h = kPopupDotSize + kPopupPadding * 2;
  int x = btn.center().x() - w / 2;
  int y = btn.bottom() + 6;
  QRect total = geometry();
  if (x < total.left() + 4) x = total.left() + 4;
  if (x + w > total.right() - 4) x = total.right() - w - 4;
  return QRect(x, y, w, h);
}

QRect ScreenShotWindow::popupColorDotRect(int index) const {
  QRect pp = popupRect();
  int x = pp.left() + kPopupPadding;
  int y = pp.top() + kPopupPadding;
  x += index * (kPopupDotSize + kGap);
  return QRect(x, y, kPopupDotSize, kPopupDotSize);
}

QRect ScreenShotWindow::popupVsepRect() const {
  QRect pp = popupRect();
  int x = pp.left() + kPopupPadding +
          kColorCount * kPopupDotSize + kGap * (kColorCount - 1) + kSepGap / 2;
  return QRect(x, pp.top() + kPopupPadding - 2, 1, kPopupDotSize + 4);
}

QRect ScreenShotWindow::popupPenRect(int index) const {
  QRect pp = popupRect();
  int sepRight = popupVsepRect().right() + kSepGap / 2;
  int x = sepRight + index * (kPopupPenBtn + kGap);
  int y = pp.top() + kPopupPadding - (kPopupPenBtn - kPopupDotSize) / 2;
  return QRect(x, y, kPopupPenBtn, kPopupPenBtn);
}

QRect ScreenShotWindow::popupSizeRect(int index) const {
  QRect pp = popupRect();
  int sepRight = popupVsepRect().right() + kSepGap / 2;
  int x = sepRight + index * (kPopupSizeW + kGap);
  int y = pp.top() + kPopupPadding - (kBtnH - kPopupDotSize) / 2;
  return QRect(x, y, kPopupSizeW, kBtnH);
}

// ---- 窗口预选 ----

void ScreenShotWindow::detectHoverWindow(QPoint globalPos) {
  HWND selfHwnd = reinterpret_cast<HWND>(winId());
  POINT pt = {globalPos.x(), globalPos.y()};
  HWND hwnd = WindowFromPoint(pt);
  if (!hwnd) { m_hoverValid = false; return; }
  HWND root = GetAncestor(hwnd, kGaRoot);
  if (!root || root == selfHwnd) { m_hoverValid = false; return; }
  if (!isHighlightableWindow(root, selfHwnd)) { m_hoverValid = false; return; }
  RECT rc;
  if (!GetWindowRect(root, &rc)) { m_hoverValid = false; return; }
  m_hoverRect = QRect(QPoint(rc.left, rc.top), QPoint(rc.right - 1, rc.bottom - 1));
  m_hoverRect = m_hoverRect.intersected(geometry());
  if (m_hoverRect.width() < 10 || m_hoverRect.height() < 10) {
    m_hoverValid = false;
    return;
  }
  WCHAR title[256] = {0};
  GetWindowTextW(root, title, 256);
  m_hoverTitle = QString::fromWCharArray(title);
  m_hoverValid = true;
}

void ScreenShotWindow::drawHoverHighlight(QPainter &p, QPoint origin) const {
  if (!m_hoverValid) return;
  QRect local = m_hoverRect.translated(-origin);
  p.fillRect(local, QColor(59, 130, 246, 30));
  QPen pen(kColorBorder, 2);
  p.setPen(pen);
  p.setBrush(Qt::NoBrush);
  p.drawRect(local);
  // 四角 L 标记
  int cs = 12;
  pen.setWidth(3);
  p.setPen(pen);
  p.drawLine(local.topLeft(), local.topLeft() + QPoint(cs, 0));
  p.drawLine(local.topLeft(), local.topLeft() + QPoint(0, cs));
  p.drawLine(local.topRight(), local.topRight() + QPoint(-cs, 0));
  p.drawLine(local.topRight(), local.topRight() + QPoint(0, cs));
  p.drawLine(local.bottomLeft(), local.bottomLeft() + QPoint(cs, 0));
  p.drawLine(local.bottomLeft(), local.bottomLeft() + QPoint(0, -cs));
  p.drawLine(local.bottomRight(), local.bottomRight() + QPoint(-cs, 0));
  p.drawLine(local.bottomRight(), local.bottomRight() + QPoint(0, -cs));
  // 标题 tooltip
  if (!m_hoverTitle.isEmpty()) {
    QFont f = p.font();
    f.setPixelSize(12);
    p.setFont(f);
    QFontMetrics fm(f);
    int tw = fm.horizontalAdvance(m_hoverTitle);
    int th = fm.height();
    int tx = local.left();
    int ty = local.top() - th - 6;
    if (ty < 0) ty = local.bottom() + 4;
    QRect tip(tx, ty, tw + 12, th + 4);
    QPainterPath tp;
    tp.addRoundedRect(tip, 3, 3);
    p.fillPath(tp, QColor(0, 0, 0, 200));
    p.setPen(Qt::white);
    p.drawText(tip, Qt::AlignCenter, m_hoverTitle);
  }
}

// ---- 标注绘制 ----

void ScreenShotWindow::drawAnnotation(QPainter &p, const Annotation &a,
                                       QPoint selOrigin) const {
  if (a.points.isEmpty()) return;
  QPen pen(a.color);
  pen.setWidth(a.penWidth);
  pen.setCapStyle(Qt::RoundCap);
  pen.setJoinStyle(Qt::RoundJoin);
  p.setPen(pen);
  p.setBrush(Qt::NoBrush);

  switch (a.tool) {
    case Rect: {
      if (a.points.size() < 2) break;
      QPoint s = a.points[0] + selOrigin;
      QPoint e = a.points[1] + selOrigin;
      p.drawRect(QRect(s, e).normalized());
      break;
    }
    case Ellipse: {
      if (a.points.size() < 2) break;
      QPoint s = a.points[0] + selOrigin;
      QPoint e = a.points[1] + selOrigin;
      p.drawEllipse(QRect(s, e).normalized());
      break;
    }
    case Arrow: {
      if (a.points.size() < 2) break;
      QPoint s = a.points[0] + selOrigin;
      QPoint e = a.points[1] + selOrigin;
      p.drawLine(s, e);
      double angle = std::atan2(e.y() - s.y(), e.x() - s.x());
      int al = 12 + a.penWidth * 2;
      double a1 = angle + M_PI * 0.8;
      double a2 = angle - M_PI * 0.8;
      p.drawLine(e, e + QPoint(static_cast<int>(al * std::cos(a1)),
                               static_cast<int>(al * std::sin(a1))));
      p.drawLine(e, e + QPoint(static_cast<int>(al * std::cos(a2)),
                               static_cast<int>(al * std::sin(a2))));
      break;
    }
    case Pen: {
      QPolygon poly;
      for (const QPoint &pt : a.points) poly << pt + selOrigin;
      p.drawPolyline(poly);
      break;
    }
    case Text: {
      if (a.text.isEmpty() || a.points.isEmpty()) break;
      QFont f = p.font();
      f.setPixelSize(a.fontSize);
      f.setBold(true);
      p.setFont(f);
      QPoint pos = a.points[0] + selOrigin;
      // 描边增强可读性
      QPainterPath textPath;
      textPath.addText(pos + QPoint(1, a.fontSize), f, a.text);
      p.strokePath(textPath, QPen(Qt::black, 3));
      p.fillPath(textPath, QBrush(a.color));
      break;
    }
    case Mosaic:
      break;  // drawMosaic 单独处理
    default:
      break;
  }
}

void ScreenShotWindow::drawMosaic(QPainter &p, const Annotation &a,
                                   const QPixmap &srcPixmap,
                                   QPoint selOrigin, qreal dpr) const {
  if (a.points.isEmpty()) return;
  QPoint offset = a.points.first() + selOrigin;
  QPoint minPt = offset, maxPt = offset;
  for (const QPoint &pt : a.points) {
    QPoint w = pt + selOrigin;
    minPt.setX(std::min(minPt.x(), w.x()));
    minPt.setY(std::min(minPt.y(), w.y()));
    maxPt.setX(std::max(maxPt.x(), w.x()));
    maxPt.setY(std::max(maxPt.y(), w.y()));
  }
  QRect widgetRect(minPt, maxPt);
  if (widgetRect.width() < 2 || widgetRect.height() < 2) return;

  QPoint pixMin = minPt * dpr;
  QPoint pixMax = maxPt * dpr;
  QRect pixRect(pixMin, pixMax);
  pixRect = pixRect.intersected(QRect(0, 0, srcPixmap.width(), srcPixmap.height()));
  if (pixRect.isEmpty()) return;

  QImage srcImg = srcPixmap.copy(pixRect).toImage();
  int blockSize = 8;
  for (int by = 0; by < srcImg.height(); by += blockSize) {
    for (int bx = 0; bx < srcImg.width(); bx += blockSize) {
      int bw = std::min(blockSize, srcImg.width() - bx);
      int bh = std::min(blockSize, srcImg.height() - by);
      int r = 0, g = 0, b = 0, cnt = 0;
      for (int y = by; y < by + bh; ++y) {
        for (int x = bx; x < bx + bw; ++x) {
          QRgb c = srcImg.pixel(x, y);
          r += qRed(c); g += qGreen(c); b += qBlue(c);
          ++cnt;
        }
      }
      if (cnt > 0) {
        QRgb avg = qRgb(r / cnt, g / cnt, b / cnt);
        for (int y = by; y < by + bh; ++y)
          for (int x = bx; x < bx + bw; ++x)
            srcImg.setPixel(x, y, avg);
      }
    }
  }
  QPixmap mosPix = QPixmap::fromImage(srcImg);
  mosPix.setDevicePixelRatio(dpr);
  p.drawPixmap(widgetRect.topLeft(), mosPix);
}

void ScreenShotWindow::paintAnnotationsOnPixmap(QPixmap &pixmap,
                                                 qreal dpr) const {
  if (m_annotations.isEmpty()) return;
  QPainter p(&pixmap);
  p.setRenderHint(QPainter::Antialiasing);
  for (const Annotation &a : m_annotations) {
    if (a.tool == Mosaic) {
      if (a.points.isEmpty()) continue;
      QPoint offset = a.points.first();
      QPoint minPt = offset, maxPt = offset;
      for (const QPoint &pt : a.points) {
        minPt.setX(std::min(minPt.x(), pt.x()));
        minPt.setY(std::min(minPt.y(), pt.y()));
        maxPt.setX(std::max(maxPt.x(), pt.x()));
        maxPt.setY(std::max(maxPt.y(), pt.y()));
      }
      QRect pixRect(minPt * dpr, maxPt * dpr);
      pixRect = pixRect.intersected(QRect(0, 0, pixmap.width(), pixmap.height()));
      if (pixRect.isEmpty()) continue;
      QImage img = pixmap.copy(pixRect).toImage();
      int blockSize = 8;
      for (int by = 0; by < img.height(); by += blockSize) {
        for (int bx = 0; bx < img.width(); bx += blockSize) {
          int bw = std::min(blockSize, img.width() - bx);
          int bh = std::min(blockSize, img.height() - by);
          int r = 0, g = 0, b = 0, cnt = 0;
          for (int y = by; y < by + bh; ++y) {
            for (int x = bx; x < bx + bw; ++x) {
              QRgb c = img.pixel(x, y);
              r += qRed(c); g += qGreen(c); b += qBlue(c);
              ++cnt;
            }
          }
          if (cnt > 0) {
            QRgb avg = qRgb(r / cnt, g / cnt, b / cnt);
            for (int y = by; y < by + bh; ++y)
              for (int x = bx; x < bx + bw; ++x)
                img.setPixel(x, y, avg);
          }
        }
      }
      p.drawImage(pixRect, img);
    } else if (a.tool == Text) {
      Annotation scaled = a;
      scaled.penWidth = static_cast<int>(a.penWidth * dpr);
      scaled.fontSize = static_cast<int>(a.fontSize * dpr);
      drawAnnotation(p, scaled, QPoint(0, 0));
    } else {
      Annotation scaled = a;
      QList<QPoint> sp;
      for (const QPoint &pt : a.points)
        sp << QPoint(static_cast<int>(pt.x() * dpr),
                     static_cast<int>(pt.y() * dpr));
      scaled.points = sp;
      scaled.penWidth = static_cast<int>(a.penWidth * dpr);
      drawAnnotation(p, scaled, QPoint(0, 0));
    }
  }
  p.end();
}

// ---- 工具图标 ----
// 从 qrc 加载 SVG（与 HTML 设计稿完全一致），渲染前替换 currentColor 为实际颜色。

static void renderSvgIcon(QPainter &p, const QString &resPath,
                          const QRect &rect, const QColor &color) {
  QFile f(resPath);
  if (!f.open(QIODevice::ReadOnly)) {
    p.fillRect(rect, Qt::red);
    return;
  }
  QByteArray svgData = f.readAll();
  f.close();
  // SVG viewBox 是 0 0 24 24（正方形），渲染时保持宽高比，居中绘制。
  QString colorHex = "#" + QString("%1%2%3")
      .arg(color.red(), 2, 16, QChar('0'))
      .arg(color.green(), 2, 16, QChar('0'))
      .arg(color.blue(), 2, 16, QChar('0'));
  svgData.replace("currentColor", colorHex.toUtf8());
  QSvgRenderer renderer(svgData);
  if (renderer.isValid()) {
    // 保持 1:1 宽高比，取 rect 较短边作为图标尺寸，居中
    int size = qMin(rect.width(), rect.height());
    int x = rect.left() + (rect.width() - size) / 2;
    int y = rect.top() + (rect.height() - size) / 2;
    renderer.render(&p, QRectF(x, y, size, size));
  } else {
    p.fillRect(rect, Qt::yellow);
  }
}

void ScreenShotWindow::drawToolIcon(QPainter &p, Tool tool, const QRect &rect,
                                     const QColor &color) const {
  p.save();
  p.setRenderHint(QPainter::Antialiasing);
  QString path;
  switch (tool) {
    case Rect: path = ":/qml/icons/screenshot/rect.svg"; break;
    case Ellipse: path = ":/qml/icons/screenshot/ellipse.svg"; break;
    case Arrow: path = ":/qml/icons/screenshot/arrow.svg"; break;
    case Pen: path = ":/qml/icons/screenshot/pen.svg"; break;
    case Mosaic: path = ":/qml/icons/screenshot/mosaic.svg"; break;
    case Text: path = ":/qml/icons/screenshot/text.svg"; break;
    default: path = ":/qml/icons/screenshot/undo.svg"; break;  // None = 撤销
  }
  if (!path.isEmpty()) renderSvgIcon(p, path, rect, color);
  p.restore();
}

// 置顶（图钉）图标
static void drawPinIcon(QPainter &p, const QRect &rect, const QColor &color) {
  p.save();
  p.setRenderHint(QPainter::Antialiasing);
  renderSvgIcon(p, ":/qml/icons/screenshot/pin.svg", rect, color);
  p.restore();
}

// ---- 选区视觉 ----

void ScreenShotWindow::drawSelectionBorder(QPainter &p,
                                            QPoint localTopLeft) const {
  QRect sel(localTopLeft, m_selection.size());
  // 2px 蓝色实线边框
  QPen pen(kColorBorder, 2);
  p.setPen(pen);
  p.setBrush(Qt::NoBrush);
  p.drawRect(sel);

  // 四角 L 标记
  int cs = 14;
  pen.setWidth(3);
  p.setPen(pen);
  p.drawLine(sel.topLeft(), sel.topLeft() + QPoint(cs, 0));
  p.drawLine(sel.topLeft(), sel.topLeft() + QPoint(0, cs));
  p.drawLine(sel.topRight(), sel.topRight() + QPoint(-cs, 0));
  p.drawLine(sel.topRight(), sel.topRight() + QPoint(0, cs));
  p.drawLine(sel.bottomLeft(), sel.bottomLeft() + QPoint(cs, 0));
  p.drawLine(sel.bottomLeft(), sel.bottomLeft() + QPoint(0, -cs));
  p.drawLine(sel.bottomRight(), sel.bottomRight() + QPoint(-cs, 0));
  p.drawLine(sel.bottomRight(), sel.bottomRight() + QPoint(0, -cs));

  // 尺寸标签
  QString sizeText = QString("%1 × %2")
                         .arg(m_selection.width())
                         .arg(m_selection.height());
  QFont f = p.font();
  f.setPixelSize(11);
  f.setBold(true);
  p.setFont(f);
  QFontMetrics fm(f);
  int tw = fm.horizontalAdvance(sizeText);
  int th = fm.height();
  QRect tagRect(sel.left(), sel.top() - th - 6, tw + 12, th + 2);
  if (tagRect.top() < 0) tagRect.moveTop(sel.bottom() + 4);
  QPainterPath tp;
  tp.addRoundedRect(tagRect, 3, 3);
  p.fillPath(tp, kColorBorder);
  p.setPen(Qt::white);
  p.drawText(tagRect, Qt::AlignCenter, sizeText);
}

// ---- paintEvent ----

void ScreenShotWindow::paintEvent(QPaintEvent *) {
  QPainter p(this);
  p.setRenderHint(QPainter::Antialiasing);
  QPoint origin = geometry().topLeft();

  // 方案：全屏底图 → 全屏遮罩 → 选区内重画底图覆盖遮罩。
  // 不能用 DestinationOut 挖洞——那会把底图也擦掉，露出透明窗口背后的真实桌面。
  //
  // 防抖动核心：m_fullPixmap 设了 devicePixelRatio 后，QPainter 绘制时会自动按
  // dpr 做物理↔逻辑坐标映射，不产生插值缩放。用 setClipRect 限定选区绘制范围，
  // 避免每次选区变化时 drawPixmap 的 source/target 坐标重新计算导致抖动。
  qreal dpr = devicePixelRatioF();
  QRect total = geometry();

  // 确保 m_fullPixmap 的 dpr 已设（initFullscreenPixmap 中未设，这里补设）
  // 不设的话 drawPixmap 会做缩放→抖动。
  if (m_fullPixmap.devicePixelRatio() != dpr) {
    m_fullPixmap.setDevicePixelRatio(dpr);
  }

  // 1. 画全屏底图（pixmap 有 dpr，Qt 自动 1:1 映射不缩放）
  p.drawPixmap(QPoint(0, 0), m_fullPixmap);

  // 2. 全屏半透明遮罩
  p.fillRect(rect(), QColor(0, 0, 0, 115));

  // 3. 选区内重画底图（覆盖遮罩）
  // 用 setClipRect 限定绘制范围，drawPixmap 只在选区内有效——坐标计算一次，
  // 不随选区尺寸变化重新缩放。
  if (!m_selection.isEmpty()) {
    QPoint localTopLeft = m_selection.topLeft() - origin;
    QRect selLocal(localTopLeft, m_selection.size());
    p.save();
    p.setClipRect(selLocal);
    p.drawPixmap(QPoint(0, 0), m_fullPixmap);
    p.restore();
  }

  // Drag 阶段：窗口预选高亮
  if (m_phase == Drag && m_hoverValid && !m_dragging) {
    drawHoverHighlight(p, origin);
  }

  if (m_selection.isEmpty()) return;

  // 绘制标注（选区内）
  QPoint selOrigin = m_selection.topLeft() - origin;
  for (const Annotation &a : m_annotations) {
    if (a.tool == Mosaic)
      drawMosaic(p, a, m_fullPixmap, selOrigin, dpr);
    else
      drawAnnotation(p, a, selOrigin);
  }
  if (m_drawingActive) {
    if (m_drawingAnno.tool == Mosaic)
      drawMosaic(p, m_drawingAnno, m_fullPixmap, selOrigin, dpr);
    else
      drawAnnotation(p, m_drawingAnno, selOrigin);
  }

  // 选区边框 + 四角 + 尺寸标签
  drawSelectionBorder(p, selOrigin);

  // Confirm 阶段：手柄 + 工具条
  if (m_phase == Confirm) {
    // 手柄（仅无工具选中且未在绘制时）
    if (m_currentTool == None && !m_drawingActive && !m_textInput) {
      p.setBrush(Qt::white);
      p.setPen(Qt::NoPen);
      for (int i = TopLeft; i <= Left; ++i) {
        QRect h = handleRect(static_cast<HandlePos>(i));
        h.translate(-origin);
        p.drawRect(h);
      }
    }

    // ---- 工具条 ----
    QRect tb = toolbarRect();
    tb.translate(-origin);
    QPainterPath tbPath;
    tbPath.addRoundedRect(tb.x(), tb.y(), tb.width(), tb.height(), 6, 6);
    p.fillPath(tbPath, kColorToolbarBg);

    // 标注工具按钮
    auto drawToolBtn = [&](Tool tool, int btnIdx) {
      QRect r = toolBtnRect(tool);
      r.translate(-origin);
      if (m_currentTool == tool) {
        QPainterPath rp;
        rp.addRoundedRect(r.adjusted(1,1,-1,-1), 4, 4);
        p.fillPath(rp, kColorActive);
      } else if (m_hoveredBtn == btnIdx) {
        QPainterPath rp;
        rp.addRoundedRect(r.adjusted(1,1,-1,-1), 4, 4);
        p.fillPath(rp, QColor(255,255,255,20));
      }
      QColor ic = (m_currentTool == tool) ? Qt::white : QColor(210,210,210);
      drawToolIcon(p, tool, r, ic);
    };
    drawToolBtn(Rect, 0);
    drawToolBtn(Ellipse, 1);
    drawToolBtn(Arrow, 2);
    drawToolBtn(Pen, 3);
    drawToolBtn(Mosaic, 4);
    drawToolBtn(Text, 5);

    // 撤销按钮
    {
      QRect r = undoBtnRect().translated(-origin);
      bool canUndo = !m_annotations.isEmpty();
      if (canUndo && m_hoveredBtn == 6) {
        QPainterPath rp;
        rp.addRoundedRect(r.adjusted(1,1,-1,-1), 4, 4);
        p.fillPath(rp, QColor(255,255,255,20));
      }
      drawToolIcon(p, None, r, canUndo ? QColor(210,210,210) : QColor(100,100,100));
    }
    // 置顶按钮
    {
      QRect r = pinBtnRect().translated(-origin);
      if (m_hoveredBtn == 7) {
        QPainterPath rp;
        rp.addRoundedRect(r.adjusted(1,1,-1,-1), 4, 4);
        p.fillPath(rp, QColor(255,255,255,20));
      }
      drawPinIcon(p, r, QColor(210,210,210));
    }

    // 识别
    if (m_showRecognizeBtn) {
      QRect r = recognizeBtnRect().translated(-origin);
      QPainterPath rp;
      rp.addRoundedRect(r, 4, 4);
      p.fillPath(rp, kColorVlm);
      QFont f = p.font(); f.setPixelSize(13); p.setFont(f);
      p.setPen(Qt::white);
      p.drawText(r, Qt::AlignCenter, QString::fromUtf8("\u2726 \u8BC6\u522B"));
    }
    // 取消
    {
      QRect r = cancelBtnRect().translated(-origin);
      QPainterPath rp; rp.addRoundedRect(r, 4, 4);
      p.fillPath(rp, kColorCancel);
      QFont f = p.font(); f.setPixelSize(13); p.setFont(f);
      p.setPen(Qt::white);
      p.drawText(r, Qt::AlignCenter, QString::fromUtf8("\u2717 \u53D6\u6D88"));
    }
    // 完成
    {
      QRect r = confirmBtnRect().translated(-origin);
      QPainterPath rp; rp.addRoundedRect(r, 4, 4);
      p.fillPath(rp, kColorConfirm);
      QFont f = p.font(); f.setPixelSize(13); p.setFont(f);
      p.setPen(Qt::white);
      p.drawText(r, Qt::AlignCenter, QString::fromUtf8("\u2713 \u5B8C\u6210"));
    }

    // ---- 弹窗（在选中工具下方）----
    if (m_popupVisible && m_currentTool != None && m_currentTool != Mosaic) {
      QRect pp = popupRect().translated(-origin);
      QPainterPath ppPath;
      ppPath.addRoundedRect(pp, 6, 6);
      p.fillPath(ppPath, kColorToolbarBg);

      // 颜色圆点
      for (int i = 0; i < kColorCount; ++i) {
        QRect cr = popupColorDotRect(i).translated(-origin);
        QPainterPath cp; cp.addEllipse(cr);
        p.fillPath(cp, annotationColor(i));
        if (annotationColor(i) == m_drawColor) {
          QPen wp(Qt::white, 2); p.setPen(wp); p.setBrush(Qt::NoBrush);
          p.drawEllipse(cr.adjusted(-2,-2,2,2));
          p.setPen(Qt::NoPen);
        }
      }
      // 竖分隔
      {
        QRect vs = popupVsepRect().translated(-origin);
        p.fillRect(vs, QColor(58,58,66));
      }
      // 第二组：笔粗 or 字号
      if (m_currentTool == Text) {
        const QString labels[kSizeCount] = {QString::fromUtf8("\u5C0F"),
                                             QString::fromUtf8("\u4E2D"),
                                             QString::fromUtf8("\u5927")};
        const int sizes[kSizeCount] = {10, 12, 15};
        for (int i = 0; i < kSizeCount; ++i) {
          QRect sr = popupSizeRect(i).translated(-origin);
          bool act = (fontSizeForIndex(i) == m_textFontSize);
          if (act) {
            QPainterPath sp; sp.addRoundedRect(sr, 4, 4);
            p.fillPath(sp, QColor(59,130,246,60));
          }
          QFont f = p.font();
          f.setPixelSize(sizes[i]);
          f.setBold(act);
          p.setFont(f);
          p.setPen(act ? QColor(96,165,250) : QColor(192,192,200));
          p.drawText(sr, Qt::AlignCenter, labels[i]);
        }
      } else {
        const int dotSizes[kPenCount] = {4, 6, 8, 10};
        for (int i = 0; i < kPenCount; ++i) {
          QRect pr = popupPenRect(i).translated(-origin);
          bool act = (penWidthForIndex(i) == m_penWidth);
          if (act) {
            QPainterPath pp2; pp2.addRoundedRect(pr, 4, 4);
            p.fillPath(pp2, QColor(59,130,246,60));
          }
          // 中心画圆点
          int ds = dotSizes[i];
          QPoint c = pr.center();
          QPainterPath dp;
          dp.addEllipse(QRect(c.x()-ds/2, c.y()-ds/2, ds, ds));
          p.fillPath(dp, act ? QColor(96,165,250) : QColor(192,192,200));
        }
      }
    }
  }
}

// ---- 落盘 ----

bool ScreenShotWindow::saveSelection(QString &outPath) {
  QDir().mkpath(m_saveDir);
  QString name = "screenshot_" +
                 QDateTime::currentDateTime().toString("yyyyMMdd_HHmmss") +
                 ".png";
  QString filePath = m_saveDir + "/" + name;

  QRect total = geometry();
  qreal dpr = devicePixelRatioF();
  QPoint offset = (m_selection.topLeft() - total.topLeft()) * dpr;
  QSize size = m_selection.size() * dpr;
  QPixmap crop = m_fullPixmap.copy(QRect(offset, size));

  if (!m_annotations.isEmpty()) {
    paintAnnotationsOnPixmap(crop, dpr);
  }

  if (!crop.save(filePath, "PNG")) return false;

  QClipboard *cb = QApplication::clipboard();
  cb->setPixmap(crop);

  outPath = filePath;
  return true;
}

// ---- 文字输入 ----

void ScreenShotWindow::startTextInput(QPoint globalPos) {
  if (m_textInput) commitTextInput();
  m_textPos = globalPos - m_selection.topLeft();
  m_textInput = new QLineEdit(this);
  m_textInput->setStyleSheet(
      "QLineEdit { background: rgba(255,255,255,0.9); border: 1px solid #3b82f6; "
      "border-radius: 3px; padding: 2px 4px; }");
  QFont f;
  f.setPixelSize(m_textFontSize);
  f.setBold(true);
  m_textInput->setFont(f);
  // QLineEdit 没有 setColor，文字色用 QPalette
  QPalette pal = m_textInput->palette();
  pal.setColor(QPalette::Text, m_drawColor);
  m_textInput->setPalette(pal);

  // 定位（全局→widget 本地）
  QPoint local = globalPos - geometry().topLeft();
  m_textInput->setGeometry(local.x(), local.y(), 200, m_textFontSize + 8);
  m_textInput->show();
  m_textInput->setFocus();
  m_textInput->activateWindow();
  m_textInput->grabKeyboard();

  connect(m_textInput, &QLineEdit::returnPressed, this, &ScreenShotWindow::commitTextInput);
  connect(m_textInput, &QLineEdit::editingFinished, this, &ScreenShotWindow::commitTextInput);
}

void ScreenShotWindow::commitTextInput() {
  if (!m_textInput) return;
  QString text = m_textInput->text().trimmed();
  m_textInput->releaseKeyboard();
  delete m_textInput;
  m_textInput = nullptr;
  if (!text.isEmpty()) {
    Annotation a;
    a.tool = Text;
    a.color = m_drawColor;
    a.fontSize = m_textFontSize;
    a.points << m_textPos;
    a.text = text;
    m_annotations.append(a);
  }
  update();
}

void ScreenShotWindow::focusOutEvent(QFocusEvent *event) {
  if (m_textInput) commitTextInput();
  QWidget::focusOutEvent(event);
}

// ---- 弹窗点击处理 ----

void ScreenShotWindow::handlePopupClick(QPoint lpos) {
  if (!m_popupVisible || m_currentTool == None || m_currentTool == Mosaic) return;
  // 颜色
  for (int i = 0; i < kColorCount; ++i) {
    if (popupColorDotRect(i).contains(lpos)) {
      m_drawColor = annotationColor(i);
      update();
      return;
    }
  }
  // 笔粗 / 字号
  if (m_currentTool == Text) {
    for (int i = 0; i < kSizeCount; ++i) {
      if (popupSizeRect(i).contains(lpos)) {
        m_textFontSize = fontSizeForIndex(i);
        update();
        return;
      }
    }
  } else {
    for (int i = 0; i < kPenCount; ++i) {
      if (popupPenRect(i).contains(lpos)) {
        m_penWidth = penWidthForIndex(i);
        update();
        return;
      }
    }
  }
}

// ---- 鼠标事件 ----

void ScreenShotWindow::mousePressEvent(QMouseEvent *event) {
  QPoint gpos = event->globalPosition().toPoint();
  QPoint lpos = event->position().toPoint();

  if (event->button() == Qt::RightButton) {
    emit cancelled();
    close();
    return;
  }
  if (event->button() != Qt::LeftButton) return;

  // 文字输入中：点击任意位置先提交
  if (m_textInput) {
    commitTextInput();
    return;
  }

  QPoint origin = geometry().topLeft();

  if (m_phase == Drag) {
    m_startPoint = gpos;
    m_selection = QRect();
    m_dragging = true;
    m_hoverValid = false;
  } else if (m_phase == Confirm) {
    // 弹窗命中优先
    if (m_popupVisible) {
      QRect pp = popupRect();
      if (pp.contains(lpos)) {
        handlePopupClick(lpos);
        return;
      }
    }

    // 工具按钮
    auto tryTool = [&](Tool t) -> bool {
      if (toolBtnRect(t).translated(-origin).contains(lpos)) {
        if (m_currentTool == t) {
          // 再次点击 = 取消
          m_currentTool = None;
          m_popupVisible = false;
        } else {
          m_currentTool = t;
          m_popupVisible = (t != Mosaic);
        }
        update();
        return true;
      }
      return false;
    };
    if (tryTool(Rect)) return;
    if (tryTool(Ellipse)) return;
    if (tryTool(Arrow)) return;
    if (tryTool(Pen)) return;
    if (tryTool(Mosaic)) return;
    if (tryTool(Text)) return;

    // 撤销
    if (!m_annotations.isEmpty() &&
        undoBtnRect().translated(-origin).contains(lpos)) {
      m_annotations.removeLast();
      update();
      return;
    }
    // 置顶
    if (pinBtnRect().translated(-origin).contains(lpos)) {
      QString filePath;
      if (saveSelection(filePath)) emit pinned(filePath);
      close();
      return;
    }
    // 识别
    if (m_showRecognizeBtn &&
        recognizeBtnRect().translated(-origin).contains(lpos)) {
      QString filePath;
      if (saveSelection(filePath)) emit recognized(filePath);
      close();
      return;
    }
    // 取消
    if (cancelBtnRect().translated(-origin).contains(lpos)) {
      emit cancelled();
      close();
      return;
    }
    // 完成
    if (confirmBtnRect().translated(-origin).contains(lpos)) {
      QString filePath;
      if (saveSelection(filePath)) emit finished(filePath);
      close();
      return;
    }

    // 有工具选中 → 画标注 / 文字
    if (m_currentTool != None && m_selection.contains(gpos)) {
      if (m_currentTool == Text) {
        startTextInput(gpos);
        return;
      }
      m_drawingActive = true;
      m_drawingAnno = Annotation();
      m_drawingAnno.tool = m_currentTool;
      m_drawingAnno.color = m_drawColor;
      m_drawingAnno.penWidth = m_penWidth;
      QPoint relPos = gpos - m_selection.topLeft();
      if (m_currentTool == Rect || m_currentTool == Ellipse ||
          m_currentTool == Arrow) {
        m_drawingAnno.points << relPos << relPos;
      } else {
        m_drawingAnno.points << relPos;
      }
      m_popupVisible = false;  // 开始绘制时关闭弹窗
      setCursor(Qt::CrossCursor);
      return;
    }

    // 无工具 → 调选区
    if (m_currentTool == None) {
      int h = handleAt(gpos);
      if (h >= 0) {
        m_activeHandle = h;
        m_dragging = true;
        // 锚点设为对角
        switch (h) {
          case TopLeft: m_resizeAnchor = m_selection.bottomRight(); break;
          case Top: break;  // 边手柄无锚点，只改 Y
          case TopRight: m_resizeAnchor = m_selection.bottomLeft(); break;
          case Right: break;  // 只改 X
          case BottomRight: m_resizeAnchor = m_selection.topLeft(); break;
          case Bottom: break;  // 只改 Y
          case BottomLeft: m_resizeAnchor = m_selection.topRight(); break;
          case Left: break;  // 只改 X
        }
        return;
      }
      // 选区内整体移动
      if (m_selection.contains(gpos)) {
        m_dragging = true;
        m_movingSelection = true;
        m_dragOffset = gpos - m_selection.topLeft();
        setCursor(Qt::SizeAllCursor);
        return;
      }
    }
  }
}

void ScreenShotWindow::mouseMoveEvent(QMouseEvent *event) {
  QPoint gpos = event->globalPosition().toPoint();

  if (m_phase == Drag) {
    if (m_dragging) {
      m_selection = QRect(m_startPoint, gpos).normalized();
      m_hoverValid = false;
      update();
    } else {
      detectHoverWindow(gpos);
      update();
    }
  } else if (m_phase == Confirm) {
    if (m_drawingActive) {
      QPoint relPos = gpos - m_selection.topLeft();
      if (m_drawingAnno.tool == Rect || m_drawingAnno.tool == Ellipse ||
          m_drawingAnno.tool == Arrow) {
        if (m_drawingAnno.points.size() >= 2)
          m_drawingAnno.points[1] = relPos;
        else
          m_drawingAnno.points << relPos;
      } else {
        m_drawingAnno.points << relPos;
      }
      update();
    } else if (m_dragging && m_activeHandle >= 0) {
      // 手柄缩放——角手柄用锚点对角，边手柄只改单维度
      switch (m_activeHandle) {
        case TopLeft:
          m_selection = QRect(gpos, m_resizeAnchor).normalized(); break;
        case Top:
          m_selection.setTop(gpos.y()); break;
        case TopRight:
          m_selection = QRect(QPoint(m_resizeAnchor.x(), gpos.y()),
                              QPoint(gpos.x(), m_resizeAnchor.y())).normalized(); break;
        case Right:
          m_selection.setRight(gpos.x()); break;
        case BottomRight:
          m_selection = QRect(m_resizeAnchor, gpos).normalized(); break;
        case Bottom:
          m_selection.setBottom(gpos.y()); break;
        case BottomLeft:
          m_selection = QRect(QPoint(gpos.x(), m_resizeAnchor.y()),
                              QPoint(m_resizeAnchor.x(), gpos.y())).normalized(); break;
        case Left:
          m_selection.setLeft(gpos.x()); break;
      }
      // 约束最小尺寸
      if (m_selection.width() < 10) m_selection.setWidth(10);
      if (m_selection.height() < 10) m_selection.setHeight(10);
      // 约束在虚拟屏内
      QRect total = geometry();
      if (m_selection.left() < total.left()) m_selection.moveLeft(total.left());
      if (m_selection.top() < total.top()) m_selection.moveTop(total.top());
      if (m_selection.right() > total.right())
        m_selection.moveRight(total.right());
      if (m_selection.bottom() > total.bottom())
        m_selection.moveBottom(total.bottom());
      update();
    } else if (m_movingSelection) {
      // 整体移动选区
      QPoint newTL = gpos - m_dragOffset;
      QRect total = geometry();
      if (newTL.x() < total.left()) newTL.setX(total.left());
      if (newTL.y() < total.top()) newTL.setY(total.top());
      if (newTL.x() + m_selection.width() > total.right())
        newTL.setX(total.right() - m_selection.width());
      if (newTL.y() + m_selection.height() > total.bottom())
        newTL.setY(total.bottom() - m_selection.height());
      m_selection.moveTopLeft(newTL);
      update();
    } else {
      // 空闲：更新 hover 状态 + 光标
      QPoint origin = geometry().topLeft();
      int newHover = -1;
      // 检查各工具按钮
      Tool tools[] = {Rect, Ellipse, Arrow, Pen, Mosaic, Text};
      for (int i = 0; i < 6; ++i) {
        if (toolBtnRect(tools[i]).translated(-origin).contains(mapFromGlobal(gpos))) {
          newHover = i;
          break;
        }
      }
      if (newHover < 0 && undoBtnRect().translated(-origin).contains(mapFromGlobal(gpos)))
        newHover = 6;
      if (newHover < 0 && pinBtnRect().translated(-origin).contains(mapFromGlobal(gpos)))
        newHover = 7;
      if (m_hoveredBtn != newHover) {
        m_hoveredBtn = newHover;
        update();
      }
      if (newHover >= 0) setCursor(Qt::PointingHandCursor);
      else if (m_currentTool != None) setCursor(Qt::CrossCursor);
      else setCursorForPos(gpos);
    }
  }
}

void ScreenShotWindow::mouseReleaseEvent(QMouseEvent *event) {
  if (event->button() != Qt::LeftButton) return;

  if (m_phase == Drag) {
    m_dragging = false;
    if (m_selection.isEmpty() || m_selection.width() <= kDragThreshold) {
      QPoint gpos = event->globalPosition().toPoint();
      detectHoverWindow(gpos);
      if (m_hoverValid) {
        m_selection = m_hoverRect;
        m_phase = Confirm;
        setCursor(Qt::ArrowCursor);
        update();
      }
      return;
    }
    if (m_selection.width() > 5 && m_selection.height() > 5) {
      m_phase = Confirm;
      setCursor(Qt::ArrowCursor);
      update();
    }
  } else if (m_phase == Confirm) {
    if (m_drawingActive) {
      m_drawingActive = false;
      bool valid = false;
      if (m_drawingAnno.tool == Rect || m_drawingAnno.tool == Ellipse ||
          m_drawingAnno.tool == Arrow) {
        if (m_drawingAnno.points.size() >= 2) {
          QPoint s = m_drawingAnno.points[0];
          QPoint e = m_drawingAnno.points[1];
          if (std::abs(e.x()-s.x()) > 2 || std::abs(e.y()-s.y()) > 2) valid = true;
        }
      } else {
        if (m_drawingAnno.points.size() >= 2) valid = true;
      }
      if (valid) m_annotations.append(m_drawingAnno);
      m_drawingAnno = Annotation();
      // 恢复弹窗
      if (m_currentTool != None && m_currentTool != Mosaic)
        m_popupVisible = true;
      update();
    }
    m_dragging = false;
    m_movingSelection = false;
    m_activeHandle = -1;
  }
}

void ScreenShotWindow::keyPressEvent(QKeyEvent *event) {
  if (event->key() == Qt::Key_Escape) {
    if (m_textInput) { commitTextInput(); return; }
    if (m_drawingActive) {
      m_drawingActive = false;
      m_drawingAnno = Annotation();
      update();
      return;
    }
    if (m_popupVisible) {
      m_popupVisible = false;
      update();
      return;
    }
    if (m_currentTool != None) {
      m_currentTool = None;
      update();
      return;
    }
    emit cancelled();
    close();
    return;
  }
  if (event->key() == Qt::Key_Z && (event->modifiers() & Qt::ControlModifier)) {
    if (!m_annotations.isEmpty()) {
      m_annotations.removeLast();
      update();
    }
    return;
  }
  if (m_phase == Confirm && !m_textInput) {
    Tool newTool = None;
    switch (event->key()) {
      case Qt::Key_1: newTool = Rect; break;
      case Qt::Key_2: newTool = Ellipse; break;
      case Qt::Key_3: newTool = Arrow; break;
      case Qt::Key_4: newTool = Pen; break;
      case Qt::Key_5: newTool = Mosaic; break;
      case Qt::Key_6: newTool = Text; break;
      default: break;
    }
    if (newTool != None) {
      if (m_currentTool == newTool) {
        m_currentTool = None;
        m_popupVisible = false;
      } else {
        m_currentTool = newTool;
        m_popupVisible = (newTool != Mosaic);
      }
      update();
    }
  }
}

void ScreenShotWindow::wheelEvent(QWheelEvent *event) {
  // 截图覆盖层拦截所有滚轮事件——截图不需要滚动。
  // 不调用基类，直接吞掉（event->accept() 已在 override 返回后自动设置）。
  event->accept();
}
