import { memo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { ChevronDown, Gauge, Timer } from 'lucide-react'
import { Form } from '@/components/ui/form'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { modelSchema, getModelPreview, type ModelFormValues } from '@/lib/validations/model.schema'
import { useTemplateStore } from '@/stores/templateSlice'
import {
    ConfigFormSection,
    ConfigNumberField,
    ConfigTextField,
    FormSaveBar,
    TemplateActions,
    useConfigForm,
} from './ConfigFormParts'

interface ModelFormProps {
    defaultValues: ModelFormValues
    onSubmit: (data: ModelFormValues) => Promise<void>
    submitLabel?: string
    onDirtyChange?: (dirty: boolean) => void
}

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
    const { modelTemplates, addModelTemplate, removeModelTemplate } = useTemplateStore()
    const { submitError, submitForm, saveTemplate, loadTemplate, isDirty, isSubmitting } = useConfigForm({
        form,
        defaultValues,
        onSubmit,
        onDirtyChange,
        addTemplate: addModelTemplate,
        afterLoadTemplate: () => setAdvancedOpen(true),
    })

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
                        <ConfigTextField control={form.control} name="displayName" label="显示名称" formItemClass="sm:col-span-2" />

                        <ConfigNumberField control={form.control} name="gpuUnits" label="显存需求" unit="G" inputClass="pr-10"/>

                        <ConfigNumberField control={form.control} name="maxConcurrency" label="最大并发" />

                        <ConfigNumberField control={form.control} name="absoluteScore" label="能力基准分" formItemClass="sm:col-span-2" description="单个预热副本的理想能力分，Orchestrator 使用该值比较扩容候选。"/>

                        <ConfigNumberField control={form.control} name="coldStartMs" label="冷启动时间" min="0" step="any" unit="ms" inputClass="pr-12" formItemClass="sm:col-span-2"/>
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
                                <ConfigNumberField control={form.control} name="performance.prefillBaseMs" label="Prefill 基础延时" unit="ms" inputClass="pr-11" unitClass="text-[11px]"/>
                                <ConfigNumberField control={form.control} name="performance.prefillPerTokenUs" label="Prefill / Token" unit="µs" inputClass="pr-9" unitClass="text-[11px]"/>
                                <ConfigNumberField control={form.control} name="performance.decodePerTokenMs" label="Decode / Token" unit="ms" inputClass="pr-11" unitClass="text-[11px]"/>
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
