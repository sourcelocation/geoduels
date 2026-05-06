#!/usr/bin/env node
/**
 * Backfill panoId values for a dataset JSON using Google Street View metadata.
 *
 * Input format: JSON array of objects containing at least { lat, lng }.
 * Output format: same rows, with panoId filled when metadata status=OK.
 *
 * Usage:
 *   node scripts/enrich-panoids.mjs \
 *     --in datasets/a-source-world.json \
 *     --out datasets/a-source-world.enriched.json \
 *     --key "$GOOGLE_MAPS_API_KEY" \
 *     --drop-missing true \
 *     --qps 8
 */

import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { spawn } from "node:child_process";

async function loadDotEnv(dotEnvPath) {
  try {
    const raw = await fs.readFile(dotEnvPath, "utf8");
    for (const line of raw.split(/\r?\n/)) {
      const trimmed = line.trim();
      if (!trimmed || trimmed.startsWith("#")) continue;
      const idx = trimmed.indexOf("=");
      if (idx <= 0) continue;
      const key = trimmed.slice(0, idx).trim();
      let value = trimmed.slice(idx + 1).trim();
      if (
        (value.startsWith('"') && value.endsWith('"')) ||
        (value.startsWith("'") && value.endsWith("'"))
      ) {
        value = value.slice(1, -1);
      }
      if (!(key in process.env)) {
        process.env[key] = value;
      }
    }
  } catch {
    // .env is optional
  }
}

//Either feed from datasets.json or from DB
function parseArgs(argv) {
  const out = {
    in: "datasets/a-source-world.json",
    out: "datasets/a-source-world.enriched.json",
    key: process.env.GOOGLE_MAPS_API_KEY || "",
    dropMissing: true,
    qps: 8,
    source: "outdoor",
    language: "en",
    dbUpdate: false,
    dbMapKey: process.env.LOCATION_MAP_KEY || "",
    dbService: "postgres",
    dbUser: process.env.POSTGRES_USER || "geoduels",
    dbName: process.env.POSTGRES_DB || "geoduels",
    dbTol: 1e-6,
  };
  for (let i = 2; i < argv.length; i++) {
    const cur = argv[i];
    const next = argv[i + 1];
    if (cur === "--in" && next) ((out.in = next), i++);
    else if (cur === "--out" && next) ((out.out = next), i++);
    else if (cur === "--key" && next) ((out.key = next), i++);
    else if (cur === "--drop-missing" && next)
      ((out.dropMissing = next.toLowerCase() !== "false"), i++);
    else if (cur === "--qps" && next)
      ((out.qps = Math.max(1, Number(next) || 8)), i++);
    else if (cur === "--source" && next) ((out.source = next), i++);
    else if (cur === "--language" && next) ((out.language = next), i++);
    else if (cur === "--db-update" && next)
      ((out.dbUpdate = next.toLowerCase() === "true"), i++);
    else if (cur === "--db-map-key" && next) ((out.dbMapKey = next), i++);
    else if (cur === "--db-service" && next) ((out.dbService = next), i++);
    else if (cur === "--db-user" && next) ((out.dbUser = next), i++);
    else if (cur === "--db-name" && next) ((out.dbName = next), i++);
    else if (cur === "--db-tol" && next)
      ((out.dbTol = Math.max(0, Number(next) || 1e-6)), i++);
  }
  if (!out.key)
    throw new Error("missing API key: set GOOGLE_MAPS_API_KEY or pass --key");
  if (out.dbUpdate && !out.dbMapKey)
    throw new Error(
      "missing db map key: set LOCATION_MAP_KEY or pass --db-map-key",
    );
  return out;
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function validCoord(lat, lng) {
  return (
    Number.isFinite(lat) &&
    Number.isFinite(lng) &&
    lat >= -90 &&
    lat <= 90 &&
    lng >= -180 &&
    lng <= 180
  );
}

async function metadataLookup({ lat, lng, key, source, language }) {
  const u = new URL("https://maps.googleapis.com/maps/api/streetview/metadata");
  u.searchParams.set("location", `${lat},${lng}`);
  u.searchParams.set("source", source);
  u.searchParams.set("language", language);
  u.searchParams.set("key", key);
  const resp = await fetch(u, { method: "GET" });
  if (!resp.ok) {
    return { ok: false, status: `HTTP_${resp.status}` };
  }
  const body = await resp.json();
  if (
    body?.status === "OK" &&
    typeof body?.pano_id === "string" &&
    body.pano_id
  ) {
    return {
      ok: true,
      panoId: body.pano_id,
      lat: Number(body?.location?.lat),
      lng: Number(body?.location?.lng),
    };
  }
  return { ok: false, status: String(body?.status || "UNKNOWN") };
}

function execCommand(command, args, opts = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      stdio: ["pipe", "pipe", "pipe"],
      shell: false,
      ...opts,
    });
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (d) => (stdout += String(d)));
    child.stderr.on("data", (d) => (stderr += String(d)));
    child.on("error", reject);
    child.on("close", (code) => {
      if (code === 0) {
        resolve({ stdout, stderr });
      } else {
        reject(new Error(`${command} exited ${code}: ${stderr || stdout}`));
      }
    });
    if (opts.stdin) {
      child.stdin.write(opts.stdin);
    }
    child.stdin.end();
  });
}

