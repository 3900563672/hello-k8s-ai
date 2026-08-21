import { Clock3, ListChecks, Loader2 } from 'lucide-react'
import { useAIOpsJobs } from '@/api/queries/aiopsQueries'
import { cn } from '@/lib/utils'
import type { AIOpsJobStatus } from '@/types/aiops.types'

const JOB_STATUS_META: Record<AIOpsJobStatus, { label: string; dot: string; text: string }> = {
    pending: { label: '排队中', dot: 'bg-[#5A6778]', text: 'text-[#8C99AC]' },
    running: { label: '执行中', dot: 'bg-[#5B8CFF]', text: 'text-[#9EB2FF]' },
    done: { label: '已完成', dot: 'bg-emerald-400', text: 'text-emerald-300' },
    failed: { label: '失败', dot: 'bg-red-400', text: 'text-red-300' },
}

function formatTime(value: string): string {
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return value
    return new Intl.DateTimeFormat('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
    }).format(date)
}

function shortSegmentId(segmentId: string): string {
    return segmentId.length > 18 ? `${segmentId.slice(0, 8)}…${segmentId.slice(-6)}` : segmentId
}

/**
 * 异步任务可见性（#110 阶段一）：任务级状态 / 重试 / 失败原因。
 * DB 即队列（aiops_jobs），worker 认领后回写；10s 轮询。
 */
export function AIOpsJobList() {
    const jobsQuery = useAIOpsJobs()
    const jobs = jobsQuery.data?.data ?? []
    const active = jobs.filter((job) => job.status === 'pending' || job.status === 'running').length

    return (
        <div className="rounded-xl border border-white/[0.06] bg-[#0A0E15]/60 px-3 py-2.5">
            <div className="flex items-center gap-2 px-1">
                <ListChecks className="h-3.5 w-3.5 text-[#7CAEFF]" />
                <span className="text-[11px] font-medium text-[#5A6778]">异步任务（10s 轮询）</span>
                {active > 0 && (
                    <span className="rounded-full bg-[#5B8CFF]/20 px-2 py-0.5 text-[10px] text-[#9EB2FF]">
                        进行中 {active}
                    </span>
                )}
                {jobsQuery.isFetching && (
                    <Loader2 className="ml-auto h-3 w-3 animate-spin text-[#5B8CFF]" />
                )}
            </div>
            {jobs.length === 0 && !jobsQuery.isLoading && (
                <p className="px-1 pt-1.5 text-[11px] text-[#4C5868]">
                    暂无任务——切面完成/失败后自动入队分析。
                </p>
            )}
            {jobs.length > 0 && (
                <div className="mt-1.5 space-y-1">
                    {jobs.slice(0, 6).map((job) => {
                        const meta = JOB_STATUS_META[job.status]
                        return (
                            <div
                                key={job.jobId}
                                className="flex flex-wrap items-center gap-x-2 gap-y-0.5 rounded-lg bg-white/[0.02] px-2 py-1.5"
                            >
                                <span className="font-mono text-[11px] text-[#C6D0DE]">
                                    {shortSegmentId(job.segmentId)}
                                </span>
                                <span className={cn('inline-flex items-center gap-1.5 text-[10px] font-medium', meta.text)}>
                                    <span className={cn('h-1.5 w-1.5 rounded-full', meta.dot)} />
                                    {meta.label}
                                </span>
                                {job.attempts > 1 && (
                                    <span className="text-[10px] text-[#5A6778]">第 {job.attempts} 次</span>
                                )}
                                {(job.status === 'running' || job.status === 'done') && job.startedAt && (
                                    <span className="ml-auto inline-flex items-center gap-1 text-[10px] text-[#5A6778]">
                                        <Clock3 className="h-2.5 w-2.5" />
                                        {formatTime(job.startedAt)}
                                    </span>
                                )}
                                {job.status === 'failed' && job.lastError && (
                                    <span className="w-full truncate text-[10px] leading-4 text-red-300/80">
                                        {job.lastError}
                                    </span>
                                )}
                            </div>
                        )
                    })}
                </div>
            )}
        </div>
    )
}
