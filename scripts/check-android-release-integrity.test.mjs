import assert from "node:assert/strict";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import {
  validateAndroidRelease,
  validateAndroidVersionManifest,
} from "./check-android-release-integrity.mjs";

const stableIdentity = {
  packageName: "com.jisudeng.chat",
  signingCertificateSha256:
    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
};

function writeFixtureRelease(manifestOverrides = {}, apkBytes = "apk-bytes") {
  const dir = mkdtempSync(join(tmpdir(), "sub2api-android-release-"));
  const apkPath = join(dir, "jisudengchat-android.apk");
  writeFileSync(apkPath, apkBytes);

  const manifest = {
    platform: "android",
    version: "2.0.18",
    versionCode: 218,
    apkUrl: "/downloads/jisudengchat-android.apk?v=2.0.18-218",
    bytes: Buffer.byteLength(apkBytes),
    sha256:
      "1e10ba560383b17472b4cf72fef8f9e76c66815a3e6ae8c5a9b0c5e696b0bdf8",
    minAndroidVersion: "8.0",
    releaseDate: "2026-07-24",
    ...stableIdentity,
    ...manifestOverrides,
  };
  const manifestPath = join(dir, "android-version.json");
  writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);

  return { dir, apkPath, manifestPath };
}

test("android version manifest declares a stable install identity", () => {
  assert.deepEqual(
    validateAndroidVersionManifest({
      platform: "android",
      version: "2.0.18",
      versionCode: 218,
      apkUrl: "/downloads/jisudengchat-android.apk?v=2.0.18-218",
      size: "6.4 MB",
      bytes: 6644212,
      sha256:
        "2d6f169140efc1f37fd0dfd2230b77dab8ec5ace80baab62b50700a27455d626",
      minAndroidVersion: "8.0",
      releaseDate: "2026-07-24",
      ...stableIdentity,
    }),
    [],
  );

  assert.deepEqual(
    validateAndroidVersionManifest({
      platform: "android",
      version: "2.0.18",
      versionCode: 218,
      apkUrl: "/downloads/jisudengchat-android.apk?v=2.0.18-218",
      bytes: 6644212,
      sha256:
        "2d6f169140efc1f37fd0dfd2230b77dab8ec5ace80baab62b50700a27455d626",
    }),
    [
      "android-version.json must declare packageName so releases cannot change app identity silently",
      "android-version.json must declare signingCertificateSha256 so releases cannot change signing identity silently",
    ],
  );
});

test("android release integrity rejects drift that would prevent in-place upgrades", () => {
  const release = writeFixtureRelease({
    packageName: "com.jisudeng.chat.beta",
    signingCertificateSha256:
      "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
  });
  try {
    const errors = validateAndroidRelease({
      apkPath: release.apkPath,
      manifestPath: release.manifestPath,
      expectedPackageName: stableIdentity.packageName,
      expectedSigningCertificateSha256:
        stableIdentity.signingCertificateSha256,
      inspectApk: () => ({
        packageName: "com.jisudeng.chat.beta",
        signingCertificateSha256:
          "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
      }),
    });

    assert.deepEqual(errors, [
      "android-version.json packageName must remain com.jisudeng.chat, got com.jisudeng.chat.beta",
      "android-version.json signingCertificateSha256 must remain 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef, got ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
      "APK packageName must remain com.jisudeng.chat, got com.jisudeng.chat.beta",
      "APK signingCertificateSha256 must remain 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef, got ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
    ]);
  } finally {
    rmSync(release.dir, { recursive: true, force: true });
  }
});

test("android release integrity checks APK bytes and digest against the manifest", () => {
  const release = writeFixtureRelease({
    bytes: 1,
    sha256:
      "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
  });
  try {
    const errors = validateAndroidRelease({
      apkPath: release.apkPath,
      manifestPath: release.manifestPath,
      expectedPackageName: stableIdentity.packageName,
      expectedSigningCertificateSha256:
        stableIdentity.signingCertificateSha256,
      inspectApk: () => stableIdentity,
    });

    assert.deepEqual(errors, [
      "android-version.json bytes must match APK size 9, got 1",
      "android-version.json sha256 must match APK digest 1e10ba560383b17472b4cf72fef8f9e76c66815a3e6ae8c5a9b0c5e696b0bdf8, got ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
    ]);
  } finally {
    rmSync(release.dir, { recursive: true, force: true });
  }
});
