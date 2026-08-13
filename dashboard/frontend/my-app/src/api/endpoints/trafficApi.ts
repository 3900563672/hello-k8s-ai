import { apiData } from '@/api/client'
import type { FutureTrafficData, TenantInfo } from '@/types/traffic.types'

interface TrafficTenant {
    tenant: { name: string }
    displayName: string
    priority: TenantInfo['priority']
    requestedQPS: number
    allocatedQPS: number
    runtimePhase?: string
}

interface TrafficReadModel {
    asOf: string
    tenants: TrafficTenant[]
}

const queryAt = (timestamp?: string) => {
    if (!timestamp || timestamp === new Date(0).toISOString()) return ''
    return `?at=${encodeURIComponent(timestamp)}`
}

const toSeries = (tenant: TrafficTenant): FutureTrafficData => ({
    tenantId: tenant.tenant.name,
    tenantName: tenant.displayName || tenant.tenant.name,
    timeSeconds: [0],
    values: [tenant.requestedQPS],
})

const fetchTraffic = (timestamp?: string): Promise<TrafficReadModel> =>
    apiData<TrafficReadModel>(`/traffic${queryAt(timestamp)}`)

export const trafficApi = {
    getTenants: async (timestamp?: string): Promise<TenantInfo[]> => {
        const traffic = await fetchTraffic(timestamp)
        return traffic.tenants.map((tenant) => ({
            id: tenant.tenant.name,
            name: tenant.displayName || tenant.tenant.name,
            priority: tenant.priority,
            requestedQPS: tenant.requestedQPS,
            allocatedQPS: tenant.allocatedQPS,
            runtimePhase: tenant.runtimePhase,
        }))
    },

    getTenantTraffic: async (
        tenantId: string,
        timestamp?: string,
    ): Promise<FutureTrafficData | null> => {
        const traffic = await fetchTraffic(timestamp)
        const tenant = traffic.tenants.find((item) => item.tenant.name === tenantId)
        return tenant ? toSeries(tenant) : null
    },

    getAllTenantsTraffic: async (timestamp?: string): Promise<FutureTrafficData[]> =>
        (await fetchTraffic(timestamp)).tenants.map(toSeries),

    getTotalQPS: async (timestamp?: string): Promise<FutureTrafficData> => {
        const traffic = await fetchTraffic(timestamp)
        return {
            tenantId: 'total',
            tenantName: '全部租户',
            timeSeconds: [0],
            values: [traffic.tenants.reduce((total, tenant) => total + tenant.requestedQPS, 0)],
        }
    },
}
