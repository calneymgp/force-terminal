import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtempSync, readFileSync, unlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import { createRequire } from "node:module";
import test from "node:test";
import YAML from "yaml";

const require = createRequire(import.meta.url);
const { GitHubProvider } = require("electron-updater/out/providers/GitHubProvider.js");

const script = new URL("./force-release.mjs", import.meta.url).pathname;
const version = "1.2.3";
const zip = `force-terminal-darwin-arm64-${version}.zip`;
const dmg = `force-terminal-darwin-arm64-${version}.dmg`;
const sha = (data) => createHash("sha512").update(data).digest("base64");
const run = (dir, ...args) => spawnSync(process.execPath, [script, ...args, "--dir", dir, "--version", version], { encoding: "utf8" });

function fixture() {
    const dir = mkdtempSync(join(tmpdir(), "force-release-test-"));
    const zipData = Buffer.from("zip fixture");
    const dmgData = Buffer.from("dmg fixture");
    writeFileSync(join(dir, zip), zipData);
    writeFileSync(join(dir, dmg), dmgData);
    writeFileSync(join(dir, `${zip}.blockmap`), "zip blockmap");
    writeFileSync(join(dir, `${dmg}.blockmap`), "dmg blockmap");
    writeFileSync(join(dir, "latest-mac.yml"), `version: ${version}\nfiles:\n  - url: ${zip}\n    sha512: ${sha(zipData)}\n    size: ${zipData.length}\n  - url: ${dmg}\n    sha512: ${sha(dmgData)}\n    size: ${dmgData.length}\npath: ${zip}\nsha512: ${sha(zipData)}\n`);
    return dir;
}

test("actual GitHub updater resolves release URLs to the uploaded asset names", () => {
    const dir = fixture();
    const feed = YAML.parse(readFileSync(join(dir, "latest-mac.yml"), "utf8"));
    const provider = new GitHubProvider(
        { provider: "github", owner: "calneymgp", repo: "force-terminal" },
        { channel: null },
        { platform: "darwin", executor: {} },
    );
    const names = provider.resolveFiles({ ...feed, tag: `v${version}` }).map((entry) => decodeURIComponent(entry.url.pathname.split("/").at(-1)));
    assert.deepEqual(names, [zip, dmg]);
});

test("valid artifact set creates a manifest and verifies every recorded hash", () => {
    const dir = fixture();
    assert.equal(run(dir, "create").status, 0);
    assert.equal(run(dir, "verify").status, 0);
    const manifest = JSON.parse(readFileSync(join(dir, "force-release-manifest.json"), "utf8"));
    assert.equal(manifest.version, version);
    assert.equal(manifest.arch, "arm64");
    assert.equal(manifest.files.length, 5);
});

test("missing blockmap fails closed", () => {
    const dir = fixture();
    unlinkSync(join(dir, `${zip}.blockmap`));
    assert.notEqual(run(dir, "create").status, 0);
});

test("wrong feed hash fails closed", () => {
    const dir = fixture();
    writeFileSync(join(dir, "latest-mac.yml"), readFileSync(join(dir, "latest-mac.yml"), "utf8").replace(sha(Buffer.from("zip fixture")), "wronghash"));
    assert.notEqual(run(dir, "create").status, 0);
});

test("feed version, primary path and size must match final artifacts", () => {
    for (const [pattern, replacement] of [["version: 1.2.3", "version: 1.2.4"], [`path: ${zip}`, "path: old.zip"], ["size: 11", "size: 99"]]) {
        const dir = fixture();
        const feed = readFileSync(join(dir, "latest-mac.yml"), "utf8");
        writeFileSync(join(dir, "latest-mac.yml"), feed.replace(pattern, replacement));
        assert.notEqual(run(dir, "create").status, 0);
    }
});

test("version mismatch and modified artifact fail verification", () => {
    const dir = fixture();
    assert.notEqual(run(dir, "create", "--tag", "v1.2.4").status, 0);
    assert.equal(run(dir, "create", "--tag", "v1.2.3").status, 0);
    writeFileSync(join(dir, zip), "tampered");
    assert.notEqual(run(dir, "verify").status, 0);
});

test("missing builder update feed stops the release", () => {
    const dir = fixture();
    unlinkSync(join(dir, "latest-mac.yml"));
    assert.notEqual(run(dir, "create").status, 0);
});

test("builder update feed is preserved verbatim during validation", () => {
    const dir = fixture();
    const path = join(dir, "latest-mac.yml");
    const original = `${readFileSync(path, "utf8")}releaseDate: 2026-10-04T12:00:00Z\n`;
    writeFileSync(path, original);
    assert.equal(run(dir, "create").status, 0);
    assert.equal(readFileSync(path, "utf8"), original);
});

test("draft release assets must match local manifest names, sizes and SHA-256", () => {
    const dir = fixture();
    assert.equal(run(dir, "create").status, 0);
    const manifest = JSON.parse(readFileSync(join(dir, "force-release-manifest.json"), "utf8"));
    const assets = [...manifest.files, { name: "force-release-manifest.json", size: readFileSync(join(dir, "force-release-manifest.json")).length, sha256: createHash("sha256").update(readFileSync(join(dir, "force-release-manifest.json"))).digest("hex") }]
        .map((entry) => ({ name: entry.name, size: entry.size, digest: `sha256:${entry.sha256}`, state: "uploaded" }));
    const path = join(dir, "release.json");
    writeFileSync(path, JSON.stringify({ draft: true, tag_name: "v1.2.3", assets }));
    assert.equal(run(dir, "remote", "--assets", path, "--tag", "v1.2.3").status, 0);
    writeFileSync(path, JSON.stringify({ draft: true, tag_name: "v1.2.2", assets }));
    assert.notEqual(run(dir, "remote", "--assets", path, "--tag", "v1.2.3").status, 0);
    assets[0].digest = "sha256:wrong";
    writeFileSync(path, JSON.stringify({ draft: true, tag_name: "v1.2.3", assets }));
    assert.notEqual(run(dir, "remote", "--assets", path, "--tag", "v1.2.3").status, 0);
});
