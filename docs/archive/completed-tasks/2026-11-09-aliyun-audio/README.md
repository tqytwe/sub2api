# 阿里云语音中转适配

状态：实现可测，发布与用户本机验收待完成。维护责任：本任务提交者。

初始基线 `5312264073d3775ca4b2a5d4c88ac223ae62621f`，普通合并生产更新 `b01ea551243536a297be84080899fe673902c09b`（独立的统计卡样式修复）。工作分支 `fix/aliyun-audio-relay-20261109`，隔离工作树 `/data/projects/sub2api-aliyun-audio-20261109`。关联 `FORK-AUDIO-022`；不涉及上游整版同步、数据库迁移、视觉修改或历史数据修复。

## 归因与范围

生产只读检查确认目标为 active/schedulable 的 OpenAI API Key 账号，配置阿里云北京业务空间的 `/compatible-mode/v1` 地址并绑定分组。近七日目标样本：两个语音合成及录音识别分别出现一次 404，实时语音两次 502。账号名称不是协议类型；旧实现统一追加 OpenAI 音频路径，阿里云这些模型使用专用接口，导致失败。未输出凭据、用户身份、原始日志或完整下载签名。

| 模型 | 用户入口 | 实际上游与转换 |
| --- | --- | --- |
| qwen-audio-3.1-tts-plus | POST /v1/audio/speech | /api/v1/services/audio/tts/SpeechSynthesizer；input.text、阿里音色，下载并返回音频 |
| qwen-audio-3.1-tts-next | POST /v1/audio/speech | 同端点；input.text_prompt，不传系统 voice，返回音频 |
| qwen-audio-3.1-asr-flash-filetrans | POST /v1/audio/transcriptions | 上传策略/OSS → 异步提交 → tasks 轮询 → 下载识别 JSON；返回 json/text/verbose_json/srt/vtt |
| qwen-audio-3.1-realtime-flash | WS /v1/realtime?model=... | /api-ws/v1/realtime；兼容 Agents GA 会话、旧事件名称、工具定义、默认 semantic_vad 与输入 PCM 采样率 |

使用平台 API Key 和平台地址；阿里密钥只保存在上游账号。阿里音色原样保留，OpenAI 内置 TTS 音色（含 Agents 默认 ash）映射 longanhuan_v3.1；Realtime 内置 OpenAI 音色映射 longanqian_v3.1。TTS 默认输出 24kHz，兼容 Agents 的 PCM 播放。保持既有分组语音计费单位（TTS 百万字符、STT 小时、Realtime 分钟），不变更售价或官方 Token 费用口径。

兼容限制：Filetrans 为同步等待包装，最长 5 分钟，适合已有 OpenAI transcription 调用；更长任务需要另行设计持久任务接口。结果文件最大 64 MiB。Realtime 支持 PCM，阿里原生 16kHz 输入保持原样，OpenAI 24kHz 输入按帧连续转换；输出 24kHz。暂不支持 WebRTC/AOQ、完整 OpenAI 可选参数或任意客户端。用户所说 agents 的具体版本尚未知，必须由原调用方复测。

用户调用示例（`PLATFORM_API_KEY` 仅由调用者在进程环境中提供）：

```bash
curl https://www.jisudeng.com/v1/audio/speech \
  -H "Authorization: Bearer $PLATFORM_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"qwen-audio-3.1-tts-plus","input":"你好，欢迎使用。","voice":"longanhuan_v3.1","response_format":"wav"}' \
  --output speech.wav

curl https://www.jisudeng.com/v1/audio/transcriptions \
  -H "Authorization: Bearer $PLATFORM_API_KEY" \
  -F model=qwen-audio-3.1-asr-flash-filetrans \
  -F file=@speech.wav
```

Next 使用同一 speech 入口，把 model 换为 `qwen-audio-3.1-tts-next`，input 填声音/对话描述。Realtime 使用 `wss://www.jisudeng.com/v1/realtime?model=qwen-audio-3.1-realtime-flash`，在握手头携平台 API Key；不使用 `/chat/completions` 或 `/responses` 调用这些专用语音模型。

## TDD、规格与质量审查

