// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { autoUpdater as nativeAutoUpdater, ipcMain } from "electron";
import { autoUpdater } from "electron-updater";
import { existsSync, readFileSync } from "fs";
import path from "path";
import YAML from "yaml";
import { RpcApi } from "../frontend/app/store/wshclientapi";
import { isDev } from "../frontend/util/isdev";
import { fireAndForget } from "../frontend/util/util";
import { setUserConfirmedQuit } from "./emain-activity";
import { getAllBuilderWindows } from "./emain-builder";
import { getAllWaveWindows } from "./emain-window";
import { ElectronWshClient } from "./emain-wsh";

export let updater: Updater;
const CheckIntervalMs = 600000;
const RendererTimeoutMs = 10000;
const DisabledState: UpdaterState = { status: "dev-disabled" };
let configurationError: string | null = null;

function isUpdateDisabled(): boolean {
    return isDev() || existsSync(path.join(process.resourcesPath ?? "", "force-adhoc.json"));
}

function getUpdateChannel(settings: SettingsType): string {
    const updaterConfigPath = path.join(process.resourcesPath!, "app-update.yml");
    const updaterConfig = YAML.parse(readFileSync(updaterConfigPath, "utf8"));
    if (updaterConfig?.provider !== "github" || updaterConfig?.owner !== "calneymgp" || updaterConfig?.repo !== "force-terminal") {
        throw new Error("Update provider does not match Force Terminal");
    }
    const binaryChannel = updaterConfig?.channel ?? "latest";
    const configured = settings["autoupdate:channel"];
    if (!configured || (configured === "latest" && binaryChannel === "beta")) {
        fireAndForget(() => RpcApi.SetConfigCommand(ElectronWshClient, { "autoupdate:channel": binaryChannel }));
        return binaryChannel;
    }
    return configured;
}

export function requestRendererPreparation(webContents: Electron.WebContents, freeze = false): Promise<UpdatePreparation> {
    if (!webContents || webContents.isDestroyed() || webContents.isLoading()) {
        return Promise.resolve({ reasons: ["An editor window is unavailable"], verifiedIdleBlocks: [] });
    }
    const requestId = crypto.randomUUID();
    return new Promise((resolve) => {
        let done = false;
        const finish = (result: UpdatePreparation) => {
            if (done) return;
            done = true;
            clearTimeout(timeout);
            ipcMain.removeListener("update-prepare-reply", receive);
            resolve(result);
        };
        const receive = (event: Electron.IpcMainEvent, receivedId: string, result: UpdatePreparation) => {
            if (event.sender !== webContents || receivedId !== requestId) return;
            if (!result || !Array.isArray(result.reasons) || !Array.isArray(result.verifiedIdleBlocks)) {
                finish({ reasons: ["Invalid editor preparation response"], verifiedIdleBlocks: [] });
                return;
            }
            finish(result);
        };
        const timeout = setTimeout(() => finish({ reasons: ["An editor did not respond within 10 seconds"], verifiedIdleBlocks: [] }), RendererTimeoutMs);
        ipcMain.on("update-prepare-reply", receive);
        try {
            webContents.send("update-prepare", requestId, { freeze });
        } catch (error) {
            finish({ reasons: [`Editor preparation failed: ${String(error)}`], verifiedIdleBlocks: [] });
        }
    });
}

function collectEditorViews(): Map<number, Electron.WebContents> {
    const views = new Map<number, Electron.WebContents>();
    for (const window of getAllWaveWindows()) {
        for (const tab of window.allLoadedTabViews.values()) {
            if (tab.webContents) views.set(tab.webContents.id, tab.webContents);
        }
    }
    for (const builder of getAllBuilderWindows()) {
        if (builder.webContents) views.set(builder.webContents.id, builder.webContents);
    }
    return views;
}

function releaseEditorViews(views: Map<number, Electron.WebContents>) {
    for (const view of views.values()) {
        if (!view.isDestroyed()) view.send("update-prepare-release");
    }
}

function addedNativeDownloadListeners(before: Function[], after: Function[]): Function[] {
    const remaining = [...before];
    return after.filter((listener) => {
        const index = remaining.indexOf(listener);
        if (index < 0) return true;
        remaining.splice(index, 1);
        return false;
    });
}

