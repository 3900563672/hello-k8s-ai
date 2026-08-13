import { memo, useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Cpu } from 'lucide-react'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { nodeSchema, getNodePreview, type NodeFormValues } from '@/lib/validations/node.schema'
import { useTemplateStore } from '@/stores/templateSlice'
import type { ConfigTemplate } from '@/types/config.types'
import {
    ConfigFormSection,
    FormSaveBar,
    TemplateActions,
    configInputClass,
    configLabelClass,
    numberFromInput,
} from './ConfigFormParts'

interface NodeFormProps {
    defaultValues: NodeFormValues
    onSubmit: (data: NodeFormValues) => Promise<void>
    submitLabel?: string
    onDirtyChange?: (dirty: boolean) => void
}

const getErrorMessage = (error: unknown): string =>
    error instanceof Error ? error.message : '保存失败，请稍后重试'

export const NodeForm = memo(function NodeForm({
    defaultValues,
    onSubmit,
    submitLabel = '保存节点',
    onDirtyChange,
}: NodeFormProps) {
    const form = useForm<NodeFormValues>({
        resolver: zodResolver(nodeSchema),
        defaultValues,
        mode: 'onBlur',
    })
    const [submitError, setSubmitError] = useState('')
    const { nodeTemplates, addNodeTemplate, removeNodeTemplate } = useTemplateStore()
    const { isDirty, isSubmitting } = form.formState

    useEffect(() => {
        form.reset(defaultValues)
        setSubmitError('')
    }, [defaultValues, form])

    useEffect(() => {
        onDirtyChange?.(isDirty)
    }, [isDirty, onDirtyChange])

    const submitForm = async (values: NodeFormValues) => {
        setSubmitError('')
        try {
            await onSubmit(values)
            form.reset(values)
        } catch (error) {
            setSubmitError(getErrorMessage(error))
        }
    }

    const saveTemplate = async (name: string) => {
        const valid = await form.trigger()
        if (!valid) return false
        addNodeTemplate(name, form.getValues())
        return true
    }

    const loadTemplate = (template: ConfigTemplate<NodeFormValues>) => {
        form.reset(template.data, { keepDefaultValues: true })
    }

    return (
        <Form {...form}>
            <form onSubmit={form.handleSubmit(submitForm)} className="space-y-4">
                <TemplateActions
                    typeLabel="节点"
                    templates={nodeTemplates}
                    onSave={saveTemplate}
                    onLoad={loadTemplate}
                    onDelete={removeNodeTemplate}
                    getPreview={getNodePreview}
                />

                <ConfigFormSection
                    title="节点容量"
                    description="配置节点的显示信息、可用显存与并发承载上限。"
                    action={<Cpu className="h-4 w-4 text-[#5E9EFF]" />}
                >
                    <div className="grid gap-4 sm:grid-cols-2">
                        <FormField
                            control={form.control}
                            name="displayName"
                            render={({ field }) => (
                                <FormItem className="sm:col-span-2">
                                    <FormLabel className={configLabelClass}>显示名称</FormLabel>
                                    <FormControl>
                                        <Input {...field} autoComplete="off" className={configInputClass} />
                                    </FormControl>
                                    <FormMessage className="text-xs text-[#FF7373]" />
                                </FormItem>
                            )}
                        />

                        <FormField
                            control={form.control}
                            name="gpu"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel className={configLabelClass}>总显存</FormLabel>
                                    <FormControl>
                                        <div className="relative">
                                            <Input
                                                type="number"
                                                min="0"
                                                step="any"
                                                {...field}
                                                value={Number.isNaN(field.value) ? '' : field.value}
                                                onChange={(event) =>
                                                    field.onChange(
                                                        numberFromInput(event.currentTarget.value, event.currentTarget.valueAsNumber),
                                                    )
                                                }
                                                className={`${configInputClass} pr-10 tabular-nums`}
                                            />
                                            <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-[#596579]">G</span>
                                        </div>
                                    </FormControl>
                                    <FormMessage className="text-xs text-[#FF7373]" />
                                </FormItem>
                            )}
                        />

                        <FormField
                            control={form.control}
                            name="maxConcurrency"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel className={configLabelClass}>最大并发</FormLabel>
                                    <FormControl>
                                        <Input
                                            type="number"
                                            min="1"
                                            step="1"
                                            {...field}
                                            value={Number.isNaN(field.value) ? '' : field.value}
                                            onChange={(event) =>
                                                field.onChange(
                                                    numberFromInput(event.currentTarget.value, event.currentTarget.valueAsNumber),
                                                )
                                            }
                                            className={`${configInputClass} tabular-nums`}
                                        />
                                    </FormControl>
                                    <FormMessage className="text-xs text-[#FF7373]" />
                                </FormItem>
                            )}
                        />
                    </div>

                    <div className="mt-4 grid grid-cols-2 gap-3 rounded-lg border border-[#222] bg-[#101010] p-3">
                        <div>
                            <p className="text-[11px] text-[#696969]">单并发平均显存</p>
                            <p className="mt-1 text-sm font-medium tabular-nums text-[#D4D4D4]">
                                {form.watch('maxConcurrency') > 0 && Number.isFinite(form.watch('gpu'))
                                    ? `${(form.watch('gpu') / form.watch('maxConcurrency')).toLocaleString('zh-CN', { maximumFractionDigits: 2 })} G`
                                    : '—'}
                            </p>
                        </div>
                        <div className="border-l border-[#252525] pl-3">
                            <p className="text-[11px] text-[#696969]">容量说明</p>
                            <p className="mt-1 text-sm font-medium text-[#D4D4D4]">本地调度上限</p>
                        </div>
                    </div>
                </ConfigFormSection>

                <FormSaveBar
                    dirty={isDirty}
                    submitting={isSubmitting}
                    error={submitError}
                    submitLabel={submitLabel}
                />
            </form>
        </Form>
    )
})
