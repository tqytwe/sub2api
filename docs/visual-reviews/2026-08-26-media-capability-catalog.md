# Visual Review: media-capability-catalog

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/modelCatalog.ts",
    "frontend/src/i18n/locales/en/legacy/admin-channels.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh/legacy/admin-channels.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/utils/modelMediaCapabilities.ts",
    "frontend/src/views/admin/ModelCatalogView.vue"
  ],
  "routes_or_surfaces": ["/admin/model-plaza"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "focus-visible", "loading", "disabled", "empty", "error", "success"],
  "viewports": ["360x800", "768x900", "1280x800", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/media-capability-catalog/prototype-1440.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/media-capability-catalog/baseline-1440.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/media-capability-catalog/updated-1440.png"],
  "commands": [
    "cd frontend && pnpm exec playwright screenshot --viewport-size=1440,1100 file:///home/codex/worktrees/sub2api-media-integration-20260826/docs/visual-reviews/assets/media-capability-catalog/static-review-board.html?variant=baseline ../docs/visual-reviews/assets/media-capability-catalog/baseline-1440.png",
    "cd frontend && pnpm exec playwright screenshot --viewport-size=1440,1100 file:///home/codex/worktrees/sub2api-media-integration-20260826/docs/visual-reviews/assets/media-capability-catalog/static-review-board.html ../docs/visual-reviews/assets/media-capability-catalog/prototype-1440.png",
    "cd frontend && pnpm exec playwright screenshot --viewport-size=1440,1100 file:///home/codex/worktrees/sub2api-media-integration-20260826/docs/visual-reviews/assets/media-capability-catalog/static-review-board.html ../docs/visual-reviews/assets/media-capability-catalog/updated-1440.png",
    "cd frontend && pnpm exec vitest run src/utils/__tests__/modelMediaCapabilities.spec.ts src/views/admin/__tests__/ModelCatalogView.mediaCapabilities.spec.ts",
    "cd frontend && pnpm typecheck"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "The declaration toggle, native modality checkboxes, labelled inputs, and existing BaseDialog focus handling remain keyboard accessible."},
    "reduced_motion": {"status": "not-applicable", "reason": "This delivery adds no animation or continuous motion."}
  },
  "residual_risks": ["The evidence is a static review board, not a live authenticated browser capture. Final Chinese/English, light/dark and responsive validation requires the authorized candidate environment and the user's local browser acceptance."]
}
-->

## Scope

- Route and role: administrator-only `/admin/model-plaza` catalog editor.
- Boundary: adding or changing a media declaration never changes model names, groups, account mappings, white lists, prices, billing rules, or model order.
- Language: all system labels, errors, descriptions, and capability names have matching Chinese and English fragment keys. Model IDs, configured group names, and adapter identifiers remain source data.

## Baseline

- Existing catalog rows could have no machine-readable media declaration. Consumers therefore had no common contract and could fall back to name detection.
- `assets/media-capability-catalog/baseline-1440.png` records the former form boundary: it has no editable declaration section.

## Prototype

- `assets/media-capability-catalog/prototype-1440.png` uses the existing wide `BaseDialog`, native inputs, checkboxes, semantic borders, and current table density. It adds a declaration toggle and structured image/video fields only.
- A null declaration remains null. Enabling the control starts a draft; saving requires at least one modality, a version, an adapter, and an operation for each selected image/video modality.
- The preflight note distinguishes catalog declaration from actual availability: group access, an active mapped account, price and adapter support still decide whether a model is usable.

## Reuse Decision

- Reused: `AppLayout`, `TablePageLayout`, `DataTable`, `BaseDialog`, `GroupSelector`, native controls, `.input`, `.btn`, existing colour tokens, and existing catalogue save endpoint.
- No new page shell, card family, icon set, width rule, colour palette, or animation is introduced.
- Capability chips only surface a declared modality in the existing table; they do not infer capability from the model name.

## State Coverage

- Default: legacy row displays localized “not declared”; a declared row displays only its server-stored modalities.
- Focus and keyboard: declaration checkbox, modality checkboxes, text/numeric fields, and save/cancel controls use existing labelled native controls and dialog focus handling.
- Empty and error: empty declaration, missing version/adapter, noncanonical or duplicate operations, invalid image limits, and invalid video limits are blocked with an active-locale error. Unknown persisted extension data is cloned and retained on save.
- Loading, disabled, and success: existing catalog loading, save disabling, API result refresh, and toast handling remain unchanged.

## Viewport Coverage

- 360px: the wide dialog's existing responsive width stacks the field grids and leaves native controls reachable.
- 768px: image and video controls use two columns where space allows.
- 1280px and 1920px: the dialog remains inside the existing `BaseDialog` width contract; the table stays in the shared workspace frame.
- 200% zoom, exact dark-theme contrast, browser layout shifts, and real locale switching remain final browser-acceptance checks.

## Evidence

- The three PNG files are Playwright-rendered static review boards at 1440px. `file` verification confirms that each is a decodable PNG.
- Focused tests verify that legacy rows do not become an empty declaration and that an empty declaration cannot submit.
- `pnpm typecheck`, i18n symmetry/route tests, design governance, lint, full frontend tests, and build are part of the delivery gate and are recorded again after integration.

## Residual Risk

- This static review is not a substitute for authorized staging/production browser acceptance. Before deployment, validate the server-returned capability schema on Chinese and English screens at 360/768/1280/1920, with both themes and all guest/user/admin flows.
