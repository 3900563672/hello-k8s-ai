// 命令：WSL 回环中继健康探针。
// 背景：WSL2 NAT 模式下 localhost 转发中继（guest 侧 Relay，/init 子进程）降级时，
// 新监听的 IPv4 回环端口首连会被 RST（127.0.0.1）或丢弃（127.0.0.2），
// 失败窗口约 0-300ms 且为间歇性；eth0/IPv6/长存活端口不受影响。
// 本探针复现该路径：每次新建 127.0.0.1 随机端口并立即拨号，统计失败率；
// 并读取 dmesg 中中继错误计数（UtilAcceptVsock）作为佐证。
// 非 WSL 环境自动 SKIP（CI 安全）。结果仅输出，不设置退出码，由调用方决定门禁。
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

const relayErrorMarker = "UtilAcceptVsock"

func main() {
	attempts := flag.Int("attempts", 10, "新建端口立即拨号次数")
	flag.Parse()
	if !isWSL() {
		fmt.Println("[wsl-loopback] RESULT: SKIP 非 WSL2 环境，无需检查")
		return
	}
	failures := 0
	for i := 0; i < *attempts; i++ {
		if err := freshDial(); err != nil {
			failures++
			if i < 5 {
				fmt.Printf("[wsl-loopback] 第 %d 次新建端口立即拨号失败: %v\n", i+1, err)
			}
		}
	}
	relayErrors := relayErrorCount()
	fmt.Printf("[wsl-loopback] 新建端口立即拨号 %d 次，失败 %d 次；中继错误计数=%s\n",
		*attempts, failures, relayErrorText(relayErrors))
	switch {
	case failures >= *attempts:
		fmt.Println("[wsl-loopback] RESULT: FAIL 新端口首连全部失败，中继严重降级，建议整机重启或 wsl --shutdown 后复测（需用户同意）")
	case failures > 0:
		fmt.Println("[wsl-loopback] RESULT: WARN 存在间歇性首连失败，中继疑似降级；本地测试可先自连一次完成端口注册，或择机重启")
	case relayErrors > 0:
		fmt.Println("[wsl-loopback] RESULT: WARN 首连正常但 dmesg 存在中继错误记录，中继曾有降级")
	default:
		fmt.Println("[wsl-loopback] RESULT: PASS 新端口首连正常，中继健康")
	}
}

// isWSL 通过 /proc/version 判断是否运行在 WSL 内核。
func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// freshDial 新建 127.0.0.1 随机端口并立即拨号一次，模拟"新监听端口首连"路径。
func freshDial() error {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer func() { _ = ln.Close() }()
	conn, err := net.DialTimeout("tcp4", ln.Addr().String(), 500*time.Millisecond)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

// relayErrorCount 统计 dmesg 中 WSL Relay 错误条数；不可读时返回 -1。
func relayErrorCount() int {
	out, err := exec.Command("dmesg").Output()
	if err != nil {
		return -1
	}
	count := 0
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.Contains(line, relayErrorMarker) {
			count++
		}
	}
	return count
}

func relayErrorText(count int) string {
	if count < 0 {
		return "dmesg 不可读"
	}
	return fmt.Sprintf("%d", count)
}
