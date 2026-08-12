# Android Google Play Billing 接入手册

> 状态：active
> 最后核验：2026-08-12
> 适用范围：Google Play 版 Android APP 的数字权益购买、服务端验单与后台商品映射。

## 目标

Google Play 版销售余额、套餐、会员等数字权益时，首发按 Google Play Billing 闭环实现。极速蹬后端只保存“Google Play 商品 ID → 平台权益”的映射，不在后台保存 Google 私钥，也不在首发版本里自动创建 Play Console 商品。

国内版 APP 继续使用国内支付与兑换码闭环；Play 全球首发版默认不展示微信、USDT、PayNow、第三方店铺等外部数字权益购买入口。若后续单独面向美国用户启用替代支付或外链，必须先做地区识别、政策文本、价格展示、数据安全和提审材料专项评审，不能与本首发 Play Billing 闭环混在同一开关里。

## 首发范围与二期边界

Play 首发只做 Google Play Billing 的一次性商品闭环：

- 余额充值：Google 一次性商品 → 平台余额到账。
- 固定期限套餐：Google 一次性商品 → 平台 30 天、90 天等固定期限套餐。
- 兑换码：Play 版可保留兑换码输入，但不得在 Play 版引导用户跳转外部渠道购买数字权益。

Google 管理的自动续费订阅列入二期，不进入首发。二期必须补齐订阅状态同步、续费、取消、退款、扣费失败、宽限期、恢复订阅、Real-time Developer Notifications 和账务对账后，才允许把 `product_type=subs` 作为生产商品类型启用。

## 谁负责什么

| 环节 | 管理位置 | 说明 |
|------|----------|------|
| 商品 ID、价格、地区、状态 | Google Play Console | 首发手动创建。商品 ID 必须与后端映射完全一致。 |
| 商品 ID 对应充多少余额/开哪个套餐 | 极速蹬后台 `/admin/orders/play-billing` | 可运行时调整，保存到系统设置 `MOBILE_PLAY_BILLING_PRODUCTS_JSON`；API 可用于自动化。 |
| Google Android Publisher 凭据 | 服务器环境变量/密钥文件 | 只用于服务端验单，禁止放入 Git、后台页面或 APP。 |
| APP 展示商品 | `/api/v1/payment/checkout-info` | 返回 `play_billing_products`，APP 不应写死权益映射。 |
| APP 提交购买 | `/api/v1/mobile/play-billing/purchases` | 后端用 Google API 验 purchase token 后再创建并履约订单。 |

## 后台商品映射

运营后台入口：

```text
支付管理 → Play 内购
/admin/orders/play-billing
```

页面可以新增、启停、复制、删除商品映射：

- 一次性余额商品：`product_type=inapp`、`order_type=balance`，填写到账余额 `amount`。
- 固定期限平台套餐：首发可用 `product_type=inapp`、`order_type=subscription`，选择平台订阅套餐 `plan_id`。这类商品通常不消耗，但需要购买确认；后端会在验单结果里要求 APP `acknowledge`，不是 `consume`。
- Google 自动续费订阅：只有确实启用 Google recurring subscription 时才用 `product_type=subs`。

后台页面只展示 `package_name` 和 `service_account_configured`，不读取、不展示、不保存 Google 私钥。服务账号 JSON 仍必须走服务器环境变量或 secret 文件。

### API

### 读取配置

```http
GET /api/v1/admin/payment/play-billing/config
Authorization: Bearer <admin jwt>
```

响应会包含：

- `package_name`：当前验单包名，默认 `com.jisudeng.chat`
- `service_account_configured`：服务端是否配置了 Google 验单凭据
- `config_source`：`settings` 或 `env`
- `products`：后台完整映射
- `public_products`：APP 可展示的启用商品

### 更新映射

