import { describe, expect, it, vi } from "vitest";
import { prepareLoadedModels } from "./update-guard";

describe("prepareLoadedModels", () => {
    it("awaits saves and rejects a preview that remains dirty", async () => {
        const save = vi.fn().mockResolvedValue(undefined);
        const result = await prepareLoadedModels([{ viewModel: { viewType: "preview", newFileContent: { dirty: true }, handleFileSave: save } }],
            (atom: any) => atom.dirty);
        expect(save).toHaveBeenCalledOnce();
        expect(result.reasons).toContain("Preview file still has unsaved changes");
    });

    it("rejects a config save that reports success but remains edited", async () => {
        const save = vi.fn().mockResolvedValue(undefined);
        const result = await prepareLoadedModels([{ viewModel: { viewType: "waveconfig", hasEditedAtom: { dirty: true }, saveFile: save } }],
            (atom: any) => atom.dirty);
        expect(result.reasons).toContain("Configuration still has unsaved changes");
    });

    it("accepts saved edits and only confirms loaded idle terminals", async () => {
        const previewDirty = { value: "draft" };
        const configDirty = { value: true };
        const result = await prepareLoadedModels([
            { viewModel: { viewType: "preview", newFileContent: previewDirty, handleFileSave: async () => { previewDirty.value = null; } } },
            { viewModel: { viewType: "waveconfig", hasEditedAtom: configDirty, saveFile: async () => { configDirty.value = false; } } },
            { viewModel: { viewType: "term", blockId: "idle", termRef: { current: { shellIntegrationStatusAtom: { value: "ready" } } } } },
            { viewModel: { viewType: "term", blockId: "busy", termRef: { current: { shellIntegrationStatusAtom: { value: "running-command" } } } } },
        ], (atom: any) => atom.value);
        expect(result).toEqual({ reasons: [], verifiedIdleBlocks: ["idle"] });
    });
});
