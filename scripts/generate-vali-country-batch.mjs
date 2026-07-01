#!/usr/bin/env node

import { spawn } from "node:child_process";
import {
  appendFile,
  lstat,
  mkdir,
  readdir,
  readFile,
  rename,
  rm,
  symlink,
  writeFile,
} from "node:fs/promises";
import { basename, dirname, join, resolve } from "node:path";
import process from "node:process";

const DEFAULT_LARGE_TARGET_COUNT = 10_000;
const DEFAULT_SMALL_TARGET_COUNT = 2_000;
const DEFAULT_LARGE_COUNTRIES = new Set([
  "AR", "AU", "AT", "BE", "BR", "CA", "CL", "CO", "CZ", "DK", "FI", "FR",
  "DE", "GB", "GR", "HU", "ID", "IE", "IL", "IT", "JP", "KR", "MX", "MY",
  "NL", "NO", "NZ", "PE", "PH", "PL", "PT", "RO", "RS", "RU", "SE", "TH",
  "TR", "UA", "US", "VN", "ZA",
]);

function usage() {
  return `Generate one Vali map per country or territory, sequentially and resumably.

Usage:
  node scripts/generate-vali-country-batch.mjs --template <file> --output-root <dir> [options]

Required:
  --template <file>        Base Vali config JSON, e.g. datasets-config/country.json
  --output-root <dir>      Directory that will receive per-country outputs

Options:
  --source-root <dir>      Directory containing downloaded country folders (default: cwd)
  --vali-bin <path>        Vali executable to run (default: vali from PATH)
  --target-count <n>       Override every country to this location goal
  --large-target-count <n> Target for large-coverage countries (default: ${DEFAULT_LARGE_TARGET_COUNT})
  --small-target-count <n> Target for smaller countries (default: ${DEFAULT_SMALL_TARGET_COUNT})
  --large-countries <list> Comma-separated country codes to target as large coverage
  --target-counts <file>   JSON object of exact per-country goals, e.g. {"US":10000,"MC":500}
  --manifest-output <file> Manifest path for the production importer (default: <output-root>/country-maps.manifest.json)
  --state-file <file>      Progress log path (default: <output-root>/.progress.jsonl)
  --countries <list>       Comma-separated allowlist, e.g. US,CA,PR
  --keep-staging           Keep staging directories after successful runs
  --help                   Show this help

Behavior:
  - Processes one country directory at a time in sorted order.
  - Creates an isolated staging directory for each country.
  - Symlinks the country data into staging so Vali can read it without writing into source data.
  - Writes completion state as JSONL so reruns skip successful countries.
  - If interrupted, rerun the same command to resume remaining countries.
`;
}

function parseArgs(argv) {
  const options = {
    sourceRoot: process.cwd(),
    valiBin: "vali",
    largeTargetCount: DEFAULT_LARGE_TARGET_COUNT,
    smallTargetCount: DEFAULT_SMALL_TARGET_COUNT,
    keepStaging: false,
  };

  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === "--help" || arg === "-h") {
      options.help = true;
      continue;
    }
    if (arg === "--keep-staging") {
      options.keepStaging = true;
      continue;
    }
    if (!arg.startsWith("--")) throw new Error(`Unexpected argument: ${arg}`);
    const key = arg.slice(2).replace(/-([a-z])/g, (_, letter) => letter.toUpperCase());
    const value = argv[i + 1];
    if (!value || value.startsWith("--")) throw new Error(`Missing value for ${arg}`);
    options[key] = value;
    i += 1;
  }

  if (options.help) return options;
  if (!options.template) throw new Error("--template is required");
  if (!options.outputRoot) throw new Error("--output-root is required");

  options.template = resolve(options.template);
  options.sourceRoot = resolve(options.sourceRoot);
  options.outputRoot = resolve(options.outputRoot);
  if (options.targetCount) options.targetCount = parsePositiveInteger(options.targetCount, "--target-count");
  options.largeTargetCount = parsePositiveInteger(options.largeTargetCount, "--large-target-count");
  options.smallTargetCount = parsePositiveInteger(options.smallTargetCount, "--small-target-count");
  options.largeCountries = parseCountries(options.largeCountries) || DEFAULT_LARGE_COUNTRIES;
  options.targetCountsPath = options.targetCounts ? resolve(options.targetCounts) : "";
  options.stateFile = resolve(options.stateFile || join(options.outputRoot, ".progress.jsonl"));
  options.manifestOutput = resolve(options.manifestOutput || join(options.outputRoot, "country-maps.manifest.json"));
  options.countries = parseCountries(options.countries);
  return options;
}

function parsePositiveInteger(value, name) {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 1) throw new Error(`${name} must be a positive integer`);
  return parsed;
}

function parseCountries(value) {
  if (!value) return null;
  const countries = value
    .split(",")
    .map((entry) => entry.trim().toUpperCase())
    .filter(Boolean);
  if (countries.length === 0) throw new Error("--countries must include at least one code");
  return new Set(countries);
}

