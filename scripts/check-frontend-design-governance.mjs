#!/usr/bin/env node

import { execFileSync } from "node:child_process";
import { existsSync, readFileSync, statSync } from "node:fs";
import { relative, resolve } from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { inflateSync } from "node:zlib";

const scriptPath = fileURLToPath(import.meta.url);
const repoRoot = resolve(scriptPath, "../..");

const requiredFiles = [
  "docs/FRONTEND_DESIGN_SYSTEM.md",
  "docs/FRONTEND_EXPERIENCE_REMEDIATION_PLAN.md",
  "docs/frontend-design-governance.json",
  "docs/visual-reviews/README.md",
  "docs/visual-reviews/TEMPLATE.md",
  "frontend/AGENTS.md",
  "scripts/check-frontend-design-governance.test.mjs"
];

const scanPaths = [
  "frontend/src",
  "frontend/public",
  "frontend/index.html",
  "frontend/tailwind.config.js"
];

const tokenFiles = new Set([
  "frontend/tailwind.config.js",
  "frontend/src/style.css",
  "frontend/src/styles/tokens.css"
]);

export const REQUIRED_RULE_NAMES = [
  "inline-svg",
  "transition-all",
  "large-radius",
  "page-shell-ownership",
  "raw-color",
  "focus-reset",
  "decorative-gradient",
  "continuous-motion"
];

function runGit(cwd, args, description) {
  try {
    return execFileSync("git", args, {
      cwd,
      encoding: "utf8",
      stdio: ["ignore", "pipe", "pipe"]
    }).trimEnd();
  } catch (error) {
    const stderr = error?.stderr?.toString().trim();
    throw new Error(
      `[design-governance] ${description} failed: git ${args.join(" ")}` +
        (stderr ? `\n${stderr}` : ""),
    );
  }
}

function tryGit(cwd, args) {
  try {
    return runGit(cwd, args, "optional Git lookup");
  } catch {
    return "";
  }
}

export function resolveBase(
  cwd,
  explicitRef = process.env.DESIGN_BASE_REF,
  remoteRef = "origin/play/main",
) {
  let candidate = explicitRef;
  if (!candidate) {
    if (tryGit(cwd, ["rev-parse", "--verify", `${remoteRef}^{commit}`])) {
      candidate = runGit(
        cwd,
        ["merge-base", "HEAD", remoteRef],
        `resolve merge-base against ${remoteRef}`,
      );
    } else {
      candidate = tryGit(cwd, ["rev-parse", "--verify", "HEAD^"]);
    }
  }

  if (!candidate) {
    throw new Error(
      "[design-governance] cannot resolve Git base; fetch origin/play/main " +
        "or set DESIGN_BASE_REF to a commit",
    );
  }

  try {
    const baseSha = runGit(
      cwd,
      ["rev-parse", "--verify", `${candidate}^{commit}`],
      `resolve base ${candidate}`,
    );
    const headSha = runGit(
      cwd,
      ["rev-parse", "--verify", "HEAD^{commit}"],
      "resolve HEAD",
    );
    if (
      process.env.GITHUB_EVENT_NAME === "pull_request" &&
      baseSha === headSha
    ) {
      throw new Error(
        "[design-governance] pull request base resolves to HEAD; refusing a zero-diff check",
      );
    }
    return baseSha;
  } catch (error) {
    if (String(error?.message).includes("pull request base resolves")) throw error;
    throw new Error(
      `[design-governance] cannot resolve Git base "${candidate}" to a commit`,
      { cause: error },
    );
  }
}

export function collectAddedLines(diff) {
  const addedByFile = new Map();
  let currentFile = "";
  let currentLine = 0;

  for (const text of diff.split("\n")) {
    if (text.startsWith("+++ b/")) {
      currentFile = text.slice(6);
      continue;
    }
    if (text === "+++ /dev/null") {
      currentFile = "";
      continue;
    }
    if (text.startsWith("@@")) {
      const match = text.match(/\+(\d+)/);
      currentLine = match ? Number(match[1]) : 0;
      continue;
    }
    if (currentFile && text.startsWith("+") && !text.startsWith("+++")) {
      const lines = addedByFile.get(currentFile) ?? [];
      lines.push({ line: currentLine, source: text.slice(1) });
      addedByFile.set(currentFile, lines);
      currentLine += 1;
      continue;
    }
    if (currentFile && !text.startsWith("-")) currentLine += 1;
  }

  return addedByFile;
}

