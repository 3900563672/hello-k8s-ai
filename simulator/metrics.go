package main

import "github.com/prometheus/client_golang/prometheus"

const (
	simulatorMetricNamespace = "hello_k8s_ai"
	simulatorMetricSubsystem = "simulator"
)

// simulatorMetrics 汇总了模拟器自身暴露的所有 Prometheus 指标
type simulatorMetrics struct {
	leader                  prometheus.Gauge       // 当前 Pod 是否持有 reporter 租约
	leadershipChanges       *prometheus.CounterVec // leader 变更事件计数 (acquired/lost/observed)
	ticks                   *prometheus.CounterVec // 每轮模拟 tick 的结果计数 (success/error)
	tickDuration            prometheus.Histogram   // 一轮模拟的耗时分布
	statusUpdates           *prometheus.CounterVec // 写入 SimulatorInstance 状态的次数，按结果分
	assignedQPS             prometheus.Gauge       // 本实例当前被分配到的 QPS
	availableReplicas       prometheus.Gauge       // 本实例的可用副本数
	effectiveScore          prometheus.Gauge       // Orchestrator 给的资源折扣后分数
	poolScore               prometheus.Gauge       // 模拟器实时算出的运行时分数
	coldStartFactor         prometheus.Gauge       // 当前冷启动衰减因子 [0,1]
	timeScale                prometheus.Gauge       // 当前模拟时间倍速
	simulationStepSeconds    prometheus.Gauge       // 本轮推进的模拟秒数
	simulationElapsedSeconds prometheus.Gauge       // 当前 reporter 任期内累计推进的模拟秒数
	queueDepth              prometheus.Gauge       // 模拟队列深度
	ttftSeconds             prometheus.Gauge       // 最近一次平均 TTFT（秒）
	engineReinitializations prometheus.Counter     // 模拟引擎因并发变化而重建的次数
}

// newSimulatorMetrics 创建并注册所有模拟器指标到给定的注册器
func newSimulatorMetrics(registerer prometheus.Registerer) *simulatorMetrics {
	metrics := &simulatorMetrics{
		leader: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: simulatorMetricNamespace,
			Subsystem: simulatorMetricSubsystem,
			Name:      "leader",
			Help:      "Whether this simulator Pod currently owns the reporter Lease.",
		}),
		leadershipChanges: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: simulatorMetricNamespace,
			Subsystem: simulatorMetricSubsystem,
			Name:      "leadership_changes_total",
			Help:      "Number of reporter Lease leadership lifecycle events.",
		}, []string{"event"}),
		ticks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: simulatorMetricNamespace,
			Subsystem: simulatorMetricSubsystem,
			Name:      "ticks_total",
			Help:      "Number of leader simulation ticks by outcome.",
		}, []string{"outcome"}),
		tickDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: simulatorMetricNamespace,
			Subsystem: simulatorMetricSubsystem,
			Name:      "tick_duration_seconds",
			Help:      "Time spent calculating and publishing one simulation tick.",
			Buckets:   prometheus.DefBuckets,
		}),
		statusUpdates: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: simulatorMetricNamespace,
			Subsystem: simulatorMetricSubsystem,
			Name:      "status_updates_total",
			Help:      "Number of SimulatorInstance status update attempts by outcome.",
		}, []string{"outcome"}),
		assignedQPS: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: simulatorMetricNamespace,
			Subsystem: simulatorMetricSubsystem,
			Name:      "assigned_qps",
			Help:      "QPS currently assigned to this SimulatorInstance pool.",
		}),
		availableReplicas: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: simulatorMetricNamespace,
			Subsystem: simulatorMetricSubsystem,
			Name:      "available_replicas",
			Help:      "Available replicas currently observed for this SimulatorInstance.",
		}),
		effectiveScore: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: simulatorMetricNamespace,
			Subsystem: simulatorMetricSubsystem,
			Name:      "effective_score",
			Help:      "Resource-discounted score assigned by the orchestrator.",
		}),
		poolScore: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: simulatorMetricNamespace,
			Subsystem: simulatorMetricSubsystem,
			Name:      "pool_score",
			Help:      "Runtime score for the available replica pool after warmup.",
		}),
		coldStartFactor: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: simulatorMetricNamespace,
			Subsystem: simulatorMetricSubsystem,
			Name:      "cold_start_factor",
			Help:      "Current cold-start capacity factor in the range zero to one.",
		}),
		timeScale: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: simulatorMetricNamespace,
			Subsystem: simulatorMetricSubsystem,
			Name:      "time_scale",
			Help:      "当前 Simulator 离散事件引擎使用的时间倍速。",
		}),
		simulationStepSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: simulatorMetricNamespace,
			Subsystem: simulatorMetricSubsystem,
			Name:      "simulation_step_seconds",
			Help:      "最近一个真实 tick 推进的模拟秒数。",
		}),
		simulationElapsedSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: simulatorMetricNamespace,
			Subsystem: simulatorMetricSubsystem,
			Name:      "simulation_elapsed_seconds",
			Help:      "当前 reporter 任期内累计推进的模拟秒数。",
		}),
		queueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: simulatorMetricNamespace,
			Subsystem: simulatorMetricSubsystem,
			Name:      "queue_depth",
			Help:      "Latest simulated request queue depth.",
		}),
		ttftSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: simulatorMetricNamespace,
			Subsystem: simulatorMetricSubsystem,
			Name:      "ttft_seconds",
			Help:      "Latest simulated average time to first token in seconds.",
		}),
		engineReinitializations: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: simulatorMetricNamespace,
			Subsystem: simulatorMetricSubsystem,
			Name:      "engine_reinitializations_total",
			Help:      "Number of simulation engine initializations after concurrency changes.",
		}),
	}
	registerer.MustRegister(
		metrics.leader,
		metrics.leadershipChanges,
		metrics.ticks,
		metrics.tickDuration,
		metrics.statusUpdates,
		metrics.assignedQPS,
		metrics.availableReplicas,
		metrics.effectiveScore,
		metrics.poolScore,
		metrics.coldStartFactor,
		metrics.timeScale,
		metrics.simulationStepSeconds,
		metrics.simulationElapsedSeconds,
		metrics.queueDepth,
		metrics.ttftSeconds,
		metrics.engineReinitializations,
	)
	return metrics
}

// setLeader 更新 leader gauge 并记录事件计数
func (metrics *simulatorMetrics) setLeader(leader bool) {
	if metrics == nil {
		return
	}
	if leader {
		metrics.leader.Set(1)
		metrics.leadershipChanges.WithLabelValues("acquired").Inc()
		return
	}
	metrics.leader.Set(0)
	metrics.leadershipChanges.WithLabelValues("lost").Inc()
}
