# v2.23.1 上游同步项目清单

> 核查日：2026-11-27。固定范围，不与后续 release 混账。

## 项目状态

- 上游：`ranxi2001/sub2api` tag `v2.23.1`，commit `00e00796080c632188cf9df7912050b144a1a44f`。
- 范围：从 `v2.21.2` 起累计 107 个 first-parent 提交，其中本次 v2.23.0→v2.23.1 有 18 个。
- 状态：107 项均已完成逐 SHA 处置（87 项 `equivalent`、20 项 `migrated`），0 项开放。代码差异闭环不等于生产交付；合并、部署、数据对账和三身份验收仍单独记录。
- 当前候选已通过本地发布门禁和全量代码验证，尚未 PR、合并或部署；不得提前记为生产交付。
- 合并门禁：`python3 scripts/check_upstream_dispositions.py --verify-git --require-merge-ready`。锁定来源、累计零开放项、终态证据及必需 CI 均通过才允许合并部署；此时不预填生产证据，后台仍保持 `fully_migrated=false`。
- 整版关闭门禁：部署后核对同 SHA 的 Zeabur、健康/关键 API、适用的数据对账及三身份本机验收，记录证据并通过 `python3 scripts/check_upstream_dispositions.py --verify-git --require-complete`，才能切换后台同步状态。状态关闭变更部署后单独核对后台显示。
- 上游 v2.25.0 已于 2026-11-24 发布；此项目仍锁定 v2.23.1。后续版本另立来源锁和账本，禁止悄悄漂移。

## 稳定跟踪号规则

开放提交以 `SYNC-` + 完整 SHA 前 8 位的大写值作为固定跟踪号；后续复核、代码、测试、PR 和部署证据都回写同一条，不因状态变化另建重复项。已关闭提交仍以完整 SHA 保留在机器账本中。表格中的“负责人”代表责任角色；实际姓名需由项目负责人认领/分派；开放事项未分派前不得标记为完成。

## 工作包、责任与完成证据

| ID | 范围 | 责任角色（姓名由项目负责人分配） | 当前状态 | 关闭条件 |
|---|---|---|---|---|
| G0 | 逐条审查累计 107 个提交及 PR、文件、SQL | 同步负责人 + 分域负责人 | 已完成 | 每个 SHA 均有固定跟踪号、处置结论和逐 SHA 审计锚点 |
| G1 | #835/#934/#940/#951 协议、安全、恢复、压缩和诊断 | 网关后端 + 安全复核 | 已完成 | HTTP/WS 规格矩阵、原生发送路径测试、安全复核已纳入候选 |
| G2 | #939/#950 生图计费闭环 | 计费后端 + 前端 | 已完成 | 所有入口、异步恢复、SSE/JSON 回执、映射价格和零产出解释已覆盖 |
| G3 | #949/#955 供应商利润、钱包快照、事件归属 | 数据/会计 + 后端 | 已完成 | 钱包窗口快照、扣费证据、报表成本和历史边界集成测试已覆盖 |
| G4 | 其余累计差异：调度、BPS、迁移、UI、部署 | 分域负责人（逐项分派） | 已完成 | 107 项均已迁入、等效或有批准排除，且有仓库证据 |
| G5 | 整版发布与验收 | 发布负责人 | 未开始 | PR CI、精确部署 SHA、健康/API、对账和本机三身份验收 |

## 本次 release 重点处置结果

| PR / SHA | 状态 | 需要完成的行为 |
|---|---|---|
| #934 `270ae1710353371552dd9f11be0dc3374de4bc9d` | migrated | 默认关闭/环境配置、HTTP 账号池重试、三种 WS 接入模式双轮续接、不写 owner 和 no-replay 控制已完成并有回归证据。 |
| #939 `afcef2d6811cb860b04dcba45ae1c6477671f5bc` | migrated | 分组开关、所有图片入口、实际产出数量、映射模型价格、结算回执和客户费用解释已完成并有回归证据。 |
| #940 `009eefc27c1dd02f27b39f54f23ed1e51d24b300` | migrated | 区域路由、HTTP 415 身份回退、Codex/WS 准入及实际仅发送一次已完成并有回归证据。 |
| #949 `4f1bfa96d760711197b692ce94d8f11f0f895c58` | migrated | `cost_mode` 运行闭环、钱包窗口快照、扣费证据、历史归属和按窗口报表读取已完成并有集成证据。 |
| #950 `48dee26a76d9430a6e883a66d069075e921a91e9` | migrated | 非流式/异步恢复、SSE/JSON 回执、映射模型价格及零产出费用解释已完成并有回归证据。 |
| #951 `b107d07da2af136995f154f57ff67f5597018844` | migrated | HTTP/WS 重建控制、运维诊断 API/UI、安全字段防泄露与工作流回归已完成。 |
| #955 `7563b886943a510e8d2e0bbd0af5618df2cecff7` | migrated | 历史事件归属、成本报表边界、旧数据时点保护和 #949 关联数据模型已完成并有集成证据。 |

