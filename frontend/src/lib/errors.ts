import { t } from '../i18n'

const errorKeys: Record<string, string> = {
  operation_failed: 'error_operation_failed', data_missing: 'error_data_missing',
  data_unavailable: 'error_data_unavailable', account_required: 'error_account_required',
  calibration_conflict: 'error_calibration_conflict', equipment_unavailable: 'error_equipment_unavailable',
  cancelled: 'error_cancelled', scan_busy: 'scan_error_busy',
  scan_diagnostic_unavailable: 'scan_error_diagnostic_unavailable', scan_failed: 'scan_error_generic',
}

export function errorMessage(value: unknown, fallback = 'error_operation_failed') {
  const message = value instanceof Error ? value.message : String(value)
  const code = /^nte:([a-z_]+)$/.exec(message)?.[1]
  return t(code && errorKeys[code] ? errorKeys[code] : fallback)
}
