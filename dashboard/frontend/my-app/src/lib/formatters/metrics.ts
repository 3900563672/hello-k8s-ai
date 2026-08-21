import type { MetricPoint, MetricResult } from '@/types/trace.types'

export interface MetricStats {
    min: number
    max: number
    avg: number
    p95: number
    latest: number
}

/** 按时间戳聚合多 series 指标点；aggregation='average' 时取同刻均值，否则求和。 */
export function aggregateMetricPoints(
    metric: MetricResult,
    aggregation: 'sum' | 'average' = 'average',
): MetricPoint[] {
    const buckets = new Map<number, { total: number; count: number }>()
    metric.series.forEach((series) => {
        series.points.forEach((point) => {
            const timestamp = new Date(point.time).getTime()
            if (!Number.isFinite(timestamp) || !Number.isFinite(point.value)) return
            const current = buckets.get(timestamp) ?? { total: 0, count: 0 }
            current.total += point.value
            current.count += 1
            buckets.set(timestamp, current)
        })
    })
    return [...buckets.entries()]
        .sort(([left], [right]) => left - right)
        .map(([timestamp, bucket]) => ({
            time: new Date(timestamp).toISOString(),
            value: aggregation === 'average' ? bucket.total / bucket.count : bucket.total,
        }))
}

/** 全部采样点的 min / avg / max / p95 与最新值。 */
export function metricStats(metric: MetricResult): MetricStats | null {
    const values = metric.series.flatMap((series) =>
        series.points.map((point) => point.value).filter(Number.isFinite),
    )
    if (values.length === 0) return null
    values.sort((left, right) => left - right)
    const sum = values.reduce((total, value) => total + value, 0)
    return {
        min: values[0],
        max: values[values.length - 1],
        avg: sum / values.length,
        p95: values[Math.min(values.length - 1, Math.floor(values.length * 0.95))],
        latest: values[values.length - 1],
    }
}