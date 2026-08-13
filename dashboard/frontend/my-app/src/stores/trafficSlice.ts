import { create } from 'zustand'
import { createClientId } from '@/lib/clientId'
import type {
    OverlayInstance,
    TrafficPoint,
    TrafficTemplate,
    TrafficViewMode,
    TrafficWorkspaceMode,
} from '@/types/traffic.types'

const overlayColors = ['#5B8CFF', '#43C6AC', '#F6B73C', '#B27CFF', '#FF7A90', '#44B9F1']

type NewTemplate = Omit<TrafficTemplate, 'id' | 'createdAt' | 'updatedAt'>
type NewOverlay = Omit<OverlayInstance, 'id' | 'createdAt' | 'color'> & { color?: string }

interface TrafficSlice {
    templates: TrafficTemplate[]
    addTemplate: (template: NewTemplate) => TrafficTemplate
    removeTemplate: (id: string) => void
    updateTemplate: (id: string, updates: Partial<NewTemplate>) => void
    getTemplate: (id: string) => TrafficTemplate | undefined
    overlays: OverlayInstance[]
    addOverlay: (overlay: NewOverlay) => OverlayInstance
    removeOverlay: (id: string) => void
    toggleOverlay: (id: string) => void
    clearOverlays: () => void
    getOverlaysForTenant: (tenantId: string) => OverlayInstance[]
    viewMode: TrafficViewMode
    setViewMode: (mode: TrafficViewMode) => void
    selectedTenant: string | null
    setSelectedTenant: (tenantId: string | null) => void
    compareTenants: string[]
    setCompareTenants: (tenantIds: string[]) => void
    toggleCompareTenant: (tenantId: string) => void
    mode: TrafficWorkspaceMode
    setMode: (mode: TrafficWorkspaceMode) => void
}

function createId() {
    return createClientId('traffic')
}

function sanitizePoints(points: TrafficPoint[]) {
    return points
        .filter((point) => Number.isFinite(point.x) && Number.isFinite(point.y))
        .map((point) => ({ x: Math.max(0, point.x), y: Math.max(0, point.y) }))
        .sort((left, right) => left.x - right.x)
}

/** 模板与 Overlay 是未保存的 UI 草稿。服务端流量和历史数据由
 * TanStack Query/PostgreSQL 管理，因此此 Store 只保存在内存中，不写 localStorage。 */
export const useTrafficStore = create<TrafficSlice>()((set, get) => ({
    templates: [],
    addTemplate: (template) => {
        const now = new Date().toISOString()
        const created: TrafficTemplate = {
            ...template,
            shapeType: 'custom',
            controlPoints: sanitizePoints(template.controlPoints),
            id: createId(),
            createdAt: now,
            updatedAt: now,
        }
        set((state) => ({ templates: [...state.templates, created] }))
        return created
    },
    removeTemplate: (id) => {
        set((state) => ({
            templates: state.templates.filter((template) => template.id !== id),
            overlays: state.overlays.filter((overlay) => overlay.templateId !== id),
        }))
    },
    updateTemplate: (id, updates) => {
        set((state) => ({
            templates: state.templates.map((template) => template.id === id
                ? {
                    ...template,
                    ...updates,
                    shapeType: 'custom',
                    controlPoints: updates.controlPoints
                        ? sanitizePoints(updates.controlPoints)
                        : template.controlPoints,
                    updatedAt: new Date().toISOString(),
                }
                : template),
        }))
    },
    getTemplate: (id) => get().templates.find((template) => template.id === id),
    overlays: [],
    addOverlay: (overlay) => {
        const created: OverlayInstance = {
            ...overlay,
            id: createId(),
            startOffsetSeconds: Math.max(0, overlay.startOffsetSeconds),
            color: overlay.color
                ?? overlayColors[get().overlays.length % overlayColors.length],
            createdAt: new Date().toISOString(),
        }
        set((state) => ({ overlays: [...state.overlays, created] }))
        return created
    },
    removeOverlay: (id) => {
        set((state) => ({ overlays: state.overlays.filter((overlay) => overlay.id !== id) }))
    },
    toggleOverlay: (id) => {
        set((state) => ({
            overlays: state.overlays.map((overlay) => overlay.id === id
                ? { ...overlay, enabled: !overlay.enabled }
                : overlay),
        }))
    },
    clearOverlays: () => set({ overlays: [] }),
    getOverlaysForTenant: (tenantId) => get().overlays.filter(
        (overlay) => overlay.tenantId === tenantId && overlay.enabled,
    ),
    viewMode: 'total',
    setViewMode: (viewMode) => set({ viewMode }),
    selectedTenant: null,
    setSelectedTenant: (selectedTenant) => set({ selectedTenant }),
    compareTenants: [],
    setCompareTenants: (tenantIds) => set({ compareTenants: [...new Set(tenantIds)] }),
    toggleCompareTenant: (tenantId) => {
        set((state) => ({
            compareTenants: state.compareTenants.includes(tenantId)
                ? state.compareTenants.filter((id) => id !== tenantId)
                : [...state.compareTenants, tenantId],
        }))
    },
    mode: 'overview',
    setMode: (mode) => set({ mode }),
}))
