#!/usr/bin/env node

import { readFile, writeFile } from "node:fs/promises";
import { dirname, isAbsolute, resolve } from "node:path";
import process from "node:process";

const DEFAULT_MIN_LOCATIONS = 5;
const DEFAULT_CONCURRENCY = 1;

function usage() {
  return `Dry-run or import generated country maps through the admin API.

Usage:
  node scripts/import-country-maps.mjs --manifest <file> [options]

Options:
  --api-base <url>              API base URL, e.g. https://geoduels.io (required with --import)
  --access-token <token>        Admin app access JWT (or GEODUELS_ADMIN_ACCESS_TOKEN)
  --import                      Write maps to the admin API; default is dry-run only
  --confirm-production          Required when --import targets a non-localhost API
  --concurrency <n>             Concurrent imports (default: ${DEFAULT_CONCURRENCY})
  --min-locations <n>           Minimum valid locations per map (default: ${DEFAULT_MIN_LOCATIONS})
  --report <file>               Write JSON validation/import report
  --countries <list>            Comma-separated allowlist, e.g. US,CA,PR
  --help                        Show this help

The script validates every file locally before import, resolves thumbnail keys
from the generated thumbnail catalog by ISO country code, and posts each map as
multipart/form-data to /v1/admin/maps/official/import.
`;
}

function parseArgs(argv) {
  const options = {
    import: false,
    confirmProduction: false,
    concurrency: DEFAULT_CONCURRENCY,
    minLocations: DEFAULT_MIN_LOCATIONS,
  };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === "--help" || arg === "-h") options.help = true;
    else if (arg === "--import") options.import = true;
    else if (arg === "--confirm-production") options.confirmProduction = true;
    else if (arg.startsWith("--")) {
      const key = arg.slice(2).replace(/-([a-z])/g, (_, letter) => letter.toUpperCase());
      const value = argv[++i];
      if (!value || value.startsWith("--")) throw new Error(`Missing value for ${arg}`);
      options[key] = value;
    } else {
      throw new Error(`Unexpected argument: ${arg}`);
    }
  }
  options.accessToken ||= process.env.GEODUELS_ADMIN_ACCESS_TOKEN;
  options.concurrency = positiveInteger(options.concurrency, "--concurrency");
  options.minLocations = positiveInteger(options.minLocations, "--min-locations");
  options.countries = parseCountries(options.countries);
  if (!options.help && !options.manifest) throw new Error("--manifest is required");
  if (options.import && (!options.apiBase || !options.accessToken)) {
    throw new Error("--api-base and --access-token (or GEODUELS_ADMIN_ACCESS_TOKEN) are required with --import");
  }
  if (options.import && !options.confirmProduction && options.apiBase && !isLocalAPI(options.apiBase)) {
    throw new Error("--confirm-production is required when importing to a non-localhost API");
  }
  return options;
}

function positiveInteger(value, name) {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 1) throw new Error(`${name} must be a positive integer`);
  return parsed;
}

function parseCountries(value) {
  if (!value) return null;
  const countries = value.split(",").map((entry) => entry.trim().toUpperCase()).filter(Boolean);
  if (countries.length === 0) throw new Error("--countries must include at least one code");
  return new Set(countries);
}

function isLocalAPI(raw) {
  try {
    const url = new URL(raw);
    return ["localhost", "127.0.0.1", "::1"].includes(url.hostname);
  } catch {
    return false;
  }
}

async function readJson(path) {
  return JSON.parse(await readFile(path, "utf8"));
}

function resolveMaybeRelative(path, baseDir) {
  return isAbsolute(path) ? path : resolve(baseDir, path);
}

function loadThumbnailCatalog() {
  return readJson(resolve("apps/web/shared/mapthumbnails/catalog.generated.json"));
}

function inspectLocations(value) {
  const raw = Array.isArray(value) ? value : value?.customCoordinates;
  if (!Array.isArray(raw)) throw new Error("map JSON must be an array or include customCoordinates");

  const seenPanos = new Set();
  const seenCoords = new Set();
  let valid = 0;
  let rejected = 0;
  for (const row of raw) {
    const lat = row?.lat;
    const lng = row?.lng;
    const panoId = String(row?.panoId || row?.extra?.panoId || "").trim();
    if (!Number.isFinite(lat) || lat < -90 || lat > 90 || !Number.isFinite(lng) || lng < -180 || lng > 180) {
      rejected += 1;
      continue;
    }
    const coord = `${lat.toFixed(8)}:${lng.toFixed(8)}`;
    if (seenCoords.has(coord)) {
      rejected += 1;
      continue;
    }
    if (panoId) {
      if (seenPanos.has(panoId) || panoId.length > 255) {
        rejected += 1;
        continue;
      }
      seenPanos.add(panoId);
    }
    seenCoords.add(coord);
    valid += 1;
  }
  return { raw: raw.length, valid, rejected };
}

function normalizeManifest(manifest) {
  const maps = Array.isArray(manifest) ? manifest : manifest?.maps;
  if (!Array.isArray(maps)) throw new Error("manifest must be an array or an object with a maps array");
  return maps;
}

