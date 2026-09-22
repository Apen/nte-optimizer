import { Download } from 'lucide-react'
import { OpenDownloadURL } from '../../wailsjs/go/main/DesktopApp'
import { t } from '../i18n'
import { Button } from './ui/button'
import { Dialog, DialogContent } from './ui/dialog'

export type UpdateInfo = {
  available: boolean
  current_version: string
  latest_version?: string
  download_url?: string
}

export function UpdateDialog({ update, onClose }: { update: UpdateInfo; onClose: () => void }) {
  const download = () => {
    if (update.download_url) void OpenDownloadURL(update.download_url)
  }
  return <Dialog open={update.available} onOpenChange={open => { if (!open) onClose() }}>
    <DialogContent className="w-[min(560px,92vw)]" title={t('update_available_title')}>
      <div className="grid gap-5 pr-10">
        <div>
          <p className="eyebrow">{t('update_available_eyebrow')}</p>
          <h2 className="mt-2 text-2xl font-black text-white">{t('update_available_title')}</h2>
        </div>
        <p className="text-sm leading-6 text-slate-300">{t('update_available_description', { current: update.current_version, latest: update.latest_version || '' })}</p>
        <p className="rounded-xl border border-emerald-800/70 bg-emerald-950/30 p-4 text-sm text-emerald-200">{t('update_preserves_data')}</p>
        <div className="flex flex-wrap justify-end gap-3">
          <Button variant="secondary" onClick={onClose}>{t('update_later')}</Button>
          <Button onClick={download}><Download className="size-4" />{t('update_download')}</Button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
}
