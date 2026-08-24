# 安全策略

Tenara 是多租户应用托管平台——租户隔离（namespace/NetworkPolicy/沙箱）、
供应链完整性（SBOM/扫描/签名）与凭据安全是产品的核心承诺。我们高度重视安全报告。

## 报告漏洞

请使用 GitHub 的**私密漏洞报告**入口（仓库页 Security → Report a vulnerability），
不要以公开 issue 形式披露。

- 48 小时内确认收到
- 7 天内给出严重性评估与修复计划
- 修复发布后在 Security Advisories 中致谢（除非你要求匿名）

## 范围

- control-plane REST API（鉴权/RBAC/幂等/审计）
- MCP 双形态网关（HTTP 会话与 stdio）
- 构建流水线（rootless BuildKit/SBOM/Trivy/Cosign 链）
- 多租户运行时隔离（PSS/NetworkPolicy/RuntimeClass/节点池）
- 数据面 Provider 凭据链（KMS 密封/最小权限身份）
- Web 控制台（用户与管理端）

## 自验基线

平台侧持续验证的安全断言见：

- `e2e/security/verify-runtime.sh` 与 `verify-data.sh`（边界与供应链断言）
- `security/poc/JUDGEMENTS.md`（六件 PoC 判据）
- [`docs/live-gates.md`](docs/live-gates.md)（需真实基础设施的验收清单）

## 支持版本

仅为主分支最新代码与最新 release tag 提供安全修复。
