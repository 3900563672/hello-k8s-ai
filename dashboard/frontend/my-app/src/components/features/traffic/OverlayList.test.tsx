import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useTrafficStore } from '@/stores/trafficSlice'
import { OverlayList } from '@/components/features/traffic/OverlayList'
import type { OverlayInstance, TrafficTemplate } from '@/types/traffic.types'

vi.mock('echarts-for-react', () => ({
    default: () => <div data-testid="echarts" />,
}))

const template: TrafficTemplate = {
    id: 'tpl-1',
    name: '潮汐流量',
    description: '',
    shapeType: 'sine',
    controlPoints: [
        { x: 0, y: 10 },
        { x: 1800, y: 50 },
    ],
    createdAt: '2026-08-20T00:00:00.000Z',
    updatedAt: '2026-08-20T00:00:00.000Z',
}

const overlay: OverlayInstance = {
    id: 'ov-1',
    templateId: 'tpl-1',
    templateName: '潮汐流量',
    tenantId: 'tenant-1',
    tenantName: '生产租户',
    startOffsetSeconds: 300,
    effectiveAt: '2026-08-20T00:00:00.000Z',
    snapshotId: null,
    enabled: true,
    color: '#5B8CFF',
    createdAt: '2026-08-20T00:00:00.000Z',
}

describe('OverlayList', () => {
    beforeEach(() => {
        useTrafficStore.setState({ overlays: [], templates: [template] })
    })

    it('无叠加时展示空态', () => {
        render(<OverlayList />)
        expect(screen.getByText('暂无叠加，点击模板即可配置')).toBeInTheDocument()
    })

    it('渲染叠加卡片并展示租户与偏移信息', () => {
        useTrafficStore.setState({ overlays: [overlay] })
        render(<OverlayList />)
        expect(screen.getByText('潮汐流量')).toBeInTheDocument()
        expect(screen.getByText(/生产租户/)).toBeInTheDocument()
        expect(screen.getByText('1')).toBeInTheDocument()
    })

    it('点击叠加打开详情弹窗', async () => {
        const user = userEvent.setup()
        useTrafficStore.setState({ overlays: [overlay] })
        render(<OverlayList />)
        await user.click(screen.getByRole('button', { name: /潮汐流量/ }))
        expect(await screen.findByText('叠加实例详情')).toBeInTheDocument()
    })

    it('点击禁用按钮切换叠加启用状态且不打开弹窗', async () => {
        const user = userEvent.setup()
        useTrafficStore.setState({ overlays: [overlay] })
        render(<OverlayList />)
        await user.click(screen.getByTitle('禁用叠加'))
        expect(useTrafficStore.getState().overlays[0].enabled).toBe(false)
        expect(screen.queryByText('叠加实例详情')).not.toBeInTheDocument()
        await user.click(screen.getByTitle('启用叠加'))
        expect(useTrafficStore.getState().overlays[0].enabled).toBe(true)
    })
})