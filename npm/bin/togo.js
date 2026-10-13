#!/usr/bin/env node
// Thin shim: exec the platform `togo` binary (placed in this dir by install.js).
// npm 11 skips dependency install scripts unless allowed (and --ignore-scripts
// always does), so when the postinstall never ran, download the binary on first use.
const path = require("path");
const fs = require("fs");
const { spawnSync } = require("child_process");

const bin = path.join(__dirname, process.platform === "win32" ? "togo.exe" : "togo");

function run() {
  const r = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
  process.exit(r.status == null ? 1 : r.status);
}

if (fs.existsSync(bin)) {
  run();
} else {
  console.error("[togo] binary not installed yet (npm skipped the postinstall script); fetching it now…");
  require("../install.js").install().then((ok) => {
    if (ok && fs.existsSync(bin)) return run();
    console.error("[togo] binary not found. Install it with: go install github.com/togo-framework/cli/cmd/togo@latest");
    process.exit(1);
  }, (e) => {
    console.error(`[togo] install failed: ${e.message}`);
    process.exit(1);
  });
}
