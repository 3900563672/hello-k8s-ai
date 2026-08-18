//go:build ignore
// 实验 2b：close 后不重 bind，观察 Windows listener 是否在 ~60s 后释放
package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

func main() {
	port := 45680
	hold := 70
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
		fmt.Println("bind FAILED:", err)
		os.Exit(1)
	}
	ts("B0_bind")
	time.Sleep(3 * time.Second)
	ln.Close()
	ts("B1_closed_no_rebind")
	time.Sleep(time.Duration(hold) * time.Second)
	ts("B2_watch_done")
}
