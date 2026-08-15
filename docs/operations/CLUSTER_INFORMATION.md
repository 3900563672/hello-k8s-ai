# 集群信息

本文件区分“用户提供的运行快照”“本仓库部署约束”和“本轮实际执行结果”。三者不可混写。

## 1. 用户提供的目标集群快照

采集日期：2026-08-13；输出由用户提供，本交付环境未重新执行。

| 项目 | 结果 |
| --- | --- |
| 当前 Context | `docker-desktop` |
| API Server | `https://127.0.0.1:56477` |
| Kubernetes | Server v1.36.1；Client v1.36.2 |
| Node | 1 个 control-plane + 9 个 worker，全部 Ready |
| Runtime | containerd 2.3.1 |
| Namespace | `hello-k8s-ai-system` 已存在 |
| StorageClass | `standard` 为默认；另有 `hostpath` |
| PV/PVC | 快照时不存在 |
| Docker Desktop | 4.86.0；Engine 29.7.2 |

Node 名称：

```text
desktop-control-plane
desktop-worker
desktop-worker2
desktop-worker3
desktop-worker4
desktop-worker5
desktop-worker6
desktop-worker7
desktop-worker8
desktop-worker9
```

## 2. 已确认的旧部署故障

快照时只有旧 Controller Deployment：

| 对象 | 状态 | 原因 |
| --- | --- | --- |
| `hello-k8s-ai-controller-manager` | 0/1 | `ImagePullBackOff` |
| 镜像 | `controller:latest` | Node 尝试拉取 `docker.io/library/controller:latest`，仓库不存在/无权限 |

这不是网络重试能解决的问题：清单引用了没有构建、没有导入且不应从 Docker Hub 拉取的占位镜像。

当前部署在 apply 前构建 `hello-k8s-ai-controller:dev`，并连同 Simulator、Backend、Frontend 与第三方运行镜像导入全部 Node 的 containerd。

## 3. CRD 与 CR 快照

用户当时的集群快照中存在 10 个旧版项目 CRD，尚无当前 CR 实例。本次源码已增加第 11 个 `simulationclocks.platform.study.com`；覆盖源码不代表目标集群已升级，仍需执行部署。另有历史 CRD：

```text
orchestratorconfigs.platform.study.com
```

部署脚本只警告，不自动删除该旧 CRD。原因是无法从 CRD 名称证明其中没有需要保留的数据；这仍需人工确认后单独处理。

## 4. 旁边的 Kind 集群

Docker 中另有独立 Kind 集群：

| 项目 | 值 |
| --- | --- |
| 名称 | `minikserve-demo` |
| API Server | `https://127.0.0.1:46449` |
| Node | `minikserve-demo-control-plane` |
| Kubernetes | v1.34.3 |

它与当前 Context 不是同一个集群。日常部署不得调用 `kind load --name minikserve-demo`，也不得删除或重置它。

## 5. 仓库部署约束

| 项目 | 声明值 |
| --- | --- |
| Context | `docker-desktop` |
| Namespace | `hello-k8s-ai-system` |
| StorageClass | `standard` |
| 镜像交付 | Docker build/save + 每 Node `ctr -n k8s.io images import` |
| 动态 WorkerNode | 从非 control-plane Kubernetes Node 名生成 |
| 数据库 | PostgreSQL 17，10Gi PVC，随机 Secret |
| 停止语义 | 工作负载缩到 0，保留集群/CRD/CR/Secret/PVC |

## 6. 部署后应出现的静态工作负载

| Kind | Name | 副本 |
| --- | --- | ---: |
| Deployment | `hello-k8s-ai-controller-manager` | 1 |
| Deployment | `hello-k8s-ai-otel-collector` | 1 |
| Deployment | `hello-k8s-ai-jaeger` | 1 |
| Deployment | `hello-k8s-ai-prometheus` | 1 |
| Deployment | `hello-k8s-ai-grafana` | 1 |
| StatefulSet | `hello-k8s-ai-dashboard-postgresql` | 1 |
| Deployment | `hello-k8s-ai-dashboard-backend` | 1 |
| Deployment | `hello-k8s-ai-dashboard-frontend` | 1 |

Controller 还会按 SimulatorInstance 动态创建 `simulator-<instance>` Deployment。

## 7. 采集最新运行快照

```bash
make cluster-status
```

或执行：

```bash
kubectl --context docker-desktop get nodes -o wide
kubectl --context docker-desktop -n hello-k8s-ai-system \
  get deployment,statefulset,pod,service,pvc,lease -o wide
kubectl --context docker-desktop get crd | grep platform.study.com
kubectl --context docker-desktop get \
  models,workernodes,tenants,tenantmodelpolicies,tenantnodepolicies,modelnodepolicies,\
simulatorinstances,tenantperformances,tenantruntimes,orchestrators
```

## 8. 当前结论

目标集群信息已经足够明确，部署方案无需再创建集群或索取拓扑信息。真正的剩余证据是用户机器执行 `bash setup.sh` 后产生的自动验收结果；在此之前只能确认旧镜像问题与部署前置条件，不能声称新工作负载已经 Ready。
