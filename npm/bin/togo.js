#!/usr/bin/env node
// Thin shim: exec the platform `togo` binary (placed in this dir by install.js).
const path = require("path");
const fs = require("fs");
const { spawnSync } = require("child_process");

const bin = path.join(__dirname, process.platform === "win32" ? "togo.exe" : "togo");
if (!fs.existsSync(bin)) {
  console.error("[togo] binary not found. Reinstall: npm install -g @togo-framework/cli");
  process.exit(1);
}
const r = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
process.exit(r.status == null ? 1 : r.status);
