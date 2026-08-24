# 贡献指南

## 本地门禁（提交前必须全绿）

```text
make lint lint-go lint-ts lint-contract
make test test-mcp-conformance
node --check e2e/scenarios/*.mjs
```

CI 在 push/PR 上执行同一套门禁（含 golangci-lint 严格规则），任何一项不过即不可合入。

## 提交规范

- Conventional Commits（feat/fix/test/docs/chore/ci…），一个逻辑单元一个提交
- 行为变更必须携带同体测试（实现+测试=同一提交）
- 禁止跨主题混合提交

## 设计决策

影响架构、协议或运维语义的变更，先在 `dd/` 落一篇简短决策记录
（决策/依赖/验收/回退 四段式即可），再动代码。

## 基础设施相关改动

触及部署、调度、Provider 或任何需要真实集群才能验证的行为时，
请同步更新 [`docs/live-gates.md`](docs/live-gates.md) 的对应验收条目。

## 依赖

依赖更新由 Dependabot 自动发起并通过全量门禁后自动合入；
k8s.io 与 sigs.k8s.io 栈锁定补丁级更新，跨发布列车的升级作为独立工程规划。
