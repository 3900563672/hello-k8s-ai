//go:build ignore
// 实验 3：Windows 侧视角的连接 churn（配合 Windows 侧脚本观察 127.0.0.1:45679）
// listener 保持监听，输出每个 accept 的时间戳；Windows 侧连续 connect 观察失败/延迟
package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

func main() {
	port := 45679
	if len(os.Args) > 1 {
		port, _ = strconv.Atoi(os.Args[1])
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Println("listen FAILED:", err)
		os.Exit(1)
	}
	fmt.Println("listening on", ln.Addr())
	for {
		c, err := ln.Accept()
		if err != nil {
			fmt.Println("accept err:", err)
			continue
		}
		fmt.Printf("accept %s %s\n", c.RemoteAddr(), time.Now().Format("15:04:05.000"))
		c.Close()
	}
}
