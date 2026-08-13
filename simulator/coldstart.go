package main

import "time"

// coldStartFactorAt 是测试用的确定性版本。
// 在冷启动的前一半时间里因子为 0，后一半用二次曲线从 0 过渡到 1。
// 如果 clock skew 导致因子超出 [0,1] 范围，会被钳制。
func coldStartFactorAt(now, startTime time.Time, coldStartMs int64) float64 {
	if coldStartMs <= 0 {
		return 1
	}

	elapsed := now.Sub(startTime).Milliseconds()
	// 尚未开始或已超过冷启动时间
	if elapsed <= 0 {
		return 0
	}
	if elapsed >= coldStartMs {
		return 1
	}

	half := coldStartMs / 2
	// 前一半时间完全无效
	if elapsed < half {
		return 0
	}

	// 后一半用二次曲线平滑上升，公式: 4 * (t/T - 0.5)^2
	ratio := float64(elapsed)/float64(coldStartMs) - 0.5
	factor := 4 * ratio * ratio

	// 防御性钳制，防止浮点误差或奇怪的时间输入
	if factor < 0 {
		return 0
	}
	if factor > 1 {
		return 1
	}
	return factor
}
