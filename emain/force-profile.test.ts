import { mkdirSync, mkdtempSync, symlinkSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { resolveForceProfile } from "./force-profile";

describe("Force profile paths", () => {
    it("uses separate macOS production and development paths even when Wave overrides exist", () => {
        const home = "/Users/tester";
        const env = { WAVETERM_HOME: "/Users/tester/.waveterm", WAVETERM_DATA_HOME: "/tmp/wave" };
        expect(resolveForceProfile("darwin", home, false, env)).toEqual({
            data: "/Users/tester/Library/Application Support/force-terminal",
            config: "/Users/tester/.config/force-terminal",
            cache: "/Users/tester/Library/Caches/force-terminal",
        });
        expect(resolveForceProfile("darwin", home, true, env)).toEqual({
            data: "/Users/tester/Library/Application Support/force-terminal-dev",
            config: "/Users/tester/.config/force-terminal-dev",
            cache: "/Users/tester/Library/Caches/force-terminal-dev",
        });
    });

    it("accepts absolute test overrides for all three homes", () => {
        expect(
            resolveForceProfile("linux", "/home/tester", false, {
                FORCE_TERMINAL_DATA_HOME: "/tmp/force/data",
                FORCE_TERMINAL_CONFIG_HOME: "/tmp/force/config",
                FORCE_TERMINAL_CACHE_HOME: "/tmp/force/cache",
            })
        ).toEqual({ data: "/tmp/force/data", config: "/tmp/force/config", cache: "/tmp/force/cache" });
    });

    it("rejects relative and Wave directory overrides, including symlinks", () => {
        const home = mkdtempSync(path.join(os.tmpdir(), "force-profile-test-"));
        const wave = path.join(home, ".waveterm");
        mkdirSync(wave);
        symlinkSync(wave, path.join(home, "alias"));
        expect(() => resolveForceProfile("linux", home, false, { FORCE_TERMINAL_DATA_HOME: "relative" })).toThrow();
        expect(() => resolveForceProfile("linux", home, false, { FORCE_TERMINAL_DATA_HOME: wave })).toThrow();
        expect(() =>
            resolveForceProfile("linux", home, false, { FORCE_TERMINAL_DATA_HOME: path.join(home, "alias", "nested") })
        ).toThrow();
    });

    it("rejects a default Force data directory symlinked to Wave", () => {
        const home = mkdtempSync(path.join(os.tmpdir(), "force-profile-test-"));
        const wave = path.join(home, ".waveterm");
        mkdirSync(wave);
        mkdirSync(path.join(home, ".local", "share"), { recursive: true });
        symlinkSync(wave, path.join(home, ".local", "share", "force-terminal"));
        expect(() => resolveForceProfile("linux", home, false, {})).toThrow();
    });

    it("rejects macOS config overrides inside either Wave profile", () => {
        const home = "/Users/tester";
        expect(() =>
            resolveForceProfile("darwin", home, false, {
                FORCE_TERMINAL_CONFIG_HOME: `${home}/.config/waveterm/config`,
            })
        ).toThrow();
        expect(() =>
            resolveForceProfile("darwin", home, true, { FORCE_TERMINAL_CONFIG_HOME: `${home}/.config/waveterm-dev` })
        ).toThrow();
    });

    it("rejects Linux data roots inside the legacy Wave XDG profile", () => {
        const home = "/home/tester";
        expect(() =>
            resolveForceProfile("linux", home, false, { FORCE_TERMINAL_DATA_HOME: `${home}/.local/share/waveterm/db` })
        ).toThrow();
    });

    it("rejects Windows data and config roots inside Wave app-data namespaces", () => {
        const home = "C:\\Users\\tester";
        const env = { LOCALAPPDATA: `${home}\\AppData\\Local`, APPDATA: `${home}\\AppData\\Roaming` };
        expect(() =>
            resolveForceProfile("win32", home, false, {
                ...env,
                FORCE_TERMINAL_DATA_HOME: `${env.LOCALAPPDATA}\\waveterm\\db`,
            })
        ).toThrow();
        expect(() =>
            resolveForceProfile("win32", home, false, {
                ...env,
                FORCE_TERMINAL_CONFIG_HOME: `${env.APPDATA}\\waveterm-dev`,
            })
        ).toThrow();
    });
});
