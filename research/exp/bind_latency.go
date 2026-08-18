//go:build ignore
// 实验 1：bind() 系统调用耗时（GNS 同步注册链延迟度量）
// 用法：go run bind_latency.go [seq|churn] [rounds]
package main

import (
	"fmt"
	"net"
	"os"
	"sort"
	"sync"
	"time"
)

func main() {
	mode := "seq"
	rounds := 200
	bindAddr := "127.0.0.1:0"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &rounds)
	}
	if len(os.Args) > 3 {
		bindAddr = os.Args[3]
	}
	var dur []time.Duration
	var mu sync.Mutex
	run := func() {
		for i := 0; i < rounds; i++ {
			start := time.Now()
			ln, err := net.Listen("tcp", bindAddr)
			el := time.Since(start)
			if err != nil {
				fmt.Println("listen err:", err)
				continue
			}
			ln.Close()
			mu.Lock()
			dur = append(dur, el)
			mu.Unlock()
		}
	}
	if mode == "churn" {
		var wg sync.WaitGroup
		for g := 0; g < 8; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				run()
			}()
		}
		wg.Wait()
	} else {
		run()
	}
	sort.Slice(dur, func(i, j int) bool { return dur[i] < dur[j] })
	n := len(dur)
	if n == 0 {
		fmt.Println("no samples")
		return
	}
	sum := time.Duration(0)
	for _, d := range dur {
		sum += d
	}
	p := func(q float64) time.Duration { return dur[int(q*float64(n-1))] }
	slow := 0
	verySlow := 0
	for _, d := range dur {
		if d > 100*time.Millisecond {
			slow++
		}
		if d > 1*time.Second {
			verySlow++
		}
	}
	fmt.Printf("mode=%s rounds=%d\nmin=%v p50=%v p95=%v p99=%v max=%v avg=%v\n>100ms=%d >1s=%d\n",
		mode, n, dur[0], p(0.5), p(0.95), p(0.99), dur[n-1], sum/time.Duration(n), slow, verySlow)
}
