## 关联 Issue

Fixes #

## 改动摘要

<!-- 用几句话说明改了什么、为什么这样改，不要复述代码。 -->

## 验证

<!-- 只列出实际执行的命令与结果，未执行的不要写“已通过”。 -->

- [ ] `make fmt && make vet && make test && make lint`
- [ ] Dashboard Backend：`gofmt -w . && go vet ./... && go test ./...`
- [ ] Frontend：`cd dashboard/frontend/my-app && npm run check`
- [ ] 清单渲染：`kubectl kustomize config/dev`、`config/demo`、`dashboard/deploy`

## 未验证范围

<!-- 缺少环境或集群时如实说明，例如：本机无 Docker / Kind，E2E 未执行。 -->

## 部署影响

<!-- 是否需要重新部署、变更 CRD / RBAC / 数据库，或影响既有数据。 -->