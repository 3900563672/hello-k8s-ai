import { describe, expect, it, vi } from 'vitest'
import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderHook } from '@testing-library/react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import {
    ConfigFormSection,
    FormSaveBar,
    LiveImpactSummary,
    TemplateActions,
    numberFromInput,
    useConfigForm,
} from './ConfigFormParts'

describe('numberFromInput', () => {
    it('空字符串转为 NaN，其余保留 valueAsNumber', () => {
        expect(numberFromInput('', 0)).toBe(Number.NaN)
        expect(numberFromInput('8', 8)).toBe(8)
    })
})

describe('ConfigFormSection', () => {
    it('渲染标题、描述与子内容', () => {
        render(
            <ConfigFormSection title="基础配置" description="定义模型资源">
                <p>内容</p>
            </ConfigFormSection>,
        )
        expect(screen.getByText('基础配置')).toBeInTheDocument()
        expect(screen.getByText('定义模型资源')).toBeInTheDocument()
        expect(screen.getByText('内容')).toBeInTheDocument()
    })
})

describe('FormSaveBar', () => {
    it('脏状态展示未保存提示并启用提交', () => {
        render(<FormSaveBar dirty submitting={false} error="" submitLabel="保存模型" />)
        expect(screen.getByText('有未保存的修改')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: '保存模型' })).toBeEnabled()
    })

    it('干净状态展示已保存提示并禁用提交', () => {
        render(<FormSaveBar dirty={false} submitting={false} error="" submitLabel="保存模型" />)
        expect(screen.getByText('当前配置已保存')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: '保存模型' })).toBeDisabled()
    })

    it('提交中展示正在保存', () => {
        render(<FormSaveBar dirty submitting error="" submitLabel="保存模型" />)
        expect(screen.getByText('正在保存')).toBeInTheDocument()
    })

    it('有错误时展示错误信息', () => {
        render(<FormSaveBar dirty={false} submitting={false} error="保存失败：超时" submitLabel="保存模型" />)
        expect(screen.getByText('保存失败：超时')).toBeInTheDocument()
    })
})

describe('LiveImpactSummary', () => {
    it('空字段列表不渲染', () => {
        const { container } = render(<LiveImpactSummary fields={[]} />)
        expect(container.firstChild).toBeNull()
    })

    it('渲染键、值与单位', () => {
        render(
            <LiveImpactSummary
                fields={[
                    { key: '显存需求', value: 16, unit: 'G' },
                    { key: '最大并发', value: 32 },
                ]}
            />,
        )
        expect(screen.getByText('变更影响')).toBeInTheDocument()
        expect(screen.getByText('显存需求')).toBeInTheDocument()
        expect(screen.getByText('16')).toBeInTheDocument()
        expect(screen.getByText('32')).toBeInTheDocument()
        expect(screen.getAllByText('G').length).toBeGreaterThan(0)
    })
})

describe('TemplateActions', () => {
    const templates = [
        { id: 'tpl-1', name: '高性能配置', preset: false, createdAt: '2026-01-01T00:00:00.000Z', data: { displayName: 'x' } },
    ]

    it('展示模板数量与按钮', () => {
        render(
            <TemplateActions
                typeLabel="模型"
                templates={templates}
                onSave={vi.fn()}
                onLoad={vi.fn()}
                onDelete={vi.fn()}
                getPreview={(data) => [{ key: '名称', value: data.displayName }]}
            />,
        )
        expect(screen.getByText('配置模板')).toBeInTheDocument()
        expect(screen.getByText(/1 个模板/)).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /模板库/ })).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /存为模板/ })).toBeInTheDocument()
    })

    it('输入名称保存模板后关闭对话框并调用 onSave', async () => {
        const user = userEvent.setup()
        const onSave = vi.fn(() => true)
        render(
            <TemplateActions
                typeLabel="模型"
                templates={templates}
                onSave={onSave}
                onLoad={vi.fn()}
                onDelete={vi.fn()}
                getPreview={(data) => [{ key: '名称', value: data.displayName }]}
            />,
        )
        await user.click(screen.getByRole('button', { name: /存为模板/ }))
        expect(screen.getByText('保存模型模板')).toBeInTheDocument()
        await user.type(screen.getByLabelText('模板名称'), '我的模板')
        await user.click(screen.getByRole('button', { name: '保存模板' }))
        expect(onSave).toHaveBeenCalledWith('我的模板')
        expect(screen.queryByText('保存模型模板')).not.toBeInTheDocument()
    })

    it('空名称时保存按钮禁用', async () => {
        const user = userEvent.setup()
        const onSave = vi.fn()
        render(
            <TemplateActions
                typeLabel="模型"
                templates={templates}
                onSave={onSave}
                onLoad={vi.fn()}
                onDelete={vi.fn()}
                getPreview={(data) => [{ key: '名称', value: data.displayName }]}
            />,
        )
        await user.click(screen.getByRole('button', { name: /存为模板/ }))
        expect(screen.getByRole('button', { name: '保存模板' })).toBeDisabled()
    })

    it('从模板库加载模板并调用 onLoad', async () => {
        const user = userEvent.setup()
        const onLoad = vi.fn()
        render(
            <TemplateActions
                typeLabel="模型"
                templates={templates}
                onSave={vi.fn()}
                onLoad={onLoad}
                onDelete={vi.fn()}
                getPreview={(data) => [{ key: '名称', value: data.displayName }]}
            />,
        )
        await user.click(screen.getByRole('button', { name: /模板库/ }))
        expect(await screen.findByText('模板库 — 模型')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: '加载模板' }))
        expect(onLoad).toHaveBeenCalledWith(templates[0])
    })

    it('模板库中删除模板调用 onDelete', async () => {
        const user = userEvent.setup()
        const onDelete = vi.fn()
        render(
            <TemplateActions
                typeLabel="模型"
                templates={templates}
                onSave={vi.fn()}
                onLoad={vi.fn()}
                onDelete={onDelete}
                getPreview={(data) => [{ key: '名称', value: data.displayName }]}
            />,
        )
        await user.click(screen.getByRole('button', { name: /模板库/ }))
        await user.click(await screen.findByRole('button', { name: '删除' }))
        expect(onDelete).toHaveBeenCalledWith('tpl-1')
    })
})

