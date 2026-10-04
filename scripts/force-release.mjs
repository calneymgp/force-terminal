import { createHash } from "node:crypto";
import { existsSync, readFileSync, readdirSync, statSync, writeFileSync } from "node:fs";
import { basename, join } from "node:path";
import YAML from "yaml";

function fail(message) {
    throw new Error(message);
}

function digest(file, algorithm, encoding = "hex") {
    return createHash(algorithm).update(readFileSync(file)).digest(encoding);
}

const [command, ...argv] = process.argv.slice(2);
const option = (name) => {
    const index = argv.indexOf(`--${name}`);
    return index < 0 ? undefined : argv[index + 1];
};
const dir = option("dir") ?? "make";
const version = option("version") ?? JSON.parse(readFileSync("package.json", "utf8")).version;
const tag = option("tag");
const name = "force-release-manifest.json";

try {
    if (!/^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$/.test(version)) fail("Invalid package version");
    if (tag && tag !== `v${version}`) fail("Tag differs from package version");
    const prefix = `force-terminal-darwin-arm64-${version}`;
    const files = [`${prefix}.dmg`, `${prefix}.dmg.blockmap`, `${prefix}.zip`, `${prefix}.zip.blockmap`, "latest-mac.yml"];
    if (command === "create") {
        const entries = readdirSync(dir).filter((file) => /\.(dmg|zip)(\.blockmap)?$|^latest-mac\.yml$/.test(file));
        if (entries.length !== files.length || files.some((file) => !entries.includes(file))) fail("Incomplete or unexpected release artifact set");
        for (const file of files) {
            const path = join(dir, file);
            if (!existsSync(path) || !statSync(path).isFile() || statSync(path).size === 0) fail(`Missing or empty artifact: ${file}`);
        }
        const feed = YAML.parse(readFileSync(join(dir, "latest-mac.yml"), "utf8"));
        if (feed?.version !== version) fail("Feed version differs from package version");
        if (feed?.path !== `${prefix}.zip` || feed?.sha512 !== digest(join(dir, `${prefix}.zip`), "sha512", "base64")) fail("Feed primary ZIP mismatch");
        for (const ext of ["zip", "dmg"]) {
            const file = `${prefix}.${ext}`;
            const feedFile = feed?.files?.find((entry) => entry.url === file);
            if (!feedFile || feedFile.sha512 !== digest(join(dir, file), "sha512", "base64") || feedFile.size !== statSync(join(dir, file)).size) fail(`Feed hash or size mismatch: ${file}`);
        }
        if (feed.files.length !== 2) fail("Feed has unexpected files");
        const manifest = {
            product: "Force Terminal",
            platform: "darwin",
            arch: "arm64",
            version,
            files: files.map((file) => ({ name: basename(file), size: statSync(join(dir, file)).size, sha256: digest(join(dir, file), "sha256") })),
        };
        writeFileSync(join(dir, name), `${JSON.stringify(manifest, null, 2)}\n`);
    } else if (command === "remote") {
        const manifest = JSON.parse(readFileSync(join(dir, name), "utf8"));
        const release = JSON.parse(readFileSync(option("assets"), "utf8"));
        if (release.draft !== true || !Array.isArray(release.assets)) fail("Release must remain a draft during verification");
        if (tag && release.tag_name !== tag) fail("Remote release tag mismatch");
        const expected = [...manifest.files, { name, size: statSync(join(dir, name)).size, sha256: digest(join(dir, name), "sha256") }];
        if (release.assets.length !== expected.length) fail("Remote asset count mismatch");
        for (const entry of expected) {
            const remote = release.assets.find((asset) => asset.name === entry.name);
            if (!remote || remote.state !== "uploaded" || remote.size !== entry.size || remote.digest !== `sha256:${entry.sha256}`) fail(`Remote asset mismatch: ${entry.name}`);
        }
    } else if (command === "verify") {
        const manifest = JSON.parse(readFileSync(join(dir, name), "utf8"));
        if (manifest.version !== version || manifest.platform !== "darwin" || manifest.arch !== "arm64" || manifest.product !== "Force Terminal") fail("Manifest metadata mismatch");
        if (manifest.files.length !== files.length || files.some((file) => !manifest.files.some((entry) => entry.name === file))) fail("Manifest artifact set mismatch");
        for (const entry of manifest.files) {
            if (!files.includes(entry.name) || entry.name !== basename(entry.name)) fail("Unsafe manifest filename");
            const path = join(dir, entry.name);
            if (!existsSync(path) || statSync(path).size !== entry.size || digest(path, "sha256") !== entry.sha256) fail(`Artifact hash mismatch: ${entry.name}`);
        }
    } else {
        fail("Usage: force-release.mjs create|verify [--dir PATH] [--version X.Y.Z] [--tag vX.Y.Z]");
    }
    process.stdout.write(`${command} verified ${version} macOS arm64 artifacts\n`);
} catch (error) {
    process.stderr.write(`${error.message}\n`);
    process.exitCode = 1;
}
