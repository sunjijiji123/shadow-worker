// SettingsViewModel 实现

#include "settings_vm.h"

#include <QDebug>
#include <QGrpcCallReply>
#include <QGrpcStatus>
#include <QHash>
#include <algorithm>

using shadowworker::ConfigData;
using shadowworker::GetConfigRequest;
using shadowworker::ProviderConfig;
using shadowworker::Result;

SettingsViewModel::SettingsViewModel(QObject *parent) : QObject(parent) {}

void SettingsViewModel::setChannel(
    std::shared_ptr<QAbstractGrpcChannel> channel) {
  m_channel = std::move(channel);
  m_client.attachChannel(m_channel);
}

// ASR setters
void SettingsViewModel::setAsrMode(const QString &v) {
  if (m_asrMode == v)
    return;
  m_asrMode = v;
  emit asrModeChanged();
}

void SettingsViewModel::setAsrActiveProvider(const QString &v) {
  if (m_asrActiveProvider == v)
    return;
  m_asrActiveProvider = v;
  emit asrActiveProviderChanged();
}

void SettingsViewModel::setAsrLocalModelPath(const QString &v) {
  if (m_asrLocalModelPath == v)
    return;
  m_asrLocalModelPath = v;
  emit asrLocalModelPathChanged();
}

void SettingsViewModel::setAsrLocalModelName(const QString &v) {
  if (m_asrLocalModelName == v)
    return;
  m_asrLocalModelName = v;
  emit asrLocalModelNameChanged();
}

void SettingsViewModel::setAsrLocalLanguage(const QString &v) {
  if (m_asrLocalLanguage == v)
    return;
  m_asrLocalLanguage = v;
  emit asrLocalLanguageChanged();
}

void SettingsViewModel::setRecordMode(const QString &v) {
  if (m_recordMode == v)
    return;
  m_recordMode = v;
  emit recordModeChanged();
}

// VLM setters
void SettingsViewModel::setVlmMode(const QString &v) {
  if (m_vlmMode == v)
    return;
  m_vlmMode = v;
  emit vlmModeChanged();
}

void SettingsViewModel::setVlmActiveProvider(const QString &v) {
  if (m_vlmActiveProvider == v)
    return;
  m_vlmActiveProvider = v;
  emit vlmActiveProviderChanged();
}

void SettingsViewModel::setVlmInterval(int v) {
  if (m_vlmInterval == v)
    return;
  m_vlmInterval = v;
  emit vlmIntervalChanged();
}

void SettingsViewModel::setVlmCaptureRange(const QString &v) {
  if (m_vlmCaptureRange == v)
    return;
  m_vlmCaptureRange = v;
  emit vlmCaptureRangeChanged();
}

void SettingsViewModel::setVlmSwitchGap(int v) {
  if (m_vlmSwitchGap == v)
    return;
  m_vlmSwitchGap = v;
  emit vlmSwitchGapChanged();
}

void SettingsViewModel::setVlmMotionGap(int v) {
  if (m_vlmMotionGap == v)
    return;
  m_vlmMotionGap = v;
  emit vlmMotionGapChanged();
}

void SettingsViewModel::setVlmPrompt(const QString &v) {
  if (m_vlmPrompt == v)
    return;
  m_vlmPrompt = v;
  emit vlmPromptChanged();
}

// LLM setters
void SettingsViewModel::setLlmEnabled(bool v) {
  if (m_llmEnabled == v)
    return;
  m_llmEnabled = v;
  emit llmEnabledChanged();
}

void SettingsViewModel::setLlmActiveProvider(const QString &v) {
  if (m_llmActiveProvider == v)
    return;
  m_llmActiveProvider = v;
  emit llmActiveProviderChanged();
}

void SettingsViewModel::setLlmPrompt(const QString &v) {
  if (m_llmPrompt == v)
    return;
  m_llmPrompt = v;
  emit llmPromptChanged();
}

void SettingsViewModel::setLlmInjectMode(const QString &v) {
  if (m_llmInjectMode == v)
    return;
  m_llmInjectMode = v;
  emit llmInjectModeChanged();
}

