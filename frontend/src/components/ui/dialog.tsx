import * as DialogPrimitive from '@radix-ui/react-dialog'
import { X } from 'lucide-react'
import { t } from '../../i18n'
export const Dialog = DialogPrimitive.Root
export const DialogTrigger = DialogPrimitive.Trigger
export function DialogContent({ children, className = '' }: { children: React.ReactNode; className?: string }) { return <DialogPrimitive.Portal><DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-black/85 backdrop-blur-sm" /><DialogPrimitive.Content className={`fixed left-1/2 top-1/2 z-50 max-h-[90vh] w-[min(1100px,92vw)] -translate-x-1/2 -translate-y-1/2 overflow-auto rounded-2xl border border-slate-700 bg-slate-950 p-5 shadow-2xl ${className}`}><DialogPrimitive.Title className="sr-only">{t('module_preview')}</DialogPrimitive.Title>{children}<DialogPrimitive.Close className="absolute right-4 top-4 rounded-full bg-slate-900 p-2 text-slate-300"><X className="size-5" /></DialogPrimitive.Close></DialogPrimitive.Content></DialogPrimitive.Portal> }