async function applyDBUpdates(args, dbUpdates) {
  if (!dbUpdates.length) return { updated: 0 };
  const tol = args.dbTol;
  const esc = (v) => String(v).replace(/'/g, "''");
  const valuesSQL = dbUpdates
    .map(
      (r) =>
        `(${Number(r.srcLat)}, ${Number(r.srcLng)}, '${esc(r.panoId)}', ${Number.isFinite(r.resolvedLat) ? Number(r.resolvedLat) : "NULL"}, ${Number.isFinite(r.resolvedLng) ? Number(r.resolvedLng) : "NULL"})`,
    )
    .join(",\n      ");
  const sql = `
with incoming(src_lat, src_lng, pano_id, resolved_lat, resolved_lng) as (
  values
      ${valuesSQL}
),
target_revision as (
  select ma.active_revision_id as revision_id
  from map_aliases ma
  where ma.map_key = '${esc(args.dbMapKey)}'
),
updated as (
  update locations l
  set pano_id = i.pano_id
  from incoming i, target_revision tr
  where l.map_revision_id = tr.revision_id
    and abs(l.lat - i.src_lat) <= ${tol}
    and abs(l.lng - i.src_lng) <= ${tol}
  returning l.id
)
select count(*)::int as updated_count from updated;
`;

  const cmdArgs = [
    "compose",
    "exec",
    "-T",
    args.dbService,
    "psql",
    "-U",
    args.dbUser,
    "-d",
    args.dbName,
    "-t",
    "-A",
    "-v",
    "ON_ERROR_STOP=1",
    "-c",
    sql,
  ];
  const { stdout } = await execCommand("docker", cmdArgs);
  const updated = Number(String(stdout).trim().split(/\s+/).pop()) || 0;
  return { updated };
}

async function run() {
  const repoRoot = path.resolve(
    path.dirname(fileURLToPath(import.meta.url)),
    "..",
  );
  const dotEnvPath = path.join(repoRoot, ".env");
  await loadDotEnv(dotEnvPath);

  const args = parseArgs(process.argv);
  const raw = await fs.readFile(args.in, "utf8");
  const input = JSON.parse(raw);
  if (!Array.isArray(input)) {
    throw new Error("input must be a JSON array");
  }

  const minDelayMs = Math.ceil(1000 / args.qps);
  const output = [];
  const dbUpdates = [];
  const stats = {
    total: input.length,
    kept: 0,
    enriched: 0,
    alreadyHadPano: 0,
    droppedInvalidCoord: 0,
    droppedMissingPano: 0,
    lookupFailures: 0,
  };

  for (let i = 0; i < input.length; i++) {
    const row = input[i];
    const lat = Number(row?.lat);
    const lng = Number(row?.lng);
    if (!validCoord(lat, lng)) {
      stats.droppedInvalidCoord++;
      continue;
    }

    const next = { ...row };
    if (typeof next.panoId === "string" && next.panoId.trim() !== "") {
      stats.alreadyHadPano++;
      output.push(next);
      stats.kept++;
      continue;
    }

    const t0 = Date.now();
    const res = await metadataLookup({
      lat,
      lng,
      key: args.key,
      source: args.source,
      language: args.language,
    });
    const elapsed = Date.now() - t0;
    if (elapsed < minDelayMs) {
      await sleep(minDelayMs - elapsed);
    }

    if (res.ok) {
      next.panoId = res.panoId;
      // Optional normalization to Google's resolved pano coordinate.
      if (validCoord(res.lat, res.lng)) {
        next.lat = res.lat;
        next.lng = res.lng;
      }
      output.push(next);
      stats.enriched++;
      stats.kept++;
      dbUpdates.push({
        srcLat: lat,
        srcLng: lng,
        panoId: res.panoId,
        resolvedLat: res.lat,
        resolvedLng: res.lng,
      });
    } else if (!args.dropMissing) {
      output.push(next);
      stats.kept++;
      stats.lookupFailures++;
    } else {
      stats.droppedMissingPano++;
      stats.lookupFailures++;
    }
  }

  await fs.mkdir(path.dirname(args.out), { recursive: true });
  await fs.writeFile(args.out, JSON.stringify(output, null, 2), "utf8");
  let dbResult = { updated: 0 };
  if (args.dbUpdate) {
    dbResult = await applyDBUpdates(args, dbUpdates);
  }

  process.stdout.write(
    JSON.stringify(
      {
        input: args.in,
        output: args.out,
        dbUpdate: args.dbUpdate,
        dbMapKey: args.dbMapKey || null,
        dbRowsUpdated: dbResult.updated,
        ...stats,
      },
      null,
      2,
    ) + "\n",
  );
}

run().catch((err) => {
  process.stderr.write(
    `enrich-panoids failed: ${err?.message || String(err)}\n`,
  );
  process.exit(1);
});