function isCommentAllowance(text, ruleName) {
  const escaped = ruleName.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return new RegExp(
    String.raw`(?:\/\/|\/\*|\*|<!--|#)\s*design-governance-allow:\s*${escaped}\s*-\s*\S.{7,}`,
    "i",
  ).test(text);
}

export function hasValidAllowance(fullLines, lineNumber, ruleName) {
  const current = fullLines[Math.max(0, lineNumber - 1)] ?? "";
  const previous = fullLines[Math.max(0, lineNumber - 2)] ?? "";
  return (
    isCommentAllowance(current, ruleName) ||
    isCommentAllowance(previous, ruleName)
  );
}

function hasFocusReplacement(fullLines, lineNumber, source) {
  if (/\bfocus-visible:(?:ring|outline|shadow)/i.test(source)) return true;
  const start = Math.max(0, lineNumber - 4);
  const end = Math.min(fullLines.length, lineNumber + 8);
  const context = fullLines.slice(start, end).join("\n");
  return (
    /:focus-visible|focus-visible:/i.test(context) &&
    /(?:outline|box-shadow|ring)(?:-|\s*:)/i.test(context)
  );
}

function isRouteView(file) {
  return (
    /^frontend\/src\/views\/.*\.(?:vue|tsx?)$/.test(file) ||
    /^frontend\/src\/features\/.*(?:View|Page)\.(?:vue|tsx?)$/.test(file)
  );
}

const rules = [
  {
    name: "inline-svg",
    pattern: /<svg(?:\s|>)/i,
    message: "use the shared Icon component instead of adding inline functional SVG",
    exempt: (file) => file === "frontend/src/components/icons/Icon.vue"
  },
  {
    name: "transition-all",
    pattern: /\btransition-all\b|transition\s*:\s*all\b/i,
    message: "transition only the properties that actually change"
  },
  {
    name: "large-radius",
    pattern:
      /\brounded-(?:xl|2xl|3xl)\b|\brounded-\[(?:1[3-9]|[2-9]\d)px\]|border-radius\s*:\s*(?:(?:1[3-9]|[2-9]\d)px|(?:0\.(?:8[1-9]|9\d)|[1-9]\d*(?:\.\d+)?)rem)/i,
    message: "operational cards use 8px and overlays use 12px"
  },
  {
    name: "page-shell-ownership",
    pattern:
      /\b(?:max-w-(?:\[[^\]]+\]|(?:xs|sm|md|lg|xl|2xl|3xl|4xl|5xl|6xl|7xl|full|none))|mx-auto|min-h-screen|h-screen|overflow-y-(?:auto|scroll))\b|max-width\s*:|margin\s*:\s*(?:[^;]*\s)?auto(?:\s|;|$)/i,
    message: "route view width, centering, height and scrolling belong to the shared page frame",
    exempt: (file) => !isRouteView(file)
  },
  {
    name: "raw-color",
    pattern:
      /#[0-9a-f]{3,8}\b|\b(?:rgb|rgba|hsl|hsla|hwb|lab|lch|oklab|oklch|color)\s*\(/i,
    message: "use semantic color tokens instead of adding raw color values",
    exempt: (file) => tokenFiles.has(file)
  },
  {
    name: "focus-reset",
    pattern: /\boutline-none\b|outline\s*:\s*(?:none|0)\b/i,
    message: "do not remove focus indication without an accessible focus-visible replacement",
    exempt: (_file, source, fullLines, line) =>
      hasFocusReplacement(fullLines, line, source)
  },
  {
    name: "decorative-gradient",
    pattern: /\bbg-gradient-to-\w+\b|(?:linear|radial)-gradient\s*\(/i,
    message: "use semantic surfaces; artistic gradients require an explicit reviewed reason"
  },
  {
    name: "continuous-motion",
    pattern:
      /\banimation\s*:[^;]*(?:infinite|shimmer|pulse|spin)|\banimate-(?:pulse|spin|ping)\b|<animate(?:Transform)?\b/i,
    message: "continuous motion requires a reduced-motion-safe reviewed exception"
  }
];

