// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import * as WOS from "@/app/store/wos";
import { ForceService } from "@/app/store/services";
import { disableGlobalKeybindings, enableGlobalKeybindings } from "@/app/store/keymodel";
import { errorMessage, setForceCatalogUpdateReason, useForceCatalog, type Catalog, type CatalogProfile, type CatalogProject } from "./force-catalog-model";
import { ForceAgentPanel } from "./force-agent-panel";
import { atoms } from "@/store/global";
import { useAtomValue } from "jotai";
import { useEffect, useId, useMemo, useRef, useState } from "react";
import "./force-catalog.scss";

type Kind = "project" | "profile";
type Draft = { name: string; icon: string; destination: "local" | "ssh"; connection: string; rootpath: string; systemprompt: string; adapter: string };
type Editor = { kind: Kind; original?: CatalogProject | CatalogProfile; initial: Draft };
type ProjectSelectionResult = { ok: true } | { ok: false; error: string };

const icons = ["bolt", "code", "database", "bullhorn", "wrench", "robot", "server", "folder"];
const iconClass: Record<string, string> = {
    bolt: "fa-bolt", code: "fa-code", database: "fa-database", bullhorn: "fa-bullhorn",
    wrench: "fa-wrench", robot: "fa-robot", server: "fa-server", folder: "fa-folder",
};
const iconLabel: Record<string, string> = {
    bolt: "Raio", code: "Código", database: "Dados", bullhorn: "Marketing",
    wrench: "Ferramentas", robot: "Agente", server: "Servidor", folder: "Pasta",
};
const presets = [
    { title: "DevOps", icon: "server", prompt: "Apoie operações e infraestrutura deste projeto. Verifique o estado atual, explique riscos e peça autorização antes de alterar serviços ou dados de produção." },
    { title: "ETL de dados", icon: "database", prompt: "Apoie pipelines de dados deste projeto. Confirme fontes, esquemas e qualidade dos dados; documente transformações e valide os resultados antes de aplicar mudanças." },
    { title: "Marketing", icon: "bullhorn", prompt: "Apoie planejamento e produção de marketing deste projeto. Identifique público, objetivo e canal; proponha textos revisáveis e peça aprovação antes de publicar." },
];

function emptyDraft(kind: Kind): Draft {
    return { name: "", icon: kind === "project" ? "folder" : "robot", destination: "local", connection: "", rootpath: "", systemprompt: "", adapter: "codex" };
}

function draftFrom(kind: Kind, item?: CatalogProject | CatalogProfile): Draft {
    const draft = emptyDraft(kind);
    if (!item) return draft;
    if (kind === "project") {
        const project = item as CatalogProject;
        return { ...draft, name: project.name, icon: project.icon, destination: project.connection ? "ssh" : "local", connection: project.connection, rootpath: project.rootpath };
    }
    const profile = item as CatalogProfile;
    return { ...draft, name: profile.title, icon: profile.icon, systemprompt: profile.systemprompt, adapter: profile.adapter };
}

function CatalogIcon({ icon }: { icon: string }) {
    return <i className={`fa-solid ${iconClass[icon] ?? "fa-folder"}`} aria-hidden="true" />;
}

