"""Regression tests for detect-changes.py carry-forward semantics.

Two compounding pipeline bugs led to these tests:

1. v1.2.156: carry-forward wrote `cksum` (source-tree contract digest)
   into `entrypoint_checksum`. The BOM claimed a binary hash that did
   not match the actual tarball.

2. v1.2.157: the v1.2.156 "preserve from prev_index" fix correctly
   preserved the value — but the prev_index (v1.2.156) was already
   polluted, so the pollution propagated unchanged.

The structural rule these tests pin (the v1.2.158 fix):
   For carry-forward (changed=false) entries, the authoritative source
   of entrypoint_checksum + provenance fields is the **origin release's**
   release-index, NOT the immediately previous release-index. The origin
   release is where the packager actually hashed the binary; intermediate
   carry-forwards cannot be trusted.

Run:
    python3 -m unittest discover golang/build/
"""

import hashlib
import json
import os
import shutil
import subprocess
import tempfile
import unittest

SCRIPT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "detect-changes.py")


def _write(path, content):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    if isinstance(content, str):
        content = content.encode()
    with open(path, "wb") as f:
        f.write(content)


class CarryForwardOriginAuthorityTest(unittest.TestCase):
    """End-to-end exercise of the carry-forward origin-recovery path.

    Each test sets up a temp services/packages tree, a prev_index, AND a
    synthetic origin-indices-dir, then runs detect-changes.py via subprocess.
    """

    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="detect-changes-test-")
        self.metadata_dir = os.path.join(self.tmp, "metadata")
        self.bin_dir = os.path.join(self.tmp, "bin")
        self.go_src = os.path.join(self.tmp, "go")
        self.origin_indices_dir = os.path.join(self.tmp, "origin-indices")
        self.overrides = os.path.join(self.tmp, "overrides.txt")
        self.manifest = os.path.join(self.tmp, "change-manifest.json")
        for d in (self.metadata_dir, self.bin_dir, self.go_src, self.origin_indices_dir):
            os.makedirs(d, exist_ok=True)

        # Minimal pkg-map — cluster-controller mirrors the live regression.
        self.pkg_map = os.path.join(self.tmp, "pkg-map.json")
        _write(self.pkg_map, json.dumps({
            "cluster-controller": {
                "binary": "cluster_controller_server",
                "go_target": "cluster_controller/cluster_controller_server",
                "kind": "service",
            },
        }))

        src_dir = os.path.join(
            self.go_src, "cluster_controller", "cluster_controller_server",
        )
        os.makedirs(src_dir, exist_ok=True)
        _write(os.path.join(src_dir, "main.go"),
               "package main\nfunc main(){}\n")
        _write(os.path.join(self.go_src, "go.sum"),
               "module/dummy v1.0.0 h1:abc\n")

        meta = os.path.join(self.metadata_dir, "cluster-controller")
        _write(os.path.join(meta, "package.json"), json.dumps({
            "type": "service",
            "name": "cluster-controller",
            "version": "1.0.0",
            "platform": "linux_amd64",
            "publisher": "core@globular.io",
            "profiles": ["control-plane"],
            "entrypoint": "bin/cluster_controller_server",
        }))

        # Placeholder binary so cksum computation succeeds.
        _write(os.path.join(self.bin_dir, "cluster_controller_server"),
               b"\x7fELF placeholder")

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def _write_origin_index(self, origin_tag, entry):
        """Place a synthetic origin release-index at <dir>/<tag>.json."""
        idx_path = os.path.join(self.origin_indices_dir, f"{origin_tag}.json")
        _write(idx_path, json.dumps({
            "schema_version": "globular.repository.index/v2",
            "platform_release": origin_tag.lstrip("v"),
            "release_tag": origin_tag,
            "packages": [entry],
        }))

    def _run(self, prev_index_entry, *, version="1.2.158", tag="v1.2.158",
             include_origin_dir=True, expect_failure=False):
        prev_index_path = os.path.join(self.tmp, "prev-index.json")
        _write(prev_index_path, json.dumps({
            "schema_version": "globular.repository.index/v2",
            "platform_release": "1.2.157",
            "release_tag": "v1.2.157",
            "packages": [prev_index_entry],
        }))
        cmd = [
            "python3", SCRIPT,
            "--prev-index", prev_index_path,
            "--metadata-dir", self.metadata_dir,
            "--bin-dir", self.bin_dir,
            "--pkg-map-json", self.pkg_map,
            "--go-src-dir", self.go_src,
            "--version", version,
            "--tag", tag,
            "--output-overrides", self.overrides,
            "--output-manifest", self.manifest,
        ]
        if include_origin_dir:
            cmd += ["--origin-indices-dir", self.origin_indices_dir]
        result = subprocess.run(cmd, capture_output=True, text=True)
        if expect_failure:
            if result.returncode == 0:
                self.fail(f"expected failure, got success:\n{result.stdout}")
            return result.stderr
        if result.returncode != 0:
            self.fail(
                f"detect-changes.py failed (rc={result.returncode}):\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        with open(self.manifest) as f:
            manifest = json.load(f)
        for p in manifest["packages"]:
            if p["name"] == "cluster-controller":
                return p
        self.fail("cluster-controller not in change-manifest")

    # ── Helper: build a prev_entry whose contract_digest matches current ──
    def _matching_prev_entry(self, **overrides):
        empty_prev = os.path.join(self.tmp, "empty-prev.json")
        _write(empty_prev, json.dumps({"packages": []}))
        cmd = [
            "python3", SCRIPT,
            "--prev-index", empty_prev,
            "--metadata-dir", self.metadata_dir,
            "--bin-dir", self.bin_dir,
            "--pkg-map-json", self.pkg_map,
            "--go-src-dir", self.go_src,
            "--version", "1.0.0",
            "--tag", "v1.0.0",
            "--output-overrides", self.overrides,
            "--output-manifest", self.manifest,
        ]
        subprocess.run(cmd, capture_output=True, text=True, check=True)
        with open(self.manifest) as f:
            m = json.load(f)
        for p in m["packages"]:
            if p["name"] == "cluster-controller":
                cd = p["package_contract_digest"]
                base = {
                    "name": "cluster-controller",
                    "kind": "service",
                    "version": "1.0.0",
                    "platform": "linux_amd64",
                    "publisher": "core@globular.io",
                    "package_contract_digest": cd,
                    # Sentinel: makes it obvious if prev is being used.
                    "entrypoint_checksum": "sha256:" + ("e" * 64),
                    "package_digest":      "sha256:" + ("d" * 64),
                    "build_id":             "prev-build-id-aaaa-bbbb-cccc-dddd",
                    "build_number":         100,
                    "filename":             "cluster-controller_1.0.0_linux_amd64.tgz",
                    "asset_url":            "https://example.invalid/cluster-controller.tgz",
                    "origin_release":       "v1.0.0",
                    "channel":              "stable",
                }
                base.update(overrides)
                return base
        self.fail("could not seed prev entry")

    # ── Test 1: clean prev_index carry-forward uses ORIGIN as authority ──
    def test_clean_prev_carry_forward_uses_origin(self):
        # Origin (v1.0.0) has the canonical entry; prev preserves the same
        # values from origin. detect-changes must produce a BOM entry that
        # carries the origin's values verbatim.
        origin_entry = {
            "name": "cluster-controller",
            "kind": "service",
            "version": "1.0.0",
            "platform": "linux_amd64",
            "publisher": "core@globular.io",
            "package_digest":      "sha256:" + ("d" * 64),
            "entrypoint_checksum": "sha256:" + ("a" * 64),  # the truth
            "build_id":             "origin-build-id-zzzz",
            "build_number":         100,
            "filename":             "cluster-controller_1.0.0_linux_amd64.tgz",
            "asset_url":            "https://example.invalid/origin.tgz",
            "origin_release":       "v1.0.0",
        }
        self._write_origin_index("v1.0.0", origin_entry)
        prev = self._matching_prev_entry(
            entrypoint_checksum=origin_entry["entrypoint_checksum"],
            origin_release="v1.0.0",
        )
        entry = self._run(prev)
        self.assertFalse(entry["changed"])
        self.assertEqual(entry["entrypoint_checksum"], origin_entry["entrypoint_checksum"])
        self.assertEqual(entry["build_id"], origin_entry["build_id"])
        self.assertEqual(entry["origin_release"], "v1.0.0")

    # ── Test 2: POLLUTED prev → REPAIRED from origin ──
    def test_polluted_prev_repaired_from_origin(self):
        # The v1.2.156 → v1.2.157 actual regression: prev_index has a wrong
        # entrypoint_checksum (the source-tree contract digest leaked into
        # the binary field), but origin's release-index has the truth.
        # detect-changes must IGNORE prev's polluted value and use origin.
        TRUTH        = "sha256:5b01caab2f21eaa72fb0d9fb935061d54e23bab26d93904368ceeef74b8ffb43"
        POLLUTED     = "sha256:b10fff4221a42aa08b36a63dcd4e7e56349983379c556fc31c5d9f91038d489b"
        self._write_origin_index("v1.0.0", {
            "name": "cluster-controller",
            "version": "1.0.0",
            "package_digest":      "sha256:" + ("d" * 64),
            "entrypoint_checksum": TRUTH,
            "build_id":             "origin-build-id-zzzz",
            "build_number":         100,
            "asset_url":            "https://example.invalid/origin.tgz",
            "filename":             "cluster-controller_1.0.0_linux_amd64.tgz",
            "origin_release":       "v1.0.0",
        })
        # prev carries the polluted value (as v1.2.156 BOM did).
        prev = self._matching_prev_entry(
            entrypoint_checksum=POLLUTED,
            origin_release="v1.0.0",
        )
        entry = self._run(prev)
        self.assertFalse(entry["changed"])
        self.assertEqual(entry["entrypoint_checksum"], TRUTH,
                         "carry-forward must take the truth from origin index, "
                         "not the polluted value in prev_index")
        self.assertNotEqual(entry["entrypoint_checksum"], POLLUTED,
                            "polluted prev value must be discarded")

    # ── Test 3: origin index file missing → fail closed ──
    def test_origin_index_missing_fails_closed(self):
        prev = self._matching_prev_entry(origin_release="v1.0.0")
        # No origin index written under self.origin_indices_dir.
        stderr = self._run(prev, expect_failure=True)
        self.assertIn("origin release-index for v1.0.0 not found", stderr)

    # ── Test 4: origin entry missing inside origin index → fail closed ──
    def test_origin_entry_missing_fails_closed(self):
        # Origin index exists but doesn't list cluster-controller.
        self._write_origin_index("v1.0.0", {
            "name": "some-other-package",
            "version": "1.0.0",
        })
        prev = self._matching_prev_entry(origin_release="v1.0.0")
        stderr = self._run(prev, expect_failure=True)
        self.assertIn("not found in origin release-index", stderr)

    # ── Test 5: native-version packages preserve identity from origin ──
    def test_native_version_carry_forward_preserves_identity(self):
        origin_entry = {
            "name": "cluster-controller",
            "version": "0.5.8",                # native-version
            "package_digest":      "sha256:" + ("e" * 64),
            "entrypoint_checksum": "sha256:" + ("f" * 64),
            "build_id":             "stable-native-bid-1234",
            "build_number":         42,
            "filename":             "cluster-controller_0.5.8_linux_amd64.tgz",
            "asset_url":            "https://example.invalid/native.tgz",
            "origin_release":       "v1.0.0-native",
        }
        self._write_origin_index("v1.0.0-native", origin_entry)
        prev = self._matching_prev_entry(
            version="0.5.8",
            origin_release="v1.0.0-native",
            build_id="stale-prev-bid-9999",   # prev has STALE value
            build_number=999,                  # prev has STALE value
        )
        entry = self._run(prev)
        self.assertFalse(entry["changed"])
        self.assertEqual(entry["version"], "0.5.8")
        self.assertEqual(entry["build_id"], "stable-native-bid-1234",
                         "carry-forward must take build_id from origin, "
                         "ignoring any stale prev_index value")
        self.assertEqual(entry["build_number"], 42,
                         "carry-forward must take build_number from origin")
        self.assertEqual(entry["origin_release"], "v1.0.0-native")

    # ── Test 6: entrypoint_checksum never == source-tree contract digest ──
    def test_entrypoint_checksum_never_source_tree_hash(self):
        # The original v1.2.156 bug class. Even with a clean origin, the
        # output BOM entry must not have entrypoint_checksum equal to the
        # freshly-computed package_contract_digest (which is the source-tree
        # combined hash for Go services).
        self._write_origin_index("v1.0.0", {
            "name": "cluster-controller",
            "version": "1.0.0",
            "package_digest":      "sha256:" + ("d" * 64),
            "entrypoint_checksum": "sha256:" + ("a" * 64),
            "build_id":             "origin-build-id",
            "build_number":         100,
            "filename":             "x.tgz",
            "asset_url":            "https://example.invalid/x.tgz",
        })
        prev = self._matching_prev_entry(
            origin_release="v1.0.0",
            entrypoint_checksum="sha256:" + ("e" * 64),
        )
        entry = self._run(prev)
        self.assertNotEqual(
            entry["entrypoint_checksum"], entry["package_contract_digest"],
            "entrypoint_checksum must be a BINARY hash, not the source-tree "
            "contract digest (v1.2.156 regression)",
        )

    # ── Test 7: missing/empty origin_release in prev → fail closed ──
    def test_missing_origin_release_in_prev_fails_closed(self):
        # Defensive: prev entry has no origin_release. detect-changes
        # cannot establish authoritative provenance.
        prev = self._matching_prev_entry()
        prev["origin_release"] = ""
        # remove the release_tag fallback too
        prev.pop("release_tag", None)
        stderr = self._run(prev, expect_failure=True)
        self.assertIn("no origin_release", stderr)

    # ── Test 8: regression fixture — exact cluster-controller hashes ──
    def test_cluster_controller_v1_2_153_regression(self):
        TRUTH_FROM_V1_2_153 = "sha256:5b01caab2f21eaa72fb0d9fb935061d54e23bab26d93904368ceeef74b8ffb43"
        POLLUTED_V1_2_156   = "sha256:b10fff4221a42aa08b36a63dcd4e7e56349983379c556fc31c5d9f91038d489b"
        self._write_origin_index("v1.2.153", {
            "name": "cluster-controller",
            "version": "1.2.153",
            "package_digest":      "sha256:25740ebd68dec11eac318f0e189f85dfad5d4fe2da6533745d9cfdcd8d58e9a8",
            "entrypoint_checksum": TRUTH_FROM_V1_2_153,
            "build_id":             "a2180517-1da1-4cf4-af47-3f5f155c7007",
            "build_number":         408,
            "filename":             "cluster-controller_1.2.153_linux_amd64.tgz",
            "asset_url":            "https://github.com/globulario/services/releases/download/v1.2.153/cluster-controller_1.2.153_linux_amd64.tgz",
        })
        prev = self._matching_prev_entry(
            version="1.2.153",
            origin_release="v1.2.153",
            # prev (simulating v1.2.156) carries the polluted value
            entrypoint_checksum=POLLUTED_V1_2_156,
        )
        entry = self._run(prev)
        self.assertEqual(entry["entrypoint_checksum"], TRUTH_FROM_V1_2_153,
                         "must recover the actual v1.2.153 binary hash from "
                         "origin release-index, not propagate v1.2.156 pollution")
        self.assertNotEqual(entry["entrypoint_checksum"], POLLUTED_V1_2_156)
        self.assertEqual(entry["build_id"], "a2180517-1da1-4cf4-af47-3f5f155c7007")


if __name__ == "__main__":
    unittest.main()
