# Execução do EVOLUTION.md

Objetivo integral: concluir a evolução do Force Terminal com uma experiência organizada e simples, centrada em projetos e agentes. Este registro acompanha o objetivo sem redefinir conclusão por uma entrega parcial. Fonte de requisitos: [EVOLUTION.md](EVOLUTION.md).

| Requisito | Evidência atual | Próxima prova necessária |
|---|---|---|
| 0 — ambiente funcional | Build arm64 e smoke do pacote em macOS CI; usuário confirmou instalação e abertura | Terminal, SSH, arquivos e persistência no M5 físico |
| 1a — identidade | Vetores aplicados; nome/ícone no pacote e popup instalado | Finder, Dock e demais superfícies no aparelho |
| 1b — isolamento/updater/distribuição | Código, testes e pipeline; DMG ad hoc servido no Tailscale | Coexistência/SSH/Keychain e N→N+1 com identidade estável; criação de credenciais continua fora da autorização atual |
| 2 — projetos e perfis | Catálogo SQLite, CRUD, perfis, versão otimista e criação idempotente implementados; oito checks Electron/backend Linux aprovados | Validar catálogo no novo DMG macOS e no M5; evidências em [CATALOG-REPORT.md](CATALOG-REPORT.md) |
| 3 — novo agente no projeto | Contrato CLI verificado; sem adaptador integrado | Prompt aplicado, sessão identificada, instância persistida e concorrência protegida; restauração de blocos não pode iniciar agentes implicitamente |
| 4 — reconectar | Jobs herdados reanexam somente quando vivos; não preservam conversa por si | Mesmo ID de conversa/projeto/host/diretório, nenhuma duplicata; encerramento abrupto, reboot e falhas SSH |
| 5 — tarefas e notas | Editor/Markdown existentes, sem domínio de tarefas | Estados manuais, relações persistentes e única fonte Markdown com conflito externo detectado |
| 6 — estados/atenção | Status de controller/job existem | Estado observado com fonte; filtro Precisa de você sem inferência por silêncio |
| 7 — Git/revisão | Terminal e acesso a arquivos existentes | Isolamento de escrita concorrente, worktrees/diff e links de issue/PR; ações Git explícitas |
| 8 — rotinas | Sem domínio Force | Ação manual repetível, depois agenda/log/pausa sem sobreposição |
| 9 — recuperação/orquestração | Sem domínio Force | Recuperação autorizada e handoff com contexto/efeitos preservados em falhas reais |
| Navegação e privacidade | Primeiro popup simplificado; painel de projetos/perfis; telemetria desativada por padrão no código | Entrada por agentes, ferramentas secundárias com menor destaque e validação no pacote |

## Execução em curso

Plano ativo: [.plans/force-catalog/PLAN.md](../../.plans/force-catalog/PLAN.md). Entrega a prioridade 2 como domínio e interface reais. O primeiro diferencial cotidiano ainda exige completar 2–4 juntos; o objetivo global permanece aberto até todos os requisitos aplicáveis estarem comprovados.

Não incorporar a animação ou redesenhar o onboarding agora. Manter somente o primeiro popup conforme decisão do usuário. Validar os recursos antes de voltar a apresentar um novo onboarding.