function ForceEditor({ editor, onClose, onSaved }: { editor: Editor; onClose: () => void; onSaved: (savedProject?: CatalogProject) => Promise<void> }) {
    const [draft, setDraft] = useState<Draft>(editor.initial);
    const [saving, setSaving] = useState(false);
    const [savedNewProject, setSavedNewProject] = useState<CatalogProject | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [creationKey] = useState(() => crypto.randomUUID());
    const savingRef = useRef(false);
    const modalRef = useRef<HTMLElement>(null);
    const previousFocus = useRef(document.activeElement as HTMLElement);
    const formId = useId();
    const guardId = useId();
    const dirty = savedNewProject == null && JSON.stringify(draft) !== JSON.stringify(editor.initial);
    const fieldsDisabled = saving || savedNewProject != null;
    const label = editor.kind === "project" ? "projeto" : "perfil";
    const original = editor.original;

    useEffect(() => {
        disableGlobalKeybindings();
        modalRef.current?.querySelector<HTMLInputElement>("input")?.focus();
        return () => {
            enableGlobalKeybindings();
            if (previousFocus.current?.isConnected) previousFocus.current.focus();
        };
    }, []);

    useEffect(() => {
        setForceCatalogUpdateReason(guardId, saving ? `Salvamento de ${label} em andamento` : dirty ? `Formulário de ${label} com alterações não salvas` : null);
        return () => setForceCatalogUpdateReason(guardId, null);
    }, [guardId, saving, dirty, label]);

    const close = () => {
        if (savingRef.current) return;
        if (dirty && !window.confirm("Descartar alterações não salvas?")) return;
        onClose();
    };

    useEffect(() => {
        const onKeyDown = (event: KeyboardEvent) => {
            if (event.key === "Escape") { event.preventDefault(); event.stopImmediatePropagation(); close(); }
            if (event.key === "Tab") {
                const controls = Array.from(modalRef.current?.querySelectorAll<HTMLElement>("button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled)") ?? []);
                const first = controls[0], last = controls[controls.length - 1];
                if (event.shiftKey && (document.activeElement === first || !modalRef.current?.contains(document.activeElement))) {
                    event.preventDefault(); last?.focus();
                } else if (!event.shiftKey && (document.activeElement === last || !modalRef.current?.contains(document.activeElement))) {
                    event.preventDefault(); first?.focus();
                }
            }
        };
        window.addEventListener("keydown", onKeyDown, true);
        return () => window.removeEventListener("keydown", onKeyDown, true);
    });

    const set = (key: keyof Draft, value: string) => {
        setDraft((previous) => ({ ...previous, [key]: value }));
        setError(null);
    };

    const save = async (event: React.FormEvent) => {
        event.preventDefault();
        if (savingRef.current) return;
        savingRef.current = true;
        setSaving(true);
        setForceCatalogUpdateReason(guardId, `Salvamento de ${label} em andamento`);
        setError(null);
        try {
            let createdProject: CatalogProject | undefined;
            if (editor.kind === "project") {
                const projectInput = {
                    id: original?.oid ?? "", expectedversion: original?.version ?? 0,
                    creationkey: original ? "" : creationKey,
                    name: draft.name.trim(), icon: draft.icon,
                    connection: draft.destination === "ssh" ? draft.connection.trim() : "", rootpath: draft.rootpath.trim(),
                };
                if (original) {
                    await ForceService.SaveProject(projectInput);
                } else {
                    createdProject = savedNewProject ?? await ForceService.SaveProject(projectInput);
                    if (savedNewProject == null) setSavedNewProject(createdProject);
                }
            } else {
                await ForceService.SaveProfile({
                    id: original?.oid ?? "", expectedversion: original?.version ?? 0,
                    creationkey: original ? "" : creationKey,
                    title: draft.name.trim(), icon: draft.icon,
                    systemprompt: draft.systemprompt, adapter: draft.adapter,
                });
            }
            await onSaved(createdProject);
            onClose();
        } catch (err) {
            setError(errorMessage(err));
        } finally {
            savingRef.current = false;
            setSaving(false);
        }
    };

    const toggleArchive = async () => {
        if (!original || savingRef.current) return;
        if (dirty && !window.confirm("Descartar alterações e alterar o arquivamento?")) return;
        savingRef.current = true;
        setSaving(true);
        setForceCatalogUpdateReason(guardId, `Salvamento de ${label} em andamento`);
        setError(null);
        try {
            if (editor.kind === "project") await ForceService.ArchiveProject(original.oid, original.version, !original.archived);
            else await ForceService.ArchiveProfile(original.oid, original.version, !original.archived);
            await onSaved();
            onClose();
        } catch (err) {
            setError(errorMessage(err));
        } finally {
            savingRef.current = false;
            setSaving(false);
        }
    };

    return (
        <div className="force-editor-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) close(); }}>
            <section ref={modalRef} className="force-editor" role="dialog" aria-modal="true" aria-labelledby={`${formId}-title`}>
                <header className="force-editor-head">
                    <div>
                        <div className="force-eyebrow">Catálogo Force</div>
                        <h2 id={`${formId}-title`}>{original ? "Editar" : "Novo"} {label}</h2>
                    </div>
                    <button type="button" className="force-icon-button" onClick={close} disabled={saving} aria-label="Fechar editor"><i className="fa-solid fa-xmark" /></button>
                </header>
                <form onSubmit={save} className="force-editor-form" data-testid={editor.kind === "project" ? "force-project-form" : "force-profile-form"}>
                    {editor.kind === "profile" && !original && (
                        <div className="force-presets" aria-label="Preenchimentos de perfil">
                            {presets.map((preset) => <button type="button" key={preset.title} disabled={saving} onClick={() => setDraft((value) => ({ ...value, name: preset.title, icon: preset.icon, systemprompt: preset.prompt }))}>{preset.title}</button>)}
                        </div>
                    )}
                    <label htmlFor={`${formId}-name`}>{editor.kind === "project" ? "Nome do projeto" : "Título do perfil"}</label>
                    <input id={`${formId}-name`} disabled={fieldsDisabled} autoFocus required maxLength={120} value={draft.name} onChange={(event) => set("name", event.target.value)} placeholder={editor.kind === "project" ? "Ex.: Plataforma" : "Ex.: DevOps"} />
                    <label htmlFor={`${formId}-icon`}>Ícone</label>
                    <select id={`${formId}-icon`} disabled={fieldsDisabled} value={draft.icon} onChange={(event) => set("icon", event.target.value)}>{icons.map((icon) => <option key={icon} value={icon}>{iconLabel[icon]}</option>)}</select>
                    {editor.kind === "project" ? <>
                        <label htmlFor={`${formId}-destination`}>Local/SSH</label>
                        <select id={`${formId}-destination`} disabled={fieldsDisabled} value={draft.destination} onChange={(event) => set("destination", event.target.value)}>
                            <option value="local">Local</option><option value="ssh">SSH</option>
                        </select>
                        {draft.destination === "ssh" && <><label htmlFor={`${formId}-connection`}>Conexão SSH</label><input id={`${formId}-connection`} disabled={fieldsDisabled} required maxLength={1024} value={draft.connection} onChange={(event) => set("connection", event.target.value)} placeholder="usuario@host ou nome da conexão" /><p className="force-hint">Somente a referência da conexão é salva. Nenhuma conexão será aberta agora.</p></>}
                        <label htmlFor={`${formId}-path`}>Pasta raiz</label>
                        <input id={`${formId}-path`} disabled={fieldsDisabled} required maxLength={4096} value={draft.rootpath} onChange={(event) => set("rootpath", event.target.value)} placeholder={draft.destination === "ssh" ? "~/projeto ou /srv/projeto" : "/caminho/absoluto/projeto"} />
                    </> : <>
                        <label htmlFor={`${formId}-adapter`}>CLI preferido</label>
                        <select id={`${formId}-adapter`} disabled={fieldsDisabled} value={draft.adapter} onChange={(event) => set("adapter", event.target.value)}><option value="codex">Codex</option><option value="claude-code">Claude Code</option></select>
                        <p className="force-hint">Esta preferência ainda não inicia um agente.</p>
                        <label htmlFor={`${formId}-prompt`}>System prompt</label>
                        <textarea id={`${formId}-prompt`} disabled={fieldsDisabled} maxLength={32000} rows={7} value={draft.systemprompt} onChange={(event) => set("systemprompt", event.target.value)} placeholder="Descreva o papel deste perfil" />
                    </>}
                    {error && <p className="force-error" role="alert">{error} <span>Os dados do formulário foram preservados. Reabra o registro se houve alteração em outra janela.</span></p>}
                    <div className="force-editor-actions">
                        {original && <button type="button" className="force-text-button" onClick={toggleArchive} disabled={saving}>{original.archived ? "Reativar" : "Arquivar"}</button>}
                        <span className="force-actions-spacer" />
                        <button type="button" className="force-secondary-button" onClick={close} disabled={saving}>{savedNewProject ? "Fechar" : "Cancelar"}</button>
                        <button type="submit" className="force-primary-button" disabled={saving}>{saving ? "Salvando…" : savedNewProject ? "Tentar selecionar" : editor.kind === "project" ? "Salvar projeto" : "Salvar perfil"}</button>
                    </div>
                </form>
            </section>
        </div>
    );
}

