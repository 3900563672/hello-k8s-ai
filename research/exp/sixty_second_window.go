//go:build ignore
// 实验 2：60s 去分配窗口验证
// bind 固定端口 → close → 立即重 bind → 保持；Windows 侧同步观察 netstat
package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

func main() {
	port := 45678
	hold := 75
	if len(os.Args) > 1 {
		port, _ = strconv.Atoi(os.Args[1])
	}
	if len(os.Args) > 2 {
		hold, _ = strconv.Atoi(os.Args[2])
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ts := func(tag string) { fmt.Printf("%s %s\n", tag, time.Now().Format("15:04:05.000")) }
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println("first bind FAILED:", err)
		os.Exit(1)
	}
	ts("T0_bind")
	time.Sleep(3 * time.Second)
	ln.Close()
	ts("T1_closed")
	ln2, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println("rebind FAILED:", err)
		os.Exit(1)
	}
	ts("T2_rebound")
	time.Sleep(time.Duration(hold) * time.Second)
	ln2.Close()
	ts("T3_done")
}
