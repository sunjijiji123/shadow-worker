#include "screenshotwindow.h"

#include <cmath>

#include <QApplication>
#include <QClipboard>
#include <QDateTime>
#include <QDir>
#include <QFontMetrics>
#include <QGuiApplication>
#include <QImage>
#include <QKeyEvent>
#include <QMouseEvent>
#include <QPainter>
#include <QPainterPath>
#include <QScreen>

// 用 qt_windows.h 而非 windows.h：Qt 封装自带 NOMINMAX，避免 min/max 宏
// 破坏 std::min/std::max（C2589 "非法的标记 (" 就是该冲突的典型症状）。
// 项目内 windowpicker.cpp / textinjector.cpp 均用此头。
#include <qt_windows.h>

// WindowFromPoint 的输入 POINT 结构体（windows.h 已定义）。
// GetAncestor + GA_ROOT(2) 回溯到顶层窗口——WindowFromPoint 返回的可能是
// 子窗口（按钮/静态控件等），需要回溯到顶层才能拿到正确的窗口矩形。

namespace {
// GA_ROOT = 2（GetAncestor 的 GetAncestorFlags 枚举值）。
constexpr UINT kGaRoot = 2;

// 判断窗口是否"值得高亮"：可见、有标题、非工具窗口、非自身覆盖层。
bool isHighlightableWindow(HWND hwnd, HWND selfHwnd) {
  if (!hwnd || hwnd == selfHwnd) return false;
  if (!IsWindowVisible(hwnd)) return false;
  // 排除无标题窗口（可能是其他覆盖层/桌面子窗口）。
  WCHAR title[256] = {0};
  int len = GetWindowTextW(hwnd, title, 256);
  if (len == 0) return false;
  // 排除自身覆盖层的子窗口（理论上 ScreenShotWindow 无子窗口，保险起见）。
  HWND ancestor = GetAncestor(hwnd, kGaRoot);
  if (ancestor == selfHwnd) return false;
  // 只要有标题且可见就高亮——包括桌面（explorer）。
  return true;
}
}  // namespace

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
      m_drawColor(QColor(244, 67, 54)),  // 默认红色（标注醒目）
      m_drawingActive(false) {
  setWindowFlags(Qt::FramelessWindowHint | Qt::Tool |
                 Qt::WindowStaysOnTopHint);
  setAttribute(Qt::WA_TranslucentBackground, true);
  setAttribute(Qt::WA_NoSystemBackground, true);
  setMouseTracking(true);

  // 多显示器：geometry = 所有屏幕并集（虚拟屏）。
  QRect total;
  const auto screens = QGuiApplication::screens();
  for (QScreen *screen : screens) {
    total = total.united(screen->geometry());
  }
  setGeometry(total);

  initFullscreenPixmap();

  setCursor(Qt::CrossCursor);
}

void ScreenShotWindow::initFullscreenPixmap() {
  // 抓取所有屏幕画面拼到一张虚拟屏 Pixmap（纯物理像素，不设 devicePixelRatio）。
  //
  // 【关键】不调 setDevicePixelRatio，让 m_fullPixmap 保持纯物理像素语义：
  // 尺寸 = 物理像素，copy()/drawPixmap 源矩形一律用物理坐标。这消除了 Qt6 下
  // "设了 dpr 的 Pixmap 几何 API 是逻辑还是物理"的歧义——那个歧义正是 125%
  // 缩放时保存的截图区域错位的根因。
  QRect total = geometry();
  qreal dpr = devicePixelRatioF();
  m_fullPixmap = QPixmap(total.size() * dpr);
  m_fullPixmap.fill(Qt::transparent);

  QPainter p(&m_fullPixmap);
  for (QScreen *screen : QGuiApplication::screens()) {
    QRect geo = screen->geometry();
    // 屏幕逻辑偏移 × dpr → 物理像素偏移（p 在物理像素 Pixmap 上绘制）
    QPoint offset = (geo.topLeft() - total.topLeft()) * dpr;
    QPixmap grab = screen->grabWindow(0);
    // grabWindow 返回物理像素图（自带 dpr）。这里去掉它的 dpr 标记，
    // 让 p.drawPixmap(offset, grab) 按 grab 的物理像素尺寸 1:1 绘制到
    // m_fullPixmap 的物理坐标 offset 处。
    grab.setDevicePixelRatio(1.0);
    p.drawPixmap(offset, grab);
  }
  p.end();
}

// ---- 颜色调色板 ----

QColor ScreenShotWindow::annotationColor(int index) const {
  static const QColor colors[kColorCount] = {
      QColor(244, 67, 54),   // 红（默认）
      QColor(255, 193, 7),   // 黄
      QColor(76, 175, 80),   // 绿
      QColor(33, 150, 243),  // 蓝
      QColor(255, 255, 255)  // 白
  };
  return colors[qBound(0, index, kColorCount - 1)];
}

// ---- 手柄几何 ----

QRect ScreenShotWindow::handleRect(HandlePos pos) const {
  QPoint pt;
  switch (pos) {
    case TopLeft:
      pt = m_selection.topLeft();
      break;
    case Top:
      pt = QPoint(m_selection.center().x(), m_selection.top());
      break;
    case TopRight:
      pt = m_selection.topRight();
      break;
    case Right:
      pt = QPoint(m_selection.right(), m_selection.center().y());
      break;
    case BottomRight:
      pt = m_selection.bottomRight();
      break;
    case Bottom:
      pt = QPoint(m_selection.center().x(), m_selection.bottom());
      break;
    case BottomLeft:
      pt = m_selection.bottomLeft();
      break;
    case Left:
      pt = QPoint(m_selection.left(), m_selection.center().y());
      break;
  }
  int hs = kHandleSize / 2;
  return QRect(pt.x() - hs, pt.y() - hs, kHandleSize, kHandleSize);
}