async function readJson(path) {
  return JSON.parse(await readFile(path, "utf8"));
}

async function readTargetCounts(path) {
  if (!path) return new Map();
  const value = await readJson(path);
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error("--target-counts must be a JSON object keyed by country code");
  }
  const out = new Map();
  for (const [country, raw] of Object.entries(value)) {
    out.set(country.toUpperCase(), parsePositiveInteger(raw, `target count for ${country}`));
  }
  return out;
}

async function readState(path) {
  try {
    const state = new Map();
    const lines = (await readFile(path, "utf8")).split("\n");
    for (const line of lines) {
      if (!line.trim()) continue;
      const entry = JSON.parse(line);
      if (entry && entry.country) state.set(entry.country, entry);
    }
    return state;
  } catch (error) {
    if (error.code === "ENOENT") return new Map();
    throw error;
  }
}

async function appendState(path, entry) {
  await mkdir(dirname(path), { recursive: true });
  await appendFile(path, `${JSON.stringify({ at: new Date().toISOString(), ...entry })}\n`, "utf8");
}

async function listCountryDirectories(sourceRoot, allowlist) {
  const entries = await readdir(sourceRoot, { withFileTypes: true });
  return entries
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name)
    .filter((name) => !allowlist || allowlist.has(name.toUpperCase()))
    .sort((left, right) => left.localeCompare(right));
}

async function directoryHasArtifacts(path) {
  try {
    const entries = await readdir(path, { withFileTypes: true });
    return entries.some((entry) => !entry.name.startsWith("."));
  } catch (error) {
    if (error.code === "ENOENT") return false;
    throw error;
  }
}

function buildCountryConfig(template, country, targetCount) {
  const config = structuredClone(template);
  config.countryCodes = [country];
  config.distributionStrategy = {
    ...config.distributionStrategy,
    locationCountGoal: targetCount,
  };
  return config;
}

function targetCountForCountry(country, options, targetCounts) {
  if (options.targetCount) return options.targetCount;
  const exact = targetCounts.get(country.toUpperCase());
  if (exact) return exact;
  return options.largeCountries.has(country.toUpperCase()) ? options.largeTargetCount : options.smallTargetCount;
}

async function collectArtifacts(stagingDir, excludedNames) {
  const entries = await readdir(stagingDir, { withFileTypes: true });
  return entries
    .filter((entry) => !excludedNames.has(entry.name))
    .map((entry) => entry.name)
    .sort((left, right) => left.localeCompare(right));
}

async function pickMapFile(outputDir, artifacts) {
  const jsonArtifacts = artifacts.filter((artifact) => artifact.toLowerCase().endsWith(".json"));
  if (jsonArtifacts.length === 0) return "";
  const withSizes = await Promise.all(jsonArtifacts.map(async (artifact) => {
    const stat = await lstat(join(outputDir, artifact));
    return { artifact, size: stat.size };
  }));
  withSizes.sort((left, right) => right.size - left.size || left.artifact.localeCompare(right.artifact));
  return join(outputDir, withSizes[0].artifact);
}

function countryName(country) {
  try {
    return new Intl.DisplayNames(["en"], { type: "region" }).of(country.toUpperCase()) || country.toUpperCase();
  } catch {
    return country.toUpperCase();
  }
}

