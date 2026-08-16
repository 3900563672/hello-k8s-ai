import { memo, useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { ShieldCheck } from 'lucide-react'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { policySchema, type PolicyFormValues } from '@/lib/validations/policy.schema'
import { useModels, useNodes, useTenants } from '@/api/queries/configQueries'
import type { PolicyKind } from '@/types/config.types'
import { ConfigFormSection, FormSaveBar, configInputClass, configLabelClass } from './ConfigFormParts'

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

const getErrorMessage = (error: unknown): string =>
    error instanceof Error ? error.message : '保存失败，请稍后重试'

const RefSelect = ({
    label,
    value,
    onChange,
    options,
    placeholder,
}: {
    label: string
    value: string
    onChange: (value: string) => void
    options: Array<{ name: string; displayName: string }>
    placeholder: string
}) => (
    <FormItem>
        <FormLabel className={configLabelClass}>{label}</FormLabel>
        <Select value={value || undefined} onValueChange={onChange}>
            <FormControl>
                <SelectTrigger className={`${configInputClass} w-full`}>
                    <SelectValue placeholder={placeholder} />
                </SelectTrigger>
            </FormControl>
            <SelectContent className="border-[#263244] bg-[#101722] text-[#EDEDED]">
                {options.map((option) => (
                    <SelectItem
                        key={option.name}
                        value={option.name}
                        className="focus:bg-[#202B3A] focus:text-white"
                    >
                        <span className="flex items-center gap-2">
                            <span>{option.displayName}</span>
                            <span className="font-mono text-[#596579]">{option.name}</span>
                        </span>
                    </SelectItem>
                ))}
            </SelectContent>
        </Select>
        <FormMessage className="text-xs text-[#FF7373]" />
    </FormItem>
)

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
    const [submitError, setSubmitError] = useState('')
    const { data: tenants = [] } = useTenants()
    const { data: models = [] } = useModels()
    const { data: nodes = [] } = useNodes()
    const { isDirty, isSubmitting } = form.formState
    const kind = form.watch('kind')

    useEffect(() => {
        form.reset(defaultValues)
        setSubmitError('')
    }, [defaultValues, form])

    useEffect(() => {
        onDirtyChange?.(isDirty)
    }, [isDirty, onDirtyChange])

    const submitForm = async (values: PolicyFormValues) => {
        setSubmitError('')
        try {
            await onSubmit(values)
            form.reset(values)
        } catch (error) {
            setSubmitError(getErrorMessage(error))
        }
    }

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
                                <FormField
                                    control={form.control}
                                    name="tenantName"
                                    render={({ field }) => (
                                        <RefSelect
                                            label="关联租户"
                                            value={field.value}
                                            onChange={field.onChange}
                                            options={tenants}
                                            placeholder="选择租户"
                                        />
                                    )}
                                />
                                <FormField
                                    control={form.control}
                                    name="modelName"
                                    render={({ field }) => (
                                        <RefSelect
                                            label="关联模型"
                                            value={field.value}
                                            onChange={field.onChange}
                                            options={models}
                                            placeholder="选择模型"
                                        />
                                    )}
                                />
                            </>
                        )}
                        {kind === 'tenantNode' && (
                            <>
                                <FormField
                                    control={form.control}
                                    name="tenantName"
                                    render={({ field }) => (
                                        <RefSelect
                                            label="关联租户"
                                            value={field.value}
                                            onChange={field.onChange}
                                            options={tenants}
                                            placeholder="选择租户"
                                        />
                                    )}
                                />
                                <FormField
                                    control={form.control}
                                    name="nodeName"
                                    render={({ field }) => (
                                        <RefSelect
                                            label="关联节点"
                                            value={field.value}
                                            onChange={field.onChange}
                                            options={nodes}
                                            placeholder="选择节点"
                                        />
                                    )}
                                />
                            </>
                        )}
                        {kind === 'modelNode' && (
                            <>
                                <FormField
                                    control={form.control}
                                    name="modelName"
                                    render={({ field }) => (
                                        <RefSelect
                                            label="关联模型"
                                            value={field.value}
                                            onChange={field.onChange}
                                            options={models}
                                            placeholder="选择模型"
                                        />
                                    )}
                                />
                                <FormField
                                    control={form.control}
                                    name="nodeName"
                                    render={({ field }) => (
                                        <RefSelect
                                            label="关联节点"
                                            value={field.value}
                                            onChange={field.onChange}
                                            options={nodes}
                                            placeholder="选择节点"
                                        />
                                    )}
                                />
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