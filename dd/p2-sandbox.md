# D2-P2-1: gVisor/Kata 沙箱接入(isolated 档)
## 决策
RuntimeClass 渲染器按 isolation=isolated 注入 runsc/kata;shared 档不受影响。
## 依赖
P0-2 PASS 为前置门;@baidu-live 凭据。
## 验收
cloud 模式脚本 runsc pod Ready + success-path 变体绿;逃逸面测试(gVisor 下 proc/sys 只读冒烟)。
## 回退
isolated 档降级路径 → shared + 额外 netpol 隔离。
