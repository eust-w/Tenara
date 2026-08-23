# D2-P2-3: Preview Deployments + GitHub Webhook
## 决策
PR 打开→自动建 preview-pr-{n} 环境→评论回填 URL→关闭/merge 自动软删(复用 T54 流水线);webhook 签名验证+重放防护。
## 依赖
GitHub App 注册;webhook secret;T54 软删流水线。
## 验收
fixture PR 生命周期全自动化断言(URL 可达→清理后 ns 消失);open/close 两例。