function thumbnailResolver(catalog) {
  const byKey = new Map(catalog.thumbnails.map((item) => [item.key, item]));
  const byCode = new Map(catalog.thumbnails.filter((item) => item.code).map((item) => [item.code.toUpperCase(), item]));
  return (entry) => {
    if (entry.thumbnailKey && byKey.has(entry.thumbnailKey)) return entry.thumbnailKey;
    const codeMatch = byCode.get(String(entry.countryCode || entry.officialRegionCode || "").toUpperCase());
    if (codeMatch) return codeMatch.key;
    return "";
  };
}

async function validateEntry(entry, manifestDir, resolveThumbnail, options) {
  const file = resolveMaybeRelative(entry.file, manifestDir);
  const parsed = await readJson(file);
  const counts = inspectLocations(parsed);
  const thumbnailKey = resolveThumbnail(entry);
  const problems = [];
  if (!entry.countryCode) problems.push("missing_country_code");
  if (!entry.mapKey) problems.push("missing_map_key");
  if (!entry.displayName) problems.push("missing_display_name");
  if (!thumbnailKey) problems.push("missing_thumbnail");
  if (counts.valid < options.minLocations) problems.push("too_few_locations");
  return {
    ...entry,
    file,
    thumbnailKey,
    counts,
    ok: problems.length === 0,
    problems,
  };
}

async function importEntry(entry, options) {
  const body = new FormData();
  const bytes = await readFile(entry.file);
  body.set("file", new Blob([bytes], { type: "application/json" }), `${entry.countryCode || "map"}.json`);
  body.set("mapKey", entry.mapKey);
  body.set("displayName", entry.displayName);
  body.set("description", entry.description || "");
  body.set("visibility", entry.visibility || "public");
  body.set("difficulty", entry.difficulty || "normal");
  body.set("thumbnailKey", entry.thumbnailKey);
  body.set("thumbnailVariant", String(entry.thumbnailVariant || 1));
  body.set("officialRegionType", entry.officialRegionType || "country");
  body.set("officialRegionCode", entry.officialRegionCode || entry.countryCode || "");

  const response = await fetch(new URL("/v1/admin/maps/official/import", options.apiBase), {
    method: "POST",
    headers: { Authorization: `Bearer ${options.accessToken}` },
    body,
  });
  if (!response.ok) {
    throw new Error(await response.text() || `HTTP ${response.status}`);
  }
  return response.json();
}

async function mapWithConcurrency(items, concurrency, worker) {
  const out = new Array(items.length);
  let next = 0;
  await Promise.all(Array.from({ length: Math.min(concurrency, items.length) }, async () => {
    for (;;) {
      const index = next;
      next += 1;
      if (index >= items.length) return;
      out[index] = await worker(items[index], index);
    }
  }));
  return out;
}

async function main() {
  const options = parseArgs(process.argv.slice(2));
  if (options.help) {
    process.stdout.write(usage());
    return;
  }

  const manifestPath = resolve(options.manifest);
  const manifestDir = dirname(manifestPath);
  const manifest = normalizeManifest(await readJson(manifestPath));
  const filtered = options.countries
    ? manifest.filter((entry) => options.countries.has(String(entry.countryCode || entry.officialRegionCode || "").toUpperCase()))
    : manifest;
  const catalog = await loadThumbnailCatalog();
  const resolveThumbnail = thumbnailResolver(catalog);

  const validated = await mapWithConcurrency(filtered, options.concurrency, (entry) => validateEntry(entry, manifestDir, resolveThumbnail, options));
  const ready = validated.filter((entry) => entry.ok);
  const failed = validated.filter((entry) => !entry.ok);
  console.log(`Validated ${validated.length} maps: ${ready.length} ready, ${failed.length} blocked.`);
  for (const entry of failed) {
    console.log(`[block] ${entry.countryCode || "??"} ${entry.displayName || ""}: ${entry.problems.join(", ")}`);
  }

  const imports = [];
  if (options.import) {
    if (failed.length > 0) throw new Error("Refusing to import while validation blockers remain");
    await mapWithConcurrency(ready, options.concurrency, async (entry) => {
      console.log(`[import] ${entry.countryCode} ${entry.displayName} (${entry.counts.valid} locations)`);
      const result = await importEntry(entry, options);
      imports.push({ countryCode: entry.countryCode, mapKey: entry.mapKey, result });
    });
  } else {
    console.log("Dry run only. Pass --import to write maps.");
  }

  const report = {
    generatedAt: new Date().toISOString(),
    import: options.import,
    totals: {
      maps: validated.length,
      ready: ready.length,
      blocked: failed.length,
      locations: ready.reduce((sum, entry) => sum + entry.counts.valid, 0),
    },
    maps: validated,
    imports,
  };
  if (options.report) {
    await writeFile(resolve(options.report), `${JSON.stringify(report, null, 2)}\n`, "utf8");
  }
}

main().catch((error) => {
  console.error(error.message || error);
  process.exit(1);
});
