import { afterEach, describe, expect, it, vi } from "vitest";
import { ObjectService } from "./services";
import { WOS, canCloseTabWithForceAgents, createBlockSplitHorizontally, isBlockSafeForGenericAction, replaceBlock } from "./global";
import { LayoutModel } from "@/layout/lib/layoutModel";
import { newLayoutNode } from "@/layout/lib/layoutNode";

vi.mock("./windowtype", () => ({ isPreviewWindow: () => true }));

afterEach(() => vi.restoreAllMocks());

function loadedBlock(id: string, agent = false) {
    WOS.mockObjectForPreview(WOS.makeORef("block", id), { otype: "block", oid: id, version: 1, meta: agent ? { "force:agentinstanceid": "agent-id", view: "term" } : { view: "term" } } as unknown as Block);
    WOS.getWaveObjectAtom<Block>(WOS.makeORef("block", id));
    WOS.setObjectValue({ otype: "block", oid: id, version: 1, meta: agent ? { "force:agentinstanceid": "agent-id", view: "term" } : { view: "term" } } as unknown as Block);
}

describe("generic block actions and Force agents", () => {
    it("rejects replacement and split before creating a duplicate or changing layout", async () => {
        loadedBlock("agent-block", true);
        const create = vi.spyOn(ObjectService, "CreateBlock").mockResolvedValue("new-block");
        expect(isBlockSafeForGenericAction("agent-block")).toBe(false);
        expect(isBlockSafeForGenericAction("unloaded-block")).toBe(false);
        await expect(replaceBlock("agent-block", { meta: { view: "launcher" } }, true)).rejects.toThrow();
        await expect(createBlockSplitHorizontally({ meta: { view: "term", controller: "shell", "force:agentinstanceid": "agent-id" } as MetaType }, "agent-block", "after")).rejects.toThrow();
        loadedBlock("ordinary-block");
        expect(isBlockSafeForGenericAction("ordinary-block")).toBe(true);
        await expect(createBlockSplitHorizontally({ meta: { view: "term", "force:agentinstanceid": "agent-id" } as MetaType }, "ordinary-block", "after")).rejects.toThrow();
        expect(create).not.toHaveBeenCalled();
    });

    it("checks inactive tab contents before tab close and fails closed on missing data", async () => {
        vi.spyOn(ObjectService, "GetObject").mockResolvedValue({ otype: "tab", oid: "tab-1", version: 1, blockids: ["agent-block"] } as unknown as WaveObj);
        const getObjects = vi.spyOn(ObjectService, "GetObjects").mockResolvedValue([{ otype: "block", oid: "agent-block", version: 1, meta: { "force:agentinstanceid": "agent-id" } } as unknown as WaveObj]);
        expect(await canCloseTabWithForceAgents("tab-1")).toBe(false);
        getObjects.mockResolvedValueOnce([]);
        expect(await canCloseTabWithForceAgents("tab-1")).toBe(false);
        getObjects.mockResolvedValueOnce([{ otype: "block", oid: "agent-block", version: 1, meta: { view: "term" } } as unknown as WaveObj]);
        expect(await canCloseTabWithForceAgents("tab-1")).toBe(true);
        getObjects.mockRejectedValueOnce(new Error("unavailable"));
        expect(await canCloseTabWithForceAgents("tab-1")).toBe(false);
    });

    it("keeps the layout intact before backend deletion of a protected block", async () => {
        loadedBlock("agent-block", true);
        const model = Object.create(LayoutModel.prototype) as LayoutModel;
        const node = newLayoutNode(undefined, undefined, undefined, { blockId: "agent-block" });
        model.treeState = { rootNode: node } as typeof model.treeState;
        const reduce = vi.spyOn(model, "treeReducer");
        const deleteBlock = vi.fn();
        (model as any).onNodeDelete = deleteBlock;
        await model.closeNode(node.id);
        expect(reduce).not.toHaveBeenCalled();
        expect(deleteBlock).not.toHaveBeenCalled();
    });
});
