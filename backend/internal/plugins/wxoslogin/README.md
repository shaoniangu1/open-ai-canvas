# wxOS 登录插件

源码内置插件，ID 为 `wxos-login`。在影策管理后台「插件中心」配置和启停，默认关闭；需要随影策源码编译，不是可上传的插件压缩包。

## 配置

1. 先通过影策原有注册流程建立管理员。
2. 在 wxOS 后台「影策接入」填写影策站点根地址并生成接入密钥。
3. 将 wxOS 后台的身份来源编号和同一密钥填入影策插件配置，保存后启用插件。
4. 在 wxOS 打开「工作流创作」，自动进入影策 `/canvas`。

密钥只在后端使用并加密保存，读取配置只返回是否已配置。身份来源编号保存后不能修改；密钥留空保存表示保留现值。影策启停状态以现有 `PluginPlatformState` 为唯一来源。停用后拒绝新的 wxOS 登录，已存在的会话仍遵循影策原有会话规则。

## 业务与安全边界

- wxOS 签发 60 秒票据，以 HMAC-SHA256 校验，绑定本次客户端随机校验值；客户端通过 POST 兑换影策会话，不把票据放入 URL。
- 以身份来源编号和 wxOS 用户公开 ID 关联账号。不按邮箱、昵称合并；首次进入创建普通账号和零余额钱包。
- 单次票据复用 OAuth 状态表记录已使用 nonce；账号、身份、钱包和会话复用现有影策机制，无新增表。
- wxOS 内嵌会话与浏览器存储隔离，退出只撤销当前内嵌会话。网络失败时本地清除不代表远端撤销已确认。
- 影策独立注册登录继续可用。自动创建的 wxOS 关联账号没有自动复制密码；同一账号的独立登录凭据设置、已有账号绑定本轮未实现。
- 本轮不迁移余额、历史画布、模板或资产，不改生成和扣费流程。

## 源码边界

票据校验、配置与账号关联在本目录；前端配置组件在 `web/src/lib/plugins/builtin/wxos-login/`。

宿主保留：`app/wxos_auth.go` 组合、插件目录登记与启用检查、`handler/wxos_auth.go` HTTP 适配、`auth/external_login.go` 统一会话接口、`repository/wxos.go` nonce 持久化，以及对应 service 类型别名。后台 API 为 `GET/PATCH /api/admin/plugins/wxos-login/settings`，兑换为 `POST /api/auth/wxos/session`。

旧的 `auth/wxos.go`、管理后台独立配置组件和 `/admin/settings/wxos` 入口不再保留。后续合并上游重点检查这些宿主接口；插件主体独立维护。

## 验证

`go test ./internal/app ./internal/plugins/wxoslogin -run 'TestWxos|TestLoginCreates|TestRejectReplay|TestDisabledPlugin' -count=1` 覆盖插件默认关闭、配置后启用、管理员权限、停用后拒绝登录、账号隔离、钱包创建、单次票据、过期/签名/校验值检查和会话撤销隔离。真实两端联调尚未验收。
