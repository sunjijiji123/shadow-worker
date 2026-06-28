#include "screenshotwindow.h"

#include <QApplication>
#include <QClipboard>
#include <QDateTime>
#include <QDir>
#include <QGuiApplication>
#include <QKeyEvent>
#include <QMouseEvent>
#include <QPainter>
#include <QPainterPath>
#include <QScreen>

ScreenShotWindow::ScreenShotWindow(const QString &saveDir, QWidget *parent)
    : QWidget(parent),
      m_saveDir(saveDir),
      m_phase(Drag),
      m_dragging(false),
      m_activeHandle(-1) {
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

QRect ScreenShotWindow::toolbarRect() const {
  const int btnW = 72;
  const int btnH = 32;
  const int gap = 6;
  const int padding = 4;
  int tw = btnW * 2 + gap + padding * 2;
  int th = btnH + padding * 2;
  int x = m_selection.right() - tw;
  int y = m_selection.bottom() + 6;

  QRect total = geometry();
  if (y + th > total.bottom()) {
    y = m_selection.top() - th - 6;
  }
  if (x < total.left()) x = total.left() + 4;
  if (x + tw > total.right()) x = total.right() - tw - 4;

  return QRect(x, y, tw, th);
}

QRect ScreenShotWindow::confirmBtnRect() const {
  QRect tb = toolbarRect();
  return QRect(tb.left() + 4, tb.top() + 4, 72, 32);
}

QRect ScreenShotWindow::cancelBtnRect() const {
  QRect tb = toolbarRect();
  return QRect(tb.left() + 4 + 72 + 6, tb.top() + 4, 72, 32);
}

void ScreenShotWindow::paintEvent(QPaintEvent *) {
  QPainter p(this);
  p.setRenderHint(QPainter::Antialiasing);

  QPoint origin = geometry().topLeft();

  // 全屏半透明遮罩
  QColor mask(0, 0, 0, 115);
  p.fillRect(rect(), mask);

  if (m_selection.isEmpty()) return;

  // m_selection 是逻辑坐标（虚拟屏全局）。m_fullPixmap 是纯物理像素（不设 dpr）。
  // 选区还原：必须用「带目标矩形」的 drawPixmap 重载（QRect target, QPixmap, QRect source）。
  //
  // 【根因】之前用 drawPixmap(QPoint target, pixmap, QRect source) 这个重载：
  // target 是选区左上角（逻辑坐标），source 是物理坐标（srcSize = 选区×dpr）。
  // 但该重载会按 source 的物理像素尺寸 1:1 画到 widget 逻辑坐标系——
  // 即 500 物理像素被画成 500 逻辑像素，在 125% 下把选区内原图放大了 1.25 倍，
  // 导致原图溢出选区边界（你看到的"显示有问题"）。
  //
  // 修复：目标矩形用逻辑坐标（选区尺寸），源矩形用物理坐标，Qt 自动按 widget
  // 的 dpr 在两者间正确缩放。
  qreal dpr = devicePixelRatioF();
  QPoint localTopLeft = m_selection.topLeft() - origin;
  QSize logicalSize = m_selection.size();           // 逻辑尺寸（目标）
  QRect targetRect(localTopLeft, logicalSize);       // 目标：逻辑坐标
  QPoint srcOffset = (m_selection.topLeft() - origin) * dpr;
  QSize srcSize = m_selection.size() * dpr;          // 物理尺寸（源）
  QRect sourceRect(srcOffset, srcSize);              // 源：物理坐标
  p.setCompositionMode(QPainter::CompositionMode_SourceOver);
  p.drawPixmap(targetRect, m_fullPixmap, sourceRect);

  // 白色虚线边框（widget 本地坐标）
  QPen pen(Qt::white, 2, Qt::DashLine);
  p.setPen(pen);
  p.setBrush(Qt::NoBrush);
  p.drawRect(QRect(localTopLeft, m_selection.size()));

  if (m_phase == Confirm) {
    // 8 个调整手柄（全局→本地）
    p.setBrush(Qt::white);
    p.setPen(Qt::NoPen);
    for (int i = TopLeft; i <= Left; ++i) {
      QRect h = handleRect(static_cast<HandlePos>(i));
      h.translate(-origin);
      p.drawRect(h);
    }

    // 工具条（全局→本地）
    QRect tb = toolbarRect();
    tb.translate(-origin);
    QPainterPath tbPath;
    tbPath.addRoundedRect(tb.x(), tb.y(), tb.width(), tb.height(), 6, 6);
    p.setCompositionMode(QPainter::CompositionMode_SourceOver);
    p.fillPath(tbPath, QColor(37, 37, 37, 235));

    // 确认按钮（绿）
    QRect btnC = confirmBtnRect();
    btnC.translate(-origin);
    QPainterPath cp;
    cp.addRoundedRect(btnC.x(), btnC.y(), btnC.width(), btnC.height(), 4, 4);
    p.fillPath(cp, QColor(76, 175, 80));

    // 取消按钮（红）
    QRect btnX = cancelBtnRect();
    btnX.translate(-origin);
    QPainterPath xp;
    xp.addRoundedRect(btnX.x(), btnX.y(), btnX.width(), btnX.height(), 4, 4);
    p.fillPath(xp, QColor(244, 67, 54));

    // 按钮文字
    QFont f = p.font();
    f.setPixelSize(13);
    p.setFont(f);
    p.setPen(Qt::white);
    p.drawText(btnC, Qt::AlignCenter, QString::fromUtf8("\u2713 \u5B8C\u6210"));
    p.drawText(btnX, Qt::AlignCenter, QString::fromUtf8("\u2717 \u53D6\u6D88"));
  }
}

void ScreenShotWindow::mousePressEvent(QMouseEvent *event) {
  // Qt6: position()/globalPosition() 替代废弃的 pos()/globalPos()
  QPoint gpos = event->globalPosition().toPoint();
  QPoint lpos = event->position().toPoint();

  if (event->button() == Qt::RightButton) {
    emit cancelled();
    close();
    return;
  }

  if (event->button() != Qt::LeftButton) return;

  if (m_phase == Drag) {
    m_startPoint = gpos;
    m_selection = QRect();
    m_dragging = true;
  } else {
    QPoint origin = geometry().topLeft();
    // Confirm 阶段：先检查工具条按钮（转本地坐标）
    if (confirmBtnRect().translated(-origin).contains(lpos)) {
      QDir().mkpath(m_saveDir);
      QString name = "screenshot_" +
                     QDateTime::currentDateTime().toString("yyyyMMdd_HHmmss") +
                     ".png";
      QString filePath = m_saveDir + "/" + name;

      QRect total = geometry();
      qreal dpr = devicePixelRatioF();
      // m_fullPixmap 是纯物理像素（不设 devicePixelRatio），故 copy() 用物理坐标。
      // 选区 m_selection 是逻辑坐标（虚拟屏全局），转换：offset = 选区相对虚拟屏
      // 左上角的逻辑偏移 × dpr；size = 选区逻辑尺寸 × dpr。这样裁出的就是用户
      // 框选的那块物理像素图像，100%/125%/150% 缩放下都正确。
      QPoint offset = (m_selection.topLeft() - total.topLeft()) * dpr;
      QSize size = m_selection.size() * dpr;
      QRect deviceRect(offset, size);
      QPixmap crop = m_fullPixmap.copy(deviceRect);

      crop.save(filePath, "PNG");

      QClipboard *cb = QApplication::clipboard();
      cb->setPixmap(crop);
      cb->setText(filePath);

      emit finished(filePath);
      close();
      return;
    }
    if (cancelBtnRect().translated(-origin).contains(lpos)) {
      emit cancelled();
      close();
      return;
    }

    // 手柄缩放（handleAt 用全局坐标）
    int h = handleAt(gpos);
    if (h >= 0) {
      m_activeHandle = h;
      m_dragging = true;
      // 锚点设为对角/对边
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
      m_dragOffset = gpos - m_selection.topLeft();
      return;
    }
  }
}

void ScreenShotWindow::mouseMoveEvent(QMouseEvent *event) {
  QPoint gpos = event->globalPosition().toPoint();

  if (m_phase == Drag && m_dragging) {
    m_selection = QRect(m_startPoint, gpos).normalized();
    update();
  } else if (m_phase == Confirm && m_dragging) {
    if (m_activeHandle >= 0) {
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
      // 约束在虚拟屏内
      QRect total = geometry();
      if (m_selection.left() < total.left()) m_selection.setLeft(total.left());
      if (m_selection.top() < total.top()) m_selection.setTop(total.top());
      if (m_selection.right() > total.right())
        m_selection.setRight(total.right());
      if (m_selection.bottom() > total.bottom())
        m_selection.setBottom(total.bottom());
      update();
    } else if (m_selection.contains(gpos)) {
      // 整体移动
      m_selection.moveTopLeft(gpos - m_dragOffset);
      update();
    }
  } else if (m_phase == Confirm) {
    setCursorForPos(gpos);
  }
}

void ScreenShotWindow::mouseReleaseEvent(QMouseEvent *event) {
  if (event->button() != Qt::LeftButton) return;
  m_dragging = false;
  m_activeHandle = -1;

  // Drag 阶段松开且选区足够大 → 进入 Confirm
  if (m_phase == Drag && !m_selection.isEmpty() && m_selection.width() > 5 &&
      m_selection.height() > 5) {
    m_phase = Confirm;
    setCursor(Qt::ArrowCursor);
    update();
  }
}

void ScreenShotWindow::keyPressEvent(QKeyEvent *event) {
  if (event->key() == Qt::Key_Escape) {
    emit cancelled();
    close();
  }
}
