package main

import (
	"cmp"
	"math"
	"math/rand"
	"slices"
	"time"
)

// 为避免内存爆炸，单步最多实例化这么多请求，超出的用虚拟队列计数
const maxMaterializedRequestsPerStep = 100000

type Request struct {
	arriveTime       time.Duration // 到达时间
	serviceTime      time.Duration // 总处理时间（含噪声）
	startTime        time.Duration // 开始被服务的时间
	prefillMs        float64       // prefill 基础延迟（毫秒）
	decodePerTokenMs float64       // 每 token decode 延迟（毫秒）
	firstTokenAt     time.Duration // 首 token 产生的时间点
	ttftRecorded     bool          // 是否已经计过 TTFT
}

type Server struct {
	busyUntil      time.Duration // 忙到什么时候
	currentRequest *Request
}

// SimEngine 离散事件模拟器，用虚拟队列防止高 QPS 时内存爆掉。
type SimEngine struct {
	queue          []*Request
	servers        []*Server
	maxConcurrency int
	rng            *rand.Rand
	currentTime    time.Duration
	completedTTFT  time.Duration
	completedCount int

	// 虚拟队列：超出实例化上限的请求只记数量和到达时间，不分配内存
	virtualQueue        int
	virtualArriveTime   time.Duration
	virtualRequestShape Request
}

func NewSimEngine(maxConcurrency int) *SimEngine {
	return newSimEngine(maxConcurrency, rand.New(rand.NewSource(time.Now().UnixNano())))
}

func newSimEngine(maxConcurrency int, rng *rand.Rand) *SimEngine {
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}
	servers := make([]*Server, maxConcurrency)
	for i := range servers {
		servers[i] = &Server{}
	}
	return &SimEngine{servers: servers, maxConcurrency: maxConcurrency, rng: rng}
}

// Step 是公开的整数 QPS 入口，实际转发到 StepRate。
func (e *SimEngine) Step(
	duration time.Duration,
	qps int,
	score, effectiveScore int,
	prefillBaseMs, prefillPerTokenUs, decodePerTokenMs int,
	coldStartFactor float64,
) (avgTTFT int, queueLen int, hasCompleted bool) {
	return e.StepRate(
		duration,
		float64(qps),
		score,
		effectiveScore,
		prefillBaseMs,
		prefillPerTokenUs,
		decodePerTokenMs,
		coldStartFactor,
	)
}

// StepRate 驱动模拟前进，支持浮点 QPS（用于分片等场景）。
// 服务器在同一个 step 内可以连续处理多个请求。
func (e *SimEngine) StepRate(
	duration time.Duration,
	qps float64,
	score, effectiveScore int,
	prefillBaseMs, prefillPerTokenUs, decodePerTokenMs int,
	coldStartFactor float64,
) (avgTTFT int, queueLen int, hasCompleted bool) {
	if qps <= 0 {
		e.reset()
		return 0, 0, false
	}
	if duration <= 0 {
		return 0, e.queueDepth(), false
	}

	// 按泊松分布生成新请求并混入噪声
	e.generateRequestsRate(duration, qps, prefillBaseMs, prefillPerTokenUs, decodePerTokenMs)

	// 冷启动因子为 0 时请求不完成，相当于无限延迟
	factor := coldStartFactor
	if score <= 0 || effectiveScore <= 0 || factor <= 0 {
		factor = 0
	} else if factor > 1 {
		factor = 1
	}

	e.advanceTo(e.currentTime+duration, factor)

	queueLen = e.queueDepth()
	if e.completedCount == 0 {
		return 0, queueLen, false
	}
	avgTTFT = int(e.completedTTFT.Milliseconds()) / e.completedCount
	e.completedTTFT = 0
	e.completedCount = 0
	return avgTTFT, queueLen, true
}