// Movement setters
void SettingsViewModel::setMovementSampleMs(int v) {
  if (m_movementSampleMs == v)
    return;
  m_movementSampleMs = v;
  emit movementSampleMsChanged();
}

void SettingsViewModel::setMovementIdleS(int v) {
  if (m_movementIdleS == v)
    return;
  m_movementIdleS = v;
  emit movementIdleSChanged();
}

void SettingsViewModel::setMovementPrecision(const QString &v) {
  if (m_movementPrecision == v)
    return;
  m_movementPrecision = v;
  emit movementPrecisionChanged();
}

// Hotkey setters
void SettingsViewModel::setHotkeyRecord(const QString &v) {
  if (m_hotkeyRecord == v)
    return;
  m_hotkeyRecord = v;
  emit hotkeyRecordChanged();
}

void SettingsViewModel::setHotkeyScreenshot(const QString &v) {
  if (m_hotkeyScreenshot == v)
    return;
  m_hotkeyScreenshot = v;
  emit hotkeyScreenshotChanged();
}

void SettingsViewModel::setHotkeyPromptPrefix(const QString &v) {
  if (m_hotkeyPromptPrefix == v)
    return;
  m_hotkeyPromptPrefix = v;
  emit hotkeyPromptPrefixChanged();
}

void SettingsViewModel::setScreenshotWithVlm(bool v) {
  if (m_screenshotWithVlm == v)
    return;
  m_screenshotWithVlm = v;
  emit screenshotWithVlmChanged();
}

void SettingsViewModel::setScreenshotPrompt(const QString &v) {
  if (m_screenshotPrompt == v)
    return;
  m_screenshotPrompt = v;
  emit screenshotPromptChanged();
}

// Update setters
void SettingsViewModel::setUpdateServerUrl(const QString &v) {
  if (m_updateServerUrl == v)
    return;
  m_updateServerUrl = v;
  emit updateServerUrlChanged();
}

void SettingsViewModel::setUpdateCheckOnStartup(bool v) {
  if (m_updateCheckOnStartup == v)
    return;
  m_updateCheckOnStartup = v;
  emit updateCheckOnStartupChanged();
}

void SettingsViewModel::setUpdateCheckDaily(bool v) {
  if (m_updateCheckDaily == v)
    return;
  m_updateCheckDaily = v;
  emit updateCheckDailyChanged();
}

void SettingsViewModel::setUpdateChannel(const QString &v) {
  if (m_updateChannel == v)
    return;
  m_updateChannel = v;
  emit updateChannelChanged();
}

// Provider helpers
QVariantList *SettingsViewModel::providerListRef(const QString &category) {
  if (category == "asr")
    return &m_asrProviders;
  if (category == "vlm")
    return &m_vlmProviders;
  if (category == "llm")
    return &m_llmProviders;
  return nullptr;
}

const QVariantList
SettingsViewModel::providerList(const QString &category) const {
  if (category == "asr")
    return m_asrProviders;
  if (category == "vlm")
    return m_vlmProviders;
  if (category == "llm")
    return m_llmProviders;
  return QVariantList();
}

void SettingsViewModel::setProviderList(const QString &category,
                                        const QVariantList &list) {
  if (category == "asr") {
    m_asrProviders = list;
    emit asrProvidersChanged();
  } else if (category == "vlm") {
    m_vlmProviders = list;
    emit vlmProvidersChanged();
  } else if (category == "llm") {
    m_llmProviders = list;
    emit llmProvidersChanged();
  }
}

void SettingsViewModel::emitProvidersChanged(const QString &category) {
  if (category == "asr")
    emit asrProvidersChanged();
  else if (category == "vlm")
    emit vlmProvidersChanged();
  else if (category == "llm")
    emit llmProvidersChanged();
}

