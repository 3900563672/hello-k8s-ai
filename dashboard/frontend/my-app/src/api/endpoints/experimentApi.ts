import { apiRequest } from '@/api/client'
import type {
    ExperimentDetailEnvelope,
    ExperimentListEnvelope,
} from '@/types/experiment.types'

export function fetchExperiments(status?: string): Promise<ExperimentListEnvelope> {
    const suffix = status ? `?status=${encodeURIComponent(status)}` : ''
    return apiRequest<ExperimentListEnvelope>(`/experiments${suffix}`)
}

export function fetchExperiment(segmentId: string): Promise<ExperimentDetailEnvelope> {
    return apiRequest<ExperimentDetailEnvelope>(`/experiments/${encodeURIComponent(segmentId)}`)
}

export function createExperiment(tenant: string, name: string): Promise<ExperimentDetailEnvelope> {
    return apiRequest<ExperimentDetailEnvelope>('/experiments', {
        method: 'POST',
        body: JSON.stringify({ tenant, name }),
    })
}

export function startExperiment(segmentId: string): Promise<ExperimentDetailEnvelope> {
    return apiRequest<ExperimentDetailEnvelope>(`/experiments/${encodeURIComponent(segmentId)}/start`, {
        method: 'POST',
    })
}

export function completeExperiment(segmentId: string): Promise<ExperimentDetailEnvelope> {
    return apiRequest<ExperimentDetailEnvelope>(`/experiments/${encodeURIComponent(segmentId)}/complete`, {
        method: 'POST',
    })
}

export function failExperiment(segmentId: string, reason: string): Promise<ExperimentDetailEnvelope> {
    return apiRequest<ExperimentDetailEnvelope>(`/experiments/${encodeURIComponent(segmentId)}/fail`, {
        method: 'POST',
        body: JSON.stringify({ reason }),
    })
}