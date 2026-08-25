# Build 编排二期 — 工程蓝图

目标：部署时不再要求预置镜像。`POST /deployments` 不带 `image` 时，
平台物化 **Build CR**，集群内流水线产出 digest 并自动回填到 AppEnv
services，触发滚动更新。

## 阶段

### P2-A 源码接入（本地）

- host 上运行 `git daemon --base-path=<fixtures> --export-all --port=9418`
- kind 节点经 docker 网关 IP 访问 `<gateway>:9418/<fixture>.git`
- 脚本化：每夹具 `git init && commit && git clone --bare` 到导出目录

### P2-B 构建镜像目录

`scripts/build-images.sh` 产物 push 至 local registry :5000：

- `tenara/git-clone:pin`（alpine+git）
- `tenara/buildkit-node:pin`（buildkitd rootless + node20 工具链）
- `tenara/syft:pin`、`tenara/trivy:pin`、`tenara/cosign:pin`
  所有 Pod template 引用改为 digest 固定。

### P2-C digest 回填（集群内自治）

app-controller 监听 `Build{labels: tenara.io/app-id}` 达到 `PUSHED`：

1. 读 `status.digest`
2. 找同 label 的 AppEnv，patch `services[*].image = <digest>`
3. 既有 renderServices 循环自动滚动更新——**无需控制面参与**

### P2-D deploy API 分流

无 `image` 且桥接开启 → 校验 appspec 含 git 来源 → 物化 Build CR；
否则保持现直通语义。缺 git 来源返回 422 `GIT_SOURCE_REQUIRED`。

## 验收

`TENARA_E2E_REPO=git://<gateway>:9418/single-nextjs.git` 下 nl-regression
S1 全链：create→analyze→plan→deploy→Build→digest 回填→pod Running。

## 回退

带 `image` 的直通路径永久保留。
