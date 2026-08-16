# 迁移与回滚：Grafana 资源限额

## 迁移

无数据迁移；`kubectl apply -k config/observability` 触发 Deployment 滚动更新即可。

## 回滚

将 resources 恢复为原值（requests 50m/128Mi，limits 500m/384Mi）并重新 apply；
无需清数据，Grafana PVC 保持不变。