ScreenShotWindow::HandlePos ScreenShotWindow::handleAt(
    QPoint globalPos) const {
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
    case TopLeft:
    case BottomRight:
      setCursor(Qt::SizeFDiagCursor);
      break;
    case TopRight:
    case BottomLeft:
      setCursor(Qt::SizeBDiagCursor);
      break;
    case Top:
    case Bottom:
      setCursor(Qt::SizeVerCursor);
      break;
    case Left:
    case Right:
      setCursor(Qt::SizeHorCursor);
      break;
    default:
      if (m_selection.contains(mapFromGlobal(globalPos))) {
        setCursor(Qt::SizeAllCursor);
      } else {
        setCursor(Qt::ArrowCursor);
      }
      break;
  }
}

// ---- 工具条几何 ----
//
// 统一布局（Confirm 阶段，不再分 Annotate 子阶段）：
//   [颜色×5] [□][→][✎][▦] | [↶] [✦?] [✓] [✗]
// 颜色组在最左，工具组紧随，撤销与动作按钮之间有分隔。
// 按钮统一高度 kBtnH，工具图标按钮宽 kBtnW，动作按钮宽 kActionBtnW。

static constexpr int kBtnW = 36;        // 标注/撤销等图标按钮宽
static constexpr int kActionBtnW = 56;  // 识别/完成/取消按钮宽（带文字）
static constexpr int kBtnH = 32;
static constexpr int kGap = 4;          // 按钮间距
static constexpr int kSepGap = 10;      // 分隔符间距（视觉分组）
static constexpr int kPadding = 4;
static constexpr int kColorDotSize = 20;

QRect ScreenShotWindow::toolbarRect() const {
  // 颜色组 + 分隔 + 工具组 + 分隔 + 撤销 + 分隔 + 动作组
  int colorW = kColorCount * kColorDotSize + kGap * (kColorCount - 1);
  int toolW = 4 * kBtnW + kGap * 3;
  int undoW = kBtnW;
  int actionCount = m_showRecognizeBtn ? 3 : 2;
  int actionW = actionCount * kActionBtnW + kGap * (actionCount - 1);

  int w = kPadding * 2 + colorW + kSepGap + toolW + kSepGap + undoW +
          kSepGap + actionW;
  int h = kBtnH + kPadding * 2;
  int x = m_selection.right() - w + kPadding;
  int y = m_selection.bottom() + 8;

  QRect total = geometry();
  if (y + h > total.bottom()) {
    y = m_selection.top() - h - 8;
  }
  if (x < total.left()) x = total.left() + 4;
  if (x + w > total.right()) x = total.right() - w - 4;

  return QRect(x, y, w, h);
}

// 工具条内按钮 X 起点（左边 padding 后的第一个像素）。
static int toolbarStartX(const QRect &tb) { return tb.left() + kPadding; }

QRect ScreenShotWindow::colorDotRect(int index) const {
  QRect tb = toolbarRect();
  int x = toolbarStartX(tb);
  int y = tb.top() + (kBtnH - kColorDotSize) / 2 + kPadding;
  x += index * (kColorDotSize + kGap);
  return QRect(x, y, kColorDotSize, kColorDotSize);
}

QRect ScreenShotWindow::toolBtnRect(Tool tool) const {
  QRect tb = toolbarRect();
  int x = toolbarStartX(tb);
  // 跳过颜色组 + 分隔
  x += kColorCount * kColorDotSize + kGap * (kColorCount - 1) + kSepGap;
  int idx = 0;
  switch (tool) {
    case Rect: idx = 0; break;
    case Arrow: idx = 1; break;
    case Pen: idx = 2; break;
    case Mosaic: idx = 3; break;
    default: idx = 0; break;
  }
  x += idx * (kBtnW + kGap);
  int y = tb.top() + kPadding;
  return QRect(x, y, kBtnW, kBtnH);
}

QRect ScreenShotWindow::undoBtnRect() const {
  QRect tb = toolbarRect();
  int x = toolbarStartX(tb);
  // 跳过颜色组 + 分隔 + 工具组 + 分隔
  x += kColorCount * kColorDotSize + kGap * (kColorCount - 1) + kSepGap;
  x += 4 * kBtnW + kGap * 3 + kSepGap;
  int y = tb.top() + kPadding;
  return QRect(x, y, kBtnW, kBtnH);
}

QRect ScreenShotWindow::recognizeBtnRect() const {
  QRect tb = toolbarRect();
  int x = undoBtnRect().right() + kSepGap;
  int y = tb.top() + kPadding;
  return QRect(x, y, kActionBtnW, kBtnH);
}

QRect ScreenShotWindow::confirmBtnRect() const {
  int x;
  if (m_showRecognizeBtn) {
    x = recognizeBtnRect().right() + kGap;
  } else {
    x = undoBtnRect().right() + kSepGap;
  }
  int y = toolbarRect().top() + kPadding;
  return QRect(x, y, kActionBtnW, kBtnH);
}

QRect ScreenShotWindow::cancelBtnRect() const {
  int x = confirmBtnRect().right() + kGap;
  int y = toolbarRect().top() + kPadding;
  return QRect(x, y, kActionBtnW, kBtnH);
}

// ---- 窗口智能预选 ----