RED 已证明旧实现把 SpeechSynthesizer、Filetrans 与 Realtime 请求发到兼容模式错误路径；Filetrans 直接返回任务而非文本。新增失败测试另确认 Agents 默认 semantic_vad、首个 session.created 缺 GA 类型判别字段，以及助手消息内容的 text/audio 与 GA output_text/output_audio 不匹配。

GREEN 定向回归覆盖二进制结果、模型与字段转换、异步任务、普通 multipart 文件上传、私网结果拒绝、已受理任务不重试、连续分帧重采样、Agents 会话及事件转换；保留原 OpenAI 音频测试。

规格审查：逐项核对四模型、原客户端入口、原生域名识别、别名与计费归属；未将语音模型送聊天接口，未改变账号/分组配置。修正 Agents 默认 VAD 类型与首个会话事件，补充分帧采样率验证。

代码质量审查：上传/下载使用无 Authorization 的独立请求，下载逐跳 public-host 校验；原生认证和签名上传禁止重定向；OSS HTTP 签名链接升级 HTTPS；错误不包含签名 URL。轮询限时，成功提交后失败不重投付费任务；沿用原 audio 的 detached 上游上下文，断线后仍有界收集结果供结算，新增断线回归先 RED 再修复。GA 状态跨读写 goroutine 用原子变量，采样器状态只归客户端写入方向使用。保留原模型路由及分组音频计费。

## 真实上游技术验证

2026-11-09，受控进程内使用目标账号现有配置进行最小请求；密钥与结果签名不写文件/日志，探测仅输出状态和计数。

- TTS Plus：原生 API 200，音频下载 200，38,444 字节。
- TTS Next：原生 API 200，音频下载 200，230,444 字节。
- Filetrans：真实 WAV 上传策略 200、OSS 上传 200、提交 200、轮询 SUCCEEDED、子任务 SUCCEEDED、结果下载 200，1 段 transcript。
- Realtime：101，session.created、session.updated，实际文本轮次完成 response.done；默认 smart_turn_v2 被接受。
- 追加 Realtime 音频实测：收到 response.audio.delta/audio_transcript.delta，response.done 的 status=completed；并非仅握手成功。
- Agents 默认 TTS 请求形态实测：24kHz PCM、默认空 instructions，原生请求和下载均为 200，38,400 字节。
- 初次诊断脚本误给签名下载 GET 带 Content-Type，产生 SignatureDoesNotMatch；修正脚本后均为 200。平台下载实现不带该请求头，已用回归固定。网络曾有瞬时 URLError，未把失败记作成功。

官方协议参考：[TTS](https://help.aliyun.com/zh/model-studio/qwen-audio-tts-http-api)、[Next](https://help.aliyun.com/zh/model-studio/audio-generation-api)、[Filetrans](https://help.aliyun.com/zh/model-studio/qwen-audio-asr-filetrans-http-api)、[Realtime](https://help.aliyun.com/zh/model-studio/audio-realtime-websocket-access)、[客户端事件](https://help.aliyun.com/zh/model-studio/fun-audiochat-client-events)。

## 验证与发布记录

- 定向测试：全部通过，含 Agents 默认 TTS ash/24kHz、首个 greeting、内容字段转换与断线用量回归。
- `make test` 最终通过：后端完整测试、golangci-lint 0 issues、前端 lint/typecheck、547 文件/4,249 测试通过，沿用基线 3 文件/8 测试跳过。额外 `make test-backend-unit`、`make build`、`./scripts/check-fork-integrity.sh` 全部通过，文档完整性检查 349 文件通过。首次 lint 发现 errcheck/staticcheck/格式问题后已修复；一次 lint 因与额外编译重叠遭资源杀死，不算通过。最终顺序执行并约束 Go 内存，未放宽测试或改无关功能。
- PR、交付 SHA、部署 SHA/健康：待记录。
- 本机游客/普通用户/管理员，以及原 Agents 调用：待用户验收。真实上游探测与服务端测试不能替代此项。
- 回退：撤销本次代码提交即可恢复原转发；无数据迁移或生产配置写入。权限、计费、部署 SHA 或健康异常时停止发布/验收，不修改用户账务。
