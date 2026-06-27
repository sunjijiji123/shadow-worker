#ifndef SINGLEINSTANCE_H
#define SINGLEINSTANCE_H

#include <QObject>

// SingleInstance 负责客户端单例保护。
// 用 Named Mutex 实现：第二个实例启动时检测到 mutex 已存在，直接退出
// 并唤醒已有实例的主窗口。
class SingleInstance : public QObject {
  Q_OBJECT

 public:
  // 尝试获取互斥体。返回 true = 当前是首个实例；
  // false = 已有实例在运行（调用方应退出并唤醒已有实例）。
  static bool tryLock();

  // 第二个实例调用：找到已运行实例的主窗口，恢复并置前台。
  // 用 FindWindowW 按窗口标题 "Shadow Worker" 查找。
  // 已知限制：qsTr 翻译后标题会变，FindWindow 会失效。
  // 当前项目无 .qm 文件，MVP 安全。
  static void activateExistingInstance();

 private:
  explicit SingleInstance(QObject *parent = nullptr) : QObject(parent) {}
};

#endif  // SINGLEINSTANCE_H
