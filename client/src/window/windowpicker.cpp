#include "windowpicker.h"

#include <QDebug>
#include <QFileInfo>
#include <qt_windows.h>

WindowPicker::WindowPicker(QObject *parent) : QObject(parent) {
  m_timer = new QTimer(this);
  m_timer->setSingleShot(true);
  connect(m_timer, &QTimer::timeout, this, &WindowPicker::finishPick);
}

void WindowPicker::pick() {
  if (m_picking)
    return;
  m_picking = true;
  emit pickingChanged();
  qDebug() << "WindowPicker: 请在 3 秒内切换到目标窗口";
  m_timer->start(3000);
}

void WindowPicker::finishPick() {
  m_picking = false;
  emit pickingChanged();

  HWND hwnd = GetForegroundWindow();
  if (!hwnd) {
    emit cancelled();
    return;
  }

  DWORD pid = 0;
  GetWindowThreadProcessId(hwnd, &pid);
  if (pid == 0) {
    emit cancelled();
    return;
  }

  HANDLE hProc = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid);
  if (!hProc) {
    emit cancelled();
    return;
  }

  WCHAR pathBuf[MAX_PATH] = {0};
  DWORD size = MAX_PATH;
  BOOL ok = QueryFullProcessImageNameW(hProc, 0, pathBuf, &size);
  CloseHandle(hProc);

  if (!ok) {
    emit cancelled();
    return;
  }

  WCHAR titleBuf[512] = {0};
  GetWindowTextW(hwnd, titleBuf, 512);

  const QString path = QString::fromWCharArray(pathBuf);
  const QString title = QString::fromWCharArray(titleBuf);
  const QString name = QFileInfo(path).fileName();

  qDebug() << "WindowPicker picked:" << path << name << title;
  emit picked(path, name, title);
}
