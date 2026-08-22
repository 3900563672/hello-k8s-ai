import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { ConfigTabPanel } from '@/components/features/config/components/ConfigTabPanel'

interface Item {
    name: string
    displayName: string
}

const items: Item[] = [
    { name: 'model-a', displayName: '模型A' },
    { name: 'model-b', displayName: '模型B' },
]

const TableStub = ({ data, onSelect, selectedIds }: {
    data: Item[]
    onSelect: (item: Item) => void
    selectedIds: string[]
}) => (
    <ul>
        {data.map((item) => (
            <li key={item.name}>
                <button type="button" onClick={() => onSelect(item)}>{item.displayName}</button>
            </li>
        ))}
        {selectedIds.length > 0 && <li data-testid="selected-count">{selectedIds.length}</li>}
    </ul>
)

const FormStub = ({ defaultValues, onSubmit, onDirtyChange }: {
    defaultValues: Item
    onSubmit: (data: Item) => Promise<void>
    onDirtyChange?: (dirty: boolean) => void
}) => {
    const [value, setValue] = useState(defaultValues.displayName)
    return (
        <form
            onSubmit={(event) => {
                event.preventDefault()
                void onSubmit({ name: defaultValues.name, displayName: value })
            }}
        >
            <input
                aria-label="displayName"
                value={value}
                onChange={(event) => {
                    setValue(event.target.value)
                    onDirtyChange?.(event.target.value !== defaultValues.displayName)
                }}
            />
            <button type="submit">保存</button>
        </form>
    )
}

const renderPanel = (props: Partial<Parameters<typeof ConfigTabPanel<Item, Item>>[0]> = {}) => {
    const base = {
        data: items,
        selectedItem: null,
        onSelect: vi.fn(),
        onDelete: vi.fn(),
        selectedIds: [],
        onSelectionChange: vi.fn(),
        TableComponent: TableStub,
        FormComponent: FormStub,
        getFormValues: (item: Item) => item,
        typeLabel: '模型',
        listTitle: '模型列表',
        listDescription: '管理模型资源',
        detailDescription: '配置详情',
        resourceIcon: <span>icon</span>,
        onCreate: vi.fn(),
        onBatchDelete: vi.fn(),
        formSubmit: vi.fn(async () => {}),
    }
    return { ...base, ...props }
}

describe('ConfigTabPanel', () => {
    it('渲染列表标题、搜索框与新建按钮', () => {
        render(<ConfigTabPanel {...renderPanel()} />)
        expect(screen.getByText('模型列表')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /新建模型/ })).toBeInTheDocument()
        expect(screen.getByRole('textbox', { name: '搜索模型' })).toBeInTheDocument()
        expect(screen.getByText('选择一个模型开始配置')).toBeInTheDocument()
    })

    it('搜索过滤列表项', async () => {
        const user = userEvent.setup()
        render(<ConfigTabPanel {...renderPanel()} />)
        await user.type(screen.getByRole('textbox', { name: '搜索模型' }), '模型B')
        expect(screen.queryByText('模型A')).not.toBeInTheDocument()
        expect(screen.getByText('模型B')).toBeInTheDocument()
    })

    it('选择条目后渲染表单与已持久化标记', async () => {
        const user = userEvent.setup()
        const onSelect = vi.fn()
        const props = renderPanel({ onSelect, selectedItem: null })
        const { rerender } = render(<ConfigTabPanel {...props} />)
        await user.click(screen.getByText('模型A'))
        expect(onSelect).toHaveBeenCalledWith(items[0])
        rerender(<ConfigTabPanel {...props} selectedItem={items[0]} />)
        expect(await screen.findByText('已持久化')).toBeInTheDocument()
        expect(screen.getByLabelText('displayName')).toHaveValue('模型A')
    })

    it('表单脏时切换条目弹出放弃确认', async () => {
        const user = userEvent.setup()
        const onSelect = vi.fn()
        const props = renderPanel({ onSelect })
        render(<ConfigTabPanel {...props} selectedItem={items[0]} />)
        await user.type(screen.getByLabelText('displayName'), 'X')
        await user.click(screen.getByText('模型B'))
        expect(onSelect).not.toHaveBeenCalled()
        expect(await screen.findByText('放弃未保存的修改？')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: '放弃并继续' }))
        expect(onSelect).toHaveBeenCalledWith(items[1])
    })

    it('表单脏时新建弹出确认并继续创建', async () => {
        const user = userEvent.setup()
        const onCreate = vi.fn()
        const props = renderPanel({ onCreate, selectedItem: items[0] })
        render(<ConfigTabPanel {...props} />)
        await user.type(screen.getByLabelText('displayName'), 'X')
        await user.click(screen.getByRole('button', { name: /新建模型/ }))
        expect(await screen.findByText('放弃未保存的修改？')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: '放弃并继续' }))
        expect(onCreate).toHaveBeenCalled()
    })

    it('有批量选择时展示删除选中按钮并触发回调', async () => {
        const user = userEvent.setup()
        const onBatchDelete = vi.fn()
        const props = renderPanel({ onBatchDelete, selectedIds: ['model-a'], selectedItem: items[0] })
        render(<ConfigTabPanel {...props} />)
        expect(screen.getByText('已选择 1 项')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: /删除选中/ }))
        expect(onBatchDelete).toHaveBeenCalled()
    })

    it('只读模式禁用新建并展示历史只读', () => {
        render(<ConfigTabPanel {...renderPanel({ readOnly: true, selectedItem: items[0] })} />)
        expect(screen.getByRole('button', { name: /新建模型/ })).toBeDisabled()
        expect(screen.getByText('历史只读')).toBeInTheDocument()
    })
})