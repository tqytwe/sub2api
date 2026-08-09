# BEpusdt Pre-Deploy Checklist

This document is the release gate for the dedicated `bepusdt` payment
provider. It is intentionally separate from the provider configuration UI:
configuration is not evidence that the database, order contract, and callback
contract are aligned.

## Non-negotiable payment model

- The application owns the final payable amount in CNY.
- Balance recharge amounts may be entered by the user in CNY, but only before
  order creation and only inside the server-configured min/max range.
- Subscription amounts come from the selected database plan and its
  server-side settlement quote.
- The browser never supplies or edits a USDT amount.
- After order creation, the application sends the immutable CNY `pay_amount` to
  BEpusdt.
- BEpusdt converts that CNY amount to a time-limited USDT TRC20 quote and owns
  the hosted checkout screen.
- The browser opens only the HTTPS `payment_url` returned by BEpusdt. It must
  not build a wallet-only QR code from `token` because that omits the locked
  amount.

## Recharge amount release gate

The recharge flow intentionally keeps the existing custom CNY amount input. The
server validates that amount, applies coupons and fees, stores the resulting CNY
settlement fields on the order, and passes only that locked CNY `pay_amount` to
BEpusdt. The user must never enter, edit, or override the USDT amount.

Quick amount buttons on the recharge screen are shortcuts only. They are not a
pricing source and must not be treated as the authoritative order amount.

If the product later changes to fixed CNY recharge tiers, that must be a
separate backend-owned product contract. Do not convert this BEpusdt release
gate into a fixed-tier requirement.

## Field contract

| Layer | Field | Required invariant |
| --- | --- | --- |
| Frontend create request | `payment_type` | `bepusdt` only for this channel |
| Frontend create request | `amount` / `plan_id` | Recharge amount in CNY or selected subscription plan; never a user-entered USDT amount |
| Backend settlement | `list_amount` | Original CNY product/plan amount |
| Backend settlement | `gateway_base_amount` | `list_amount - discount_amount` |
| Backend settlement | `fee_amount` | Server-side fee in CNY |
| Backend settlement | `pay_amount` | `gateway_base_amount + fee_amount`, greater than zero |
| Backend order | `payment_currency` | `CNY` |
| Backend order | `provider_key` | `bepusdt` |
| Backend order | `provider_instance_id` | Numeric string referencing the selected BEpusdt instance |
| Backend snapshot | `schema_version` | `2` |
| Backend snapshot | `provider_key` | `bepusdt` |
| Backend snapshot | `provider_instance_id` | Exactly equals the order column |
| Backend snapshot | `currency` | `CNY` |
| Backend snapshot | secret fields | Must not contain `apiToken`, `notifyUrl`, or `returnUrl` |
| Provider create request | `amount` / `fiat` | Backend CNY amount and `CNY` |
| Provider create request | `trade_type` | `usdt.trc20` |
| Provider create response | `trade_id` / `payment_url` | Both present; URL is absolute HTTP(S) |
| Provider paid callback | `status` | Numeric `2` only; unknown/fractional values are rejected |
| Provider paid callback | `order_id` / `trade_id` | Must match the immutable order and stored provider trade number |
| Provider paid callback | `amount` | Exact CNY amount after minor-unit normalization |
| Provider paid callback | `actual_amount` / `block_transaction_id` | Required as chain evidence for a successful callback |

The callback signature is the BEpusdt MD5 signature over sorted, non-empty
fields (excluding `signature`) followed by the API token. A callback is not a
payment confirmation until signature, order identity, trade identity, CNY
amount, network, and chain evidence all pass.

## Database contract

Migration `backend/migrations/252_bepusdt_payment_contract.sql` must be applied
after the existing payment migrations. It fails closed when it finds invalid
historical BEpusdt rows and adds validated PostgreSQL CHECK constraints for:

- provider instance type, supported method, empty payment mode, CNY config,
  and disabled refund flags;
- order type, selected instance binding, CNY currency, positive amount, and
  schema-versioned provider snapshot;
- absence of BEpusdt secrets in the immutable order snapshot.

Run the read-only gate against the target database before enabling the channel:

```sh
psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 \
  -f backend/scripts/bepusdt-predeploy-check.sql
```

The command is acceptable only when every result set returns zero rows. The
script checks required migration records, validated BEpusdt constraints, exact
payment column types, the partial unique `out_trade_no` index, provider-instance
rows, order snapshot bindings, and accounting identities. It does not change
data.

Do not “repair” a non-zero result by editing financial rows during deployment.
Stop, export the affected IDs, investigate the originating code or migration,
and obtain a separate finance-approved remediation plan.

## Release acceptance

Before production deployment, all of the following are required:

1. The target branch contains the provider, route, migration, frontend hosted
   checkout behavior, and tests.
2. Backend focused tests, frontend payment tests, lint, typecheck, build, and
   fork-integrity checks pass.
3. The production read-only database check returns zero rows.
4. A small real recharge and a plan-based subscription are completed through
   BEpusdt, with CNY-to-USDT quote evidence and the on-chain transaction ID
   recorded outside source control.
5. The same signed callback is delivered twice and credits/fulfills exactly
   once. Invalid signatures, wrong order IDs, wrong trade IDs, wrong amounts,
   wrong network, and missing chain evidence are rejected.
6. Expiry/cancel releases any coupon lock and does not credit the account.
7. The user sees the hosted BEpusdt page and never sees an editable USDT amount.

No commit, push, merge, or deployment is authorized by this checklist. Those
actions require an explicit release instruction after the evidence above is
available.
