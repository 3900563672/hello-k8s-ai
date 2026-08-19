# 默认 Controller 镜像名。保留 IMG 变量兼容已有 Kubebuilder/CI 命令。
IMG ?= hello-k8s-ai-controller:dev
# 用于生成代码头部的年份，一般不用改
YEAR ?= $(shell date +%Y)

# 只有开发类 target 才需要 Go；完整栈部署只依赖 Docker 和 kubectl。
ifneq (,$(shell command -v go 2>/dev/null))
  ifeq (,$(shell go env GOBIN))
    GOBIN=$(shell go env GOPATH)/bin
  else
    GOBIN=$(shell go env GOBIN)
  endif
else
  GOBIN=$(CURDIR)/bin
endif

# 用什么构建镜像，默认 docker，也能换成 podman
CONTAINER_TOOL ?= docker
# CI 可注入 BuildKit 缓存参数（如 --cache-from=type=gha --cache-to=type=gha,mode=max）。
DOCKER_BUILD_CACHE ?=
# 注入缓存参数时走 docker buildx（需要 --load 把镜像放进本地 Docker 供后续校验）；
# 本地默认空，保持 docker build 行为不变。
ifeq ($(strip $(DOCKER_BUILD_CACHE)),)
BUILD_CMD := $(CONTAINER_TOOL) build
else
BUILD_CMD := $(CONTAINER_TOOL) buildx build --load
endif

# 本地完整栈默认使用 Kind 开发集群（kind-hello-k8s-ai-dev）；兼容 docker-desktop 旧 Context。
KUBE_CONTEXT ?= kind-hello-k8s-ai-dev
NAMESPACE ?= hello-k8s-ai-system
# MANAGER_IMG 默认跟随 IMG，保证 `make docker-build IMG=...` 等旧用法仍然有效。
MANAGER_IMG ?= $(IMG)
SIMULATOR_IMG ?= hello-k8s-ai-simulator:dev
BACKEND_IMG ?= hello-k8s-ai-dashboard-backend:dev
FRONTEND_IMG ?= hello-k8s-ai-dashboard-frontend:dev
ROOT_DOCKERFILE ?= Dockerfile
# 演示 Model.spec.absoluteScore 的初始能力基准分。
DEMO_MODEL_ABSOLUTE_SCORE ?= 100

# bash 模式，出错就停
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ 通用

.PHONY: help
help: ## 打印帮助
	@awk 'BEGIN {FS = ":.*##"; printf "\n用法：\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ 开发

.PHONY: manifests
manifests: controller-gen ## 生成 CRD、RBAC、webhook 配置
	"$(CONTROLLER_GEN)" rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## 生成 DeepCopy 等代码
	"$(CONTROLLER_GEN)" object:headerFile="hack/boilerplate.go.txt",year=$(YEAR) paths="./..."

.PHONY: fmt
fmt: ## go fmt
	go fmt ./...

.PHONY: vet
vet: ## go vet
	go vet ./...

.PHONY: test
test: vet ## 跑单元测试（不会重新生成或改写 CRD/API）
	go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

.PHONY: fmt-check
fmt-check: ## 检查全部 Go 源码格式，不改写文件
	@files="$$(find api cmd internal simulator test dashboard/backend -type f -name '*.go' -print0 | xargs -0 gofmt -l)"; \
	if [ -n "$$files" ]; then \
		echo "以下 Go 文件需要执行 gofmt："; \
		echo "$$files"; \
		exit 1; \
	fi

.PHONY: test-backend
test-backend: ## 检查 Dashboard Backend
	cd dashboard/backend && go vet ./... && go test ./... -count=1

.PHONY: test-e2e-compile
test-e2e-compile: ## 只编译 E2E 测试，不创建集群
	go test -tags=e2e ./test/e2e -run '^$$' -count=1

.PHONY: test-frontend
test-frontend: ## 检查 Frontend lint、类型、构建和状态验证
	cd dashboard/frontend/my-app && npm ci && npm run check

