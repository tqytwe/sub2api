# Production Acceptance Repairs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Repair the four remaining production acceptance gaps without changing restored homepage or Play Hub behavior.

**Architecture:** Reuse the existing required JWT middleware behind a small optional wrapper, keep image-key probing in a pure utility, add the missing locale contract, and prevent managed asset URLs from reaching image elements before authenticated Blob loading completes.

**Tech Stack:** Go, Gin, Vue 3, TypeScript, Vitest, Playwright, GitHub Actions

---

### Task 1: Arena optional authentication

**Files:**
- Modify: `backend/internal/server/middleware/jwt_auth.go`
- Modify: `backend/internal/server/routes/play.go`
- Test: `backend/internal/server/middleware/jwt_auth_test.go`

- [ ] Add tests proving a request without an Authorization header continues anonymously and a request with a header delegates to required JWT authentication.
- [ ] Run `go test ./internal/server/middleware -run OptionalJWT -count=1` and confirm the tests fail because the wrapper does not exist.
- [ ] Implement `OptionalJWTAuth` by bypassing required authentication only when the header is absent.
- [ ] Attach it to monthly and daily Arena current routes.
- [ ] Re-run the focused middleware tests and the server route tests.

### Task 2: Image Studio key selection

**Files:**
- Modify: `frontend/src/utils/imageStudioWorkspace.ts`
- Modify: `frontend/src/composables/useImageStudioWorkspace.ts`
- Test: `frontend/src/utils/__tests__/imageStudioWorkspace.spec.ts`

- [ ] Add tests proving failed and empty model probes advance to the first compatible key, while all-incompatible keys return no selection.
- [ ] Run the focused utility test and confirm the new assertions fail because the selector does not exist.
- [ ] Implement an ordered async selector returning both the selected key and its models.
- [ ] Use the selector only for initial workspace loading and preserve manual selection behavior.
- [ ] Re-run the focused utility and composable-adjacent tests.

### Task 3: Image Studio localization and authenticated thumbnails

**Files:**
- Modify: `frontend/src/i18n/locales/jisudeng-pages.zh.ts`
- Modify: `frontend/src/i18n/locales/jisudeng-pages.en.ts`
- Modify: `frontend/src/components/imageStudio/ImageStudioGallery.vue`
- Test: `frontend/src/components/imageStudio/__tests__/ImageStudioGallery.spec.ts`
- Test: `frontend/src/i18n/locales/__tests__/jisudeng-pages.spec.ts`

- [ ] Add a locale test for `imageStudio.subtitle` and a gallery test proving managed assets do not render their protected URL before the Blob request resolves.
- [ ] Run both tests and confirm the expected failures.
- [ ] Add translated subtitle values and return an empty thumbnail source for managed assets until an object URL exists.
- [ ] Re-run focused Image Studio tests.

### Task 4: Verification and production acceptance

**Files:**
- Modify: `docs/superpowers/specs/2026-07-15-production-acceptance.md`

- [ ] Run backend focused tests and the repository's broader backend suite.
- [ ] Run frontend lint check, typecheck, tests, and build.
- [ ] Run Fork Integrity.
- [ ] Commit intended files, push the repair branch, and open a PR to `play/main`.
- [ ] Wait for protected checks, merge, and verify the deployed commit.
- [ ] Use the normal-user production session for read-only acceptance and update every acceptance row with evidence.

