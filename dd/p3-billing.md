# P3-1: Billing + Usage Metering 产品化(RB§29 §46 R10)

## 决策

organizations.tier 启用(free/pro);plan 定义+超额策略(限流不删数据);账单周期任务从 usage_records(T71)聚合;stripe 类支付抽象 Provider 化(国内通道留接口)。

## 依赖

usage_records 表(T71);支付通道抽象。

## 验收

tier 切换→quota 中间件立即读新值(mock 支付回调);升降级两例绿。
