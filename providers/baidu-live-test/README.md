# Baidu Live Contract Tests

真机合同测试套件。默认 **SKIP**(无凭据时不运行,Must NOT 默认 CI 运行 ✓)。

## 启用方式

设置以下环境变量后 `make test-baidu-live` 一键执行:

| 变量 | 说明 |
|------|------|
| `TENARA_BAIDU_ACCESS_KEY` | 百度云 AK |
| `TENARA_BAIDU_SECRET_KEY` | 百度云 SK |
| `TENARA_BAIDU_REGION` | 区域(如 bj) |

## Secret 注入规范

凭据仅经环境变量注入进程,**禁止写入代码/配置文件/git**。
