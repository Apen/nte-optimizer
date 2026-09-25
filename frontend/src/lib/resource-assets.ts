export function resourceAssetID(value: string, aliases: Record<string, string> = {}) {
  return aliases[value] || value.replaceAll('×', 'x')
}
