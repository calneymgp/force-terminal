// Copyright 2026, Force Terminal contributors.
// SPDX-License-Identifier: Apache-2.0

import { ForceSidebar } from "@/app/force/force-sidebar";

const previewCatalog: ForceCatalog = {
    projects: [
        { otype: "forceproject", oid: "project-devops", version: 2, meta: {}, name: "Infraestrutura", icon: "server", connection: "", rootpath: "/Users/ana/projetos/infra", archived: false, createdat: 1, updatedat: 2 },
        { otype: "forceproject", oid: "project-data", version: 1, meta: {}, name: "Pipelines de dados", icon: "database", connection: "etl@dados.example", rootpath: "~/etl", archived: false, createdat: 1, updatedat: 2 },
        { otype: "forceproject", oid: "project-brand", version: 1, meta: {}, name: "Campanha de outubro", icon: "bullhorn", connection: "", rootpath: "/Users/ana/projetos/campanha", archived: false, createdat: 1, updatedat: 2 },
    ],
    profiles: [
        { otype: "forceagentprofile", oid: "profile-devops", version: 1, meta: {}, title: "DevOps", icon: "server", systemprompt: "Apoie operações e infraestrutura.", adapter: "codex", archived: false, createdat: 1, updatedat: 2 },
        { otype: "forceagentprofile", oid: "profile-etl", version: 1, meta: {}, title: "ETL de dados", icon: "database", systemprompt: "Apoie pipelines de dados.", adapter: "claude-code", archived: false, createdat: 1, updatedat: 2 },
        { otype: "forceagentprofile", oid: "profile-marketing", version: 1, meta: {}, title: "Marketing", icon: "bullhorn", systemprompt: "Apoie planejamento de marketing.", adapter: "codex", archived: false, createdat: 1, updatedat: 2 },
    ],
};

export default function ForceCatalogPreview() {
    return <div style={{ width: "min(100%, 1440px)", minHeight: 740, display: "flex", background: "#171719", border: "1px solid #444" }}>
        <ForceSidebar workspace={{ oid: "preview-workspace", meta: { "force:projectid": "project-devops" } } as unknown as Workspace} initialCatalog={previewCatalog} />
        <div style={{ flex: 1, padding: 28, color: "#e4e4e7" }}>
            <div style={{ fontSize: 12, color: "#a1a1aa" }}>TERMINAL / ARQUIVOS</div>
            <p style={{ marginTop: 16 }}>As ferramentas existentes continuam neste espaço.</p>
        </div>
    </div>;
}
