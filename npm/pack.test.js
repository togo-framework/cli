// Packaging guard (run: node --test npm/pack.test.js). Packs the npm package the
// way `npm publish` would and fails if any packed file contains a CR byte, so a
// CRLF checkout (core.autocrlf=true) can't reach the registry again (#9).
// Not published: package.json "files" lists only the runtime files.
const test = require("node:test");
const assert = require("node:assert");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { execFileSync } = require("child_process");
const { tarBin } = require("./install.js");

// filesWithCR returns the files under dir (relative, "/" separated) that contain a CR byte.
function filesWithCR(dir, rel = "") {
  const out = [];
  for (const e of fs.readdirSync(path.join(dir, rel), { withFileTypes: true })) {
    const r = rel ? `${rel}/${e.name}` : e.name;
    if (e.isDirectory()) out.push(...filesWithCR(dir, r));
    else if (fs.readFileSync(path.join(dir, r)).includes(0x0d)) out.push(r);
  }
  return out;
}

test("filesWithCR flags CRLF files and passes LF files", () => {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), "togo-cr-"));
  fs.mkdirSync(path.join(d, "bin"));
  fs.writeFileSync(path.join(d, "ok.js"), "a\nb\n");
  fs.writeFileSync(path.join(d, "bin", "bad.js"), "a\r\nb\r\n");
  assert.deepStrictEqual(filesWithCR(d), ["bin/bad.js"]);
});

test("npm pack output has no CR bytes", () => {
  const dest = fs.mkdtempSync(path.join(os.tmpdir(), "togo-pack-"));
  const npm = process.platform === "win32" ? "npm.cmd" : "npm";
  const raw = execFileSync(npm, ["pack", "--json", "--pack-destination", dest, "--ignore-scripts"], {
    cwd: __dirname,
    shell: process.platform === "win32",
    encoding: "utf8",
  });
  const filename = JSON.parse(raw.slice(raw.indexOf("[")))[0].filename;
  const unpacked = path.join(dest, "unpacked");
  fs.mkdirSync(unpacked);
  execFileSync(tarBin(process.platform, process.env), ["-xzf", path.join(dest, filename), "-C", unpacked]);
  assert.deepStrictEqual(filesWithCR(unpacked), [], "packed files must use LF line endings");
});
