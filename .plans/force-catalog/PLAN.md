# Catálogo Agent-first — plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development ou dev-coding para executar as tarefas com revisão do coordenador. Passos usam checkboxes para acompanhar evidências.

**Goal:** entregar o cadastro persistente de projetos locais/SSH e perfis reutilizáveis de agentes, correspondente à prioridade 2 de EVOLUTION.md.

**Architecture:** usar novos objetos no SQLite Force existente e o serviço HTTP/RPC gerado do aplicativo. A interface adiciona um painel de projetos e um editor de perfis, preservando terminal, blocos, abas, SSH e o primeiro popup aprovado. Instâncias, execução e retomada são entregas seguintes; o objetivo global permanece executar as prioridades do EVOLUTION.md.

**Tech Stack:** Go, SQLite/wstore, waveobj/wps, React, Jotai e bindings TypeScript gerados.

**Spec:** `docs/force-terminal/EVOLUTION.md`, direção Agent-first, prioridade 2, navegação e invariantes de persistência.

## Global Constraints

- Não acessar nem importar dados ou credenciais Wave.
- Não mudar os vetores aprovados nem o primeiro popup; não incorporar a animação.
- Projetos locais apontam para pasta; projetos SSH referenciam a conexão e a raiz remota, sem copiar credenciais.
- Perfil é reutilizável, distinto da futura instância; edição tem controle de versão.
- Preservar arquivos concorrentes/untracked, em especial `assets/force-agent/`.
- Não iniciar CLI, SSH, tarefa paga, assinatura, release ou processo externo como efeito de salvar um cadastro.
- Arquivar não apaga entidades nem terminais. Cadastros sobrevivem ao fechamento e ao reinício do backend.
- Erros de validação/conflito devem permanecer visíveis, sem descartar o formulário.
- Formulários alterados ou salvamentos em curso bloqueiam o reinício do updater.
- O objetivo completo 0–9 continua acompanhado em `docs/force-terminal/EVOLUTION-EXECUTION.md`; esta fatia não substitui seu aceite.

### Task 1: Domínio persistente e serviço do catálogo

**Files:**
- Create: `pkg/waveobj/forcecatalog.go`
- Modify: `pkg/waveobj/wtype.go`, `pkg/service/service.go`
- Create: `pkg/service/forceservice/forceservice.go`, `pkg/service/forceservice/forceservice_test.go`
- Create: próxima migração `db/migrations-wstore/*_force_catalog.up.sql` e `.down.sql`

**Interfaces:**
- `ForceProject`: `oid`, `version`, `meta`, `name`, `icon`, `connection`, `rootpath`, `archived`, `createdat`, `updatedat`.
- `ForceAgentProfile`: `oid`, `version`, `meta`, `title`, `icon`, `systemprompt`, `adapter`, `archived`, `createdat`, `updatedat`.
- OTypes `forceproject` e `forceagentprofile`, tipos registrados pela infraestrutura existente.
- Serviço `force`, implementado por `ForceService`.
- `GetCatalog(ctx context.Context) (*ForceCatalog, error)` retorna `{projects: ForceProject[], profiles: ForceAgentProfile[]}`, incluindo arquivados para preservar referências.
- `SaveProject(ctx context.Context, input ForceProjectInput) (*waveobj.ForceProject, waveobj.UpdatesRtnType, error)`.
- `SaveProfile(ctx context.Context, input ForceProfileInput) (*waveobj.ForceAgentProfile, waveobj.UpdatesRtnType, error)`.
- `ArchiveProject(ctx context.Context, id string, expectedVersion int, archived bool) (waveobj.UpdatesRtnType, error)` e equivalente `ArchiveProfile`.
- Inputs contêm campos editáveis e `id` opcional/`expectedversion` e `creationkey` estável por tentativa de criação; criação exige ID vazio e versão 0, edição exige ID existente e sua versão atual. Estado/timestamps/versão são controlados no backend.
- `connection=""` representa local; SSH guarda nome/target válido, sem senha/URI ou WSL. Raiz local deve existir e ser diretório; raiz SSH aceita caminho absoluto ou `~/...`, sem abrir conexão ao salvar.
- Título/nome: 1–120 caracteres; prompt: máximo 32000 caracteres; ícone pertence a `bolt, code, database, bullhorn, wrench, robot, server, folder`; adaptador pertence a `codex, claude-code`, apenas preferência de cadastro neste passo.

