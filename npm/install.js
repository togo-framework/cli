#!/usr/bin/env node
/*
 * Postinstall: download the prebuilt `togo` binary for this platform from the
 * matching GitHub release into ./bin. Falls back to `go install` if the download
 * fails (e.g. an unsupported platform but Go is present). Never hard-fails the
 * npm install — a missing binary is reported by bin/togo.js at run time.
 */
const fs = require("fs");
const path = require("path");
const https = require("https");
const { execFileSync } = require("child_process");
const { version } = require("./package.json");

const PLAT = { darwin: "darwin", linux: "linux", win32: "windows" }[process.platform];
const ARCH = { x64: "amd64", arm64: "arm64" }[process.arch];
const binDir = path.join(__dirname, "bin");
const binName = PLAT === "windows" ? "togo.exe" : "togo";

if (!PLAT || !ARCH) {
  console.error(`[togo] no prebuilt binary for ${process.platform}/${process.arch} — install from source: go install github.com/togo-framework/cli/cmd/togo@v${version}`);
  process.exit(0);
}

const ext = PLAT === "windows" ? "zip" : "tar.gz";
const asset = `togo_${PLAT}_${ARCH}.${ext}`;
const url = `https://github.com/togo-framework/cli/releases/download/v${version}/${asset}`;

function download(u, dest, redirects = 0) {
  return new Promise((resolve, reject) => {
    https.get(u, (res) => {
      if ([301, 302, 307, 308].includes(res.statusCode) && res.headers.location && redirects < 5) {
        res.resume();
        return resolve(download(res.headers.location, dest, redirects + 1));
      }
      if (res.statusCode !== 200) { res.resume(); return reject(new Error(`HTTP ${res.statusCode}`)); }
      const f = fs.createWriteStream(dest);
      res.pipe(f);
      f.on("finish", () => f.close(() => resolve(dest)));
      f.on("error", reject);
    }).on("error", reject);
  });
}

(async () => {
  fs.mkdirSync(binDir, { recursive: true });
  const archive = path.join(binDir, asset);
  try {
    await download(url, archive);
    // System `tar` extracts both .tar.gz and .zip (Windows 10+ ships bsdtar).
    execFileSync("tar", ["-xf", archive, "-C", binDir, binName], { stdio: "ignore" });
    fs.rmSync(archive, { force: true });
    if (PLAT !== "windows") fs.chmodSync(path.join(binDir, binName), 0o755);
    console.log(`[togo] installed ${binName} v${version}`);
  } catch (e) {
    fs.rmSync(archive, { force: true });
    console.error(`[togo] prebuilt download failed (${e.message}); trying 'go install'…`);
    try {
      execFileSync("go", ["install", `github.com/togo-framework/cli/cmd/togo@v${version}`],
        { stdio: "inherit", env: { ...process.env, GOBIN: binDir } });
      console.log(`[togo] installed via go install`);
    } catch (e2) {
      console.error(`[togo] could not install automatically. Download manually: https://github.com/togo-framework/cli/releases/tag/v${version}`);
    }
  }
})();
