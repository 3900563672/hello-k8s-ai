import { useDroppable } from '@dnd-kit/core'
import { MoveDown } from 'lucide-react'
import { cn } from '@/lib/utils'
import { TrafficCanvas } from './TrafficCanvas'

interface CanvasDropZoneProps {
    className?: string
}

export function CanvasDropZone({ className = '' }: CanvasDropZoneProps) {
    const { setNodeRef, isOver } = useDroppable({ id: 'traffic-canvas-dropzone' })

    return (
        <div
            ref={setNodeRef}
            className={cn('relative h-full w-full transition-all duration-200', className)}
        >
            <TrafficCanvas className={cn('h-full w-full transition-all', isOver && 'border-[#5B8CFF]/45')} />
            {isOver && (
                <div className="pointer-events-none absolute inset-0 z-30 flex items-center justify-center rounded-2xl border-2 border-[#5B8CFF]/55 bg-[#5B8CFF]/[0.055] backdrop-blur-[1px]">
                    <div className="flex items-center gap-2 rounded-xl border border-[#5B8CFF]/30 bg-[#0B1220]/95 px-4 py-3 text-xs font-medium text-[#9BC7FF] shadow-2xl">
                        <MoveDown className="h-4 w-4" />
                        释放后配置目标租户与逻辑偏移
                    </div>
                </div>
            )}
        </div>
    )
}