// generateRequestsRate 生成一批请求，超出上限的进虚拟队列。
func (e *SimEngine) generateRequestsRate(
	duration time.Duration,
	qps float64,
	prefillBaseMs, prefillPerTokenUs, decodePerTokenMs int,
) {
	lambda := qps * duration.Seconds()
	if lambda <= 0 || math.IsNaN(lambda) {
		return
	}
	numNew := poisson(e.rng, lambda)
	if numNew <= 0 {
		return
	}

	const (
		promptTokens = 500
		outputTokens = 200
	)
	prefill := float64(max(0, prefillBaseMs)) + float64(max(0, prefillPerTokenUs))*promptTokens/1000
	decodePerToken := float64(max(0, decodePerTokenMs))
	baseServiceMs := prefill + decodePerToken*outputTokens
	if baseServiceMs < 1 {
		baseServiceMs = 1
	}

	// 请求的"模板"，后面复制并加噪声
	shape := Request{
		serviceTime:      millisecondsDuration(baseServiceMs),
		prefillMs:        prefill,
		decodePerTokenMs: decodePerToken,
	}

	// 超出实例化上限的部分计入虚拟队列，避免内存无限增长
	materialized := numNew
	availableSlots := max(0, maxMaterializedRequestsPerStep-len(e.queue))
	if materialized > availableSlots {
		materialized = availableSlots
		overflow := numNew - materialized
		if e.virtualQueue == 0 {
			e.virtualArriveTime = e.currentTime
			e.virtualRequestShape = shape
		}
		e.virtualQueue = saturatingAdd(e.virtualQueue, overflow)
	}

	// 逐个实例化，到达时间在 duration 内均匀分布
	for i := 0; i < materialized; i++ {
		offset := time.Duration(e.rng.Float64() * float64(duration))
		request := shape
		request.arriveTime = e.currentTime + offset
		noise := 1 + (e.rng.Float64()-0.5)*0.4 // [0.8, 1.2]
		request.serviceTime = scaleDuration(shape.serviceTime, noise)
		e.queue = append(e.queue, &request)
	}

	// 按到达时间排序，保证 FIFO 顺序
	slices.SortStableFunc(e.queue, func(left, right *Request) int {
		return cmp.Compare(left.arriveTime, right.arriveTime)
	})
}

// advanceTo 以事件驱动方式推进时间到 end，处理到达、完成和首 token 事件。
func (e *SimEngine) advanceTo(end time.Duration, capacityFactor float64) {
	now := e.currentTime
	for {
		// 先记录当前时刻之前应该产生的首 token
		e.recordFirstTokens(now)
		// 释放已完成服务的请求
		e.releaseCompleted(now)

		if capacityFactor > 0 {
			// 分配新请求并立即记录首 token（分配时延迟可能为 0）
			e.assignAvailable(now, capacityFactor)
			e.recordFirstTokens(now)
		}

		if now >= end {
			break
		}

		// 下一个需要关注的时间点：下一次到达、最早的首 token 或最早的完成时间
		next := end
		if arrival, ok := e.nextArrivalAfter(now); ok && arrival < next {
			next = arrival
		}
		for _, server := range e.servers {
			request := server.currentRequest
			if request == nil {
				continue
			}
			if !request.ttftRecorded && request.firstTokenAt > now && request.firstTokenAt < next {
				next = request.firstTokenAt
			}
			if server.busyUntil > now && server.busyUntil < next {
				next = server.busyUntil
			}
		}
		if next <= now {
			next = end
		}
		now = next
	}
	e.currentTime = end
}

// recordFirstTokens 统计已到达但尚未记录的首 token 延迟。
func (e *SimEngine) recordFirstTokens(now time.Duration) {
	for _, server := range e.servers {
		request := server.currentRequest
		if request == nil || request.ttftRecorded || request.firstTokenAt > now {
			continue
		}
		ttft := max(time.Duration(0), request.firstTokenAt-request.arriveTime)
		e.completedTTFT += ttft
		e.completedCount++
		request.ttftRecorded = true
	}
}

func (e *SimEngine) releaseCompleted(now time.Duration) {
	for _, server := range e.servers {
		if server.currentRequest != nil && server.busyUntil <= now {
			server.currentRequest = nil
		}
	}
}

// assignAvailable 从队列（含虚拟队列）中取请求分配给空闲服务器。
func (e *SimEngine) assignAvailable(now time.Duration, factor float64) {
	for _, server := range e.servers {
		if server.currentRequest != nil {
			continue
		}
		request := e.popArrived(now)
		if request == nil {
			continue
		}
		request.startTime = now

		// 实际服务时间和首 token 时间受容量因子影响，因子越小耗时越长
		service := scaleDuration(request.serviceTime, 1/factor)
		firstTokenBase := millisecondsDuration(request.prefillMs + request.decodePerTokenMs)
		firstTokenDelay := scaleDuration(firstTokenBase, 1/factor)

		request.firstTokenAt = addDurationSaturated(now, firstTokenDelay)
		server.busyUntil = addDurationSaturated(now, service)
		server.currentRequest = request
	}
}

