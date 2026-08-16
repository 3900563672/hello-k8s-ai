package main

import (
	"math"
	"testing"
	"time"
)

// TestColdStartFactorAt 验证冷启动因子的边界值和曲线形状
func TestColdStartFactorAt(t *testing.T) {
	start := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		now         time.Time
		coldStartMs int64
		want        float64
	}{
		{name: "disabled", now: start, coldStartMs: 0, want: 1},                                  // 0 表示无冷启动
		{name: "clock skew", now: start.Add(-time.Second), coldStartMs: 1000, want: 0},           // 时钟回拨直接返回0
		{name: "first half", now: start.Add(499 * time.Millisecond), coldStartMs: 1000, want: 0}, // 前一半时间为0
		{name: "ramp", now: start.Add(750 * time.Millisecond), coldStartMs: 1000, want: 0.25},    // 后一半时间二次上升
		{name: "complete", now: start.Add(time.Second), coldStartMs: 1000, want: 1},              // 完成时到1
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := coldStartFactorAt(test.now, start, test.coldStartMs); got != test.want {
				t.Fatalf("coldStartFactorAt() = %v, want %v", got, test.want)
			}
		})
	}
}

// TestScaledScoreBounds 分数缩放：正常情况、最小值、负数和溢出
func TestScaledScoreBounds(t *testing.T) {
	if got := scaledScore(100, 0.25); got != 25 {
		t.Fatalf("scaledScore(100, 0.25) = %d, want 25", got)
	}
	// 即使因子很低，只要分数>0，结果至少为1
	if got := scaledScore(1, 0.25); got != 1 {
		t.Fatalf("positive capacity rounded to %d, want minimum 1", got)
	}
	// 负分数按 0 处理。
	if got := scaledScore(-1, 1); got != 0 {
		t.Fatalf("negative score scaled to %d, want 0", got)
	}
	// 溢出保护
	if got := scaledScore(math.MaxInt, 2); got != math.MaxInt {
		t.Fatalf("overflow result = %d, want %d", got, math.MaxInt)
	}
}

// TestSaturatingMultiply 饱和乘法：正常、溢出、零值
func TestSaturatingMultiply(t *testing.T) {
	if got := saturatingMultiply(25, 4); got != 100 {
		t.Fatalf("saturatingMultiply(25, 4) = %d, want 100", got)
	}
	// 溢出时钳制到 MaxInt
	if got := saturatingMultiply(math.MaxInt, 2); got != math.MaxInt {
		t.Fatalf("overflow result = %d, want %d", got, math.MaxInt)
	}
	// 乘0得0
	if got := saturatingMultiply(10, 0); got != 0 {
		t.Fatalf("zero replica result = %d, want 0", got)
	}
}