- [x] Ler wstore/objetos/serviços e cadeia AGENTS antes de editar.
- [x] Escrever e executar um teste de cadastro persistente (três projetos e perfis) através do serviço real e SQLite temporário, confirmando falha inicial por implementação ausente.
- [x] Implementar objetos/tabelas/serviço, validação de fronteira, conflitos otimistas e eventos após commit.
- [x] Testar reabertura do DB, edição conflitante sem sobrescrita, arquivar/reabrir sem perda e entradas inválidas sem criar registros.
- [x] Executar `go test -mod=readonly ./pkg/service/forceservice ./pkg/waveobj ./pkg/wstore` e revisar o diff.

### Task 2: Bindings reproduzíveis

**Files:** `frontend/types/gotypes.d.ts`, `frontend/app/store/services.ts` e saídas adicionais realmente produzidas pelos geradores existentes.

**Interfaces:** consumidores usam `ForceService.GetCatalog()`, `SaveProject(input)`, `SaveProfile(input)`, `ArchiveProject(id, version, archived)` e `ArchiveProfile(id, version, archived)`, com tipos da Task 1.

- [x] Rodar `go run -mod=readonly cmd/generatets/main-generatets.go`.
- [x] Conferir OTypes no contrato `WaveObj` e bindings de métodos/args.
- [x] Repetir gerador e confirmar que a segunda execução não altera os arquivos.
- [x] Executar TypeScript completo e ajustar apenas mocks afetados pelo novo contrato.

### Task 3: Painel de projetos e biblioteca de perfis

**Files:**
- Create: `frontend/app/force/force-catalog-model.ts`, `force-sidebar.tsx`, `force-catalog.scss`.
- Modify: `frontend/app/workspace/workspace.tsx`, `workspace-layout-model.ts`, `frontend/wave.ts`.
- Create: preview do painel com cadastros fictícios, usando mecanismo de preview existente.

**Interfaces:** carregar catálogo pelo serviço, acompanhar `waveobj:update` e recarregar após reconexão/foco sem executar agentes. UI mostra projetos/destino e perfis/título/ícone/CLI; escolhe projeto sem misturar suas configurações. CRUD usa versão exibida, bloqueia envio repetido e mantém erro/formulário quando o servidor rejeita. Salvar preferências de seleção no metadado de workspace é permitido; cadastros residem exclusivamente no backend.

- [x] Apresentar lista de projetos, ação Novo projeto, painel de perfis e estado vazio claro.
- [x] Implementar criação/edição local e SSH, biblioteca reutilizável e arquivo/reativação explícitos.
- [x] Mostrar destino local/SSH e raiz do projeto; oferecer perfis DevOps/ETL/Marketing como preenchimentos do formulário, sem criar cadastros ocultos.
- [x] Integrar formulários ao preparo de atualização com motivo explícito para alterações pendentes/salvamento em curso.
- [x] Verificar navegação/teclado, erro de validação/conflito, cliques repetidos e atualização de outra janela.
- [x] Executar `npx tsc --noEmit`, build frontend e verificar o painel real nas larguras 1440 e 900.

### Task 4: Verificação de catálogo e registro de aceites

**Files:** smoke isolado do catálogo e atualização de `docs/force-terminal/EVOLUTION.md`, `EVOLUTION-EXECUTION.md`.

- [x] Exercitar backend e renderer reais em perfil temporário: criar três projetos e três perfis, editar, arquivar/reativar, fechar e reabrir.
- [x] Confirmar ausência de processos CLI/SSH iniciados pelo cadastro, importação de registros Wave ou cópia de credenciais pelo cadastro; conferir os diretórios Force temporários.
- [x] Conferir persistência, IDs/versões e recarga entre views; não usar teste unitário do armazenamento para alegar validação da UI.
- [x] Registrar prova disponível e aceites físicos ainda pendentes, sem declarar prioridades 3–4 concluídas.
- [ ] Revisar/commitar somente arquivos da fatia e continuar para a integração CLI/retomada.

## Status Log

- 2026-10-04: plano derivado do EVOLUTION.md autorizado; auditoria confirma ausência de domínios Agent-first. Prioridades 0/1a/1b possuem provas parciais e aceites externos pendentes. Implementação do catálogo inicia sem depender de assinatura.

- 2026-10-04: domínio, UI e contratos implementados. Backend sem cache, TypeScript, build e oito checks reais passaram; duas janelas, teclado, layouts, retry idempotente e reabertura comprovados. Teste macOS do novo pacote ainda pendente.
