#!/usr/bin/env node

import { appendFile, mkdir, readFile, rename, rm, writeFile } from "node:fs/promises";
import { dirname, isAbsolute, resolve } from "node:path";
import process from "node:process";
import { pathToFileURL } from "node:url";

const METADATA_URL = "https://maps.googleapis.com/maps/api/streetview/metadata";
const DEFAULT_CONCURRENCY = 32;
const DEFAULT_REPAIR_RADIUS = 50;
const DEFAULT_RETRIES = 5;
const DEFAULT_TIMEOUT_MS = 15_000;

function usage() {
  return `Validate a Vali export with Google's no-charge Street View metadata endpoint.

Usage:
  npm run validate:streetview -- --input <file> --output <file> [options]

Required:
  --input <file>          Vali JSON array to validate
  --output <file>         Clean JSON array containing working locations
  --api-key <key>         Street View Static API key (or GOOGLE_MAPS_API_KEY)

Options:
  --report <file>         Report path (default: <output>.report.json)
  --checkpoint <file>     Resume log (default: <output>.checkpoint.jsonl)
  --concurrency <n>       Simultaneous requests (default: ${DEFAULT_CONCURRENCY})
  --repair-radius <n>     Refresh stale IDs within this many meters (default: ${DEFAULT_REPAIR_RADIUS}; 0 disables)
  --retries <n>           Retries for throttling/server failures (default: ${DEFAULT_RETRIES})
  --timeout-ms <n>        Timeout for each request (default: ${DEFAULT_TIMEOUT_MS})
  --keep-checkpoint       Retain the checkpoint after success
  --help                  Show this help

Metadata requests do not load Street View imagery. Google documents the
Street View Metadata SKU as no-charge with an unlimited free usage cap.
`;
}

export function parseArgs(argv) {
  const options = {
    concurrency: DEFAULT_CONCURRENCY,
    repairRadius: DEFAULT_REPAIR_RADIUS,
    retries: DEFAULT_RETRIES,
    timeoutMs: DEFAULT_TIMEOUT_MS,
    keepCheckpoint: false,
  };

  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === "--help" || arg === "-h") options.help = true;
    else if (arg === "--keep-checkpoint") options.keepCheckpoint = true;
    else if (arg.startsWith("--")) {
      const key = arg.slice(2).replace(/-([a-z])/g, (_, letter) => letter.toUpperCase());
      const value = argv[++i];
      if (!value || value.startsWith("--")) throw new Error(`Missing value for ${arg}`);
      options[key] = value;
    } else {
      throw new Error(`Unexpected argument: ${arg}`);
    }
  }

  options.apiKey ||= process.env.GOOGLE_MAPS_API_KEY;
  options.concurrency = positiveInteger(options.concurrency, "--concurrency");
  options.repairRadius = nonNegativeInteger(options.repairRadius, "--repair-radius");
  options.retries = nonNegativeInteger(options.retries, "--retries");
  options.timeoutMs = positiveInteger(options.timeoutMs, "--timeout-ms");
  return options;
}

function positiveInteger(value, name) {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 1) throw new Error(`${name} must be a positive integer`);
  return parsed;
}

function nonNegativeInteger(value, name) {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 0) throw new Error(`${name} must be a non-negative integer`);
  return parsed;
}

function absolutePath(path) {
  return isAbsolute(path) ? path : resolve(process.cwd(), path);
}

export function inspectLocations(value) {
  if (!Array.isArray(value)) throw new Error("Vali output must be a top-level JSON array");

  const candidates = [];
  const rejected = [];
  const seenPanos = new Set();

  value.forEach((location, index) => {
    let reason = "";
    if (!location || typeof location !== "object" || Array.isArray(location)) reason = "not_an_object";
    else if (!Number.isFinite(location.lat) || location.lat < -90 || location.lat > 90) reason = "invalid_lat";
    else if (!Number.isFinite(location.lng) || location.lng < -180 || location.lng > 180) reason = "invalid_lng";
    else if (typeof location.panoId !== "string" || !location.panoId.trim()) reason = "missing_pano_id";
    else if (seenPanos.has(location.panoId)) reason = "duplicate_pano_id";

    if (reason) rejected.push({ index, reason, location });
    else {
      seenPanos.add(location.panoId);
      candidates.push({ index, location });
    }
  });

  return { candidates, rejected };
}

