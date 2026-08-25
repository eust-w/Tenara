# Live Gates —— 需要真实基础设施的验收清单

本地交付策略：一切可用 mock/env 门控验证的均已落地并全绿（见 CI 与各包测试）；
以下条目必须在实际栈上执行后签署。每项通过打 `[x]` 并附命令输出摘要；
失败项记入「偏差」并回实现侧修复重跑。

## 0. 环境拉起

1. `make tools && docker compose -f deploy/docker-compose.yml up -d`
2. `make cluster-up`（kind 三池 + Calico）+ Envoy Gateway 安装
3. `make build-images helm-install migrate-up`
4. `ADMIN_EMAIL=<ops> make run-cp` 后注册首个用户并 bootstrap platform_admin

## 1. RB§51 剧本与门禁（Success criteria #2/#3）

- [x] `make test`（Go 全量，43 包绿）
- [ ] `make e2e-smoke && make verify-security-runtime && make verify-security-data`（需先跑通部署主链产出 app/ns）
- [ ] `time make e2e-success-path`：exit 0 且总时长 <600s；用户路径零 kubectl
- [x] `make e2e-negative`：7 场景中 5 过；rollback/diagnostics 为已知桩缺口
- [x] `make e2e-nl-regression`：RB§47 五场景 **5/5 全过**（2026-08-24 实测）
- [ ] `make e2e-nl-regression`（RB§47 五场景自然语言回归）

## 2. 云凭据门（Success criteria #5）

- [ ] 注入 `BAIDU_*` 后 `make test-baidu-live`：baidu-live-test 由 SKIP 转 RUN 全绿
- [ ] `make poc-all --cloud`：netpol / runtimeclass / rootless-buildkit / docdb-user / wildcard-routing / ccr-pipeline 六项 PASS

## 3. P2–P4 专项 live 面

- [ ] **沙箱**：`security/poc/p0-2-runtimeclass.sh cloud`——runsc pod Ready 且 `/proc` 只读冒烟
- [ ] **自定义域 TLS**：cert-manager 安装 → TXT verify → Certificate Ready → HTTPS 绿
- [ ] **Cell 故障隔离**：kill cell-B 内 mongo → 仅 cell-B 应用 DEGRADED
- [ ] **多云连通**：ACK / TKE / Self-hosted 三适配器 Healthz 冒烟（凭据注入）
- [ ] **支付回调**：tier 升降级端到端即时生效（stripe 类或国内通道其一）

## 4. 签署

全项通过即视为技术方案成立（W15+ 解锁）；任一长期阻塞在此登记原因与复核日期。

## 已知缺口（F3 部分执行记录，2026-08-24）

本地栈（kind tenara + compose 六服务 + 控制面 :28080）已实测：

- ✅ 认证链（注册→邮箱验证→登录 JWT）· 配额层级感知（pro 生效）
- ✅ 分析→计划→部署提交 · 数据库绑定（含 mongo 别名）· 域名添加
- ✅ 双控制器活体（Build 冒烟状态机首跳 CREATED）
- ⬜ control-plane → 集群桥接器：部署 API 落库后无组件将 AppEnv/Build
  物化到集群（下一步核心工程项）
- ⬜ rollback / diagnostics / logs 三端点为契约桩（501），实现待排期
- ⬜ 云凭据门：test-baidu-live、poc-all --cloud、ACK/TKE/Self-hosted 冒烟、
  cert-manager ACME、支付回调端到端