void SettingsViewModel::addProvider(const QString &category,
                                    const QString &key) {
  qDebug() << "[VM] addProvider:" << category << "key=" << key;
  auto list = providerList(category);
  for (const auto &v : list) {
    if (v.toMap()["key"].toString() == key) {
      qDebug() << "[VM]   key already exists, skip";
      return;
    }
  }
  QVariantMap m;
  m["key"] = key;
  m["name"] = key;
  m["baseUrl"] = "";
  m["model"] = "";
  m["apiKey"] = "";
  m["authType"] = "bearer";
  m["apiFormat"] = "openai";
  m["numCtx"] = 0;
  m["type"] = "cloud"; // 默认云端，本地模型通过 AddModelDialog 设为 local
  m["language"] = "";
  m["stream"] = false;
  m["localModelPath"] = ""; // ASR type=local: whisper .bin 路径（per-provider）
  m["retryCount"] = 3;      // 云请求 HTTP 重试次数（默认 3，与后端 DefaultMaxRetries 一致）
  list.append(m);
  setProviderList(category, list);
  qDebug() << "[VM]   added, total=" << list.size();
}

void SettingsViewModel::removeProvider(const QString &category,
                                       const QString &key) {
  auto list = providerList(category);
  for (int i = 0; i < list.size(); ++i) {
    if (list[i].toMap()["key"].toString() == key) {
      list.removeAt(i);
      break;
    }
  }
  setProviderList(category, list);
}

void SettingsViewModel::updateProvider(const QString &category,
                                       const QString &key,
                                       const QVariantMap &data) {
  qDebug() << "[VM] updateProvider:" << category << "key=" << key << "data keys=" << data.keys();
  auto list = providerList(category);
  for (int i = 0; i < list.size(); ++i) {
    auto m = list[i].toMap();
    if (m["key"].toString() == key) {
      qDebug() << "[VM]   found provider at index" << i << "before name=" << m["name"];
      auto update = [&m, &data](const char *k) {
        if (data.contains(k))
          m[k] = data[k];
      };
      update("name");
      update("baseUrl");
      update("model");
      update("apiKey");
      update("authType");
      update("apiFormat");
      update("type");
      update("language");
      update("localModelPath");
      if (data.contains("stream"))
        m["stream"] = data["stream"].toBool();
      if (data.contains("numCtx"))
        m["numCtx"] = data["numCtx"].toInt();
      if (data.contains("retryCount"))
        m["retryCount"] = data["retryCount"].toInt();
      qDebug() << "[VM]   after name=" << m["name"] << "type=" << m["type"];
      list[i] = m;
      setProviderList(category, list);
      return;
    }
  }
  qDebug() << "[VM]   provider NOT FOUND for key=" << key;
}

void SettingsViewModel::setActiveProvider(const QString &category,
                                          const QString &key) {
  if (category == "asr")
    setAsrActiveProvider(key);
  else if (category == "vlm")
    setVlmActiveProvider(key);
  else if (category == "llm")
    setLlmActiveProvider(key);
}

void SettingsViewModel::load() {
  if (!m_channel) {
    setError(QStringLiteral("gRPC channel 未初始化"));
    emit loadFinished(false);
    return;
  }
  setLoading(true);
  setError({});

  GetConfigRequest req;
  auto reply = m_client.GetConfig(req);
  auto *replyPtr = reply.get();
  reply.release();

  QObject::connect(replyPtr, &QGrpcCallReply::finished, this,
                   [this, replyPtr](const QGrpcStatus &status) {
                     replyPtr->deleteLater();
                     setLoading(false);

                     if (!status.isOk()) {
                       setError(QStringLiteral("gRPC 错误: ") +
                                status.message());
                       emit loadFinished(false);
                       return;
                     }

                     auto opt = replyPtr->read<ConfigData>();
                     if (!opt.has_value()) {
                       setError(QStringLiteral("解析响应失败"));
                       emit loadFinished(false);
                       return;
                     }
                     applyConfig(*opt);
                     emit loadFinished(true);
                   });
}

