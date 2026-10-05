# Runtime Agent-first — implementação local

Estado: implementação local entregue no DMG de teste, com CLI fictício validado no aplicativo completo Linux/macOS. Este relatório não encerra as prioridades 3–4 do EVOLUTION.md: conversa real, reboot físico e execução SSH continuam pendentes.

## Comportamento disponível no código

- **Novo agente** cria uma instância no projeto escolhido, com snapshot imutável do perfil, destino, raiz, versão do adaptador e terminal lógico. Nenhuma tarefa é exigida. Salvar o catálogo ou restaurar um bloco não executa um CLI.
- Claude local inicia em PTY por argv validado e arquivo privado de prompt (diretório 0700, arquivo 0600), sem comando shell interpolado. O UUID é persistido antes do lançamento. A descoberta usa PATH e, quando ausente, a instalação nativa `~/.local/bin/claude`; no macOS considera também os caminhos Homebrew usuais, sem carregar scripts de login.
- **Reconectar agente** reutiliza uma execução local confirmada ou solicita `--resume` com o UUID exato depois de confirmar ausência da anterior. Mantém instância, bloco, destino e perfil congelado. Falhas não escolhem a última conversa nem criam outra silenciosamente.
- **Nova sessão** é uma escolha separada e explícita. Recibos persistentes guardam as operações aceitas: repetir uma ação antiga não pode gerar outra sessão/processo.
- A reserva de escrita protege o destino canônico e caminhos sobrepostos; agentes de código concorrentes precisam de checkouts distintos. Nesta etapa, cada instância usa a raiz do seu projeto; cadastrar um checkout separado como outro projeto permite a concorrência enquanto a seleção de worktree por agente permanece na prioridade 7. Um perfil não é uma permissão de filesystem.
- Metadados, fechamento e reinício genéricos não podem substituir ou remover o terminal de um agente. Navegar até ele recupera o bloco original. O updater considera também reservas/execuções persistidas sem controller carregado.
- A saída do CLI não libera a reserva enquanto seu grupo de processos estiver presente ou incerto. Ausência confirmada pelo kernel ou mudança de boot ID permite reconciliar; o Force nunca sinaliza um PGID antigo. O prompt privado é removido após o encerramento confirmado no mesmo processo do aplicativo.

## Evidências disponíveis

Testes Go cobrem criação idempotente e reabertura do armazenamento, snapshots, reservas, falhas de preparação, repetição histórica de operações, migração 13→14, proteção dos caminhos genéricos e bloqueio de atualização. A prova com CLI fictício saindo antes do filho confirma reserva retida, conflito para o segundo escritor e limpeza do prompt somente após o grupo desaparecer. Testes de interface verificam ausência de início implícito, operações coordenadas e falha de reconexão sem fallback.

Verificação em 5 de outubro de 2026:

