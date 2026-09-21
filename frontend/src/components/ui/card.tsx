import { type HTMLAttributes } from 'react'
import { cn } from '../../lib/utils'
export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) { return <div className={cn('nte-card min-w-0 rounded-xl border border-slate-800 bg-slate-900/80', className)} {...props} /> }
export function CardHeader({ className, ...props }: HTMLAttributes<HTMLDivElement>) { return <div className={cn('p-4 pb-2', className)} {...props} /> }
export function CardContent({ className, ...props }: HTMLAttributes<HTMLDivElement>) { return <div className={cn('p-4 pt-2', className)} {...props} /> }
