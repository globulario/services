"""Regression tests for detect-changes.py carry-forward semantics.

Reproduces the v1.2.156 release-pipeline defect: the carry-forward path
wrote the freshly-computed source-tree contract digest into
`entrypoint_checksum`, producing a BOM whose claimed binary hash did not
match the actual tarball on disk. `repository sync` rejected the import
on the next release that consumed this BOM (observed: cluster-controller
on 2026-06-04).

Each test sets up a temp services/packages tree, writes a prev_index
fixture, runs detect-changes.py as a subprocess, and inspects the
resulting change-manifest.json. Subprocess invocation matches how
release.yml invokes the script in CI.

Run:
    python3 -m unittest discover golang/build/

Or directly:
    python3 golang/build/detect_changes_test.py
"""

import hashlib
import json
import os
import shutil
import subprocess
import tempfile
import unittest

SCRIPT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "detect-changes.py")
PKG_MAP_JSON = os.path.join(os.path.dirname(os.path.abspath(__file__)), "pkg-map.json")


def sha256_of(content: bytes) -> str:
    return f"sha256:{hashlib.sha256(content).hexdigest()}"


def _write(path, content):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    if isinstance(content, str):
        content = content.encode()
    with open(path, "wb") as f:
        f.write(content)


class CarryForwardTest(unittest.TestCase):
    """Exercise the prev_index → BOM carry-forward path end-to-end."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="detect-changes-test-")
        self.metadata_dir = os.path.join(self.tmp, "metadata")
        self.bin_dir = os.path.join(self.tmp, "bin")
        self.go_src = os.path.join(self.tmp, "go")
        self.overrides = os.path.join(self.tmp, "overrides.txt")
        self.manifest = os.path.join(self.tmp, "change-manifest.json")
        os.makedirs(self.metadata_dir, exist_ok=True)
        os.makedirs(self.bin_dir, exist_ok=True)
        os.makedirs(self.go_src, exist_ok=True)

        # Minimal pkg-map for the one service we exercise. We use
        # cluster-controller to mirror the live regression exactly.
        self.pkg_map = os.path.join(self.tmp, "pkg-map.json")
        _write(self.pkg_map, json.dumps({
            "cluster-controller": {
                "binary": "cluster_controller_server",
                "go_target": "cluster_controller/cluster_controller_server",
                "kind": "service",
            },
        }))

        # Synthetic Go service source — content stays identical between
        # prev release and current run so the contract_digest matches.
        src_dir = os.path.join(
            self.go_src,
            "cluster_controller",
            "cluster_controller_server",
        )
        os.makedirs(src_dir, exist_ok=True)
        _write(os.path.join(src_dir, "main.go"),
               "package main\nfunc main(){}\n")
        # go.sum (any deterministic content)
        _write(os.path.join(self.go_src, "go.sum"),
               "module/dummy v1.0.0 h1:abc\n")

        # Minimal package.json for cluster-controller
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

        # Placeholder binary (release.yml builds the real one at sentinel
        # version; the script needs the file to exist to compute cksum).
        _write(os.path.join(self.bin_dir, "cluster_controller_server"),
               b"\x7fELF placeholder")

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def _run(self, prev_index_entry, version="1.2.157", tag="v1.2.157",
             extra_args=()):
        """Run detect-changes.py with a prev_index containing one entry and
        return the change-manifest result for that package."""
        prev_index_path = os.path.join(self.tmp, "prev-index.json")
        _write(prev_index_path, json.dumps({
            "schema_version": "globular.repository.index/v2",
            "platform_release": "1.2.156",
            "release_tag": "v1.2.156",
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
        ] + list(extra_args)
        result = subprocess.run(cmd, capture_output=True, text=True)
        if result.returncode != 0:
            self.fail(f"detect-changes.py failed (rc={result.returncode}):\n"
                      f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}")
        manifest = json.load(open(self.manifest))
        # Single package: cluster-controller
        for p in manifest["packages"]:
            if p["name"] == "cluster-controller":
                return p
        self.fail("cluster-controller not in change-manifest")

    # ── Test 1: unchanged carry-forward preserves entrypoint_checksum ──
    def test_unchanged_carry_forward_preserves_entrypoint_checksum(self):
        prev_entry = self._matching_prev_entry()
        prev_entrypoint = prev_entry["entrypoint_checksum"]
        entry = self._run(prev_entry)
        self.assertFalse(entry["changed"],
                         "package should be unchanged when contract matches")
        self.assertEqual(entry["entrypoint_checksum"], prev_entrypoint,
                         "carry-forward must preserve prev entrypoint_checksum, "
                         "not substitute a freshly-computed value")

    # ── Test 2: carry-forward does NOT write source-tree contract digest ──
    def test_unchanged_carry_forward_does_not_write_source_tree_hash(self):
        # The bug: detect-changes.py wrote `cksum` (source-tree combined
        # hash) into entrypoint_checksum. Assert the regression: the BOM's
        # entrypoint_checksum must NOT equal the contract_digest of the
        # carry-forward entry.
        prev_entry = self._matching_prev_entry()
        entry = self._run(prev_entry)
        self.assertFalse(entry["changed"])
        self.assertNotEqual(
            entry["entrypoint_checksum"],
            entry["package_contract_digest"],
            "entrypoint_checksum must be a binary hash, not the source-tree "
            "contract digest — this is the v1.2.156 regression",
        )

    # ── Test 3: changed package still gets binary entrypoint via packager ──
    def test_changed_package_marked_changed_with_cksum_placeholder(self):
        # When the contract digest differs from prev_index, the entry is
        # flagged changed=true. The "Package changed packages" step in
        # release.yml will re-pack the tarball and re-hash the binary,
        # overwriting entrypoint_checksum with the actual binary sha256.
        # detect-changes itself populates `cksum` as a placeholder; what
        # matters for this regression is that the field is NOT the prev
        # binary hash (which the packager will replace).
        prev_entry = self._matching_prev_entry()
        # Force a contract mismatch by changing prev's contract digest.
        prev_entry["package_contract_digest"] = "sha256:" + ("0" * 64)
        entry = self._run(prev_entry)
        self.assertTrue(entry["changed"],
                        "package with mismatched contract_digest must be marked changed")
        # The previous entrypoint must NOT be carried forward unchanged
        # for a changed package — the packager will replace it.
        self.assertNotEqual(entry["entrypoint_checksum"], prev_entry["entrypoint_checksum"],
                            "changed package must not silently carry forward prev entrypoint_checksum")

    # ── Test 4: package_digest is the tarball digest, distinct ──
    def test_package_digest_distinct_from_entrypoint_checksum(self):
        prev_entry = self._matching_prev_entry()
        # Set them deliberately different so we can tell them apart
        prev_entry["package_digest"] = "sha256:" + ("a" * 64)
        prev_entry["entrypoint_checksum"] = "sha256:" + ("b" * 64)
        entry = self._run(prev_entry)
        self.assertEqual(entry["package_digest"], prev_entry["package_digest"],
                         "carry-forward must preserve prev package_digest (tarball hash)")
        self.assertEqual(entry["entrypoint_checksum"], prev_entry["entrypoint_checksum"],
                         "carry-forward must preserve prev entrypoint_checksum (binary hash)")
        self.assertNotEqual(entry["package_digest"], entry["entrypoint_checksum"],
                            "tarball digest and binary entrypoint checksum are distinct fields")

    # ── Test 5: native-version carry-forward preserves identity ──
    def test_native_version_carry_forward_preserves_identity(self):
        prev_entry = self._matching_prev_entry()
        # Simulate a native-version (non-platform) package by using an
        # arbitrary upstream version + matching origin.
        prev_entry["version"] = "0.5.8"
        prev_entry["origin_release"] = "v1.2.135"
        prev_entry["build_number"] = 42
        prev_entry["build_id"] = "fixed-build-id-aaaa-bbbb-cccc-dddd"
        prev_entry["filename"] = "cluster-controller_0.5.8_linux_amd64.tgz"
        entry = self._run(prev_entry, version="1.2.157", tag="v1.2.157")
        self.assertFalse(entry["changed"])
        self.assertEqual(entry["version"], "0.5.8")
        self.assertEqual(entry["origin_release"], "v1.2.135")
        self.assertEqual(entry["build_number"], 42)
        self.assertEqual(entry["build_id"], "fixed-build-id-aaaa-bbbb-cccc-dddd")
        self.assertEqual(entry["filename"], "cluster-controller_0.5.8_linux_amd64.tgz")

    # ── Test 6: regression fixture — exact cluster-controller hashes ──
    def test_cluster_controller_v1_2_153_carry_forward_regression(self):
        # Reproduces the exact v1.2.156 failure:
        #   - prev (v1.2.153 release): entrypoint_checksum=sha256:5b01caab...
        #   - buggy v1.2.156 BOM had:  entrypoint_checksum=sha256:b10fff42...
        #     (the source-tree contract digest leaking into the binary field)
        # After the fix, the BOM must preserve the 5b01caab value verbatim.
        prev_entrypoint = "sha256:5b01caab2f21eaa72fb0d9fb935061d54e23bab26d93904368ceeef74b8ffb43"
        buggy_value     = "sha256:b10fff4221a42aa08b36a63dcd4e7e56349983379c556fc31c5d9f91038d489b"
        prev_entry = self._matching_prev_entry()
        prev_entry["entrypoint_checksum"] = prev_entrypoint
        prev_entry["package_digest"] = "sha256:25740ebd68dec11eac318f0e189f85dfad5d4fe2da6533745d9cfdcd8d58e9a8"
        prev_entry["build_id"] = "a2180517-1da1-4cf4-af47-3f5f155c7007"
        prev_entry["build_number"] = 408
        prev_entry["version"] = "1.2.153"
        prev_entry["origin_release"] = "v1.2.153"
        entry = self._run(prev_entry)
        self.assertFalse(entry["changed"])
        self.assertEqual(entry["entrypoint_checksum"], prev_entrypoint,
                         "regression: carry-forward must keep sha256:5b01caab... "
                         "(the actual v1.2.153 binary hash), not substitute the "
                         "buggy source-tree contract digest")
        self.assertNotEqual(entry["entrypoint_checksum"], buggy_value)
        # Provenance also preserved
        self.assertEqual(entry["build_id"], "a2180517-1da1-4cf4-af47-3f5f155c7007")
        self.assertEqual(entry["build_number"], 408)
        self.assertEqual(entry["version"], "1.2.153")

    # ── Helper ────────────────────────────────────────────────────────
    def _matching_prev_entry(self):
        """Build a prev_index entry whose contract_digest matches what
        detect-changes.py will compute for the synthetic source tree, so
        the carry-forward path is exercised."""
        # The contract digest is deterministic from the source tree +
        # manifest content. The simplest way to make prev match current is
        # to run detect-changes once with an EMPTY prev_index, capture the
        # resulting contract_digest, and feed it back as the prev_index
        # entry's package_contract_digest. The "changed" run gives us the
        # current digest verbatim.
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
        m = json.load(open(self.manifest))
        for p in m["packages"]:
            if p["name"] == "cluster-controller":
                # Build the prev entry: same contract_digest, with a
                # deliberate sentinel entrypoint_checksum so carry-forward
                # tests can detect substitution.
                return {
                    "name": "cluster-controller",
                    "kind": "service",
                    "version": "1.0.0",
                    "platform": "linux_amd64",
                    "publisher": "core@globular.io",
                    "package_contract_digest": p["package_contract_digest"],
                    "entrypoint_checksum": "sha256:" + ("e" * 64),  # sentinel
                    "package_digest": "sha256:" + ("d" * 64),       # sentinel
                    "build_id": "prev-build-id-1111-2222-3333-4444",
                    "build_number": 100,
                    "filename": "cluster-controller_1.0.0_linux_amd64.tgz",
                    "asset_url": "https://example.invalid/cluster-controller.tgz",
                    "origin_release": "v1.0.0",
                    "channel": "stable",
                }
        self.fail("could not seed prev entry")


if __name__ == "__main__":
    unittest.main()
