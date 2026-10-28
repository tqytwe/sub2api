import type { CredentialOperationFailure } from '@/types'

const failureCodes = new Set([
  'phone_verification_required',
  'account_deactivated',
  'account_deleted',
  'account_retired',
  'requires_relogin',
  'password_rejected',
  'mfa_required',
  'security_check',
  'authorization_denied',
  'rate_limited',
  'timeout',
  'transport_error',
  'service_error',
  'unknown',
])

const actions: Record<string, string> = {
  phone_verification_required: 'phoneVerification',
  account_deactivated: 'contactProvider',
  account_deleted: 'contactProvider',
  account_retired: 'contactProvider',
  requires_relogin: 'reauthorize',
  password_rejected: 'updatePassword',
  mfa_required: 'completeVerification',
  security_check: 'reviewAuthorization',
  authorization_denied: 'reviewAuthorization',
  rate_limited: 'tryLater',
  timeout: 'tryLater',
  transport_error: 'tryLater',
  service_error: 'tryLater',
  unknown: 'reviewDiagnostics',
}

export interface SafeCredentialOperationFailure {
  code: string
  action: string
  occurred_at?: string
}

export function safeCredentialOperationFailure(
  failure?: Partial<CredentialOperationFailure> | null
): SafeCredentialOperationFailure | null {
  if (!failure) return null
  const code = typeof failure.code === 'string' && failureCodes.has(failure.code) ? failure.code : 'unknown'
  const occurredAt = failure.occurred_at
  const isValidDate = typeof occurredAt === 'string' && Number.isFinite(Date.parse(occurredAt))
  return { code, action: actions[code] || actions.unknown, ...(isValidDate ? { occurred_at: occurredAt } : {}) }
}
