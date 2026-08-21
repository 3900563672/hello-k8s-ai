import { useMemo } from 'react'
import ReactECharts from 'echarts-for-react'
import type { EChartsOption } from 'echarts'
import { useTrafficStore } from '@/stores/trafficSlice'
import type { OverlayInstance, TrafficTemplate } from '@/types/traffic.types'
import { useReplayTimeContext } from '@/stores/timeSlice'
import {
    buildScenarioTimePoints,
    formatLogicalTime,
    formatQps,
    getOverlayEndSeconds,
    getScenarioHorizon,
    getTenantSeriesValues,
} from './trafficMath'

interface PreviewCanvasProps {
    template: TrafficTemplate | null
    tenantId: string
    offsetSeconds: number | null
    className?: string
}

export function PreviewCanvas({
    template,
    tenantId,
    offsetSeconds,
    className = '',
}: PreviewCanvasProps) {
    const { templates, overlays } = useTrafficStore()
    const replay = useReplayTimeContext()

    const preview = useMemo(() => {
        if (!template || !tenantId || offsetSeconds === null || offsetSeconds < 0) return null

        const existing = overlays.filter(
            (overlay) => overlay.enabled && overlay.tenantId === tenantId,
        )
        const candidate: OverlayInstance = {
            id: '__preview__',
            templateId: template.id,
            templateName: template.name,
            tenantId,
            tenantName: '',
            startOffsetSeconds: offsetSeconds,
            effectiveAt: replay.effectiveAt,
            snapshotId: replay.snapshotId,
            enabled: true,
            color: '#5B8CFF',
            createdAt: '',
        }
        const availableTemplates = templates.some((item) => item.id === template.id)
            ? templates
            : [...templates, template]
        const afterOverlays = [...existing, candidate]
        const candidateEnd = getOverlayEndSeconds(candidate, availableTemplates)
        const horizon = getScenarioHorizon(
            availableTemplates,
            afterOverlays,
            Math.max(120, Math.ceil(candidateEnd * 1.08)),
        )
        const timePoints = buildScenarioTimePoints(availableTemplates, afterOverlays, horizon)
        const beforeValues = getTenantSeriesValues(
            tenantId,
            timePoints,
            availableTemplates,
            existing,
        )
        const afterValues = getTenantSeriesValues(
            tenantId,
            timePoints,
            availableTemplates,
            afterOverlays,
        )
        const peakIncrease = Math.max(
            0,
            ...afterValues.map((value, index) => value - beforeValues[index]),
        )
        return { timePoints, beforeValues, afterValues, horizon, peakIncrease }
    }, [offsetSeconds, overlays, replay.effectiveAt, replay.snapshotId, template, templates, tenantId])

    const option = useMemo<EChartsOption>(() => {
        if (!preview) return {}
        const { timePoints, beforeValues, afterValues, horizon } = preview
        return {
            animationDuration: 360,
            backgroundColor: 'transparent',
            tooltip: {
                trigger: 'axis',
                confine: true,
                backgroundColor: 'rgba(9,13,20,.96)',
                borderColor: 'rgba(255,255,255,.10)',
                textStyle: { color: '#E7EDF7', fontSize: 11 },
                formatter: (params: unknown) => {
                    const items = Array.isArray(params) ? params : []
                    const time = Number(items[0]?.value?.[0] ?? 0)
                    const rows = items.map((item) => `<div style="display:flex;justify-content:space-between;gap:24px"><span style="color:#8995A8">${item.seriesName}</span><b style="color:${item.color}">${formatQps(Number(item.value?.[1] ?? 0))}</b></div>`).join('')
                    return `<div style="margin-bottom:5px;color:#7F8B9D">T+${formatLogicalTime(time)}</div>${rows}`
                },
            },
            legend: {
                top: 0,
                right: 6,
                itemWidth: 14,
                itemHeight: 2,
                textStyle: { color: '#7E8A9D', fontSize: 10 },
            },
            grid: { left: 48, right: 14, top: 32, bottom: 40 },
            xAxis: {
                type: 'value',
                min: 0,
                max: horizon,
                axisLine: { lineStyle: { color: 'rgba(148,163,184,.18)' } },
                axisTick: { show: false },
                axisLabel: {
                    color: '#647084',
                    fontSize: 9,
                    formatter: (value: number) => `T+${formatLogicalTime(value)}`,
                },
                splitLine: { show: false },
            },
            yAxis: {
                type: 'value',
                min: 0,
                axisLine: { show: false },
                axisTick: { show: false },
                axisLabel: { color: '#647084', fontSize: 9, formatter: formatQps },
                splitLine: { lineStyle: { color: 'rgba(148,163,184,.065)', type: 'dashed' } },
            },
            series: [
                {
                    name: '当前流量',
                    type: 'line',
                    data: timePoints.map((time, index) => [time, beforeValues[index]]),
                    smooth: 0.18,
                    symbol: 'none',
                    lineStyle: { color: '#697588', width: 1.4, type: 'dashed' },
                },
                {
                    name: '叠加后',
                    type: 'line',
                    data: timePoints.map((time, index) => [time, afterValues[index]]),
                    smooth: 0.18,
                    symbol: 'none',
                    lineStyle: { color: '#67A8FF', width: 2.3 },
                    areaStyle: { color: 'rgba(47,129,247,.16)' },
                    markLine: {
                        silent: true,
                        symbol: 'none',
                        label: { show: false },
                        lineStyle: { color: 'rgba(103,168,255,.34)', type: 'dashed' },
                        data: [{ xAxis: offsetSeconds ?? 0 }],
                    },
                },
            ],
        }
    }, [offsetSeconds, preview])

    if (!preview) {
        return (
            <div className={`flex h-full w-full items-center justify-center rounded-xl border border-dashed border-white/[0.08] bg-[#080C12] px-8 text-center ${className}`}>
                <div>
                    <div className="text-xs font-medium text-[#8995A8]">等待配置</div>
                    <div className="mt-1 text-[12px] leading-4 text-[#586476]">选择租户并填写逻辑偏移后，这里会显示叠加前后对比</div>
                </div>
            </div>
        )
    }

    return (
        <div className={`relative h-full w-full ${className}`}>
            <ReactECharts
                option={option}
                style={{ width: '100%', height: '100%' }}
                opts={{ renderer: 'canvas' }}
                notMerge
            />
            <div className="pointer-events-none absolute bottom-2 right-2 rounded-md border border-[#5B8CFF]/20 bg-[#0B1220]/90 px-2 py-1 text-[13px] text-[#82B9FF] backdrop-blur">
                峰值增量 +{formatQps(preview.peakIncrease)} QPS
            </div>
        </div>
    )
}
