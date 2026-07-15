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

## Production Evidence

- Accepted on 2026-07-15 against `https://www.jisudeng.com/` as a normal signed-in user, using deployment `6a57ae623c393b66819cbfb7` from merge commit `bb8ae7db428156e8fcaae86b2d3558340ed066e0`.
- The acceptance pass was read-only: no image generation, check-in, blindbox opening, quiz submission, team change, purchase, or recharge action was performed.
- The homepage statistics endpoint returned HTTP 200 with live, non-zero values including `total_requests: 8989` and `availability_pct: 99.97`.
- Monthly and daily Arena current endpoints returned HTTP 200 for both guests and the signed-in user. The signed-in response contained monthly `token_sum: 18330613, rank: 1` and daily `token_sum: 453750, rank: 1`.
- Image Studio selected API key `7` and model `gpt-image-2`. An incompatible-key model probe returned the expected HTTP 400 before fallback succeeded. A managed historical asset returned HTTP 200, rendered from one Blob URL, and no protected asset API URL was assigned to an image element.
- Check-in, blindbox, quiz, team, and models pages rendered their feature content. Their business endpoints returned HTTP 200 with no API 4xx/5xx responses during the observation window.

## Acceptance Matrix

| Area | Expected production result | Status |
| --- | --- | --- |
| Homepage statistics | Non-empty live request statistics | Passed: HTTP 200 with non-zero live statistics |
| Play Hub | Original hub content is present | Passed: hub content rendered and `/api/v1/play/hub` returned HTTP 200 |
| Arena guest | Current period and leaderboard load without login | Passed: monthly and daily current/leaderboard endpoints returned HTTP 200 |
| Arena signed-in | Monthly and daily personal score/rank are present | Passed: both periods returned non-zero scores and rank 1 |
| Image Studio copy | No raw `imageStudio.subtitle` key is visible | Passed: translated subtitle rendered |
| Image Studio key | A compatible key and image model are selected | Passed: key 7 and `gpt-image-2` selected after compatibility probing |
| Image Studio history | Historical thumbnails/previews load without direct 401 image requests | Passed: authenticated asset HTTP 200, one Blob image, zero protected API image sources |
| Other user pages | Check-in, blindbox, quiz, teams, and models load without API errors | Passed: feature APIs returned HTTP 200 with no observed 4xx/5xx |