async function prepareAllRenderers(views: Map<number, Electron.WebContents>): Promise<UpdatePreparation> {
    const results = await Promise.all([...views.values()].map((view) => requestRendererPreparation(view, true)));
    return {
        reasons: results.flatMap((result) => result.reasons),
        verifiedIdleBlocks: [...new Set(results.flatMap((result) => result.verifiedIdleBlocks))],
    };
}

export class Updater {
    autoCheckInterval: NodeJS.Timeout | null = null;
    intervalms: number;
    autoCheckEnabled: boolean;
    lastUpdateCheck = 0;
    private state: UpdaterState = { status: "pending" };
    private operation: Promise<void> | null = null;
    private downloaded = false;
    private manualRequested = false;
    private installingViews: Map<number, Electron.WebContents> | null = null;
    private nativeListenerBaseline: Function[] | null = null;
    private nativeAttemptListeners: Function[] | null = null;
    isPreparing = false;

    constructor(settings: SettingsType) {
        this.intervalms = Math.max(CheckIntervalMs, settings["autoupdate:intervalms"] ?? CheckIntervalMs);
        this.autoCheckEnabled = settings["autoupdate:enabled"] !== false;
        autoUpdater.autoDownload = false;
        autoUpdater.autoInstallOnAppQuit = false;
        autoUpdater.channel = getUpdateChannel(settings);
        autoUpdater.allowDowngrade = false;
        autoUpdater.on("checking-for-update", () => this.setState({ status: "checking" }));
        autoUpdater.on("update-available", (info) => this.setState({ status: "available", version: info.version }));
        autoUpdater.on("update-not-available", () => this.setState({ status: "up-to-date" }));
        autoUpdater.on("download-progress", (info) => this.setState({ status: "downloading", version: this.state.version, percent: info.percent }));
        autoUpdater.on("update-downloaded", (info) => {
            this.downloaded = true;
            this.setState({ status: "ready", version: info.version, percent: 100 });
        });
        autoUpdater.on("error", (error) => {
            if (this.installingViews) {
                this.clearNativeAttemptListeners();
                const views = this.installingViews;
                this.installingViews = null;
                setUserConfirmedQuit(false);
                releaseEditorViews(new Map([...views, ...collectEditorViews()]));
                this.setState({ status: "ready", version: this.state.version, percent: 100,
                    blockedReasons: [`Installation failed: ${String(error)}`] });
                return;
            }
            this.setState({ status: "error", error: String(error) });
        });
    }

    get status(): UpdaterStatus { return this.state.status; }
    get updateState(): UpdaterState { return this.state; }

    private clearNativeAttemptListeners() {
        if (!this.nativeListenerBaseline) return;
        const current = nativeAutoUpdater.listeners("update-downloaded");
        const extras = addedNativeDownloadListeners(this.nativeListenerBaseline, current);
        const attemptListeners = this.nativeAttemptListeners ?? extras;
        for (const listener of attemptListeners) {
            const index = extras.indexOf(listener);
            if (index < 0) continue;
            nativeAutoUpdater.removeListener("update-downloaded", listener as (...args: any[]) => void);
            extras.splice(index, 1);
        }
        this.nativeListenerBaseline = null;
        this.nativeAttemptListeners = null;
    }

    private setState(state: UpdaterState) {
        this.state = state;
        for (const window of getAllWaveWindows()) {
            for (const tab of window.allLoadedTabViews.values()) {
                if (!tab.webContents?.isDestroyed()) tab.webContents.send("app-update-status", state);
            }
        }
    }

    async start() {
        if (!this.autoCheckEnabled) return;
        this.autoCheckInterval = setInterval(() => fireAndForget(() => this.checkForUpdates(false)), CheckIntervalMs);
        await this.checkForUpdates(false);
    }

    stop() {
        if (this.autoCheckInterval) clearInterval(this.autoCheckInterval);
        this.autoCheckInterval = null;
    }

    async checkForUpdates(manual: boolean) {
        if (this.installingViews) return;
        if (manual) this.manualRequested = true;
        if (this.operation) return this.operation;
        if (!manual && (this.downloaded || Date.now() - this.lastUpdateCheck < this.intervalms)) return;
        this.operation = this.runCheck().finally(() => { this.operation = null; this.manualRequested = false; });
        return this.operation;
    }

