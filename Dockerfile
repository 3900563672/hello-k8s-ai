# syntax=docker/dockerfile:1

# 公共构建基础：manager 和 simulator 共用 Go 依赖缓存。
FROM golang:1.26 AS builder-base
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /workspace

# 先复制依赖描述，最大化利用 Docker layer cache。
COPY go.mod go.sum ./
RUN go mod download

# 再复制源码。
COPY . .

# 构建 Controller Manager。
FROM builder-base AS builder-manager
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/manager ./cmd

# 构建 Simulator。
FROM builder-base AS builder-simulator
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/simulator ./simulator

# Simulator 最终镜像。
# 需要 simulator 时必须显式使用：docker build --target simulator ...
FROM gcr.io/distroless/static:nonroot AS simulator
WORKDIR /
COPY --from=builder-simulator --chown=65532:65532 /out/simulator /simulator
USER 65532:65532
ENTRYPOINT ["/simulator"]

# Manager 最终镜像。
# 故意放在 Dockerfile 最后：即使误执行 `docker build .`，默认得到的也是 Controller，
# 不会再次把 simulator 镜像错误标记成 hello-k8s-ai-controller。
FROM gcr.io/distroless/static:nonroot AS manager
WORKDIR /
COPY --from=builder-manager --chown=65532:65532 /out/manager /manager
USER 65532:65532
ENTRYPOINT ["/manager"]
