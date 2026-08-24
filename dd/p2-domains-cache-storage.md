# D2-P2-2: 自定义域名自动签发 + Redis/BOS 产品面

## 决策

cert-manager(ACME)接入 domain-controller verified 后自动证书;Redis/BOS 从 provider 能力升为 MCP database.create type 扩展。

## 依赖

cert-manager 安装;domain-controller verified 流程;BOS/Redis 凭据。

## 验收

自定义域 TLS 绿 + redis 绑定读写成功 + bos 上传下载成功;三 e2e 场景绿。
