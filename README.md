<div align="center">

<img src="/web/public/tooolx-icon.svg" width="120" alt="tooolx" />

# dash-new-api

**个人 AI 网关 + 小程序订阅后端** · [tooolx.com](https://tooolx.com)

基于 [QuantumNous/new-api](https://github.com/QuantumNous/new-api) 的定制分支

</div>

---

## 这是什么

自部署的 [new-api](https://github.com/QuantumNous/new-api) 定制版，收窄为个人自用形态，同时作为自有小程序的订阅计费后端：

- **AI 网关**：聚合上游模型渠道（OpenAI / Claude / Gemini / Azure 等 40+），对外提供统一的中转 API（`sk-` 令牌、额度、限流、用量看板）
- **订阅后端**：内置订阅计划系统，并新增**微信小程序虚拟支付（个人主体 · 道具直购）**通道，为自有应用（如记账小程序「钱喵」）提供 付费 → 订阅 → 权益 的完整链路；后续接入其他应用只需新建计划

## 与上游 new-api 的差异

| 方面 | 上游 | 本分支 |
| --- | --- | --- |
| 前端 | 完整的用户中心 + 管理后台 | 仅保留 root 登录与管理看板（渠道 / 令牌 / 订阅 / 用户 / 系统设置 / 日志） |
| 角色体系 | 访客 / 普通 / 管理员 / root | root + 单一普通用户角色（管理员角色已移除，管理面 root-only，服务端强制） |
| 对外集成 | — | 外部系统持 root PAT 调用管理 API：建用户、签发 `sk-` 令牌、下单、查询订阅权益 |
| 支付通道 | Stripe / Creem / Epay / Waffo 等 | 全部保留，新增微信小程序虚拟支付 `wechat_vpay` |

## 微信虚拟支付（wechat_vpay）

适用于**个人主体**小程序的道具直购（`short_series_goods`）。链路：

```
小程序后端(root PAT) ──下单──► new-api 建订单 + 返回签名 payData
小程序前端 ── wx.requestVirtualPayment(payData) ──► 微信
微信 ── 发货推送(XML) ──► new-api 核销订单、发放订阅
微信 ── 退款推送 ──► new-api 自动作废对应订阅、订单标记 refunded
```

- 签名算法（paySig / signature，HMAC-SHA256）按官方文档实现，并以官方测试向量锁定
- 发货以推送为主、`/xpay/query_order` 查单兜底（推送丢失时可远程补核销）
- 退款推送自动作废该订单创建的订阅（幂等），并记操作日志

**对外端点**（管理端点均为 root 鉴权，适配外部小程序后端）：

| 端点 | 说明 |
| --- | --- |
| `POST /api/user/` | 创建用户，响应返回 `data.id`（应用侧 openid ↔ 用户映射用） |
| `POST /api/user/token` | 生成 root PAT（个人访问令牌，需 2FA）——外部系统的管理凭证 |
| `POST /api/token/admin/` | 为指定 `user_id` 签发 `sk-` 中转令牌（响应打码） |
| `POST /api/token/admin/:id/key?user_id=` | 取完整 `sk-` key |
| `GET /api/subscription/admin/plans` | 订阅计划列表 |
| `POST /api/subscription/admin/users/:id/wechat-vpay/orders` | 创建虚拟支付订单，入参 `{plan_id, openid, session_key}`，返回 `pay_data`；`session_key` 仅用于签名，不落库不记日志 |
| `GET /api/subscription/admin/wechat-vpay/orders/:trade_no` | 订单状态轮询 |
| `POST /api/subscription/admin/wechat-vpay/orders/:trade_no/query` | 调微信查单兜底（已支付自动补核销 / 已退款自动对账） |
| `GET /api/subscription/admin/users/:id/subscriptions` | 用户订阅权益查询 |
| `GET+POST /api/subscription/wechat-vpay/notify` | 微信消息推送回调（公开，Token 验签） |

**配置**：

1. 系统设置 → 支付网关 → WeChat VPay：AppID / AppSecret / OfferID / 现网 AppKey / 推送 Token（密钥项留空 = 保持不变）
2. MP 后台【开发管理 → 消息推送】配置 URL `https://<域名>/api/subscription/wechat-vpay/notify` 与同一 Token，**明文模式**；道具在【虚拟支付 → 道具管理】创建
3. 订阅计划需 **CNY 计价、价格精确到分**，并填写微信道具 ID

## 部署

- **二进制部署**：GitHub Actions 提供 `deploy` 手动工作流（构建 + gzip + 上传）
- **环境变量**：`PORT`（默认 3000）、`SQL_DSN`（MySQL/PostgreSQL 连接串，缺省用 SQLite）、`SESSION_SECRET`（生产必设，否则重启后登录态失效）、`SQLITE_PATH`
- **数据库**：SQLite / MySQL ≥ 5.7.8 / PostgreSQL ≥ 9.6
- 首次启动访问 `/setup` 初始化 root 账号；前端仅 root 可登录

## 开发

```bash
# 后端（Go 1.25）
go build ./...
go test ./...

# 前端（web/，Bun）
bun install
bun run dev       # 开发
bun run build     # 生产构建
```

开发规范见 `AGENTS.md` 与 `web/AGENTS.md`（数据库三库兼容、JSON 封装、组件复用、i18n 等强制规则）。

## 上游项目与许可

<div align="left">

![new-api](/web/public/logo.png)

</div>

本项目基于 **[new-api](https://github.com/QuantumNous/new-api)**（[QuantumNous](https://github.com/QuantumNous)）二次开发，遵循并保留上游 AGPL-3.0 许可与全部版权、署名信息。上游文档：[docs.newapi.ai](https://docs.newapi.ai/)。
