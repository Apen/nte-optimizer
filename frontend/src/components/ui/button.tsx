import { type ButtonHTMLAttributes } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '../../lib/utils'

const variants = cva('nte-button inline-flex min-h-10 shrink-0 items-center justify-center gap-1 rounded-lg border px-4 py-2 text-sm font-semibold transition disabled:pointer-events-none disabled:opacity-50', {
  variants: { variant: { default: 'border-pink-400 bg-pink-600 text-white hover:bg-pink-500', secondary: 'border-slate-700 bg-slate-950 text-slate-300 hover:bg-slate-900', destructive: 'border-red-500/50 bg-red-950 text-red-200 hover:bg-red-900' } },
  defaultVariants: { variant: 'default' },
})
export function Button({ className, variant, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & VariantProps<typeof variants>) { return <button className={cn(variants({ variant }), className)} {...props} /> }
