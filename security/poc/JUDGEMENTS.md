# RB§50 六项 PoC 判定清单

| #    | PoC               | 判定标准                         | 通过条件                        | 不通过回退                        |
| ---- | ----------------- | -------------------------------- | ------------------------------- | --------------------------------- |
| P0-1 | netpol-isolation  | 跨 ns deny 实测                  | conn timeout/refused            | → CNI 决策点(Calico↔Cilium)       |
| P0-2 | runtimeclass      | gVisor workload 运行无逃逸       | RuntimeClass 存在且 pod Running | → isolated 档降级路径(P2 todo88)  |
| P0-3 | rootless-buildkit | build→push→digest exit0          | builder pod 完成                | → 排查 sandbox flags/镜像版本     |
| P0-4 | docdb-user        | per-app user 创建+越权拒绝       | mongosh 连通自库/他库 denied    | → 检查 DocDB IAM 映射             |
| P0-5 | wildcard-routing  | ≥50 子域建删+TLS 有效            | curl 200 ×N                     | → 检查 DNS zone 委派/BLB listener |
| P0-6 | ccr-pipeline      | build→push→scan→digest→pull 全链 | digest 匹配+pull 成功           | → 检查 CCR 凭据/网络策略          |
