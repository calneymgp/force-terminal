// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

import { ForceService } from "@/app/store/services";
import { waveEventSubscribeSingle } from "@/app/store/wps";
import { addWSReconnectHandler, removeWSReconnectHandler } from "@/app/store/ws";
import { useCallback, useEffect, useRef, useState } from "react";

export type AgentInstance = ForceAgentInstance;
export type AgentCreateInput = ForceAgentInstanceInput;
export type AgentOperationResult = ForceAgentOperationResult;

export type AgentServicePort = {
    CreateAgentInstance(input: AgentCreateInput): Promise<AgentInstance>;
    ListAgentInstances(projectID: string): Promise<AgentInstance[]>;
    StartAgent(instanceID: string, requestKey: string): Promise<AgentOperationResult>;
    ReconnectAgent(instanceID: string, requestKey: string): Promise<AgentOperationResult>;
    StartNewSession(instanceID: string, requestKey: string): Promise<AgentOperationResult>;
    FocusAgentTerminal(instanceID: string): Promise<void>;
};

type KeyStore = Pick<Storage, "getItem" | "setItem" | "removeItem">;
type AgentIntent = "start" | "reconnect" | "new-session";

const memoryKeys = new Map<string, string>();
const memoryStore: KeyStore = {
    getItem: (key) => memoryKeys.get(key) ?? null,
    setItem: (key, value) => { memoryKeys.set(key, value); },
    removeItem: (key) => { memoryKeys.delete(key); },
};

function defaultKeyStore(): KeyStore {
    try { return window.sessionStorage; } catch { return memoryStore; }
}

export function canExecuteAgent(adapter: string, connection: string): boolean {
    return adapter === "claude-code" && connection === "";
}

export function agentStatusLabel(instance: AgentInstance): string {
    if (!canExecuteAgent(instance.adapter, instance.connection)) return "Execução indisponível";
    const identity = instance.identityevidence === "observed" || instance.identityevidence === "validated" ? "Conversa confirmada" : "Conversa preparada";
    if (instance.status === "running") return `${identity} · processo ativo`;
    if (instance.status === "uncertain") return `${identity} · processo não confirmado`;
    if (instance.status === "resume_failed") return `${identity} · retomada falhou`;
    if (instance.status === "exited") return `${identity} · pronto para reconectar`;
    if (instance.operationphase === "reserved" || instance.operationphase === "launch_requested") return `${identity} · iniciando`;
    return identity;
}

export function safeAgentCause(cause: unknown): string {
    const code = cause instanceof Error ? cause.message.toLowerCase() : String(cause ?? "").toLowerCase();
    if (code.includes("cli unavailable") || code.includes("cli_missing") || code.includes("claude cli")) return "Claude Code não foi encontrado no destino.";
    if (code.includes("writer") || code.includes("lease") || code.includes("conflit")) return "Este destino já tem um agente escrevendo. Use checkouts separados.";
    if (code.includes("process_group") || code.includes("boot_evidence")) return "Ainda há processos ativos, ou seu encerramento não foi confirmado. O destino permanece reservado.";
    if (code.includes("uncertain") || code.includes("não confirmado") || code.includes("process_unconfirmed") || code.includes("process_check_failed") || code.includes("process_evidence_missing") || code.includes("live_process_without_terminal")) return "O processo anterior não foi confirmado. Verifique o terminal antes de tentar novamente.";
    if (code.includes("terminal_storage_failed")) return "A saída do terminal não pôde ser salva. Verifique o estado do agente.";
    if (code.includes("persistence_failed")) return "O estado do agente não pôde ser salvo. A execução precisa ser conferida antes de retomar.";
    if (code.includes("resume_exit_nonzero") || code.includes("resume_failed")) return "O CLI não retomou a conversa. Abra o terminal para conferir e tente novamente.";
    if (code.includes("prepare_failed")) return "Não foi possível preparar o agente no destino. Verifique o CLI e a pasta do projeto.";
    if (code.includes("operation_superseded")) return "Essa ação já foi processada. Confira o estado atual do agente antes de tentar novamente.";
    if (code.includes("remote") || code.includes("ssh")) return "Execução SSH ainda indisponível para agentes.";
    if (code.includes("codex")) return "Execução Codex ainda indisponível para agentes.";
    if (code.includes("destination") || code.includes("directory") || code.includes("cwd")) return "A pasta de execução não está disponível.";
    return "Não foi possível concluir a operação. O agente foi preservado; tente novamente depois de verificar seu estado.";
}

