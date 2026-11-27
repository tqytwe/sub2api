# v2.23.1 生产验收记录

> 核验时间：2026-11-27 10:58（Asia/Tokyo）
>
> 生产 revision：`4ba7e6a64c44a2f676fd3ff5fff56322f4a8fd24`

## 部署基线

- PR：[ #460](https://github.com/tqytwe/sub2api/pull/460) 已合并。
- GitHub production deployment：`8096214204`，状态 `success`，SHA 与生产 revision 一致。
- Zeabur deployment：`6b08a34b9a6efec056d2e9a8`，状态 `success`。
- `GET https://www.jisudeng.com/health`：HTTP `200`，响应 `{"status":"ok"}`。
- 匿名 `GET /api/v1/admin/system/check-updates`：HTTP `401`，符合管理员认证边界。

## G5-GUEST：已完成

验收方式为 Chromium `141.0.7390.37` 无头浏览器；每个场景使用全新上下文，不携带 Cookie、登录态或业务凭证。页面错误数为 0。

| 场景 | 结果 | 观测 |
|---|---|---|
| `/` | 通过 | HTTP `200`；首页标题、导航、模型数量和系统状态内容可见 |
| `/catalog` | 通过 | HTTP `200`；模型广场、平台/分组/倍率筛选和模型列表可见 |
| `/login` | 通过 | HTTP `200`；邮箱、密码、登录和注册入口可见 |
| `/dashboard` | 通过 | 未登录跳转到 `/login?redirect=/dashboard` |
| `/admin/dashboard` | 通过 | 未登录跳转到 `/login?redirect=/admin/dashboard` |

## 尚不能关闭的验收项

- **G5-DATA：待授权生产只读核对。** 当前执行环境没有生产数据库连接，未读取生产表、凭证或用户数据；因此没有假造 `431_provider_profit_links.sql` 的应用结果、覆盖率、`cost_mode` 分布或历史成本汇总。
- **G5-USER：待普通用户登录验收。** 当前没有可用的生产普通用户会话。
- **G5-ADMIN：待管理员登录验收。** 当前没有可用的生产管理员会话；匿名 `401` 不能替代版本弹窗、新管理功能和保存/刷新验收。

上述限制不影响代码差异已闭环、精确 SHA 已部署或游客边界已通过，但在三项证据补齐前，整版继续保持 `deployed_partial` / `fully_migrated=false`。
