import { useEffect, useRef, useState } from 'react'
import { Database } from 'lucide-react'

import { currentIntlLocale, t } from '../i18n'
import type { OptimizationLog } from '../types'
import { Button } from './ui/button'
import { Dialog, DialogContent, DialogTrigger } from './ui/dialog'

export function OptimizationLogDialog({ log }: { log: OptimizationLog }) {
  const [copied, setCopied] = useState(false)
  const resetTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const text = [`${t('start')}: ${log.started_at}`, `${t('end')}: ${log.finished_at || '—'}`, `${t('status')}: ${log.status}`, ...log.entries.map(entry => `[${entry.level}] [${entry.stage}] ${entry.message}`)].join('\n')

  useEffect(() => () => {
    if (resetTimer.current) clearTimeout(resetTimer.current)
  }, [])

  const copy = async () => {
    await navigator.clipboard.writeText(text)
    setCopied(true)
    if (resetTimer.current) clearTimeout(resetTimer.current)
    resetTimer.current = setTimeout(() => setCopied(false), 1500)
  }

  return <Dialog><DialogTrigger asChild><button className="text-action"><Database className="size-3.5" />{t('calculation_log')}</button></DialogTrigger><DialogContent><div className="flex items-start justify-between gap-4 pr-10"><div><p className="eyebrow">{t('diagnostic')}</p><h2 className="mt-1 text-xl font-bold">{t('last_calculation_log')}</h2></div><Button variant="secondary" onClick={copy}>{copied ? t('copied') : t('copy')}</Button></div><div className="log-summary"><span className={`log-status is-${log.status}`}>{log.status === 'success' ? t('completed') : log.status === 'error' ? t('error') : t('in_progress')}</span><span>{new Date(log.started_at).toLocaleString(currentIntlLocale())}</span></div><div className="optimization-log">{log.entries.map((entry, index) => <div className="log-entry" key={`${entry.stage}-${index}`}><span className={`log-level is-${entry.level.toLowerCase()}`}>{entry.level}</span><b>{entry.stage}</b><code>{entry.message}</code></div>)}</div></DialogContent></Dialog>
}
