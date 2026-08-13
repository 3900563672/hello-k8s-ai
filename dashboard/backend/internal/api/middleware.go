package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request-id"

type responseMeta struct {
	RequestID      string            `json:"requestId"`
	ServedAt       time.Time         `json:"servedAt"`
	Partial        bool              `json:"partial"`
	Warnings       []string          `json:"warnings"`
	SourceVersions map[string]string `json:"sourceVersions"`
}

type envelope struct {
	Data any          `json:"data"`
	Meta responseMeta `json:"meta"`
}

type problem struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
}

type problemEnvelope struct {
	Error problem      `json:"error"`
	Meta  responseMeta `json:"meta"`
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (writer *statusRecorder) Flush() {
	if flusher, ok := writer.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (writer *statusRecorder) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func (writer *statusRecorder) WriteHeader(status int) {
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *statusRecorder) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	count, err := writer.ResponseWriter.Write(body)
	writer.bytes += count
	return count, err
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := strings.TrimSpace(request.Header.Get("X-Request-ID"))
		if requestID == "" || len(requestID) > 128 {
			requestID = randomIdentifier("req")
		}
		writer.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), requestIDKey, requestID)))
	})
}

func requestTimeoutMiddleware(timeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if timeout <= 0 || request.URL.Path == "/api/v1/stream" {
			next.ServeHTTP(writer, request)
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), timeout)
		defer cancel()
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func recoveryMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error(
					"Dashboard API handler panicked",
					"requestId", requestID(request.Context()),
					"method", request.Method,
					"path", request.URL.Path,
					"panic", recovered,
					"stack", string(debug.Stack()),
				)
				writeProblem(writer, request, http.StatusInternalServerError, "INTERNAL_ERROR", "The Dashboard Backend could not complete the request.", true, nil)
			}
		}()
		next.ServeHTTP(writer, request)
	})
}

func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: writer}
		next.ServeHTTP(recorder, request)
		logger.Info(
			"Served Dashboard API request",
			"requestId", requestID(request.Context()),
			"method", request.Method,
			"path", request.URL.Path,
			"status", recorder.status,
			"bytes", recorder.bytes,
			"durationMs", time.Since(started).Milliseconds(),
		)
	})
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(writer, request)
	})
}

func corsMiddleware(allowedOrigins []string, next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if _, ok := allowed[origin]; ok {
			writer.Header().Set("Access-Control-Allow-Origin", origin)
			writer.Header().Set("Vary", "Origin")
			writer.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, If-Match, Idempotency-Key, X-Request-ID")
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		}
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func writeData(writer http.ResponseWriter, request *http.Request, status int, data any, partial bool, warnings []string, versions map[string]string) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(envelope{
		Data: data,
		Meta: responseMeta{
			RequestID:      requestID(request.Context()),
			ServedAt:       time.Now().UTC(),
			Partial:        partial,
			Warnings:       nonNilStrings(warnings),
			SourceVersions: nonNilMap(versions),
		},
	})
}

func writeProblem(writer http.ResponseWriter, request *http.Request, status int, code, message string, retryable bool, details map[string]any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(problemEnvelope{
		Error: problem{Code: code, Message: message, Retryable: retryable, Details: details},
		Meta: responseMeta{
			RequestID:      requestID(request.Context()),
			ServedAt:       time.Now().UTC(),
			Warnings:       []string{},
			SourceVersions: map[string]string{},
		},
	})
}

func requestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func randomIdentifier(prefix string) string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err == nil {
		return prefix + "-" + hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func nonNilMap(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	return values
}
