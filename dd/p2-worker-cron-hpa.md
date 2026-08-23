# D2-P2-4: Cron/Worker 服务类型 + Auto Scaling
## 决策
AppSpec service.type 扩展 worker/cron(schedule 字段);worker=无 HTTPRoute 的长驻 Deployment;cron=CronJob;HPA(cpu 目标 70%)按 plan 配额上限。
## 依赖
AppSpec schema v2;HPA API 可用。
## 验收
cron 触发记录可见 + 负载扩容演练副本上升回落。
