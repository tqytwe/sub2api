# Moderation Quick-Enable Visual Review

## Scope

- Route: `/admin/risk-control`
- Surface: disabled-user action in the existing moderation log and its confirmation dialog.
- Reuse: existing table row actions, `BaseDialog`, shared buttons, inputs, icons and toast.

## Baseline and Evidence

The existing route exposes a moderation-specific Unban action for auto-banned accounts. Quick enable is a separate admin user-status update. No authenticated browser capture of this route was available in this environment, so no baseline, prototype, or updated screenshot is claimed. The static review mockups created during development were removed rather than retained as product evidence.

The 360, 768, 1280 and wide-screen rendered route checks, theme and language screenshots, and browser interaction recording remain unverified. `pnpm` is unavailable; targeted tests and type checks were run through the installed frontend binaries.

## Prototype

No prototype image is retained because it would not represent the production route.

## Reuse Decision

The change uses the existing risk-control table, `BaseDialog`, form styles, icon registry, and app toast. No new visual pattern or color token was introduced.

## State Coverage

- Default: disabled users with an ID get an Enable user action; active or unidentified users do not.
- Loading: duplicate submissions and dialog dismissal are blocked while the request is pending.
- Error: preflight, update, unavailable status and readback failures stay in the dialog and retain the reason.
- Success: status is re-read before the dialog closes; the rows update without changing moderation history or counts.
- Keyboard, focus order, dark theme and rendered responsive layouts: not visually verified.

## Viewport Coverage

The actual route was not captured at 360, 768, 1280 or wide-screen widths. Responsive behavior remains for authenticated browser review.

## Evidence

No product screenshot or prototype evidence is available. Tests cover the interaction contract only; they do not replace visual inspection.

## Residual Risk

An authenticated browser review is still needed to verify the rendered route, dialog behavior, responsive layout and themes. The governance visual-evidence gate is not claimed to have passed.
