import { beforeEach, describe, expect, it, vi } from "vitest";

const fake = vi.hoisted(() => ({
    check: vi.fn(), download: vi.fn(), quit: vi.fn(), flush: vi.fn(), blockers: vi.fn(), fullConfig: vi.fn(), confirmQuit: vi.fn(),
    handlers: new Map<string, (...args: any[]) => void>(),
    ipcHandlers: new Set<(...args: any[]) => void>(),
    ipcNamed: new Map<string, (...args: any[]) => void>(),
    adhoc: false,
    configYaml: "provider: github\nowner: calneymgp\nrepo: force-terminal\nchannel: latest",
    windows: [] as any[],
    builders: [] as any[],
    nativeListeners: [] as Array<(event?: any) => void>,
    nativeQuit: vi.fn(),
}));
vi.mock("electron", () => ({ autoUpdater: {
    listeners: (event: string) => event === "update-downloaded" ? [...fake.nativeListeners] : [],
    on: (event: string, listener: (event?: any) => void) => {
        if (event === "update-downloaded") fake.nativeListeners.push(listener);
    },
    removeListener: (event: string, listener: (event?: any) => void) => {
        if (event === "update-downloaded") {
            const index = fake.nativeListeners.lastIndexOf(listener);
            if (index >= 0) fake.nativeListeners.splice(index, 1);
        }
    },
    emit: (event: string) => {
        if (event === "update-downloaded") for (const listener of [...fake.nativeListeners]) listener();
    },
}, ipcMain: {
    on: (name: string, handler: (...args: any[]) => void) => {
        if (name === "update-prepare-reply") fake.ipcHandlers.add(handler);
        else fake.ipcNamed.set(name, handler);
    },
    removeListener: (name: string, handler: (...args: any[]) => void) => { if (name === "update-prepare-reply") fake.ipcHandlers.delete(handler); },
} }));
vi.mock("electron-updater", () => ({ autoUpdater: {
    on: (name: string, handler: (...args: any[]) => void) => { fake.handlers.set(name, handler); },
    checkForUpdates: fake.check, downloadUpdate: fake.download, quitAndInstall: fake.quit,
} }));
vi.mock("fs", () => ({ readFileSync: () => fake.configYaml, existsSync: () => fake.adhoc }));
vi.mock("./emain-builder", () => ({ getAllBuilderWindows: () => fake.builders }));
vi.mock("./emain-window", () => ({ getAllWaveWindows: () => fake.windows }));
vi.mock("./emain-wsh", () => ({ ElectronWshClient: {} }));
vi.mock("./emain-activity", () => ({ setUserConfirmedQuit: fake.confirmQuit }));
vi.mock("../frontend/app/store/wshclientapi", () => ({ RpcApi: {
    SetConfigCommand: vi.fn(), GetFullConfigCommand: fake.fullConfig,
    FlushForUpdateCommand: fake.flush, GetUpdateBlockersCommand: fake.blockers,
} }));
vi.mock("../frontend/util/isdev", () => ({ isDev: () => false }));
vi.mock("../frontend/util/util", () => ({ fireAndForget: (fn: () => Promise<any>) => fn() }));

import { configureAutoUpdater, getResolvedUpdateChannel, requestRendererPreparation, Updater } from "./updater";
import { autoUpdater } from "electron-updater";

