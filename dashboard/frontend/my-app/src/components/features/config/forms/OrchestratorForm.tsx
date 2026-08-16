import { memo, useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Gauge, Timer } from 'lucide-react'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { orchestratorSchema, getOrchestratorPreview, type OrchestratorFormValues } from '@/lib/validations/orchestrator.schema'
import { useTenants } from '@/api/queries/configQueries'
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

interface OrchestratorFormProps {
    defaultValues: OrchestratorFormValues
    onSubmit: (data: OrchestratorFormValues) => Promise<void>
    submitLabel?: string
    onDirtyChange?: (dirty: boolean) => void
}

const getErrorMessage = (error: unknown): string =>
    error instanceof Error ? error.message : '保存失败，请稍后重试'

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
    const [submitError, setSubmitError] = useState('')
    const { orchestratorTemplates, addOrchestratorTemplate, removeOrchestratorTemplate } = useTemplateStore()
    const { data: tenants = [] } = useTenants()
    const { isDirty, isSubmitting } = form.formState

    useEffect(() => {
        form.reset(defaultValues)
        setSubmitError('')
    }, [defaultValues, form])

    useEffect(() => {
        onDirtyChange?.(isDirty)
    }, [isDirty, onDirtyChange])

    const submitForm = async (values: OrchestratorFormValues) => {
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
        addOrchestratorTemplate(name, form.getValues())
        return true
    }

    const loadTemplate = (template: ConfigTemplate<OrchestratorFormValues>) => {
        form.reset(template.data, { keepDefaultValues: true })
    }

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
                        <FormField
                            control={form.control}
                            name="tenantName"
                            render={({ field }) => (
                                <FormItem className="sm:col-span-2">
                                    <FormLabel className={configLabelClass}>关联租户</FormLabel>
                                    <Select value={field.value} onValueChange={field.onChange}>
                                        <FormControl>
                                            <SelectTrigger className={`${configInputClass} w-full`}>
                                                <SelectValue placeholder="选择租户" />
                                            </SelectTrigger>
                                        </FormControl>
                                        <SelectContent className="border-[#263244] bg-[#101722] text-[#EDEDED]">
                                            {tenants.map((tenant) => (
                                                <SelectItem
                                                    key={tenant.name}
                                                    value={tenant.name}
                                                    className="focus:bg-[#202B3A] focus:text-white"
                                                >
                                                    <span className="flex items-center gap-2">
                                                        <span>{tenant.displayName}</span>
                                                        <span className="font-mono text-[#596579]">{tenant.name}</span>
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
                            name="scaleUpCooldownSeconds"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel className={configLabelClass}>扩容冷却</FormLabel>
                                    <FormControl>
                                        <div className="relative">
                                            <Input
                                                type="number"
                                                min="0"
                                                step="1"
                                                {...field}
                                                value={Number.isNaN(field.value) ? '' : field.value}
                                                onChange={(event) =>
                                                    field.onChange(
                                                        numberFromInput(event.currentTarget.value, event.currentTarget.valueAsNumber),
                                                    )
                                                }
                                                className={`${configInputClass} pr-8 tabular-nums`}
                                            />
                                            <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-[#596579]">s</span>
                                        </div>
                                    </FormControl>
                                    <FormMessage className="text-xs text-[#FF7373]" />
                                </FormItem>
                            )}
                        />

                        <FormField
                            control={form.control}
                            name="scaleDownCooldownSeconds"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel className={configLabelClass}>缩容冷却</FormLabel>
                                    <FormControl>
                                        <div className="relative">
                                            <Input
                                                type="number"
                                                min="0"
                                                step="1"
                                                {...field}
                                                value={Number.isNaN(field.value) ? '' : field.value}
                                                onChange={(event) =>
                                                    field.onChange(
                                                        numberFromInput(event.currentTarget.value, event.currentTarget.valueAsNumber),
                                                    )
                                                }
                                                className={`${configInputClass} pr-8 tabular-nums`}
                                            />
                                            <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-[#596579]">s</span>
                                        </div>
                                    </FormControl>
                                    <FormMessage className="text-xs text-[#FF7373]" />
                                </FormItem>
                            )}
                        />
                    </div>
                </ConfigFormSection>

                <ConfigFormSection
                    title="副本策略"
                    description="定义租户副本的上下限与无流量时的缩容行为。"
                    action={<Gauge className="h-4 w-4 text-[#5E9EFF]" />}
                >
                    <div className="grid gap-4 sm:grid-cols-2">
                        <FormField
                            control={form.control}
                            name="minReplicas"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel className={configLabelClass}>最小副本数</FormLabel>
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

                        <FormField
                            control={form.control}
                            name="maxReplicas"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel className={configLabelClass}>最大副本数</FormLabel>
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
