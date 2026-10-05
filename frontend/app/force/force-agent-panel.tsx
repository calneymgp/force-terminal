// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

import { disableGlobalKeybindings, enableGlobalKeybindings } from "@/app/store/keymodel";
import { useEffect, useId, useRef, useState } from "react";
import { setForceCatalogUpdateReason, type CatalogProfile, type CatalogProject } from "./force-catalog-model";
import { agentStatusLabel, canExecuteAgent, forceAgentWorkflow, safeAgentCause, useForceAgentInstances, type AgentInstance } from "./force-agent-model";

function NewAgentDialog({ project, profiles, tabID, onClose, onCreated }: {
    project: CatalogProject; profiles: CatalogProfile[]; tabID: string; onClose: () => void; onCreated: (profile: CatalogProfile) => Promise<void>;
}) {
    const [profileID, setProfileID] = useState(profiles[0]?.oid ?? "");
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const savingRef = useRef(false);
    const modalRef = useRef<HTMLElement>(null);
    const previousFocus = useRef(document.activeElement as HTMLElement);
    const formID = useId();
    const guardID = useId();
    const profile = profiles.find((item) => item.oid === profileID);
    const executable = profile ? canExecuteAgent(profile.adapter, project.connection) : false;

    useEffect(() => {
        disableGlobalKeybindings();
        modalRef.current?.querySelector<HTMLSelectElement>("select")?.focus();
        return () => {
            enableGlobalKeybindings();
            if (previousFocus.current?.isConnected) previousFocus.current.focus();
        };
    }, []);

    useEffect(() => {
        setForceCatalogUpdateReason(guardID, saving ? "Criação de agente em andamento" : `Novo agente selecionado para ${project.name}`);
        return () => setForceCatalogUpdateReason(guardID, null);
    }, [guardID, saving, project.name]);

    const close = () => { if (!savingRef.current) onClose(); };
    useEffect(() => {
        const onKeyDown = (event: KeyboardEvent) => {
            if (event.key === "Escape") { event.preventDefault(); event.stopImmediatePropagation(); close(); }
            if (event.key !== "Tab") return;
            const controls = Array.from(modalRef.current?.querySelectorAll<HTMLElement>("button:not(:disabled), select:not(:disabled)") ?? []);
            const first = controls[0], last = controls[controls.length - 1];
            if (event.shiftKey && (document.activeElement === first || !modalRef.current?.contains(document.activeElement))) {
                event.preventDefault(); last?.focus();
            } else if (!event.shiftKey && (document.activeElement === last || !modalRef.current?.contains(document.activeElement))) {
                event.preventDefault(); first?.focus();
            }
        };
        window.addEventListener("keydown", onKeyDown, true);
        return () => window.removeEventListener("keydown", onKeyDown, true);
    });

    const submit = async (event: React.FormEvent) => {
        event.preventDefault();
        if (!profile || !tabID || savingRef.current) return;
        savingRef.current = true;
        setSaving(true);
        setError(null);
        try { await onCreated(profile); onClose(); }
        catch (cause) { setError(safeAgentCause(cause)); }
        finally { savingRef.current = false; setSaving(false); }
    };

    return <div className="force-editor-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) close(); }}>
        <section ref={modalRef} className="force-editor" role="dialog" aria-modal="true" aria-labelledby={`${formID}-title`}>
            <header className="force-editor-head"><div><div className="force-eyebrow">{project.name}</div><h2 id={`${formID}-title`}>Novo agente</h2></div>
                <button type="button" className="force-icon-button" onClick={close} disabled={saving} aria-label="Fechar novo agente"><i className="fa-solid fa-xmark" /></button></header>
            <form onSubmit={submit} className="force-editor-form" data-testid="force-new-agent-form">
                <label htmlFor={`${formID}-profile`}>Perfil</label>
                <select id={`${formID}-profile`} required disabled={saving || profiles.length === 0} value={profileID} onChange={(event) => { setProfileID(event.target.value); setError(null); }}>
                    {profiles.map((item) => <option value={item.oid} key={item.oid}>{item.title} · {item.adapter === "claude-code" ? "Claude Code" : "Codex"}</option>)}
                </select>
                {profiles.length === 0 && <p className="force-hint">Cadastre um perfil antes de criar um agente.</p>}
                <p className="force-hint">O agente preserva este perfil e o terminal neste projeto. Nenhuma tarefa inicial é obrigatória.</p>
                {!executable && profile && <p className="force-agent-unavailable">{project.connection ? "A execução SSH ainda não está disponível. O agente será criado para abrir depois." : "A execução Codex ainda não está disponível. O agente será criado para abrir depois."}</p>}
                {error && <p className="force-error" role="alert">{error}</p>}
                <div className="force-editor-actions"><span className="force-actions-spacer" />
                    <button type="button" className="force-secondary-button" disabled={saving} onClick={close}>Cancelar</button>
                    <button type="submit" className="force-primary-button" disabled={saving || !profile || !tabID}>{saving ? "Preparando…" : executable ? "Criar e iniciar agente" : "Criar agente"}</button>
                </div>
            </form>
        </section>
    </div>;
}

function AgentDetails({ instance }: { instance: AgentInstance }) {
    return <details className="force-agent-details"><summary>Detalhes</summary><dl>
        <dt>Perfil salvo</dt><dd>Versão {instance.profileversion}</dd>
        <dt>Destino salvo</dt><dd>{instance.connection || "Local"}</dd>
        <dt>Pasta de retomada</dt><dd>{instance.executionroot || instance.rootpath}</dd>
        <dt>Local preservado</dt><dd>Raiz do projeto. O diretório atual dentro do CLI ainda não é acompanhado.</dd>
        <dt>Instância</dt><dd><code>{instance.oid}</code></dd>
        {instance.claudesessionid && <><dt>Conversa solicitada</dt><dd><code>{instance.claudesessionid}</code></dd></>}
    </dl></details>;
}

