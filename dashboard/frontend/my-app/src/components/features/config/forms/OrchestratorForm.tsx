import { memo } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Gauge, Timer } from 'lucide-react'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Checkbox } from '@/components/ui/checkbox'
import { orchestratorSchema, getOrchestratorPreview, type OrchestratorFormValues } from '@/lib/validations/orchestrator.schema'
import { useTenants } from '@/api/queries/configQueries'
import { useTemplateStore } from '@/stores/templateSlice'
import {
    ConfigFormSection,
    ConfigNumberField,
    ConfigRefSelect,
    FormSaveBar,
    TemplateActions,
    useConfigForm,
} from './ConfigFormParts'

interface OrchestratorFormProps {
    defaultValues: OrchestratorFormValues
    onSubmit: (data: OrchestratorFormValues) => Promise<void>
    submitLabel?: string
    onDirtyChange?: (dirty: boolean) => void
}

export const OrchestratorForm = memo(function OrchestratorForm({
    defaultValues,
    onSubmit,
    submitLabel = '保存编排策略',
    onDirtyChange,
}: OrchestratorFormProps) {
    const form = useForm<OrchestratorFormValues>({
        resolver: zodResolver(orchestratorSchema),
        defaultValues,
        mode: 'onBlur',
    })
    const { orchestratorTemplates, addOrchestratorTemplate, removeOrchestratorTemplate } = useTemplateStore()
    const { data: tenants = [] } = useTenants()
    const { submitError, submitForm, saveTemplate, loadTemplate, isDirty, isSubmitting } = useConfigForm({
        form,
        defaultValues,
        onSubmit,
        onDirtyChange,
        addTemplate: addOrchestratorTemplate,
    })

    return (
        <Form {...form}>
            <form onSubmit={form.handleSubmit(submitForm)} className="space-y-4">
                <TemplateActions
                    typeLabel="编排策略"
                    templates={orchestratorTemplates}
                    onSave={saveTemplate}
                    onLoad={loadTemplate}
                    onDelete={removeOrchestratorTemplate}
                    getPreview={getOrchestratorPreview}
                />

                <ConfigFormSection
                    title="关联与冷却"
                    description="绑定租户并设置两次扩缩容之间的冷却时间。"
                    action={<Timer className="h-4 w-4 text-[#5E9EFF]" />}
                >
                    <div className="grid gap-4 sm:grid-cols-2">
                        <ConfigRefSelect
                            control={form.control}
                            name="tenantName"
                            label="关联租户"
                            options={tenants}
                            placeholder="选择租户"
                            formItemClass="sm:col-span-2"
                        />

                        <ConfigNumberField control={form.control} name="scaleUpCooldownSeconds" label="扩容冷却" min="0" unit="s" inputClass="pr-8"/>

                        <ConfigNumberField control={form.control} name="scaleDownCooldownSeconds" label="缩容冷却" min="0" unit="s" inputClass="pr-8"/>
                    </div>
                </ConfigFormSection>

                <ConfigFormSection
                    title="副本策略"
                    description="定义租户副本的上下限与无流量时的缩容行为。"
                    action={<Gauge className="h-4 w-4 text-[#5E9EFF]" />}
                >
                    <div className="grid gap-4 sm:grid-cols-2">
                        <ConfigNumberField control={form.control} name="minReplicas" label="最小副本数" />

                        <ConfigNumberField control={form.control} name="maxReplicas" label="最大副本数" min="0" description="填 0 表示不限制副本数（模拟器无网关，接受任意 QPS，扩到容量上限为止）" />

                        <ConfigNumberField control={form.control} name="maxScaleUpBatch" label="单次扩容步长" min="0" description="每轮扩容最多补的副本数；填 0 使用默认 10" />

                        <FormField
                            control={form.control}
                            name="allowScaleToZero"
                            render={({ field }) => (
                                <FormItem className="sm:col-span-2">
                                    <div className="flex items-start gap-3 rounded-lg border border-[#202B3A] bg-[#101010] p-3.5">
                                        <FormControl>
                                            <Checkbox
                                                checked={field.value}
                                                onCheckedChange={(value) => field.onChange(value === true)}
                                                className="mt-0.5 border-[#484848] data-[state=checked]:border-[#5B8CFF] data-[state=checked]:bg-[#5B8CFF]"
                                            />
                                        </FormControl>
                                        <div className="space-y-1">
                                            <FormLabel className="text-xs font-medium text-[#D7D7D7]">
                                                允许缩容到零
                                            </FormLabel>
                                            <FormDescription className="text-[11px] leading-5 text-[#596579]">
                                                仅当租户无流量时生效；关闭时至少保留最小副本数。
                                            </FormDescription>
                                        </div>
                                    </div>
                                    <FormMessage className="text-xs text-[#FF7373]" />
                                </FormItem>
                            )}
                        />
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
