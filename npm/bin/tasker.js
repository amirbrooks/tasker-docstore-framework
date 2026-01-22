#!/usr/bin/env node
const fs = require("node:fs");
const path = require("node:path");
const { spawnSync } = require("node:child_process");

const binDir = path.join(__dirname, "..", "bin", `${process.platform}-${process.arch}`);
const exeName = process.platform === "win32" ? "tasker.exe" : "tasker";
const binPath = path.join(binDir, exeName);

function ensureInstalled() {
  if (fs.existsSync(binPath)) return;
  const installer = path.join(__dirname, "..", "scripts", "install.js");
  const res = spawnSync(process.execPath, [installer], { stdio: "inherit" });
  if (res.status !== 0) {
    process.exit(res.status || 1);
  }
}

ensureInstalled();

const result = spawnSync(binPath, process.argv.slice(2), { stdio: "inherit" });
process.exit(result.status ?? 0);