void ScreenShotWindow::detectHoverWindow(QPoint globalPos) {
  HWND selfHwnd = reinterpret_cast<HWND>(winId());

  // 临时让自身不拦截命中检测：WindowFromPoint 会命中透明覆盖层自身。
  // 方案：临时隐藏自身（setWindowOpacity(0) 不够，需 SetWindowPos 或
  // ShowWindow）。但隐藏会导致鼠标事件中断。改用直接调 WindowFromPoint
  // 后用 GetAncestor 回溯 + 排除自身。
  //
  // 注意：WindowFromPoint 对透明窗口的行为——WA_TranslucentBackground 的
  // 窗口仍会被命中（它不是真正"透明"的，只是绘制透明）。所以必须排除自身。
  POINT pt = {globalPos.x(), globalPos.y()};
  HWND hwnd = WindowFromPoint(pt);
  if (!hwnd) {
    m_hoverValid = false;
    return;
  }

  // 回溯到顶层窗口
  HWND root = GetAncestor(hwnd, kGaRoot);
  if (!root || root == selfHwnd) {
    m_hoverValid = false;
    return;
  }

  if (!isHighlightableWindow(root, selfHwnd)) {
    m_hoverValid = false;
    return;
  }

  RECT rc;
  if (!GetWindowRect(root, &rc)) {
    m_hoverValid = false;
    return;
  }

  m_hoverRect = QRect(QPoint(rc.left, rc.top),
                      QPoint(rc.right - 1, rc.bottom - 1));
  // 约束在虚拟屏内
  QRect total = geometry();
  m_hoverRect = m_hoverRect.intersected(total);
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

  // 半透明高亮遮罩（窗口内部稍亮，突出选区）
  // 先填充窗口区域为微透明红色
  p.fillRect(local, QColor(244, 67, 54, 30));

  // 2px 红色边框
  QPen pen(QColor(244, 67, 54), 2);
  p.setPen(pen);
  p.setBrush(Qt::NoBrush);
  p.drawRect(local);

  // 四角 L 标记（增强可见性，类似 Snipaste）
  int cs = 12;  // 角标边长
  pen.setWidth(3);
  pen.setColor(QColor(244, 67, 54));
  p.setPen(pen);
  // 左上
  p.drawLine(local.topLeft(), local.topLeft() + QPoint(cs, 0));
  p.drawLine(local.topLeft(), local.topLeft() + QPoint(0, cs));
  // 右上
  p.drawLine(local.topRight(), local.topRight() + QPoint(-cs, 0));
  p.drawLine(local.topRight(), local.topRight() + QPoint(0, cs));
  // 左下
  p.drawLine(local.bottomLeft(), local.bottomLeft() + QPoint(cs, 0));
  p.drawLine(local.bottomLeft(), local.bottomLeft() + QPoint(0, -cs));
  // 右下
  p.drawLine(local.bottomRight(), local.bottomRight() + QPoint(-cs, 0));
  p.drawLine(local.bottomRight(), local.bottomRight() + QPoint(0, -cs));

  // 标题 tooltip（边框上方）
  if (!m_hoverTitle.isEmpty()) {
    QFont f = p.font();
    f.setPixelSize(12);
    p.setFont(f);
    QFontMetrics fm(f);
    int textW = fm.horizontalAdvance(m_hoverTitle);
    int textH = fm.height();
    int tipX = local.left();
    int tipY = local.top() - textH - 6;
    if (tipY < 0) tipY = local.bottom() + 4;  // 空间不够放下方

    QRect tipRect(tipX, tipY, textW + 12, textH + 4);
    QPainterPath tipPath;
    tipPath.addRoundedRect(tipRect, 3, 3);
    p.fillPath(tipPath, QColor(0, 0, 0, 200));
    p.setPen(Qt::white);
    p.drawText(tipRect, Qt::AlignCenter, m_hoverTitle);
  }
}

// ---- 标注绘制 ----

void ScreenShotWindow::drawAnnotation(QPainter &p, const Annotation &a,
                                       QPoint selOrigin) const {
  // 标注坐标是选区相对坐标，绘制时加回 selOrigin（虚拟屏全局→widget 本地）。
  // 但这里 selOrigin 已是 m_selection.topLeft() - geometry().topLeft()
  // （即选区在 widget 本地坐标系的位置）。标注点 + selOrigin = widget 本地坐标。
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
    case Arrow: {
      if (a.points.size() < 2) break;
      QPoint s = a.points[0] + selOrigin;
      QPoint e = a.points[1] + selOrigin;
      p.drawLine(s, e);
      // 箭头头：沿方向画两条 45° 线
      double angle = std::atan2(e.y() - s.y(), e.x() - s.x());
      int arrowLen = 12 + a.penWidth * 2;
      double a1 = angle + M_PI * 0.8;
      double a2 = angle - M_PI * 0.8;
      p.drawLine(e, e + QPoint(static_cast<int>(arrowLen * std::cos(a1)),
                               static_cast<int>(arrowLen * std::sin(a1))));
      p.drawLine(e, e + QPoint(static_cast<int>(arrowLen * std::cos(a2)),
                               static_cast<int>(arrowLen * std::sin(a2))));
      break;
    }
    case Pen: {
      QPolygon poly;
      for (const QPoint &pt : a.points) {
        poly << pt + selOrigin;
      }
      p.drawPolyline(poly);
      break;
    }
    case Mosaic: {
      // 马赛克需要从底图取像素，在 drawMosaic 中单独处理。
      // 这里不画（drawMosaic 在 paintEvent 中被显式调用）。
      break;
    }
    default:
      break;
  }
}

