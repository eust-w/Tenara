# MCP 接入指南(D4)

Tenara 提供 MCP 双形态接入:远程 streamable HTTP 与本地 stdio 包装
(`npx tenara-mcp`)。两者暴露完全相同的工具集(见 `e2e/mcp` 一致性测试)。

## 远程接入(streamable HTTP)

服务地址:`https://<your-tenant>.tenara.example/mcp`(Bearer 认证)。

### Codex(`~/.codex/config.toml`)

```toml
[mcp_servers.tenara]
type = "http"                       # §53.1 gotcha: 必须显式 type=http
url = "https://<tenant>.tenara.example/mcp"
bearer_token_env_var = "TENARA_API_TOKEN"
```

### Claude Code(`.mcp.json`)

```json
{
  "mcpServers": {
    "tenara": {
      "type": "http",
      "url": "https://<tenant>.tenara.example/mcp",
      "headers": { "Authorization": "Bearer <YOUR_TENARA_TOKEN>" }
    }
  }
}
```

> §53.1 gotcha:HTTP 条目必须写 `"type": "http"`;缺省会按 stdio 解释导致
> 连接失败。Token 一律走环境变量或占位符,禁止把真实令牌提交进仓库。

## 本地接入(stdio)

```sh
export TENARA_API_URL="https://api.<tenant>.tenara.example"
export TENARA_API_TOKEN="<YOUR_TENARA_TOKEN>"
npx tenara-mcp
```

stdio 形态与远程共享同一工具注册表;`initialize` → `tools/list` 的返回集合
与远程完全一致(由 `e2e/mcp` parity 测试保证)。

## 工具集速览

| 分组   | 工具                                                                                                        |
| ------ | ----------------------------------------------------------------------------------------------------------- |
| app.*  | create / analyze / plan / deploy / status / logs / restart / rollback / delete / set_env                    |
| data   | database.create · domain.bind                                                                               |
| admin* | user.list · user.suspend · app.list · app.stop · quota.set · cluster.health · security.events(需平台管理员) |
