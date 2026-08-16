package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
)

const idempotencyRetention = 24 * time.Hour

type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newBufferedResponse() *bufferedResponse {
	return &bufferedResponse{header: make(http.Header)}
}

func (response *bufferedResponse) Header() http.Header {
	return response.header
}

func (response *bufferedResponse) WriteHeader(status int) {
	if response.status != 0 {
		return
	}
	response.status = status
}

func (response *bufferedResponse) Write(body []byte) (int, error) {
	if response.status == 0 {
		response.status = http.StatusOK
	}
	return response.body.Write(body)
}

func idempotencyMiddleware(
	database store.Store,
	maxBodyBytes int64,
	logger *slog.Logger,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !isMutation(request.Method) {
			next.ServeHTTP(writer, request)
			return
		}
		key := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
		if key == "" {
			writeProblem(writer, request, http.StatusBadRequest, "MISSING_IDEMPOTENCY_KEY", "Idempotency-Key is required for commands.", false, nil)
			return
		}
		if !database.Available() {
			writeProblem(writer, request, http.StatusServiceUnavailable, "COMMAND_STORE_UNAVAILABLE", "Commands are disabled while the audit and idempotency store is unavailable.", true, nil)
			return
		}
		if len(key) > 200 || strings.ContainsAny(key, "\x00\r\n") {
			writeProblem(writer, request, http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "Idempotency-Key must contain at most 200 safe characters.", false, nil)
			return
		}

		body, err := readRequestBody(request, maxBodyBytes)
		if err != nil {
			writeProblem(writer, request, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", err.Error(), false, nil)
			return
		}
		requestHash := commandRequestHash(request, body)
		record, owned, err := database.ReserveIdempotency(
			request.Context(), key, requestHash, time.Now().UTC().Add(idempotencyRetention),
		)
		if err != nil {
			writeProblem(writer, request, http.StatusServiceUnavailable, "IDEMPOTENCY_STORE_UNAVAILABLE", "The command could not be safely reserved.", true, nil)
			return
		}
		if !owned {
			serveIdempotencyRecord(writer, request, record, requestHash)
			return
		}

		defer func() {
			if recovered := recover(); recovered != nil {
				releaseContext, cancel := context.WithTimeout(context.WithoutCancel(request.Context()), 3*time.Second)
				if releaseErr := database.ReleaseIdempotency(releaseContext, key, requestHash); releaseErr != nil {
					logger.Error("Could not release failed idempotency reservation", "key", key, "error", releaseErr)
				}
				cancel()
				panic(recovered)
			}
		}()

		response := newBufferedResponse()
		next.ServeHTTP(response, request)
		if response.status == 0 {
			response.status = http.StatusOK
		}
		completeContext, cancel := context.WithTimeout(context.WithoutCancel(request.Context()), 3*time.Second)
		err = database.CompleteIdempotency(
			completeContext, key, requestHash, response.status, response.body.Bytes(),
		)
		cancel()
		if err != nil {
			logger.Error("Could not complete command idempotency record", "key", key, "error", err)
			// 完成记录失败时释放占位，避免同一 key 被 pending 卡满保留期；
			// 命令本身已执行，重放依赖 Kubernetes 侧 apply 幂等语义。
			releaseContext, releaseCancel := context.WithTimeout(context.WithoutCancel(request.Context()), 3*time.Second)
			if releaseErr := database.ReleaseIdempotency(releaseContext, key, requestHash); releaseErr != nil {
				logger.Error("Could not release idempotency reservation after completion failure", "key", key, "error", releaseErr)
			}
			releaseCancel()
		}
		copyHeaders(writer.Header(), response.header)
		writer.WriteHeader(response.status)
		_, _ = writer.Write(response.body.Bytes())
	})
}

func serveIdempotencyRecord(
	writer http.ResponseWriter,
	request *http.Request,
	record *store.IdempotencyRecord,
	requestHash string,
) {
	if record == nil {
		writeProblem(writer, request, http.StatusServiceUnavailable, "IDEMPOTENCY_STATE_INVALID", "The command reservation could not be read.", true, nil)
		return
	}
	if record.RequestHash != requestHash {
		writeProblem(writer, request, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "The Idempotency-Key is already associated with a different request.", false, nil)
		return
	}
	if record.State != "completed" {
		writer.Header().Set("Retry-After", "1")
		writeProblem(writer, request, http.StatusConflict, "COMMAND_IN_PROGRESS", "A command with this Idempotency-Key is still in progress.", true, nil)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Idempotent-Replay", "true")
	writer.WriteHeader(record.StatusCode)
	_, _ = writer.Write(record.Response)
}

func readRequestBody(request *http.Request, maxBodyBytes int64) ([]byte, error) {
	if request.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxBodyBytes+1))
	_ = request.Body.Close()
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBodyBytes {
		return nil, &http.MaxBytesError{Limit: maxBodyBytes}
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func commandRequestHash(request *http.Request, body []byte) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(request.Method))
	_, _ = digest.Write([]byte("\n"))
	_, _ = digest.Write([]byte(request.URL.EscapedPath()))
	_, _ = digest.Write([]byte("?"))
	_, _ = digest.Write([]byte(request.URL.RawQuery))
	_, _ = digest.Write([]byte("\n"))
	_, _ = digest.Write(body)
	return hex.EncodeToString(digest.Sum(nil))
}

func copyHeaders(target http.Header, source http.Header) {
	for key, values := range source {
		target[key] = append([]string(nil), values...)
	}
}

func isMutation(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}
