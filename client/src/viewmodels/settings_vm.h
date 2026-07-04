// SettingsViewModel: 设置页 ↔ ConfigService gRPC 桥

#pragma once

#include <QObject>
#include <QString>
#include <QVariantList>
#include <QVariantMap>
#include <memory>

#include "config.qpb.h"
#include "config_client.grpc.qpb.h"
#include <QAbstractGrpcChannel>

class SettingsViewModel : public QObject {
  Q_OBJECT
  Q_PROPERTY(bool loading READ loading NOTIFY loadingChanged)
  Q_PROPERTY(QString error READ error NOTIFY errorChanged)

  // ASR
  Q_PROPERTY(
      QString asrMode READ asrMode WRITE setAsrMode NOTIFY asrModeChanged)
  Q_PROPERTY(QString asrActiveProvider READ asrActiveProvider WRITE
                 setAsrActiveProvider NOTIFY asrActiveProviderChanged)
  Q_PROPERTY(
      QVariantList asrProviders READ asrProviders NOTIFY asrProvidersChanged)
  Q_PROPERTY(QString asrLocalModelPath READ asrLocalModelPath WRITE
                 setAsrLocalModelPath NOTIFY asrLocalModelPathChanged)
  Q_PROPERTY(QString asrLocalModelName READ asrLocalModelName WRITE
                 setAsrLocalModelName NOTIFY asrLocalModelNameChanged)
  Q_PROPERTY(QString asrLocalLanguage READ asrLocalLanguage WRITE
                 setAsrLocalLanguage NOTIFY asrLocalLanguageChanged)
  // recording trigger mode: hold | press
  Q_PROPERTY(QString recordMode READ recordMode WRITE setRecordMode NOTIFY
                 recordModeChanged)

  // VLM
  Q_PROPERTY(
      QString vlmMode READ vlmMode WRITE setVlmMode NOTIFY vlmModeChanged)
  Q_PROPERTY(QString vlmActiveProvider READ vlmActiveProvider WRITE
                 setVlmActiveProvider NOTIFY vlmActiveProviderChanged)
  Q_PROPERTY(
      QVariantList vlmProviders READ vlmProviders NOTIFY vlmProvidersChanged)
  Q_PROPERTY(int vlmInterval READ vlmInterval WRITE setVlmInterval NOTIFY
                 vlmIntervalChanged)
  Q_PROPERTY(QString vlmCaptureRange READ vlmCaptureRange WRITE
                 setVlmCaptureRange NOTIFY vlmCaptureRangeChanged)
  Q_PROPERTY(int vlmSwitchGap READ vlmSwitchGap WRITE setVlmSwitchGap NOTIFY
                 vlmSwitchGapChanged)
  Q_PROPERTY(int vlmMotionGap READ vlmMotionGap WRITE setVlmMotionGap NOTIFY
                 vlmMotionGapChanged)
  Q_PROPERTY(QString vlmPrompt READ vlmPrompt WRITE setVlmPrompt NOTIFY
                 vlmPromptChanged)

  // LLM / Polish
  Q_PROPERTY(bool llmEnabled READ llmEnabled WRITE setLlmEnabled NOTIFY
                 llmEnabledChanged)
  Q_PROPERTY(QString llmActiveProvider READ llmActiveProvider WRITE
                 setLlmActiveProvider NOTIFY llmActiveProviderChanged)
  Q_PROPERTY(
      QVariantList llmProviders READ llmProviders NOTIFY llmProvidersChanged)
  Q_PROPERTY(QString llmPrompt READ llmPrompt WRITE setLlmPrompt NOTIFY
                 llmPromptChanged)
  Q_PROPERTY(QString llmInjectMode READ llmInjectMode WRITE setLlmInjectMode
                 NOTIFY llmInjectModeChanged)

  // Movement
  Q_PROPERTY(int movementSampleMs READ movementSampleMs WRITE
                 setMovementSampleMs NOTIFY movementSampleMsChanged)
  Q_PROPERTY(int movementIdleS READ movementIdleS WRITE setMovementIdleS NOTIFY
                 movementIdleSChanged)
  Q_PROPERTY(QString movementPrecision READ movementPrecision WRITE
                 setMovementPrecision NOTIFY movementPrecisionChanged)

  // Hotkeys
  Q_PROPERTY(QString hotkeyRecord READ hotkeyRecord WRITE setHotkeyRecord NOTIFY
                 hotkeyRecordChanged)
  Q_PROPERTY(QString hotkeyScreenshot READ hotkeyScreenshot WRITE
                 setHotkeyScreenshot NOTIFY hotkeyScreenshotChanged)
  Q_PROPERTY(QString hotkeyPromptPrefix READ hotkeyPromptPrefix WRITE
                 setHotkeyPromptPrefix NOTIFY hotkeyPromptPrefixChanged)

