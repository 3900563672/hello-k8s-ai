import { memo } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Activity, ArrowDownToLine, ArrowUpFromLine, Shield } from 'lucide-react'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { tenantSchema, getTenantPreview, type TenantFormValues } from '@/lib/validations/tenant.schema'
import { useTemplateStore } from '@/stores/templateSlice'
import type { TenantPriority } from '@/types/config.types'
import {
    ConfigFormSection,
    ConfigNumberField,
    ConfigTextField,
    FormSaveBar,
    LiveImpactSummary,
    TemplateActions,
    configInputClass,
    configLabelClass,
    useConfigForm,
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
    const { tenantTemplates, addTenantTemplate, removeTenantTemplate } = useTemplateStore()
    const { submitError, submitForm, saveTemplate, loadTemplate, isDirty, isSubmitting } = useConfigForm({
        form,
        defaultValues,
        onSubmit,
        onDirtyChange,
        addTemplate: addTenantTemplate,
    })

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
                <LiveImpactSummary fields={getTenantPreview(form.watch())} />

                <ConfigFormSection
                    title="身份与流量"
                    description="定义租户的显示信息、调度优先级和基准流量。"
                    action={<Shield className="h-4 w-4 text-[#5E9EFF]" />}
                >
                    <div className="grid gap-4 sm:grid-cols-2">
                        <ConfigTextField control={form.control} name="displayName" label="显示名称" formItemClass="sm:col-span-2" />

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

                        <ConfigNumberField control={form.control} name="qps" label="基准 QPS" min="0" step="any" unit="QPS" inputClass="pr-12"/>
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
                                    <p className="text-[14px] text-[#596579]">超过阈值时增加容量</p>
                                </div>
                            </div>
                            <div className="space-y-4">
                                <ConfigNumberField control={form.control} name="ttftThresholdMs" label="TTFT 阈值" unit="ms" inputClass="pr-11"/>
                                <ConfigNumberField control={form.control} name="queueThreshold" label="队列阈值" />
                            </div>
                        </div>

                        <div className="rounded-lg border border-[#202B3A] bg-[#101010] p-3.5">
                            <div className="mb-4 flex items-center gap-2">
                                <div className="flex h-7 w-7 items-center justify-center rounded-md bg-emerald-500/10">
                                    <ArrowDownToLine className="h-3.5 w-3.5 text-[#57C894]" />
                                </div>
                                <div>
                                    <p className="text-xs font-medium text-[#D7D7D7]">缩容触发</p>
                                    <p className="text-[14px] text-[#596579]">回落至阈值时释放容量</p>
                                </div>
                            </div>
                            <div className="space-y-4">
                                <ConfigNumberField control={form.control} name="ttftScaleDownThresholdMs" label="TTFT 阈值" unit="ms" inputClass="pr-11"/>
                                <ConfigNumberField control={form.control} name="queueScaleDownThreshold" label="队列阈值" />
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
