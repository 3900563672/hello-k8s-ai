import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { configApi } from '@/api/endpoints/configApi'
import { useReplayTimeContext } from '@/stores/timeSlice'

export const configKeys = {
    all: ['config'] as const,
    configuration: () => [...configKeys.all, 'configuration'] as const,
    models: (version: string = 'latest') => [...configKeys.all, 'models', version] as const,
    modelDetail: (name: string) => [...configKeys.all, 'models', 'latest', name] as const,
    nodes: (version: string = 'latest') => [...configKeys.all, 'nodes', version] as const,
    nodeDetail: (name: string) => [...configKeys.all, 'nodes', 'latest', name] as const,
    tenants: (version: string = 'latest') => [...configKeys.all, 'tenants', version] as const,
    tenantDetail: (name: string) => [...configKeys.all, 'tenants', 'latest', name] as const,
}

const queryDefaults = {
    staleTime: 10_000,
    refetchInterval: 30_000,
    retry: 1,
}

const replayQuery = (mode: 'latest' | 'historical', snapshotId: string | null, effectiveAt: string) => ({
    version: mode === 'historical' ? snapshotId ?? effectiveAt : 'latest',
    timestamp: mode === 'historical' ? effectiveAt : undefined,
    historical: mode === 'historical',
})

export const useModels = () => {
    const replay = useReplayTimeContext()
    const query = replayQuery(replay.mode, replay.snapshotId, replay.effectiveAt)
    return useQuery({
        queryKey: configKeys.models(query.version),
        queryFn: () => configApi.getModels(query.timestamp),
        ...queryDefaults,
        staleTime: query.historical ? Number.POSITIVE_INFINITY : queryDefaults.staleTime,
        refetchInterval: query.historical ? false : queryDefaults.refetchInterval,
    })
}

export const useNodes = () => {
    const replay = useReplayTimeContext()
    const query = replayQuery(replay.mode, replay.snapshotId, replay.effectiveAt)
    return useQuery({
        queryKey: configKeys.nodes(query.version),
        queryFn: () => configApi.getNodes(query.timestamp),
        ...queryDefaults,
        staleTime: query.historical ? Number.POSITIVE_INFINITY : queryDefaults.staleTime,
        refetchInterval: query.historical ? false : queryDefaults.refetchInterval,
    })
}

export const useTenants = () => {
    const replay = useReplayTimeContext()
    const query = replayQuery(replay.mode, replay.snapshotId, replay.effectiveAt)
    return useQuery({
        queryKey: configKeys.tenants(query.version),
        queryFn: () => configApi.getTenants(query.timestamp),
        ...queryDefaults,
        staleTime: query.historical ? Number.POSITIVE_INFINITY : queryDefaults.staleTime,
        refetchInterval: query.historical ? false : queryDefaults.refetchInterval,
    })
}

const useConfigMutation = <TVariables, TResult>(
    mutationFn: (variables: TVariables) => Promise<TResult>,
) => {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn,
        onSuccess: async () => {
            await queryClient.invalidateQueries({ queryKey: configKeys.all })
        },
    })
}

export const useCreateModel = () => useConfigMutation(configApi.createModel)
export const useUpdateModel = () => useConfigMutation(configApi.updateModel)
export const useDeleteModel = () => useConfigMutation(configApi.deleteModel)
export const useDeleteModels = () => useConfigMutation(configApi.deleteModels)

export const useCreateNode = () => useConfigMutation(configApi.createNode)
export const useUpdateNode = () => useConfigMutation(configApi.updateNode)
export const useDeleteNode = () => useConfigMutation(configApi.deleteNode)
export const useDeleteNodes = () => useConfigMutation(configApi.deleteNodes)

export const useCreateTenant = () => useConfigMutation(configApi.createTenant)
export const useUpdateTenant = () => useConfigMutation(configApi.updateTenant)
export const useDeleteTenant = () => useConfigMutation(configApi.deleteTenant)
export const useDeleteTenants = () => useConfigMutation(configApi.deleteTenants)
