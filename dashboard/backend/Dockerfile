FROM golang:1.26 AS builder

ARG TARGETOS=linux
ARG TARGETARCH

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /dashboard-backend ./cmd/server

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /dashboard-backend /dashboard-backend
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/dashboard-backend"]