  // Screenshot tool: 截图完成后是否自动触发 VLM 分析（写时间线事件）。
  Q_PROPERTY(bool screenshotWithVlm READ screenshotWithVlm WRITE
                 setScreenshotWithVlm NOTIFY screenshotWithVlmChanged)
  // Screenshot tool: 桌面截图识别专用提示词（与 vlmPrompt 区分）。
  Q_PROPERTY(QString screenshotPrompt READ screenshotPrompt WRITE
                 setScreenshotPrompt NOTIFY screenshotPromptChanged)

  // Update
  Q_PROPERTY(QString updateServerUrl READ updateServerUrl WRITE
                 setUpdateServerUrl NOTIFY updateServerUrlChanged)
  Q_PROPERTY(bool updateCheckOnStartup READ updateCheckOnStartup WRITE
                 setUpdateCheckOnStartup NOTIFY updateCheckOnStartupChanged)
  Q_PROPERTY(bool updateCheckDaily READ updateCheckDaily WRITE
                 setUpdateCheckDaily NOTIFY updateCheckDailyChanged)
  Q_PROPERTY(QString updateChannel READ updateChannel WRITE
                 setUpdateChannel NOTIFY updateChannelChanged)

public:
  explicit SettingsViewModel(QObject *parent = nullptr);

  void setChannel(std::shared_ptr<QAbstractGrpcChannel> channel);

  bool loading() const { return m_loading; }
  QString error() const { return m_error; }

  // ASR
  QString asrMode() const { return m_asrMode; }
  void setAsrMode(const QString &v);
  QString asrActiveProvider() const { return m_asrActiveProvider; }
  void setAsrActiveProvider(const QString &v);
  QVariantList asrProviders() const { return m_asrProviders; }
  QString asrLocalModelPath() const { return m_asrLocalModelPath; }
  void setAsrLocalModelPath(const QString &v);
  QString asrLocalModelName() const { return m_asrLocalModelName; }
  void setAsrLocalModelName(const QString &v);
  QString asrLocalLanguage() const { return m_asrLocalLanguage; }
  void setAsrLocalLanguage(const QString &v);
  QString recordMode() const { return m_recordMode; }
  void setRecordMode(const QString &v);

  // VLM
  QString vlmMode() const { return m_vlmMode; }
  void setVlmMode(const QString &v);
  QString vlmActiveProvider() const { return m_vlmActiveProvider; }
  void setVlmActiveProvider(const QString &v);
  QVariantList vlmProviders() const { return m_vlmProviders; }
  int vlmInterval() const { return m_vlmInterval; }
  void setVlmInterval(int v);
  QString vlmCaptureRange() const { return m_vlmCaptureRange; }
  void setVlmCaptureRange(const QString &v);
  int vlmSwitchGap() const { return m_vlmSwitchGap; }
  void setVlmSwitchGap(int v);
  int vlmMotionGap() const { return m_vlmMotionGap; }
  void setVlmMotionGap(int v);
  QString vlmPrompt() const { return m_vlmPrompt; }
  void setVlmPrompt(const QString &v);

  // LLM
  bool llmEnabled() const { return m_llmEnabled; }
  void setLlmEnabled(bool v);
  QString llmActiveProvider() const { return m_llmActiveProvider; }
  void setLlmActiveProvider(const QString &v);
  QVariantList llmProviders() const { return m_llmProviders; }
  QString llmPrompt() const { return m_llmPrompt; }
  void setLlmPrompt(const QString &v);
  QString llmInjectMode() const { return m_llmInjectMode; }
  void setLlmInjectMode(const QString &v);

  // Movement
  int movementSampleMs() const { return m_movementSampleMs; }
  void setMovementSampleMs(int v);
  int movementIdleS() const { return m_movementIdleS; }
  void setMovementIdleS(int v);
  QString movementPrecision() const { return m_movementPrecision; }
  void setMovementPrecision(const QString &v);

  // Hotkeys
  QString hotkeyRecord() const { return m_hotkeyRecord; }
  void setHotkeyRecord(const QString &v);
  QString hotkeyScreenshot() const { return m_hotkeyScreenshot; }
  void setHotkeyScreenshot(const QString &v);
  QString hotkeyPromptPrefix() const { return m_hotkeyPromptPrefix; }
  void setHotkeyPromptPrefix(const QString &v);