const agentIconClass: Record<string, string> = {
    bolt: "fa-bolt", code: "fa-code", database: "fa-database", bullhorn: "fa-bullhorn",
    wrench: "fa-wrench", robot: "fa-robot", server: "fa-server", folder: "fa-folder",
};

export function ForceAgentPanel({ project, profiles, tabID }: { project: CatalogProject; profiles: CatalogProfile[]; tabID: string }) {
    const { instances, loading, error: listError, refresh } = useForceAgentInstances(project.oid);
    const [creating, setCreating] = useState(false);
    const [busy, setBusy] = useState<string | null>(null);
    const [error, setError] = useState<string | null>(null);
    const busyRef = useRef(false);
    const operationGuard = useRef(`agent-operation:${crypto.randomUUID()}`);

    const run = async (label: string, action: () => Promise<void>): Promise<boolean> => {
        if (busyRef.current) return false;
        busyRef.current = true;
        setBusy(label);
        setForceCatalogUpdateReason(operationGuard.current, "Operação de agente em andamento");
        setError(null);
        try { await action(); return true; }
        catch (cause) { setError(safeAgentCause(cause)); await refresh(); return false; }
        finally { setForceCatalogUpdateReason(operationGuard.current, null); busyRef.current = false; setBusy(null); }
    };

    const create = async (profile: CatalogProfile) => {
        let created = false;
        await run("new", async () => {
            const instance = await forceAgentWorkflow.create({ projectid: project.oid, projectversion: project.version,
                profileid: profile.oid, profileversion: profile.version, tabid: tabID });
            created = true;
            await refresh();
            if (canExecuteAgent(profile.adapter, project.connection)) {
                await forceAgentWorkflow.operate("start", instance);
                await refresh();
                await forceAgentWorkflow.focus(instance.oid);
            }
        });
        // A failed launch still leaves a durable, inert instance in the list.
        if (created) setCreating(false);
        else throw new Error("agent creation failed");
    };

    const operate = (intent: "start" | "reconnect" | "new-session", instance: AgentInstance) => {
        if (intent === "new-session" && !window.confirm("Iniciar uma nova conversa para este agente? O contexto da conversa atual ficará no histórico.")) return;
        void run(instance.oid, async () => {
            await forceAgentWorkflow.operate(intent, instance);
            await refresh();
            await forceAgentWorkflow.focus(instance.oid);
        });
    };

    const focus = (instance: AgentInstance) => void run(instance.oid, () => forceAgentWorkflow.focus(instance.oid));

    return <section aria-labelledby="force-agents-title" className="force-agent-section">
        <div className="force-section-head"><h2 id="force-agents-title">Agentes</h2><button type="button" className="force-new-agent-button force-primary-button" onClick={() => setCreating(true)} disabled={!!busy || !tabID} aria-label="Novo agente" title={tabID ? "Novo agente" : "Abra uma aba para criar um agente"}><i className="fa-solid fa-plus" aria-hidden="true" /> Novo agente</button></div>
        {listError && <p className="force-sidebar-error" role="alert">{listError} <button type="button" onClick={() => void refresh()}>Tentar novamente</button></p>}
        {error && <p className="force-sidebar-error" role="alert">{error}</p>}
        {loading && instances.length === 0 ? <p className="force-empty">Carregando agentes…</p> : instances.length === 0 ? <p className="force-empty">Crie um agente para este projeto.</p> :
            <ul className="force-agent-list">{instances.map((instance) => {
                const executable = canExecuteAgent(instance.adapter, instance.connection);
                const pending = instance.status !== "uncertain" && (instance.operationphase === "launch_requested" || instance.operationphase === "reserved");
                const operationBlocked = !!busy || pending;
                const newSessionBlocked = operationBlocked || instance.status === "running" || instance.status === "uncertain";
                return <li key={instance.oid}>
                    <strong><i className={`fa-solid ${agentIconClass[instance.iconsnapshot] ?? "fa-robot"}`} aria-hidden="true" /> {instance.titlesnapshot}</strong>
                    <span className="force-agent-status">{agentStatusLabel(instance)}</span>
                    {instance.errorcode && <small className="force-agent-cause">{safeAgentCause(instance.errorcode)}</small>}
                    <div className="force-agent-actions">
                        <button type="button" disabled={!!busy} onClick={() => focus(instance)}>Abrir terminal</button>
                        {executable && <>
                            {!instance.currentsessionlaunched && <button type="button" disabled={operationBlocked} onClick={() => operate("start", instance)}>Iniciar agente</button>}
                            {instance.currentsessionlaunched && <button type="button" disabled={operationBlocked} onClick={() => operate("reconnect", instance)}>Reconectar</button>}
                            {instance.waslaunched && <button type="button" disabled={newSessionBlocked} onClick={() => operate("new-session", instance)}>Nova sessão</button>}
                        </>}
                    </div>
                    <AgentDetails instance={instance} />
                </li>;
            })}</ul>}
        {busy && <p className="force-hint" role="status">{busy === "new" ? "Preparando agente…" : "Atualizando agente…"}</p>}
        {creating && <NewAgentDialog project={project} profiles={profiles} tabID={tabID} onClose={() => setCreating(false)} onCreated={create} />}
    </section>;
}
