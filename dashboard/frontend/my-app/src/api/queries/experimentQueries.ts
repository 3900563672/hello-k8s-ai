import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
    completeExperiment,
    createExperiment,
    failExperiment,
    fetchExperiment,
    fetchExperiments,
    startExperiment,
} from '@/api/endpoints/experimentApi'


export const experimentQueryKeys = {
    all: ['experiments'] as const,
    list: (status?: string) => [...experimentQueryKeys.all, 'list', status ?? null] as const,
    detail: (segmentId: string) => [...experimentQueryKeys.all, 'detail', segmentId] as const,
}

export function useExperiments(status?: string) {
    return useQuery({
        queryKey: experimentQueryKeys.list(status),
        queryFn: () => fetchExperiments(status),
        refetchInterval: 10_000,
        staleTime: 5_000,
        retry: 1,
    })
}

export function useExperimentDetail(segmentId: string | null) {
    return useQuery({
        queryKey: experimentQueryKeys.detail(segmentId ?? ''),
        queryFn: () => fetchExperiment(segmentId!),
        enabled: Boolean(segmentId),
        staleTime: Number.POSITIVE_INFINITY,
        retry: 1,
    })
}

const useExperimentMutation = <TVariables, TResult>(
    mutationFn: (variables: TVariables) => Promise<TResult>,
) => {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn,
        onSuccess: async () => {
            await queryClient.invalidateQueries({ queryKey: experimentQueryKeys.all })
        },
    })
}

export const useCreateExperiment = () => useExperimentMutation(
    ({ tenant, name }: { tenant: string; name: string }) => createExperiment(tenant, name),
)
export const useStartExperiment = () => useExperimentMutation(startExperiment)
export const useCompleteExperiment = () => useExperimentMutation(completeExperiment)
export const useFailExperiment = () => useExperimentMutation(
    ({ segmentId, reason }: { segmentId: string; reason: string }) => failExperiment(segmentId, reason),
)