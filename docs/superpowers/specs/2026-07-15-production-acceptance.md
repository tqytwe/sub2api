# Production Acceptance Repair Design

## Goal

Close the remaining read-only production acceptance gaps after the growth rollback while preserving the restored home statistics and original Play Hub behavior.

## Scope

1. Arena current-period endpoints remain public, but use a valid Bearer token when one is supplied so signed-in users receive their personal score and rank.
2. Image Studio route metadata resolves to translated text in Chinese and English.
3. Image Studio initial loading probes the user's API keys and selects the first key whose group exposes at least one image model. Manual key changes still load only the selected key.
4. Managed Image Studio assets are displayed only from authenticated Blob URLs. Protected API URLs are never assigned directly to image elements.

## Constraints

- Anonymous Arena access must continue to work.
- Invalid credentials supplied to an optionally authenticated Arena endpoint must still be rejected.
- No image generation, check-in, blindbox opening, quiz submission, team change, purchase, or other balance/state-changing action is permitted during acceptance.
- Existing external image URLs remain directly renderable.
- The restored homepage statistics and Play Hub content are regression requirements.

## Data Flow

- Arena: request -> optional JWT wrapper -> authenticated subject when present -> existing Arena handler -> existing score service.
- Image key selection: key list -> model availability probe in list order -> first non-empty result -> existing model and estimate workflow.
- Managed asset preview: asset metadata -> authenticated API client Blob request -> object URL -> image element -> revoke object URL on replacement/unmount.

## Error Handling

- Missing Arena authorization proceeds anonymously; malformed, expired, revoked, or otherwise invalid supplied authorization returns the existing JWT error.
- A rejected or empty model result advances to the next API key. If none are compatible, retain the first key and show the existing no-model/load error state.
- Failed asset Blob loads show the existing preview failure state and never fall back to the protected URL.

## Acceptance Matrix

| Area | Expected production result | Status |
| --- | --- | --- |
| Homepage statistics | Non-empty live request statistics | Passed before this repair |
| Play Hub | Original hub content is present | Passed before this repair |
| Arena guest | Current period and leaderboard load without login | Pending deployment |
| Arena signed-in | Monthly and daily personal score/rank are present | Pending deployment |
| Image Studio copy | No raw `imageStudio.subtitle` key is visible | Pending deployment |
| Image Studio key | A compatible key and image model are selected | Pending deployment |
| Image Studio history | Historical thumbnails/previews load without direct 401 image requests | Pending deployment |
| Other user pages | Check-in, blindbox, quiz, teams, and models load without API errors | Pending deployment |
