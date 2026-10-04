import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";

const require = createRequire(import.meta.url);
const { releaseMode, validateReleaseInputs } = require("./force-release-policy.cjs");
const { validate } = require("schema-utils");
const builderSchema = require("app-builder-lib/scheme.json");
const configPath = require.resolve("../electron-builder.config.cjs");
const signingInputs = ["FORCE_RELEASE_MODE", "FORCE_MACOS_CERT_P12_BASE64", "FORCE_MACOS_CERT_PASSWORD", "FORCE_MACOS_SIGNING_IDENTITY", "FORCE_MACOS_CERT_SHA256", "CSC_LINK", "CSC_KEY_PASSWORD", "APPLE_ID", "APPLE_APP_SPECIFIC_PASSWORD", "APPLE_TEAM_ID", "SM_CODE_SIGNING_CERT_SHA1_HASH"];
const originalInputs = Object.fromEntries(signingInputs.map((name) => [name, process.env[name]]));

function configFor(mode, identity) {
    process.env.FORCE_RELEASE_MODE = mode;
    if (mode === "community") {
        process.env.FORCE_MACOS_CERT_P12_BASE64 = "Y2VydA==";
        process.env.FORCE_MACOS_CERT_PASSWORD = "test-password";
        process.env.FORCE_MACOS_CERT_SHA256 = "a".repeat(64);
    }
    if (mode === "official") {
        for (const name of ["CSC_LINK", "CSC_KEY_PASSWORD", "APPLE_ID", "APPLE_APP_SPECIFIC_PASSWORD", "APPLE_TEAM_ID"]) process.env[name] = "test";
    }
    if (identity === undefined) delete process.env.FORCE_MACOS_SIGNING_IDENTITY;
    else process.env.FORCE_MACOS_SIGNING_IDENTITY = identity;
    delete require.cache[configPath];
    return require(configPath);
}

test.after(() => {
    for (const name of signingInputs) {
        if (originalInputs[name] === undefined) delete process.env[name];
        else process.env[name] = originalInputs[name];
    }
    delete require.cache[configPath];
});

test("community requires stable signing material and fingerprint, without Apple account inputs", () => {
    const env = {
        FORCE_RELEASE_MODE: "community",
        FORCE_MACOS_CERT_P12_BASE64: "Y2VydA==",
        FORCE_MACOS_CERT_PASSWORD: "test-password",
        FORCE_MACOS_SIGNING_IDENTITY: "Force Terminal Community",
        FORCE_MACOS_CERT_SHA256: "a".repeat(64),
    };
    assert.equal(validateReleaseInputs(env), "community");
    for (const name of Object.keys(env).filter((key) => key !== "FORCE_RELEASE_MODE")) {
        assert.throws(() => validateReleaseInputs({ ...env, [name]: "" }), new RegExp(name));
    }
    assert.throws(() => validateReleaseInputs({ ...env, FORCE_MACOS_SIGNING_IDENTITY: "-" }), /ad hoc/);
    assert.throws(() => validateReleaseInputs({ ...env, FORCE_MACOS_CERT_SHA256: "wrong" }), /SHA-256/);
    assert.throws(() => configFor("community"), /FORCE_MACOS_SIGNING_IDENTITY/);
    const config = configFor("community", env.FORCE_MACOS_SIGNING_IDENTITY);
    assert.equal(config.mac.identity, env.FORCE_MACOS_SIGNING_IDENTITY);
    assert.equal(config.mac.notarize, false);
    assert.equal(config.mac.forceCodeSigning, true);
    assert.equal(config.mac.preAutoEntitlements, false);
    assert.equal(config.extraResources.some((resource) => resource.to === "force-adhoc.json"), false);
});

test("ad hoc builds carry the updater blocker", () => {
    assert.equal(validateReleaseInputs({ FORCE_RELEASE_MODE: "adhoc" }), "adhoc");
    const config = configFor("adhoc");
    assert.equal(config.mac.identity, "-");
    assert.equal(config.mac.notarize, false);
    assert.equal(config.extraResources.some((resource) => resource.to === "force-adhoc.json"), true);
});

test("official builds retain Developer ID and notarization prerequisites", () => {
    const env = { FORCE_RELEASE_MODE: "official", CSC_LINK: "cert", CSC_KEY_PASSWORD: "password", APPLE_ID: "id", APPLE_APP_SPECIFIC_PASSWORD: "password", APPLE_TEAM_ID: "team" };
    assert.equal(validateReleaseInputs(env), "official");
    for (const name of Object.keys(env).filter((key) => key !== "FORCE_RELEASE_MODE")) {
        assert.throws(() => validateReleaseInputs({ ...env, [name]: "" }), new RegExp(name));
    }
    const config = configFor("official");
    assert.equal(config.mac.notarize, true);
    assert.equal(config.mac.forceCodeSigning, true);
    assert.equal(config.extraResources.some((resource) => resource.to === "force-adhoc.json"), false);
});

test("unknown modes fail closed", () => {
    assert.throws(() => releaseMode({ FORCE_RELEASE_MODE: "unknown" }), /FORCE_RELEASE_MODE/);
    assert.throws(() => configFor("unknown"), /FORCE_RELEASE_MODE/);
});

test("complete builder configuration satisfies the installed schema in every release mode", () => {
    delete process.env.SM_CODE_SIGNING_CERT_SHA1_HASH;
    for (const [mode, identity] of [["adhoc"], ["community", "Force Terminal Community"], ["official"]]) {
        const config = configFor(mode, identity);
        assert.equal(Object.hasOwn(config.win, "signtoolOptions"), false);
        assert.doesNotThrow(() => validate(builderSchema, config));
    }
    process.env.SM_CODE_SIGNING_CERT_SHA1_HASH = "a".repeat(40);
    const signedConfig = configFor("adhoc");
    assert.equal(signedConfig.win.signtoolOptions.certificateSha1, "a".repeat(40));
    assert.doesNotThrow(() => validate(builderSchema, signedConfig));
});
