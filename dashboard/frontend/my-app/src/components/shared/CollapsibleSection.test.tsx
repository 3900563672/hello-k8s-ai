import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { CollapsibleSection } from '@/components/shared/CollapsibleSection'

describe('CollapsibleSection', () => {
    it('默认收起时不渲染内容，点击后展开', async () => {
        const user = userEvent.setup()
        render(
            <CollapsibleSection title="拓扑配置" subtitle="节点与租户">
                <div>内部内容</div>
            </CollapsibleSection>,
        )
        expect(screen.getByText('拓扑配置')).toBeInTheDocument()
        expect(screen.getByText('展开')).toBeInTheDocument()
        expect(screen.queryByText('内部内容')).not.toBeInTheDocument()

        await user.click(screen.getByText('拓扑配置'))
        expect(screen.getByText('内部内容')).toBeInTheDocument()
        expect(screen.getByText('收起')).toBeInTheDocument()
    })

    it('defaultOpen 时直接渲染内容，可再次收起', async () => {
        const user = userEvent.setup()
        render(
            <CollapsibleSection title="默认展开" subtitle="" defaultOpen>
                <div>已展开内容</div>
            </CollapsibleSection>,
        )
        expect(screen.getByText('已展开内容')).toBeInTheDocument()
        await user.click(screen.getByText('默认展开'))
        expect(screen.queryByText('已展开内容')).not.toBeInTheDocument()
    })
})