  // Screenshot
  bool screenshotWithVlm() const { return m_screenshotWithVlm; }
  void setScreenshotWithVlm(bool v);
  QString screenshotPrompt() const { return m_screenshotPrompt; }
  void setScreenshotPrompt(const QString &v);

  // Update
  QString updateServerUrl() const { return m_updateServerUrl; }
  void setUpdateServerUrl(const QString &v);
  bool updateCheckOnStartup() const { return m_updateCheckOnStartup; }
  void setUpdateCheckOnStartup(bool v);
  bool updateCheckDaily() const { return m_updateCheckDaily; }
  void setUpdateCheckDaily(bool v);
  QString updateChannel() const { return m_updateChannel; }
  void setUpdateChannel(const QString &v);

  Q_INVOKABLE void addProvider(const QString &category, const QString &key);
  Q_INVOKABLE void removeProvider(const QString &category, const QString &key);
  Q_INVOKABLE void updateProvider(const QString &category, const QString &key,
                                  const QVariantMap &data);
  Q_INVOKABLE void setActiveProvider(const QString &category,
                                     const QString &key);

  Q_INVOKABLE void load();
  Q_INVOKABLE void save();

signals:
  void loadingChanged();
  void errorChanged();

  void asrModeChanged();
  void asrActiveProviderChanged();
  void asrProvidersChanged();
  void asrLocalModelPathChanged();
  void asrLocalModelNameChanged();
  void asrLocalLanguageChanged();
  void recordModeChanged();

  void vlmModeChanged();
  void vlmActiveProviderChanged();
  void vlmProvidersChanged();
  void vlmIntervalChanged();
  void vlmCaptureRangeChanged();
  void vlmSwitchGapChanged();
  void vlmMotionGapChanged();
  void vlmPromptChanged();

  void llmEnabledChanged();
  void llmActiveProviderChanged();
  void llmProvidersChanged();
  void llmPromptChanged();
  void llmInjectModeChanged();

  void movementSampleMsChanged();
  void movementIdleSChanged();
  void movementPrecisionChanged();

  void hotkeyRecordChanged();
  void hotkeyScreenshotChanged();
  void hotkeyPromptPrefixChanged();

  void screenshotWithVlmChanged();
  void screenshotPromptChanged();

  void updateServerUrlChanged();
  void updateCheckOnStartupChanged();
  void updateCheckDailyChanged();
  void updateChannelChanged();

  void loadFinished(bool ok);
  void saveFinished(bool ok, const QString &error);

private:
  void setLoading(bool v);
  void setError(const QString &e);
  void applyConfig(const shadowworker::ConfigData &data);
  shadowworker::ConfigData buildConfig() const;

  QVariantList *providerListRef(const QString &category);
  const QVariantList providerList(const QString &category) const;
  void setProviderList(const QString &category, const QVariantList &list);
  void emitProvidersChanged(const QString &category);

  shadowworker::ConfigService::Client m_client;
  std::shared_ptr<QAbstractGrpcChannel> m_channel;

  bool m_loading = false;
  QString m_error;

  QString m_asrMode;
  QString m_asrActiveProvider;
  QVariantList m_asrProviders;
  QString m_asrLocalModelPath;
  QString m_asrLocalModelName;
  QString m_asrLocalLanguage;
  QString m_recordMode = "hold";

  QString m_vlmMode;
  QString m_vlmActiveProvider;
  QVariantList m_vlmProviders;
  int m_vlmInterval = 5;
  QString m_vlmCaptureRange = QStringLiteral("active");
  int m_vlmSwitchGap = 20;  // on-demand: 切窗口触发冷却秒
  int m_vlmMotionGap = 60;  // on-demand: 活跃点触发冷却秒
  QString m_vlmPrompt;

  bool m_llmEnabled = false;
  QString m_llmActiveProvider;
  QVariantList m_llmProviders;
  QString m_llmPrompt;
  QString m_llmInjectMode;

  int m_movementSampleMs = 300;
  int m_movementIdleS = 10;
  QString m_movementPrecision = "medium";

  QString m_hotkeyRecord = "F9";
  QString m_hotkeyScreenshot;
  QString m_hotkeyPromptPrefix = "Ctrl";
  bool m_screenshotWithVlm = false;
  QString m_screenshotPrompt;

  QString m_updateServerUrl;
  bool m_updateCheckOnStartup = true;
  bool m_updateCheckDaily = true;
  QString m_updateChannel = QStringLiteral("stable");
};
