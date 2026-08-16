import { useRef } from 'react'
import { Loader2, Plus, ShieldCheck } from 'lucide-react'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { Model, Node, PolicyEffect, PolicyKind, Tenant } from '@/types/config.types'

interface PolicyCreateDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    kind: PolicyKind
    onKindChange: (kind: PolicyKind) => void
    tenantName: string
    onTenantNameChange: (value: string) => void
    modelName: string
    onModelNameChange: (value: string) => void
    nodeName: string
    onNodeNameChange: (value: string) => void
    effect: PolicyEffect
    onEffectChange: (effect: PolicyEffect) => void
    identifierPreview: string
    tenants: Tenant[]
    models: Model[]
    nodes: Node[]
    pending?: boolean
    error?: string
    onConfirm: () => void
}

const kindOptions: Array<{ value: PolicyKind; label: string; description: string }> = [
    { value: 'tenantModel', label: '租户-模型', description: '租户能否使用某个模型' },
    { value: 'tenantNode', label: '租户-节点', description: '租户可调度的计算节点' },
    { value: 'modelNode', label: '模型-节点', description: '模型可运行的节点范围' },
]

const selectClass =
    'h-9 border-[#303C50] bg-[#0A0A0A] text-[#F2F2F2] placeholder:text-[#555] focus-visible:border-[#5B8CFF]/60 focus-visible:ring-2 focus-visible:ring-[#5B8CFF]/15'

const RefSelect = ({
    id,
    label,
    value,
    onChange,
    options,
    placeholder,
}: {
    id: string
    label: string
    value: string
    onChange: (value: string) => void
    options: Array<{ name: string; displayName: string }>
    placeholder: string
}) => (
    <div>
        <Label htmlFor={id} className="text-xs font-medium text-[#A0A0A0]">
            {label}
        </Label>
        <Select value={value || undefined} onValueChange={onChange}>
            <SelectTrigger id={id} className={`mt-2 w-full ${selectClass}`}>
                <SelectValue placeholder={placeholder} />
            </SelectTrigger>
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
    </div>
)

