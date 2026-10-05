import { describe, expect, it, vi } from "vitest";
import { ForceAgentWorkflow, agentStatusLabel, safeAgentCause, type AgentInstance, type AgentServicePort } from "./force-agent-model";

function fixture(): AgentInstance {
	return { otype: "forceagentinstance", oid: "agent-1", version: 1, meta: {}, projectid: "project-1", projectversion: 2, profileid: "profile-1", profileversion: 3,
        tabid: "tab-1", blockid: "block-1", titlesnapshot: "DevOps", adapter: "claude-code", connection: "",
        iconsnapshot: "server", promptsnapshot: "test-owned prompt", prompthash: "test-owned hash", adapterversion: "claude-code-cli-v1",
        rootpath: "/test-owned/project", generation: 0, createdat: 1, updatedat: 1,
        status: "prepared", identityevidence: "requested" };
}

function fakePort() {
    const instance = fixture();
    return {
        CreateAgentInstance: vi.fn(async (_input: Parameters<AgentServicePort["CreateAgentInstance"]>[0]) => instance),
        ListAgentInstances: vi.fn(async (_projectID: string) => [instance]),
        StartAgent: vi.fn(async (_instanceID: string, requestKey: string) => ({ instance, operation: { generation: 1, requestkey: requestKey, intent: "start", phase: "launch_requested", status: "running" }, terminalblockid: instance.blockid })),
        ReconnectAgent: vi.fn(async (_instanceID: string, requestKey: string) => ({ instance, operation: { generation: 2, requestkey: requestKey, intent: "reconnect", phase: "launch_requested", status: "running" }, terminalblockid: instance.blockid })),
        StartNewSession: vi.fn(async (_instanceID: string, requestKey: string) => ({ instance, operation: { generation: 3, requestkey: requestKey, intent: "new-session", phase: "launch_requested", status: "running" }, terminalblockid: instance.blockid })),
        FocusAgentTerminal: vi.fn(async (_instanceID: string) => undefined),
    } satisfies AgentServicePort;
}

function keyStore() {
    const values = new Map<string, string>();
    return { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => { values.set(key, value); }, removeItem: (key: string) => { values.delete(key); } };
}

describe("ForceAgentWorkflow", () => {
    it("listing/restoring agents never starts a process", async () => {
        const api = fakePort();
        const workflow = new ForceAgentWorkflow(api, keyStore(), () => "key-1");
        expect(await workflow.list("project-1")).toHaveLength(1);
        expect(api.StartAgent).not.toHaveBeenCalled();
        expect(api.ReconnectAgent).not.toHaveBeenCalled();
    });

    it("serializes two explicit start clicks with one request key", async () => {
        const api = fakePort();
        let release: (value: Awaited<ReturnType<typeof api.StartAgent>>) => void;
        api.StartAgent.mockImplementation(() => new Promise((resolve) => { release = resolve; }));
        const workflow = new ForceAgentWorkflow(api, keyStore(), () => "stable-key");
        const first = workflow.operate("start", fixture());
        const second = workflow.operate("start", fixture());
        expect(api.StartAgent).toHaveBeenCalledTimes(1);
        expect(api.StartAgent).toHaveBeenCalledWith("agent-1", "stable-key");
        release!({ instance: fixture(), operation: { generation: 1, requestkey: "stable-key", intent: "start", phase: "launch_requested", status: "running" }, terminalblockid: "block-1" });
        expect(await first).toEqual(await second);
    });

    it("reuses a creation key after a lost response and never starts automatically", async () => {
        const api = fakePort();
        api.CreateAgentInstance.mockRejectedValueOnce(new Error("response lost"));
        const keys = keyStore();
        const input = { projectid: "project-1", projectversion: 2, profileid: "profile-1", profileversion: 3, tabid: "tab-1" };
        const first = new ForceAgentWorkflow(api, keys, () => "creation-key");
        await expect(first.create(input)).rejects.toThrow();
        const restored = new ForceAgentWorkflow(api, keys, () => "different-key");
        await restored.create(input);
        expect(api.CreateAgentInstance.mock.calls.map(([value]) => value.creationkey)).toEqual(["creation-key", "creation-key"]);
        expect(api.StartAgent).not.toHaveBeenCalled();
    });

    it("failed exact reconnect keeps its key and never falls back to start/new session", async () => {
        const api = fakePort();
        api.ReconnectAgent.mockRejectedValueOnce(new Error("uncertain"));
        const keys = keyStore();
        const first = new ForceAgentWorkflow(api, keys, () => "resume-key");
        await expect(first.operate("reconnect", fixture())).rejects.toThrow();
        const restored = new ForceAgentWorkflow(api, keys, () => "different-key");
        await restored.operate("reconnect", fixture());
        expect(api.ReconnectAgent.mock.calls.map(([, key]) => key)).toEqual(["resume-key", "resume-key"]);
        expect(api.StartAgent).not.toHaveBeenCalled();
        expect(api.StartNewSession).not.toHaveBeenCalled();
    });

    it("keeps provider identity unconfirmed despite a running process and maps safe causes", () => {
        expect(agentStatusLabel({ ...fixture(), status: "running", identityevidence: "requested" })).toContain("Conversa preparada");
        expect(safeAgentCause(new Error("agent destination already has a writer"))).toContain("checkouts separados");
        expect(safeAgentCause(new Error("SECRET_PROMPT unknown error"))).not.toContain("SECRET_PROMPT");
        expect(safeAgentCause(new Error("process_group_members_remaining"))).toContain("destino permanece reservado");
    });

    it("reports a known preparation failure and gives its explicit retry a new operation key", async () => {
        const api = fakePort();
        api.StartAgent.mockResolvedValueOnce({ instance: { ...fixture(), errorcode: "cli_missing", status: "prepared", operationphase: "prepare_failed" },
            operation: { generation: 1, requestkey: "attempt-1", intent: "start", phase: "prepare_failed", status: "prepared" }, terminalblockid: "block-1" });
        let nextKey = 0;
        const workflow = new ForceAgentWorkflow(api, keyStore(), () => `attempt-${++nextKey}`);
        await expect(workflow.operate("start", fixture())).rejects.toThrow("cli_missing");
        await workflow.operate("start", fixture());
        expect(api.StartAgent.mock.calls.map(([, key]) => key)).toEqual(["attempt-1", "attempt-2"]);
        expect(api.CreateAgentInstance).not.toHaveBeenCalled();
        expect(api.StartNewSession).not.toHaveBeenCalled();
    });

    it("retires a superseded request without automatically starting another conversation", async () => {
        const api = fakePort(), keys = keyStore();
        const current = { ...fixture(), version: 4, generation: 3, claudesessionid: "current-conversation" };
        api.StartNewSession.mockResolvedValueOnce({ instance: current,
            operation: { generation: 1, requestkey: "old-request", intent: "new-session", phase: "superseded", status: "superseded" }, terminalblockid: current.blockid });
        const workflow = new ForceAgentWorkflow(api, keys, () => "old-request");
        await expect(workflow.operate("new-session", current)).rejects.toThrow("operation_superseded");
        expect(api.StartNewSession).toHaveBeenCalledTimes(1);
        expect(api.StartAgent).not.toHaveBeenCalled(); expect(api.ReconnectAgent).not.toHaveBeenCalled();
        expect(api.CreateAgentInstance).not.toHaveBeenCalled();
        expect(keys.getItem("force:agent-intent:new-session:agent-1")).toBeNull();
        expect(current.claudesessionid).toBe("current-conversation");
    });
});