void SettingsViewModel::save() {
  if (!m_channel) {
    setError(QStringLiteral("gRPC channel 未初始化"));
    emit saveFinished(false, m_error);
    return;
  }
  setLoading(true);
  setError({});

  ConfigData cfg = buildConfig();
  auto reply = m_client.SaveConfig(cfg);
  auto *replyPtr = reply.get();
  reply.release();

  QObject::connect(replyPtr, &QGrpcCallReply::finished, this,
                   [this, replyPtr](const QGrpcStatus &status) {
                     replyPtr->deleteLater();
                     setLoading(false);

                     if (!status.isOk()) {
                       setError(QStringLiteral("保存失败: ") +
                                status.message());
                       emit saveFinished(false, m_error);
                       return;
                     }

                     auto opt = replyPtr->read<Result>();
                     if (!opt.has_value() || !opt->ok()) {
                       setError(QStringLiteral("保存返回失败"));
                       emit saveFinished(false, m_error);
                       return;
                     }
                     emit saveFinished(true, QString{});
                   });
}

// 按 provider 的 key 排序，保证 chip 列表顺序稳定（map 遍历顺序不固定）。
static QVariantList sortProvidersByKey(QVariantList list) {
  std::sort(list.begin(), list.end(),
            [](const QVariant &a, const QVariant &b) {
              return a.toMap()["key"].toString() <
                     b.toMap()["key"].toString();
            });
  return list;
}

static QVariantMap providerToMap(const QString &key, const ProviderConfig &p) {
  QVariantMap m;
  m["key"] = key;
  m["name"] = p.name();
  m["baseUrl"] = p.baseUrl();
  m["model"] = p.model();
  m["apiKey"] = p.apiKey();
  m["authType"] = p.authType();
  m["apiFormat"] = p.apiFormat();
  m["numCtx"] = (int)p.numCtx();
  m["type"] = p.type();
  m["language"] = p.language();
  m["stream"] = p.stream();
  m["localModelPath"] = p.localModelPath();
  m["retryCount"] = (int)p.retryCount();
  return m;
}

static QHash<QString, ProviderConfig>
providersFromList(const QVariantList &list) {
  QHash<QString, ProviderConfig> map;
  for (const auto &v : list) {
    auto m = v.toMap();
    ProviderConfig p;
    p.setName(m["name"].toString());
    p.setBaseUrl(m["baseUrl"].toString());
    p.setModel(m["model"].toString());
    p.setApiKey(m["apiKey"].toString());
    p.setAuthType(m["authType"].toString());
    p.setApiFormat(m["apiFormat"].toString());
    p.setNumCtx((qint32)m["numCtx"].toInt());
    p.setType(m["type"].toString());
    p.setLanguage(m["language"].toString());
    p.setStream(m["stream"].toBool());
    p.setLocalModelPath(m["localModelPath"].toString());
    p.setRetryCount((qint32)m["retryCount"].toInt());
    map.insert(m["key"].toString(), p);
  }
  return map;
}

