#!/usr/bin/env node
import fs from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import sharp from "sharp";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const genericSourceDir = path.join(root, "assets/source-map-thumbnails/generic");
const continentSourceDir = path.join(root, "assets/source-map-thumbnails/continents");
const countrySourceDir = path.join(root, "assets/source-map-thumbnails/countries");
const publicRoot = path.join(root, "public/map-thumbnails");
const genericOutDir = path.join(publicRoot, "generic");
const continentsOutDir = path.join(publicRoot, "continents");
const countriesOutDir = path.join(publicRoot, "countries");
const catalogPath = path.join(root, "shared/mapthumbnails/catalog.generated.json");
const sourceExtensions = new Set([".jpg", ".jpeg", ".png", ".webp", ".avif", ".tif", ".tiff"]);
const genericCount = 5;
const continents = [
  ["africa", "Africa"],
  ["antarctica", "Antarctica"],
  ["asia", "Asia"],
  ["europe", "Europe"],
  ["north-america", "North America"],
  ["oceania", "Oceania"],
  ["south-america", "South America"],
];
const countryAliases = {
  GB: ["UK", "Britain", "England", "Scotland", "Wales"],
  US: ["USA", "America", "United States of America"],
  KR: ["South Korea", "Korea"],
  ZA: ["South Africa"],
};

const args = new Set(process.argv.slice(2));
const checkOnly = args.has("--check");

