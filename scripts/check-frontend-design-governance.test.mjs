import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import {
  mkdtempSync,
  mkdirSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import {
  REQUIRED_RULE_NAMES,
  collectAddedLines,
  findRuleViolations,
  hasValidAllowance,
  resolveBase,
  validateEvidenceRecords,
} from "./check-frontend-design-governance.mjs";

function git(cwd, args) {
  return execFileSync("git", args, { cwd, encoding: "utf8" }).trim();
}

test("governance rule set cannot silently shrink", () => {
  assert.deepEqual(REQUIRED_RULE_NAMES, [
    "inline-svg",
    "transition-all",
    "large-radius",
    "page-shell-ownership",
    "raw-color",
    "focus-reset",
    "decorative-gradient",
    "continuous-motion",
  ]);
});

test("invalid explicit base fails closed", () => {
  const repo = mkdtempSync(join(tmpdir(), "sub2api-design-base-"));
  try {
    git(repo, ["init"]);
    git(repo, ["config", "user.email", "design@example.com"]);
    git(repo, ["config", "user.name", "Design Test"]);
    writeFileSync(join(repo, "file.txt"), "baseline\n");
    git(repo, ["add", "file.txt"]);
    git(repo, ["commit", "-m", "baseline"]);

    assert.throws(
      () => resolveBase(repo, "definitely-not-a-ref", "origin/play/main"),
      /cannot resolve Git base/,
    );
  } finally {
    rmSync(repo, { recursive: true, force: true });
  }
});

test("diff parser preserves added file and line numbers", () => {
  const parsed = collectAddedLines(
    [
      "diff --git a/frontend/src/views/Test.vue b/frontend/src/views/Test.vue",
      "--- a/frontend/src/views/Test.vue",
      "+++ b/frontend/src/views/Test.vue",
      "@@ -2,0 +3,2 @@",
      "+<div>",
      "+  hello",
    ].join("\n"),
  );

  assert.deepEqual(parsed.get("frontend/src/views/Test.vue"), [
    { line: 3, source: "<div>" },
    { line: 4, source: "  hello" },
  ]);
});

test("allowances require a comment and a concrete reason", () => {
  assert.equal(
    hasValidAllowance(
      ["// design-governance-allow: raw-color - provider brand asset"],
      1,
      "raw-color",
    ),
    true,
  );
  assert.equal(
    hasValidAllowance(
      ["design-governance-allow: raw-color"],
      1,
      "raw-color",
    ),
    false,
  );
});

test("each documented visual rule has an executable negative probe", () => {
  const cases = [
    ["frontend/src/components/Test.vue", "<svg>", "inline-svg"],
    ["frontend/src/components/Test.vue", "class=\"transition-all\"", "transition-all"],
    ["frontend/src/components/Test.vue", "border-radius: 24px;", "large-radius"],
    ["frontend/src/views/Test.vue", "class=\"mx-auto max-w-7xl\"", "page-shell-ownership"],
    ["frontend/src/components/Test.vue", "color: rgb(1 2 3);", "raw-color"],
    ["frontend/src/components/Test.vue", "outline: none;", "focus-reset"],
    ["frontend/src/components/Test.vue", "background: linear-gradient(red, blue);", "decorative-gradient"],
    ["frontend/src/components/Test.vue", "animation: spin 1s infinite;", "continuous-motion"]
  ];
  for (const [file, source, expectedRule] of cases) {
    const violations = findRuleViolations(
      file,
      [{ line: 1, source }],
      [source],
    );
    assert.ok(
      violations.some((item) => item.rule === expectedRule),
      `${expectedRule} did not reject its negative probe`,
    );
  }

  assert.deepEqual(
    findRuleViolations(
      "frontend/src/components/Test.vue",
      [{ line: 1, source: "class=\"outline-none focus-visible:ring-2\"" }],
      ["class=\"outline-none focus-visible:ring-2\""],
    ),
    [],
  );
});

test("visual evidence requires structured artifacts and file coverage", () => {
  const repo = mkdtempSync(join(tmpdir(), "sub2api-design-evidence-"));
  try {
    const reviewDir = join(repo, "docs/visual-reviews");
    const assetDir = join(reviewDir, "assets");
    mkdirSync(assetDir, { recursive: true });
    const pngHeader = Buffer.from("89504e470d0a1a0a", "hex");
    writeFileSync(join(assetDir, "before.png"), pngHeader);
    writeFileSync(join(assetDir, "after.png"), pngHeader);

    const review = "docs/visual-reviews/2026-07-21-test.md";
    writeFileSync(
      join(repo, review),
      [
        "# Visual Review: Test",
        "",
        "<!-- visual-review-manifest",
        JSON.stringify({
          schema_version: 1,
          changed_files: ["frontend/src/views/Test.vue"],
          routes_or_surfaces: ["/test"],
          languages_and_themes: ["zh-CN/light"],
          states: ["default", "focus-visible"],
          viewports: ["360x800", "1280x800"],
          baseline_artifacts: ["docs/visual-reviews/assets/before.png"],
          updated_artifacts: ["docs/visual-reviews/assets/after.png"],
          commands: ["playwright screenshot /test"],
          checks: {
            keyboard: { status: "passed" },
            reduced_motion: { status: "passed" }
          },
          residual_risks: ["No known residual risk after local review."]
        }),
        "-->",
        "",
        "## Scope",
        "Test route.",
        "## Baseline",
        "Existing route.",
        "## Reuse Decision",
        "Shared frame.",
        "## State Coverage",
        "Default and focus.",
        "## Viewport Coverage",
        "Mobile and desktop.",
        "## Evidence",
        "Artifacts above.",
        "## Residual Risk",
        "None found."
      ].join("\n"),
    );

    assert.deepEqual(
      validateEvidenceRecords({
        repoRoot: repo,
        visualFiles: ["frontend/src/views/Test.vue"],
        evidenceFiles: [review]
      }),
      [],
    );

    const violations = validateEvidenceRecords({
      repoRoot: repo,
      visualFiles: ["frontend/src/views/Uncovered.vue"],
      evidenceFiles: [review]
    });
    assert.ok(violations.some((item) => item.rule === "visual-evidence"));
  } finally {
    rmSync(repo, { recursive: true, force: true });
  }
});
