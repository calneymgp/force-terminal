import { atom } from "jotai";
import { afterEach, describe, expect, it, vi } from "vitest";
import { RpcApi } from "@/app/store/wshclientapi";
import { WOS } from "@/app/store/global";
import { initGlobalAtoms } from "@/app/store/global-atoms";
import { TermViewModel } from "./term-model";

vi.mock("@/app/view/term/term", () => ({ TermClaudeIcon: () => null, TerminalView: () => null }));
vi.mock("@/app/store/windowtype", async (importOriginal) => ({ ...await importOriginal<typeof import("@/app/store/windowtype")>(), isPreviewWindow: () => true }));

afterEach(() => vi.restoreAllMocks());

describe("Force agent terminal safety", () => {
    it("does not destroy an agent process or change durability through generic restart actions", async () => {
        const destroy = vi.spyOn(RpcApi, "ControllerDestroyCommand").mockRejectedValue(new Error("destructive restart called"));
        const resync = vi.spyOn(RpcApi, "ControllerResyncCommand").mockResolvedValue(undefined);
        const setMeta = vi.spyOn(RpcApi, "SetMetaCommand").mockResolvedValue(undefined);
        const model = Object.create(TermViewModel.prototype) as TermViewModel;
        model.blockId = "agent-block";
        model.blockAtom = atom({ meta: { "force:agentinstanceid": "agent-id" } } as unknown as Block);
        model.isRestarting = atom(false);
        model.termRef = { current: null };

        await model.forceRestartController();
        await model.restartSessionWithDurability(true);

        expect(destroy).not.toHaveBeenCalled();
        expect(resync).not.toHaveBeenCalled();
        expect(setMeta).not.toHaveBeenCalled();
    });

    it("keeps the generic restart path for an ordinary terminal", async () => {
        const destroy = vi.spyOn(RpcApi, "ControllerDestroyCommand").mockRejectedValue(new Error("restart reached"));
        const model = Object.create(TermViewModel.prototype) as TermViewModel;
        model.blockId = "ordinary-block";
        model.blockAtom = atom({ meta: { view: "term" } } as unknown as Block);
        model.isRestarting = atom(false);
        model.termRef = { current: null };

        await expect(model.forceRestartController()).rejects.toThrow("restart reached");
        expect(destroy).toHaveBeenCalledOnce();
    });

    it("preserves file and appearance actions while omitting agent lifecycle settings", () => {
        vi.spyOn(console, "log").mockImplementation(() => {});
        initGlobalAtoms({ windowId: "test-window", tabId: "test-tab", isPreview: true } as GlobalInitOptions);
        const block = { otype: "block", oid: "agent-menu-block", version: 1, meta: {
            view: "term", "force:agentinstanceid": "agent-id", "cmd:cwd": "/project", connection: "remote",
            "term:conndebug": "debug", "term:vdomtoolbarblockid": "toolbar",
        } } as unknown as Block;
        WOS.mockObjectForPreview(WOS.makeORef("block", block.oid), block);
        const model = Object.create(TermViewModel.prototype) as TermViewModel;
        model.blockId = block.oid;
        model.blockAtom = atom(block);
        model.termRef = { current: { lastCommandAtom: atom("ls") } } as unknown as typeof model.termRef;
        const menu = model.getSettingsMenuItems();
        const labels = menu.map((item) => item.label);
        const advanced = menu.find((item) => item.label === "Advanced")?.submenu as ContextMenuItem[];
        const advancedLabels = advanced.map((item) => item.label);
        expect(labels).not.toContain("Split Horizontally");
        expect(labels).not.toContain("Split Vertically");
        expect(labels).toContain("File Browser");
        expect(labels).not.toContain("Close Toolbar");
        expect(advancedLabels).not.toContain("Clear Output On Restart");
        expect(advancedLabels).not.toContain("Run On Startup");
        expect(advancedLabels).not.toContain("Debug Connection");
        expect(labels).toContain("Save Session As...");
        expect(labels).toContain("Themes");

        const ordinary = { ...block, oid: "ordinary-menu-block", meta: { view: "term", "cmd:cwd": "/project", connection: "remote" } } as unknown as Block;
        WOS.mockObjectForPreview(WOS.makeORef("block", ordinary.oid), ordinary);
        model.blockId = ordinary.oid;
        model.blockAtom = atom(ordinary);
        const ordinaryMenu = model.getSettingsMenuItems();
        expect(ordinaryMenu.map((item) => item.label)).toContain("Split Horizontally");
        expect(ordinaryMenu.map((item) => item.label)).toContain("File Browser");
        const ordinaryAdvanced = ordinaryMenu.find((item) => item.label === "Advanced")?.submenu as ContextMenuItem[];
        expect(ordinaryAdvanced.map((item) => item.label)).toContain("Run On Startup");
        expect(ordinaryAdvanced.map((item) => item.label)).toContain("Debug Connection");
    });
});
