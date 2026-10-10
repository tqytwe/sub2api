export default {
  walletNotice: 'These are existing wallet entries. A reservation alone does not prove final settlement.', walletOperations: { video_balance_hold: 'Video reservation', video_balance_capture: 'Video capture', video_balance_release: 'Video release', image_balance_hold: 'Image reservation', image_balance_capture: 'Image capture', image_balance_release: 'Image release' },
  phase: 'Phase', phases: { external_search: 'External search', ws_input: 'Continuous WS input', ws_observed: 'Observed automatic turn', request: 'Request', ws_connect: 'WS connection', ws_control: 'WS control' },
  walletTransaction: 'Wallet transaction',
  inputTokens: 'Input tokens', outputTokens: 'Output tokens',
  title: 'Request ledger', description: 'Execution, usage and settlement are tracked separately.',
  notice: 'Unknown usage is not a zero charge. Missing history before rollout cannot be reconstructed here.',
  back: 'Usage records', privateId: 'Private request ID', request: 'Request / time', identity: 'Identity', userId: 'User ID', keyId: 'Key ID', accountId: 'Account or credential ID',
  execution: 'Execution', usage: 'Usage', settlement: 'Settlement', attempts: 'Attempts', all: 'All states',
  start: 'From (local time)', end: 'Until (exclusive)', filter: 'Apply filters', reset: 'Reset', refresh: 'Refresh', detail: 'Details',
  loadError: 'The request ledger could not be loaded. Your filters are preserved; retry shortly.',
  invalidDates: 'The end must be later than the start.', retry: 'Retry', empty: 'No requests match these filters.',
  anonymous: 'Anonymous', parent: 'Parent request', began: 'Started', ended: 'Ended', unfinished: 'Not finished',
  output: 'Upstream response bytes observed', yes: 'Yes', no: 'No evidence', http: 'HTTP status', errorCode: 'Safe error code',
  account: 'Scheduled account', credential: 'Credential account', billing: 'Billing references', noBilling: 'No verified usage link yet.',
  usageLog: 'Usage record', pendingCost: 'Awaiting reconciliation', applied: 'Applied by this request', deduplicated: 'Existing settlement; no new charge',
  subscription: 'Subscription', package: 'Entitlement', amount: 'Verified billed amount', operations: 'Actions',
  states: { inflight: 'In progress', succeeded: 'Succeeded', failed: 'Failed', cancelled: 'Cancelled', timeout: 'Timed out', interrupted: 'Interrupted', not_applicable: 'Not applicable', pending: 'Awaiting usage', known: 'Known', usage_unknown: 'Unknown usage', not_required: 'Not required', settlement_pending: 'Pending settlement', settled: 'Settled' }
}