export function buildMetadataUrl(apiKey, parameters) {
  const url = new URL(METADATA_URL);
  url.searchParams.set("key", apiKey);
  for (const [key, value] of Object.entries(parameters)) {
    if (value !== undefined && value !== null && value !== "") url.searchParams.set(key, String(value));
  }
  return url;
}

function sleep(ms) {
  return new Promise((resolveSleep) => setTimeout(resolveSleep, ms));
}

function retryDelay(attempt) {
  return Math.min(10_000, 250 * (2 ** attempt)) + Math.floor(Math.random() * 250);
}

function isRetryableStatus(status) {
  return status === "OVER_QUERY_LIMIT" || status === "UNKNOWN_ERROR";
}

function nonRetryableError(message) {
  const error = new Error(message);
  error.nonRetryable = true;
  return error;
}

async function requestMetadata(parameters, options, fetchImpl = fetch) {
  for (let attempt = 0; ; attempt += 1) {
    try {
      const response = await fetchImpl(buildMetadataUrl(options.apiKey, parameters), {
        signal: AbortSignal.timeout(options.timeoutMs),
      });
      if (response.status === 429 || response.status >= 500) {
        if (attempt < options.retries) {
          await sleep(retryDelay(attempt));
          continue;
        }
        throw new Error(`Street View metadata HTTP ${response.status} after ${attempt + 1} attempts`);
      }
      if (!response.ok) throw nonRetryableError(`Street View metadata HTTP ${response.status}`);

      const metadata = await response.json();
      if (isRetryableStatus(metadata.status) && attempt < options.retries) {
        await sleep(retryDelay(attempt));
        continue;
      }
      if (metadata.status === "REQUEST_DENIED" || metadata.status === "INVALID_REQUEST") {
        throw nonRetryableError(`Street View metadata ${metadata.status}: ${metadata.error_message || "check the API key and request configuration"}`);
      }
      return metadata;
    } catch (error) {
      if (error.nonRetryable || attempt >= options.retries) throw error;
      await sleep(retryDelay(attempt));
    }
  }
}

export async function validateLocation(item, options, fetchImpl = fetch) {
  const { index, location } = item;
  const exact = await requestMetadata({ pano: location.panoId }, options, fetchImpl);
  if (exact.status === "OK" && exact.pano_id) {
    return {
      index,
      panoId: location.panoId,
      repairRadius: options.repairRadius,
      ok: true,
      status: exact.pano_id === location.panoId ? "OK" : "PANO_ID_REFRESHED",
      returnedPanoId: exact.pano_id,
      returnedLat: exact.location?.lat,
      returnedLng: exact.location?.lng,
    };
  }

  if (exact.status !== "ZERO_RESULTS" || options.repairRadius === 0) {
    return { index, panoId: location.panoId, repairRadius: options.repairRadius, ok: false, status: exact.status || "INVALID_RESPONSE" };
  }

  const refreshed = await requestMetadata({
    location: `${location.lat},${location.lng}`,
    radius: options.repairRadius,
    source: "outdoor",
  }, options, fetchImpl);
  return {
    index,
    panoId: location.panoId,
    repairRadius: options.repairRadius,
    ok: refreshed.status === "OK" && Boolean(refreshed.pano_id),
    status: refreshed.status === "OK" && refreshed.pano_id ? "PANO_ID_REFRESHED" : refreshed.status || "INVALID_RESPONSE",
    returnedPanoId: refreshed.pano_id || "",
    returnedLat: refreshed.location?.lat,
    returnedLng: refreshed.location?.lng,
  };
}

