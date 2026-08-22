import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { PreviewCanvas } from '@/components/features/traffic/PreviewCanvas'
import { useTrafficStore } from '@/stores/trafficSlice'
import { useTimeStore } from '@/stores/timeSlice'
import type { TrafficTemplate } from '@/types/traffic.types'

vi.mock('echarts-for-react', () => ({
    default: () => <div data-testid="preview-chart" />,
}))

const template: TrafficTemplate = {
    id: 'tpl-1',
    name: '脉冲峰值',
    shapeType: 'spike',
    controlPoints: [
        { x: 0, y: 0 },
        { x: 60, y: 50 },
        { x: 120, y: 0 },
    ],
    createdAt: '2026-01-01T00:00:00.000Z',
    updatedAt: '2026-01-01T00:00:00.000Z',
}

describe('PreviewCanvas', () => {
    beforeEach(() => {
        useTrafficStore.setState({ templates: [template], overlays: [] })
        useTimeStore.setState({
            mode: 'latest',
            timestamp: '2026-01-01T00:00:00.000Z',
            selectedSnapshotId: null,
            revision: 0,
            snapshots: [],
        })
    })

    it('无模板/无租户/负偏移时显示等待配置空态', () => {
        render(<PreviewCanvas template={null} tenantId="" offsetSeconds={null} />)
        expect(screen.getByText('等待配置')).toBeInTheDocument()
    })

    it('有效输入时渲染叠加前后对比图表与峰值增量', () => {
        render(<PreviewCanvas template={template} tenantId="tenant-a" offsetSeconds={30} />)
        expect(screen.getByTestId('preview-chart')).toBeInTheDocument()
        expect(screen.getByText(/峰值增量/)).toBeInTheDocument()
    })

    it('负偏移视同无效输入，保持空态', () => {
        render(<PreviewCanvas template={template} tenantId="tenant-a" offsetSeconds={-1} />)
        expect(screen.getByText('等待配置')).toBeInTheDocument()
    })
})