void ScreenShotWindow::drawMosaic(QPainter &p, const Annotation &a,
                                   const QPixmap &srcPixmap,
                                   QPoint selOrigin, qreal dpr) const {
  if (a.points.isEmpty()) return;

  // 计算标注覆盖区域（在 widget 本地坐标系）。
  // Mosaic 的 points 是选区相对坐标的一系列点（画笔轨迹），
  // 取其 bounding box 做块状像素化。
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

  // 从 srcPixmap（物理像素 m_fullPixmap）取对应区域。
  // widget 本地坐标 → 物理像素坐标：× dpr + 选区在 pixmap 中的偏移。
  // selOrigin 是选区在 widget 本地的位置；选区在 m_fullPixmap 中的偏移
  // = (m_selection.topLeft() - geometry().topLeft()) * dpr = selOrigin * dpr。
  // 标注点（选区相对）在 m_fullPixmap 中的位置 = (selOrigin + pt) * dpr。
  QPoint pixMin = (minPt) * dpr;  // widget 本地→物理像素
  QPoint pixMax = (maxPt) * dpr;
  QRect pixRect(pixMin, pixMax);
  pixRect = pixRect.intersected(QRect(0, 0, srcPixmap.width(), srcPixmap.height()));
  if (pixRect.isEmpty()) return;

  // 取出该区域转 QImage 做块平均
  QImage srcImg = srcPixmap.copy(pixRect).toImage();
  int blockSize = 8;  // 马赛克块大小（物理像素）
  for (int by = 0; by < srcImg.height(); by += blockSize) {
    for (int bx = 0; bx < srcImg.width(); bx += blockSize) {
      int bw = std::min(blockSize, srcImg.width() - bx);
      int bh = std::min(blockSize, srcImg.height() - by);
      // 取块内平均色
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
        for (int y = by; y < by + bh; ++y) {
          for (int x = bx; x < bx + bw; ++x) {
            srcImg.setPixel(x, y, avg);
          }
        }
      }
    }
  }

  // 把马赛克后的图像画回 widget（widget 本地坐标，pixmap 自带 dpr 缩放）
  // 注意：srcImg 是物理像素，需要设 dpr 让 QPainter 正确缩放到逻辑坐标。
  QPixmap mosPix = QPixmap::fromImage(srcImg);
  mosPix.setDevicePixelRatio(dpr);
  p.drawPixmap(widgetRect.topLeft(), mosPix);
}

void ScreenShotWindow::paintAnnotationsOnPixmap(QPixmap &pixmap,
                                                 qreal dpr) const {
  if (m_annotations.isEmpty()) return;

  // pixmap 是选区裁剪后的物理像素图（不设 dpr）。
  // 标注坐标是选区相对坐标（逻辑），需 × dpr 转物理像素。
  QPainter p(&pixmap);
  p.setRenderHint(QPainter::Antialiasing);

  // selOrigin 在 pixmap 坐标系中是 (0,0)——pixmap 就是选区内容。
  // 标注点 × dpr = pixmap 中的物理像素坐标。
  for (const Annotation &a : m_annotations) {
    if (a.tool == Mosaic) {
      // 马赛克：pixmap 本身就是底图，直接在 pixmap 上做块平均覆盖。
      if (a.points.isEmpty()) continue;
      QPoint offset = a.points.first();
      QPoint minPt = offset, maxPt = offset;
      for (const QPoint &pt : a.points) {
        minPt.setX(std::min(minPt.x(), pt.x()));
        minPt.setY(std::min(minPt.y(), pt.y()));
        maxPt.setX(std::max(maxPt.x(), pt.x()));
        maxPt.setY(std::max(maxPt.y(), pt.y()));
      }
      QRect logicRect(minPt, maxPt);
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
            for (int y = by; y < by + bh; ++y) {
              for (int x = bx; x < bw + bx; ++x) {
                img.setPixel(x, y, avg);
              }
            }
          }
        }
      }
      p.drawImage(pixRect, img);
    } else {
      // 非马赛克：构造临时 Annotation（坐标乘 dpr）复用 drawAnnotation。
      Annotation scaled = a;
      QList<QPoint> scaledPoints;
      for (const QPoint &pt : a.points) {
        scaledPoints << QPoint(static_cast<int>(pt.x() * dpr),
                               static_cast<int>(pt.y() * dpr));
      }
      scaled.points = scaledPoints;
      scaled.penWidth = static_cast<int>(a.penWidth * dpr);
      drawAnnotation(p, scaled, QPoint(0, 0));
    }
  }
  p.end();
}

// ---- 工具条图标矢量绘制 ----
//
// 所有图标在 rect 居中绘制，用 QPainter 矢量图形（非 Unicode 符号），
// 保证清晰度和一致的视觉风格。iconColor 控制图标颜色。
// tool=None 时画撤销图标（↶ 箭头）。

