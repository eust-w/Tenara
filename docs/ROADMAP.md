# Roadmap

状态基准：执行计划 tenara-agent-paas 已完成（98 个实现行全部交付，
F1/F2/F4 验收回执 APPROVE）。当前能力总览见 [README](../README.md)，
需真实基础设施的验收路径见 [live-gates](live-gates.md)。

## 待执行（近期）

- **control-plane → 集群桥接器**（头号工程项）：部署 API 落库后尚无组件将
  AppEnv/Build 物化到集群。方案：dynamic client 直 apply（本地 KUBECONFIG /
  生产 SA），plan 输出映射 CRD；配套集成测试走通 nl-regression S1 全链收敛。
- **F3 真实手工 QA 收尾**：本地栈已完成认证链、五场景回归、双控制器活体等
  （见 live-gates 已知缺口节）；剩余项依赖桥接器与云凭据。

## 候选（中期）

- **Kyverno admission**：镜像签名校验联动与跨命名空间高级策略；
  当前基线由内建 Pod Security Standard `restricted` 模式覆盖。
- **Auto-Repair 接入 LLM 兜底**：`repair.PatchSource` 的 Codex/Claude 真实现，
  打通「诊断包→补丁生成→重建→复验」闭环的最后一环。
- **支付通道产品化**：billing Provider 抽象之上的 stripe 类/国内通道真实回调接线。

## 已交付里程碑（摘要）

- 多租户控制面 · rootless 构建供应链（Syft SBOM / Trivy 门禁 / Cosign 签名）
- MCP 双形态网关（streamable HTTP + stdio）· 用户/管理双控制台
- isolated（gVisor/Kata）与 dedicated（独占节点池）两档真实隔离
- Cell 多集群注册路由与故障半径隔离 · Preview Deployments 自动化
- 计费产品化 · RBAC 七角色矩阵 · 三次硬闸 Auto-Repair 循环
- 四云 runtime 适配器（baidu/aliyun/tencent/selfhosted）与数据面 Provider 舰队
- k8s 发布列车升级至 0.36 / controller-runtime 0.24（事件 API 与 RequeueAfter 迁移）
