# D2-P2-5: 平台内 Auto Repair 循环(RB§28 完整版)

## 决策

verify 失败→诊断包(T56)→LLM(Codex/Claude API 抽象)生成 patch→新 revision build/deploy→verify;max_auto_repair_attempts=3 硬闸+审计每次尝试。

## 依赖

诊断包(T56);LLM API 抽象;审计完备性。

## 验收

DATABASE_URL 缺失注入案例 ≤3 次内修复至 RUNNING;第 4 次禁止触发。