.PHONY: verify-deploy
verify-deploy: kustomize ## 检查脚本语法并渲染全部部署清单
	bash -n setup.sh hack/local-cluster.sh hack/cleanup-obsolete.sh
	"$(KUSTOMIZE)" build config/dev >/dev/null
	"$(KUSTOMIZE)" build config/demo >/dev/null
	"$(KUSTOMIZE)" build dashboard/deploy >/dev/null

.PHONY: lint-sh
lint-sh: ## shellcheck 全部 *.sh（防"漏定义变量/sleep/引号"类低级错上线跑）
	@command -v shellcheck >/dev/null 2>&1 || { echo "缺少 shellcheck：请在 WSL 执行 apt-get install -y shellcheck" >&2; exit 1; }; \
	set -e; for f in $$(find . -path ./node_modules -prune -o -path ./.git -prune -o -path ./.runtime -prune -o -name '*.sh' -print); do shellcheck "$$f" || exit 1; done; echo "shellcheck OK"

.PHONY: lint-md
lint-md: ## markdownlint 检查 Agent 文档层（docs/agents、journal、lessons、remote-ai、change-history、README）
	@command -v markdownlint-cli2 >/dev/null 2>&1 || { echo "缺少 markdownlint-cli2：请在 WSL 执行 npm install -g markdownlint-cli2" >&2; exit 1; }; \
	markdownlint-cli2 && echo "markdownlint OK"

.PHONY: lint-ps1
lint-ps1: ## PSScriptAnalyzer 检查仓库 .ps1（非 Windows 或仓库无 .ps1 时跳过）
	bash hack/lint-ps1.sh

.PHONY: selfcheck
selfcheck: lint-sh lint-md ## 工具链自检：shell/markdown 静态检查 + Node 语法 + 清单渲染（防"漏定义函数上线跑"）
	@echo "== Node 脚本语法 =="; \
	set -e; for f in $$(find hack -name '*.mjs'); do node --check "$$f" || exit 1; done; echo OK
	@echo "== Node 单测 =="; \
	set -e; node --test hack/night-run/day-watch.test.mjs >/dev/null || exit 1; echo OK
	@echo "== WSL 回环探针 =="; \
	go run ./hack/wsl-loopback-probe
	@echo "== 清单渲染 =="; \
	"$(KUSTOMIZE)" build config/dev >/dev/null && echo OK
	"$(KUSTOMIZE)" build config/demo >/dev/null && echo OK
	"$(KUSTOMIZE)" build dashboard/deploy >/dev/null && echo OK
	@echo "selfcheck 通过"

.PHONY: verify
verify: fmt-check test test-backend test-e2e-compile test-frontend verify-deploy selfcheck lint ## 执行提交前完整静态验证

.PHONY: doctor
doctor: ## 环境自检：磁盘 / Docker / WSL 回环 / 端口 / 内存 / tmpfs / dmesg（开工与长跑前先跑）
	bash hack/doctor.sh

# e2e 使用独立集群，避免测试清理误删日常开发集群。
E2E_KIND_CLUSTER ?= hello-k8s-ai-test-e2e
DEV_KIND_CLUSTER ?= hello-k8s-ai-dev
# 固定 Kind 节点镜像，避免 CI 因 latest 指向变化而漂移。
KIND_NODE_IMAGE ?= kindest/node:v1.36.1@sha256:3489c7674813ba5d8b1a9977baea8a6e553784dab7b84759d1014dbd78f7ebd5

.PHONY: setup-test-e2e
setup-test-e2e: ## 创建 Kind 集群（没有才建）
	@command -v $(KIND) >/dev/null 2>&1 || { \
		echo "Kind 没装，先装一下"; \
		exit 1; \
	}
	@case "$$($(KIND) get clusters)" in \
		*"$(E2E_KIND_CLUSTER)"*) \
			echo "集群 $(E2E_KIND_CLUSTER) 已经在了，跳过";; \
		*) \
			echo "建 Kind 集群 $(E2E_KIND_CLUSTER) ..."; \
			$(KIND) create cluster --name $(E2E_KIND_CLUSTER) --image "$(KIND_NODE_IMAGE)" ;; \
	esac

