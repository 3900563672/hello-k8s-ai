import { create } from 'zustand'
import { createClientId } from '@/lib/clientId'
import type { ModelFormValues } from '@/lib/validations/model.schema'
import type { NodeFormValues } from '@/lib/validations/node.schema'
import type { TenantFormValues } from '@/lib/validations/tenant.schema'
import type { ConfigTemplate } from '@/types/config.types'

interface TemplateStore {
    modelTemplates: ConfigTemplate<ModelFormValues>[]
    nodeTemplates: ConfigTemplate<NodeFormValues>[]
    tenantTemplates: ConfigTemplate<TenantFormValues>[]
    addModelTemplate: (name: string, data: ModelFormValues) => void
    addNodeTemplate: (name: string, data: NodeFormValues) => void
    addTenantTemplate: (name: string, data: TenantFormValues) => void
    removeModelTemplate: (id: string) => void
    removeNodeTemplate: (id: string) => void
    removeTenantTemplate: (id: string) => void
}

const createTemplate = <T,>(name: string, data: T): ConfigTemplate<T> => ({
    id: createClientId('template'),
    name: name.trim(),
    data,
    createdAt: new Date().toISOString(),
})

/**
 * 配置模板是编辑器中的临时草稿。Kubernetes 资源通过 configQueries 加载，
 * 不会从浏览器状态恢复。
 */
export const useTemplateStore = create<TemplateStore>()((set) => ({
    modelTemplates: [],
    nodeTemplates: [],
    tenantTemplates: [],
    addModelTemplate: (name, data) =>
        set((state) => ({
            modelTemplates: [...state.modelTemplates, createTemplate(name, data)],
        })),
    addNodeTemplate: (name, data) =>
        set((state) => ({
            nodeTemplates: [...state.nodeTemplates, createTemplate(name, data)],
        })),
    addTenantTemplate: (name, data) =>
        set((state) => ({
            tenantTemplates: [...state.tenantTemplates, createTemplate(name, data)],
        })),
    removeModelTemplate: (id) =>
        set((state) => ({
            modelTemplates: state.modelTemplates.filter((template) => template.id !== id),
        })),
    removeNodeTemplate: (id) =>
        set((state) => ({
            nodeTemplates: state.nodeTemplates.filter((template) => template.id !== id),
        })),
    removeTenantTemplate: (id) =>
        set((state) => ({
            tenantTemplates: state.tenantTemplates.filter((template) => template.id !== id),
        })),
}))