describe("Updater", () => {
    beforeEach(() => {
        (process as any).resourcesPath = "/test/resources";
        fake.check.mockReset(); fake.download.mockReset(); fake.quit.mockReset();
        fake.confirmQuit.mockReset();
        fake.flush.mockReset().mockResolvedValue(undefined);
        fake.blockers.mockReset().mockResolvedValue([]);
        fake.fullConfig.mockReset();
        fake.handlers.clear();
        fake.ipcHandlers.clear();
        fake.adhoc = false;
        fake.configYaml = "provider: github\nowner: calneymgp\nrepo: force-terminal\nchannel: latest";
        fake.windows = [];
        fake.builders = [];
        fake.nativeListeners = [];
        fake.nativeQuit.mockReset();
    });

    it("startup only checks metadata and exposes available version", async () => {
        fake.check.mockResolvedValue({ isUpdateAvailable: true, updateInfo: { version: "2.0" } });
        const updater = new Updater({ "autoupdate:enabled": true } as SettingsType);
        await updater.start();
        expect(updater.updateState).toEqual({ status: "available", version: "2.0" });
        expect(autoUpdater.autoDownload).toBe(false);
        expect(autoUpdater.autoInstallOnAppQuit).toBe(false);
        expect(fake.download).not.toHaveBeenCalled();
        updater.stop();
    });

    it("checks every 600000 ms without downloading", async () => {
        vi.useFakeTimers();
        vi.setSystemTime(new Date("2026-10-04T12:00:00Z"));
        try {
            fake.check.mockResolvedValue({ isUpdateAvailable: true, updateInfo: { version: "2.0" } });
            const updater = new Updater({ "autoupdate:enabled": true } as SettingsType);
            await updater.start();
            expect(fake.check).toHaveBeenCalledTimes(1);
            await vi.advanceTimersByTimeAsync(599999);
            expect(fake.check).toHaveBeenCalledTimes(1);
            await vi.advanceTimersByTimeAsync(1);
            expect(fake.check).toHaveBeenCalledTimes(2);
            expect(fake.download).not.toHaveBeenCalled();
            updater.stop();
        } finally {
            vi.useRealTimers();
        }
    });

    it("allows manual updates when automatic checks are disabled", async () => {
        fake.check.mockResolvedValue({ isUpdateAvailable: true, updateInfo: { version: "2.0" } });
        fake.download.mockResolvedValue([]);
        const updater = new Updater({ "autoupdate:enabled": false } as SettingsType);
        await updater.start();
        expect(fake.check).not.toHaveBeenCalled();
        await updater.checkForUpdates(true);
        expect(fake.check).toHaveBeenCalledOnce();
        expect(fake.download).toHaveBeenCalledOnce();
        expect(fake.quit).toHaveBeenCalledOnce();
    });

    it("manual click during a queued automatic check still downloads", async () => {
        let finishCheck: (value: any) => void;
        fake.check.mockReturnValue(new Promise((resolve) => { finishCheck = resolve; }));
        fake.download.mockResolvedValue([]);
        const updater = new Updater({} as SettingsType);
        const checking = updater.checkForUpdates(false);
        const clicked = updater.checkForUpdates(true);
        finishCheck({ isUpdateAvailable: true, updateInfo: { version: "2.0" } });
        await Promise.all([checking, clicked]);
        expect(fake.download).toHaveBeenCalledOnce();
        expect(fake.flush).toHaveBeenCalledOnce();
        expect(fake.quit).toHaveBeenCalledOnce();
    });

    it("shares one download and install across repeated clicks", async () => {
        let finishDownload: (value: string[]) => void;
        fake.check.mockResolvedValue({ isUpdateAvailable: true, updateInfo: { version: "2.0" } });
        fake.download.mockReturnValue(new Promise((resolve) => { finishDownload = resolve; }));
        const updater = new Updater({} as SettingsType);
        const first = updater.checkForUpdates(true);
        await vi.waitFor(() => expect(fake.download).toHaveBeenCalledOnce());
        const second = updater.checkForUpdates(true);
        finishDownload([]);
        await Promise.all([first, second]);
        expect(fake.check).toHaveBeenCalledOnce();
        expect(fake.download).toHaveBeenCalledOnce();
        expect(fake.flush).toHaveBeenCalledOnce();
        expect(fake.quit).toHaveBeenCalledOnce();
    });

    it("keeps a downloaded update ready after flush failure and retries without downloading again", async () => {
        fake.check.mockResolvedValue({ isUpdateAvailable: true, updateInfo: { version: "2.0" } });
        fake.download.mockResolvedValue([]);
        fake.flush.mockRejectedValueOnce(new Error("disk busy")).mockResolvedValueOnce(undefined);
        const updater = new Updater({} as SettingsType);
        await updater.checkForUpdates(true);
        expect(updater.updateState.status).toBe("ready");
        expect(updater.updateState.blockedReasons?.[0]).toContain("disk busy");
        expect(fake.quit).not.toHaveBeenCalled();
        await updater.checkForUpdates(true);
        expect(fake.check).toHaveBeenCalledOnce();
        expect(fake.download).toHaveBeenCalledOnce();
        expect(fake.flush).toHaveBeenCalledTimes(2);
        expect(fake.quit).toHaveBeenCalledOnce();
    });

    it("ignores stale or foreign renderer replies and times out safely", async () => {
        vi.useFakeTimers();
        try {
            const webContents = { id: 7, isDestroyed: () => false, isLoading: () => false, send: vi.fn() };
            const pending = requestRendererPreparation(webContents as any);
            const requestId = webContents.send.mock.calls[0][1];
            for (const handler of fake.ipcHandlers) {
                handler({ sender: { id: 8 } }, requestId, { reasons: [], verifiedIdleBlocks: [] });
                handler({ sender: webContents }, "stale-id", { reasons: [], verifiedIdleBlocks: [] });
            }
            await vi.advanceTimersByTimeAsync(10000);
            expect(await pending).toEqual({ reasons: ["An editor did not respond within 10 seconds"], verifiedIdleBlocks: [] });
            expect(fake.ipcHandlers.size).toBe(0);
        } finally {
            vi.useRealTimers();
        }
    });

    it("keeps a downloaded update ready when a terminal blocks restart", async () => {
        fake.check.mockResolvedValue({ isUpdateAvailable: true, updateInfo: { version: "2.0" } });
        fake.download.mockResolvedValue([]);
        fake.blockers.mockResolvedValue(["Terminal command is active"]);
        const updater = new Updater({} as SettingsType);
        await updater.checkForUpdates(true);
        expect(updater.updateState).toEqual({
            status: "ready", version: "2.0", percent: 100, blockedReasons: ["Terminal command is active"],
        });
        expect(fake.flush).not.toHaveBeenCalled();
        expect(fake.quit).not.toHaveBeenCalled();
    });

    it("does not claim a download completed after a download error", async () => {
        fake.check.mockResolvedValue({ isUpdateAvailable: true, updateInfo: { version: "2.0" } });
        fake.download.mockRejectedValue(new Error("network down"));
        const updater = new Updater({} as SettingsType);
        await updater.checkForUpdates(true);
        expect(updater.updateState.status).toBe("error");
        expect(updater.updateState.percent).toBeUndefined();
        expect(fake.quit).not.toHaveBeenCalled();
    });

    it("reports actual download progress before installation", async () => {
        let finishDownload: (value: string[]) => void;
        fake.check.mockResolvedValue({ isUpdateAvailable: true, updateInfo: { version: "2.0" } });
        fake.download.mockReturnValue(new Promise((resolve) => { finishDownload = resolve; }));
        const updater = new Updater({} as SettingsType);
        const installing = updater.checkForUpdates(true);
        await vi.waitFor(() => expect(fake.download).toHaveBeenCalledOnce());
        fake.handlers.get("download-progress")!({ percent: 42.5 });
        expect(updater.updateState).toEqual({ status: "downloading", version: "2.0", percent: 42.5 });
        finishDownload([]);
        await installing;
        expect(fake.quit).toHaveBeenCalledOnce();
    });

    it("does not configure the updater for an adhoc packaged build", async () => {
        fake.adhoc = true;
        await configureAutoUpdater();
        expect(getResolvedUpdateChannel()).toBe("dev");
        expect(fake.check).not.toHaveBeenCalled();
        expect(fake.download).not.toHaveBeenCalled();
    });

    it("rejects a release feed from another product before checking", () => {
        fake.configYaml = "provider: github\nowner: wavetermdev\nrepo: waveterm";
        expect(() => new Updater({} as SettingsType)).toThrow("Update provider does not match Force Terminal");
        expect(fake.check).not.toHaveBeenCalled();
    });

    it("freezes and releases the editor when a terminal blocks restart", async () => {
        const view = { id: 9, isDestroyed: () => false, isLoading: () => false, send: vi.fn() };
        view.send.mockImplementation((channel: string, requestId: string) => {
            if (channel === "update-prepare") {
                for (const handler of fake.ipcHandlers) handler({ sender: view }, requestId, { reasons: [], verifiedIdleBlocks: [] });
            }
        });
        fake.windows = [{ allLoadedTabViews: new Map([["tab", { webContents: view }]]) }];
        fake.check.mockResolvedValue({ isUpdateAvailable: true, updateInfo: { version: "2.0" } });
        fake.download.mockResolvedValue([]);
        fake.blockers.mockResolvedValue(["Terminal command is active"]);
        const updater = new Updater({} as SettingsType);
        await updater.checkForUpdates(true);
        expect(view.send).toHaveBeenCalledWith("update-prepare", expect.any(String), { freeze: true });
        expect(view.send).toHaveBeenCalledWith("update-prepare-release");
        expect(updater.updateState.status).toBe("ready");
        expect(fake.quit).not.toHaveBeenCalled();
    });

    it("aborts and releases if a new editor appears during flush", async () => {
        const view = { id: 9, isDestroyed: () => false, isLoading: () => false, send: vi.fn() };
        view.send.mockImplementation((channel: string, requestId: string) => {
            if (channel === "update-prepare") {
                for (const handler of fake.ipcHandlers) handler({ sender: view }, requestId, { reasons: [], verifiedIdleBlocks: [] });
            }
        });
        const tabs = new Map<string, any>([["tab", { webContents: view }]]);
        fake.windows = [{ allLoadedTabViews: tabs }];
        fake.check.mockResolvedValue({ isUpdateAvailable: true, updateInfo: { version: "2.0" } });
        fake.download.mockResolvedValue([]);
        fake.flush.mockImplementation(async () => {
            tabs.set("new", { webContents: { id: 10, isDestroyed: () => false, send: vi.fn() } });
        });
        const updater = new Updater({} as SettingsType);
        await updater.checkForUpdates(true);
        expect(updater.updateState.blockedReasons).toContain("Editor windows changed during preparation");
        expect(view.send).toHaveBeenCalledWith("update-prepare-release");
        expect(fake.quit).not.toHaveBeenCalled();
    });

    it("releases frozen editors after a native installation error and retries without downloading again", async () => {
        const originalPlatform = process.platform;
        Object.defineProperty(process, "platform", { value: "darwin", configurable: true });
        try {
            const baseline = vi.fn();
            fake.nativeListeners.push(baseline);
            fake.quit.mockImplementation(() => {
                fake.nativeListeners.push(() => fake.nativeQuit());
            });
            const view = { id: 9, isDestroyed: () => false, isLoading: () => false, send: vi.fn() };
            view.send.mockImplementation((channel: string, requestId: string) => {
                if (channel === "update-prepare") {
                    for (const handler of fake.ipcHandlers) handler({ sender: view }, requestId, { reasons: [], verifiedIdleBlocks: [] });
                }
            });
            fake.windows = [{ allLoadedTabViews: new Map([["tab", { webContents: view }]]) }];
            fake.check.mockResolvedValue({ isUpdateAvailable: true, updateInfo: { version: "2.0" } });
            fake.download.mockResolvedValue([]);
            const updater = new Updater({} as SettingsType);
            await updater.checkForUpdates(true);
            expect(updater.updateState.status).toBe("installing");
            await updater.checkForUpdates(true);
            expect(fake.quit).toHaveBeenCalledOnce();
            fake.handlers.get("error")!(new Error("signature rejected"));
            expect(view.send).toHaveBeenCalledWith("update-prepare-release");
            expect(fake.confirmQuit).toHaveBeenLastCalledWith(false);
            expect(updater.updateState.status).toBe("ready");
            expect(fake.nativeListeners).toEqual([baseline]);
            await updater.checkForUpdates(true);
            expect(fake.download).toHaveBeenCalledOnce();
            expect(fake.quit).toHaveBeenCalledTimes(2);
            for (const listener of [...fake.nativeListeners]) listener();
            expect(baseline).toHaveBeenCalledOnce();
            expect(fake.nativeQuit).toHaveBeenCalledOnce();
        } finally {
            Object.defineProperty(process, "platform", { value: originalPlatform, configurable: true });
        }
    });

    it("reports a startup configuration error rather than staying pending", async () => {
        fake.fullConfig.mockRejectedValue(new Error("config unavailable"));
        const warning = vi.spyOn(console, "warn").mockImplementation(() => {});
        try {
            await configureAutoUpdater();
        } finally {
            warning.mockRestore();
        }
        const event: { returnValue?: UpdaterState } = {};
        fake.ipcNamed.get("get-app-update-status")!(event);
        expect(event.returnValue?.status).toBe("error");
    });

    it("keeps a click received during initial configuration until updater is ready", async () => {
        let resolveConfig: (value: any) => void;
        fake.fullConfig.mockReturnValue(new Promise((resolve) => { resolveConfig = resolve; }));
        fake.check.mockResolvedValue({ isUpdateAvailable: true, updateInfo: { version: "2.0" } });
        fake.download.mockResolvedValue([]);
        const configuring = configureAutoUpdater();
        const sameConfiguration = configureAutoUpdater();
        const clicked = fake.ipcNamed.get("install-app-update")!();
        resolveConfig({ settings: { "autoupdate:enabled": false } });
        await Promise.all([configuring, sameConfiguration, clicked]);
        expect(fake.fullConfig).toHaveBeenCalledOnce();
        expect(fake.download).toHaveBeenCalledOnce();
        expect(fake.quit).toHaveBeenCalledOnce();
    });
});
