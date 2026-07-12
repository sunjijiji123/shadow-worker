#pragma once

#include <QQuickWindow>
#include <QObject>
#include <QPoint>
#include <QSize>
#include <qqmlintegration.h>

// WindowHelper exposes reliable screen geometry + native window move/drag to
// QML. Used by the recording overlay window (RecordingWindow.qml), where the
// QML attached `Screen` object is unreliable right after the window becomes
// visible and `startSystemMove()` must be called from C++ at press time.
//
// All methods are Q_INVOKABLE so they can be called directly from QML JS.
class WindowHelper : public QObject {
  Q_OBJECT
  QML_ELEMENT

public:
  explicit WindowHelper(QObject *parent = nullptr);

  // Primary screen's full geometry (includes taskbar area).
  // Returns the top-left + size as a QRect exposed via QVariantMap.
  Q_INVOKABLE QVariantMap primaryScreenGeometry() const;

  // Primary screen's available geometry (work area, taskbar excluded).
  // This is the reliable equivalent of QML's Screen.availableGeometry.
  Q_INVOKABLE QVariantMap primaryAvailableGeometry() const;

  // Position a window bottom-centered in the primary screen's work area,
  // leaving `gap` pixels above the taskbar. The window grows upward from
  // there (call reanchorBottom on height changes to keep the bottom fixed).
  // Returns the absolute Y of the window's bottom edge (for reanchoring).
  Q_INVOKABLE qreal moveToBottomCenter(QQuickWindow *window, qreal gap = 24.0);

  // Keep the window's bottom edge fixed at bottomEdgeY while height changes.
  Q_INVOKABLE void reanchorBottom(QQuickWindow *window, qreal bottomEdgeY);

  // Pin the window's bottom edge to bottomEdgeY and install a C++ connection on
  // its heightChanged signal so that EVERY height change (across any number of
  // layout frames) immediately recomputes y = bottomEdgeY - height, keeping the
  // bottom edge pixel-fixed. Call this once after initial positioning.
  // Pass updatedBottomEdgeY via updatePinnedBottomEdge() if the user drags.
  Q_INVOKABLE void pinBottomEdge(QQuickWindow *window, qreal bottomEdgeY);

  // Update the pinned bottom-edge Y without reinstalling the connection.
  Q_INVOKABLE void updatePinnedBottomEdge(qreal bottomEdgeY);

  // Stop the automatic bottom-edge pinning (e.g. when the user drags).
  Q_INVOKABLE void unpinBottomEdge();

  // Begin a native OS window drag (frameless drag). MUST be called on press.
  // Returns true if the drag was handed to the OS.
  Q_INVOKABLE bool startDrag(QQuickWindow *window);

  // Begin a native OS window resize from the given edges.
  // edges is a bitmask of Qt::Edge values (1=Top,2=Left,4=Right,8=Bottom).
  Q_INVOKABLE bool startResize(QQuickWindow *window, int edges);
};
