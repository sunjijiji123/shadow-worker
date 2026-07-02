#ifndef COLLECTIONCLIENT_H
#define COLLECTIONCLIENT_H

#include <QObject>
#include <QString>
#include <memory>

#include "collection.qpb.h"
#include "collection_client.grpc.qpb.h"
#include <QAbstractGrpcChannel>

// CollectionClient: 调用后端 CollectionService 的轻量 gRPC 客户端。
//
// 目前仅封装 TriggerVLM（手动触发一次 VLM 截图理解），用于"截图后可选
// AI 分析"开关 —— 用户在设置页开启后，区域截图完成时调用本类的
// triggerVLM()，让后端对当前屏幕做一次 VLM 摘要并写时间线事件。
//
// 暴露为 context property `collectionClient`。
class CollectionClient : public QObject {
  Q_OBJECT
 public:
  explicit CollectionClient(QObject *parent = nullptr);

  void setChannel(std::shared_ptr<QAbstractGrpcChannel> channel);

  // 手动触发一次 VLM 截图理解。后端按当前 vlm_capture_range 截整屏或活动
  // 窗口 + 白名单过滤（活动窗口模式下非白名单会静默跳过，summary 为空）。
  // VLM 未启用时后端返回 error。结果经 vlmSummaryReady 信号发出。
  Q_INVOKABLE void triggerVLM();

  // 分析指定路径的 PNG 截图（"快捷工具-桌面截图"框选结果）。后端直接对这张
  // 图做 VLM 分析，不重新截图——保证"用户框选什么"和"VLM 分析什么"一致。
  // prompt 是桌面截图识别专用提示词（空=后端引擎回落默认），与全局 VLM 提示词区分。
  // 结果经 vlmSummaryReady 信号发出（summary 字段带路径）。
  Q_INVOKABLE void analyzeImage(const QString &path, const QString &prompt);

 signals:
  // VLM 触发完成。summary 为摘要文本（失败或被白名单跳过时可能为空），
  // error 非空表示失败（含"VLM 未启用"）。
  void vlmSummaryReady(const QString &summary, const QString &error);

  // AnalyzeImage 完成。imagePath 是被分析的截图路径（供 QML 显示缩略图），
  // summary 是 VLM 摘要，error 非空表示失败。独立信号，避免与
  // vlmSummaryReady 的 error 参数语义混淆（坑 #15 信号复用副作用）。
  void imageAnalyzed(const QString &imagePath, const QString &summary,
                     const QString &error);

 private:
  shadowworker::CollectionService::Client m_client;
  std::shared_ptr<QAbstractGrpcChannel> m_channel;
};

#endif  // COLLECTIONCLIENT_H
