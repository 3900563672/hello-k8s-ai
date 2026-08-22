package api

import (
	"bytes"
	"net/http"
	"testing"
)

func TestBufferedResponseWriteBranches(t *testing.T) {
	response := newBufferedResponse()
	if response.Header() == nil {
		t.Fatal("Header 不应为 nil")
	}
	response.WriteHeader(http.StatusCreated)
	response.WriteHeader(http.StatusTeapot) // 第二次写入被忽略
	if response.status != http.StatusCreated {
		t.Fatalf("status = %d, want 201", response.status)
	}
	if _, err := response.Write([]byte("hello")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !bytes.Equal(response.body.Bytes(), []byte("hello")) {
		t.Fatalf("body = %q", response.body.Bytes())
	}

	implicit := newBufferedResponse()
	if _, err := implicit.Write([]byte("x")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if implicit.status != http.StatusOK {
		t.Fatalf("Write 未设置隐式 200: %d", implicit.status)
	}
}
