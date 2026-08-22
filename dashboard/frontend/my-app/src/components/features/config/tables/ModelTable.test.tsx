import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ModelTable } from '@/components/features/config/tables/ModelTable'
import type { Model } from '@/types/config.types'

const data: Model[] = [
    {
        name: 'model-a',
        displayName: '模型A',
        gpuUnits: 16,
        maxConcurrency: 32,
        absoluteScore: 100,
        coldStartMs: 1500,
        performance: { prefillBaseMs: 50, prefillPerTokenUs: 500, decodePerTokenMs: 20 },
    },
    {
        name: 'model-b',
        displayName: '模型B',
        gpuUnits: 8,
        maxConcurrency: 16,
        absoluteScore: 0,
        coldStartMs: 800,
        performance: { prefillBaseMs: 50, prefillPerTokenUs: 500, decodePerTokenMs: 20 },
    },
]

const renderTable = (props: Partial<Parameters<typeof ModelTable>[0]> = {}) =>
    render(
        <ModelTable
            data={data}
            onSelect={vi.fn()}
            onDelete={vi.fn()}
            selectedIds={[]}
            onSelectionChange={vi.fn()}
            {...props}
        />,
    )

describe('ModelTable', () => {
    it('渲染模型列表与数值列', () => {
        renderTable()
        expect(screen.getByText('模型A')).toBeInTheDocument()
        expect(screen.getByText('model-a')).toBeInTheDocument()
        expect(screen.getByText('16 G')).toBeInTheDocument()
        expect(screen.getByText('32')).toBeInTheDocument()
    })

    it('基准分为 0 时展示待配置', () => {
        renderTable()
        expect(screen.getByText('待配置')).toBeInTheDocument()
    })

    it('点击行触发 onSelect', async () => {
        const user = userEvent.setup()
        const onSelect = vi.fn()
        renderTable({ onSelect })
        await user.click(screen.getByText('模型A'))
        expect(onSelect).toHaveBeenCalledWith(data[0])
    })
})