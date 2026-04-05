#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TS_ROOT="$SCRIPT_DIR/typescript"
DIST_DIR="$TS_ROOT/dist"

if [ ! -d "$TS_ROOT" ]; then
  echo "TypeScript root not found at $TS_ROOT" >&2
  exit 1
fi

cd "$TS_ROOT"

echo "=> Bumping TypeScript package version"
NEW_VERSION="$(node <<'NODE'
const fs = require("fs");
const pkgPath = "package.json";
const pkg = JSON.parse(fs.readFileSync(pkgPath, "utf8"));
const [major, minor, patchRaw] = pkg.version.split(".").map((v) => v.replace(/^0+/, "") || "0");
const patch = String(Number(patchRaw) + 1);
pkg.version = [major, minor, patch].join(".");
fs.writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + "\n");
console.log(pkg.version);
NODE
)"

echo "=> Installing npm dependencies"
npm install --no-audit --no-fund >/dev/null

echo "=> Compiling TypeScript client"
npm run build_

if [ ! -f "$DIST_DIR/package.json" ]; then
  echo "dist/package.json not found after build" >&2
  exit 1
fi

echo "=> Syncing dist package version to $NEW_VERSION"
VERSION="$NEW_VERSION" node <<'NODE'
const fs = require("fs");
const path = require("path");
const version = process.env.VERSION;
if (!version) {
  throw new Error("VERSION env var is required");
}
const pkgPath = path.join("dist", "package.json");
const pkg = JSON.parse(fs.readFileSync(pkgPath, "utf8"));
pkg.version = version;
fs.writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + "\n");
NODE

echo "=> Publishing dist package"
(
  cd "$DIST_DIR"
  npm publish
)
