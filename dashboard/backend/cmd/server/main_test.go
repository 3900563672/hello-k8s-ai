package main

import (
	"os"
	"os/exec"
	"testing"
)

// helper 子进程模式：TestMainBadConfigExits / TestMainDBUnreachableExits 以
// BE_MAIN_HELPER=1 重新执行自身并调用 main()，验证不同配置下的退出码。
// 避免在测试进程内直接调用 main()（os.Exit 会终止测试进程）。

func TestMainBadConfigExits(t *testing.T) {
	if os.Getenv("BE_MAIN_HELPER") == "1" {
		_ = os.Setenv("LOG_LEVEL", "not-a-level")
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMainBadConfigExits")
	cmd.Env = append(os.Environ(), "BE_MAIN_HELPER=1")
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 2 {
		t.Fatalf("bad config should exit 2, got %v", err)
	}
}

func TestMainDBUnreachableExits(t *testing.T) {
	if os.Getenv("BE_MAIN_HELPER") == "1" {
		_ = os.Setenv("LOG_LEVEL", "info")
		_ = os.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/none?sslmode=disable")
		_ = os.Setenv("DATABASE_REQUIRED", "true")
		_ = os.Setenv("DATABASE_CONNECT_TIMEOUT", "1s")
		_ = os.Setenv("DATABASE_STARTUP_RETRIES", "0")
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMainDBUnreachableExits")
	cmd.Env = append(os.Environ(), "BE_MAIN_HELPER=1")
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("unreachable DB should exit 1, got %v", err)
	}
}