async function readCheckpoint(path) {
  try {
    const lines = (await readFile(path, "utf8")).split("\n");
    const results = {};
    for (const line of lines) {
      if (!line.trim()) continue;
      const result = JSON.parse(line);
      results[result.index] = result;
    }
    return results;
  } catch (error) {
    if (error.code === "ENOENT") return {};
    throw new Error(`Cannot read checkpoint ${path}: ${error.message}`);
  }
}

async function appendCheckpoint(path, results) {
  await mkdir(dirname(path), { recursive: true });
  await appendFile(path, `${results.map((result) => JSON.stringify(result)).join("\n")}\n`);
}

async function atomicJson(path, value, compact = false) {
  await mkdir(dirname(path), { recursive: true });
  const tempPath = `${path}.tmp-${process.pid}`;
  await writeFile(tempPath, `${JSON.stringify(value, null, compact ? 0 : 2)}\n`);
  await rename(tempPath, path);
}

async function main() {
  const options = parseArgs(process.argv.slice(2));
  if (options.help) {
    process.stdout.write(usage());
    return;
  }
  if (!options.input || !options.output || !options.apiKey) {
    throw new Error("--input, --output, and --api-key (or GOOGLE_MAPS_API_KEY) are required\n\n" + usage());
  }

  const inputPath = absolutePath(options.input);
  const outputPath = absolutePath(options.output);
  const reportPath = absolutePath(options.report || `${options.output}.report.json`);
  const checkpointPath = absolutePath(options.checkpoint || `${options.output}.checkpoint.jsonl`);
  const input = JSON.parse(await readFile(inputPath, "utf8"));
  const { candidates, rejected } = inspectLocations(input);
  const checkpoint = await readCheckpoint(checkpointPath);
  const pending = candidates.filter(({ index, location }) => (
    checkpoint[index]?.panoId !== location.panoId ||
    checkpoint[index]?.repairRadius !== options.repairRadius
  ));

  process.stdout.write(`Loaded ${input.length} locations: ${candidates.length} structurally valid, ${rejected.length} rejected, ${pending.length} metadata checks pending.\n`);
  for (let offset = 0; offset < pending.length; offset += options.concurrency) {
    const batch = pending.slice(offset, offset + options.concurrency);
    const results = await Promise.all(batch.map((item) => validateLocation(item, options)));
    for (const result of results) checkpoint[result.index] = result;
    await appendCheckpoint(checkpointPath, results);
    process.stdout.write(`\rChecked ${Math.min(offset + batch.length, pending.length)}/${pending.length}`);
  }
  if (pending.length) process.stdout.write("\n");

  const clean = [];
  const repairs = [];
  for (const { index, location } of candidates) {
    const result = checkpoint[index];
    if (result?.ok) {
      const repaired = result.returnedPanoId && result.returnedPanoId !== location.panoId;
      clean.push(repaired ? {
        ...location,
        lat: Number.isFinite(result.returnedLat) ? result.returnedLat : location.lat,
        lng: Number.isFinite(result.returnedLng) ? result.returnedLng : location.lng,
        panoId: result.returnedPanoId,
      } : location);
      if (repaired) repairs.push({ index, oldPanoId: location.panoId, newPanoId: result.returnedPanoId });
    } else {
      rejected.push({ index, reason: "street_view_unavailable", status: result?.status || "NOT_CHECKED", location });
    }
  }
  rejected.sort((a, b) => a.index - b.index);

  await atomicJson(outputPath, clean, true);
  await atomicJson(reportPath, {
    input: inputPath,
    checkedAt: new Date().toISOString(),
    total: input.length,
    accepted: clean.length,
    repaired: repairs.length,
    rejected: rejected.length,
    repairs,
    locations: rejected,
  });
  if (!options.keepCheckpoint) await rm(checkpointPath, { force: true });
  process.stdout.write(`Wrote ${clean.length} working locations (${repairs.length} refreshed IDs) to ${outputPath}; ${rejected.length} rejections to ${reportPath}.\n`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main().catch((error) => {
    process.stderr.write(`Error: ${error.message}\n`);
    process.exitCode = 1;
  });
}
