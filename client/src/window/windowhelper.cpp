#include "windowhelper.h"

#include <QGuiApplication>
#include <QScreen>
#include <QVariantMap>

WindowHelper::WindowHelper(QObject *parent) : QObject(parent) {}

QVariantMap WindowHelper::primaryScreenGeometry() const {
  QVariantMap m;
  QScreen *s = QGuiApplication::primaryScreen();
  if (!s) return m;
  QRect g = s->geometry();
  m["x"] = g.x();
  m["y"] = g.y();
  m["width"] = g.width();
  m["height"] = g.height();
  return m;
}

QVariantMap WindowHelper::primaryAvailableGeometry() const {
  QVariantMap m;
  QScreen *s = QGuiApplication::primaryScreen();
  if (!s) return m;
  // availableGeometry() = work area (taskbar excluded).
  QRect g = s->availableGeometry();
  m["x"] = g.x();
  m["y"] = g.y();
  m["width"] = g.width();
  m["height"] = g.height();
  return m;
}

qreal WindowHelper::moveToBottomCenter(QQuickWindow *window, qreal gap) {
  if (!window) return 0.0;
  QScreen *s = QGuiApplication::primaryScreen();
  if (!s) return 0.0;
  QRect wa = s->availableGeometry();
  if (wa.isEmpty()) return 0.0;

  const qreal w = window->width();
  const qreal h = window->height();
  // horizontal center of the work area
  const qreal x = wa.x() + (wa.width() - w) / 2.0;
  // bottom edge: work-area bottom minus the gap
  const qreal bottomEdgeY = wa.y() + wa.height() - gap;
  const qreal y = bottomEdgeY - h;

  window->setX(static_cast<int>(std::round(x)));
  window->setY(static_cast<int>(std::round(y)));
  return bottomEdgeY;
}

void WindowHelper::reanchorBottom(QQuickWindow *window, qreal bottomEdgeY) {
  if (!window) return;
  const qreal y = bottomEdgeY - window->height();
  window->setY(static_cast<int>(std::round(y)));
}

// ---- pinned bottom edge (C++ signal connection) ----
namespace {
// track the connection + state so we can disconnect cleanly
QMetaObject::Connection g_pinConn;
QQuickWindow *g_pinnedWindow = nullptr;
qreal g_pinnedBottomY = 0.0;

void reanchorPinned() {
  if (!g_pinnedWindow) return;
  const qreal y = g_pinnedBottomY - g_pinnedWindow->height();
  g_pinnedWindow->setY(static_cast<int>(std::round(y)));
}
}  // namespace

void WindowHelper::pinBottomEdge(QQuickWindow *window, qreal bottomEdgeY) {
  // disconnect any previous pin
  if (g_pinConn) {
    QObject::disconnect(g_pinConn);
    g_pinConn = QMetaObject::Connection();
  }
  g_pinnedWindow = window;
  g_pinnedBottomY = bottomEdgeY;
  if (!window) return;
  // reanchor now, then on every height change (synchronous in C++).
  reanchorPinned();
  g_pinConn =
      QObject::connect(window, &QQuickWindow::heightChanged, []() {
        reanchorPinned();
      });
}

void WindowHelper::updatePinnedBottomEdge(qreal bottomEdgeY) {
  g_pinnedBottomY = bottomEdgeY;
  reanchorPinned();
}

void WindowHelper::unpinBottomEdge() {
  if (g_pinConn) {
    QObject::disconnect(g_pinConn);
    g_pinConn = QMetaObject::Connection();
  }
  g_pinnedWindow = nullptr;
}

bool WindowHelper::startDrag(QQuickWindow *window) {
  if (!window) return false;
  // startSystemMove() hands the drag to the OS window manager. Must be called
  // immediately on mouse press (before any movement) to take effect.
  return window->startSystemMove();
}
