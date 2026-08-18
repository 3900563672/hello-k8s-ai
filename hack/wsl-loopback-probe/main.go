// 命令：WSL 回环中继健康探针。
// 背景：WSL2 NAT 模式下 localhost 转发注册需要时间（本机实测健康态约 50-100ms），
// 旧版"listen 后立即拨号"在健康态也必然失败，属误报。
// 本版语义：
//  1. 每轮新建 127.0.0.1 随机端口 + HTTP 应答，测量 WSL 内首次连接成功时延（0→2s 每 50ms 重试）；
//  2. 若有 curl.exe（Windows 互操作），以 Windows 侧视角请求新端口，验证 localhost 转发注册已落地；
//  3. 读取 dmesg 中中继错误计数（UtilAcceptVsock）作为症状计数（不单独判级）。
//
// 已知平台行为（2026-08-18 实测）：同一进程内"listen→连接→curl→关闭"连续多轮时，
// 第 2 轮起的新端口注册大概率停滞 >2s（约 2-5s 后自愈），与实现语言/轮间间隔无关的
// 会话拆除竞争。因此：
//   - 默认单轮（-attempts 1）：新鲜进程单轮在健康态 100% 通过（15+ 次观测），持久故障态必失败；
//   - -attempts >1 仅作诊断/压力复现用（会命中上述瞬态，结果 WARN/FAIL 不代表持久故障）。
//
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
	attempts := flag.Int("attempts", 1, "测量轮数；>1 会命中会话拆除竞争（诊断用），默认 1")
	delay := flag.Int("delay", 0, "轮间间隔毫秒（诊断用）")
	flag.Parse()
	if !isWSL() {
		fmt.Println("[wsl-loopback] RESULT: SKIP 非 WSL2 环境，无需检查")
		return
	}
	latencyOK, windowsOK := 0, 0
	hasCurl := false
	if _, err := exec.LookPath("curl.exe"); err == nil {
		hasCurl = true
	}
	for i := 0; i < *attempts; i++ {
		lat, win := freshRound(hasCurl)
		latOK := lat > 0 && lat <= 2*time.Second
		if latOK {
			latencyOK++
		}
		if win {
			windowsOK++
		}
		fmt.Printf("[wsl-loopback] 第 %d 轮：WSL 内首次成功=%s；Windows 侧 curl=%v\n", i+1, lat, win)
		if i < *attempts-1 && *delay > 0 {
			time.Sleep(time.Duration(*delay) * time.Millisecond)
		}
	}
	relayErrors := relayErrorCount()
	fmt.Printf("[wsl-loopback] %d 轮：注册延迟达标 %d/%d；Windows 侧可达 %d/%d；中继错误计数=%s；curl.exe=%v\n",
		*attempts, latencyOK, *attempts, windowsOK, *attempts, relayErrorText(relayErrors), hasCurl)
	switch {
	case hasCurl && windowsOK == 0:
		fmt.Println("[wsl-loopback] RESULT: FAIL Windows 侧新端口全部不可达（注册链路失效），建议 wsl --shutdown 后复测（需用户同意）")
	case !hasCurl && latencyOK < *attempts:
		fmt.Println("[wsl-loopback] RESULT: FAIL WSL 内注册延迟超过 2s（无 curl.exe，仅本地判据）")
	case windowsOK > 0 && windowsOK < *attempts:
		fmt.Println("[wsl-loopback] RESULT: WARN 部分轮次失败（-attempts>1 时多为已知会话拆除竞争，属瞬态，正常间隔重测即可）")
	case latencyOK > 0 && latencyOK < *attempts:
		fmt.Println("[wsl-loopback] RESULT: WARN 部分轮次注册延迟超 2s（-attempts>1 时多为已知会话拆除竞争，属瞬态）")
	default:
		fmt.Println("[wsl-loopback] RESULT: PASS 新端口注册正常，回环中继健康")
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

// freshRound 新建 127.0.0.1 随机端口，测量 WSL 内首次连接成功时延，并可选做 Windows 侧 curl 校验。
// 返回 (首次成功时延；0 表示 2s 内未成功, Windows 侧是否 200)。
func freshRound(checkWindows bool) (time.Duration, bool) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return 0, false
	}
	defer func() { _ = ln.Close() }()
	go func() { // 简易 HTTP 应答，供 Windows 侧 curl 校验
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
				buf := make([]byte, 256)
				_, _ = c.Read(buf)
				_, _ = c.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK"))
			}(conn)
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	start := time.Now()
	firstOK := time.Duration(0)
	for {
		conn, err := net.DialTimeout("tcp4", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			firstOK = time.Since(start)
			break
		}
		if time.Since(start) > 2*time.Second {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	winOK := false
	if checkWindows {
		time.Sleep(300 * time.Millisecond) // 留出 Windows 侧注册窗口
		cmd := exec.Command("curl.exe", "-s", "-m", "3", "-o", "NUL", "-w", "%{http_code}", fmt.Sprintf("http://127.0.0.1:%d/", port))
		out, err := cmd.Output()
		winOK = err == nil && strings.Contains(string(out), "200")
	}
	return firstOK, winOK
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
