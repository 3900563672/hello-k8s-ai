import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { PreviewCurve } from '@/components/features/traffic/PreviewCurve'
import type { TrafficTemplate } from '@/types/traffic.types'

vi.mock('echarts-for-react', () => ({
    default: ({ option, style }: { option: Record<string, unknown>; style: Record<string, string> }) => (
        <div data-testid="echarts" data-option={JSON.stringify(option)} style={style} />
    ),
}))

const makeTemplate = (overrides: Partial<TrafficTemplate> = {}): TrafficTemplate => ({
    id: 'tpl-1',
    name: '潮汐流量',
    shapeType: 'sine',
    description: '',
    controlPoints: [
        { x: 0, y: 20 },
        { x: 1_800_000, y: 100 },
        { x: 3_600_000, y: 20 },
    ],
    createdAt: '2026-08-20T00:00:00.000Z',
    updatedAt: '2026-08-20T00:00:00.000Z',
    ...overrides,
})

describe('PreviewCurve', () => {
    it('有效坐标时渲染图表并生成 series 数据', () => {
        render(<PreviewCurve template={makeTemplate()} />)
        const chart = screen.getByTestId('echarts')
        expect(chart).toBeInTheDocument()
        const option = JSON.parse(chart.getAttribute('data-option') ?? '{}') as {
            series: Array<{ name: string; data: Array<[number, number]> }>
        }
        expect(option.series[0].name).toBe('潮汐流量')
        expect(option.series[0].data).toEqual([
            [0, 20],
            [1800000, 100],
            [3600000, 20],
        ])
    })

    it('坐标点不足 2 个时展示占位提示', () => {
        render(<PreviewCurve template={makeTemplate({ controlPoints: [{ x: 0, y: 10 }] })} />)
        expect(screen.getByText('模板没有可预览的有效坐标')).toBeInTheDocument()
    })

    it('无效坐标会被 sanitize 过滤', () => {
        render(
            <PreviewCurve
                template={makeTemplate({
                    controlPoints: [
                        { x: 0, y: 10 },
                        { x: Number.NaN, y: 50 },
                        { x: 100, y: Number.NaN },
                        { x: 200, y: 30 },
                    ],
                })}
            />,
        )
        const chart = screen.getByTestId('echarts')
        const option = JSON.parse(chart.getAttribute('data-option') ?? '{}') as {
            series: Array<{ data: Array<[number, number]> }>
        }
        expect(option.series[0].data).toEqual([
            [0, 10],
            [200, 30],
        ])
    })
})