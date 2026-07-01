import { spawnSync } from "node:child_process";
import { cpSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const extensionRoot = join(repoRoot, "extensions", "geoduels-enhancer");
const outputRoot = join(repoRoot, "dist", "extensions", "geoduels-enhancer");
const productionMatchesByTarget = {
  chrome: [
    "https://geoduels.io/*",
    "https://*.geoduels.io/*",
    "https://www.google.com/maps/embed/*",
  ],
  firefox: [
    "https://geoduels.io/*",
    "https://*.geoduels.io/*",
    "https://www.google.com/maps/embed/*",
    "https://*.google.com/maps/embed/*",
  ],
};

function readManifest() {
  return JSON.parse(readFileSync(join(extensionRoot, "manifest.json"), "utf8"));
}

function writeJson(filePath, value) {
  writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
}

function prepareTarget(target) {
  const targetDir = join(outputRoot, target);
  rmSync(targetDir, { recursive: true, force: true });
  mkdirSync(targetDir, { recursive: true });
  cpSync(join(extensionRoot, "src"), join(targetDir, "src"), { recursive: true });
  cpSync(join(extensionRoot, "README.md"), join(targetDir, "README.md"));
  return targetDir;
}

function productionManifest(target) {
  const manifest = readManifest();
  const productionMatches = productionMatchesByTarget[target];
  manifest.content_scripts = manifest.content_scripts.map((script) => ({
    ...script,
    matches: script.matches.filter((match) => productionMatches.includes(match)),
  }));

  if (target === "chrome") {
    delete manifest.browser_specific_settings;
  }

  return manifest;
}

function zipDirectory(sourceDir, zipPath) {
  rmSync(zipPath, { force: true });
  const result = spawnSync("zip", ["-r", zipPath, "."], {
    cwd: sourceDir,
    encoding: "utf8",
    stdio: "pipe",
  });

  if (result.status !== 0) {
    throw new Error(result.stderr || result.stdout || "zip command failed");
  }
}

function packageTarget(target) {
  const targetDir = prepareTarget(target);
  writeJson(join(targetDir, "manifest.json"), productionManifest(target));

  const manifest = readManifest();
  const zipPath = join(outputRoot, `geoduels-enhancer-${target}-v${manifest.version}.zip`);
  zipDirectory(targetDir, zipPath);
  return zipPath;
}

if (!existsSync(extensionRoot)) {
  throw new Error(`Extension directory not found: ${extensionRoot}`);
}

mkdirSync(outputRoot, { recursive: true });

const archives = ["chrome", "firefox"].map(packageTarget);

for (const archive of archives) {
  console.log(`Created ${relative(repoRoot, archive)}`);
}
