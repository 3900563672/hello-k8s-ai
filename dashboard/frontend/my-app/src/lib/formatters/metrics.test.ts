import { describe, expect, it } from 'vitest'
import { aggregateMetricPoints, metricStats } from '@/lib/formatters/metrics'
import type { MetricResult } from '@/types/trace.types'

const makeMetric = (series: MetricResult['series']): MetricResult => ({
    metricId: 'simulator.qps',
    unit: 'req/s',
    start: '2026-08-12T12:00:00.000Z',
    end: '2026-08-12T12:01:00.000Z',
    stepSeconds: 30,
    series,
    resultType: 'matrix',
    warnings: [],
    queriedAt: '2026-08-12T12:01:00.000Z',
})

describe('aggregateMetricPoints', () => {
    it('默认按平均聚合同时刻多 series 并按时序排序', () => {
        const metric = makeMetric([
            {
                labels: { instance: 'a' },
                points: [
                    { time: '2026-08-12T12:00:00.000Z', value: 10 },
                    { time: '2026-08-12T12:00:30.000Z', value: 20 },
                ],
            },
            {
                labels: { instance: 'b' },
                points: [
                    { time: '2026-08-12T12:00:00.000Z', value: 30 },
                    { time: '2026-08-12T12:00:30.000Z', value: 40 },
                ],
            },
        ])
        expect(aggregateMetricPoints(metric)).toEqual([
            { time: '2026-08-12T12:00:00.000Z', value: 20 },
            { time: '2026-08-12T12:00:30.000Z', value: 30 },
        ])
    })

    it('aggregation=sum 时同刻求和', () => {
        const metric = makeMetric([
            {
                labels: { instance: 'a' },
                points: [{ time: '2026-08-12T12:00:00.000Z', value: 10 }],
            },
            {
                labels: { instance: 'b' },
                points: [{ time: '2026-08-12T12:00:00.000Z', value: 30 }],
            },
        ])
        expect(aggregateMetricPoints(metric, 'sum')).toEqual([
            { time: '2026-08-12T12:00:00.000Z', value: 40 },
        ])
    })

    it('过滤非法时间戳与非法数值', () => {
        const metric = makeMetric([
            {
                labels: { instance: 'a' },
                points: [
                    { time: 'bad-time', value: 10 },
                    { time: '2026-08-12T12:00:00.000Z', value: Number.NaN },
                    { time: '2026-08-12T12:00:00.000Z', value: 5 },
                ],
            },
        ])
        expect(aggregateMetricPoints(metric)).toEqual([
            { time: '2026-08-12T12:00:00.000Z', value: 5 },
        ])
    })
})

describe('metricStats', () => {
    it('计算 min/avg/max/p95 与最新值', () => {
        const metric = makeMetric([
            { labels: {}, points: [
                { time: '2026-08-12T12:00:00.000Z', value: 10 },
                { time: '2026-08-12T12:00:10.000Z', value: 20 },
                { time: '2026-08-12T12:00:20.000Z', value: 30 },
                { time: '2026-08-12T12:00:30.000Z', value: 40 },
            ] },
        ])
        expect(metricStats(metric)).toEqual({ min: 10, max: 40, avg: 25, p95: 40, latest: 40 })
    })

    it('无有效采样时返回 null', () => {
        const metric = makeMetric([
            { labels: {}, points: [{ time: '2026-08-12T12:00:00.000Z', value: Number.NaN }] },
        ])
        expect(metricStats(metric)).toBeNull()
    })

    it('单点采样 p95 为该点', () => {
        const metric = makeMetric([
            { labels: {}, points: [{ time: '2026-08-12T12:00:00.000Z', value: 7 }] },
        ])
        expect(metricStats(metric)).toEqual({ min: 7, max: 7, avg: 7, p95: 7, latest: 7 })
    })
})
