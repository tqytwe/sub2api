#!/usr/bin/env node

import { createHash } from "node:crypto";
import { existsSync, readFileSync, statSync } from "node:fs";
import { dirname, resolve } from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { inflateRawSync } from "node:zlib";

const scriptPath = fileURLToPath(import.meta.url);
const repoRoot = resolve(dirname(scriptPath), "..");

export const DEFAULT_ANDROID_APK_PATH = "frontend/public/downloads/jisudengchat-android.apk";
export const DEFAULT_ANDROID_MANIFEST_PATH = "frontend/public/downloads/android-version.json";
export const EXPECTED_ANDROID_PACKAGE_NAME = "com.jisudeng.chat";
export const EXPECTED_ANDROID_SIGNING_CERTIFICATE_SHA256 =
  "cd7abbd79daf6648a429ff34d7450b18cfb6b416e660b2f5169178e0a488627e";

function normalizeHex(value) {
  return typeof value === "string" ? value.trim().toLowerCase() : "";
}

function sha256Hex(bytes) {
  return createHash("sha256").update(bytes).digest("hex");
}

function isPlainObject(value) {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function validateAndroidVersionManifest(manifest) {
  const errors = [];
  if (!isPlainObject(manifest)) {
    return ["android-version.json must contain a JSON object"];
  }

  if (manifest.platform !== "android") {
    errors.push('android-version.json platform must be "android"');
  }
  if (typeof manifest.version !== "string" || manifest.version.trim() === "") {
    errors.push("android-version.json must declare version");
  }
  if (!Number.isInteger(manifest.versionCode) || manifest.versionCode <= 0) {
    errors.push("android-version.json versionCode must be a positive integer");
  }
  if (typeof manifest.apkUrl !== "string" || !manifest.apkUrl.includes(".apk")) {
    errors.push("android-version.json apkUrl must point to an APK");
  }
  if (!Number.isInteger(manifest.bytes) || manifest.bytes <= 0) {
    errors.push("android-version.json bytes must be a positive integer");
  }
  if (!/^[a-f0-9]{64}$/i.test(String(manifest.sha256 ?? ""))) {
    errors.push("android-version.json sha256 must be a SHA-256 hex digest");
  }
  if (typeof manifest.packageName !== "string" || manifest.packageName.trim() === "") {
    errors.push(
      "android-version.json must declare packageName so releases cannot change app identity silently",
    );
  }
  if (
    typeof manifest.signingCertificateSha256 !== "string" ||
    !/^[a-f0-9]{64}$/i.test(manifest.signingCertificateSha256)
  ) {
    errors.push(
      "android-version.json must declare signingCertificateSha256 so releases cannot change signing identity silently",
    );
  }

  return errors;
}

function readJsonFile(path) {
  try {
    return JSON.parse(readFileSync(path, "utf8"));
  } catch (error) {
    throw new Error(`failed to read JSON ${path}: ${error.message}`);
  }
}

function readZipEntries(path) {
  const zip = readFileSync(path);
  const eocdOffset = zip.lastIndexOf(Buffer.from([0x50, 0x4b, 0x05, 0x06]));
  if (eocdOffset < 0) throw new Error(`cannot find ZIP end of central directory in ${path}`);

  const entryCount = zip.readUInt16LE(eocdOffset + 10);
  const centralDirectoryOffset = zip.readUInt32LE(eocdOffset + 16);
  const entries = new Map();
  let offset = centralDirectoryOffset;

  for (let index = 0; index < entryCount; index += 1) {
    if (zip.readUInt32LE(offset) !== 0x02014b50) {
      throw new Error(`invalid ZIP central directory entry in ${path}`);
    }

    const method = zip.readUInt16LE(offset + 10);
    const compressedSize = zip.readUInt32LE(offset + 20);
    const uncompressedSize = zip.readUInt32LE(offset + 24);
    const nameLength = zip.readUInt16LE(offset + 28);
    const extraLength = zip.readUInt16LE(offset + 30);
    const commentLength = zip.readUInt16LE(offset + 32);
    const localHeaderOffset = zip.readUInt32LE(offset + 42);
    const name = zip
      .subarray(offset + 46, offset + 46 + nameLength)
      .toString("utf8")
      .replaceAll("\\", "/");

    entries.set(name, {
      method,
      compressedSize,
      uncompressedSize,
      localHeaderOffset,
    });

    offset += 46 + nameLength + extraLength + commentLength;
  }

  return {
    readEntry(name) {
      const entry = entries.get(name);
      if (!entry) return null;
      const localOffset = entry.localHeaderOffset;
      if (zip.readUInt32LE(localOffset) !== 0x04034b50) {
        throw new Error(`invalid ZIP local header for ${name}`);
      }
      const localNameLength = zip.readUInt16LE(localOffset + 26);
      const localExtraLength = zip.readUInt16LE(localOffset + 28);
      const dataStart = localOffset + 30 + localNameLength + localExtraLength;
      const compressed = zip.subarray(dataStart, dataStart + entry.compressedSize);

      if (entry.method === 0) return compressed;
      if (entry.method === 8) {
        const inflated = inflateRawSync(compressed);
        if (inflated.length !== entry.uncompressedSize) {
          throw new Error(`inflated ZIP entry ${name} has unexpected size`);
        }
        return inflated;
      }
      throw new Error(`unsupported ZIP compression method ${entry.method} for ${name}`);
    },
    names() {
      return [...entries.keys()];
    },
  };
}

function readUtf8Length(buffer, offset) {
  let value = buffer[offset];
  let length = 1;
  if ((value & 0x80) !== 0) {
    value = ((value & 0x7f) << 8) | buffer[offset + 1];
    length = 2;
  }
  return { value, length };
}

function readUtf16Length(buffer, offset) {
  let value = buffer.readUInt16LE(offset);
  let length = 2;
  if ((value & 0x8000) !== 0) {
    value = ((value & 0x7fff) << 16) | buffer.readUInt16LE(offset + 2);
    length = 4;
  }
  return { value, length };
}

function parseStringPool(buffer, offset) {
  if (buffer.readUInt16LE(offset) !== 0x0001) {
    throw new Error("AndroidManifest.xml string pool is missing");
  }

  const chunkSize = buffer.readUInt32LE(offset + 4);
  const stringCount = buffer.readUInt32LE(offset + 8);
  const flags = buffer.readUInt32LE(offset + 16);
  const stringsStart = buffer.readUInt32LE(offset + 20);
  const utf8 = (flags & 0x00000100) !== 0;
  const strings = [];

  for (let index = 0; index < stringCount; index += 1) {
    const stringOffset = offset + stringsStart + buffer.readUInt32LE(offset + 28 + index * 4);
    if (utf8) {
      const utf16Length = readUtf8Length(buffer, stringOffset);
      const byteLength = readUtf8Length(buffer, stringOffset + utf16Length.length);
      const start = stringOffset + utf16Length.length + byteLength.length;
      strings.push(buffer.subarray(start, start + byteLength.value).toString("utf8"));
    } else {
      const utf16Length = readUtf16Length(buffer, stringOffset);
      const start = stringOffset + utf16Length.length;
      strings.push(buffer.subarray(start, start + utf16Length.value * 2).toString("utf16le"));
    }
  }

  return { nextOffset: offset + chunkSize, strings };
}

function stringAt(strings, index) {
  return index === 0xffffffff ? "" : strings[index] ?? "";
}

function typedValue(attributeStrings, buffer, attributeOffset) {
  const dataType = buffer.readUInt8(attributeOffset + 15);
  const data = buffer.readUInt32LE(attributeOffset + 16);
  if (dataType === 0x03) return stringAt(attributeStrings, data);
  if (dataType >= 0x10 && dataType <= 0x1f) return data;
  if (dataType === 0x12) return data !== 0;
  return "";
}

export function parseAndroidManifestIdentity(manifestBytes) {
  if (manifestBytes.readUInt16LE(0) !== 0x0003) {
    throw new Error("AndroidManifest.xml is not a binary XML document");
  }

  const xmlHeaderSize = manifestBytes.readUInt16LE(2);
  let offset = xmlHeaderSize;
  let strings = [];
  let resourceIds = [];

  while (offset < manifestBytes.length) {
    const chunkType = manifestBytes.readUInt16LE(offset);
    const headerSize = manifestBytes.readUInt16LE(offset + 2);
    const chunkSize = manifestBytes.readUInt32LE(offset + 4);
    if (chunkType === 0x0001) {
      const parsed = parseStringPool(manifestBytes, offset);
      strings = parsed.strings;
      offset = parsed.nextOffset;
      continue;
    }
    if (chunkType === 0x0180) {
      resourceIds = [];
      for (let cursor = offset + headerSize; cursor < offset + chunkSize; cursor += 4) {
        resourceIds.push(manifestBytes.readUInt32LE(cursor));
      }
      offset += chunkSize;
      continue;
    }

    if (chunkType === 0x0102) {
      const elementName = stringAt(strings, manifestBytes.readUInt32LE(offset + 20));
      const attributeStart = manifestBytes.readUInt16LE(offset + 24);
      const attributeSize = manifestBytes.readUInt16LE(offset + 26);
      const attributeCount = manifestBytes.readUInt16LE(offset + 28);

      if (elementName === "manifest") {
        const identity = {
          packageName: "",
          versionCode: undefined,
          versionName: "",
        };

        for (let index = 0; index < attributeCount; index += 1) {
          const attributeOffset = offset + 16 + attributeStart + index * attributeSize;
          const attributeNameIndex = manifestBytes.readUInt32LE(attributeOffset + 4);
          const attributeName = stringAt(strings, attributeNameIndex);
          const attributeResourceId = resourceIds[attributeNameIndex];
          const rawValue = stringAt(strings, manifestBytes.readUInt32LE(attributeOffset + 8));
          const value = rawValue || typedValue(strings, manifestBytes, attributeOffset);

          if (attributeName === "package") identity.packageName = String(value);
          if (
            (attributeName === "versionCode" || attributeResourceId === 0x0101021b) &&
            Number.isInteger(value)
          ) {
            identity.versionCode = value;
          }
          if (attributeName === "versionName" || attributeResourceId === 0x0101021c) {
            identity.versionName = String(value);
          }
        }

        return identity;
      }
    }

    offset += chunkSize;
  }

  return { packageName: "", versionCode: undefined, versionName: "" };
}

export function parseAndroidManifestPackageName(manifestBytes) {
  return parseAndroidManifestIdentity(manifestBytes).packageName;
}

function readDerLength(buffer, offset) {
  const first = buffer[offset];
  if ((first & 0x80) === 0) return { length: first, bytes: 1 };
  const lengthBytes = first & 0x7f;
  if (lengthBytes === 0 || lengthBytes > 4) {
    throw new Error("unsupported DER length encoding");
  }
  let length = 0;
  for (let index = 0; index < lengthBytes; index += 1) {
    length = (length << 8) | buffer[offset + 1 + index];
  }
  return { length, bytes: 1 + lengthBytes };
}

function readDerElement(buffer, offset) {
  const tag = buffer[offset];
  const lengthInfo = readDerLength(buffer, offset + 1);
  const contentStart = offset + 1 + lengthInfo.bytes;
  const end = contentStart + lengthInfo.length;
  if (end > buffer.length) throw new Error("DER element exceeds input length");
  return {
    tag,
    start: offset,
    contentStart,
    end,
    constructed: (tag & 0x20) !== 0,
  };
}

function readDerChildren(buffer, element) {
  const children = [];
  let offset = element.contentStart;
  while (offset < element.end) {
    const child = readDerElement(buffer, offset);
    children.push(child);
    offset = child.end;
  }
  return children;
}

function findCertificateDer(buffer, offset = 0, end = buffer.length) {
  let cursor = offset;
  while (cursor < end) {
    const element = readDerElement(buffer, cursor);
    if (element.tag === 0x30) {
      const children = readDerChildren(buffer, element);
      if (
        children.length >= 3 &&
        children[0].tag === 0x30 &&
        children[1].tag === 0x30 &&
        children[2].tag === 0x03
      ) {
        return buffer.subarray(element.start, element.end);
      }
    }
    if (element.constructed) {
      const nested = findCertificateDer(buffer, element.contentStart, element.end);
      if (nested) return nested;
    }
    cursor = element.end;
  }
  return null;
}

function signingCertificateSha256FromPkcs7(bytes) {
  const certificate = findCertificateDer(bytes);
  if (!certificate) throw new Error("could not find an X.509 certificate in META-INF signature");
  return sha256Hex(certificate);
}

export function inspectAndroidApk(apkPath) {
  const zip = readZipEntries(apkPath);
  const manifestBytes = zip.readEntry("AndroidManifest.xml");
  const capacitorConfigBytes = zip.readEntry("assets/capacitor.config.json");
  const signatureEntry = zip
    .names()
    .find((name) => /^META-INF\/[^/]+\.(RSA|DSA|EC)$/i.test(name));

  if (!signatureEntry) {
    throw new Error("APK does not contain a v1 signing certificate entry in META-INF");
  }

  let manifestIdentity = {
    packageName: "",
    versionCode: undefined,
    versionName: "",
  };
  if (manifestBytes) {
    manifestIdentity = parseAndroidManifestIdentity(manifestBytes);
  }
  if (!manifestIdentity.packageName && capacitorConfigBytes) {
    const capacitorConfig = JSON.parse(capacitorConfigBytes.toString("utf8"));
    manifestIdentity.packageName = String(capacitorConfig.appId ?? "");
  }

  return {
    packageName: manifestIdentity.packageName,
    versionCode: manifestIdentity.versionCode,
    versionName: manifestIdentity.versionName,
    signingCertificateSha256: signingCertificateSha256FromPkcs7(zip.readEntry(signatureEntry)),
  };
}

export function validateAndroidRelease({
  apkPath,
  manifestPath,
  expectedPackageName = EXPECTED_ANDROID_PACKAGE_NAME,
  expectedSigningCertificateSha256 = EXPECTED_ANDROID_SIGNING_CERTIFICATE_SHA256,
  inspectApk = inspectAndroidApk,
} = {}) {
  const errors = [];
  const resolvedApkPath = resolve(repoRoot, apkPath ?? DEFAULT_ANDROID_APK_PATH);
  const resolvedManifestPath = resolve(repoRoot, manifestPath ?? DEFAULT_ANDROID_MANIFEST_PATH);

  if (!existsSync(resolvedApkPath)) errors.push(`APK file is missing: ${resolvedApkPath}`);
  if (!existsSync(resolvedManifestPath)) {
    errors.push(`android-version.json is missing: ${resolvedManifestPath}`);
  }
  if (errors.length > 0) return errors;

  const manifest = readJsonFile(resolvedManifestPath);
  errors.push(...validateAndroidVersionManifest(manifest));

  const normalizedExpectedPackageName = String(expectedPackageName).trim();
  const normalizedExpectedSigningSha = normalizeHex(expectedSigningCertificateSha256);
  const manifestSigningSha = normalizeHex(manifest.signingCertificateSha256);

  if (manifest.packageName && manifest.packageName !== normalizedExpectedPackageName) {
    errors.push(
      `android-version.json packageName must remain ${normalizedExpectedPackageName}, got ${manifest.packageName}`,
    );
  }
  if (manifestSigningSha && manifestSigningSha !== normalizedExpectedSigningSha) {
    errors.push(
      `android-version.json signingCertificateSha256 must remain ${normalizedExpectedSigningSha}, got ${manifestSigningSha}`,
    );
  }

  const actualBytes = statSync(resolvedApkPath).size;
  if (Number.isInteger(manifest.bytes) && manifest.bytes !== actualBytes) {
    errors.push(`android-version.json bytes must match APK size ${actualBytes}, got ${manifest.bytes}`);
  }

  const actualSha256 = sha256Hex(readFileSync(resolvedApkPath));
  const manifestSha256 = normalizeHex(manifest.sha256);
  if (manifestSha256 && manifestSha256 !== actualSha256) {
    errors.push(`android-version.json sha256 must match APK digest ${actualSha256}, got ${manifestSha256}`);
  }

  try {
    const apkIdentity = inspectApk(resolvedApkPath);
    if (apkIdentity.packageName !== normalizedExpectedPackageName) {
      errors.push(
        `APK packageName must remain ${normalizedExpectedPackageName}, got ${apkIdentity.packageName || "(empty)"}`,
      );
    }

    const apkSigningSha = normalizeHex(apkIdentity.signingCertificateSha256);
    if (apkSigningSha !== normalizedExpectedSigningSha) {
      errors.push(
        `APK signingCertificateSha256 must remain ${normalizedExpectedSigningSha}, got ${apkSigningSha || "(empty)"}`,
      );
    }
  } catch (error) {
    errors.push(`failed to inspect APK identity: ${error.message}`);
  }

  return errors;
}

function main() {
  const errors = validateAndroidRelease();
  if (errors.length > 0) {
    for (const error of errors) {
      console.error(`[android-release] ${error}`);
    }
    process.exitCode = 1;
    return;
  }

  console.log(
    `[android-release] OK package=${EXPECTED_ANDROID_PACKAGE_NAME} signingCertificateSha256=${EXPECTED_ANDROID_SIGNING_CERTIFICATE_SHA256}`,
  );
}

if (process.argv[1] && resolve(process.argv[1]) === scriptPath) {
  main();
}