function slugify(value) {
  return value
    .normalize("NFKD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .replace(/&/g, " and ")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

async function writeManifest(path, entries) {
  await mkdir(dirname(path), { recursive: true });
  const manifest = {
    version: 1,
    generatedAt: new Date().toISOString(),
    maps: entries.sort((left, right) => left.countryCode.localeCompare(right.countryCode)),
  };
  await writeFile(path, `${JSON.stringify(manifest, null, 2)}\n`, "utf8");
}

async function buildManifestEntries(outputRoot, state) {
  const entries = [];
  for (const entry of state.values()) {
    if (entry.status !== "success" || !entry.outputDir || !entry.country) continue;
    const artifacts = entry.artifacts || [];
    const mapFile = entry.mapFile || await pickMapFile(entry.outputDir, artifacts);
    if (!mapFile) continue;
    const name = countryName(entry.country);
    const countrySlug = slugify(name);
    entries.push({
      countryCode: entry.country.toUpperCase(),
      displayName: name,
      mapKey: `country/${countrySlug}`,
      description: `Official GeoDuels ${name} country map generated from Vali coverage.`,
      visibility: "public",
      difficulty: "normal",
      thumbnailKey: `countries/${countrySlug}`,
      thumbnailVariant: 1,
      officialRegionType: "country",
      officialRegionCode: entry.country.toUpperCase(),
      targetCount: entry.targetCount || null,
      file: mapFile,
    });
  }
  return entries;
}

function runVali(configPath, cwd, valiBin) {
  const child = spawn(valiBin, ["generate", "--file", basename(configPath)], {
    cwd,
    stdio: "inherit",
  });
  const promise = new Promise((resolveRun, rejectRun) => {
    child.on("error", rejectRun);
    child.on("exit", (code, signal) => resolveRun({ code, signal }));
  });
  return { child, promise };
}

let activeChild = null;
let stopRequested = false;
let sigintCount = 0;

process.on("SIGINT", () => {
  sigintCount += 1;
  stopRequested = true;
  if (activeChild && activeChild.exitCode === null) {
    activeChild.kill("SIGINT");
  }
  if (sigintCount > 1) process.exit(130);
});

async function main() {
  const options = parseArgs(process.argv.slice(2));
  if (options.help) {
    console.log(usage());
    return;
  }

  const template = await readJson(options.template);
  const targetCounts = await readTargetCounts(options.targetCountsPath);
  const countries = await listCountryDirectories(options.sourceRoot, options.countries);
  const state = await readState(options.stateFile);
  const stagingRoot = join(options.outputRoot, ".staging");

  await mkdir(options.outputRoot, { recursive: true });
  await mkdir(stagingRoot, { recursive: true });

  if (countries.length === 0) {
    console.log("No country directories matched the current filters.");
    return;
  }

  console.log(`Found ${countries.length} country directories under ${options.sourceRoot}`);
  console.log(`Writing outputs to ${options.outputRoot}`);
  console.log(`Progress log: ${options.stateFile}`);
  console.log(`Manifest: ${options.manifestOutput}`);
  console.log(`Vali: ${options.valiBin}`);

  for (const country of countries) {
    if (stopRequested) break;

    const finalDir = join(options.outputRoot, country);
    const latestState = state.get(country);
    if (latestState?.status === "success" && (await directoryHasArtifacts(finalDir))) {
      console.log(`[skip] ${country} already completed`);
      continue;
    }

    const sourceDir = join(options.sourceRoot, country);
    const sourceStat = await lstat(sourceDir);
    if (!sourceStat.isDirectory()) {
      console.log(`[skip] ${country} is not a directory`);
      await appendState(options.stateFile, { country, status: "skipped", reason: "not_directory" });
      continue;
    }

    const stagingDir = join(stagingRoot, country);
    const targetCount = targetCountForCountry(country, options, targetCounts);
    const configName = `${country.toLowerCase()}-${targetCount}-country.json`;
    const configPath = join(stagingDir, configName);
    const linkedCountryDir = join(stagingDir, country);

    await rm(stagingDir, { recursive: true, force: true });
    await mkdir(stagingDir, { recursive: true });
    await symlink(sourceDir, linkedCountryDir, "dir");
    await writeFile(
      configPath,
      `${JSON.stringify(buildCountryConfig(template, country, targetCount), null, 2)}\n`,
      "utf8",
    );

    console.log(`[run ] ${country} target=${targetCount}`);
    await appendState(options.stateFile, { country, status: "started", outputDir: finalDir, targetCount });

    const run = runVali(configPath, stagingDir, options.valiBin);
    activeChild = run.child;
    const result = await run.promise;
    activeChild = null;

    if (result.code !== 0) {
      const status = stopRequested || result.signal === "SIGINT" ? "interrupted" : "failed";
      await appendState(options.stateFile, {
        country,
        status,
        exitCode: result.code,
        signal: result.signal || null,
        stagingDir,
      });
      if (status === "interrupted") break;
      continue;
    }

    const artifacts = await collectArtifacts(stagingDir, new Set([country, configName]));
    if (artifacts.length === 0) {
      await appendState(options.stateFile, {
        country,
        status: "failed",
        reason: "no_artifacts_detected",
        stagingDir,
      });
      console.log(`[fail] ${country} produced no output artifacts`);
      continue;
    }

    await rm(finalDir, { recursive: true, force: true });
    await mkdir(finalDir, { recursive: true });
    for (const artifact of artifacts) {
      await rename(join(stagingDir, artifact), join(finalDir, artifact));
    }
    const mapFile = await pickMapFile(finalDir, artifacts);

    const successEntry = {
      country,
      status: "success",
      outputDir: finalDir,
      artifacts,
      mapFile,
      targetCount,
    };
    state.set(country, successEntry);
    await appendState(options.stateFile, successEntry);
    console.log(`[done] ${country} -> ${finalDir}`);
    await writeManifest(options.manifestOutput, await buildManifestEntries(options.outputRoot, state));

    if (!options.keepStaging) await rm(stagingDir, { recursive: true, force: true });
  }

  if (stopRequested) {
    console.log("Stopped. Re-run the same command to resume remaining countries.");
  } else {
    await writeManifest(options.manifestOutput, await buildManifestEntries(options.outputRoot, state));
    console.log("Batch generation complete.");
  }
}

main().catch((error) => {
  console.error(error.message || error);
  process.exit(1);
});