// popArrived 从实例化队列或虚拟队列中取出一个在当前时间之前到达的请求。
func (e *SimEngine) popArrived(now time.Duration) *Request {
	queueArrived := len(e.queue) > 0 && e.queue[0].arriveTime <= now
	virtualArrived := e.virtualQueue > 0 && e.virtualArriveTime <= now

	// 虚拟队列的请求到达时间更早就先取虚拟的
	if virtualArrived && (!queueArrived || e.virtualArriveTime <= e.queue[0].arriveTime) {
		request := e.virtualRequestShape
		request.arriveTime = e.virtualArriveTime
		e.virtualQueue--
		if e.virtualQueue == 0 {
			e.virtualArriveTime = 0
		}
		return &request
	}
	if !queueArrived {
		return nil
	}
	request := e.queue[0]
	e.queue[0] = nil
	e.queue = e.queue[1:]
	return request
}

// nextArrivalAfter 返回严格晚于 now 的下一次到达时间（含虚拟队列）。
func (e *SimEngine) nextArrivalAfter(now time.Duration) (time.Duration, bool) {
	next := time.Duration(0)
	found := false
	if len(e.queue) > 0 && e.queue[0].arriveTime > now {
		next = e.queue[0].arriveTime
		found = true
	}
	if e.virtualQueue > 0 && e.virtualArriveTime > now && (!found || e.virtualArriveTime < next) {
		next = e.virtualArriveTime
		found = true
	}
	return next, found
}

func (e *SimEngine) queueDepth() int {
	return saturatingAdd(len(e.queue), e.virtualQueue)
}

func (e *SimEngine) reset() {
	e.queue = nil
	e.virtualQueue = 0
	e.virtualArriveTime = 0
	e.completedTTFT = 0
	e.completedCount = 0
	for _, server := range e.servers {
		server.currentRequest = nil
		server.busyUntil = 0
	}
	e.currentTime = 0
}

// millisecondsDuration 将毫秒转为 Duration，防御 NaN 和溢出。
func millisecondsDuration(milliseconds float64) time.Duration {
	if milliseconds <= 0 || math.IsNaN(milliseconds) {
		return 0
	}
	value := milliseconds * float64(time.Millisecond)
	if value >= float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(value)
}

// scaleDuration 安全地缩放 Duration，因子非法或溢出时钳制。
func scaleDuration(duration time.Duration, factor float64) time.Duration {
	if duration <= 0 || factor <= 0 || math.IsNaN(factor) {
		return 0
	}
	value := float64(duration) * factor
	if value >= float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
	}
	if value < 1 {
		return 1
	}
	return time.Duration(value)
}

// addDurationSaturated 饱和加法，溢出时返回 MaxInt64。
func addDurationSaturated(left, right time.Duration) time.Duration {
	if right > 0 && left > time.Duration(math.MaxInt64)-right {
		return time.Duration(math.MaxInt64)
	}
	return left + right
}

// saturatingAdd 对 int 做饱和加法。
func saturatingAdd(left, right int) int {
	maxInt := int(^uint(0) >> 1)
	if right > 0 && left > maxInt-right {
		return maxInt
	}
	return left + right
}

// poisson 生成泊松分布随机数。
// 小 lambda (<30) 用 Knuth 乘法，大 lambda 用 Atkinson 变换拒绝法，避免溢出。
func poisson(rng *rand.Rand, lambda float64) int {
	if rng == nil || lambda <= 0 || math.IsNaN(lambda) {
		return 0
	}
	if math.IsInf(lambda, 1) || lambda >= float64(int(^uint(0)>>1)) {
		return int(^uint(0) >> 1)
	}

	// 小 lambda 用经典 Knuth 算法
	if lambda < 30 {
		limit := math.Exp(-lambda)
		product := 1.0
		count := 0
		for product > limit {
			count++
			product *= rng.Float64()
		}
		return count - 1
	}

	// 大 lambda 用 Atkinson 变换拒绝法
	c := 0.767 - 3.36/lambda
	beta := math.Pi / math.Sqrt(3*lambda)
	alpha := beta * lambda
	constant := math.Log(c) - lambda - math.Log(beta)

	for {
		u := rng.Float64()
		if u <= 0 || u >= 1 {
			continue
		}
		x := (alpha - math.Log((1-u)/u)) / beta
		n := math.Floor(x + 0.5)
		if n < 0 {
			continue
		}
		v := rng.Float64()
		if v <= 0 {
			continue
		}
		y := alpha - beta*x
		lhs := y + math.Log(v) - 2*logOnePlusExp(y)
		logFactorial, _ := math.Lgamma(n + 1)
		rhs := constant + n*math.Log(lambda) - logFactorial
		if lhs <= rhs {
			return int(n)
		}
	}
}

// logOnePlusExp 计算 log(1 + exp(x))，避免 exp 溢出。
func logOnePlusExp(value float64) float64 {
	if value > 0 {
		return value + math.Log1p(math.Exp(-value))
	}
	return math.Log1p(math.Exp(value))
}
