import { useEffect, useState, type ReactNode } from 'react'
import { AlertCircle, Check, FolderOpen, Loader2, Save, Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
    FormControl,
    FormDescription,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from '@/components/ui/form'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { Control, FieldValues, Path, UseFormReturn } from 'react-hook-form'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import { TemplateLibraryDialog } from '@/components/shared/dialogs/TemplateLibraryDialog'
import type { ConfigTemplate, PreviewConfig } from '@/types/config.types'

interface ConfigFormSectionProps {
    title: string
    description: string
    children: ReactNode
    action?: ReactNode
}

export function ConfigFormSection({ title, description, children, action }: ConfigFormSectionProps) {
    return (
        <section className="rounded-xl border border-[#232323] bg-[#0B1018]">
            <header className="flex items-start justify-between gap-4 border-b border-[#1B2634] px-4 py-3.5">
                <div>
                    <h3 className="text-sm font-medium text-[#EDEDED]">{title}</h3>
                    <p className="mt-1 text-xs leading-5 text-[#6F6F6F]">{description}</p>
                </div>
                {action}
            </header>
            <div className="p-4">{children}</div>
        </section>
    )
}

interface TemplateActionsProps<T> {
    typeLabel: string
    templates: ConfigTemplate<T>[]
    onSave: (name: string) => boolean | Promise<boolean>
    onLoad: (template: ConfigTemplate<T>) => void
    onDelete: (id: string) => void
    getPreview: (data: T) => PreviewConfig
}

export function TemplateActions<T>({
    typeLabel,
    templates,
    onSave,
    onLoad,
    onDelete,
    getPreview,
}: TemplateActionsProps<T>) {
    const [saveOpen, setSaveOpen] = useState(false)
    const [libraryOpen, setLibraryOpen] = useState(false)
    const [templateName, setTemplateName] = useState('')
    const [saving, setSaving] = useState(false)

    const confirmSave = async () => {
        const name = templateName.trim()
        if (!name || saving) return
        setSaving(true)
        const saved = await onSave(name)
        setSaving(false)
        if (!saved) return
        setTemplateName('')
        setSaveOpen(false)
    }

    return (
        <>
            <div className="flex flex-col gap-3 rounded-xl border border-[#232323] bg-[linear-gradient(135deg,rgba(0,112,243,0.08),rgba(10,10,10,0.2)_55%)] p-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex min-w-0 items-center gap-3">
                    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-blue-500/20 bg-blue-500/10">
                        <Sparkles className="h-3.5 w-3.5 text-[#67A6FF]" />
                    </div>
                    <div className="min-w-0">
                        <p className="text-xs font-medium text-[#DADADA]">配置模板</p>
                        <p className="mt-0.5 truncate text-[11px] text-[#6F6F6F]">
                            保存常用参数，快速复用到其他{typeLabel} · {templates.length} 个模板
                        </p>
                    </div>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                    <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => setLibraryOpen(true)}
                        className="h-8 gap-1.5 border-[#303C50] bg-[#131313] px-2.5 text-xs text-[#B7C2D1] hover:bg-[#1E1E1E] hover:text-white"
                    >
                        <FolderOpen className="h-3.5 w-3.5" />
                        模板库
                    </Button>
                    <Button
                        type="button"
                        size="sm"
                        onClick={() => setSaveOpen(true)}
                        className="h-8 gap-1.5 bg-[#F4F4F5] px-2.5 text-xs text-[#111] hover:bg-white"
                    >
                        <Save className="h-3.5 w-3.5" />
                        存为模板
                    </Button>
                </div>
            </div>

            <Dialog
                open={saveOpen}
                onOpenChange={(open) => {
                    setSaveOpen(open)
                    if (!open) setTemplateName('')
                }}
            >
                <DialogContent className="max-w-md border-[#263244] bg-[#111] p-0 text-[#F4F4F5] shadow-2xl">
                    <DialogHeader className="px-6 pb-2 pt-6 text-left">
                        <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg border border-blue-500/20 bg-blue-500/10">
                            <Save className="h-4 w-4 text-[#67A6FF]" />
                        </div>
                        <DialogTitle className="text-base">保存{typeLabel}模板</DialogTitle>
                        <DialogDescription className="leading-6 text-[#858585]">
                            当前表单中的参数会保存为可复用模板，不会覆盖现有资源。
                        </DialogDescription>
                    </DialogHeader>
                    <div className="px-6 py-3">
                        <label htmlFor={`${typeLabel}-template-name`} className="mb-2 block text-xs font-medium text-[#A0A0A0]">
                            模板名称
                        </label>
                        <Input
                            id={`${typeLabel}-template-name`}
                            autoFocus
                            value={templateName}
                            onChange={(event) => setTemplateName(event.target.value)}
                            onKeyDown={(event) => {
                                if (event.key === 'Enter') {
                                    event.preventDefault()
                                    void confirmSave()
                                }
                            }}
                            placeholder={`例如：高性能${typeLabel}配置`}
                            className="border-[#303C50] bg-[#0A0A0A] text-[#F2F2F2] placeholder:text-[#555] focus-visible:border-[#5B8CFF]/60 focus-visible:ring-2 focus-visible:ring-[#5B8CFF]/15"
                        />
                    </div>
                    <DialogFooter className="border-t border-[#202B3A] bg-[#080C12] px-6 py-4">
                        <Button
                            type="button"
                            variant="outline"
                            disabled={saving}
                            onClick={() => setSaveOpen(false)}
                            className="border-[#303C50] bg-[#141C28] text-[#D4D4D4] hover:bg-[#222] hover:text-white"
                        >
                            取消
                        </Button>
                        <Button
                            type="button"
                            disabled={!templateName.trim() || saving}
                            onClick={() => void confirmSave()}
                            className="min-w-24 bg-[#5B8CFF] text-white hover:bg-[#70A0FF]"
                        >
                            {saving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                            保存模板
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            <TemplateLibraryDialog
                open={libraryOpen}
                onOpenChange={setLibraryOpen}
                templates={templates}
                typeLabel={typeLabel}
                onLoad={(template: ConfigTemplate<T>) => {
                    onLoad(template)
                    setLibraryOpen(false)
                }}
                onDelete={onDelete}
                getPreview={getPreview}
            />
        </>
    )
}

interface FormSaveBarProps {
    dirty: boolean
    submitting: boolean
    error: string
    submitLabel: string
}

export function FormSaveBar({ dirty, submitting, error, submitLabel }: FormSaveBarProps) {
    return (
        <div className="sticky -bottom-5 z-10 mt-5 rounded-xl border border-[#263244] bg-[#0D131C]/95 p-3 shadow-[0_-12px_36px_rgba(0,0,0,0.35)] backdrop-blur">
            {error && (
                <div className="mb-3 flex items-start gap-2 rounded-md border border-red-500/20 bg-red-500/10 px-3 py-2 text-xs leading-5 text-[#FF8A8A]">
                    <AlertCircle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                    {error}
                </div>
            )}
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex items-center gap-2 text-xs">
                    {dirty ? (
                        <>
                            <span className="h-1.5 w-1.5 rounded-full bg-[#F5A524]" />
                            <span className="text-[#B7A47A]">有未保存的修改</span>
                        </>
                    ) : (
                        <>
                            <Check className="h-3.5 w-3.5 text-[#57C894]" />
                            <span className="text-[#747474]">当前配置已保存</span>
                        </>
                    )}
                </div>
                <Button
                    type="submit"
                    disabled={!dirty || submitting}
                    className="h-9 min-w-28 bg-[#5B8CFF] px-4 text-sm font-medium text-white hover:bg-[#70A0FF] disabled:bg-[#202B3A] disabled:text-[#596579]"
                >
                    {submitting ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Save className="mr-2 h-4 w-4" />}
                    {submitting ? '正在保存' : submitLabel}
                </Button>
            </div>
        </div>
    )
}

export const configInputClass =
    'h-9 border-[#2B2B2B] bg-[#0D131C] text-[#ECECEC] placeholder:text-[#555] focus-visible:border-[#5B8CFF]/60 focus-visible:ring-2 focus-visible:ring-[#5B8CFF]/15'

export const configLabelClass = 'text-xs font-medium text-[#9AA7B9]'

export const numberFromInput = (value: string, valueAsNumber: number): number =>
    value === '' ? Number.NaN : valueAsNumber
const getErrorMessage = (error: unknown): string =>
    error instanceof Error ? error.message : '保存失败，请稍后重试'

interface UseConfigFormOptions<TFieldValues extends FieldValues> {
    form: UseFormReturn<TFieldValues>
    defaultValues: TFieldValues
    onSubmit: (data: TFieldValues) => Promise<void>
    onDirtyChange?: (dirty: boolean) => void
    addTemplate?: (name: string, data: TFieldValues) => void
    afterLoadTemplate?: (template: ConfigTemplate<TFieldValues>) => void
}

// useConfigForm 收敛各配置表单共用的提交、模板与脏状态逻辑。
export function useConfigForm<TFieldValues extends FieldValues>({
    form,
    defaultValues,
    onSubmit,
    onDirtyChange,
    addTemplate,
    afterLoadTemplate,
}: UseConfigFormOptions<TFieldValues>) {
    const [submitError, setSubmitError] = useState('')
    const { isDirty, isSubmitting } = form.formState

    useEffect(() => {
        form.reset(defaultValues)
        setSubmitError('')
    }, [defaultValues, form])

    useEffect(() => {
        onDirtyChange?.(isDirty)
    }, [isDirty, onDirtyChange])

    const submitForm = async (values: TFieldValues) => {
        setSubmitError('')
        try {
            await onSubmit(values)
            form.reset(values)
        } catch (error) {
            setSubmitError(getErrorMessage(error))
        }
    }

    const saveTemplate = async (name: string) => {
        if (!addTemplate) return false
        const valid = await form.trigger()
        if (!valid) return false
        addTemplate(name, form.getValues())
        return true
    }

    const loadTemplate = (template: ConfigTemplate<TFieldValues>) => {
        form.reset(template.data, { keepDefaultValues: true })
        afterLoadTemplate?.(template)
    }

    return { submitError, submitForm, saveTemplate, loadTemplate, isDirty, isSubmitting }
}

interface ConfigFieldBaseProps<TFieldValues extends FieldValues> {
    control: Control<TFieldValues>
    name: Path<TFieldValues>
    label: string
    formItemClass?: string
}

// ConfigTextField 渲染单行文本输入字段（显示名称等）。
export function ConfigTextField<TFieldValues extends FieldValues>({
    control,
    name,
    label,
    formItemClass,
}: ConfigFieldBaseProps<TFieldValues>) {
    return (
        <FormField
            control={control}
            name={name}
            render={({ field }) => (
                <FormItem className={formItemClass}>
                    <FormLabel className={configLabelClass}>{label}</FormLabel>
                    <FormControl>
                        <Input {...field} autoComplete="off" className={configInputClass} />
                    </FormControl>
                    <FormMessage className="text-xs text-[#FF7373]" />
                </FormItem>
            )}
        />
    )
}

interface ConfigNumberFieldProps<TFieldValues extends FieldValues> extends ConfigFieldBaseProps<TFieldValues> {
    min?: string
    step?: string
    unit?: string
    inputClass?: string
    unitClass?: string
    description?: string
}

// ConfigNumberField 渲染带单位后缀的数字输入字段，空值以 NaN 进入表单校验。
export function ConfigNumberField<TFieldValues extends FieldValues>({
    control,
    name,
    label,
    min = '1',
    step = '1',
    unit,
    inputClass,
    unitClass = 'text-xs',
    formItemClass,
    description,
}: ConfigNumberFieldProps<TFieldValues>) {
    const inputClassName = [configInputClass, inputClass, 'tabular-nums'].filter(Boolean).join(' ')
    return (
        <FormField
            control={control}
            name={name}
            render={({ field }) => (
                <FormItem className={formItemClass}>
                    <FormLabel className={configLabelClass}>{label}</FormLabel>
                    <FormControl>
                        <div className="relative">
                            <Input
                                type="number"
                                min={min}
                                step={step}
                                {...field}
                                value={Number.isNaN(field.value) ? '' : field.value}
                                onChange={(event) =>
                                    field.onChange(
                                        numberFromInput(event.currentTarget.value, event.currentTarget.valueAsNumber),
                                    )
                                }
                                className={inputClassName}
                            />
                            {unit && (
                                <span className={`pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 ${unitClass} text-[#596579]`}>
                                    {unit}
                                </span>
                            )}
                        </div>
                    </FormControl>
                    {description && (
                        <FormDescription className="text-xs leading-5 text-[#596579]">{description}</FormDescription>
                    )}
                    <FormMessage className="text-xs text-[#FF7373]" />
                </FormItem>
            )}
        />
    )
}

interface ConfigRefSelectProps<TFieldValues extends FieldValues> extends ConfigFieldBaseProps<TFieldValues> {
    options: Array<{ name: string; displayName: string }>
    placeholder: string
}

// ConfigRefSelect 渲染引用型下拉选择（租户/模型/节点）。
export function ConfigRefSelect<TFieldValues extends FieldValues>({
    control,
    name,
    label,
    options,
    placeholder,
    formItemClass,
}: ConfigRefSelectProps<TFieldValues>) {
    return (
        <FormField
            control={control}
            name={name}
            render={({ field }) => (
                <FormItem className={formItemClass}>
                    <FormLabel className={configLabelClass}>{label}</FormLabel>
                    <Select value={field.value || undefined} onValueChange={field.onChange}>
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
            )}
        />
    )
}
