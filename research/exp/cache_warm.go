//go:build ignore
// 实验（issue #72）：缓存预热对照——seccomp 通知链 vs Windows 排他锁瓶颈判定
// 用法：go run cache_warm.go
// 阶段 1（预热）：100 个固定端口（30000-30099）各 bind+close 一次，填充 m_allocatedPorts 缓存（60s TTL）
// 阶段 2（缓存命中）：128 并发重复 bind 这 100 个端口——期望 100% 缓存命中，不经 Windows 排他锁
// 阶段 3（未缓存对照）：128 并发 bind 128 个未预热显式端口（30100-30227）——走完整 Windows 路径
// 判定：阶段 2 总耗时 ≈ 128×10.4ms ≈ 1.33s → 瓶颈在 guest 侧 seccomp 通知链；明显低于阶段 3 → Windows 锁是主瓶颈
package main

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"
)

const (
	warmPortStart = 30000
	warmPorts     = 100
	conc          = 128
)

func listenAndClose(port int) (time.Duration, error) {
	s := time.Now()
	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	el := time.Since(s)
	if err != nil {
		return el, err
	}
	ln.Close()
	return el, nil
}

func runPhase(name string, ports func(i int) int, total int) {
	var mu sync.Mutex
	var dur []time.Duration
	var fails []int
	start := time.Now()
	var wg sync.WaitGroup
	for g := 0; g < conc; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := g; i < total; i += conc {
				p := ports(i)
				el, err := listenAndClose(p)
				mu.Lock()
				if err != nil {
					fails = append(fails, p)
				} else {
					dur = append(dur, el)
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)
	n := len(dur)
	fmt.Printf("[%s] attempts=%d ok=%d fail=%d elapsed=%v throughput=%.0f bind/s\n",
		name, total, n, len(fails), elapsed, float64(n)/elapsed.Seconds())
	if n == 0 {
		return
	}
	sort.Slice(dur, func(i, j int) bool { return dur[i] < dur[j] })
	sum := time.Duration(0)
	for _, d := range dur {
		sum += d
	}
	p := func(q float64) time.Duration { return dur[int(q*float64(n-1))] }
	fmt.Printf("[%s] min=%v p50=%v p95=%v p99=%v max=%v avg=%v\n",
		name, dur[0], p(0.5), p(0.95), p(0.99), dur[n-1], sum/time.Duration(n))
	if len(fails) > 0 && len(fails) <= 10 {
		fmt.Printf("[%s] failed ports: %v\n", name, fails)
	}
}

func main() {
	fmt.Printf("phase1 warm: %d ports x1 bind+close (sequential)\n", warmPorts)
	warmStart := time.Now()
	warmFail := 0
	for i := 0; i < warmPorts; i++ {
		el, err := listenAndClose(warmPortStart + i)
		if err != nil {
			warmFail++
			fmt.Printf("  warm fail port=%d err=%v\n", warmPortStart+i, err)
		} else if el > time.Second {
			fmt.Printf("  warm slow port=%d el=%v\n", warmPortStart+i, el)
		}
	}
	fmt.Printf("phase1 done: elapsed=%v fail=%d\n", time.Since(warmStart), warmFail)

	runPhase("phase2 cached(30000-30099)", func(i int) int { return warmPortStart + (i % warmPorts) }, conc*4)
	runPhase("phase3 uncached(30100-30227)", func(i int) int { return warmPortStart + warmPorts + i }, conc*4)
}
