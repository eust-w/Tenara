# Tenara

[![ci](https://github.com/eust-w/Tenara/actions/workflows/ci.yml/badge.svg)](https://github.com/eust-w/Tenara/actions/workflows/ci.yml)

Tenara 是一个「对 AI 编程助手说话就能上线应用」的多租户托管平台：用户对 Codex/Claude 说"帮我把这个项目上线"，平台自动分析代码、构建镜像、创建带独立数据库、密钥、域名和资源配额的隔离运行环境。

## 架构总览

```text
Agent (Codex / Claude)
   │  MCP · streamable HTTP ─┐        ┌─ stdio wrapper
   ▼                         ▼        ▼
控制面 REST API (Go chi) ──── MCP 网关（十二应用工具 + 管理工具）
   │ analyze → plan → approve
   ▼
Analyzer (框架识别/AppSpec)     Build 面板：git clone → rootless BuildKit
                                → Syft SBOM → Trivy 门禁 → Cosign 签名
   │
   ▼
AppEnv 控制器：命名空间工厂 · RB§15 六项加固渲染 · NetworkPolicy 链
   │            isolated=gVisor/Kata · dedicated=独占节点池
   ▼
数据面 Provider 抽象：mongo / redis / minio / secret
   │                  local ─┬─ baidu-cce/ccr/docdb/bos/blb-dns
   │                        ├─ aliyun-ack
   │                        ├─ tencent-tke
   │                        └─ selfhosted（BYO 采纳式）
   ▼
Envoy Gateway → 租户应用 · Preview(pr-N) · 自动 TLS(cert-manager)
```

多集群按 Cell 注册路由（`TargetForOrg` 显式指派 > 默认 cell），故障半径以 cell 为界。

## 目录布局

| 目录             | 内容                                                          |
| ---------------- | ------------------------------------------------------------- |
| `api/`           | OpenAPI 契约                                                  |
| `mcp/`           | MCP server（远程 streamable HTTP + stdio）                    |
| `sdk/ts`         | TS 共享平台客户端                                             |
| `control-plane/` | Go 控制面服务                                                 |
| `controllers/`   | K8s controllers（build / appenv / database-binding）          |
| `analyzer/`      | 仓库分析器（AppSpec 生成）                                    |
| `builder/`       | rootless BuildKit 构建流水线（规格文档占位）                  |
| `verifier/`      | 部署验证九步链                                                |
| `providers/`     | 数据面 Provider 抽象与四云适配器（mock 合同套件）             |
| `web/`           | 用户/管理双控制台（Next.js）                                  |
| `e2e/`           | 场景回归（success-path / negative / nl-regression）+ 安全断言 |
| `security/poc/`  | 六件 PoC 脚本与 JUDGEMENTS 判据                               |
| `deploy/`        | compose / kind / helm / observability / certmanager           |
| `dd/`            | P2–P4 设计决策记录                                            |
| `docs/`          | 路线图、MCP onboarding、live-gates 手册                       |

需求基线与执行计划为会话工件（不入库）。

## 快速开始（本地）

```bash
# 前置：docker、kind、go ≥1.25、node ≥24 + pnpm@9
make tools                                   # gofumpt/golangci-lint 等
docker compose -f deploy/docker-compose.yml up -d   # pg/mongo/redis/minio/registry/mailhog
make cluster-up                              # kind 三池 + Calico（deploy/kind）
make build-images helm-install migrate-up
ADMIN_EMAIL=ops@example.test make run-cp     # 控制面 :8080
pnpm install && pnpm --filter user-console dev
```

## MCP 接入

远程形态由 MCP 网关暴露 streamable HTTP；本地 CLI 形态走 stdio wrapper。
工具清单、鉴权与 onboarding 步骤见 [`docs/mcp-onboarding.md`](docs/mcp-onboarding.md)。

## 测试与质量门禁

```text
make lint lint-go lint-ts lint-contract   # 静态三族
make test test-mcp-conformance            # Go 全量 + MCP 协议一致性
make e2e-smoke e2e-success-path e2e-negative e2e-nl-regression
make verify-security-runtime verify-security-data
make test-baidu-live poc-all              # 云凭据门控（无凭据自动 SKIP）
```

CI（GitHub Actions）在 push/PR 上跑 gofmt/vet/test 六模块矩阵与 node 门禁。

## Live gates（需真实基础设施）

本地交付以 mock 合同+env 门控为准；需要在线栈的验收项统一收录于
[`docs/live-gates.md`](docs/live-gates.md)——含 RB§51 剧本、六项 PoC 云模式、
gVisor 实测、跨 cell 故障演练与支付回调端到端。

## 设计决策记录（dd/）

P2–P4 里程碑的实现前置决策全部落档：沙箱接入、域名/缓存/存储产品面、
Preview 部署、Cron/Worker+HPA、Auto-Repair 循环、计费产品化、RBAC 七角色
矩阵、Cell 多集群。逐篇见 [`dd/`](dd/)。