.PHONY: test-e2e
test-e2e: setup-test-e2e vet ## 跑 e2e 测试（使用仓库中已有 CRD）
	@status=0; \
	KIND=$(KIND) KIND_CLUSTER=$(E2E_KIND_CLUSTER) go test -tags=e2e ./test/e2e/ -v -ginkgo.v || status=$$?; \
	cleanup_status=0; \
	$(MAKE) cleanup-test-e2e || cleanup_status=$$?; \
	if [ "$$status" -ne 0 ]; then exit "$$status"; fi; \
	exit "$$cleanup_status"

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## 删掉 Kind 集群
	@$(KIND) delete cluster --name $(E2E_KIND_CLUSTER)

.PHONY: lint
lint: golangci-lint lint-sh lint-md lint-ps1 ## lint 检查（Go + shell + markdown + PowerShell）
	"$(GOLANGCI_LINT)" run

.PHONY: lint-fix
lint-fix: golangci-lint ## lint 并自动修
	"$(GOLANGCI_LINT)" run --fix

.PHONY: lint-config
lint-config: golangci-lint ## 检查 lint 配置是否有效
	"$(GOLANGCI_LINT)" config verify

##@ 构建

.PHONY: build
build: vet ## 编译 manager（不会重新生成或改写 CRD/API）
	go build -o bin/manager cmd/main.go

.PHONY: run
run: vet ## 本地跑 controller
	go run ./cmd/main.go

# Dockerfile 同时提供 manager/simulator 两个显式 target。
# manager 被放在 Dockerfile 最后，保证误用 `docker build .` 时默认也得到 Controller 镜像。
# Makefile 内所有正式构建仍显式指定 --target，避免依赖 Dockerfile 阶段顺序。
.PHONY: docker-build
docker-build: docker-build-manager ## 兼容旧命令；始终构建 manager 镜像

.PHONY: docker-build-manager
docker-build-manager: ## 构建 manager 镜像并校验入口和 /manager 二进制
	$(BUILD_CMD) $(DOCKER_BUILD_CACHE) --target manager -f $(ROOT_DOCKERFILE) -t $(MANAGER_IMG) .
	@entrypoint="$$( $(CONTAINER_TOOL) image inspect --format '{{json .Config.Entrypoint}}' $(MANAGER_IMG) )"; \
		test "$$entrypoint" = '["/manager"]' || { echo "manager 镜像 ENTRYPOINT 异常：$$entrypoint"; exit 1; }; \
		cid="$$( $(CONTAINER_TOOL) create $(MANAGER_IMG) )"; \
		trap '$(CONTAINER_TOOL) rm -f "$$cid" >/dev/null 2>&1 || true' EXIT; \
		$(CONTAINER_TOOL) export "$$cid" | tar -tf - | sed 's#^\./##' | grep -x 'manager' >/dev/null || { echo "manager 镜像缺少 /manager"; exit 1; }; \
		echo "manager 镜像校验通过：$(MANAGER_IMG)"

.PHONY: docker-build-simulator
docker-build-simulator: ## 构建 simulator 镜像并校验入口和 /simulator 二进制
	$(BUILD_CMD) $(DOCKER_BUILD_CACHE) --target simulator -f $(ROOT_DOCKERFILE) -t $(SIMULATOR_IMG) .
	@entrypoint="$$( $(CONTAINER_TOOL) image inspect --format '{{json .Config.Entrypoint}}' $(SIMULATOR_IMG) )"; \
		test "$$entrypoint" = '["/simulator"]' || { echo "simulator 镜像 ENTRYPOINT 异常：$$entrypoint"; exit 1; }; \
		cid="$$( $(CONTAINER_TOOL) create $(SIMULATOR_IMG) )"; \
		trap '$(CONTAINER_TOOL) rm -f "$$cid" >/dev/null 2>&1 || true' EXIT; \
		$(CONTAINER_TOOL) export "$$cid" | tar -tf - | sed 's#^\./##' | grep -x 'simulator' >/dev/null || { echo "simulator 镜像缺少 /simulator"; exit 1; }; \
		echo "simulator 镜像校验通过：$(SIMULATOR_IMG)"