## 累计提交处置记录（逐项维护；不得批量重建跟踪号）

SHA 与标题取自已锁定的 `v2.21.2-first-parent.tsv`。每项状态必须与 `dispositions.json` 一致；后续回补继续写回原跟踪号，不得新建重复事项。

| # | 跟踪号（固定 SHA 派生） | 上游 commit | 状态 | 提交标题 / 当前结论 | 负责人 |
|---:|---|---|---|---|---|
| 1 | `SYNC-CCF997C7` | `ccf997c73868083d96c0bec56e71c1a34d26c98a` | `equivalent` | chore: sync VERSION to 2.21.2 [skip ci]；仅上游发布/CI/fixture 或版本元数据变更；Fork 生产契约不变，已由来源锁与仓库门禁覆盖。 | 同步负责人 |
| 2 | `SYNC-F895EA74` | `f895ea743f34a4c4ab6c9d01c01e13c931625a6e` | `equivalent` | Merge pull request #832 from ranxi2001/test/issue795-scheduled-e2e-20261113；仅上游发布/CI/fixture 或版本元数据变更；Fork 生产契约不变，已由来源锁与仓库门禁覆盖。 | 同步负责人 |
| 3 | `SYNC-69141821` | `6914182116aa8f6904ebb3b69cdcbd5fc1b1d4b8` | `equivalent` | Merge pull request #837 from ranxi2001/feat/account-ops-quality-selector；Fork 采用本地账号运维/认证边界；对应管理端 API、仓库和前端契约已覆盖。 | 同步负责人 |
| 4 | `SYNC-E10FE8C4` | `e10fe8c43f7ccc6bf3360824c0ed11031f596389` | `equivalent` | Merge pull request #836 from ranxi2001/feat/admin-usage-platform-filter；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 5 | `SYNC-0B9CBFBA` | `0b9cbfbabb2d0ff03adc7f3f5f65b9b4edaa029b` | `equivalent` | Merge pull request #835 from ranxi2001/codex/400-encrypted-history-beta；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 6 | `SYNC-DFC0219A` | `dfc0219a9e66a19642eb73875e4914e540b49575` | `equivalent` | Merge pull request #839 from ranxi2001/fix/oauth429-weekly-only；Fork 采用本地账号运维/认证边界；对应管理端 API、仓库和前端契约已覆盖。 | 同步负责人 |
| 7 | `SYNC-EA1F3A28` | `ea1f3a28d8d1839e92ac5e48c6e9d7aeb90b0fff` | `equivalent` | Merge pull request #833 from ranxi2001/fix/http2-stream-error-clarity；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 8 | `SYNC-7827A2F5` | `7827a2f5f900ce50aafa37ab71ca1918dfe7cf8a` | `equivalent` | Merge pull request #841 from ranxi2001/test/issue795-complete-rounds-20261114；仅上游发布/CI/fixture 或版本元数据变更；Fork 生产契约不变，已由来源锁与仓库门禁覆盖。 | 同步负责人 |
| 9 | `SYNC-FA9EC49D` | `fa9ec49d433a9898becdb779e0b3f7960df4dfae` | `equivalent` | Merge pull request #843 from ranxi2001/fix/account-alert-data-ux-20261114；Fork 采用本地账号运维/认证边界；对应管理端 API、仓库和前端契约已覆盖。 | 同步负责人 |
| 10 | `SYNC-0BDE0EA6` | `0bde0ea69024018c0060db34f807aa5291ee5d64` | `equivalent` | Merge pull request #847 from pixcook-ai/codex/bps-service-tier-422-20261114；Fork 保留 BPS 适配层；请求、工具、搜索和诊断契约已由 BPS 回归覆盖。 | 同步负责人 |
| 11 | `SYNC-B3472496` | `b347249687db84dac3f5272189fda6cf86eb9035` | `equivalent` | Merge pull request #846 from ranxi2001/fix/http2-reset-review-followup-20261114；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 12 | `SYNC-89E06C08` | `89e06c086fcdcc200238e5d208af2219e2d21e43` | `equivalent` | Merge pull request #840 from ranxi2001/codex/cpa-bps-diagnostics-auth-20261114；Fork 保留 BPS 适配层；请求、工具、搜索和诊断契约已由 BPS 回归覆盖。 | 同步负责人 |
| 13 | `SYNC-CDD9D790` | `cdd9d7901041d02aa39291ae0e1615c5a6853da0` | `equivalent` | Merge pull request #845 from akihitohyh/feat/usage-trend-tooltip-rpm-tpm；Fork 保留本地计费/投影模型；客户侧契约由计费、余额、用量和价格回归覆盖。 | 同步负责人 |
| 14 | `SYNC-4189ED82` | `4189ed821d6e4308b2f5fc3791b73d2eb53cda8a` | `equivalent` | Merge pull request #838 from spake404/cursor/temp-unsched-controls-0093；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 15 | `SYNC-9AEEEB3A` | `9aeeeb3a235014a09994ce5dc6c441edcdd9f1b3` | `equivalent` | Merge pull request #848 from ranxi2001/feat/moderation-upstream-account-scope；Fork 保留集中式审核/安全适配层；请求门禁和协议回归覆盖安全契约。 | 同步负责人 |
| 16 | `SYNC-37DC4620` | `37dc4620a41a68abaeab329358c15821c41e3f7f` | `migrated` | Merge pull request #849 from ranxi2001/fix/issue817-ack-fencing-20261114；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 17 | `SYNC-02C30B74` | `02c30b748944dd7a405b33f3757293d15d7a5578` | `equivalent` | Merge pull request #834 from ranxi2001/fix/agent409-readable-continuation；Fork 通过本地 HTTP/WS 恢复和请求账本实现等效行为；网关与归属回归覆盖。 | 同步负责人 |
| 18 | `SYNC-F6AC19FF` | `f6ac19ffe390124e3a951f4d107ecbdad591317b` | `equivalent` | Merge pull request #851 from ranxi2001/fix/responses-moderation-continuation-20261114；Fork 保留集中式审核/安全适配层；请求门禁和协议回归覆盖安全契约。 | 同步负责人 |
| 19 | `SYNC-602808E1` | `602808e179aded7d908d01a79e2f6572e3209298` | `equivalent` | Merge pull request #852: 原账号限流或过载时继续重建可读会话；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 20 | `SYNC-AA8E596E` | `aa8e596e6d6f3e4648d6beba73b39b240dd5357a` | `equivalent` | chore: release 2.21.3；仅上游发布/CI/fixture 或版本元数据变更；Fork 生产契约不变，已由来源锁与仓库门禁覆盖。 | 同步负责人 |
| 21 | `SYNC-AA0B1ABF` | `aa0b1abfe5b32c483b32c70fe0c813659ef58083` | `equivalent` | Merge pull request #857 from ranxi2001/fix/bps-context-boundaries-20261115；Fork 保留 BPS 适配层；请求、工具、搜索和诊断契约已由 BPS 回归覆盖。 | 同步负责人 |
| 22 | `SYNC-999D599F` | `999d599f411040de435a012765bff031f72c5df1` | `equivalent` | Merge pull request #856 from ranxi2001/feat/bps-tool-choice-20261115；Fork 保留 BPS 适配层；请求、工具、搜索和诊断契约已由 BPS 回归覆盖。 | 同步负责人 |
| 23 | `SYNC-E90811E2` | `e90811e27bc442763a496027f935a7bb1fb4396b` | `equivalent` | Merge pull request #858 from ranxi2001/feat/bps-account-search-20261115；Fork 保留 BPS 适配层；请求、工具、搜索和诊断契约已由 BPS 回归覆盖。 | 同步负责人 |
| 24 | `SYNC-713CB0F3` | `713cb0f3efe3f838d159cd57998128c2cdaa84b1` | `equivalent` | Merge pull request #860: 修复重建会话的规范化冲突与 WS 首帧恢复；Fork 通过本地 HTTP/WS 恢复和请求账本实现等效行为；网关与归属回归覆盖。 | 同步负责人 |
| 25 | `SYNC-236200C6` | `236200c670cbc7d674aeb484d1405fc0e96e9f7a` | `migrated` | Merge pull request #850 from ranxi2001/feat/mature-channel-community-ops；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 26 | `SYNC-7D87624A` | `7d87624a908fffa7940f0493c3070e8e4aa99dae` | `equivalent` | Merge pull request #863 from Cd1s/fix/antigravity-strip-claude-code-security-policy；Fork 保留集中式审核/安全适配层；请求门禁和协议回归覆盖安全契约。 | 同步负责人 |
| 27 | `SYNC-0293BFA7` | `0293bfa7977ce229a8a2cc2119fdfc98e72a6cf3` | `equivalent` | Merge pull request #859 from ranxi2001/feat/bps-search-settings-20261115；Fork 保留 BPS 适配层；请求、工具、搜索和诊断契约已由 BPS 回归覆盖。 | 同步负责人 |
| 28 | `SYNC-EFB19A4D` | `efb19a4df2ed1d3828f5464d585824ea3fc38f31` | `equivalent` | Merge pull request #862 from ranxi2001/feat/account-ops-multi-email-20261115；Fork 采用本地账号运维/认证边界；对应管理端 API、仓库和前端契约已覆盖。 | 同步负责人 |
| 29 | `SYNC-AB9B4D91` | `ab9b4d9135a246b395aec45a22e184388fe73c96` | `equivalent` | Merge pull request #861 from spake404/feat/magpie-import-upstream；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 30 | `SYNC-820D349F` | `820d349f701c7e9c6420dcd63f61c6c94858d240` | `equivalent` | Merge pull request #853 from ranxi2001/fix/issue842-nonbeta-normalization-20261115；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 31 | `SYNC-36DD3D1B` | `36dd3d1b6b3aff63dff8ab025205a3032878a9ec` | `equivalent` | Merge pull request #855 from ranxi2001/fix/issue854-ws-owner-continuation；Fork 通过本地 HTTP/WS 恢复和请求账本实现等效行为；网关与归属回归覆盖。 | 同步负责人 |
| 32 | `SYNC-0E8E97AA` | `0e8e97aa23afd8e88a61ea64a84a8fb84f1bfdc6` | `equivalent` | chore: release v2.21.4；仅上游发布/CI/fixture 或版本元数据变更；Fork 生产契约不变，已由来源锁与仓库门禁覆盖。 | 同步负责人 |
| 33 | `SYNC-C115BEF9` | `c115bef917eae22fc2f5a4d0ba6ac6cbe600e8e4` | `equivalent` | Merge pull request #864 from ranxi2001/fix/free-fast-customer-cost-details；Fork 保留本地计费/投影模型；客户侧契约由计费、余额、用量和价格回归覆盖。 | 同步负责人 |
| 34 | `SYNC-70413F14` | `70413f1404f4f09515223c6e941cd4458a2eb32f` | `equivalent` | Merge pull request #868 from ranxi2001/fix/qq-bot-smart-ops-sidebar-20261115；Fork 采用本地账号运维/认证边界；对应管理端 API、仓库和前端契约已覆盖。 | 同步负责人 |
| 35 | `SYNC-4C660794` | `4c660794b89279dcbf699275091340eb2c87959a` | `equivalent` | Merge pull request #867 from ranxi2001/fix/rpm-http409-projection-20261115；Fork 通过本地 HTTP/WS 恢复和请求账本实现等效行为；网关与归属回归覆盖。 | 同步负责人 |
| 36 | `SYNC-E6B2AFDE` | `e6b2afde0485301a14f8dc653a87262fbbfd8e21` | `equivalent` | Merge pull request #866 from ranxi2001/fix/ws-owner-conflict-safe-continuation-20261115；Fork 通过本地 HTTP/WS 恢复和请求账本实现等效行为；网关与归属回归覆盖。 | 同步负责人 |
| 37 | `SYNC-4BB98D75` | `4bb98d75f270f46f70f5c9d0d12bea5822dd01c6` | `equivalent` | Merge pull request #869: 移除余额耗尽预测后台扫描；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 38 | `SYNC-B9FEADF7` | `b9feadf7dd6228a4117e360821d9994b7cc54897` | `equivalent` | chore: release v2.21.5；仅上游发布/CI/fixture 或版本元数据变更；Fork 生产契约不变，已由来源锁与仓库门禁覆盖。 | 同步负责人 |
| 39 | `SYNC-FF1082FD` | `ff1082fd51038f10dcb3a8ecb1d5a738258b0f66` | `equivalent` | Merge pull request #870 from ranxi2001/fix/ws-plaintext-response-owner-20261116；Fork 通过本地 HTTP/WS 恢复和请求账本实现等效行为；网关与归属回归覆盖。 | 同步负责人 |
| 40 | `SYNC-8D8661D1` | `8d8661d1c9d06781cf171cf29aa666a225105bc4` | `equivalent` | Merge pull request #872 from ranxi2001/fix/issue871-bps-search-diagnostics-20261116；Fork 保留 BPS 适配层；请求、工具、搜索和诊断契约已由 BPS 回归覆盖。 | 同步负责人 |
| 41 | `SYNC-5D7EBB9B` | `5d7ebb9bf5d4b680c524098c39bf3aeb13b80eb1` | `equivalent` | Merge pull request #874: OAuth 首次配置后自动处置 API Key 账号调度；Fork 采用本地账号运维/认证边界；对应管理端 API、仓库和前端契约已覆盖。 | 同步负责人 |
| 42 | `SYNC-2927209F` | `2927209f5492d8cb690b83a1964b2f093c19ad7a` | `equivalent` | Merge pull request #875 from ranxi2001/fix/ws-bridge-residual-409-20261116；Fork 通过本地 HTTP/WS 恢复和请求账本实现等效行为；网关与归属回归覆盖。 | 同步负责人 |
| 43 | `SYNC-F3E2F333` | `f3e2f3333a023d53388e4f7597163f9abece33f3` | `equivalent` | Merge pull request #877 from ranxi2001/fix/ws-unsent-delta-context-20261116；Fork 通过本地 HTTP/WS 恢复和请求账本实现等效行为；网关与归属回归覆盖。 | 同步负责人 |
| 44 | `SYNC-DA526BFB` | `da526bfbab47efd40549cab98f1069f9b2b45b9f` | `equivalent` | Merge pull request #878: 免费 UltraFast 分组计费开关；Fork 保留本地计费/投影模型；客户侧契约由计费、余额、用量和价格回归覆盖。 | 同步负责人 |
| 45 | `SYNC-C16DA7D8` | `c16da7d8658787b5fa88f3ad23108ced1a8ac49c` | `equivalent` | Merge pull request #879 from ranxi2001/fix/maintenance-playwright-runner-20261116；仅上游发布/CI/fixture 或版本元数据变更；Fork 生产契约不变，已由来源锁与仓库门禁覆盖。 | 同步负责人 |
| 46 | `SYNC-F260E305` | `f260e3055c95930aabf06c4f9689dfb2d916f52a` | `equivalent` | Merge pull request #880 from ranxi2001/fix/ws-followup-capture-20261116；Fork 通过本地 HTTP/WS 恢复和请求账本实现等效行为；网关与归属回归覆盖。 | 同步负责人 |
| 47 | `SYNC-41778EFE` | `41778efe6f7cb28dd2c70a4b70c6c8b6f04a352c` | `equivalent` | Merge pull request #882 from ranxi2001/fix/issue881-totp-factor-binding；Fork 采用本地账号运维/认证边界；对应管理端 API、仓库和前端契约已覆盖。 | 同步负责人 |
| 48 | `SYNC-58628D98` | `58628d98b9e0781c1b8e3df6259ab487a1490f70` | `equivalent` | Merge pull request #884 from ranxi2001/fix/ws-inherited-model-proof-20261116；Fork 通过本地 HTTP/WS 恢复和请求账本实现等效行为；网关与归属回归覆盖。 | 同步负责人 |
| 49 | `SYNC-AF24E0F1` | `af24e0f1add122a95d46ad3cc02c63d26ea9f6bf` | `equivalent` | Merge pull request #883 from ranxi2001/fix/issue871-search-business-followup-20261116；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 50 | `SYNC-E3F238E3` | `e3f238e3f173271a82b43d7fd504462c553b495c` | `equivalent` | Merge pull request #886: 保留 Gemini 结构化输出约束并拒绝静默降级；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 51 | `SYNC-DC3A5403` | `dc3a5403be7053397c43c2ef4b1e4fff01536433` | `equivalent` | Merge pull request #887 from ranxi2001/fix/gpt62-max-effort-20261116；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 52 | `SYNC-D4CD6642` | `d4cd664271d822e0c1743802e25cdd8196f97aa8` | `equivalent` | fix: 清理历史 CI、消除重复检查并提高前端堆上限 (#890)；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 53 | `SYNC-C269390A` | `c269390a8d2a3ecd718ec6d71f24bc808e1bb52b` | `equivalent` | feat: 用户管理显示当前并发与 RPM (#889)；Fork 保留本地计费/投影模型；客户侧契约由计费、余额、用量和价格回归覆盖。 | 同步负责人 |
| 54 | `SYNC-5CC5D608` | `5cc5d608c55f4cb37ad184f5cdfa647354c94390` | `equivalent` | fix: CI 按改动范围运行并复用相同输入的成功结果 (#893)；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 55 | `SYNC-D7D611CF` | `d7d611cf3054ba66cbd92c935a0dbe706e994771` | `equivalent` | fix: 避免恢复等待截止掩盖独立存储错误 (#891) [skip ci]；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 56 | `SYNC-EF34FDB2` | `ef34fdb20e3ca142b230042b9de3b71e9c9e6335` | `equivalent` | fix: 支持 HTTP 与 WS 丢弃冲突历史后续接 (#888) [skip ci]；Fork 通过本地 HTTP/WS 恢复和请求账本实现等效行为；网关与归属回归覆盖。 | 同步负责人 |
| 57 | `SYNC-65010E17` | `65010e176e5e0695791995072c3861e40c6a68de` | `equivalent` | feat: OpenAI 账号按请求多代理分流及刷新出口保护 (#885) [skip ci]；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 58 | `SYNC-321F11C1` | `321f11c1a4d23302961c223ac030fd09e3f9e6e8` | `equivalent` | chore: release v2.21.6 with 6 GiB frontend build heap [skip ci]；仅上游发布/CI/fixture 或版本元数据变更；Fork 生产契约不变，已由来源锁与仓库门禁覆盖。 | 同步负责人 |
| 59 | `SYNC-DD84AC1D` | `dd84ac1da71391a9a112aec89f549c188a4ebe97` | `equivalent` | Merge pull request #894 from pixcook-ai/codex/pr885-historical-fixture-followup-20261117；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 60 | `SYNC-5C48F4B3` | `5c48f4b324dd794b1d51d85755d488650ad62825` | `equivalent` | Merge pull request #898 from ranxi2001/fix/sol61-no-default-instructions-20261117；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 61 | `SYNC-9C0EE229` | `9c0ee2298661da0a12b4109511a4b4345a3e5d08` | `equivalent` | Merge pull request #896 from ranxi2001/fix/user-live-load-review-followup-20261117；Fork 采用本地账号运维/认证边界；对应管理端 API、仓库和前端契约已覆盖。 | 同步负责人 |
| 62 | `SYNC-58A54543` | `58a54543fa385d603176ff2916104f3e01bbb4f4` | `equivalent` | Merge pull request #897 from ArnonEnbar/fix/api-key-range-usage-sort；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 63 | `SYNC-4B78BCEA` | `4b78bceae41c42b9e1241cb0a7515f4b314222d0` | `equivalent` | Merge pull request #895 from ranxi2001/fix/issue892-timezone-integrity-20261117；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 64 | `SYNC-8750EC19` | `8750ec1956be7c8de327e384c884bb737f943017` | `equivalent` | Merge pull request #903 from ranxi2001/fix/pr899-platform-credential-takeover；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 65 | `SYNC-E1C28B37` | `e1c28b3704859bbcff2e3258ee092fc551151e6b` | `equivalent` | Merge pull request #901 from ranxi2001/fix/lossy-compaction-tool-history-20261117；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 66 | `SYNC-A2ED6930` | `a2ed69304aeb20d1ae007d00845c548abdd2ccf3` | `equivalent` | Merge pull request #900 from ranxi2001/cherry/upstream-v0.2.22-pr；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 67 | `SYNC-4BA33F87` | `4ba33f8713cbf6c6406e0f2639ffcc2ef1e31c87` | `equivalent` | Merge pull request #904 from ranxi2001/release/v2.22.0-20261117；仅上游发布/CI/fixture 或版本元数据变更；Fork 生产契约不变，已由来源锁与仓库门禁覆盖。 | 同步负责人 |
| 68 | `SYNC-DCB228EB` | `dcb228eb8f077a52a223fb998bc0c83e073e7394` | `equivalent` | Merge pull request #906 from ranxi2001/fix/issue905-ws-upstream-close；Fork 通过本地 HTTP/WS 恢复和请求账本实现等效行为；网关与归属回归覆盖。 | 同步负责人 |
| 69 | `SYNC-D1EC21EB` | `d1ec21ebd35a19530dbb77e9e89e195e5a8a8c4a` | `equivalent` | Merge pull request #911 from ranxi2001/fix/openai-official-pricing-20261118；Fork 保留本地计费/投影模型；客户侧契约由计费、余额、用量和价格回归覆盖。 | 同步负责人 |
| 70 | `SYNC-883FA395` | `883fa395dc06e6dbf6c9529ca8caf7e3197bcf1b` | `equivalent` | Merge pull request #912 from ranxi2001/fix/issue905-pool-session-affinity；Fork 通过本地 HTTP/WS 恢复和请求账本实现等效行为；网关与归属回归覆盖。 | 同步负责人 |
| 71 | `SYNC-D01DD5C2` | `d01dd5c26f239e9fdb18e8f048a4e0f207d7b964` | `equivalent` | Merge pull request #910: 修复 WS 模型冷却与上下文安全；Fork 通过本地 HTTP/WS 恢复和请求账本实现等效行为；网关与归属回归覆盖。 | 同步负责人 |
| 72 | `SYNC-1C345720` | `1c345720d465206606bb680ea6fc5045ed6bb0ec` | `migrated` | Merge pull request #913 from spake404/cursor/profit-ops-spend-windows-e407；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 73 | `SYNC-693D4662` | `693d46621fd493849135373c7b2513ef635488c9` | `equivalent` | Merge pull request #915 from ranxi2001/ci/go-test-sharding-20261118；仅上游发布/CI/fixture 或版本元数据变更；Fork 生产契约不变，已由来源锁与仓库门禁覆盖。 | 同步负责人 |
| 74 | `SYNC-701612F6` | `701612f6e9f7b697eb36397b9fb49bf7324e56a1` | `equivalent` | Merge pull request #909 from Yukino-Akane/codex/cache-ttl-billing-from-request；Fork 保留本地计费/投影模型；客户侧契约由计费、余额、用量和价格回归覆盖。 | 同步负责人 |
| 75 | `SYNC-657BB38D` | `657bb38d141901abcbeef2f178d9234109d37167` | `equivalent` | Merge pull request #916 from ranxi2001/fix/issue905-http-bridge-warmup；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 76 | `SYNC-52A745D9` | `52a745d9405cc41ad64edf41c327fb978017f7ff` | `equivalent` | Merge pull request #923: 修复工具增量续接及 Pod 取消归因；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 77 | `SYNC-73B958AD` | `73b958ad07babaf56a3f998d8c682e0f84525f4a` | `equivalent` | Merge pull request #925 from Roins-hub/fix/account-test-managed-connection；Fork 采用本地账号运维/认证边界；对应管理端 API、仓库和前端契约已覆盖。 | 同步负责人 |
| 78 | `SYNC-EE7A978A` | `ee7a978ac5a86c3f32777b3fc99ae1f021841ac7` | `equivalent` | Merge pull request #919 from ranxi2001/maintenance/pr916-proof-20261118；仅上游发布/CI/fixture 或版本元数据变更；Fork 生产契约不变，已由来源锁与仓库门禁覆盖。 | 同步负责人 |
| 79 | `SYNC-2552CCD0` | `2552ccd084f48846f1a8a756062acae48366781c` | `equivalent` | Merge pull request #920 from ranxi2001/fix/issue917-initial-sse-cooldown；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 80 | `SYNC-BCF4CC37` | `bcf4cc37dc4433c747c871b1b3b8fed62c1aa13a` | `equivalent` | Merge pull request #927: 多分组余额不足返回明确错误码；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 81 | `SYNC-812A1649` | `812a164957a42d9f1977d8d4b405ed1fc9c681ab` | `equivalent` | Merge pull request #922 from ranxi2001/cherry/upstream-v0.2.23；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 82 | `SYNC-C2A335CE` | `c2a335ce59356b5f4bb323b34b42a5ce4c115195` | `equivalent` | Merge pull request #918 from ranxi2001/feat/account-batch-upstream-model-sync；Fork 采用本地账号运维/认证边界；对应管理端 API、仓库和前端契约已覆盖。 | 同步负责人 |
| 83 | `SYNC-B6404034` | `b6404034c9234e0ae552c419454a14d8f5cb59b8` | `equivalent` | Merge pull request #928 from ranxi2001/maintenance/issue926-responses-gemini-20261118；仅上游发布/CI/fixture 或版本元数据变更；Fork 生产契约不变，已由来源锁与仓库门禁覆盖。 | 同步负责人 |
| 84 | `SYNC-1DCD7628` | `1dcd7628743701918f331b26c85aaceb16201b10` | `equivalent` | Merge pull request #924 from Yukino-Akane/codex/relogin-interaction-diagnostics；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 85 | `SYNC-DA85B1C2` | `da85b1c2e2343f8079f646b5e13c9b75a3b21639` | `equivalent` | Merge pull request #930: VIP 自动垫付金额显示两位小数；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 86 | `SYNC-E687827F` | `e687827fcb5c2bbb347dfb1d15f116c0ab0580c9` | `equivalent` | Merge pull request #929 from ranxi2001/fix/remove-token-guard-harvest-pages-20261118；Fork 采用本地账号运维/认证边界；对应管理端 API、仓库和前端契约已覆盖。 | 同步负责人 |
| 87 | `SYNC-F5F96482` | `f5f9648277e9e8b600b90f86e14ec739fd203c50` | `equivalent` | Merge pull request #931: 对齐官方 UltraFast 促销与长上下文计费；Fork 保留本地计费/投影模型；客户侧契约由计费、余额、用量和价格回归覆盖。 | 同步负责人 |
| 88 | `SYNC-338F73DF` | `338f73dfa31032972d8860adab9c25016115c18d` | `equivalent` | Merge pull request #932 from ranxi2001/fix/issue817-save-result-ack-20261119；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 89 | `SYNC-5BEF330D` | `5bef330d5edf039fe390a2b17bb9dc2e5731ba92` | `equivalent` | Merge pull request #933: 准备发布 v2.23.0；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
| 90 | `SYNC-A71BCD62` | `a71bcd6238777630bb74f18106097689ce7a5836` | `migrated` | Merge pull request #935 from ranxi2001/fix/account-balance-notification-mute-20261119；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 91 | `SYNC-270AE171` | `270ae1710353371552dd9f11be0dc3374de4bc9d` | `migrated` | Merge pull request #934: 增加 OpenAI 旧版兼容模式；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 92 | `SYNC-45EBCB70` | `45ebcb708867dd61caae10435d19d2df17cd41f3` | `migrated` | Merge pull request #936 from ranxi2001/fix/newapi-query-user-agent-20261119；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 93 | `SYNC-FEB76154` | `feb761541918d25a73779c6ea42ee28f61b5591e` | `migrated` | Merge pull request #937 from ranxi2001/fix/issue817-persistence-followup-20261119；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 94 | `SYNC-AFCEF2D6` | `afcef2d6811cb860b04dcba45ae1c6477671f5bc` | `migrated` | Merge pull request #939: 生图 usage 计费保底与结算金额折算回包；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 95 | `SYNC-6ED27208` | `6ed27208b7dc1f0c3e615f7c7b4e00ad16aec309` | `migrated` | Merge pull request #942 from akihitohyh/fix/usage-csv-export-rate-limit；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 96 | `SYNC-587CCB28` | `587ccb28e498c3ac8101306e1a5c565412cd802c` | `migrated` | Merge pull request #943 from Yukino-Akane/feat/pelican-preview-pointer-lock；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 97 | `SYNC-009EEFC2` | `009eefc27c1dd02f27b39f54f23ed1e51d24b300` | `migrated` | Merge pull request #940 from ranxi2001/maintenance/pr938-safe-compression-20261119；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 98 | `SYNC-8F4E0D2B` | `8f4e0d2b225df1f53d83fe3a66e7f62451f91b24` | `migrated` | Merge pull request #945 from ranxi2001/fix/newapi-wallet-currency-20261119；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 99 | `SYNC-5A519971` | `5a51997112d0f93c8ec06747748680a9d7e8e4a7` | `migrated` | Merge pull request #944 from ranxi2001/fix/account-performance-query-load-20261119；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 100 | `SYNC-72E4545A` | `72e4545a0f1232a1ded49dac3b6383700089569c` | `migrated` | Merge pull request #948 from ranxi2001/fix/issue-947-group-dropdown-width；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 101 | `SYNC-4F1BFA96` | `4f1bfa96d760711197b692ce94d8f11f0f895c58` | `migrated` | Merge pull request #949: 统一渠道配置并分离钱包成本模式；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 102 | `SYNC-D846D9E5` | `d846d9e592c3a472074b92f2f041f90798bc0025` | `migrated` | Merge pull request #952 from QQQHHHDDD/fix/minimax-h3-model-aliases；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 103 | `SYNC-48DEE26A` | `48dee26a76d9430a6e883a66d069075e921a91e9` | `migrated` | Merge pull request #950 from ranxi2001/fix/image-billing-followup-20261119；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 104 | `SYNC-B107D07D` | `b107d07da2af136995f154b57ff67f5597018844` | `migrated` | Merge pull request #951 from ranxi2001/codex/encrypted-controls-integrity-diagnostics；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 105 | `SYNC-7563B886` | `7563b886943a510e8d2e0bbd0af5618df2cecff7` | `migrated` | Merge pull request #955 from ranxi2001/fix/profit-snapshot-954；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 106 | `SYNC-3B79C7D9` | `3b79c7d9a9a2f1a7b46c1c993e4b86812535cfb7` | `migrated` | Merge pull request #953 from QQQHHHDDD/feat/openai-compatible-videos；已完成迁入；运行时实现、回归测试和逐 SHA 审计锚点均已记录。 | 同步负责人 |
| 107 | `SYNC-00E00796` | `00e00796080c632188cf9df7912050b144a1a44f` | `equivalent` | Merge pull request #956 from ranxi2001/release/prepare-v2.23.1；已完成逐 SHA diff 审查；未发现 Fork 支持范围内缺失的运行时契约，现有全量门禁覆盖。 | 同步负责人 |
## 关闭规则与交付记录

- 仅能按实证将条目改为 `migrated`、`equivalent` 或有审批引用的 `approved_exclusion`。当前 107 项均已满足终态条件，后续修复继续回写原 SHA 跟踪号。
- 更新账本同时核对 TSV SHA256、source-lock、管理员版本状态；未知的 CI、部署、对账或本机验收继续记 pending。
- 每次部署记录 PR、合并 SHA、Zeabur deployment ID 与 SHA、健康/关键 API、迁移数据核对和游客/普通用户/管理员本机验收。
- 以前反复“部分完成”的原因：按功能分批实现，却以累计提交范围判断整版；过去开放项没有 CI 强制阻止同步 PR 放行；有些能力只做了配置/页面而未做运行时和历史账务闭环；上游持续发布导致版本范围容易漂移。
- `--require-merge-ready` 防止开放项被误合并/部署；`--require-complete` 防止未验收却标记整版完成。人工审查、责任人分配和真实数据验收仍必须完成。
