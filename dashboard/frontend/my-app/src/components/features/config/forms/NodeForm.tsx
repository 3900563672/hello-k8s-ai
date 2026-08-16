import { memo } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Cpu } from 'lucide-react'
import { Form } from '@/components/ui/form'
import { nodeSchema, getNodePreview, type NodeFormValues } from '@/lib/validations/node.schema'
import { useTemplateStore } from '@/stores/templateSlice'
import {
    ConfigFormSection,
    ConfigNumberField,
    ConfigTextField,
    FormSaveBar,
    TemplateActions,
    useConfigForm,
} from './ConfigFormParts'

interface NodeFormProps {
    defaultValues: NodeFormValues
    onSubmit: (data: NodeFormValues) => Promise<void>
    submitLabel?: string
    onDirtyChange?: (dirty: boolean) => void
}

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
    const { nodeTemplates, addNodeTemplate, removeNodeTemplate } = useTemplateStore()
    const { submitError, submitForm, saveTemplate, loadTemplate, isDirty, isSubmitting } = useConfigForm({
        form,
        defaultValues,
        onSubmit,
        onDirtyChange,
        addTemplate: addNodeTemplate,
    })

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
                        <ConfigTextField control={form.control} name="displayName" label="显示名称" formItemClass="sm:col-span-2" />

                        <ConfigNumberField control={form.control} name="gpu" label="总显存" min="0" step="any" unit="G" inputClass="pr-10"/>

                        <ConfigNumberField control={form.control} name="maxConcurrency" label="最大并发" />
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
