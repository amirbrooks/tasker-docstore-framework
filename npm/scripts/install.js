const fs = require("node:fs");
const path = require("node:path");
const https = require("node:https");
const os = require("node:os");
const tar = require("tar");

const pkg = require("../package.json");
const version = process.env.npm_package_version || pkg.version;
const repo = "amirbrooks/tasker-docstore-framework";

const platformMap = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
};
const archMap = {
  x64: "amd64",
  arm64: "arm64",
};

function platform() {
  const goos = platformMap[process.platform];
  if (!goos) throw new Error(`Unsupported platform: ${process.platform}`);
  return goos;
}

function arch() {
  const goarch = archMap[process.arch];
  if (!goarch) throw new Error(`Unsupported arch: ${process.arch}`);
  return goarch;
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    https
      .get(url, (res) => {
        if (res.statusCode !== 200) {
          reject(new Error(`Download failed: ${res.statusCode} ${url}`));
          res.resume();
          return;
        }
        res.pipe(file);
        file.on("finish", () => file.close(resolve));
      })
      .on("error", (err) => {
        fs.unlink(dest, () => reject(err));
      });
  });
}

async function install() {
  const goos = platform();
  const goarch = arch();
  const filename = `tasker_${version}_${goos}_${goarch}.tar.gz`;
  const url = `https://github.com/${repo}/releases/download/v${version}/${filename}`;

  const binDir = path.join(__dirname, "..", "bin", `${process.platform}-${process.arch}`);
  const exeName = process.platform === "win32" ? "tasker.exe" : "tasker";
  const binPath = path.join(binDir, exeName);
  const versionFile = path.join(binDir, ".version");

  if (fs.existsSync(binPath) && fs.existsSync(versionFile)) {
    const current = fs.readFileSync(versionFile, "utf8").trim();
    if (current === version) return;
  }

  fs.mkdirSync(binDir, { recursive: true });
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "tasker-"));
  const archive = path.join(tmp, filename);

  await download(url, archive);
  await tar.x({ file: archive, cwd: binDir, strip: 0 });

  if (!fs.existsSync(binPath)) {
    throw new Error(`Binary not found after extract: ${binPath}`);
  }
  if (process.platform !== "win32") {
    fs.chmodSync(binPath, 0o755);
  }
  fs.writeFileSync(versionFile, version);
}

install().catch((err) => {
  console.error(err.message);
  console.error("Install failed. You can install manually:");
  console.error("  go install github.com/amirbrooks/tasker-docstore-framework/cmd/tasker@latest");
  process.exit(1);
});
