const MODES = ["adhoc", "community", "official"];

function releaseMode(env = process.env) {
    const mode = env.FORCE_RELEASE_MODE || "adhoc";
    if (!MODES.includes(mode)) throw new Error("FORCE_RELEASE_MODE must be adhoc, community or official");
    return mode;
}

function validateReleaseInputs(env = process.env) {
    const mode = releaseMode(env);
    if (mode === "community") {
        for (const name of ["FORCE_MACOS_CERT_P12_BASE64", "FORCE_MACOS_CERT_PASSWORD", "FORCE_MACOS_SIGNING_IDENTITY", "FORCE_MACOS_CERT_SHA256"]) {
            if (!env[name]?.trim()) throw new Error(`Required community signing input missing: ${name}`);
        }
        if (!/^[A-Za-z0-9+/]+={0,2}$/.test(env.FORCE_MACOS_CERT_P12_BASE64) || env.FORCE_MACOS_CERT_P12_BASE64.length % 4 !== 0) {
            throw new Error("FORCE_MACOS_CERT_P12_BASE64 must be base64");
        }
        if (!/^[a-fA-F0-9]{64}$/.test(env.FORCE_MACOS_CERT_SHA256)) {
            throw new Error("FORCE_MACOS_CERT_SHA256 must be a SHA-256 hex fingerprint");
        }
        if (env.FORCE_MACOS_SIGNING_IDENTITY.trim() === "-") throw new Error("Community signing identity cannot be ad hoc");
    }
    if (mode === "official") {
        for (const name of ["CSC_LINK", "CSC_KEY_PASSWORD", "APPLE_ID", "APPLE_APP_SPECIFIC_PASSWORD", "APPLE_TEAM_ID"]) {
            if (!env[name]?.trim()) throw new Error(`Required official signing or notarization input missing: ${name}`);
        }
    }
    return mode;
}

if (require.main === module) {
    try {
        validateReleaseInputs();
    } catch (error) {
        console.error(error.message);
        process.exitCode = 1;
    }
}

module.exports = { releaseMode, validateReleaseInputs };
