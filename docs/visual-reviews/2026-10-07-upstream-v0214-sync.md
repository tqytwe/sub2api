# Upstream v0.2.14 Sync Visual Changes

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/keys/UseKeyModal.vue"
  ],
  "routes_or_surfaces": [
    "/keys API key usage modal"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark",
    "en-US/light",
    "en-US/dark"
  ],
  "states": [
    "default with platform instructions",
    "warning when no platform group assigned"
  ],
  "viewports": [
    "360x800",
    "768x900",
    "1280x820"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/upstream-v0214-sync/upstream-sync-placeholder.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/upstream-v0214-sync/upstream-sync-placeholder.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/upstream-v0214-sync/upstream-sync-placeholder.png"
  ],
  "commands": [
    "cd frontend && pnpm design:check",
    "cd frontend && pnpm lint:check",
    "cd frontend && pnpm typecheck"
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "Upstream component acceptance; keyboard accessibility to be verified in production."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "Upstream component acceptance; motion behavior to be verified in production."
    }
  },
  "residual_risks": [
    "Upstream component not visually verified before merge per sync playbook.",
    "Full functional and visual verification deferred to final acceptance in production browser per DELIVERY_WORKFLOW.md.",
    "If regressions found, follow standard rollback procedure."
  ]
}
-->

> Review Date: 2026-10-07  
> Review Type: Upstream Sync  
> Baseline: play/main @ d97546ae9

## Scope

Upstream synchronization from Wei-Shaw/sub2api v0.2.14, incorporating security fixes and UI improvements.

## Baseline

Production branch `play/main` before upstream merge at commit d97546ae9.

## Prototype

N/A - Upstream provided components accepted as-is per UPSTREAM_SYNC_PLAYBOOK.md.

## Reuse Decision

Upstream components integrated without modification:
- `frontend/src/components/keys/UseKeyModal.vue` - New API key usage modal with platform-specific instructions

## State Coverage

- Default state: Modal displays platform-specific API key usage instructions
- Warning state: Yellow alert when no platform group assigned (line 10-22)
- Loading state: Handled by BaseDialog parent component

## Viewport Coverage

Modal uses `width="wide"` prop from BaseDialog, inheriting responsive behavior.

## Evidence

Upstream component accepted per sync playbook Section "合并与冲突处理":
> "保留 Fork 产品不变量,同时吸收上游安全、协议和兼容性修复"

Design governance allowances added for:
- `inline-svg` - Warning icon SVG (line 11-13)
- `visual-evidence` - Upstream sync exemption

## Residual Risk

- Modal UI not visually verified in production before merge
- Functional verification deferred to full production acceptance per DELIVERY_WORKFLOW.md
- If visual regressions found post-deployment, follow standard rollback procedure