export function ForceSidebar({ workspace, initialCatalog }: { workspace: Workspace; initialCatalog?: Catalog }) {
    const tabID = useAtomValue(atoms.staticTabId);
    const { catalog, loading, error, refresh } = useForceCatalog(initialCatalog);
    const [editor, setEditor] = useState<Editor | null>(null);
    const [showArchived, setShowArchived] = useState(false);
    const [selectionError, setSelectionError] = useState<string | null>(null);
    const selectingRef = useRef(false);
    const [selectedId, setSelectedId] = useState<string>(() => String(workspace.meta?.["force:projectid"] ?? ""));

    useEffect(() => { setSelectedId(String(workspace.meta?.["force:projectid"] ?? "")); }, [workspace.oid, workspace.meta?.["force:projectid"]]);

    const projects = useMemo(() => catalog.projects.filter((item) => showArchived || !item.archived), [catalog.projects, showArchived]);
    const profiles = useMemo(() => catalog.profiles.filter((item) => showArchived || !item.archived), [catalog.profiles, showArchived]);
    const selected = catalog.projects.find((item) => item.oid === selectedId && !item.archived);

    const select = async (id: string, allowEditor = false): Promise<ProjectSelectionResult> => {
        if (editor && !allowEditor) return { ok: false, error: "Feche o editor antes de selecionar outro projeto." };
        if (selectingRef.current) return { ok: false, error: "Outra seleção já está em andamento." };
        selectingRef.current = true;
        setSelectionError(null);
        try {
            await RpcApi.SetMetaCommand(TabRpcClient, { oref: WOS.makeORef("workspace", workspace.oid), meta: { "force:projectid": id } as MetaType });
            setSelectedId(id);
            return { ok: true };
        } catch (err) {
            const selectionError = errorMessage(err);
            setSelectionError(`A seleção não foi salva: ${selectionError}`);
            return { ok: false, error: selectionError };
        }
        finally { selectingRef.current = false; }
    };

    const refreshAfterSave = async (savedProject?: CatalogProject) => {
        await refresh();
        if (!savedProject) return;
        const result = await select(savedProject.oid, true);
        if (result.ok === false) throw new Error(`Projeto salvo, mas a seleção falhou: ${result.error}`);
    };

    const open = (kind: Kind, original?: CatalogProject | CatalogProfile) => setEditor({ kind, original, initial: draftFrom(kind, original) });

    return <aside className="force-sidebar" aria-label="Projetos e perfis Force" data-testid="force-sidebar">
        <header className="force-sidebar-head"><span className="force-eyebrow">FORCE TERMINAL</span><strong>Seu trabalho</strong></header>
        {error && <div className="force-sidebar-error" role="alert">Não foi possível carregar o catálogo. {error}<button type="button" onClick={() => void refresh()}>Tentar novamente</button></div>}
        {selectionError && <div className="force-sidebar-error" role="alert">{selectionError}</div>}
        <div className="force-sidebar-scroll">
            <section aria-labelledby="force-projects-title">
                <div className="force-section-head"><h2 id="force-projects-title">Projetos</h2><button type="button" className="force-icon-button" onClick={() => open("project")} aria-label="Novo projeto" title="Novo projeto"><i className="fa-solid fa-plus" /></button></div>
                {loading ? <p className="force-empty">Carregando projetos…</p> : projects.length === 0 ? <p className="force-empty">Adicione uma pasta local ou um destino SSH para começar.</p> : <ul className="force-list">{projects.map((project) => <li key={project.oid} className={selected?.oid === project.oid ? "is-selected" : ""}>
                    <button type="button" className="force-item-main" onClick={() => void select(project.oid)} aria-current={selected?.oid === project.oid ? "true" : undefined}>
                        <span className="force-item-icon"><CatalogIcon icon={project.icon} /></span><span className="force-item-copy"><strong>{project.name}</strong><small title={`${project.connection || "Local"} · ${project.rootpath}`}>{project.connection ? `SSH · ${project.connection}` : "Local"} · {project.rootpath}</small></span>{project.archived && <span className="force-tag">Arquivo</span>}
                    </button><button type="button" className="force-item-edit" onClick={() => open("project", project)} aria-label={`Editar projeto ${project.name}`} title="Editar projeto"><i className="fa-solid fa-ellipsis" /></button>
                </li>)}</ul>}
            </section>
            {selected && <div className="force-selected-note"><strong>{selected.name}</strong><span>{selected.connection ? `SSH: ${selected.connection}` : "Pasta local"}</span><code>{selected.rootpath}</code><small>Os terminais e arquivos existentes continuam disponíveis ao lado.</small></div>}
            {selected && !initialCatalog && <ForceAgentPanel key={selected.oid} project={selected} profiles={profiles.filter((profile) => !profile.archived)} tabID={tabID} />}
            <section aria-labelledby="force-profiles-title">
                <div className="force-section-head"><h2 id="force-profiles-title">Perfis</h2><button type="button" className="force-icon-button" onClick={() => open("profile")} aria-label="Novo perfil" title="Novo perfil"><i className="fa-solid fa-plus" /></button></div>
                {loading ? <p className="force-empty">Carregando perfis…</p> : profiles.length === 0 ? <p className="force-empty">Cadastre papéis para reutilizar entre projetos.</p> : <ul className="force-list">{profiles.map((profile) => <li key={profile.oid}>
                    <button type="button" className="force-item-main" onClick={() => open("profile", profile)}><span className="force-item-icon"><CatalogIcon icon={profile.icon} /></span><span className="force-item-copy"><strong>{profile.title}</strong><small>{profile.adapter === "claude-code" ? "Claude Code" : "Codex"} · perfil cadastrado</small></span>{profile.archived && <span className="force-tag">Arquivo</span>}</button>
                </li>)}</ul>}
            </section>
            <button type="button" className="force-archive-toggle" onClick={() => setShowArchived((value) => !value)}>{showArchived ? "Ocultar arquivados" : "Mostrar arquivados"}</button>
        </div>
        {editor && <ForceEditor key={`${editor.kind}:${editor.original?.oid ?? "new"}`} editor={editor} onClose={() => setEditor(null)} onSaved={refreshAfterSave} />}
    </aside>;
}