.PHONY: docker-push
docker-push: ## 兼容旧命令；推送 manager 镜像
	$(CONTAINER_TOOL) push $(MANAGER_IMG)

.PHONY: docker-push-manager
docker-push-manager: ## 推送 manager 镜像
	$(CONTAINER_TOOL) push $(MANAGER_IMG)

.PHONY: docker-push-simulator
docker-push-simulator: ## 推送 simulator 镜像
	$(CONTAINER_TOOL) push $(SIMULATOR_IMG)

.PHONY: docker-build-local
docker-build-local: ## 并行构建并校验本地完整栈四个项目镜像
	@$(MAKE) -j2 docker-build-manager docker-build-simulator & \
	$(BUILD_CMD) $(DOCKER_BUILD_CACHE) -t $(BACKEND_IMG) dashboard/backend & \
	$(BUILD_CMD) $(DOCKER_BUILD_CACHE) -t $(FRONTEND_IMG) dashboard/frontend/my-app & \
	wait

# 跨平台构建。显式 target，避免再次把 simulator 误打成 controller。
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
BUILDX_BUILDER ?= hello-k8s-ai-builder
.PHONY: docker-buildx
docker-buildx: ## 跨平台构建并推送 manager
	- $(CONTAINER_TOOL) buildx create --name $(BUILDX_BUILDER)
	$(CONTAINER_TOOL) buildx use $(BUILDX_BUILDER)
	$(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --target manager --tag $(MANAGER_IMG) -f $(ROOT_DOCKERFILE) .
	- $(CONTAINER_TOOL) buildx rm $(BUILDX_BUILDER)

.PHONY: docker-buildx-simulator
docker-buildx-simulator: ## 跨平台构建并推送 simulator
	- $(CONTAINER_TOOL) buildx create --name $(BUILDX_BUILDER)
	$(CONTAINER_TOOL) buildx use $(BUILDX_BUILDER)
	$(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --target simulator --tag $(SIMULATOR_IMG) -f $(ROOT_DOCKERFILE) .
	- $(CONTAINER_TOOL) buildx rm $(BUILDX_BUILDER)

.PHONY: build-installer
build-installer: kustomize ## 用仓库中已有 CRD 生成一键安装的 all-in-one yaml
	mkdir -p dist
	cd config/manager && "$(KUSTOMIZE)" edit set image hello-k8s-ai-controller=$(MANAGER_IMG)
	"$(KUSTOMIZE)" build config/default > dist/install.yaml

##@ 部署

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: kustomize ## 安装仓库中已有 CRD
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply -f -; else echo "没有可安装的 CRD，跳过。"; fi

.PHONY: uninstall
uninstall: kustomize ## 卸载仓库中已有 CRD
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -; else echo "没有可删除的 CRD，跳过。"; fi

.PHONY: deploy
deploy: kustomize ## 部署 controller（不会重新生成 CRD）
	cd config/manager && "$(KUSTOMIZE)" edit set image hello-k8s-ai-controller=$(MANAGER_IMG)
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" apply -f -

.PHONY: undeploy
undeploy: kustomize ## 卸载 controller
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -

##@ Docker Desktop 本地完整栈

.PHONY: cluster-up
kind-up: ## 创建/复用 Kind 开发集群并安装持久化存储（幂等）
	@DEV_KIND_CLUSTER="$(DEV_KIND_CLUSTER)" KIND_NODE_IMAGE="$(KIND_NODE_IMAGE)" ./hack/kind/cluster-up.sh

kind-down: ## 删除 Kind 开发集群（PVC 数据保留在 /var/lib/hello-k8s-ai-pv，重建自动挂回）
	@DEV_KIND_CLUSTER="$(DEV_KIND_CLUSTER)" ./hack/kind/cluster-down.sh

cluster-up: kind-up ## 一键：Kind 集群（没有才建）→ 构建部署 → 验收 → 本地端口
	@KUBE_CONTEXT="$(KUBE_CONTEXT)" NAMESPACE="$(NAMESPACE)" \
		MANAGER_IMG="$(MANAGER_IMG)" SIMULATOR_IMG="$(SIMULATOR_IMG)" \
		BACKEND_IMG="$(BACKEND_IMG)" FRONTEND_IMG="$(FRONTEND_IMG)" \
		DEMO_MODEL_ABSOLUTE_SCORE="$(DEMO_MODEL_ABSOLUTE_SCORE)" \
		CONTAINER_TOOL="$(CONTAINER_TOOL)" ./hack/local-cluster.sh up

.PHONY: cluster-status
cluster-status: ## 查看完整栈、CR、PVC 与 API 健康状态
	@KUBE_CONTEXT="$(KUBE_CONTEXT)" NAMESPACE="$(NAMESPACE)" ./hack/local-cluster.sh status

.PHONY: cluster-open
cluster-open: ## 启动 Dashboard 本地端口（Grafana/Prometheus/Jaeger 经 Dashboard 单入口访问）
	@KUBE_CONTEXT="$(KUBE_CONTEXT)" NAMESPACE="$(NAMESPACE)" ./hack/local-cluster.sh open

.PHONY: cluster-urls
cluster-urls: ## 打印本地访问地址
	@KUBE_CONTEXT="$(KUBE_CONTEXT)" NAMESPACE="$(NAMESPACE)" ./hack/local-cluster.sh urls

.PHONY: cluster-down
cluster-down: ## 停止项目工作负载并保留集群、CRD、CR 与数据库 PVC
	@KUBE_CONTEXT="$(KUBE_CONTEXT)" NAMESPACE="$(NAMESPACE)" ./hack/local-cluster.sh down

.PHONY: cluster-render
cluster-render: ## 使用 kubectl 内置 Kustomize 渲染完整栈
	@"$(KUBECTL)" kustomize config/dev
	@"$(KUBECTL)" kustomize dashboard/deploy

.PHONY: cleanup-obsolete
cleanup-obsolete: ## 覆盖旧目录后清理已确认废弃的文件
	@./hack/cleanup-obsolete.sh

##@ 工具依赖

# 本地 bin 目录
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

KUBECTL ?= kubectl
KIND ?= kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint

# 工具版本，可按需改
KUSTOMIZE_VERSION ?= v5.8.1
CONTROLLER_TOOLS_VERSION ?= v0.21.0
ENVTEST_VERSION ?= release-0.24

# envtest 用的 K8s 版本，根据 go.mod 里 k8s.io/api 自动取
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "无法从 k8s.io/api 版本推导 ENVTEST_K8S_VERSION，请手工设置" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

GOLANGCI_LINT_VERSION ?= v2.12.2

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## 本地装 kustomize
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## 本地装 controller-gen
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## 下载 envtest 二进制
	@echo "准备 envtest (K8s $(ENVTEST_K8S_VERSION))..."
	@"$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path || { \
		echo "envtest 二进制拉取失败"; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## 本地装 setup-envtest
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## 本地装 golangci-lint
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))
	@test -f .custom-gcl.yml && { \
		echo "有自定义插件，重新编译 golangci-lint ..." && \
		$(GOLANGCI_LINT) custom --destination $(LOCALBIN) --name golangci-lint-custom && \
		mv -f $(LOCALBIN)/golangci-lint-custom $(GOLANGCI_LINT); \
	} || true

# go-install-tool: 如果没有或者版本不对就 go install，然后链接到固定路径
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "下载 $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

# 从 go.mod 中取某个模块的版本
define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef

# 生成远程 AI 上下文包（输出 .runtime/context-pack/，不提交）
.PHONY: context-pack
context-pack:
	bash hack/gen-context-pack.sh

# 检查 Markdown 相对链接与图片路径
.PHONY: docs-check
docs-check:
	python3 hack/check-docs.py

# 生成派生文档（README 时间线段 / docs/status.md / llms.txt / 所有权表）
.PHONY: docs-sync
docs-sync:
	python3 hack/gen-docs.py

# 派生文档必须是最新的：先重新生成，再要求工作区无差异
.PHONY: docs-sync-check
docs-sync-check: docs-sync
	@if [ -n "$$(git status --porcelain)" ]; then echo "派生文件或工作区存在未提交差异："; git status --porcelain; exit 1; fi
	@echo "docs-sync-check OK"

