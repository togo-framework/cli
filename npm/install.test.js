// Tests for the postinstall extraction (run: node --test npm/install.test.js).
// Not published: package.json "files" lists only the runtime files.
const test = require("node:test");
const assert = require("node:assert");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { execFileSync } = require("child_process");
const { tarBin, extract } = require("./install.js");

test("tarBin uses System32 bsdtar on Windows, PATH tar elsewhere", () => {
  assert.strictEqual(tarBin("win32", { SystemRoot: "D:\\Win" }), "D:\\Win\\System32\\tar.exe");
  assert.strictEqual(tarBin("win32", {}), "C:\\Windows\\System32\\tar.exe");
  assert.strictEqual(tarBin("linux", { SystemRoot: "C:\\Windows" }), "tar");
  assert.strictEqual(tarBin("darwin", {}), "tar");
});

function tmp() {
  return fs.mkdtempSync(path.join(os.tmpdir(), "togo-npm-"));
}

test("extracts the binary from a release .tar.gz", { skip: process.platform === "win32" }, () => {
  const src = tmp();
  fs.writeFileSync(path.join(src, "togo"), "binary");
  fs.writeFileSync(path.join(src, "README.md"), "readme");
  const archive = path.join(tmp(), "togo_linux_amd64.tar.gz");
  execFileSync("tar", ["-czf", archive, "-C", src, "togo", "README.md"]);

  const out = tmp();
  extract(archive, out, "togo");
  assert.strictEqual(fs.readFileSync(path.join(out, "togo"), "utf8"), "binary");
  assert.ok(!fs.existsSync(path.join(out, "README.md")), "only the binary is extracted");
});

// Regression for #5: npm's script-shell set to Git Bash puts Git's GNU tar ahead
// of System32 in PATH; GNU tar can't read the release .zip.
const gnuTarDir = process.platform === "win32" &&
  [process.env.ProgramFiles, process.env["ProgramFiles(x86)"]]
    .filter(Boolean)
    .map((p) => path.join(p, "Git", "usr", "bin"))
    .find((d) => fs.existsSync(path.join(d, "tar.exe")));

test("extracts the release .zip on Windows when GNU tar comes first in PATH",
  // CI must run it: only a local machine without Git for Windows may skip.
  { skip: process.platform !== "win32" ? "Windows only" : !gnuTarDir && !process.env.CI && "Git for Windows (GNU tar) not installed" },
  (t) => {
    assert.ok(gnuTarDir, "Git for Windows' GNU tar is required to reproduce #5");
    const src = tmp();
    fs.writeFileSync(path.join(src, "togo.exe"), "binary");
    fs.writeFileSync(path.join(src, "LICENSE"), "license");
    const archive = path.join(tmp(), "togo_windows_amd64.zip");
    execFileSync(tarBin(), ["-a", "-cf", archive, "-C", src, "togo.exe", "LICENSE"]);

    const origPath = process.env.PATH;
    process.env.PATH = `${gnuTarDir}${path.delimiter}${origPath}`;
    t.after(() => { process.env.PATH = origPath; });
    // Precondition: a bare `tar` now resolves to GNU tar, which is the bug.
    assert.match(execFileSync("tar", ["--version"]).toString(), /GNU tar/);

    const out = tmp();
    extract(archive, out, "togo.exe");
    assert.strictEqual(fs.readFileSync(path.join(out, "togo.exe"), "utf8"), "binary");
    assert.ok(!fs.existsSync(path.join(out, "LICENSE")), "only the binary is extracted");
  });