// This workflow owns only user-initiated calls. Listing and refresh have no
// launch side effect. Keys stay in session storage after a lost RPC response.
export class ForceAgentWorkflow {
    private inFlight = new Map<string, Promise<unknown>>();
    constructor(private api: AgentServicePort, private keys: KeyStore = defaultKeyStore(), private makeKey: () => string = () => crypto.randomUUID()) {}

    list(projectID: string): Promise<AgentInstance[]> {
        return this.api.ListAgentInstances(projectID);
    }

    private key(scope: string): string {
        const storageKey = `force:agent-intent:${scope}`;
        let key = this.keys.getItem(storageKey);
        if (!key) {
            key = this.makeKey();
            this.keys.setItem(storageKey, key);
        }
        return key;
    }

    private once<T>(scope: string, call: (key: string) => Promise<T>): Promise<T> {
        const pending = this.inFlight.get(scope);
        if (pending) return pending as Promise<T>;
        const storageKey = `force:agent-intent:${scope}`;
        const requestKey = this.key(scope);
        const task = call(requestKey).then((result) => {
            this.keys.removeItem(storageKey);
            return result;
        }).finally(() => { this.inFlight.delete(scope); });
        this.inFlight.set(scope, task);
        return task;
    }

    create(input: Omit<AgentCreateInput, "creationkey">): Promise<AgentInstance> {
        const scope = `create:${input.projectid}:${input.projectversion}:${input.profileid}:${input.profileversion}:${input.tabid}`;
        return this.once(scope, (creationkey) => this.api.CreateAgentInstance({ ...input, creationkey }));
    }

    operate(intent: AgentIntent, instance: AgentInstance): Promise<AgentOperationResult> {
        if (!canExecuteAgent(instance.adapter, instance.connection)) return Promise.reject(new Error(instance.connection ? "ssh execution unavailable" : "codex execution unavailable"));
        const scope = `${intent}:${instance.oid}`;
        return this.once(scope, (requestKey) => {
            if (intent === "start") return this.api.StartAgent(instance.oid, requestKey);
            if (intent === "reconnect") return this.api.ReconnectAgent(instance.oid, requestKey);
            return this.api.StartNewSession(instance.oid, requestKey);
        }).then((result) => {
            if (result.operation.phase === "superseded") throw new Error("operation_superseded");
            // A confirmed pre-launch failure completed this operation. Its key
            // has been cleared; an explicit retry keeps the instance/session
            // and receives a new operation key. Ambiguous RPC failures retain it.
            if (result.operation.phase === "prepare_failed" || result.operation.status === "resume_failed") {
                throw new Error(result.instance.errorcode || "prepare_failed");
            }
            return result;
        });
    }

    focus(instanceID: string): Promise<void> {
        return this.api.FocusAgentTerminal(instanceID);
    }
}

export const forceAgentWorkflow = new ForceAgentWorkflow(ForceService);

export function useForceAgentInstances(projectID: string | null) {
    const [instances, setInstances] = useState<AgentInstance[]>([]);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const requestNumber = useRef(0);
    const refresh = useCallback(async () => {
        const current = ++requestNumber.current;
        if (!projectID) {
            setInstances([]);
            setError(null);
            setLoading(false);
            return;
        }
        setLoading(true);
        try {
            const next = await forceAgentWorkflow.list(projectID);
            if (current !== requestNumber.current) return;
            setInstances(next ?? []);
            setError(null);
        } catch (cause) {
            if (current === requestNumber.current) setError(safeAgentCause(cause));
        } finally {
            if (current === requestNumber.current) setLoading(false);
        }
    }, [projectID]);

    useEffect(() => {
        void refresh();
        const unsubscribe = waveEventSubscribeSingle({ eventType: "waveobj:update", handler: (event) => {
            if (event.data?.otype === "forceagentinstance") void refresh();
        } });
        const onFocus = () => void refresh();
        const onVisible = () => { if (document.visibilityState === "visible") void refresh(); };
        addWSReconnectHandler(onFocus);
        window.addEventListener("focus", onFocus);
        document.addEventListener("visibilitychange", onVisible);
        return () => {
            requestNumber.current++;
            unsubscribe();
            removeWSReconnectHandler(onFocus);
            window.removeEventListener("focus", onFocus);
            document.removeEventListener("visibilitychange", onVisible);
        };
    }, [refresh]);
    return { instances, loading, error, refresh };
}
