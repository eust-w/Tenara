# Roadmap

## P2(云端档接入点)

- **Kyverno admission 引擎**:MVP 明确采用 Kubernetes 内建 Pod Security Standard
  强制(`pod-security.kubernetes.io/enforce=restricted`,R3)。租户命名空间由
  app-controller 打标,渲染器一致性单测保证平台产物永不越界。Kyverno 在 P2 接入,
  用于跨命名空间策略与镜像签名校验联动等高级规则;**MVP 不安装 Kyverno**。
- **真实沙箱隔离**:isolation=isolated/dedicated 当前仅产生显式
  `IsolationUpgradeRequired` 事件(workload 行为等同 shared);真实隔离由
  P2(todo88)接管,本地不建 RuntimeClass、不假装启用 gVisor。

## P3

- 计费/账单、LLM 兜底分析、平台内自动修复环(MVP 只返回诊断信息给调用方 Agent,RB§28)。