```http
PUT /api/v1/admin/payment/play-billing/config
Authorization: Bearer <admin jwt>
Content-Type: application/json

{
  "products": [
    {
      "product_id": "jisudeng.balance.50",
      "product_type": "inapp",
      "order_type": "balance",
      "amount": 50,
      "pay_amount": 7.99,
      "currency": "USD",
      "title": "50 credits",
      "description": "Top up 50 credits",
      "formatted_price": "$7.99",
      "enabled": true
    },
    {
      "product_id": "jisudeng.plan.pro.30d",
      "product_type": "inapp",
      "order_type": "subscription",
      "plan_id": 7,
      "amount": 19.99,
      "pay_amount": 19.99,
      "currency": "USD",
      "title": "Pro 30 days",
      "description": "Unlock Pro plan for 30 days",
      "formatted_price": "$19.99",
      "enabled": true
    }
  ]
}
```

字段规则：

- `product_id`：必须与 Play Console 完全一致；只能用小写字母、数字、下划线和点，且首字符必须是小写字母或数字。
- `product_type`：首发建议用 `inapp`。只有真正使用 Google 订阅商品时才用 `subs`。
- `order_type=balance`：必须设置 `amount > 0`，表示给用户充值的平台余额。
- `order_type=subscription`：必须设置有效 `plan_id`，表示购买后发放该平台套餐。
- `pay_amount`：用于订单记录的实付金额；不设置时使用 `amount`。
- `currency`：三位货币代码，Play 版首发建议 `USD`。
- `enabled=false`：后台保留映射，但不会下发给 APP，也不会允许验单发放。

## 服务器环境变量

生产必须配置：

```bash
PLAY_BILLING_PACKAGE_NAME=com.jisudeng.chat
PLAY_BILLING_SERVICE_ACCOUNT_FILE=/data/google-play/jisudeng-androidpublisher-service-account.json
```

兼容变量：

- `GOOGLE_PLAY_PACKAGE_NAME`
- `GOOGLE_PLAY_SERVICE_ACCOUNT_FILE`
- `PLAY_BILLING_SERVICE_ACCOUNT_JSON`
- `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON`

推荐只使用 `PLAY_BILLING_SERVICE_ACCOUNT_FILE`，文件放在持久 secret 目录，避免 JSON 被环境变量面板截断。

## APP 调用约定

1. 登录后调用：

   ```http
   GET /api/v1/payment/checkout-info
   ```

2. Play 版只展示 `play_billing_products` 中启用的商品。
3. 用户完成 Google Play Billing 购买后，APP 提交：

   ```http
   POST /api/v1/mobile/play-billing/purchases
   Authorization: Bearer <user jwt>
   X-Client-Request-Id: <uuid>
   Idempotency-Key: <uuid>
   Content-Type: application/json

   {
     "product_id": "jisudeng.balance.50",
     "product_type": "inapp",
     "purchase_token": "<google purchase token>",
     "package_name": "com.jisudeng.chat",
     "client_request_id": "<uuid>"
   }
   ```

4. 后端验单通过后创建 `payment_orders`，`payment_type/provider_key` 为 `google_play`，并复用现有支付履约链路给余额或套餐到账。

## Play Console 首发配置建议

1. 在 Google Play Console 创建一组 one-time products，例如：

   - `jisudeng.balance.50`
   - `jisudeng.balance.100`
   - `jisudeng.plan.pro.30d`

2. 每个 product ID 都同步写入后台映射。
3. 余额类商品设为消耗型，由 APP 按后端返回的 `consume=true` 调用 Billing Library consume。
4. 固定 30 天平台套餐可以首发用 one-time product 映射到 `order_type=subscription`；除非确实要 Google 管理自动续费，否则不要在首发误用 recurring subscription。
5. 创建 Google Cloud service account，授予 Play Console 对应应用的订单/订阅读取权限，并把 JSON 放到服务器 secret 文件路径。

## 验收清单

- 后台 `/admin/orders/play-billing` 可打开，且显示 `service_account_configured=true`。
- 后台 `GET /api/v1/admin/payment/play-billing/config` 返回同一组映射。
- 后台保存的 `product_id` 与 Play Console 完全一致。
- APP `checkout-info` 能看到 `play_billing_products`。
- Play 测试账号购买后，`/api/v1/mobile/play-billing/purchases` 返回 `accepted=true`、`verified=true`。
- 数据库出现 `payment_type='google_play'` 的订单，且余额或套餐到账。
- Play 全球首发版没有展示外部数字权益购买链接；国内版仍展示国内支付/兑换码入口。
