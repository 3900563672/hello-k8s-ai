//go:build ignore
// 实验 3b：有连接历史的端口 rebind 时长（Windows 侧 TIME_WAIT 阻塞验证）
// 流程：listen P → accept 1 → 立即 close（服务端主动关闭 → Windows 侧 P 进入 TIME_WAIT）
//       → close listener → 循环重 listen P，记录每次失败与最终成功的时间
// 用法：go run tw_rebind.go <port>
package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

func main() {
	port := 45681
	if len(os.Args) > 1 {
		port, _ = strconv.Atoi(os.Args[1])
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ts := func(tag string) { fmt.Printf("%s %s\n", tag, time.Now().Format("15:04:05.000")) }

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println("first listen FAILED:", err)
		os.Exit(1)
	}
	ts("L0_listen")
	// accept one connection and close it immediately (server active close -> Windows side P -> TIME_WAIT)
	c, err := ln.Accept()
	if err != nil {
		fmt.Println("accept err:", err)
	} else {
		ts("L1_accepted " + c.RemoteAddr().String())
		c.Close()
		ts("L2_server_closed_conn")
	}
	ln.Close()
	ts("L3_listener_closed")

	// rebind loop: measure how long WSAEADDRINUSE (Windows TIME_WAIT) blocks rebind
	start := time.Now()
	attempt := 0
	for {
		attempt++
		ln2, err := net.Listen("tcp", addr)
		if err != nil {
			if attempt == 1 || attempt%10 == 0 {
				fmt.Printf("REBIND_FAIL #%d elapsed=%v err=%v\n", attempt, time.Since(start).Round(time.Millisecond), err)
			}
			time.Sleep(1 * time.Second)
			if time.Since(start) > 150*time.Second {
				fmt.Println("REBIND_TIMEOUT after 150s")
				os.Exit(2)
			}
			continue
		}
		fmt.Printf("REBIND_OK #%d elapsed=%v\n", attempt, time.Since(start).Round(time.Millisecond))
		ln2.Close()
		break
	}
	ts("L4_done")
}
