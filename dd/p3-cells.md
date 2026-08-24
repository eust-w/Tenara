# P3-3: Dedicated Node 档 + Cell 架构多集群(RB§39 §38)

## 决策

isolated/dedicated 档真实调度(独占节点池 label+taint);cell 注册表(control plane 管 N 个 cluster 的 provider endpoint)+按 org 路由 cell。

## 依赖

多集群网络;cell 注册表 API;故障演练基础设施。

## 验收

kill cell B 内 mongo→仅 cell B 应用 DEGRADED(故障隔离证明)。
