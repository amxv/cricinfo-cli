#!/usr/bin/env node

const fs = require("node:fs");
const path = require("node:path");
const { spawnSync } = require("node:child_process");

const pkg = require("../package.json");
const cliName = resolveCLIName(pkg);
const executableName = process.platform === "win32" ? `${cliName}.exe` : `${cliName}-bin`;
const executablePath = path.join(__dirname, executableName);

if (!fs.existsSync(executablePath)) {
  console.error(`${cliName} binary is not installed. Re-run: npm rebuild -g ${pkg.name}`);
  process.exit(1);
}

const child = spawnSync(executablePath, process.argv.slice(2), { stdio: "inherit" });

if (child.error) {
  console.error(child.error.message);
  process.exit(1);
}

if (child.signal) {
  process.kill(process.pid, child.signal);
}

process.exit(child.status ?? 1);

function resolveCLIName(packageJSON) {
  const configured = `${packageJSON?.config?.cliBinaryName || ""}`.trim();
  if (configured) {
    return configured;
  }

  if (packageJSON?.bin && typeof packageJSON.bin === "object") {
    const names = Object.keys(packageJSON.bin);
    if (names.length > 0 && `${names[0]}`.trim()) {
      return `${names[0]}`.trim();
    }
  }

  return "cricinfo";
}
