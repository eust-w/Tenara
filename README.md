# Tenara

Tenara 是一个「对 AI 编程助手说话就能上线应用」的多租户托管平台:用户对 Codex/Claude 说"帮我把这个项目上线",平台自动分析代码、构建镜像、创建带独立数据库、密钥、域名和资源配额的隔离运行环境。

- 需求唯一来源:`.omo/spec/requirements-baseline.md`(RB)
- 执行计划:`.omo/plans/tenara-agent-paas.md`
- 架构:控制面 Go + MCP 双形态 TypeScript + Analyzer + rootless BuildKit 流水线 + 沙箱化 K8s 运行时 + 数据面 Provider 抽象(local/baidu)+ Web 控制台 + 可观测栈

## 布局(RB§41)

| 目录             | 内容                                      |
| ---------------- | ----------------------------------------- |
| `api/`           | OpenAPI 契约                              |
| `mcp/`           | MCP server(远程 streamable HTTP + stdio)  |
| `sdk/ts`         | TS 共享平台客户端                         |
| `control-plane/` | Go 控制面服务                             |
| `controllers/`   | K8s controllers(build/app/database)       |
| `analyzer/`      | 仓库分析器(AppSpec 生成)                  |
| `builder/`       | rootless BuildKit 构建流水线              |
| `verifier/`      | 部署验证九步链                            |
| `providers/`     | 数据面 Provider 八接口 + local/baidu 实现 |
| `security/`      | PoC 脚本与策略                            |
| `web/`           | 用户台 / 管理台(Next.js)                  |
| `deploy/`        | compose / kind / helm 本地环境            |
| `e2e/`           | 端到端验收 harness                        |

## 快速开始

```sh
make lint   # golangci-lint(v2)+ eslint + prettier
make test   # go test -race -shuffle=on + workspace tests
```

本地数据面栈与 kind 集群见 `deploy/`(由计划 Wave 1 后续 todo 接线)。