function slugify(value) {
  return value
    .normalize("NFKD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .replace(/&/g, " and ")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function countryName(code) {
  const display = new Intl.DisplayNames(["en"], { type: "region" });
  return display.of(code) || code;
}

function titleFromSlug(slug) {
  return slug
    .split("-")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

async function exists(file) {
  try {
    await fs.access(file);
    return true;
  } catch {
    return false;
  }
}

async function readExistingCatalog() {
  try {
    return JSON.parse(await fs.readFile(catalogPath, "utf8"));
  } catch (error) {
    if (error?.code === "ENOENT") return { thumbnails: [] };
    throw error;
  }
}

async function sourceFiles(dir) {
  const entries = await fs.readdir(dir, { withFileTypes: true });
  return entries
    .filter((entry) => entry.isFile() && sourceExtensions.has(path.extname(entry.name).toLowerCase()))
    .map((entry) => path.join(dir, entry.name))
    .sort();
}

async function outputSlugs(dir) {
  const entries = await fs.readdir(dir, { withFileTypes: true });
  return entries
    .filter((entry) => entry.isFile() && path.extname(entry.name).toLowerCase() === ".webp")
    .map((entry) => path.basename(entry.name, path.extname(entry.name)))
    .sort();
}

function countryCode(file) {
  const stem = path.basename(file, path.extname(file));
  const match = /^([a-z]{2})(?:[_-].*)?$/i.exec(stem);
  if (!match) {
    throw new Error(
      `Cannot determine country from ${path.basename(file)}. Start the filename with its two-letter country code, for example US.jpg or US_001.jpg.`,
    );
  }
  return match[1].toUpperCase();
}

function continentForSource(file) {
  const stem = slugify(path.basename(file, path.extname(file)).replace(/[_-]\d+$/, ""));
  const continent = continents.find(([slug]) => slug === stem);
  if (!continent) {
    throw new Error(
      `Cannot determine continent from ${path.basename(file)}. Use one of: ${continents.map(([slug]) => `${slug}.jpg`).join(", ")}.`,
    );
  }
  return continent;
}

async function processImage(source, output) {
  await sharp(source)
    .resize(1280, 720, { fit: "cover", position: "center" })
    .webp({ quality: 82 })
    .toFile(output);
}

function countryMetadataBySlug() {
  const countries = new Map();
  for (let first = 65; first <= 90; first += 1) {
    for (let second = 65; second <= 90; second += 1) {
      const code = String.fromCharCode(first, second);
      const label = countryName(code);
      if (label === code || label === "Unknown Region") continue;
      const slug = slugify(label);
      if (!countries.has(slug)) {
        countries.set(slug, { code, label });
      }
    }
  }
  return countries;
}

async function main() {
  await fs.mkdir(genericOutDir, { recursive: true });
  await fs.mkdir(continentSourceDir, { recursive: true });
  await fs.mkdir(continentsOutDir, { recursive: true });
  await fs.mkdir(countriesOutDir, { recursive: true });
  const existingCatalog = await readExistingCatalog();
  const existingOptions = new Map(
    (existingCatalog.thumbnails || []).map((option) => [option.key, option]),
  );

  const genericSources = await sourceFiles(genericSourceDir);
  const generic = [];
  const missingGeneric = [];

  for (let index = 0; index < genericCount; index += 1) {
    const variant = index + 1;
    const source = genericSources.find(
      (file) => path.basename(file, path.extname(file)).toLowerCase() === `variant-${variant}`,
    );
    const output = path.join(genericOutDir, `variant-${variant}.webp`);

    if (source && !checkOnly) {
      await processImage(source, output);
    }

    if (!source || (checkOnly && !(await exists(output)))) {
      missingGeneric.push(`variant-${variant}`);
    }

    generic.push({
      key: `generic/variant-${variant}`,
      label: `Generic ${variant}`,
      category: "generic",
      search: `generic default stock variant ${variant}`,
    });
  }

  const continentSources = new Map();
  for (const source of await sourceFiles(continentSourceDir)) {
    const [slug] = continentForSource(source);
    if (continentSources.has(slug)) {
      throw new Error(
        `Multiple source images found for ${slug}: ${path.basename(continentSources.get(slug))} and ${path.basename(source)}.`,
      );
    }
    continentSources.set(slug, source);
  }

  const countriesByCode = new Map();
  for (const source of await sourceFiles(countrySourceDir)) {
    const code = countryCode(source);
    if (countriesByCode.has(code)) {
      throw new Error(
        `Multiple source images found for ${code}: ${path.basename(countriesByCode.get(code))} and ${path.basename(source)}.`,
      );
    }
    countriesByCode.set(code, source);
  }
  const sourceMetadataBySlug = new Map(
    [...countriesByCode].map(([code]) => {
      const label = countryName(code);
      return [slugify(label), { code, label }];
    }),
  );

  const missingContinents = [];
  const missingCountries = [];
  for (const [slug, source] of continentSources) {
    const output = path.join(continentsOutDir, `${slug}.webp`);
    if (!checkOnly) {
      await processImage(source, output);
    } else if (!(await exists(output))) {
      missingContinents.push(slug);
    }
  }

  for (const [code, source] of countriesByCode) {
    const label = countryName(code);
    const slug = slugify(label);
    const output = path.join(countriesOutDir, `${slug}.webp`);

    if (!checkOnly) {
      await processImage(source, output);
    } else if (!(await exists(output))) {
      missingCountries.push(`${code} ${label}`);
    }
  }

  const continentLabels = new Map(continents);
  const continentOptions = (await outputSlugs(continentsOutDir)).map((slug) => ({
    key: `continents/${slug}`,
    label: continentLabels.get(slug) || titleFromSlug(slug),
    category: "continents",
    search: continentLabels.get(slug) || titleFromSlug(slug),
  }));

  const metadataBySlug = countryMetadataBySlug();
  const countryOptions = (await outputSlugs(countriesOutDir)).map((slug) => {
    const existing = existingOptions.get(`countries/${slug}`);
    const metadata = sourceMetadataBySlug.get(slug)
      || (existing && { code: existing.code, label: existing.label })
      || metadataBySlug.get(slug);
    const code = metadata?.code;
    const label = metadata?.label || titleFromSlug(slug);
    return {
      key: `countries/${slug}`,
      label,
      category: "countries",
      code,
      search: [label, code, ...(countryAliases[code] || [])].filter(Boolean).join(" "),
    };
  });
  countryOptions.sort((a, b) => a.label.localeCompare(b.label));

  const content = `${JSON.stringify({
    version: 1,
    thumbnails: [...generic, ...continentOptions, ...countryOptions],
  }, null, 2)}\n`;

  if (missingGeneric.length > 0) {
    console.error(`Generic sources or outputs missing (${missingGeneric.length}):`);
    for (const item of missingGeneric) console.error(`- ${item}`);
  }
  if (missingContinents.length > 0) {
    console.error(`Continent outputs missing (${missingContinents.length}):`);
    for (const item of missingContinents) console.error(`- ${item}`);
  }
  if (missingCountries.length > 0) {
    console.error(`Country outputs missing (${missingCountries.length}):`);
    for (const item of missingCountries) console.error(`- ${item}`);
  }
  if (missingGeneric.length > 0 || missingContinents.length > 0 || missingCountries.length > 0) {
    process.exitCode = 1;
    return;
  }

  if (checkOnly) {
    const existingCatalog = await fs.readFile(catalogPath, "utf8");
    if (existingCatalog !== content) {
      console.error("Thumbnail picker catalog is out of date. Run npm run maps:thumbnails.");
      process.exitCode = 1;
      return;
    }
  } else {
    await fs.mkdir(path.dirname(catalogPath), { recursive: true });
    await fs.writeFile(catalogPath, content);
  }

  console.log(`${checkOnly ? "Checked" : "Generated"} ${continentOptions.length} continent thumbnails, ${countryOptions.length} country thumbnails, and ${generic.length + continentOptions.length + countryOptions.length} catalog options.`);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
