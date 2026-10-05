# Execução do EVOLUTION.md

Objetivo integral: concluir a evolução do Force Terminal com uma experiência organizada e simples, centrada em projetos e agentes. Este registro acompanha o objetivo sem redefinir conclusão por uma entrega parcial. Fonte de requisitos: [EVOLUTION.md](EVOLUTION.md).

| Requisito | Evidência atual | Próxima prova necessária |
|---|---|---|
| 0 — ambiente funcional | Build arm64 e smoke do pacote em macOS CI; usuário confirmou instalação e abertura | Terminal, SSH, arquivos e persistência no M5 físico |
| 1a — identidade | Vetores aplicados; nome/ícone no pacote e popup instalado | Finder, Dock e demais superfícies no aparelho |
| 1b — isolamento/updater/distribuição | Código, testes e pipeline; DMG ad hoc servido no Tailscale | Coexistência/SSH/Keychain e N→N+1 com identidade estável; criação de credenciais continua fora da autorização atual |
| 2 — projetos e perfis | Catálogo SQLite, CRUD, perfis, versão otimista e criação idempotente; oito checks reais Linux e oito no DMG macOS; instalador entregue | Conferir catálogo no M5; evidências em [CATALOG-REPORT.md](CATALOG-REPORT.md) |
| 3 — novo agente no projeto | Runtime Claude local em validação: instância e bloco estáveis, snapshot, UUID solicitado, criação explícita e reserva de escrita; walkthrough com CLI fictício | Pacote macOS, conversa real autorizada e adaptador SSH; um UUID solicitado não prova identidade confirmada pelo fornecedor |
| 4 — reconectar | Retomada local pede o UUID exato e preserva bloco/diretório/perfil; replay de ações e restauração não iniciam duplicatas; walkthrough local fictício aprovado | Reboot físico, contexto real da conversa e reanexação/retomada SSH; processos filhos/incertos devem manter a reserva |
| 5 — tarefas e notas | Editor/Markdown existentes, sem domínio de tarefas | Estados manuais, relações persistentes e única fonte Markdown com conflito externo detectado |
| 6 — estados/atenção | Status de controller/job existem | Estado observado com fonte; filtro Precisa de você sem inferência por silêncio |
| 7 — Git/revisão | Terminal e acesso a arquivos existentes | Isolamento de escrita concorrente, worktrees/diff e links de issue/PR; ações Git explícitas |
| 8 — rotinas | Sem domínio Force | Ação manual repetível, depois agenda/log/pausa sem sobreposição |
| 9 — recuperação/orquestração | Sem domínio Force | Recuperação autorizada e handoff com contexto/efeitos preservados em falhas reais |
| Navegação e privacidade | Primeiro popup e painel de projetos/perfis validados no pacote; novos perfis com telemetria desativada por padrão | Entrada por agentes e ferramentas secundárias com menor destaque |

## Execução em curso

Catálogo implementado conforme [.plans/force-catalog/PLAN.md](../../.plans/force-catalog/PLAN.md), com pacote de teste entregue. O plano [.plans/force-agent-runtime/PLAN.md](../../.plans/force-agent-runtime/PLAN.md) está em execução para as prioridades 3–4: caminho local implementado e testado com CLI fictício, integração SSH e aceites externos abertos. O [relatório do runtime](AGENT-RUNTIME-REPORT.md) separa essas evidências. Nenhuma chamada paga é autorizada pelo plano. O primeiro diferencial cotidiano ainda exige completar 2–4 juntos; o objetivo global permanece aberto até todos os requisitos aplicáveis estarem comprovados.

Não incorporar a animação ou redesenhar o onboarding agora. Manter somente o primeiro popup conforme decisão do usuário. Validar os recursos antes de voltar a apresentar um novo onboarding.
