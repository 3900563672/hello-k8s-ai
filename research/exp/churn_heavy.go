//go:build ignore
// 实验 4：极端 churn——更高并发下 bind 积压与秒级停滞复现
// 用法：go run churn_heavy.go <并发数> <每并发轮数>
package main

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"
)

func main() {
	conc := 64
	rounds := 200
	if len(os.Args) > 1 {
		conc, _ = strconv.Atoi(os.Args[1])
	}
	if len(os.Args) > 2 {
		rounds, _ = strconv.Atoi(os.Args[2])
	}
	var mu sync.Mutex
	var dur []time.Duration
	stalls := 0
	start := time.Now()
	var wg sync.WaitGroup
	for g := 0; g < conc; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				s := time.Now()
				ln, err := net.Listen("tcp", "127.0.0.1:0")
				el := time.Since(s)
				if err != nil {
					fmt.Printf("listen err: %v\n", err)
					continue
				}
				ln.Close()
				mu.Lock()
				dur = append(dur, el)
				if el > time.Second {
					stalls++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)
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
	fmt.Printf("conc=%d rounds=%d total=%d elapsed=%v throughput=%.0f bind/s\n", conc, rounds, n, elapsed, float64(n)/elapsed.Seconds())
	fmt.Printf("min=%v p50=%v p95=%v p99=%v max=%v avg=%v\n", dur[0], p(0.5), p(0.95), p(0.99), dur[n-1], sum/time.Duration(n))
	fmt.Printf("stalls(>1s)=%d (%.2f%%)\n", stalls, float64(stalls)*100/float64(n))
}
