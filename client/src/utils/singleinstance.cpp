#include "singleinstance.h"

#include <windows.h>

bool SingleInstance::tryLock() {
  // CreateMutexW: 第二个实例创建同名 mutex 时，GetLastError 返回
  // ERROR_ALREADY_EXISTS。Local\ 前缀确保会话级单例（非管理员可用）。
  HANDLE h = CreateMutexW(nullptr, FALSE, L"Shadow-Worker-Client");
  if (GetLastError() == ERROR_ALREADY_EXISTS) {
    if (h) CloseHandle(h);
    return false;
  }
  // h 不主动关闭——进程退出内核自动回收。
  return true;
}

void SingleInstance::activateExistingInstance() {
  // 用窗口标题查找（main.qml:21 title: qsTr("Shadow Worker")）。
  // 已知限制：接入 .qm 翻译后标题会变，FindWindow 会失效。
  // 当前无 .qm 文件，MVP 安全。后续可改用窗口类名。
  HWND hwnd = FindWindowW(nullptr, L"Shadow Worker");
  if (hwnd) {
    ShowWindow(hwnd, SW_RESTORE);
    SetForegroundWindow(hwnd);
  }
}
