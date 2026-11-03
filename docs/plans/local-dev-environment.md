# Windows 本地开发与 wxOS 接入验收

## 目录和分支

本项目在原目录使用 `codex/wxos-integration`，`main` 保留原有提交。wxOS 在自己的原目录使用 `codex/yingce-integration`。本机继续维护这两个项目的原目录，不另建开发工作区。

本机运行时、启动配置、凭证、数据库、桌面资料和日志都留在 Git 忽略目录；本文只记录用途和相对路径。

## 开发工具

本机工具位于 `.local/tools/`：Go 1.25.14、Bun 1.3.9、mingw-w64 GCC 16.2.0。Node 24.16.0 由本机启动入口指定。升级时以 `backend/go.mod`、`web/package.json` 和 Agent runtime 的实际依赖要求为准。

SQLite 驱动使用 cgo，源码编译需要 `CGO_ENABLED=1` 和可用的 C 编译器。缓存放在 `.local/cache/`；不要提交运行时或用其他包管理器覆盖 Bun 管理的前端依赖。

## 当前验收入口

只启动影策时，在本项目根目录执行：

```powershell
& .\.local\start-acceptance.ps1
```

同时启动 wxOS 和影策时，在 wxOS 项目根目录执行：

```powershell
& .\artifacts\yingce-acceptance\start.ps1
```

上述脚本是本机配置，不随仓库分发。换机器需要按本机运行时位置配置，并参考 [本地开发说明](../content/docs/backend/local-development.mdx) 启动源码环境。

| 项目 | 当前配置 |
| --- | --- |
| 影策页面 | `http://127.0.0.1:3110/` |
| 影策后端 | `http://127.0.0.1:8080` |
| wxOS 管理端 | `http://127.0.0.1:5174/` |
| wxOS API | `http://127.0.0.1:3001` |
| 影策验收数据 | `.local/project-workbench-debug/wxos-integration/` |
| 影策验收后端程序 | `.local/cache/wxos-integration/yingce-backend.exe` |
| 影策验收日志及进程记录 | `.local/logs/wxos-integration/` |
| wxOS 验收账号文件 | wxOS 项目的 `artifacts/yingce-acceptance/accounts.md` |

影策验收数据与 `.local/project-workbench-debug/` 下原有数据库分开，保留原有数据。不要把两者互相覆盖，迁移时同时保留 SQLite 的配套文件和 `.settings-key`。

当前影策入口复用已记录且归属正确的进程，端口被其他环境占用时拒绝启动。wxOS 统一入口调用自己的 `start-dev.cmd` 管理服务复用；同项目保持一套运行环境。

影策验收后端使用已经构建的程序。修改 Go 源码后需要重新构建并更新该程序，重启对应后端后再验收；前端由 Vite 提供热更新。

旧的 `scripts/start-dev.ps1`、`scripts/start-host.ps1` 和 `docker-compose.dev.override.yml` 已被本机验收入口替代并删除。仓库原有 `docker-compose.dev.yml` 继续保留，使用 Docker 开发时按官方说明单独配置，不与当前验收环境同时启动。

## Windows 验证注意事项

- 路径比较需注意 Windows 的反斜杠；出现测试失败时核对实际输入，不直接认定为业务缺陷。
- CRLF 行尾可能影响文本断言和 shell 脚本执行；核对 Git 行尾设置及测试要求。
- 依赖 POSIX shell、PostgreSQL、Redis 或双架构 Docker 构建的检查，需要相应环境。跳过的用例列为未验证。
- 进程测试出现 `WaitDelay` 或管道未关闭时，检查 Node 子进程是否退出，不把断言通过等同于测试进程成功结束。

推送前按根目录 `AGENTS.md` 执行验证，并明确基线：

```powershell
bun scripts/verify-before-push.mjs --base origin/main
```

只有确认 `origin` 指向自有 fork 且对应引用已更新后才使用该基线。共享历史按根规范使用 merge，不机械执行验证脚本中的 rebase 提示。

本机已完成数据库完整性、管理员登录、桌面签名登录、会话交换、身份稳定性、票据重放拒绝和退出隔离检查。完整界面、真实模型生成及生产行为仍需分别验收；页面可打开不等于这些业务均已通过。
