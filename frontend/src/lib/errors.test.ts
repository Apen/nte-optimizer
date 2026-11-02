import { describe, expect, it } from 'vitest'
import { applyLocalization } from '../i18n'
import { errorMessage } from './errors'
import en from '../../../data/presentation/en.json'
import fr from '../../../data/presentation/fr.json'

describe('localized client errors', () => {
  it('localizes stable codes in both languages and hides technical errors', () => {
    for (const [locale, catalog] of [['en', en], ['fr', fr]] as const) {
      applyLocalization(locale, { ...catalog, locale })
      expect(errorMessage(new Error('nte:calibration_conflict'))).toBe(catalog.ui.error_calibration_conflict)
      expect(errorMessage('a private technical path')).toBe(catalog.ui.error_operation_failed)
      expect(errorMessage('nte:unknown_code')).toBe(catalog.ui.error_operation_failed)
    }
  })
})
