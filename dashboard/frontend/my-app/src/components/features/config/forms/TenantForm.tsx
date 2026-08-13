import { memo, useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Activity, ArrowDownToLine, ArrowUpFromLine, Shield } from 'lucide-react'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { tenantSchema, getTenantPreview, type TenantFormValues } from '@/lib/validations/tenant.schema'
import { useTemplateStore } from '@/stores/templateSlice'
import type { ConfigTemplate, TenantPriority } from '@/types/config.types'
import {
    ConfigFormSection,
    FormSaveBar,
    TemplateActions,
    configInputClass,
    configLabelClass,
    numberFromInput,
} from './ConfigFormParts'

interface TenantFormProps {
    defaultValues: TenantFormValues
    onSubmit: (data: TenantFormValues) => Promise<void>
    submitLabel?: string
    onDirtyChange?: (dirty: boolean) => void
}

const priorityOptions: Array<{
    value: TenantPriority
    label: string
    description: string
    color: string
}> = [
    { value: 'P1', label: 'P1 · 关键', description: '最高调度优先级', color: 'bg-red-400' },
    { value: 'P2', label: 'P2 · 高', description: '高优先级业务', color: 'bg-orange-400' },
    { value: 'P3', label: 'P3 · 标准', description: '常规业务默认级别', color: 'bg-amber-300' },
    { value: 'P4', label: 'P4 · 低', description: '可延迟任务', color: 'bg-blue-400' },
    { value: 'P5', label: 'P5 · 后台', description: '最低调度优先级', color: 'bg-zinc-400' },
]

const getErrorMessage = (error: unknown): string =>
    error instanceof Error ? error.message : '保存失败，请稍后重试'

