import { memo } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { ShieldCheck } from 'lucide-react'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { policySchema, type PolicyFormValues } from '@/lib/validations/policy.schema'
import { useModels, useNodes, useTenants } from '@/api/queries/configQueries'
import type { PolicyKind } from '@/types/config.types'
import {
    ConfigFormSection,
    ConfigRefSelect,
    FormSaveBar,
    configInputClass,
    configLabelClass,
    useConfigForm,
} from './ConfigFormParts'

interface PolicyFormProps {
    defaultValues: PolicyFormValues
    onSubmit: (data: PolicyFormValues) => Promise<void>
    submitLabel?: string
    onDirtyChange?: (dirty: boolean) => void
}

const kindMeta: Record<PolicyKind, { label: string; description: string }> = {
    tenantModel: { label: '租户-模型', description: '决定租户能否使用某个模型。' },
    tenantNode: { label: '租户-节点', description: '决定租户可调度的计算节点。' },
    modelNode: { label: '模型-节点', description: '决定模型可运行的节点，未配置时沿用租户节点范围。' },
}

export const PolicyForm = memo(function PolicyForm({
    defaultValues,
    onSubmit,
    submitLabel = '保存策略',
    onDirtyChange,
}: PolicyFormProps) {
    const form = useForm<PolicyFormValues>({
        resolver: zodResolver(policySchema),
        defaultValues,
        mode: 'onBlur',
    })
    const { data: tenants = [] } = useTenants()
    const { data: models = [] } = useModels()
    const { data: nodes = [] } = useNodes()
    const kind = form.watch('kind')
    const { submitError, submitForm, isDirty, isSubmitting } = useConfigForm({
        form,
        defaultValues,
        onSubmit,
        onDirtyChange,
    })

    const meta = kindMeta[kind]

    return (
        <Form {...form}>
            <form onSubmit={form.handleSubmit(submitForm)} className="space-y-4">
                <ConfigFormSection
                    title={`${meta.label}策略`}
                    description={meta.description}
                    action={<ShieldCheck className="h-4 w-4 text-[#5E9EFF]" />}
                >
                    <div className="grid gap-4 sm:grid-cols-2">
                        {kind === 'tenantModel' && (
                            <>
                                <ConfigRefSelect control={form.control} name="tenantName" label="关联租户" options={tenants} placeholder="选择租户" />
                                <ConfigRefSelect control={form.control} name="modelName" label="关联模型" options={models} placeholder="选择模型" />
                            </>
                        )}
                        {kind === 'tenantNode' && (
                            <>
                                <ConfigRefSelect control={form.control} name="tenantName" label="关联租户" options={tenants} placeholder="选择租户" />
                                <ConfigRefSelect control={form.control} name="nodeName" label="关联节点" options={nodes} placeholder="选择节点" />
                            </>
                        )}
                        {kind === 'modelNode' && (
                            <>
                                <ConfigRefSelect control={form.control} name="modelName" label="关联模型" options={models} placeholder="选择模型" />
                                <ConfigRefSelect control={form.control} name="nodeName" label="关联节点" options={nodes} placeholder="选择节点" />
                            </>
                        )}
                        <FormField
                            control={form.control}
                            name="effect"
                            render={({ field }) => (
                                <FormItem className="sm:col-span-2">
                                    <FormLabel className={configLabelClass}>策略效果</FormLabel>
                                    <Select value={field.value} onValueChange={field.onChange}>
                                        <FormControl>
                                            <SelectTrigger className={`${configInputClass} w-full`}>
                                                <SelectValue placeholder="选择效果" />
                                            </SelectTrigger>
                                        </FormControl>
                                        <SelectContent className="border-[#263244] bg-[#101722] text-[#EDEDED]">
                                            <SelectItem value="Allow" className="focus:bg-[#202B3A] focus:text-white">
                                                <span className="flex items-center gap-2">
                                                    <span className="text-[#57C894]">Allow</span>
                                                    <span className="text-[#596579]">允许使用，参与调度与扩容</span>
                                                </span>
                                            </SelectItem>
                                            <SelectItem value="Deny" className="focus:bg-[#202B3A] focus:text-white">
                                                <span className="flex items-center gap-2">
                                                    <span className="text-[#FF7373]">Deny</span>
                                                    <span className="text-[#596579]">禁止使用，优先级高于 Allow</span>
                                                </span>
                                            </SelectItem>
                                        </SelectContent>
                                    </Select>
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