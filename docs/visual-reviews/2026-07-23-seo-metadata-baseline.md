# SEO Metadata Baseline Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/index.html",
    "frontend/src/components/home/LmspeedBadge.vue"
  ],
  "routes_or_surfaces": [
    "public home HTML shell",
    "public search result metadata",
    "home LMSpeed badge link"
  ],
  "languages_and_themes": [
    "zh-CN light static board",
    "zh-CN dark compatibility unchanged through existing shell"
  ],
  "states": [
    "default metadata",
    "crawler-visible robots intent",
    "social preview metadata",
    "external badge link relationship"
  ],
  "viewports": [
    "390x844",
    "768x1024",
    "1280x860",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/seo-metadata-baseline/prototype-seo-metadata-baseline.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/seo-metadata-baseline/baseline-seo-metadata-baseline.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/seo-metadata-baseline/updated-seo-metadata-baseline.png"
  ],
  "commands": [
    "python3 generated static SEO metadata review boards with PIL",
    "pnpm design:check",
    "pnpm vitest run src/components/home/__tests__/LmspeedBadge.spec.ts",
    "pnpm build",
    "make test",
    "GOFLAGS=-buildvcs=false make build",
    "./scripts/check-fork-integrity.sh",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "No keyboard interaction or visible control behavior changed."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "No animation or motion behavior changed."
    }
  },
  "residual_risks": [
    "These artifacts are static review boards rather than browser screenshots, so final browser acceptance still needs a real home page screenshot after deployment.",
    "Search snippets are controlled by crawlers and may lag until Google recrawls the updated title, sitemap, and robots files."
  ]
}
-->

## Scope

This review covers crawler-visible metadata in the public HTML shell and the home page LMSpeed badge link relationship. It does not redesign the home page, navigation, responsive layout, buttons, cards, typography scale, or authenticated console surfaces.

## Baseline

The baseline search signal was thin: the title did not consistently include the product category, the HTML shell lacked canonical and social preview metadata, and the LMSpeed badge link passed a normal followed external link even though that page can compete for the same brand query.

Baseline artifact: `docs/visual-reviews/assets/seo-metadata-baseline/baseline-seo-metadata-baseline.png`.

## Prototype

The prototype keeps the visible layout unchanged and improves crawler context only: a stronger brand plus category title, an expanded description, canonical and social preview hints, and a nofollow relationship on the LMSpeed badge link while preserving the visible badge.

Prototype artifact: `docs/visual-reviews/assets/seo-metadata-baseline/prototype-seo-metadata-baseline.png`.

Approval status: narrow SEO metadata repair under the existing frontend governance rule that requires evidence for changed visible files.

Scope boundary: HTML metadata, crawler route exposure, and external link relationship only.

## Reuse Decision

The implementation reuses the existing public shell and existing `LmspeedBadge` component. No new visual component, route frame, page section, decorative treatment, or interaction pattern is introduced.

## State Coverage

Default metadata now names the brand and product category. The crawler-visible route state is supported by separate `robots.txt` and `sitemap.xml` backend responses. Social preview state is covered by Open Graph and Twitter metadata. The external badge state remains visually identical while changing the link relationship to `nofollow`.

## Viewport Coverage

The changed files do not alter rendered sizing, spacing, position, responsive breakpoints, or typography. The static review board records the same viewport intent used by recent frontend governance evidence: mobile, tablet, desktop, and wide desktop.

## Evidence

Updated artifact: `docs/visual-reviews/assets/seo-metadata-baseline/updated-seo-metadata-baseline.png`.

Commands run:

```bash
python3 generated static SEO metadata review boards with PIL
pnpm design:check
pnpm vitest run src/components/home/__tests__/LmspeedBadge.spec.ts
pnpm build
make test
GOFLAGS=-buildvcs=false make build
./scripts/check-fork-integrity.sh
git diff --check
```

## Residual Risk

The artifacts are static review boards rather than browser screenshots. After deployment, the home page should still be checked in a real browser and Search Console should request recrawling for `https://www.jisudeng.com/`, `https://www.jisudeng.com/robots.txt`, and `https://www.jisudeng.com/sitemap.xml`.