export function findRuleViolations(file, lines, fullLines) {
  const violations = [];
  for (const { line, source } of lines) {
    for (const rule of rules) {
      if (rule.exempt?.(file, source, fullLines, line)) continue;
      if (!rule.pattern.test(source)) continue;
      if (hasValidAllowance(fullLines, line, rule.name)) continue;
      violations.push({
        file,
        line,
        rule: rule.name,
        message: rule.message,
        source: source.trim()
      });
    }
  }
  return violations;
}

function isVisualFile(file) {
  if (
    /(?:^|\/)(?:__tests__|tests)\//.test(file) ||
    /\.(?:spec|test)\.[cm]?[jt]sx?$/.test(file)
  ) {
    return false;
  }
  return (
    /^frontend\/src\/.*\.(?:vue|css|scss|less|svg)$/.test(file) ||
    /^frontend\/src\/(?:components|views|features|layouts|router|i18n\/locales|content|assets)\/.*\.[cm]?[jt]sx?$/.test(
      file,
    ) ||
    /^frontend\/public\//.test(file) ||
    file === "frontend/index.html" ||
    file === "frontend/tailwind.config.js"
  );
}

const requiredEvidenceSections = [
  "## Scope",
  "## Baseline",
  "## Prototype",
  "## Reuse Decision",
  "## State Coverage",
  "## Viewport Coverage",
  "## Evidence",
  "## Residual Risk"
];
const MIN_VISUAL_ARTIFACT_WIDTH = 120;
const MIN_VISUAL_ARTIFACT_HEIGHT = 90;
const PNG_SIGNATURE = Buffer.from("89504e470d0a1a0a", "hex");
let crcTable;

function parseEvidenceManifest(content) {
  const match = content.match(
    /<!--\s*visual-review-manifest\s*([\s\S]*?)-->/i,
  );
  if (!match) throw new Error("missing visual-review-manifest JSON block");
  return JSON.parse(match[1].trim());
}

function nonEmptyStringArray(value) {
  return (
    Array.isArray(value) &&
    value.length > 0 &&
    value.every((item) => typeof item === "string" && item.trim().length >= 3)
  );
}

function validCheck(value) {
  if (!value || typeof value !== "object") return false;
  if (value.status === "passed") return true;
  return (
    value.status === "not-applicable" &&
    typeof value.reason === "string" &&
    value.reason.trim().length >= 8
  );
}

function validArtifactMode(manifest) {
  if (manifest.artifact_mode === "browser-capture") {
    return manifest.commands.some((command) =>
      /\b(?:playwright|browser|screenshot|recording|video)\b/i.test(command),
    );
  }
  if (manifest.artifact_mode === "static-review-board") {
    return manifest.residual_risks.some((risk) =>
      /browser|screenshot|final acceptance|浏览器|截图|最终验收/i.test(risk),
    );
  }
  return false;
}

