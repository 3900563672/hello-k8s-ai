import { useEffect, useMemo, useRef, useState } from 'react'
import * as echarts from 'echarts/core'
import { BarChart, type BarSeriesOption } from 'echarts/charts'
import {
    AriaComponent,
    AxisPointerComponent,
    DataZoomComponent,
    GridComponent,
    MarkLineComponent,
    TooltipComponent,
    type AriaComponentOption,
    type AxisPointerComponentOption,
    type DataZoomComponentOption,
    type GridComponentOption,
    type MarkLineComponentOption,
    type TooltipComponentOption,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { ComposeOption } from 'echarts/core'
import { useTimeStore } from '@/stores/timeSlice'
import {
    aggregateSnapshots,
    chooseGranularity,
    escapeHtml,
    formatAxisUtc,
    formatUtc,
    getTimelineBounds,
    triggerLabel,
    viewportEquals,
    type TimelineBucket,
    type TimelineViewport,
} from './timelineMath'

type TimelineEChartsOption = ComposeOption<
    | AriaComponentOption
    | AxisPointerComponentOption
    | BarSeriesOption
    | DataZoomComponentOption
    | GridComponentOption
    | MarkLineComponentOption
    | TooltipComponentOption
>

echarts.use([
    AriaComponent,
    AxisPointerComponent,
    BarChart,
    CanvasRenderer,
    DataZoomComponent,
    GridComponent,
    MarkLineComponent,
    TooltipComponent,
])

interface TimelineChartProps {
    variant: 'mini' | 'explorer'
    className?: string
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
    typeof value === 'object' && value !== null

const numberValue = (value: unknown): number | null => {
    const parsed = typeof value === 'number' ? value : Number(value)
    return Number.isFinite(parsed) ? parsed : null
}

const zoomViewportFromEvent = (
    event: unknown,
    bounds: TimelineViewport,
): TimelineViewport | null => {
    if (!isRecord(event)) return null
    const batch = Array.isArray(event.batch) ? event.batch[0] : null
    const payload = isRecord(batch) ? batch : event
    const startValue = numberValue(payload.startValue)
    const endValue = numberValue(payload.endValue)
    if (startValue !== null && endValue !== null) {
        return { start: startValue, end: endValue }
    }

    const startPercent = numberValue(payload.start)
    const endPercent = numberValue(payload.end)
    if (startPercent === null || endPercent === null) return null
    const span = bounds.end - bounds.start
    return {
        start: bounds.start + (span * startPercent) / 100,
        end: bounds.start + (span * endPercent) / 100,
    }
}

const eventName = (params: unknown): string | null => {
    if (!isRecord(params)) return null
    return typeof params.name === 'string' ? params.name : null
}

const eventOffset = (
    event: unknown,
): { offsetX: number; offsetY: number } | null => {
    if (!isRecord(event)) return null
    const offsetX = numberValue(event.offsetX)
    const offsetY = numberValue(event.offsetY)
    return offsetX !== null && offsetY !== null ? { offsetX, offsetY } : null
}

export function TimelineChart({ variant, className = '' }: TimelineChartProps) {
    const containerRef = useRef<HTMLDivElement>(null)
    const chartRef = useRef<echarts.ECharts | null>(null)
    const applyingOptionRef = useRef(false)
    const bucketMapRef = useRef<Map<string, TimelineBucket>>(new Map())
    const viewportRef = useRef<TimelineViewport>({ start: 0, end: 1 })
    const boundsRef = useRef<TimelineViewport>({ start: 0, end: 1 })
    const [width, setWidth] = useState(900)

    const snapshots = useTimeStore((state) => state.snapshots)
    const timestamp = useTimeStore((state) => state.timestamp)
    const viewport = useTimeStore((state) => state.viewport)
    const setViewport = useTimeStore((state) => state.setViewport)
    const jumpToTimestamp = useTimeStore((state) => state.jumpToTimestamp)

    const bounds = useMemo(() => getTimelineBounds(snapshots), [snapshots])
    const visibleSpan = Math.max(1_000, viewport.end - viewport.start)
    const granularity = useMemo(
        () => chooseGranularity(visibleSpan, width),
        [visibleSpan, width],
    )
    const buckets = useMemo(
        () => aggregateSnapshots(snapshots, bounds, granularity.bucketMs),
        [snapshots, bounds, granularity.bucketMs],
    )

    useEffect(() => {
        bucketMapRef.current = new Map(buckets.map((bucket) => [bucket.key, bucket]))
        viewportRef.current = viewport
        boundsRef.current = bounds
    }, [buckets, viewport, bounds])

    useEffect(() => {
        const container = containerRef.current
        if (!container) return
        const chart = echarts.init(container, undefined, { renderer: 'canvas' })
        chartRef.current = chart

        const resizeObserver = new ResizeObserver((entries) => {
            const nextWidth = Math.round(entries[0]?.contentRect.width ?? 0)
            if (nextWidth > 0) setWidth(nextWidth)
            chart.resize()
        })
        resizeObserver.observe(container)

        const handleDataZoom = (event: unknown) => {
            if (applyingOptionRef.current) return
            const next = zoomViewportFromEvent(event, boundsRef.current)
            if (!next || viewportEquals(next, viewportRef.current, 10)) return
            setViewport(next)
        }

        const handleCanvasClick = (event: unknown) => {
            const offset = eventOffset(event)
            if (!offset) return
            if (
                !chart.containPixel(
                    { gridIndex: 0 },
                    [offset.offsetX, offset.offsetY],
                )
            ) {
                return
            }
            const converted = chart.convertFromPixel(
                { xAxisIndex: 0 },
                [offset.offsetX, offset.offsetY],
            )
            const target = Array.isArray(converted)
                ? numberValue(converted[0])
                : numberValue(converted)
            if (target !== null) jumpToTimestamp(target)
        }

        chart.on('datazoom', handleDataZoom)
        chart.getZr().on('click', handleCanvasClick)

        return () => {
            resizeObserver.disconnect()
            chart.off('datazoom', handleDataZoom)
            chart.getZr().off('click', handleCanvasClick)
            chart.dispose()
            chartRef.current = null
        }
    }, [jumpToTimestamp, setViewport])

    useEffect(() => {
        const chart = chartRef.current
        if (!chart) return

        const bucketMap = bucketMapRef.current
        const showAxis = variant === 'explorer'
        const timeData = buckets.map((bucket) => ({
            name: bucket.key,
            value: [bucket.timestamp, bucket.timeScore],
        }))
        const eventData = buckets.map((bucket) => ({
            name: bucket.key,
            value: [bucket.timestamp, bucket.eventScore],
        }))

        const tooltipFormatter = (params: unknown): string => {
            const key = eventName(params)
            const bucket = key ? bucketMap.get(key) : null
            if (!bucket) return ''
            const range =
                bucket.count === 1
                    ? formatUtc(bucket.start, true)
                    : formatUtc(bucket.start) + ' — ' + formatUtc(bucket.end)
            return (
                '<div style="min-width:210px;padding:2px 1px">' +
                '<div style="font:600 12px system-ui;color:#F3F6FA">' +
                escapeHtml(range) +
                ' UTC</div>' +
                '<div style="margin-top:8px;display:flex;gap:12px;font:11px system-ui;color:#9AA6B6">' +
                '<span><i style="display:inline-block;width:7px;height:7px;border-radius:2px;background:#6E8BFF;margin-right:5px"></i>' +
                triggerLabel('time') +
                ' ' +
                bucket.timeCount +
                '</span>' +
                '<span><i style="display:inline-block;width:7px;height:7px;border-radius:2px;background:#F0A33B;margin-right:5px"></i>' +
                triggerLabel('event') +
                ' ' +
                bucket.eventCount +
                '</span>' +
                '</div>' +
                '<div style="margin-top:7px;font:11px system-ui;color:#667286">共 ' +
                bucket.count +
                ' 个切面 · 峰值权重 ' +
                bucket.peakWeight +
                ' · 点击按该时刻回放</div>' +
                '</div>'
            )
        }

        const option: TimelineEChartsOption = {
            animation: true,
            animationDurationUpdate: 180,
            animationEasingUpdate: 'cubicOut',
            backgroundColor: 'transparent',
            aria: {
                enabled: true,
                label: {
                    description:
                        '时间切面分布图。蓝色为时间驱动切面，琥珀色为事件驱动切面。',
                },
            },
            grid: showAxis
                ? { left: 18, right: 18, top: 22, bottom: 68, containLabel: true }
                : { left: 1, right: 1, top: 3, bottom: 3, containLabel: false },
            xAxis: {
                type: 'time',
                min: bounds.start,
                max: bounds.end,
                boundaryGap: [0, 0],
                axisLine: {
                    show: showAxis,
                    lineStyle: { color: '#273142' },
                },
                axisTick: { show: false },
                axisLabel: {
                    show: showAxis,
                    color: '#687487',
                    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                    fontSize: 10,
                    margin: 14,
                    hideOverlap: true,
                    formatter: (value: number) => formatAxisUtc(value, visibleSpan),
                },
                splitLine: {
                    show: showAxis,
                    lineStyle: { color: 'rgba(255,255,255,0.035)', type: 'dashed' },
                },
                axisPointer: {
                    show: showAxis,
                    snap: false,
                    lineStyle: { color: '#7E8DA3', width: 1, type: 'dashed' },
                    label: {
                        show: showAxis,
                        color: '#DCE4EE',
                        backgroundColor: '#1B2431',
                        formatter: (params) =>
                            formatUtc(numberValue(params.value) ?? bounds.start) +
                            ' UTC',
                    },
                },
            },
            yAxis: {
                type: 'value',
                min: 0,
                show: false,
                scale: true,
            },
            tooltip: {
                trigger: 'item',
                confine: true,
                appendToBody: variant === 'explorer',
                backgroundColor: 'rgba(12, 16, 23, 0.96)',
                borderColor: 'rgba(255,255,255,0.11)',
                borderWidth: 1,
                padding: [9, 11],
                textStyle: { color: '#EDF2F8', fontSize: 11 },
                extraCssText:
                    'border-radius:10px;box-shadow:0 16px 44px rgba(0,0,0,.44);backdrop-filter:blur(12px)',
                formatter: tooltipFormatter,
            },
            dataZoom: [
                {
                    id: 'timeline-inside',
                    type: 'inside',
                    startValue: viewport.start,
                    endValue: viewport.end,
                    filterMode: 'none',
                    minValueSpan: 1_000,
                    zoomOnMouseWheel: true,
                    moveOnMouseMove: true,
                    moveOnMouseWheel: false,
                    preventDefaultMouseMove: true,
                    throttle: 50,
                },
                ...(showAxis
                    ? [
                          {
                              id: 'timeline-slider',
                              type: 'slider' as const,
                              startValue: viewport.start,
                              endValue: viewport.end,
                              filterMode: 'none' as const,
                              minValueSpan: 1_000,
                              height: 22,
                              bottom: 13,
                              borderColor: 'rgba(255,255,255,0.07)',
                              backgroundColor: '#080B10',
                              fillerColor: 'rgba(110,139,255,0.13)',
                              dataBackground: {
                                  lineStyle: { color: '#343F51', opacity: 0.5 },
                                  areaStyle: { color: '#111722', opacity: 0.5 },
                              },
                              selectedDataBackground: {
                                  lineStyle: { color: '#6E8BFF', opacity: 0.75 },
                                  areaStyle: { color: '#6E8BFF', opacity: 0.14 },
                              },
                              handleSize: 14,
                              handleStyle: {
                                  color: '#AFC0FF',
                                  borderColor: '#111826',
                                  borderWidth: 2,
                              },
                              moveHandleSize: 4,
                              moveHandleStyle: { color: '#52627A' },
                              textStyle: {
                                  color: '#657184',
                                  fontSize: 9,
                              },
                              labelFormatter: (value: number) =>
                                  formatAxisUtc(value, visibleSpan),
                              brushSelect: false,
                          },
                      ]
                    : []),
            ],
            series: [
                {
                    name: '时间驱动',
                    type: 'bar',
                    stack: 'activity',
                    data: timeData,
                    barMaxWidth: showAxis ? 8 : 5,
                    barMinHeight: 2,
                    large: buckets.length > 500,
                    largeThreshold: 500,
                    itemStyle: {
                        color: '#6E8BFF',
                        borderRadius: [3, 3, 0, 0],
                        opacity: 0.72,
                    },
                    emphasis: {
                        disabled: buckets.length > 500,
                        itemStyle: {
                            opacity: 1,
                            shadowBlur: 14,
                            shadowColor: 'rgba(110,139,255,0.42)',
                        },
                    },
                    markLine: {
                        silent: true,
                        animation: false,
                        symbol: 'none',
                        lineStyle: {
                            color: '#E9EEF6',
                            width: showAxis ? 1.5 : 1,
                            opacity: 0.92,
                        },
                        label: { show: false },
                        data: [{ xAxis: Date.parse(timestamp) }],
                    },
                },
                {
                    name: '事件驱动',
                    type: 'bar',
                    stack: 'activity',
                    data: eventData,
                    barMaxWidth: showAxis ? 8 : 5,
                    barMinHeight: 2,
                    large: buckets.length > 500,
                    largeThreshold: 500,
                    itemStyle: {
                        color: '#F0A33B',
                        borderRadius: [3, 3, 0, 0],
                        opacity: 0.86,
                    },
                    emphasis: {
                        disabled: buckets.length > 500,
                        itemStyle: {
                            opacity: 1,
                            shadowBlur: 14,
                            shadowColor: 'rgba(240,163,59,0.42)',
                        },
                    },
                },
            ],
        }

        applyingOptionRef.current = true
        chart.setOption(option, { notMerge: true, lazyUpdate: true })
        const frame = requestAnimationFrame(() => {
            applyingOptionRef.current = false
        })
        return () => cancelAnimationFrame(frame)
    }, [buckets, bounds, timestamp, variant, viewport, visibleSpan])

    return (
        <div
            className={
                'relative h-full w-full overflow-hidden ' + className
            }
        >
            <div
                ref={containerRef}
                className={
                    'h-full w-full cursor-crosshair select-none touch-none transition-opacity ' +
                    (snapshots.length === 0 ? 'opacity-0' : 'opacity-100')
                }
                role="img"
                aria-label="时间切面分布；滚轮缩放，拖动平移，点击选择回放时间"
            />
            {snapshots.length === 0 && (
                <div className="absolute inset-0 flex items-center justify-center rounded-lg border border-dashed border-white/[0.08] bg-white/[0.015] text-[11px] text-[#596579]">
                    当前时间范围内没有可回放切面
                </div>
            )}
        </div>
    )
}
