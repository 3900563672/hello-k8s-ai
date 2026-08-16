package api

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
)

const identityKey contextKey = "identity"

// principal 表示一次请求的已识别身份；authenticated 为 true 表示该请求已通过
// ADMIN_TOKEN 的 Bearer 校验，只有这类请求才允许携带可信任的上游身份头。
type principal struct {
	name          string
	authenticated bool
}

// authMiddleware 为所有写请求建立可信身份边界：
//   - 配置了 ADMIN_TOKEN 时，写请求必须携带匹配的 Bearer Token，否则返回 401；
//   - 未配置 ADMIN_TOKEN 时，仅非生产环境允许匿名写（保持本地演示可用），
//     生产环境直接拒绝写请求（503），避免控制面无认证暴露；
//   - 只读请求不要求认证，身份记为 system:anonymous。
func authMiddleware(httpConfig config.HTTPConfig, environment string, logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		identity := principal{name: "system:anonymous"}
		if isWriteRequest(request) {
			token := bearerToken(request)
			switch {
			case httpConfig.AdminToken != "":
				if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(httpConfig.AdminToken)) != 1 {
					writeProblem(writer, request, http.StatusUnauthorized, "UNAUTHORIZED", "A valid admin bearer token is required for write operations.", false, nil)
					return
				}
				identity = principal{name: "admin", authenticated: true}
			case environment == "production":
				writeProblem(writer, request, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED", "Write operations are disabled because ADMIN_TOKEN is not configured.", false, nil)
				return
			default:
				logger.Warn("Write request accepted without admin token in non-production environment", "method", request.Method, "path", request.URL.Path)
			}
		}
		next.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), identityKey, identity)))
	})
}

func isWriteRequest(request *http.Request) bool {
	switch request.Method {
	case http.MethodPost, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func bearerToken(request *http.Request) string {
	header := strings.TrimSpace(request.Header.Get("Authorization"))
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

// actorName 返回用于审计日志的主体名。X-Remote-User 只在请求已通过 Bearer
// 认证且显式开启 TRUST_REMOTE_USER_HEADER 时可信；否则一律使用认证身份，
// 防止任意调用方伪造上游身份头。
func actorName(request *http.Request, trustRemoteUser bool) string {
	identity, _ := request.Context().Value(identityKey).(principal)
	if identity.authenticated && trustRemoteUser {
		if user := strings.TrimSpace(request.Header.Get("X-Remote-User")); user != "" {
			return user
		}
	}
	if identity.name == "" {
		return "system:anonymous"
	}
	return identity.name
}
