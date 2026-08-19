import { apiData, apiRequest } from '@/api/client'
import { createClientId } from '@/lib/clientId'
import type { ApiEnvelope } from '@/types/api.types'
import type {
    FutureTrafficData,
    TenantInfo,
    TrafficApplyReceipt,
} from '@/types/traffic.types'

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

interface OperationReceipt {
    results: Array<{
        resourceVersion?: string
        convergence: string
        error?: string
    }>
}

/** 把叠加后的目标 QPS 写入租户（Tenant.spec.qps，控制面为常量目标值）。 */
export async function setTenantTraffic(
    tenantId: string,
    qps: number,
    resourceVersion?: string,
): Promise<TrafficApplyReceipt> {
    const target = Math.max(0, Math.round(qps))
    const response = await apiRequest<ApiEnvelope<OperationReceipt>>(
        `/tenants/${encodeURIComponent(tenantId)}/traffic`,
        {
            method: 'PATCH',
            headers: {
                'Idempotency-Key': createClientId('tenant-traffic'),
            },
            body: JSON.stringify({
                qps: target,
                ...(resourceVersion ? { resourceVersion } : {}),
                dryRun: false,
            }),
        },
    )
    const result = response.data.results[0]
    if (!result) throw new Error('Backend 未返回流量更新结果')
    return {
        tenantId,
        qps: target,
        resourceVersion: result.resourceVersion || resourceVersion,
        convergence: result.convergence,
    }
}

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
