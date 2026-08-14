import { useQuery } from '@tanstack/react-query'
import { trafficApi } from '@/api/endpoints/trafficApi'
import { useReplayTimeContext } from '@/stores/timeSlice'

export const trafficKeys = {
    all: ['traffic'] as const,
    tenants: (version: string) => [...trafficKeys.all, 'tenants', version] as const,
    tenantTraffic: (tenantId: string) => [...trafficKeys.all, 'traffic', tenantId] as const,
    allTenantsTraffic: () => [...trafficKeys.all, 'all-traffic'] as const,
    totalQPS: () => [...trafficKeys.all, 'total-qps'] as const,
}

// ===== 获取租户列表 =====
export const useTenants = () => {
    const replay = useReplayTimeContext()
    const timestamp = replay.mode === 'historical' ? replay.effectiveAt : undefined
    return useQuery({
        queryKey: trafficKeys.tenants(replay.mode === 'historical' ? replay.snapshotId ?? replay.effectiveAt : 'latest'),
        queryFn: () => trafficApi.getTenants(timestamp),
    })
}

// ===== 获取单个租户流量 =====
export const useTenantTraffic = (tenantId: string | null) => {
    const replay = useReplayTimeContext()
    const timestamp = replay.mode === 'historical' ? replay.effectiveAt : undefined
    return useQuery({
        queryKey: [...trafficKeys.tenantTraffic(tenantId || ''), timestamp ?? 'latest'],
        queryFn: () => trafficApi.getTenantTraffic(tenantId!, timestamp),
        enabled: !!tenantId,
    })
}

// ===== 获取所有租户流量 =====
export const useAllTenantsTraffic = () => {
    const replay = useReplayTimeContext()
    const timestamp = replay.mode === 'historical' ? replay.effectiveAt : undefined
    return useQuery({
        queryKey: [...trafficKeys.allTenantsTraffic(), timestamp],
        queryFn: () => trafficApi.getAllTenantsTraffic(timestamp),
    })
}

// ===== 获取总 QPS（前端聚合） =====
export const useTotalQPS = () => {
    const replay = useReplayTimeContext()
    const timestamp = replay.mode === 'historical' ? replay.effectiveAt : undefined
    return useQuery({
        queryKey: [...trafficKeys.totalQPS(), timestamp],
        queryFn: () => trafficApi.getTotalQPS(timestamp),
    })
}
