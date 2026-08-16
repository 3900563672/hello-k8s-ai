package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
)

func authTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func authTestHandler(adminToken string, trustRemoteUser bool, environment string) http.Handler {
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})
	return authMiddleware(config.HTTPConfig{AdminToken: adminToken, TrustRemoteUser: trustRemoteUser}, environment, authTestLogger(), next)
}

func TestAuthMiddlewareWriteRequiresValidBearerToken(t *testing.T) {
	handler := authTestHandler("secret-token", false, "production")

	request := httptest.NewRequest(http.MethodPost, "/api/v1/configuration:apply", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("write without token: status=%d, want %d", recorder.Code, http.StatusUnauthorized)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/configuration:apply", nil)
	request.Header.Set("Authorization", "Bearer wrong-token")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("write with wrong token: status=%d, want %d", recorder.Code, http.StatusUnauthorized)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/configuration:apply", nil)
	request.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("write with non-bearer scheme: status=%d, want %d", recorder.Code, http.StatusUnauthorized)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/configuration:apply", nil)
	request.Header.Set("Authorization", "Bearer secret-token")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("write with valid token: status=%d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestAuthMiddlewareNonProductionAllowsAnonymousWrite(t *testing.T) {
	handler := authTestHandler("", false, "development")

	request := httptest.NewRequest(http.MethodPatch, "/api/v1/tenants/tenant-a/traffic", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("anonymous write in development: status=%d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestAuthMiddlewareProductionRejectsWriteWithoutToken(t *testing.T) {
	handler := authTestHandler("", false, "production")

	request := httptest.NewRequest(http.MethodDelete, "/api/v1/configuration/Tenant/tenant-a", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("anonymous write in production: status=%d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestAuthMiddlewareReadRequestBypassesAuth(t *testing.T) {
	handler := authTestHandler("secret-token", false, "production")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("read without token: status=%d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestAuthMiddlewareOptionsPreflightBypassesAuth(t *testing.T) {
	handler := authTestHandler("secret-token", false, "production")

	request := httptest.NewRequest(http.MethodOptions, "/api/v1/configuration:apply", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("CORS preflight: status=%d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestActorNameTrustsRemoteUserOnlyWhenAuthenticated(t *testing.T) {
	cases := []struct {
		name            string
		adminToken      string
		trustRemoteUser bool
		environment     string
		authorization   string
		remoteUser      string
		want            string
	}{
		{name: "authenticated with trusted header", adminToken: "secret", trustRemoteUser: true, environment: "production", authorization: "Bearer secret", remoteUser: "alice", want: "alice"},
		{name: "authenticated without trusted header", adminToken: "secret", trustRemoteUser: false, environment: "production", authorization: "Bearer secret", remoteUser: "attacker", want: "admin"},
		{name: "anonymous cannot spoof header", adminToken: "", trustRemoteUser: true, environment: "development", remoteUser: "attacker", want: "system:anonymous"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				got = actorName(request, tc.trustRemoteUser)
				writer.WriteHeader(http.StatusOK)
			})
			handler := authMiddleware(config.HTTPConfig{AdminToken: tc.adminToken, TrustRemoteUser: tc.trustRemoteUser}, tc.environment, authTestLogger(), next)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/configuration:apply", nil)
			if tc.authorization != "" {
				request.Header.Set("Authorization", tc.authorization)
			}
			if tc.remoteUser != "" {
				request.Header.Set("X-Remote-User", tc.remoteUser)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("request rejected: status=%d", recorder.Code)
			}
			if got != tc.want {
				t.Fatalf("actorName = %q, want %q", got, tc.want)
			}
		})
	}
}
