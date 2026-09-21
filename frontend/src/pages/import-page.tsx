import { useState } from 'react'
import { Database, Play } from 'lucide-react'

import { ScanAndImportAccount } from '../../wailsjs/go/main/DesktopApp'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardHeader } from '../components/ui/card'
import { t, type Locale } from '../i18n'
import { formatDateTime } from '../lib/format'
import type { AccountImportSummary } from '../types'

type ImportPageProps = {
  summary: AccountImportSummary
  locale: Locale
  onImported: (summary: AccountImportSummary) => Promise<void>
}

export function ImportPage({ summary, locale, onImported }: ImportPageProps) {
  const [scanning, setScanning] = useState(false)
  const [seconds, setSeconds] = useState(35)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  async function scanAndImport() {
    setScanning(true)
    setMessage(t('scan_waiting'))
    setError('')
    try {
      const next = await ScanAndImportAccount(locale, seconds) as AccountImportSummary
      await onImported(next)
      setMessage(t('scan_finished', { characters: next.characters, pieces: next.modules + next.cartridges, arcs: next.weapons }))
    } catch (value) {
      setMessage('')
      setError(String(value))
    } finally {
      setScanning(false)
    }
  }

  return <>
    <header className="mb-7"><p className="eyebrow">{t('import_section')}</p><h1 className="text-4xl font-black tracking-tight sm:text-5xl">{t('update_game_data')}</h1><p className="mt-2 max-w-2xl text-slate-400">{t('import_description')}</p></header>
    <div className="grid gap-5 xl:grid-cols-[1.35fr_.65fr]">
      <div>
        <Card><CardHeader><div className="flex items-start gap-4"><span className="grid size-12 shrink-0 place-items-center rounded-xl bg-pink-950 text-pink-300"><Play className="size-6" /></span><div><h2 className="text-xl font-bold">{t('scan_title')}</h2><p className="mt-1 text-sm text-slate-400">{t('scan_description')}</p></div></div></CardHeader><CardContent><ol className="grid gap-3 rounded-xl border border-slate-800 bg-slate-950/60 p-4 text-sm text-slate-300"><li><b className="mr-2 text-pink-300">1.</b>{t('scan_step_ready')}</li><li><b className="mr-2 text-pink-300">2.</b>{t('scan_step_admin')}</li><li><b className="mr-2 text-pink-300">3.</b>{t('scan_step_login')}</li><li><b className="mr-2 text-pink-300">4.</b>{t('scan_step_import')}</li></ol><div className="mt-4 flex flex-wrap items-end gap-3"><label className="grid gap-1 text-xs font-semibold text-slate-400"><span>{t('scan_duration')}</span><select className="h-11 rounded-lg border border-slate-700 bg-slate-950 px-3 text-sm" value={seconds} onChange={event => setSeconds(Number(event.target.value))}><option value={30}>30 s</option><option value={35}>35 s</option><option value={45}>45 s</option><option value={60}>60 s</option></select></label><Button className="h-11 px-6" disabled={scanning} onClick={scanAndImport}><Play className="mr-2 size-4" />{scanning ? t('scan_running') : t('scan_start')}</Button></div><p className="mt-3 text-xs leading-relaxed text-slate-500">{t('scan_privacy')}</p></CardContent></Card>
        {error && <p role="alert" className="rounded-xl border border-red-900 bg-red-950/40 p-4 text-sm text-red-200">{error}</p>}{message && <p role="status" className="rounded-xl border border-emerald-900 bg-emerald-950/30 p-4 text-sm text-emerald-200">{message}</p>}
      </div>
      <Card><CardHeader><div className="flex items-center gap-3"><Database className="size-5 text-pink-300" /><h2 className="text-lg font-bold">{t('latest_import')}</h2></div></CardHeader><CardContent>{summary.has_import ? <><strong className="block text-xl">{formatDateTime(summary.imported_at)}</strong>{summary.source_generated_at && <p className="mt-1 text-xs text-slate-500">{t('export_generated', { date: formatDateTime(summary.source_generated_at) })}</p>}<div className="mt-5 grid grid-cols-2 gap-3">{[[t('count_characters'), summary.characters], [t('count_modules'), summary.modules], [t('count_cartridges'), summary.cartridges], [t('count_arcs'), summary.weapons]].map(([label, value]) => <div key={String(label)} className="rounded-xl border border-slate-800 bg-slate-950/60 p-3"><strong className="block text-xl tabular-nums">{value}</strong><span className="text-xs text-slate-500">{label}</span></div>)}</div></> : <p className="text-sm text-slate-500">{t('no_account_data')}</p>}</CardContent></Card>
    </div>
  </>
}
