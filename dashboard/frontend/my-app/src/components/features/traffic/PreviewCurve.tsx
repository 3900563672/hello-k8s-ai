import { useMemo } from 'react'
import ReactECharts from 'echarts-for-react'
import type { EChartsOption } from 'echarts'
import type { TrafficTemplate } from '@/types/traffic.types'
import {
    formatLogicalTime,
    formatQps,
    getTemplateDuration,
    getTemplatePeakQps,
    sanitizeControlPoints,
} from './trafficMath'

interface PreviewCurveProps {
    template: TrafficTemplate
    className?: string
}

export function PreviewCurve({ template, className = '' }: PreviewCurveProps) {
    const points = useMemo(
        () => sanitizeControlPoints(template.controlPoints),
        [template.controlPoints],
    )

    const option = useMemo<EChartsOption>(() => {
        const duration = Math.max(1, getTemplateDuration(template))
        const peak = getTemplatePeakQps(template)

        return {
            animationDuration: 420,
            animationEasing: 'cubicOut',
            backgroundColor: 'transparent',
            aria: { enabled: true },
            tooltip: {
                trigger: 'axis',
                confine: true,
                backgroundColor: 'rgba(9, 13, 20, 0.96)',
                borderColor: 'rgba(255,255,255,0.10)',
                borderWidth: 1,
                padding: [9, 11],
                textStyle: { color: '#E8EEF7', fontSize: 11 },
                axisPointer: {
                    type: 'line',
                    lineStyle: { color: 'rgba(116, 177, 255, 0.38)', type: 'dashed' },
                },
                formatter: (params: any) => {
                    const item = Array.isArray(params) ? params[0] : params
                    const value = item?.value as [number, number] | undefined
                    if (!value) return ''
                    return `<div style="color:#7F8B9D;margin-bottom:4px">T+${formatLogicalTime(value[0])}</div><div style="display:flex;gap:24px;justify-content:space-between"><span>QPS</span><b style="color:#78B5FF">${formatQps(value[1])}</b></div>`
                },
            },
            grid: { left: 62, right: 24, top: 22, bottom: 48 },
            xAxis: {
                type: 'value',
                min: 0,
                max: duration,
                name: '逻辑时间',
                nameLocation: 'middle',
                nameGap: 32,
                nameTextStyle: { color: '#697588', fontSize: 10 },
                axisLine: { lineStyle: { color: 'rgba(148,163,184,0.22)' } },
                axisTick: { show: false },
                axisLabel: {
                    color: '#738095',
                    fontSize: 10,
                    formatter: (value: number) => `T+${formatLogicalTime(value)}`,
                },
                splitLine: { show: false },
            },
            yAxis: {
                type: 'value',
                min: 0,
                max: Math.max(1, Math.ceil(peak * 1.16)),
                name: 'QPS',
                nameGap: 12,
                nameTextStyle: { color: '#697588', fontSize: 10 },
                axisLine: { show: false },
                axisTick: { show: false },
                axisLabel: { color: '#738095', fontSize: 10, formatter: formatQps },
                splitLine: { lineStyle: { color: 'rgba(148,163,184,0.075)', type: 'dashed' } },
            },
            series: [{
                name: template.name,
                type: 'line',
                data: points.map((point) => [point.x, point.y]),
                smooth: 0.22,
                showSymbol: points.length <= 36,
                symbol: 'circle',
                symbolSize: 4,
                itemStyle: { color: '#78B5FF', borderColor: '#0B1018', borderWidth: 1.5 },
                lineStyle: { color: '#64A5FF', width: 2.5, shadowBlur: 12, shadowColor: 'rgba(47,129,247,0.28)' },
                areaStyle: {
                    color: {
                        type: 'linear',
                        x: 0,
                        y: 0,
                        x2: 0,
                        y2: 1,
                        colorStops: [
                            { offset: 0, color: 'rgba(47,129,247,0.28)' },
                            { offset: 1, color: 'rgba(47,129,247,0.01)' },
                        ],
                    },
                },
            }],
        }
    }, [points, template])

    if (points.length < 2) {
        return (
            <div className={`flex h-full w-full items-center justify-center text-xs text-[#697588] ${className}`}>
                模板没有可预览的有效坐标
            </div>
        )
    }

    return (
        <div className={`h-full w-full ${className}`}>
            <ReactECharts
                option={option}
                style={{ width: '100%', height: '100%' }}
                opts={{ renderer: 'canvas' }}
                notMerge
            />
        </div>
    )
}
