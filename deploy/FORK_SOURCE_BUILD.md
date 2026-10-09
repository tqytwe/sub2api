# tqytwe Fork：更新、构建与回退

> 状态：active。软件源码仅跟进 `ranxi2001/sub2api` 的正式 release pin；
> 生产源码仍为 `tqytwe/sub2api` 的 `play/main`。本页不表示任何发布产物已获验证。

## 当前可用路径

管理界面的版本检查只展示 ranxi 正式发布，比较对象是已锁定的上游版本，
不是独立编号的 fork 构建版本。看到上游新版本不代表可以直接安装。
`build_type=release` 也不能证明产物包含自研功能。

当前没有获准在线安装的、经过来源与完整性验证的 tqytwe 产物。
`POST /api/v1/admin/system/update`、有或无版本参数的
`POST /api/v1/admin/system/rollback` 均返回 HTTP 409
`FORK_SOURCE_DEPLOYMENT_REQUIRED`，不下载文件、不替换程序，也不恢复旧 `.backup`。
回退列表为空。旧 Redis key 及旧来源 payload 不会复用；相同来源的缓存仍可在
GitHub 暂时不可用时展示，并明确提示查询警告。

## 从已审查的 fork 提交构建

1. 按 [同步手册](../docs/UPSTREAM_SYNC_PLAYBOOK.md) 核验正式来源、完整 SHA、
   差异及自研定制；经过测试和 PR 审查后才合入 `play/main`。
2. 从 **tqytwe/sub2api** 获取明确的 40 位已审查 commit，在独立 worktree 构建。
   不用 ranxi/Wei-Shaw 原版源码、原版二进制或原版镜像替代 fork。
3. 示例只产生本地镜像，不推送、不启动容器、不修改生产配置：

```bash
# REVIEWED_FORK_SHA 由发布协调者提供，必须是已审查的完整 commit。
: "${REVIEWED_FORK_SHA:?set the reviewed fork commit}"
git clone --no-tags --single-branch --branch play/main https://github.com/tqytwe/sub2api.git sub2api-fork
cd sub2api-fork
git cat-file -e "${REVIEWED_FORK_SHA}^{commit}"
test "$(git rev-parse origin/play/main)" = "$REVIEWED_FORK_SHA"
git worktree add --detach ../sub2api-fork-build "$REVIEWED_FORK_SHA"
cd ../sub2api-fork-build
docker build --build-arg COMMIT="$REVIEWED_FORK_SHA" --tag "sub2api-fork-local:$REVIEWED_FORK_SHA" .
```

`sub2api-fork-local` 是在本机刚构建的标签，**不是公共仓库地址**。
源码构建也可先安装锁定依赖，执行 `make test`、`make build`；嵌入前端的二进制
需先 `pnpm --dir frontend build`，再按 Dockerfile/GoReleaser 的 `-tags=embed`
和版本/commit ldflags 构建。不要把不含前端资源的默认后端 build 误当完整发行包。

生产仍由 `origin/play/main` 触发既有 Zeabur 流程；发布协调者核对实际部署 commit、
健康状态和 API，由用户在本地电脑浏览器完成最终验收。上述示例不是部署授权，
不能把本地镜像名粘贴进真实生产配置并假设远端可以拉取。

## fork 发布能力与 dry run

`.github/workflows/release.yml` 保留现有完整/简化构建、owner-derived GHCR 和
可选 DockerHub 发布、按平台 archive/commit/SHA256 核验。

- 此版本不再监听 `v*` tag push。源码导入必须使用 `--no-tags`，只推审查分支，
  禁止将上游 `v*` tag 推到 fork。历史 tag 自带的旧工作流不受新文件的追溯保护；
  仓库级 tag/ruleset 若需变更，由主线程单独协调，本变更不修改真实仓库设置。
- 手动选择运行分支 `play/main`；dry run 默认为 **true**，输入可为审查分支或完整 SHA。
  dry run 只构建和上传 CI 证据，不登录 registry、不推镜像、不发 release/通知，
  不写入也不推送 VERSION。
- 真发布必须显式取消 dry run，repository 必须为 `tqytwe/sub2api`，所选 tag
  必须指向当前 `origin/play/main` 的完整 commit，源码 VERSION 必须与 tag 相同。
  还必须存在经审查的 `docs/upstream-migrations/source-lock.json`（由 PR #338 引入），
  并且 UpdateService 的上游基线与锁定版本一致；否则拒绝发布。
  VERSION 的变更必须先走 `play/main` PR。工作流不创建 tag、不创建授权、不直接推分支。
- 发布前再次读取 `play/main`，用 `ls-remote` 核对远端 fork tag 的实际 commit，
  并验证 tag/commit/VERSION；分支前进或标签删除/漂移时本次发布会拒绝，
  由发布协调者重新确认目标。不要通过回写旧分支或强推来满足校验。
- `release-provenance.json` 记录仓库、源码 commit、版本、工作流 SHA/run ID 和各平台
  archive hash，以及来源锁快照与 SHA256；full release 附带该文件，simple release 留在 CI artifact，镜像携带
  OCI source/revision/version。保留的上游 pin 是审查来源，不自动证明源码已整合或产物已验收。
- GHCR/DockerHub 地址由实际仓库 owner/凭据派生。只有发布成功并核对产物 digest、
  溯源及 fork 定制之后才能使用真实的镜像地址；本页不承诺任何尚未存在的镜像。

以后若恢复在线安装，必须另行实现并测试 fork 产物的 commit、来源证明、SHA256、
平台匹配及回退备份来源校验，不能只改下载仓库名或打开前端按钮。

## 回退

优先重新部署已验证的前一个 fork 镜像 digest/部署记录，或经 PR revert 问题合并，
从已审查的 fork commit 重新构建。核对数据兼容性和部署健康；本变更不执行数据库
回退、不改变迁移、不修改真实环境设置。禁止使用不带来源证明的本地 `.backup`。

## 旧来源分类（不作批量替换）

| 位置 | 来源含义 | fork 的处理 |
| --- | --- | --- |
| `deploy/install.sh`、`docker-deploy.sh` 中的 Wei-Shaw URL | 原版分发/安装路径 | 仅保留为原版参考，不能用于本 fork 升级或回退 |
| `docker-compose*.yml` 中 `weishaw/sub2api`、`zeabur.template.yaml`、Apple 示例 | 原版镜像/模板 | 不是 tqytwe 镜像，不盲换成不存在的 fork 镜像；按上文构建并验证 |
| `deploy/docker-compose.server.yml` 的 `sub2api:local` | 既有本地源码构建模式 | 不代表公共镜像，检查它实际使用的 fork commit |
| Dockerfile 的 OCI source | 当前源码产物归属 | 标记 tqytwe；作者/maintainer 历史署名保留 |
| Go module/import、generated、LICENSE、作者与历史记录、原 CLA/法律确认 | 兼容标识与历史/法律归属 | 保留，不是软件更新源 |
| `Wei-Shaw/model-price-repo` | 独立模型价格数据源 | 保留，不是 Sub2API 软件更新源 |
| 供应商客户端版本同步、API upstream host | 第三方协议/客户端兼容信息 | 保留，不属于本次软件来源变更 |

移动端应用与 Canvas 不在本次范围内。
