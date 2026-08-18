//go:build ignore
// 实验 3c：被动关闭监听器——客户端主动关闭后进入 TIME_WAIT 的验证
// 持续监听 port，每连接：读至 EOF（客户端先关闭）→ 服务端关闭（被动关闭）
// 用法：go run tw_passive_listener.go <port>
package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

func main() {
	port := 45682
	if len(os.Args) > 1 {
		port, _ = strconv.Atoi(os.Args[1])
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println("listen FAILED:", err)
		os.Exit(1)
	}
	fmt.Printf("%s listening on %s\n", time.Now().Format("15:04:05.000"), addr)
	nconn := 0
	for {
		c, err := ln.Accept()
		if err != nil {
			fmt.Println("accept err:", err)
			continue
		}
		nconn++
		fmt.Printf("%s ACCEPT #%d %s\n", time.Now().Format("15:04:05.000"), nconn, c.RemoteAddr())
		buf := make([]byte, 64)
		for {
			if _, err := c.Read(buf); err != nil {
				break // EOF: client closed first
			}
		}
		c.Close()
		fmt.Printf("%s CLOSE_AFTER_CLIENT_EOF #%d\n", time.Now().Format("15:04:05.000"), nconn)
	}
}
