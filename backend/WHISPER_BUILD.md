# 本地 Whisper ASR 构建说明

后端通过 CGO 静态链接 [whisper.cpp](https://github.com/ggerganov/whisper.cpp)，在 Go 进程内完成本地语音识别（PCM 不离开后端进程）。

## 架构

```
Qt 客户端 --gRPC--> Go 后端 (voice_server)
                      |
                      +-- audio.Capture (Win32 waveIn, 纯 Go)
                      +-- audio.Spectrum (FFT, 实时频谱)
                      +-- asr.localEngine (CGO -> whisper.cpp/libwhisper.a)
                      +-- SQLite (持久化)
```

`internal/asr/local.go` 通过官方 Go 绑定 `github.com/ggerganov/whisper.cpp/bindings/go` 调用 whisper。模型 `.bin` 在引擎初始化时加载一次，之后每次 `Recognize` 复用同一个 model context。

## 前置依赖

| 依赖 | 版本 | 路径（默认） | 说明 |
|------|------|-------------|------|
| w64devkit | **1.21.0 (gcc 13.2.0)** | `D:\Qt\w64devkit` | gcc 16 会产生 Go cgo 无法解析的 COFF，**必须用 gcc 13** |
| Ninja | 任意 | `D:\Qt\Tools\Ninja` | whisper.cpp 的 CMake 构建器 |
| CMake | ≥3.16 | PATH 上 | 配置 whisper.cpp |
| Go | ≥1.21 | PATH 上 | `CGO_ENABLED=1` |

## 一次性准备

### 1. 克隆 whisper.cpp 源码

```cmd
cd backend
git clone https://github.com/ggerganov/whisper.cpp third_party\whisper.cpp
```

> `third_party/` 不在 `.gitignore`（源码可读、可改），但 `third_party/whisper.cpp/build/` 和 `.git/` 建议忽略以避免提交编译产物。

### 2. 应用 MinGW 源码补丁

w64devkit 的 MinGW 头文件缺少 Windows 11 的 `THREAD_POWER_THROTTLING_*` 线程级 API（只有进程级）。在 `third_party/whisper.cpp/ggml/src/ggml-cpu/ggml-cpu.c` 的 `ggml_thread_apply_priority` 函数里，`#if _WIN32_WINNT >= 0x0602` 块**之前**插入 fallback 定义：

```c
        #ifndef THREAD_POWER_THROTTLING_CURRENT_VERSION
        #define THREAD_POWER_THROTTLING_CURRENT_VERSION 1
        #endif
        #ifndef THREAD_POWER_THROTTLING_EXECUTION_SPEED
        #define THREAD_POWER_THROTTLING_EXECUTION_SPEED 0x1
        #endif
        #ifndef THREAD_POWER_THROTTLING_STATE
        typedef struct _THREAD_POWER_THROTTLING_STATE {
            ULONG Version;
            ULONG ControlMask;
            ULONG StateMask;
        } THREAD_POWER_THROTTLING_STATE;
        #endif
```

`SetThreadInformation` 和 `ThreadPowerThrottling` 枚举值已在 w64devkit 的 `winbase.h` 中定义，无需补。

### 3. 编译静态库

```cmd
backend\build-whisper-libs.bat
```

产出：
- `third_party/whisper.cpp/build/src/libwhisper.a`
- `third_party/whisper.cpp/build/ggml/src/libggml.a`、`libggml-base.a`、`libggml-cpu.a`

> CMake 产出的 ggml 库名是 `ggml.a`（无 `lib` 前缀），脚本会自动重命名为 `libggml.a`，因为 cgo 的 `-lggml` 按 `libggml.a` 查找。

### 4. 同步 Go 依赖

```cmd
cd backend
go mod tidy
go mod vendor
```

## 构建

```cmd
backend\build-whisper-cgo.bat
```

脚本会：
1. **自动 patch vendor**：上游 whisper Go 绑定只在 linux 加 `-fopenmp`，windows 下需补 `-lgomp`。脚本用 PowerShell 幂等地替换 `vendor/.../whisper.go` 里的 LDFLAGS 行（`go mod vendor` 会清掉这个改动，所以每次构建前都跑一遍）。
2. 设置 CGO 工具链环境（`CC`/`CXX`/`C_INCLUDE_PATH`/`LIBRARY_PATH`）。
3. `go build` 产出 `build/shadow-worker.exe`。

## 获取模型

下载 ggml 格式的 whisper 模型（`.bin`）放到任意路径，然后在设置页或 `config.yaml` 里配置 `local_asr.model_path`：

- 推荐 `ggml-small.bin`（~466MB，中英平衡）
- 轻量 `ggml-tiny.bin`（~75MB，快速但精度低）
- 下载：<https://huggingface.co/ggerganov/whisper.cpp/tree/main>

## 故障排查

- **`undefined reference to GOMP_*`**：vendor patch 没生效，确认 `build-whisper-cgo.bat` 跑了 patch 步骤。
- **`cannot find -lggml`**：静态库未重命名为 `libggml.a`，重跑 `build-whisper-libs.bat`。
- **`cannot parse gcc output ... as ELF/PE`**：gcc 版本太新（16.x），换 w64devkit 1.21.0（gcc 13.2.0）。
- **`unknown type name THREAD_POWER_THROTTLING_STATE`**：MinGW 补丁未应用，见上文第 2 步。
