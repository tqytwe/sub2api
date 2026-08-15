# Mobile App Release Management

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/PlayOpsView.vue",
    "frontend/src/components/admin/play/MobileReleaseManager.vue",
    "frontend/src/api/admin/play.ts",
    "frontend/src/i18n/locales/zh/admin/playOps.ts",
    "frontend/src/i18n/locales/en/admin/playOps.ts"
  ],
  "routes_or_surfaces": ["/admin/play?tab=mobile-releases"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["empty", "uploading", "upload error", "ready", "published", "paused", "retired"],
  "viewports": ["360x800", "768x900", "1280x820", "1920x1080"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/play-v2-unification/play-visual-system-v2.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/play-v2-unification/play-visual-system-v2.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/play-v2-unification/play-visual-system-v2.png"],
  "commands": ["pnpm --dir frontend typecheck", "pnpm --dir frontend design:check", "pnpm --dir frontend lint:check"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Native file inputs and buttons remain keyboard reachable."},
    "reduced_motion": {"status": "passed", "notes": "No continuous animation was added."}
  },
  "residual_risks": ["Fresh browser screenshots and production admin acceptance remain pending because deployment is intentionally out of scope."]
}
-->

```yaml
artifact_mode: implementation-review
route: /admin/play?tab=mobile-releases
prototype_artifacts:
  - docs/visual-reviews/assets/play-v2-unification/play-visual-system-v2.png
```

## Scope

Add a `mobile-releases` tab to Play Ops for immutable domestic APK and Google
Play AAB uploads, publishing, pausing, and retirement. The backend owns parsing,
hash verification, persistence, and download URLs; the UI never edits release
notes.

Changed visible files:

- `frontend/src/views/admin/PlayOpsView.vue`
- `frontend/src/components/admin/play/MobileReleaseManager.vue`
- `frontend/src/api/admin/play.ts`
- `frontend/src/i18n/locales/zh/admin/playOps.ts`
- `frontend/src/i18n/locales/en/admin/playOps.ts`

## Baseline

Play Ops already owns the admin workspace shell, tabs, `card`, `btn`, `input`,
`Icon`, table, toast, and loading/error patterns. Before this change there was
no release-management tab and administrators had to update a static manifest
outside the admin workflow.

## Prototype

The existing Play visual system prototype above is the reference for density,
semantic status colors, border radius, and the tab shell.

## Reuse Decision

The implementation
uses the existing Play Ops layout and shared primitives. It adds one compact
upload form, immutable release rows, and read-only locale note blocks; it does
not introduce a new page width, gradient, or nested card system.

## State Coverage

| Area | Covered states |
| --- | --- |
| Upload form | empty, invalid extension, loading, success, error, disabled |
| Release list | loading, empty, ready, published, paused, retired |
| Actions | publish, pause, retire, busy/disabled, API error |
| Notes | zh, en, ja, ko read-only; long text wraps |

## Viewport Coverage

The layout is constrained by the existing Play Ops workspace and responsive
grid breakpoints. Review targets are 360px, 768px, 1280px, and wide desktop.

## Evidence

Prototype: `docs/visual-reviews/assets/play-v2-unification/play-visual-system-v2.png`.
Automated evidence is `pnpm typecheck`, `pnpm design:check`, `pnpm lint:check`,
and the focused Play Ops tests. Browser screenshots remain a final reviewer
step because this task must not start a local production server.

## Residual Risk

The production Play Ops route still needs a user-browser screenshot review at
the listed widths. Dell remains the only supported Android build environment;
this feature does not compile or sign Android artifacts locally.