export const TenantForm = memo(function TenantForm({
    defaultValues,
    onSubmit,
    submitLabel = '保存租户',
    onDirtyChange,
}: TenantFormProps) {
    const form = useForm<TenantFormValues>({
        resolver: zodResolver(tenantSchema),
        defaultValues,
        mode: 'onBlur',
    })
    const [submitError, setSubmitError] = useState('')
    const { tenantTemplates, addTenantTemplate, removeTenantTemplate } = useTemplateStore()
    const { isDirty, isSubmitting } = form.formState

    useEffect(() => {
        form.reset(defaultValues)
        setSubmitError('')
    }, [defaultValues, form])

    useEffect(() => {
        onDirtyChange?.(isDirty)
    }, [isDirty, onDirtyChange])

    const submitForm = async (values: TenantFormValues) => {
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
        addTenantTemplate(name, form.getValues())
        return true
    }

    const loadTemplate = (template: ConfigTemplate<TenantFormValues>) => {
        form.reset(template.data, { keepDefaultValues: true })
    }

    return (
        <Form {...form}>
            <form onSubmit={form.handleSubmit(submitForm)} className="space-y-4">
                <TemplateActions
                    typeLabel="租户"
                    templates={tenantTemplates}
                    onSave={saveTemplate}
                    onLoad={loadTemplate}
                    onDelete={removeTenantTemplate}
                    getPreview={getTenantPreview}
                />

                <ConfigFormSection
                    title="身份与流量"
                    description="定义租户的显示信息、调度优先级和基准流量。"
                    action={<Shield className="h-4 w-4 text-[#5E9EFF]" />}
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
                            name="priority"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel className={configLabelClass}>调度优先级</FormLabel>
                                    <Select value={field.value} onValueChange={field.onChange}>
                                        <FormControl>
                                            <SelectTrigger className={`${configInputClass} w-full`}>
                                                <SelectValue placeholder="选择优先级" />
                                            </SelectTrigger>
                                        </FormControl>
                                        <SelectContent className="border-[#263244] bg-[#101722] text-[#EDEDED]">
                                            {priorityOptions.map((option) => (
                                                <SelectItem
                                                    key={option.value}
                                                    value={option.value}
                                                    className="focus:bg-[#202B3A] focus:text-white"
                                                >
                                                    <span className="flex items-center gap-2">
                                                        <span className={`h-1.5 w-1.5 rounded-full ${option.color}`} />
                                                        <span>{option.label}</span>
                                                        <span className="text-[#596579]">— {option.description}</span>
                                                    </span>
                                                </SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                    <FormMessage className="text-xs text-[#FF7373]" />
                                </FormItem>
                            )}
                        />

                        <FormField
                            control={form.control}
                            name="qps"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel className={configLabelClass}>基准 QPS</FormLabel>
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
                                                className={`${configInputClass} pr-12 tabular-nums`}
                                            />
                                            <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-[#596579]">QPS</span>
                                        </div>
                                    </FormControl>
                                    <FormMessage className="text-xs text-[#FF7373]" />
                                </FormItem>
                            )}
                        />
                    </div>
                </ConfigFormSection>

                <ConfigFormSection
                    title="弹性阈值"
                    description="分别设置扩容与缩容触发线，缩容阈值应低于对应扩容阈值。"
                    action={<Activity className="h-4 w-4 text-[#5E9EFF]" />}
                >
                    <div className="grid gap-4 lg:grid-cols-2">
                        <div className="rounded-lg border border-[#202B3A] bg-[#101010] p-3.5">
                            <div className="mb-4 flex items-center gap-2">
                                <div className="flex h-7 w-7 items-center justify-center rounded-md bg-blue-500/10">
                                    <ArrowUpFromLine className="h-3.5 w-3.5 text-[#67A6FF]" />
                                </div>
                                <div>
                                    <p className="text-xs font-medium text-[#D7D7D7]">扩容触发</p>
                                    <p className="text-[11px] text-[#596579]">超过阈值时增加容量</p>
                                </div>
                            </div>
                            <div className="space-y-4">
                                <FormField
                                    control={form.control}
                                    name="ttftThresholdMs"
                                    render={({ field }) => (
                                        <FormItem>
                                            <FormLabel className={configLabelClass}>TTFT 阈值</FormLabel>
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
                                                                numberFromInput(
                                                                    event.currentTarget.value,
                                                                    event.currentTarget.valueAsNumber,
                                                                ),
                                                            )
                                                        }
                                                        className={`${configInputClass} pr-11 tabular-nums`}
                                                    />
                                                    <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-[#596579]">ms</span>
                                                </div>
                                            </FormControl>
                                            <FormMessage className="text-xs text-[#FF7373]" />
                                        </FormItem>
                                    )}
                                />
                                <FormField
                                    control={form.control}
                                    name="queueThreshold"
                                    render={({ field }) => (
                                        <FormItem>
                                            <FormLabel className={configLabelClass}>队列阈值</FormLabel>
                                            <FormControl>
                                                <Input
                                                    type="number"
                                                    min="0"
                                                    step="any"
                                                    {...field}
                                                    value={Number.isNaN(field.value) ? '' : field.value}
                                                    onChange={(event) =>
                                                        field.onChange(
                                                            numberFromInput(
                                                                event.currentTarget.value,
                                                                event.currentTarget.valueAsNumber,
                                                            ),
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
                        </div>

                        <div className="rounded-lg border border-[#202B3A] bg-[#101010] p-3.5">
                            <div className="mb-4 flex items-center gap-2">
                                <div className="flex h-7 w-7 items-center justify-center rounded-md bg-emerald-500/10">
                                    <ArrowDownToLine className="h-3.5 w-3.5 text-[#57C894]" />
                                </div>
                                <div>
                                    <p className="text-xs font-medium text-[#D7D7D7]">缩容触发</p>
                                    <p className="text-[11px] text-[#596579]">回落至阈值时释放容量</p>
                                </div>
                            </div>
                            <div className="space-y-4">
                                <FormField
                                    control={form.control}
                                    name="ttftScaleDownThresholdMs"
                                    render={({ field }) => (
                                        <FormItem>
                                            <FormLabel className={configLabelClass}>TTFT 阈值</FormLabel>
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
                                                                numberFromInput(
                                                                    event.currentTarget.value,
                                                                    event.currentTarget.valueAsNumber,
                                                                ),
                                                            )
                                                        }
                                                        className={`${configInputClass} pr-11 tabular-nums`}
                                                    />
                                                    <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-[#596579]">ms</span>
                                                </div>
                                            </FormControl>
                                            <FormMessage className="text-xs text-[#FF7373]" />
                                        </FormItem>
                                    )}
                                />
                                <FormField
                                    control={form.control}
                                    name="queueScaleDownThreshold"
                                    render={({ field }) => (
                                        <FormItem>
                                            <FormLabel className={configLabelClass}>队列阈值</FormLabel>
                                            <FormControl>
                                                <Input
                                                    type="number"
                                                    min="0"
                                                    step="any"
                                                    {...field}
                                                    value={Number.isNaN(field.value) ? '' : field.value}
                                                    onChange={(event) =>
                                                        field.onChange(
                                                            numberFromInput(
                                                                event.currentTarget.value,
                                                                event.currentTarget.valueAsNumber,
                                                            ),
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