    private async runCheck() {
        try {
            if (this.manualRequested && this.downloaded) {
                await this.installUpdate();
                return;
            }
            this.setState({ status: "checking" });
            const result = await autoUpdater.checkForUpdates();
            this.lastUpdateCheck = Date.now();
            if (!result?.isUpdateAvailable) {
                this.setState({ status: "up-to-date" });
                return;
            }
            this.setState({ status: "available", version: result.updateInfo.version });
            if (!this.manualRequested) return;
            this.setState({ status: "downloading", version: result.updateInfo.version, percent: 0 });
            await autoUpdater.downloadUpdate();
            this.downloaded = true;
            this.setState({ status: "ready", version: result.updateInfo.version, percent: 100 });
            await this.installUpdate();
        } catch (error) {
            this.setState({ status: "error", version: this.state.version, error: String(error) });
        }
    }

    async promptToInstallUpdate() { await this.checkForUpdates(true); }

    async installUpdate() {
        if (!this.downloaded) return;
        this.isPreparing = true;
        const preparedViews = collectEditorViews();
        let installing = false;
        try {
            const prepared = await prepareAllRenderers(preparedViews);
            const blockers = await RpcApi.GetUpdateBlockersCommand(ElectronWshClient, {
                verifiedidleblocks: prepared.verifiedIdleBlocks,
            });
            const reasons = [...prepared.reasons, ...blockers];
            if (reasons.length) {
                this.setState({ status: "ready", version: this.state.version, percent: 100, blockedReasons: reasons });
                return;
            }
            await RpcApi.FlushForUpdateCommand(ElectronWshClient, {});
            const finalBlockers = await RpcApi.GetUpdateBlockersCommand(ElectronWshClient, {
                verifiedidleblocks: prepared.verifiedIdleBlocks,
            });
            const currentViews = collectEditorViews();
            if (finalBlockers.length || currentViews.size !== preparedViews.size ||
                [...currentViews.keys()].some((id) => !preparedViews.has(id))) {
                this.setState({ status: "ready", version: this.state.version, percent: 100,
                    blockedReasons: [...finalBlockers, ...(currentViews.size !== preparedViews.size ||
                        [...currentViews.keys()].some((id) => !preparedViews.has(id)) ? ["Editor windows changed during preparation"] : [])] });
                return;
            }
            this.setState({ status: "installing", version: this.state.version, percent: 100 });
            setUserConfirmedQuit(true);
            this.installingViews = preparedViews;
            if (process.platform === "darwin") {
                this.nativeListenerBaseline = nativeAutoUpdater.listeners("update-downloaded");
                this.nativeAttemptListeners = null;
            }
            autoUpdater.quitAndInstall();
            if (this.nativeListenerBaseline) {
                this.nativeAttemptListeners = addedNativeDownloadListeners(
                    this.nativeListenerBaseline, nativeAutoUpdater.listeners("update-downloaded")
                );
            }
            installing = true;
        } catch (error) {
            this.clearNativeAttemptListeners();
            this.installingViews = null;
            setUserConfirmedQuit(false);
            this.setState({ status: "ready", version: this.state.version, percent: 100, blockedReasons: [`Update preparation failed: ${String(error)}`] });
        } finally {
            this.isPreparing = false;
            if (!installing) releaseEditorViews(new Map([...preparedViews, ...collectEditorViews()]));
        }
    }
}

export function getResolvedUpdateChannel(): string {
    return isUpdateDisabled() ? "dev" : (autoUpdater.channel ?? "latest");
}

ipcMain.on("install-app-update", () => fireAndForget(async () => {
    if (isUpdateDisabled()) return;
    if (!updater) await configureAutoUpdater();
    await updater?.promptToInstallUpdate();
}));
ipcMain.on("get-app-update-status", (event) => {
    event.returnValue = updater?.updateState ?? (isUpdateDisabled() ? DisabledState : configurationError ? { status: "error", error: configurationError } : { status: "pending" });
});
ipcMain.on("get-updater-channel", (event) => { event.returnValue = getResolvedUpdateChannel(); });

let configurationPromise: Promise<void> | null = null;
export async function configureAutoUpdater() {
    if (isUpdateDisabled()) return;
    if (configurationPromise) return configurationPromise;
    if (updater) return;
    configurationPromise = (async () => {
        try {
            const settings = (await RpcApi.GetFullConfigCommand(ElectronWshClient)).settings;
            updater = new Updater(settings);
            configurationError = null;
            await updater.start();
        } catch (error) {
            configurationError = String(error);
            console.warn("error configuring updater", error);
        }
    })().finally(() => { configurationPromise = null; });
    return configurationPromise;
}