void ScreenShotWindow::drawToolIcon(QPainter &p, Tool tool, const QRect &rect,
                                     const QColor &color) const {
  p.save();
  p.setRenderHint(QPainter::Antialiasing);

  // 图标绘制区域：rect 居中缩进 8px
  int m = 8;
  QRect ir = rect.adjusted(m, m, -m, -m);
  if (ir.width() < 6 || ir.height() < 6) ir = rect;

  QPen pen(color);
  pen.setCapStyle(Qt::RoundCap);
  pen.setJoinStyle(Qt::RoundJoin);
  p.setPen(pen);
  p.setBrush(Qt::NoBrush);

  switch (tool) {
    case Rect: {
      // 空心圆角矩形
      pen.setWidth(2);
      p.setPen(pen);
      p.drawRoundedRect(ir, 2, 2);
      break;
    }
    case Arrow: {
      // 斜向箭头（左上→右下）+ 箭头头
      pen.setWidth(2);
      p.setPen(pen);
      QPoint s = ir.topLeft();
      QPoint e = ir.bottomRight();
      p.drawLine(s, e);
      // 箭头头：两条短线
      int ah = 7;  // 箭头头长度
      double angle = std::atan2(e.y() - s.y(), e.x() - s.x());
      double a1 = angle + M_PI * 0.8;
      double a2 = angle - M_PI * 0.8;
      p.drawLine(e, e + QPoint(static_cast<int>(ah * std::cos(a1)),
                               static_cast<int>(ah * std::sin(a1))));
      p.drawLine(e, e + QPoint(static_cast<int>(ah * std::cos(a2)),
                               static_cast<int>(ah * std::sin(a2))));
      break;
    }
    case Pen: {
      // 铅笔图标：斜线 + 笔尖三角
      pen.setWidth(2);
      p.setPen(pen);
      // 笔杆：从左下到右上
      QPoint p1 = ir.bottomLeft();
      QPoint p2 = ir.topRight();
      p.drawLine(p1, p2);
      // 笔尖：在 p1（左下）画一个小三角
      pen.setWidth(1.5);
      p.setPen(pen);
      p.setBrush(color);
      QPolygon tip;
      tip << p1 << QPoint(p1.x() + 5, p1.y()) << QPoint(p1.x(), p1.y() - 5);
      p.drawPolygon(tip);
      p.setBrush(Qt::NoBrush);
      break;
    }
    case Mosaic: {
      // 马赛克：4×4 小方格网格
      pen.setWidth(1);
      p.setPen(pen);
      int cols = 4, rows = 4;
      int cw = ir.width() / cols;
      int ch = ir.height() / rows;
      // 画网格线
      for (int c = 0; c <= cols; ++c) {
        int x = ir.left() + c * cw;
        p.drawLine(x, ir.top(), x, ir.top() + rows * ch);
      }
      for (int r = 0; r <= rows; ++r) {
        int y = ir.top() + r * ch;
        p.drawLine(ir.left(), y, ir.left() + cols * cw, y);
      }
      // 隔格填充（棋盘格效果）
      p.setBrush(color);
      p.setPen(Qt::NoPen);
      for (int r = 0; r < rows; ++r) {
        for (int c = 0; c < cols; ++c) {
          if ((r + c) % 2 == 0) {
            p.drawRect(ir.left() + c * cw, ir.top() + r * ch, cw, ch);
          }
        }
      }
      break;
    }
    default: {
      // None = 撤销图标（弯曲箭头 ↶）
      pen.setWidth(2);
      p.setPen(pen);
      // 画一段弧线（从右下弯到左上）
      QPoint center = ir.center();
      int radius = ir.width() / 2;
      // 弧线：从 0° 到 270°（顺时针）
      p.drawArc(QRect(center.x() - radius, center.y() - radius,
                      radius * 2, radius * 2),
                0 * 16, 270 * 16);
      // 箭头头在弧线终点（左上方向）
      QPoint arrowTip = center + QPoint(
          static_cast<int>(radius * std::cos(M_PI * 1.5)),
          static_cast<int>(radius * std::sin(M_PI * 1.5)));
      int ah = 6;
      // 箭头朝右下（弧线切线方向）
      p.drawLine(arrowTip, arrowTip + QPoint(ah, 0));
      p.drawLine(arrowTip, arrowTip + QPoint(0, ah));
      break;
    }
  }
  p.restore();
}

// ---- paintEvent ----

