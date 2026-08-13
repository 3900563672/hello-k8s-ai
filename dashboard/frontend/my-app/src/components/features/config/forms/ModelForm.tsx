import { memo, useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { ChevronDown, Gauge, Timer } from 'lucide-react'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { modelSchema, getModelPreview, type ModelFormValues } from '@/lib/validations/model.schema'
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

interface ModelFormProps {
    defaultValues: ModelFormValues
    onSubmit: (data: ModelFormValues) => Promise<void>
    submitLabel?: string
    onDirtyChange?: (dirty: boolean) => void
}

const getErrorMessage = (error: unknown): string =>
    error instanceof Error ? error.message : '保存失败，请稍后重试'

export const ModelForm = memo(function ModelForm({
    defaultValues,
    onSubmit,
    submitLabel = '保存模型',
    onDirtyChange,
}: ModelFormProps) {
    const form = useForm<ModelFormValues>({
        resolver: zodResolver(modelSchema),
        defaultValues,
        mode: 'onBlur',
    })
    const [advancedOpen, setAdvancedOpen] = useState(false)
    const [submitError, setSubmitError] = useState('')
    const { modelTemplates, addModelTemplate, removeModelTemplate } = useTemplateStore()
    const { isDirty, isSubmitting } = form.formState

    useEffect(() => {
        form.reset(defaultValues)
        setSubmitError('')
    }, [defaultValues, form])

    useEffect(() => {
        onDirtyChange?.(isDirty)
    }, [isDirty, onDirtyChange])

    const submitForm = async (values: ModelFormValues) => {
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
        addModelTemplate(name, form.getValues())
        return true
    }

    const loadTemplate = (template: ConfigTemplate<ModelFormValues>) => {
        form.reset(template.data, { keepDefaultValues: true })
        setAdvancedOpen(true)
    }

    return (
        <Form {...form}>
            <form onSubmit={form.handleSubmit(submitForm)} className="space-y-4">
                <TemplateActions
                    typeLabel="模型"
                    templates={modelTemplates}
                    onSave={saveTemplate}
                    onLoad={loadTemplate}
                    onDelete={removeModelTemplate}
                    getPreview={getModelPreview}
                />

                <ConfigFormSection
                    title="基础配置"
                    description="定义模型的显示信息、资源需求和承载能力。"
                    action={<Gauge className="h-4 w-4 text-[#5E9EFF]" />}
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
                            name="gpuUnits"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel className={configLabelClass}>显存需求</FormLabel>
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

                        <FormField
                            control={form.control}
                            name="coldStartMs"
                            render={({ field }) => (
                                <FormItem className="sm:col-span-2">
                                    <FormLabel className={configLabelClass}>冷启动时间</FormLabel>
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
                                            <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-[#596579]">ms</span>
                                        </div>
                                    </FormControl>
                                    <FormMessage className="text-xs text-[#FF7373]" />
                                </FormItem>
                            )}
                        />
                    </div>
                </ConfigFormSection>

                <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
                    <ConfigFormSection
                        title="性能画像"
                        description="用于模拟 Prefill 和 Decode 阶段的推理延时。"
                        action={
                            <CollapsibleTrigger asChild>
                                <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    className="h-7 gap-1 px-2 text-xs text-[#8A8A8A] hover:bg-[#1B1B1B] hover:text-white"
                                >
                                    {advancedOpen ? '收起' : '展开'}
                                    <ChevronDown className={`h-3.5 w-3.5 transition-transform ${advancedOpen ? 'rotate-180' : ''}`} />
                                </Button>
                            </CollapsibleTrigger>
                        }
                    >
                        <CollapsibleContent>
                            <div className="mb-4 flex items-center gap-2 rounded-md border border-[#202B3A] bg-[#101010] px-3 py-2 text-xs leading-5 text-[#727272]">
                                <Timer className="h-3.5 w-3.5 shrink-0 text-[#6E9DDD]" />
                                延时参数只参与本地调度模拟，可按实测数据持续校准。
                            </div>
                            <div className="grid gap-4 sm:grid-cols-3">
                                <FormField
                                    control={form.control}
                                    name="performance.prefillBaseMs"
                                    render={({ field }) => (
                                        <FormItem>
                                            <FormLabel className={configLabelClass}>Prefill 基础延时</FormLabel>
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
                                                    <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-[11px] text-[#596579]">ms</span>
                                                </div>
                                            </FormControl>
                                            <FormMessage className="text-xs text-[#FF7373]" />
                                        </FormItem>
                                    )}
                                />
                                <FormField
                                    control={form.control}
                                    name="performance.prefillPerTokenUs"
                                    render={({ field }) => (
                                        <FormItem>
                                            <FormLabel className={configLabelClass}>Prefill / Token</FormLabel>
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
                                                        className={`${configInputClass} pr-9 tabular-nums`}
                                                    />
                                                    <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-[11px] text-[#596579]">µs</span>
                                                </div>
                                            </FormControl>
                                            <FormMessage className="text-xs text-[#FF7373]" />
                                        </FormItem>
                                    )}
                                />
                                <FormField
                                    control={form.control}
                                    name="performance.decodePerTokenMs"
                                    render={({ field }) => (
                                        <FormItem>
                                            <FormLabel className={configLabelClass}>Decode / Token</FormLabel>
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
                                                    <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-[11px] text-[#596579]">ms</span>
                                                </div>
                                            </FormControl>
                                            <FormMessage className="text-xs text-[#FF7373]" />
                                        </FormItem>
                                    )}
                                />
                            </div>
                        </CollapsibleContent>
                    </ConfigFormSection>
                </Collapsible>

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
