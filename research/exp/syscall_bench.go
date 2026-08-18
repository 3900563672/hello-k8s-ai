//go:build ignore
// syscall 级基准：区分 WSL 调度固有开销 vs GNS 同步链延迟
// 对照：getpid / socket 创建+close / bind(127.0.0.1) / bind(127.0.0.2)
package main

import (
	"fmt"
	"sort"
	"syscall"
	"time"
)

func bench(name string, fn func()) {
	var d []time.Duration
	for i := 0; i < 300; i++ {
		s := time.Now()
		fn()
		d = append(d, time.Since(s))
	}
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	n := len(d)
	p := func(q float64) time.Duration { return d[int(q*float64(n-1))] }
	fmt.Printf("%s: min=%v p50=%v p95=%v max=%v\n", name, d[0], p(0.5), p(0.95), d[n-1])
}

func main() {
	bench("getpid", func() { syscall.Getpid() })
	bench("socket+close", func() {
		fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
		if err == nil {
			syscall.Close(fd)
		}
	})
	bench("bind 127.0.0.1:0", func() {
		fd, _ := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
		addr := &syscall.SockaddrInet4{Port: 0, Addr: [4]byte{127, 0, 0, 1}}
		syscall.Bind(fd, addr)
		syscall.Close(fd)
	})
	bench("bind 127.0.0.2:0", func() {
		fd, _ := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
		addr := &syscall.SockaddrInet4{Port: 0, Addr: [4]byte{127, 0, 0, 2}}
		syscall.Bind(fd, addr)
		syscall.Close(fd)
	})
}