void ScreenShotWindow::paintEvent(QPaintEvent *) {
  QPainter p(this);
  p.setRenderHint(QPainter::Antialiasing);

  QPoint origin = geometry().topLeft();

  // 全屏半透明遮罩
  QColor mask(0, 0, 0, 115);
  p.fillRect(rect(), mask);

  // Drag 阶段：窗口智能预选高亮（在选区之前画，因为还没选区）
  if (m_phase == Drag && m_hoverValid && !m_dragging) {
    drawHoverHighlight(p, origin);
  }

  if (m_selection.isEmpty() && !(m_phase == Drag && m_dragging)) {
    // Drag 阶段正在拖拽但还没松开时，m_selection 可能有值（mouseMoveEvent 设置），
    // 不会进这个分支。Drag 阶段未拖拽且无选区 → 只显示遮罩 + 窗口高亮。
    return;
  }

  // m_selection 是逻辑坐标（虚拟屏全局）。m_fullPixmap 是纯物理像素（不设 dpr）。
  // 选区还原：目标矩形用逻辑坐标，源矩形用物理坐标，Qt 自动按 dpr 缩放。
  qreal dpr = devicePixelRatioF();
  QPoint localTopLeft = m_selection.topLeft() - origin;
  QSize logicalSize = m_selection.size();
  QRect targetRect(localTopLeft, logicalSize);
  QPoint srcOffset = (m_selection.topLeft() - origin) * dpr;
  QSize srcSize = m_selection.size() * dpr;
  QRect sourceRect(srcOffset, srcSize);
  p.setCompositionMode(QPainter::CompositionMode_SourceOver);
  p.drawPixmap(targetRect, m_fullPixmap, sourceRect);

  // 绘制标注（已完成 + 正在绘制）
  // selOrigin = 选区在 widget 本地坐标系的位置。
  QPoint selOrigin = localTopLeft;
  for (const Annotation &a : m_annotations) {
    if (a.tool == Mosaic) {
      drawMosaic(p, a, m_fullPixmap, selOrigin, dpr);
    } else {
      drawAnnotation(p, a, selOrigin);
    }
  }
  if (m_drawingActive) {
    if (m_drawingAnno.tool == Mosaic) {
      drawMosaic(p, m_drawingAnno, m_fullPixmap, selOrigin, dpr);
    } else {
      drawAnnotation(p, m_drawingAnno, selOrigin);
    }
  }

  // 白色虚线边框（widget 本地坐标）
  QPen pen(Qt::white, 2, Qt::DashLine);
  p.setPen(pen);
  p.setBrush(Qt::NoBrush);
  p.drawRect(QRect(localTopLeft, m_selection.size()));

  if (m_phase == Confirm) {
    // 8 个调整手柄（仅未选标注工具时显示——选了工具时手柄会干扰标注绘制）
    if (m_currentTool == None && !m_drawingActive) {
      p.setBrush(Qt::white);
      p.setPen(Qt::NoPen);
      for (int i = TopLeft; i <= Left; ++i) {
        QRect h = handleRect(static_cast<HandlePos>(i));
        h.translate(-origin);
        p.drawRect(h);
      }
    }

    // ---- 工具条（统一布局）----
    QRect tb = toolbarRect();
    tb.translate(-origin);
    QPainterPath tbPath;
    tbPath.addRoundedRect(tb.x(), tb.y(), tb.width(), tb.height(), 6, 6);
    p.setCompositionMode(QPainter::CompositionMode_SourceOver);
    p.fillPath(tbPath, QColor(37, 37, 37, 235));

    // ---- 颜色圆点 ----
    for (int i = 0; i < kColorCount; ++i) {
      QRect r = colorDotRect(i);
      r.translate(-origin);
      QColor c = annotationColor(i);
      QPainterPath cp;
      cp.addEllipse(r);
      p.fillPath(cp, c);
      // 选中态：白色外圈
      if (c == m_drawColor) {
        QPen selPen(Qt::white, 2);
        p.setPen(selPen);
        p.setBrush(Qt::NoBrush);
        p.drawEllipse(r.adjusted(-2, -2, 2, 2));
        p.setPen(Qt::NoPen);
      }
    }

    // ---- 标注工具按钮（矢量图标）----
    auto drawToolBtn = [&](Tool tool) {
      QRect r = toolBtnRect(tool);
      r.translate(-origin);
      // 选中态高亮背景
      if (m_currentTool == tool) {
        QPainterPath rp;
        rp.addRoundedRect(r.adjusted(1, 1, -1, -1), 4, 4);
        p.fillPath(rp, QColor(59, 130, 246, 180));
      }
      QColor iconColor = (m_currentTool == tool) ? Qt::white : QColor(210, 210, 210);
      drawToolIcon(p, tool, r, iconColor);
    };
    drawToolBtn(Rect);
    drawToolBtn(Arrow);
    drawToolBtn(Pen);
    drawToolBtn(Mosaic);

    // ---- 撤销按钮（矢量图标）----
    {
      QRect r = undoBtnRect();
      r.translate(-origin);
      bool canUndo = !m_annotations.isEmpty();
      QColor iconColor = canUndo ? QColor(210, 210, 210) : QColor(100, 100, 100);
      drawToolIcon(p, None, r, iconColor);  // None 在 drawToolIcon 里画撤销图标
    }

    // ---- 识别按钮 ----
    if (m_showRecognizeBtn) {
      QRect btnR = recognizeBtnRect();
      btnR.translate(-origin);
      QPainterPath rp;
      rp.addRoundedRect(btnR.x(), btnR.y(), btnR.width(), btnR.height(), 4, 4);
      p.fillPath(rp, QColor(59, 130, 246));
      QFont f = p.font();
      f.setPixelSize(13);
      p.setFont(f);
      p.setPen(Qt::white);
      p.drawText(btnR, Qt::AlignCenter, QString::fromUtf8("\u2726 \u8BC6\u522B"));
    }

    // ---- 确认按钮（绿）----
    {
      QRect btnC = confirmBtnRect();
      btnC.translate(-origin);
      QPainterPath cp;
      cp.addRoundedRect(btnC.x(), btnC.y(), btnC.width(), btnC.height(), 4, 4);
      p.fillPath(cp, QColor(76, 175, 80));
      QFont f = p.font();
      f.setPixelSize(13);
      p.setFont(f);
      p.setPen(Qt::white);
      p.drawText(btnC, Qt::AlignCenter, QString::fromUtf8("\u2713 \u5B8C\u6210"));
    }

    // ---- 取消按钮（红）----
    {
      QRect btnX = cancelBtnRect();
      btnX.translate(-origin);
      QPainterPath xp;
      xp.addRoundedRect(btnX.x(), btnX.y(), btnX.width(), btnX.height(), 4, 4);
      p.fillPath(xp, QColor(244, 67, 54));
      QFont f = p.font();
      f.setPixelSize(13);
      p.setFont(f);
      p.setPen(Qt::white);
      p.drawText(btnX, Qt::AlignCenter, QString::fromUtf8("\u2717 \u53D6\u6D88"));
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
  QRect deviceRect(offset, size);
  QPixmap crop = m_fullPixmap.copy(deviceRect);

  // 合成标注到 crop（物理像素坐标）
  if (!m_annotations.isEmpty()) {
    paintAnnotationsOnPixmap(crop, dpr);
  }

  if (!crop.save(filePath, "PNG")) {
    return false;
  }

  // 只写图像（贴近微信行为）。Windows 剪贴板是单一槽位，
  // 若再 setText(filePath) 会把刚写入的 pixmap 清掉变成纯文本路径。
  QClipboard *cb = QApplication::clipboard();
  cb->setPixmap(crop);

  outPath = filePath;
  return true;
}

// ---- 鼠标事件 ----
//
// Confirm 阶段的核心交互逻辑：
//   1. 先检查工具条按钮命中（工具切换/颜色/撤销/完成/取消等）。
//   2. 若 m_currentTool != None 且点击在选区内 → 开始画标注。
//   3. 若 m_currentTool == None：
//      a. 手柄命中 → 缩放选区。
//      b. 选区内点击 → 整体移动选区（m_movingSelection=true）。
//
// 关键修复：移动选区期间不检查 contains——快速移动鼠标时 gpos 可能跳出
// m_selection 范围，若此时用 contains 判断会停止移动（"拖拽丢失"症状）。
// 用 m_movingSelection 标志在 press 时锁定，release 时解锁，move 时无条件跟随。

void ScreenShotWindow::mousePressEvent(QMouseEvent *event) {
  QPoint gpos = event->globalPosition().toPoint();
  QPoint lpos = event->position().toPoint();

  if (event->button() == Qt::RightButton) {
    emit cancelled();
    close();
    return;
  }

  if (event->button() != Qt::LeftButton) return;

  QPoint origin = geometry().topLeft();

  if (m_phase == Drag) {
    m_startPoint = gpos;
    m_selection = QRect();
    m_dragging = true;
    m_hoverValid = false;  // 拖拽开始后清除 hover，mouseMove 会重建自由选区
  } else if (m_phase == Confirm) {
    // ---- 工具条按钮检查（与是否有工具无关，始终可点）----
    // 颜色圆点
    for (int i = 0; i < kColorCount; ++i) {
      if (colorDotRect(i).translated(-origin).contains(lpos)) {
        m_drawColor = annotationColor(i);
        update();
        return;
      }
    }
    // 工具切换（toggle：再点一次取消选中）
    if (toolBtnRect(Rect).translated(-origin).contains(lpos)) {
      m_currentTool = (m_currentTool == Rect) ? None : Rect;
      update();
      return;
    }
    if (toolBtnRect(Arrow).translated(-origin).contains(lpos)) {
      m_currentTool = (m_currentTool == Arrow) ? None : Arrow;
      update();
      return;
    }
    if (toolBtnRect(Pen).translated(-origin).contains(lpos)) {
      m_currentTool = (m_currentTool == Pen) ? None : Pen;
      update();
      return;
    }
    if (toolBtnRect(Mosaic).translated(-origin).contains(lpos)) {
      m_currentTool = (m_currentTool == Mosaic) ? None : Mosaic;
      update();
      return;
    }
    // 撤销
    if (!m_annotations.isEmpty() &&
        undoBtnRect().translated(-origin).contains(lpos)) {
      m_annotations.removeLast();
      update();
      return;
    }
    // 识别
    if (m_showRecognizeBtn &&
        recognizeBtnRect().translated(-origin).contains(lpos)) {
      QString filePath;
      if (saveSelection(filePath)) {
        emit recognized(filePath);
      }
      close();
      return;
    }
    // 完成
    if (confirmBtnRect().translated(-origin).contains(lpos)) {
      QString filePath;
      if (saveSelection(filePath)) {
        emit finished(filePath);
      }
      close();
      return;
    }
    // 取消
    if (cancelBtnRect().translated(-origin).contains(lpos)) {
      emit cancelled();
      close();
      return;
    }

    // ---- 有标注工具选中：在选区内开始绘制标注 ----
    if (m_currentTool != None && m_selection.contains(gpos)) {
      m_drawingActive = true;
      m_drawingAnno = Annotation();
      m_drawingAnno.tool = m_currentTool;
      m_drawingAnno.color = m_drawColor;
      m_drawingAnno.penWidth = 3;
      QPoint relPos = gpos - m_selection.topLeft();
      if (m_currentTool == Rect || m_currentTool == Arrow) {
        m_drawingAnno.points << relPos << relPos;
      } else {
        m_drawingAnno.points << relPos;
      }
      setCursor(Qt::CrossCursor);
      return;
    }

    // ---- 无标注工具：调整选区（手柄缩放 / 整体移动）----
    if (m_currentTool == None) {
      // 手柄缩放
      int h = handleAt(gpos);
      if (h >= 0) {
        m_activeHandle = h;
        m_dragging = true;
        switch (h) {
          case TopLeft:
            m_resizeAnchor = m_selection.bottomRight();
            break;
          case Top:
            m_resizeAnchor =
                QPoint(m_selection.center().x(), m_selection.bottom());
            break;
          case TopRight:
            m_resizeAnchor = m_selection.bottomLeft();
            break;
          case Right:
            m_resizeAnchor =
                QPoint(m_selection.left(), m_selection.center().y());
            break;
          case BottomRight:
            m_resizeAnchor = m_selection.topLeft();
            break;
          case Bottom:
            m_resizeAnchor =
                QPoint(m_selection.center().x(), m_selection.top());
            break;
          case BottomLeft:
            m_resizeAnchor = m_selection.topRight();
            break;
          case Left:
            m_resizeAnchor =
                QPoint(m_selection.right(), m_selection.center().y());
            break;
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
      // 正在绘制标注
      QPoint relPos = gpos - m_selection.topLeft();
      if (m_drawingAnno.tool == Rect || m_drawingAnno.tool == Arrow) {
        if (m_drawingAnno.points.size() >= 2) {
          m_drawingAnno.points[1] = relPos;
        } else {
          m_drawingAnno.points << relPos;
        }
      } else {
        m_drawingAnno.points << relPos;
      }
      update();
    } else if (m_dragging && m_activeHandle >= 0) {
      // 手柄缩放
      QRect r;
      switch (m_activeHandle) {
        case TopLeft:
          r = QRect(gpos, m_resizeAnchor);
          break;
        case Top:
          r = QRect(QPoint(m_resizeAnchor.x(), gpos.y()),
                    QPoint(gpos.x(), m_resizeAnchor.y()));
          break;
        case TopRight:
          r = QRect(QPoint(m_resizeAnchor.x(), gpos.y()),
                    QPoint(gpos.x(), m_resizeAnchor.y()));
          break;
        case Right:
          r = QRect(QPoint(m_resizeAnchor.x(), m_resizeAnchor.y()),
                    QPoint(gpos.x(), m_selection.bottom()));
          break;
        case BottomRight:
          r = QRect(m_resizeAnchor, gpos);
          break;
        case Bottom:
          r = QRect(QPoint(m_resizeAnchor.x(), m_resizeAnchor.y()),
                    QPoint(gpos.x(), gpos.y()));
          break;
        case BottomLeft:
          r = QRect(QPoint(gpos.x(), m_resizeAnchor.y()),
                    QPoint(m_resizeAnchor.x(), gpos.y()));
          break;
        case Left:
          r = QRect(QPoint(gpos.x(), m_resizeAnchor.y()),
                    QPoint(m_resizeAnchor.x(), gpos.y()));
          break;
      }
      m_selection = r.normalized();
      QRect total = geometry();
      if (m_selection.left() < total.left()) m_selection.setLeft(total.left());
      if (m_selection.top() < total.top()) m_selection.setTop(total.top());
      if (m_selection.right() > total.right())
        m_selection.setRight(total.right());
      if (m_selection.bottom() > total.bottom())
        m_selection.setBottom(total.bottom());
      update();
    } else if (m_movingSelection) {
      // 整体移动——不检查 contains！快速移动时 gpos 可能跳出选区，
      // 但只要 m_movingSelection=true（press 时锁定）就无条件跟随。
      // 约束在虚拟屏内。
      QPoint newTopLeft = gpos - m_dragOffset;
      QRect total = geometry();
      if (newTopLeft.x() < total.left())
        newTopLeft.setX(total.left());
      if (newTopLeft.y() < total.top())
        newTopLeft.setY(total.top());
      if (newTopLeft.x() + m_selection.width() > total.right())
        newTopLeft.setX(total.right() - m_selection.width());
      if (newTopLeft.y() + m_selection.height() > total.bottom())
        newTopLeft.setY(total.bottom() - m_selection.height());
      m_selection.moveTopLeft(newTopLeft);
      update();
    } else {
      // 空闲：设置光标提示
      if (m_currentTool != None) {
        setCursor(Qt::CrossCursor);
      } else {
        setCursorForPos(gpos);
      }
    }
  }
}

void ScreenShotWindow::mouseReleaseEvent(QMouseEvent *event) {
  if (event->button() != Qt::LeftButton) return;

  if (m_phase == Drag) {
    m_dragging = false;
    if (m_selection.isEmpty() || m_selection.width() <= kDragThreshold) {
      // 没有拖拽（或拖拽距离太小）→ 检查是否是单击窗口预选
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
      // 提交正在绘制的标注
      m_drawingActive = false;
      bool valid = false;
      if (m_drawingAnno.tool == Rect || m_drawingAnno.tool == Arrow) {
        if (m_drawingAnno.points.size() >= 2) {
          QPoint s = m_drawingAnno.points[0];
          QPoint e = m_drawingAnno.points[1];
          if (std::abs(e.x() - s.x()) > 2 || std::abs(e.y() - s.y()) > 2) {
            valid = true;
          }
        }
      } else {
        if (m_drawingAnno.points.size() >= 2) {
          valid = true;
        }
      }
      if (valid) {
        m_annotations.append(m_drawingAnno);
      }
      m_drawingAnno = Annotation();
      update();
    }
    // 重置选区调整状态
    m_dragging = false;
    m_movingSelection = false;
    m_activeHandle = -1;
  }
}

void ScreenShotWindow::keyPressEvent(QKeyEvent *event) {
  if (event->key() == Qt::Key_Escape) {
    // 如果正在画标注或选了工具，ESC 先取消工具/绘制，不退出整个截图。
    if (m_drawingActive) {
      m_drawingActive = false;
      m_drawingAnno = Annotation();
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

  // Ctrl+Z 撤销
  if (event->key() == Qt::Key_Z && (event->modifiers() & Qt::ControlModifier)) {
    if (!m_annotations.isEmpty()) {
      m_annotations.removeLast();
      update();
    }
    return;
  }

  // 数字键切换工具（Confirm 阶段）
  if (m_phase == Confirm) {
    Tool newTool = None;
    switch (event->key()) {
      case Qt::Key_1: newTool = Rect; break;
      case Qt::Key_2: newTool = Arrow; break;
      case Qt::Key_3: newTool = Pen; break;
      case Qt::Key_4: newTool = Mosaic; break;
      default: break;
    }
    if (newTool != None) {
      m_currentTool = (m_currentTool == newTool) ? None : newTool;
      update();
    }
  }
}