describe('useConfigForm', () => {
    const schema = z.object({ displayName: z.string().min(1, '不能为空') })

    it('提交成功时清空错误并 reset 表单', async () => {
        const { result } = renderHook(() => {
            const form = useForm({ defaultValues: { displayName: 'a' } })
            return { form, hook: useConfigForm({ form, defaultValues: { displayName: 'a' }, onSubmit: vi.fn(), onDirtyChange: vi.fn(), addTemplate: vi.fn() }) }
        })
        await act(async () => {
            await result.current.hook.submitForm({ displayName: 'a' })
        })
        expect(result.current.hook.submitError).toBe('')
    })

    it('提交抛错时写入 submitError', async () => {
        const { result } = renderHook(() => {
            const form = useForm({ defaultValues: { displayName: 'a' } })
            return useConfigForm({
                form,
                defaultValues: { displayName: 'a' },
                onSubmit: vi.fn(async () => {
                    throw new Error('后端超时')
                }),
                onDirtyChange: vi.fn(),
                addTemplate: vi.fn(),
            })
        })
        await act(async () => {
            await result.current.submitForm({ displayName: 'a' })
        })
        expect(result.current.submitError).toBe('后端超时')
    })

    it('提交非 Error 异常时使用兜底文案', async () => {
        const { result } = renderHook(() => {
            const form = useForm({ defaultValues: { displayName: 'a' } })
            return useConfigForm({
                form,
                defaultValues: { displayName: 'a' },
                onSubmit: vi.fn(async () => {
                    throw 'boom'
                }),
                onDirtyChange: vi.fn(),
                addTemplate: vi.fn(),
            })
        })
        await act(async () => {
            await result.current.submitForm({ displayName: 'a' })
        })
        expect(result.current.submitError).toBe('保存失败，请稍后重试')
    })

    it('saveTemplate 校验不通过时不保存', async () => {
        const addTemplate = vi.fn()
        const { result } = renderHook(() => {
            const form = useForm({ resolver: zodResolver(schema), defaultValues: { displayName: '' } })
            return useConfigForm({ form, defaultValues: { displayName: '' }, onSubmit: vi.fn(), onDirtyChange: vi.fn(), addTemplate })
        })
        const saved = await act(async () => result.current.saveTemplate('模板'))
        expect(saved).toBe(false)
        expect(addTemplate).not.toHaveBeenCalled()
    })

    it('saveTemplate 校验通过时写入模板', async () => {
        const addTemplate = vi.fn()
        const { result } = renderHook(() => {
            const form = useForm({ resolver: zodResolver(schema), defaultValues: { displayName: 'a' } })
            return useConfigForm({ form, defaultValues: { displayName: '' }, onSubmit: vi.fn(), onDirtyChange: vi.fn(), addTemplate })
        })
        const saved = await act(async () => result.current.saveTemplate('模板'))
        expect(saved).toBe(true)
        expect(addTemplate).toHaveBeenCalledWith('模板', { displayName: 'a' })
    })

    it('loadTemplate 重置表单并触发 afterLoadTemplate', () => {
        const afterLoadTemplate = vi.fn()
        const { result } = renderHook(() => {
            const form = useForm({ defaultValues: { displayName: 'a' } })
            return { form, hook: useConfigForm({ form, defaultValues: { displayName: 'a' }, onSubmit: vi.fn(), onDirtyChange: vi.fn(), afterLoadTemplate }) }
        })
        act(() => {
            result.current.hook.loadTemplate({ id: 't', name: 't', createdAt: '2026-01-01T00:00:00.000Z', data: { displayName: 'b' } })
        })
        expect(result.current.form.getValues('displayName')).toBe('b')
        expect(afterLoadTemplate).toHaveBeenCalled()
    })
})