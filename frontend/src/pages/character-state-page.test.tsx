import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { CharacterStatePage } from './character-state-page'
import { applyLocalization } from '../i18n'
import en from '../../../data/presentation/en.json'
import fr from '../../../data/presentation/fr.json'

vi.mock('../../wailsjs/go/main/DesktopApp', () => ({ CharacterGameState: async () => { throw new Error('nte:calibration_conflict') } }))

describe('character failure presentation', () => {
  it.each(['en', 'fr'] as const)('shows a localized alert in %s', async locale => {
    const catalog = locale === 'en' ? en : fr
    applyLocalization(locale, { ...catalog, locale })
    render(<CharacterStatePage characterID={42} locale={locale} onBack={() => {}} />)
    expect(await screen.findByRole('alert')).toHaveTextContent(catalog.ui.error_calibration_conflict)
    expect(screen.queryByText('nte:calibration_conflict')).not.toBeInTheDocument()
  })
})