export function PolicyCreateDialog({
    open,
    onOpenChange,
    kind,
    onKindChange,
    tenantName,
    onTenantNameChange,
    modelName,
    onModelNameChange,
    nodeName,
    onNodeNameChange,
    effect,
    onEffectChange,
    identifierPreview,
    tenants,
    models,
    nodes,
    pending = false,
    error = '',
    onConfirm,
}: PolicyCreateDialogProps) {
    const confirmRef = useRef<HTMLButtonElement>(null)
    const ready =
        (kind === 'tenantModel' && Boolean(tenantName) && Boolean(modelName)) ||
        (kind === 'tenantNode' && Boolean(tenantName) && Boolean(nodeName)) ||
        (kind === 'modelNode' && Boolean(modelName) && Boolean(nodeName))

    return (
        <Dialog open={open} onOpenChange={(nextOpen) => !pending && onOpenChange(nextOpen)}>
            <DialogContent
                className="max-w-md border-[#263244] bg-[#111] p-0 text-[#F4F4F5] shadow-2xl"
                onOpenAutoFocus={(event) => {
                    event.preventDefault()
                    confirmRef.current?.focus()
                }}
            >
                <DialogHeader className="px-6 pb-2 pt-6 text-left">
                    <div className="mb-2 flex h-10 w-10 items-center justify-center rounded-xl border border-blue-500/20 bg-blue-500/10">
                        <ShieldCheck className="h-4.5 w-4.5 text-[#67A6FF]" />
                    </div>
                    <DialogTitle className="text-base font-semibold">新建策略</DialogTitle>
                    <DialogDescription className="leading-6 text-[#858585]">
                        定义租户、模型与节点之间的访问关系；Controller 会依据策略拉起对应的 Simulator 工作负载。
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-3 px-6 py-3">
                    <div>
                        <Label htmlFor="create-policy-kind" className="text-xs font-medium text-[#A0A0A0]">
                            策略类型
                        </Label>
                        <Select value={kind} onValueChange={(value) => onKindChange(value as PolicyKind)}>
                            <SelectTrigger id="create-policy-kind" className={`mt-2 w-full ${selectClass}`}>
                                <SelectValue placeholder="选择策略类型" />
                            </SelectTrigger>
                            <SelectContent className="border-[#263244] bg-[#101722] text-[#EDEDED]">
                                {kindOptions.map((option) => (
                                    <SelectItem
                                        key={option.value}
                                        value={option.value}
                                        className="focus:bg-[#202B3A] focus:text-white"
                                    >
                                        <span className="flex items-center gap-2">
                                            <span>{option.label}</span>
                                            <span className="text-[#596579]">{option.description}</span>
                                        </span>
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>

                    {kind === 'tenantModel' && (
                        <div className="grid grid-cols-2 gap-3">
                            <RefSelect
                                id="create-policy-tenant"
                                label="租户"
                                value={tenantName}
                                onChange={onTenantNameChange}
                                options={tenants}
                                placeholder="选择租户"
                            />
                            <RefSelect
                                id="create-policy-model"
                                label="模型"
                                value={modelName}
                                onChange={onModelNameChange}
                                options={models}
                                placeholder="选择模型"
                            />
                        </div>
                    )}
                    {kind === 'tenantNode' && (
                        <div className="grid grid-cols-2 gap-3">
                            <RefSelect
                                id="create-policy-tenant"
                                label="租户"
                                value={tenantName}
                                onChange={onTenantNameChange}
                                options={tenants}
                                placeholder="选择租户"
                            />
                            <RefSelect
                                id="create-policy-node"
                                label="节点"
                                value={nodeName}
                                onChange={onNodeNameChange}
                                options={nodes}
                                placeholder="选择节点"
                            />
                        </div>
                    )}
                    {kind === 'modelNode' && (
                        <div className="grid grid-cols-2 gap-3">
                            <RefSelect
                                id="create-policy-model"
                                label="模型"
                                value={modelName}
                                onChange={onModelNameChange}
                                options={models}
                                placeholder="选择模型"
                            />
                            <RefSelect
                                id="create-policy-node"
                                label="节点"
                                value={nodeName}
                                onChange={onNodeNameChange}
                                options={nodes}
                                placeholder="选择节点"
                            />
                        </div>
                    )}

                    <div>
                        <Label htmlFor="create-policy-effect" className="text-xs font-medium text-[#A0A0A0]">
                            策略效果
                        </Label>
                        <Select value={effect} onValueChange={(value) => onEffectChange(value as PolicyEffect)}>
                            <SelectTrigger id="create-policy-effect" className={`mt-2 w-full ${selectClass}`}>
                                <SelectValue placeholder="选择效果" />
                            </SelectTrigger>
                            <SelectContent className="border-[#263244] bg-[#101722] text-[#EDEDED]">
                                <SelectItem value="Allow" className="focus:bg-[#202B3A] focus:text-white">
                                    <span className="flex items-center gap-2">
                                        <span className="text-[#57C894]">Allow</span>
                                        <span className="text-[#596579]">允许使用</span>
                                    </span>
                                </SelectItem>
                                <SelectItem value="Deny" className="focus:bg-[#202B3A] focus:text-white">
                                    <span className="flex items-center gap-2">
                                        <span className="text-[#FF7373]">Deny</span>
                                        <span className="text-[#596579]">禁止使用</span>
                                    </span>
                                </SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    <div className="rounded-md border border-[#202B3A] bg-[#0B1018] px-3 py-2">
                        <p className="text-[11px] text-[#596579]">系统标识</p>
                        <code className="mt-1 block truncate font-mono text-xs text-[#9A9A9A]">
                            {identifierPreview || '选择引用后自动生成'}
                        </code>
                    </div>
                    {error && (
                        <div className="rounded-md border border-red-500/20 bg-red-500/10 px-3 py-2 text-sm text-[#FF8A8A]">
                            {error}
                        </div>
                    )}
                </div>

                <DialogFooter className="border-t border-[#202B3A] bg-[#080C12] px-6 py-4">
                    <Button
                        type="button"
                        variant="outline"
                        disabled={pending}
                        onClick={() => onOpenChange(false)}
                        className="border-[#303C50] bg-[#141C28] text-[#D4D4D4] hover:bg-[#222] hover:text-white"
                    >
                        取消
                    </Button>
                    <Button
                        type="button"
                        ref={confirmRef}
                        disabled={!ready || pending}
                        onClick={onConfirm}
                        className="min-w-24 bg-[#5B8CFF] text-white hover:bg-[#70A0FF]"
                    >
                        {pending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Plus className="mr-2 h-4 w-4" />}
                        {pending ? '正在创建' : '创建策略'}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}