| Verificação | Resultado |
|---|---|
| Go (Linux): `wstore`, `wcore`, `blockcontroller`, `shellexec`, `updateguard`, `cmd/server`, Force/Object/Workspace services e `wshserver` | Aprovado |
| `-race` (Linux): Force/Object services, `blockcontroller`, `shellexec`, `wcore` e `wshserver` | Aprovado após isolar as fixtures do gravador periódico |
| `filestore`, Force service e controller depois da alteração das fixtures | Aprovado; inicialização normal mantém o gravador periódico |
| Geração TS/Go/schema e `npx tsc --noEmit` | Aprovado |
| Seis arquivos Vitest de perfil, updater, guardas e ações de agentes | 40 testes aprovados |
| `npm run force:release:test` | 14 testes aprovados; nenhuma publicação realizada |
| Build Electron de produção e backend `osusergo,sqlite_omit_load_extension` | Aprovado |
| `force-agent-smoke.mjs` no Linux, aplicativo completo com CLI fictício | 9/9 fluxos aprovados; relatório em `/tmp/force-agent-smoke-reviewed-20261005/report.json` |
| Regressão `force-catalog-smoke.mjs` no Linux | 8/8 fluxos aprovados com o painel de agentes integrado |
| DMG macOS arm64: build [37259821901](https://github.com/calneymgp/force-terminal/actions/runs/37259821901), fonte `5e0922a443f439b8cc936b787790d577d484d31c` | Aprovado; testes Go, TypeScript, geração e validadores passaram antes do pacote |
| App extraído do DMG: smoke [37260967299](https://github.com/calneymgp/force-terminal/actions/runs/37260967299) | 7/7 fluxos essenciais, 8/8 do catálogo e 9/9 de agentes locais com CLI fictício; perfil descartável no runner macOS arm64 |

O primeiro build macOS desta etapa ([37259242057](https://github.com/calneymgp/force-terminal/actions/runs/37259242057)) parou antes do pacote por duas expectativas de fixture: `/var` versus `/private/var` no cwd e um filho de shell que permaneceu no grupo após a saída do CLI. A reserva permaneceu corretamente incerta. Os testes foram corrigidos para usar caminho físico e um CLI fictício que encerra/recolhe seu próprio filho; o teste separado de filho sobrevivente continua exigindo reserva retida. A execução seguinte e os três smokes do pacote foram aprovados.

O DMG ad hoc 0.14.5 substituiu o instalador na [página privada Tailscale](https://calneyserver.tail802eab.ts.net:8443/force-terminal/). Tamanho: **201383210 bytes**; SHA-256: `053630f4b77202cd3eb1954f038bd11d40be44faeaf62f042abbfa2be5943608`. O conjunto DMG/ZIP/blockmaps/metadados foi validado pelo manifesto; o download completo pela URL teve o mesmo hash. O primeiro popup foi preservado, Continue abre diretamente o aplicativo e a reabertura não repete o onboarding. Não foi publicado feed ou release estável.

O roteiro `scripts/force-agent-smoke.mjs` usa o aplicativo Electron e backend reais, com perfil descartável e CLI fictício. Verifica criação pela interface, um único processo em clique repetido, contexto congelado, fechamento/reabertura sem início automático, reconexão no mesmo bloco/UUID e cadastro inerte de SSH/Codex. Nenhum provedor ou host SSH é chamado.

## Limites e próximos aceites

- O UUID é **solicitado**, não observado/validado pelo fornecedor. É necessário autorizar e observar uma conversa Claude descartável para comprovar contexto anterior na retomada.
- SSH não tem stager nem execução/reanexação de agente integrados neste caminho. Perfis remotos e Codex ficam preparados com execução indisponível.
- O diretório de retomada é a raiz configurada. Ainda não há evento confiável para observar mudanças de cwd dentro do CLI; a interface informa essa limitação.
- O painel filtra agentes por projeto, mas os blocos continuam nas abas/layouts herdados. Selecionar outro projeto não estabelece um layout exclusivo: agentes de projetos diferentes podem ocupar a mesma aba, embora seus contextos de execução permaneçam separados.
- O CLI usa o contexto de histórico existente do usuário. O Force não instala, autentica nem lê conversas/credenciais do provedor; instalações customizadas fora dos locais considerados dependem de PATH.
- Fechar o aplicativo pode encerrar processos locais. Não há promessa de sobrevivência após desligamento, contenção de processos que se destacam para outro grupo, ou conclusão de tarefa inferida a partir do exit code.
- Uma queda entre o checkpoint de lançamento e a persistência de PID/boot ID permanece incerta, inclusive após reboot: não há prova suficiente para iniciar novamente. A remoção do prompt é best-effort e sua referência de limpeza é mantida em memória; um crash pode deixar o arquivo privado temporário para a limpeza do sistema. Não há alegação de remoção após crash/reinício nem retry de erro de remoção.
- O pacote macOS arm64 foi exercitado no runner com CLI fictício. Reboot real do notebook, conversa real, SSH e aceite físico no M5 precisam de provas próprias. O canal de atualização instalada N→N+1 continua pendente de identidade de assinatura estável autorizada.

Fonte da instalação nativa: [documentação oficial Claude Code](https://code.claude.com/docs/en/setup#auto-updates). Critérios completos em [.plans/force-agent-runtime/PLAN.md](../../.plans/force-agent-runtime/PLAN.md).
