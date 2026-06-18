// SettingsViewModel 实现

#include "settings_vm.h"

#include <QDebug>
#include <QGrpcCallReply>
#include <QGrpcStatus>
#include <QHash>

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
  auto list = providerList(category);
  for (const auto &v : list) {
    if (v.toMap()["key"].toString() == key)
      return;
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
  list.append(m);
  setProviderList(category, list);
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
  auto list = providerList(category);
  for (int i = 0; i < list.size(); ++i) {
    auto m = list[i].toMap();
    if (m["key"].toString() == key) {
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
      if (data.contains("numCtx"))
        m["numCtx"] = data["numCtx"].toInt();
      list[i] = m;
      setProviderList(category, list);
      return;
    }
  }
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
                       return;
                     }

                     auto opt = replyPtr->read<ConfigData>();
                     if (!opt.has_value()) {
                       setError(QStringLiteral("解析响应失败"));
                       return;
                     }
                     applyConfig(*opt);
                   });
}

void SettingsViewModel::save() {
  if (!m_channel) {
    setError(QStringLiteral("gRPC channel 未初始化"));
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
                       return;
                     }

                     auto opt = replyPtr->read<Result>();
                     if (!opt.has_value() || !opt->ok()) {
                       setError(QStringLiteral("保存返回失败"));
                       return;
                     }
                   });
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

  QVariantList asrList;
  const auto asrMap = data.asrProviders();
  for (auto it = asrMap.begin(); it != asrMap.end(); ++it)
    asrList.append(providerToMap(it.key(), it.value()));
  m_asrProviders = asrList;
  emit asrProvidersChanged();

  setVlmMode(data.vlmMode());
  setVlmActiveProvider(data.vlmActiveProvider());
  setVlmInterval((int)data.vlmScheduleIntervalMin());

  QVariantList vlmList;
  const auto vlmMap = data.vlmProviders();
  for (auto it = vlmMap.begin(); it != vlmMap.end(); ++it)
    vlmList.append(providerToMap(it.key(), it.value()));
  m_vlmProviders = vlmList;
  emit vlmProvidersChanged();

  setLlmEnabled(data.polishEnabled());
  setLlmActiveProvider(data.polishActiveProvider());
  setLlmPrompt(data.polishPrompt());
  setLlmInjectMode(data.injectMode());

  QVariantList llmList;
  const auto llmMap = data.polishProviders();
  for (auto it = llmMap.begin(); it != llmMap.end(); ++it)
    llmList.append(providerToMap(it.key(), it.value()));
  m_llmProviders = llmList;
  emit llmProvidersChanged();

  setMovementSampleMs((int)data.movementSampleIntervalMs());
  setMovementIdleS((int)data.movementIdleTimeoutS());
  setMovementPrecision(data.movementPrecision());

  setHotkeyRecord(data.hotkeyRecord());
  setHotkeyScreenshot(data.hotkeyScreenshot());
  setHotkeyPromptPrefix(data.hotkeyPromptPrefix());
}

ConfigData SettingsViewModel::buildConfig() const {
  ConfigData data;
  data.setAsrMode(m_asrMode);
  data.setAsrActiveProvider(m_asrActiveProvider);
  data.setAsrLocalModelPath(m_asrLocalModelPath);
  data.setAsrLocalModelName(m_asrLocalModelName);
  data.setAsrLocalLanguage(m_asrLocalLanguage);
  data.setAsrProviders(providersFromList(m_asrProviders));

  data.setVlmMode(m_vlmMode);
  data.setVlmActiveProvider(m_vlmActiveProvider);
  data.setVlmScheduleIntervalMin((qint32)m_vlmInterval);
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