function crc32(bytes) {
  if (!crcTable) {
    crcTable = new Uint32Array(256);
    for (let n = 0; n < 256; n += 1) {
      let c = n;
      for (let k = 0; k < 8; k += 1) {
        c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
      }
      crcTable[n] = c >>> 0;
    }
  }

  let crc = 0xffffffff;
  for (const byte of bytes) {
    crc = crcTable[(crc ^ byte) & 0xff] ^ (crc >>> 8);
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function pngBitsPerPixel(colorType, bitDepth) {
  const channelsByType = new Map([
    [0, 1],
    [2, 3],
    [3, 1],
    [4, 2],
    [6, 4]
  ]);
  const allowedDepths = new Map([
    [0, new Set([1, 2, 4, 8, 16])],
    [2, new Set([8, 16])],
    [3, new Set([1, 2, 4, 8])],
    [4, new Set([8, 16])],
    [6, new Set([8, 16])]
  ]);
  const channels = channelsByType.get(colorType);
  if (!channels || !allowedDepths.get(colorType)?.has(bitDepth)) return 0;
  return channels * bitDepth;
}

function validatePngArtifact(bytes, artifact) {
  if (!bytes.subarray(0, 8).equals(PNG_SIGNATURE)) {
    return `artifact has an invalid media signature: ${artifact}`;
  }
  if (bytes.length < 45) {
    return `artifact is too small to be a valid PNG: ${artifact}`;
  }

  let offset = 8;
  let width = 0;
  let height = 0;
  let bitDepth = 0;
  let colorType = 0;
  let interlaceMethod = 0;
  let seenIHDR = false;
  let seenIDAT = false;
  let seenIEND = false;
  const idatChunks = [];

  while (offset + 12 <= bytes.length) {
    const length = bytes.readUInt32BE(offset);
    const type = bytes.subarray(offset + 4, offset + 8).toString("ascii");
    const dataStart = offset + 8;
    const dataEnd = dataStart + length;
    const crcOffset = dataEnd;
    const nextOffset = crcOffset + 4;
    if (nextOffset > bytes.length) {
      return `artifact has an invalid PNG chunk length: ${artifact}`;
    }
    const expectedCrc = bytes.readUInt32BE(crcOffset);
    const actualCrc = crc32(bytes.subarray(offset + 4, dataEnd));
    if (actualCrc !== expectedCrc) {
      return `artifact has an invalid PNG ${type} CRC: ${artifact}`;
    }

    if (!seenIHDR && type !== "IHDR") {
      return `artifact PNG is missing a leading IHDR chunk: ${artifact}`;
    }
    if (type === "IHDR") {
      if (seenIHDR || length !== 13) {
        return `artifact has an invalid PNG IHDR chunk: ${artifact}`;
      }
      width = bytes.readUInt32BE(dataStart);
      height = bytes.readUInt32BE(dataStart + 4);
      bitDepth = bytes[dataStart + 8];
      colorType = bytes[dataStart + 9];
      const compressionMethod = bytes[dataStart + 10];
      const filterMethod = bytes[dataStart + 11];
      interlaceMethod = bytes[dataStart + 12];
      if (
        width < 1 ||
        height < 1 ||
        !pngBitsPerPixel(colorType, bitDepth) ||
        compressionMethod !== 0 ||
        filterMethod !== 0 ||
        ![0, 1].includes(interlaceMethod)
      ) {
        return `artifact has unsupported PNG image metadata: ${artifact}`;
      }
      seenIHDR = true;
    } else if (type === "IDAT") {
      seenIDAT = true;
      idatChunks.push(bytes.subarray(dataStart, dataEnd));
    } else if (type === "IEND") {
      if (length !== 0) return `artifact has an invalid PNG IEND chunk: ${artifact}`;
      seenIEND = true;
      if (nextOffset !== bytes.length) {
        return `artifact has trailing bytes after PNG IEND: ${artifact}`;
      }
      break;
    }
    offset = nextOffset;
  }

  if (!seenIHDR || !seenIDAT || !seenIEND) {
    return `artifact is not a complete PNG image: ${artifact}`;
  }
  if (width < MIN_VISUAL_ARTIFACT_WIDTH || height < MIN_VISUAL_ARTIFACT_HEIGHT) {
    return `artifact dimensions are too small: ${artifact} (${width}x${height})`;
  }

  try {
    const inflated = inflateSync(Buffer.concat(idatChunks));
    if (interlaceMethod === 0) {
      const bitsPerPixel = pngBitsPerPixel(colorType, bitDepth);
      const expectedBytes = (Math.ceil((width * bitsPerPixel) / 8) + 1) * height;
      if (inflated.length !== expectedBytes) {
        return `artifact PNG pixel data length is invalid: ${artifact}`;
      }
    }
  } catch {
    return `artifact PNG pixel data cannot be decoded: ${artifact}`;
  }

  return "";
}

function validateArtifact(cwd, artifact) {
  if (
    typeof artifact !== "string" ||
    !/^docs\/visual-reviews\/assets\/[a-zA-Z0-9._/-]+\.(?:png|jpe?g|webp|gif|mp4|webm|json|zip)$/.test(
      artifact,
    ) ||
    artifact.includes("..")
  ) {
    return `invalid artifact path: ${String(artifact)}`;
  }
  const fullPath = resolve(cwd, artifact);
  if (!existsSync(fullPath) || !statSync(fullPath).isFile()) {
    return `artifact does not exist: ${artifact}`;
  }
  if (statSync(fullPath).size === 0) return `artifact is empty: ${artifact}`;
  const extension = artifact.split(".").pop().toLowerCase();
  if (["png", "jpg", "jpeg", "webp", "gif", "mp4", "webm"].includes(extension)) {
    const bytes = readFileSync(fullPath);
    if (extension === "png") {
      return validatePngArtifact(bytes, artifact);
    }
    const validSignature =
      ((extension === "jpg" || extension === "jpeg") &&
        bytes.subarray(0, 3).equals(Buffer.from("ffd8ff", "hex"))) ||
      (extension === "gif" && bytes.subarray(0, 4).toString() === "GIF8") ||
      (extension === "webp" &&
        bytes.subarray(0, 4).toString() === "RIFF" &&
        bytes.subarray(8, 12).toString() === "WEBP") ||
      (extension === "mp4" && bytes.subarray(4, 8).toString() === "ftyp") ||
      (extension === "webm" &&
        bytes.subarray(0, 4).equals(Buffer.from("1a45dfa3", "hex")));
    if (!validSignature) return `artifact has an invalid media signature: ${artifact}`;
  }
  return "";
}

export function validateEvidenceRecords({
  repoRoot: cwd,
  visualFiles,
  evidenceFiles
}) {
  const violations = [];
  const coveredFiles = new Set();

  if (visualFiles.length > 0 && evidenceFiles.length === 0) {
    violations.push({
      file: visualFiles.join(", "),
      line: 0,
      rule: "visual-evidence",
      message: "visible UI changes require a structured visual review record",
      source: "Add docs/visual-reviews/YYYY-MM-DD-<slug>.md and rendered artifacts."
    });
    return violations;
  }

  for (const file of evidenceFiles) {
    const fullPath = resolve(cwd, file);
    if (!existsSync(fullPath)) {
      violations.push({
        file,
        line: 0,
        rule: "visual-evidence",
        message: "visual review record was deleted or cannot be read",
        source: file
      });
      continue;
    }

    const content = readFileSync(fullPath, "utf8");
    if (/\bTODO\b|replace-with|<slug>|待填写|截图待补/i.test(content)) {
      violations.push({
        file,
        line: 0,
        rule: "visual-evidence",
        message: "visual review still contains template placeholders",
        source: "Replace every placeholder with observed evidence."
      });
      continue;
    }
    const missing = requiredEvidenceSections.filter(
      (section) => !content.includes(section),
    );
    if (missing.length > 0) {
      violations.push({
        file,
        line: 0,
        rule: "visual-evidence",
        message: "visual review record is missing required sections",
        source: missing.join(", ")
      });
      continue;
    }

    let manifest;
    try {
      manifest = parseEvidenceManifest(content);
    } catch (error) {
      violations.push({
        file,
        line: 0,
        rule: "visual-evidence",
        message: "visual review manifest is invalid",
        source: error.message
      });
      continue;
    }

    const fields = [
      "changed_files",
      "routes_or_surfaces",
      "languages_and_themes",
      "states",
      "viewports",
      "prototype_artifacts",
      "baseline_artifacts",
      "updated_artifacts",
      "commands",
      "residual_risks"
    ];
    const invalidFields = fields.filter(
      (field) => !nonEmptyStringArray(manifest[field]),
    );
    if (
      manifest.schema_version !== 1 ||
      !validArtifactMode(manifest) ||
      invalidFields.length > 0 ||
      manifest.viewports.length < 2 ||
      !manifest.viewports.every((viewport) => /^\d{3,4}x\d{3,4}$/.test(viewport)) ||
      !manifest.prototype_artifacts.some((artifact) =>
        /\.(?:png|jpe?g|webp|gif)$/i.test(artifact),
      ) ||
      !manifest.baseline_artifacts.some((artifact) =>
        /\.(?:png|jpe?g|webp|gif|mp4|webm)$/i.test(artifact),
      ) ||
      !manifest.updated_artifacts.some((artifact) =>
        /\.(?:png|jpe?g|webp|gif|mp4|webm)$/i.test(artifact),
      ) ||
      !validCheck(manifest.checks?.keyboard) ||
      !validCheck(manifest.checks?.reduced_motion)
    ) {
      violations.push({
        file,
        line: 0,
        rule: "visual-evidence",
        message: "visual review manifest is incomplete",
        source:
          `invalid fields: ${invalidFields.join(", ") || "checks/schema/viewports/artifact_mode"}`
      });
      continue;
    }

    const artifactErrors = [
      ...manifest.prototype_artifacts,
      ...manifest.baseline_artifacts,
      ...manifest.updated_artifacts
    ]
      .map((artifact) => validateArtifact(cwd, artifact))
      .filter(Boolean);
    if (artifactErrors.length > 0) {
      violations.push({
        file,
        line: 0,
        rule: "visual-evidence",
        message: "visual review artifacts are invalid",
        source: artifactErrors.join("; ")
      });
      continue;
    }

    manifest.changed_files.forEach((changedFile) => coveredFiles.add(changedFile));
  }

  const uncovered = visualFiles.filter((file) => !coveredFiles.has(file));
  if (uncovered.length > 0) {
    violations.push({
      file: uncovered.join(", "),
      line: 0,
      rule: "visual-evidence",
      message: "visible files are not mapped by any visual review manifest",
      source: "List every visible changed file in manifest.changed_files."
    });
  }

  return violations;
}

function main() {
  process.chdir(repoRoot);

  for (const file of requiredFiles) {
    if (!existsSync(resolve(repoRoot, file))) {
      throw new Error(`[design-governance] missing required file: ${file}`);
    }
  }

  const policy = JSON.parse(
    readFileSync(resolve(repoRoot, "docs/frontend-design-governance.json"), "utf8"),
  );
  if (
    policy.schema_version !== 1 ||
    policy.policy_status?.prototype_visual_evidence !== "enforced" ||
    JSON.stringify(policy.required_rule_names) !==
      JSON.stringify(REQUIRED_RULE_NAMES) ||
    rules.map((rule) => rule.name).join("\n") !== REQUIRED_RULE_NAMES.join("\n")
  ) {
    throw new Error(
      "[design-governance] machine policy and executable rule set are out of sync",
    );
  }

  const base = resolveBase(repoRoot);
  const diff = runGit(
    repoRoot,
    [
      "diff",
      "--unified=0",
      "--no-ext-diff",
      "--find-renames",
      base,
      "--",
      ...scanPaths
    ],
    "read frontend diff",
  );
  const addedByFile = collectAddedLines(diff);
  const untrackedScanFiles = runGit(
    repoRoot,
    ["ls-files", "--others", "--exclude-standard", "--", ...scanPaths],
    "list untracked frontend files",
  )
    .split("\n")
    .filter(Boolean);

  for (const file of untrackedScanFiles) {
    const content = readFileSync(resolve(repoRoot, file), "utf8");
    addedByFile.set(
      file,
      content.split("\n").map((source, index) => ({ line: index + 1, source })),
    );
  }

  const changedFiles = new Set(
    runGit(
      repoRoot,
      ["diff", "--name-only", "--find-renames", base],
      "list changed files",
    )
      .split("\n")
      .filter(Boolean),
  );
  for (const file of runGit(
    repoRoot,
    ["ls-files", "--others", "--exclude-standard"],
    "list all untracked files",
  )
    .split("\n")
    .filter(Boolean)) {
    changedFiles.add(file);
  }

  const violations = [];
  for (const [file, lines] of addedByFile) {
    const fullPath = resolve(repoRoot, file);
    const fullLines = existsSync(fullPath)
      ? readFileSync(fullPath, "utf8").split("\n")
      : [];

    violations.push(...findRuleViolations(file, lines, fullLines));
  }

  const visualFiles = [...changedFiles].filter(isVisualFile);
  const evidenceFiles = [...changedFiles].filter(
    (file) =>
      /^docs\/visual-reviews\/\d{4}-\d{2}-\d{2}-[a-z0-9-]+\.md$/.test(file) &&
      !file.endsWith("/README.md") &&
      !file.endsWith("/TEMPLATE.md"),
  );
  violations.push(
    ...validateEvidenceRecords({ repoRoot, visualFiles, evidenceFiles }),
  );

  if (violations.length > 0) {
    console.error("[design-governance] new visual-contract violations found:\n");
    for (const violation of violations) {
      const displayFile = violation.file.includes(", ")
        ? violation.file
        : relative(repoRoot, resolve(repoRoot, violation.file));
      console.error(
        `- ${displayFile}${violation.line ? `:${violation.line}` : ""} ` +
          `[${violation.rule}]: ${violation.message}`,
      );
      console.error(`  ${violation.source}`);
    }
    console.error(
      "\nUse the shared design system, or add a reviewed comment " +
        "`design-governance-allow: <rule> - <concrete reason>`.",
    );
    process.exitCode = 1;
    return;
  }

  console.log(
    `[design-governance] passed (base: ${base}; changed files: ${changedFiles.size}; ` +
      `checked files: ${addedByFile.size}; visual files: ${visualFiles.length}; ` +
      `evidence records: ${evidenceFiles.length})`,
  );
}

if (process.argv[1] && resolve(process.argv[1]) === scriptPath) {
  try {
    main();
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
