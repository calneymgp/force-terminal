import { existsSync, realpathSync } from "node:fs";
import path from "node:path";

export type ForceProfilePaths = { data: string; config: string; cache: string };

function resolveExistingAncestor(target: string, lib: typeof path.posix | typeof path.win32 = path.posix): string {
    let ancestor = target;
    const missing: string[] = [];
    while (!existsSync(ancestor)) {
        const parent = lib.dirname(ancestor);
        if (parent === ancestor) break;
        missing.unshift(lib.basename(ancestor));
        ancestor = parent;
    }
    return lib.join(realpathSync(ancestor), ...missing);
}

function validatedOverride(
    value: string | undefined,
    home: string,
    platform: NodeJS.Platform,
    env: NodeJS.ProcessEnv
): string | undefined {
    if (value === undefined || value === "") return undefined;
    const lib = platform === "win32" ? path.win32 : path.posix;
    if (!lib.isAbsolute(value)) throw new Error("Force profile override must be an absolute path");
    const target = lib.normalize(value);
    const waveHomes = [lib.join(home, ".waveterm"), lib.join(home, ".waveterm-dev")];
    const addWaveHomes = (base: string | undefined) => {
        if (base) waveHomes.push(lib.join(base, "waveterm"), lib.join(base, "waveterm-dev"));
    };
    addWaveHomes(lib.join(home, ".config"));
    addWaveHomes(lib.join(home, ".cache"));
    addWaveHomes(lib.join(home, ".local", "share"));
    if (platform === "darwin") {
        addWaveHomes(lib.join(home, "Library", "Application Support"));
        addWaveHomes(lib.join(home, "Library", "Caches"));
    } else if (platform === "win32") {
        addWaveHomes(env.LOCALAPPDATA || home);
        addWaveHomes(env.APPDATA || home);
    }
    if (platform !== "win32") {
        addWaveHomes(env.XDG_DATA_HOME);
        addWaveHomes(env.XDG_CONFIG_HOME);
        addWaveHomes(env.XDG_CACHE_HOME);
    }
    const canResolve = platform !== "win32" || process.platform === "win32";
    const resolved = canResolve ? resolveExistingAncestor(target, lib) : target;
    for (const waveHome of waveHomes) {
        const known = canResolve ? resolveExistingAncestor(waveHome, lib) : waveHome;
        const compared = platform === "win32" ? resolved.toLowerCase() : resolved;
        const comparedKnown = platform === "win32" ? known.toLowerCase() : known;
        if (compared === comparedKnown || compared.startsWith(comparedKnown + lib.sep)) {
            throw new Error("Force profile override points inside a Wave profile");
        }
    }
    return target;
}

export function resolveForceProfile(
    platform: NodeJS.Platform,
    home: string,
    isDev: boolean,
    env: NodeJS.ProcessEnv
): ForceProfilePaths {
    const name = isDev ? "force-terminal-dev" : "force-terminal";
    let data: string;
    let config: string;
    let cache: string;
    if (platform === "darwin") {
        data = path.join(home, "Library", "Application Support", name);
        config = path.join(home, ".config", name);
        cache = path.join(home, "Library", "Caches", name);
    } else if (platform === "win32") {
        data = path.win32.join(env.LOCALAPPDATA || home, name);
        config = path.win32.join(env.APPDATA || home, name);
        cache = path.win32.join(env.LOCALAPPDATA || home, name, "Cache");
    } else {
        data = path.join(env.XDG_DATA_HOME || path.join(home, ".local", "share"), name);
        config = path.join(env.XDG_CONFIG_HOME || path.join(home, ".config"), name);
        cache = path.join(env.XDG_CACHE_HOME || path.join(home, ".cache"), name);
    }
    return {
        data: validatedOverride(env.FORCE_TERMINAL_DATA_HOME || data, home, platform, env)!,
        config: validatedOverride(env.FORCE_TERMINAL_CONFIG_HOME || config, home, platform, env)!,
        cache: validatedOverride(env.FORCE_TERMINAL_CACHE_HOME || cache, home, platform, env)!,
    };
}
