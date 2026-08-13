import { useMemo } from 'react'
import ReactECharts from 'echarts-for-react'
import type { EChartsOption } from 'echarts'
import { Activity, Layers3, MousePointer2 } from 'lucide-react'
import { useTenants } from '@/api/queries/trafficQueries'
import { useTrafficStore } from '@/stores/trafficSlice'
import type { OverlayInstance } from '@/types/traffic.types'
import {
    buildScenarioTimePoints,
    formatLogicalTime,
    formatQps,
    getScenarioHorizon,
    getTenantSeriesValues,
    getTotalSeriesValues,
    TRAFFIC_COLORS,
} from './trafficMath'

interface TrafficCanvasProps {
    className?: string
}

interface ChartState {
    option: EChartsOption
    emptyTitle?: string
    emptyDescription?: string
}

export function TrafficCanvas({ className = '' }: TrafficCanvasProps) {
    const { data: tenants = [] } = useTenants()
    const {
        viewMode,
        selectedTenant,
        compareTenants,
        templates,
        overlays,
    } = useTrafficStore()

    const chartState = useMemo<ChartState>(() => {
        const enabledOverlays = overlays.filter((overlay) =>
            overlay.enabled && templates.some((template) => template.id === overlay.templateId),
        )
        const horizon = getScenarioHorizon(templates, enabledOverlays)
        const timePoints = buildScenarioTimePoints(templates, enabledOverlays, horizon)
        const series: Array<Record<string, unknown>> = []
        let relevantOverlays: OverlayInstance[] = []
        let emptyTitle: string | undefined
        let emptyDescription: string | undefined

        if (viewMode === 'total') {
            const values = getTotalSeriesValues(
                tenants.map((tenant) => tenant.id),
                timePoints,
                templates,
                enabledOverlays,
            )
            relevantOverlays = enabledOverlays
            series.push(makeSeries('总 QPS', '#5B8CFF', timePoints, values, true))
            if (enabledOverlays.length === 0) {
                emptyTitle = '还没有布置流量'
                emptyDescription = '从左侧选择模板，配置租户和逻辑偏移后开始构建场景'
            }
        } else if (viewMode === 'single') {
            const tenant = tenants.find((item) => item.id === selectedTenant)
            if (!tenant) {
                emptyTitle = '请选择一个租户'
                emptyDescription = '选择后将显示该租户由所有模板相加得到的最终 QPS'
            } else {
                const values = getTenantSeriesValues(
                    tenant.id,
                    timePoints,
                    templates,
                    enabledOverlays,
                )
                relevantOverlays = enabledOverlays.filter((overlay) => overlay.tenantId === tenant.id)
                series.push(makeSeries(tenant.name, TRAFFIC_COLORS[0], timePoints, values, true))
                if (relevantOverlays.length === 0) {
                    emptyTitle = `${tenant.name} 尚无流量`
                    emptyDescription = '为该租户叠加一个模板后，曲线会立即出现'
                }
            }
        } else {
            const selected = tenants.filter((tenant) => compareTenants.includes(tenant.id))
            if (selected.length === 0) {
                emptyTitle = '请选择要对比的租户'
                emptyDescription = '不同租户保持独立计算，以不同颜色同时展示'
            } else {
                selected.forEach((tenant, index) => {
                    const values = getTenantSeriesValues(
                        tenant.id,
                        timePoints,
                        templates,
                        enabledOverlays,
                    )
                    series.push(makeSeries(
                        tenant.name,
                        TRAFFIC_COLORS[index % TRAFFIC_COLORS.length],
                        timePoints,
                        values,
                        false,
                    ))
                })
                relevantOverlays = enabledOverlays.filter((overlay) =>
                    selected.some((tenant) => tenant.id === overlay.tenantId),
                )
                if (relevantOverlays.length === 0) {
                    emptyTitle = '所选租户尚无流量'
                    emptyDescription = '叠加模板后即可比较各租户的独立曲线'
                }
            }
        }

        if (series.length > 0 && relevantOverlays.length > 0) {
            const uniqueStarts = [...new Set(relevantOverlays.map((overlay) => overlay.startOffsetSeconds))]
            series[0] = {
                ...series[0],
                markLine: {
                    silent: true,
                    symbol: 'none',
                    label: { show: false },
                    lineStyle: { color: 'rgba(126,161,214,.22)', width: 1, type: 'dashed' },
                    data: uniqueStarts.map((start) => ({ xAxis: start })),
                },
            }
        }

        const option: EChartsOption = {
            animationDuration: 480,
            animationEasing: 'cubicOut',
            backgroundColor: 'transparent',
            aria: { enabled: true },
            tooltip: {
                trigger: 'axis',
                confine: true,
                backgroundColor: 'rgba(8,12,18,.97)',
                borderColor: 'rgba(255,255,255,.10)',
                borderWidth: 1,
                padding: [10, 12],
                textStyle: { color: '#E8EEF7', fontSize: 11 },
                axisPointer: {
                    type: 'line',
                    lineStyle: { color: 'rgba(116,177,255,.30)', type: 'dashed' },
                },
                formatter: (params: unknown) => {
                    const items = Array.isArray(params) ? [...params] : []
                    const time = Number(items[0]?.value?.[0] ?? 0)
                    items.sort((a, b) => Number(b.value?.[1] ?? 0) - Number(a.value?.[1] ?? 0))
                    const rows = items.map((item) => `<div style="display:flex;min-width:170px;justify-content:space-between;gap:22px;margin-top:3px"><span style="color:#8B97A9">${item.seriesName}</span><b style="color:${item.color}">${formatQps(Number(item.value?.[1] ?? 0))} QPS</b></div>`).join('')
                    return `<div style="color:#778397;margin-bottom:6px">逻辑时间 · T+${formatLogicalTime(time)}</div>${rows}`
                },
            },
            legend: {
                show: viewMode === 'compare' && series.length > 0,
                top: 8,
                right: 18,
                itemWidth: 16,
                itemHeight: 3,
                textStyle: { color: '#8290A4', fontSize: 11 },
            },
            grid: { left: 72, right: 26, top: 48, bottom: 72 },
            xAxis: {
                type: 'value',
                min: 0,
                max: horizon,
                name: '逻辑时间（从 T+0 开始）',
                nameLocation: 'middle',
                nameGap: 34,
                nameTextStyle: { color: '#667286', fontSize: 10 },
                axisLine: { lineStyle: { color: 'rgba(148,163,184,.22)' } },
                axisTick: { show: false },
                axisLabel: {
                    color: '#758197',
                    fontSize: 10,
                    hideOverlap: true,
                    formatter: (value: number) => `T+${formatLogicalTime(value)}`,
                },
                splitLine: { show: false },
            },
            yAxis: {
                type: 'value',
                min: 0,
                name: 'QPS',
                nameTextStyle: { color: '#667286', fontSize: 10, padding: [0, 0, 6, 0] },
                axisLine: { show: false },
                axisTick: { show: false },
                axisLabel: { color: '#758197', fontSize: 10, formatter: formatQps },
                splitLine: { lineStyle: { color: 'rgba(148,163,184,.07)', type: 'dashed' } },
            },
            dataZoom: [
                { type: 'inside', filterMode: 'none', start: 0, end: 100, minSpan: 1 },
                {
                    type: 'slider',
                    filterMode: 'none',
                    start: 0,
                    end: 100,
                    height: 16,
                    bottom: 8,
                    borderColor: 'rgba(255,255,255,.06)',
                    backgroundColor: 'rgba(255,255,255,.018)',
                    fillerColor: 'rgba(91,140,255,.13)',
                    dataBackground: {
                        lineStyle: { color: 'rgba(91,140,255,.30)' },
                        areaStyle: { color: 'rgba(91,140,255,.05)' },
                    },
                    selectedDataBackground: {
                        lineStyle: { color: 'rgba(91,140,255,.62)' },
                        areaStyle: { color: 'rgba(91,140,255,.10)' },
                    },
                    handleStyle: { color: '#5B8CFF', borderColor: '#9BC2FF' },
                    moveHandleStyle: { color: 'rgba(91,140,255,.35)' },
                    textStyle: { color: '#667286', fontSize: 9 },
                },
            ],
            series: series as EChartsOption['series'],
        }

        return { option, emptyTitle, emptyDescription }
    }, [compareTenants, overlays, selectedTenant, templates, tenants, viewMode])

    return (
        <div className={`relative h-full w-full overflow-hidden rounded-2xl border border-white/[0.07] bg-[#080C12] ${className}`}>
            <div className="pointer-events-none absolute inset-x-0 top-0 z-10 h-24 bg-gradient-to-b from-white/[0.018] to-transparent" />
            <ReactECharts
                option={chartState.option}
                style={{ width: '100%', height: '100%' }}
                opts={{ renderer: 'canvas' }}
                notMerge
            />
            {chartState.emptyTitle && (
                <div className="pointer-events-none absolute inset-0 flex items-center justify-center px-6">
                    <div className="mb-8 max-w-[390px] rounded-2xl border border-white/[0.07] bg-[#0C111A]/88 px-7 py-6 text-center shadow-[0_24px_80px_rgba(0,0,0,.28)] backdrop-blur-xl">
                        <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-xl border border-[#5B8CFF]/20 bg-[#5B8CFF]/10 text-[#77ADFF]">
                            {viewMode === 'total' ? <Layers3 className="h-4 w-4" /> : viewMode === 'compare' ? <Activity className="h-4 w-4" /> : <MousePointer2 className="h-4 w-4" />}
                        </div>
                        <div className="text-sm font-medium text-[#DDE6F2]">{chartState.emptyTitle}</div>
                        <div className="mt-1.5 text-[11px] leading-5 text-[#657184]">{chartState.emptyDescription}</div>
                    </div>
                </div>
            )}
        </div>
    )
}

function makeSeries(
    name: string,
    color: string,
    timePoints: number[],
    values: number[],
    withArea: boolean,
): Record<string, unknown> {
    return {
        name,
        type: 'line',
        data: timePoints.map((time, index) => [time, values[index]]),
        smooth: 0.16,
        symbol: 'none',
        sampling: 'lttb',
        lineStyle: {
            color,
            width: 2.4,
            shadowBlur: 12,
            shadowColor: `${color}33`,
        },
        itemStyle: { color },
        areaStyle: withArea ? {
            color: {
                type: 'linear',
                x: 0,
                y: 0,
                x2: 0,
                y2: 1,
                colorStops: [
                    { offset: 0, color: `${color}2B` },
                    { offset: 1, color: `${color}02` },
                ],
            },
        } : undefined,
    }
}
