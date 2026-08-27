export type Translate = (key: string) => string

/**
 * Server-owned enum values must never leak as a raw translation key or a
 * provider-facing token when the frontend has not learned that value yet.
 */
export function localizedEnumOrUnknown(
  t: Translate,
  key: string,
  unknownKey = 'common.unknownStatus',
): string {
  const translated = t(key)
  return translated === key ? t(unknownKey) : translated
}
