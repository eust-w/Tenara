# Observability local stack(RB§31 §0.1)

精简子集:Prometheus(+Grafana)与 Loki(+Promtail,携带 `tenara.io/app-id` /
`tenara.io/env` 标签)。**全部 ClusterIP,Must NOT 暴露公网端口**——仅经
`kubectl port-forward` 使用。

```sh
make observability-up
```

## 探针(验收三例)

```sh
kubectl -n observability port-forward svc/observability-kube-prom-prometheus 9090:9090 &
curl -sf localhost:9090/-/healthy && echo PROM_OK

kubectl -n observability port-forward svc/observability-loki 3100:3100 &
curl -sf 'localhost:3100/loki/api/v1/labels' | grep -q app_id && echo LOKI_LABELS_OK

kubectl -n observability port-forward svc/observability-grafana 3000:80 &
curl -sf localhost:3000/api/health | grep -q ok && echo GRAFANA_OK
```