void SettingsViewModel::applyConfig(const ConfigData &data) {
  setAsrMode(data.asrMode());
  setAsrActiveProvider(data.asrActiveProvider());
  setAsrLocalModelPath(data.asrLocalModelPath());
  setAsrLocalModelName(data.asrLocalModelName());
  setAsrLocalLanguage(data.asrLocalLanguage());
  setRecordMode(data.asrRecordMode());

  QVariantList asrList;
  const auto asrMap = data.asrProviders();
  for (auto it = asrMap.begin(); it != asrMap.end(); ++it)
    asrList.append(providerToMap(it.key(), it.value()));
  m_asrProviders = sortProvidersByKey(asrList);
  emit asrProvidersChanged();

  setVlmMode(data.vlmMode());
  setVlmActiveProvider(data.vlmActiveProvider());
  setVlmInterval((int)data.vlmScheduleIntervalMin());
  setVlmCaptureRange(data.vlmCaptureRange());
  // on_demand gap：proto int32 零值=未设置，回落默认（switch 20 / motion 60）。
  int sg = data.vlmOnDemandSwitchGapS();
  setVlmSwitchGap(sg > 0 ? sg : 20);
  int mg = data.vlmOnDemandMotionGapS();
  setVlmMotionGap(mg > 0 ? mg : 60);
  setVlmPrompt(data.vlmPrompt());

  QVariantList vlmList;
  const auto vlmMap = data.vlmProviders();
  for (auto it = vlmMap.begin(); it != vlmMap.end(); ++it)
    vlmList.append(providerToMap(it.key(), it.value()));
  m_vlmProviders = sortProvidersByKey(vlmList);
  emit vlmProvidersChanged();

  setLlmEnabled(data.polishEnabled());
  setLlmActiveProvider(data.polishActiveProvider());
  setLlmPrompt(data.polishPrompt());
  setLlmInjectMode(data.injectMode());

  QVariantList llmList;
  const auto llmMap = data.polishProviders();
  for (auto it = llmMap.begin(); it != llmMap.end(); ++it)
    llmList.append(providerToMap(it.key(), it.value()));
  m_llmProviders = sortProvidersByKey(llmList);
  emit llmProvidersChanged();

  setMovementSampleMs((int)data.movementSampleIntervalMs());
  setMovementIdleS((int)data.movementIdleTimeoutS());
  setMovementPrecision(data.movementPrecision());

  setHotkeyRecord(data.hotkeyRecord());
  setHotkeyScreenshot(data.hotkeyScreenshot());
  setHotkeyPromptPrefix(data.hotkeyPromptPrefix());
  setScreenshotWithVlm(data.screenshotWithVlm());
  setScreenshotPrompt(data.screenshotPrompt());

  setUpdateServerUrl(data.updateServerUrl());
  setUpdateCheckOnStartup(data.updateCheckOnStartup());
  setUpdateCheckDaily(data.updateCheckDaily());
  QString ch = data.updateChannel();
  setUpdateChannel(ch.isEmpty() ? QStringLiteral("stable") : ch);
}

ConfigData SettingsViewModel::buildConfig() const {
  ConfigData data;
  data.setAsrMode(m_asrMode);
  data.setAsrActiveProvider(m_asrActiveProvider);
  data.setAsrLocalModelPath(m_asrLocalModelPath);
  data.setAsrLocalModelName(m_asrLocalModelName);
  data.setAsrLocalLanguage(m_asrLocalLanguage);
  data.setAsrRecordMode(m_recordMode);
  data.setAsrProviders(providersFromList(m_asrProviders));

  data.setVlmMode(m_vlmMode);
  data.setVlmActiveProvider(m_vlmActiveProvider);
  data.setVlmScheduleIntervalMin((qint32)m_vlmInterval);
  data.setVlmCaptureRange(m_vlmCaptureRange);
  data.setVlmOnDemandSwitchGapS((qint32)m_vlmSwitchGap);
  data.setVlmOnDemandMotionGapS((qint32)m_vlmMotionGap);
  data.setVlmPrompt(m_vlmPrompt);
  data.setVlmProviders(providersFromList(m_vlmProviders));

  data.setPolishEnabled(m_llmEnabled);
  data.setPolishActiveProvider(m_llmActiveProvider);
  data.setPolishPrompt(m_llmPrompt);
  data.setInjectMode(m_llmInjectMode);
  data.setPolishProviders(providersFromList(m_llmProviders));

  data.setMovementSampleIntervalMs((qint32)m_movementSampleMs);
  data.setMovementIdleTimeoutS((qint32)m_movementIdleS);
  data.setMovementPrecision(m_movementPrecision);

  data.setHotkeyRecord(m_hotkeyRecord);
  data.setHotkeyScreenshot(m_hotkeyScreenshot);
  data.setHotkeyPromptPrefix(m_hotkeyPromptPrefix);
  data.setScreenshotWithVlm(m_screenshotWithVlm);
  data.setScreenshotPrompt(m_screenshotPrompt);

  data.setUpdateServerUrl(m_updateServerUrl);
  data.setUpdateCheckOnStartup(m_updateCheckOnStartup);
  data.setUpdateCheckDaily(m_updateCheckDaily);
  data.setUpdateChannel(m_updateChannel);

  return data;
}

void SettingsViewModel::setLoading(bool v) {
  if (m_loading == v)
    return;
  m_loading = v;
  emit loadingChanged();
}

void SettingsViewModel::setError(const QString &e) {
  if (m_error == e)
    return;
  m_error = e;
  emit errorChanged();
}
