# Force Terminal: evolução e estado do produto

Análise do compilado de ideias fornecido pelo usuário e do checkout `bc6de9a138dfe8d37f53c46d2f05ff3594d74a2e` do Wave. Atualizada em 4 de outubro de 2026. Esta página registra o estado do rebranding e as próximas etapas; recursos ainda não implementados permanecem como requisitos ou propostas, conforme indicado abaixo.

## Estado em 4 de outubro de 2026

- **Concluído:** identidade oficial Raio Lunar e aplicação da marca Force Terminal à interface, ícones, janelas, favicons e metadados do pacote. Os arquivos e regras oficiais estão em [BRAND.md](BRAND.md).
- **Implementado:** perfis Force locais próprios, cache e namespaces de helper remoto separados, sem descoberta/importação automática dos dados Wave. Produção usa `force-terminal`; desenvolvimento usa `force-terminal-dev`; o helper remoto usa `~/.force-terminal`.
- **Implementado:** botão permanente de atualização, consulta de metadados ao abrir e por padrão a cada 10 minutos, download somente após clique, progresso e reinício condicionado à confirmação de persistência e ausência de trabalho em risco. Builds de desenvolvimento/ad hoc exibem indisponibilidade de atualização oficial.
- **Persistência corrigida:** o preparo para atualização aguarda também secrets recém-alterados, incluindo gravações concorrentes. No fechamento comum, o backend tenta gravar arquivos e secrets antes de sair, mantendo a conexão de criptografia disponível durante essa etapa.
- **Distribuição pelo GitHub:** seguir o modelo do tamz-bot, mantendo Electron. Tags usam o modo `community` com certificado próprio estável e sem notarização; `official` com Developer ID/notarização fica opcional. Execução manual gera artefatos ad hoc sem updater. A release só é publicada após conferir pacotes e hashes. Nenhuma release Force foi publicada nesta implementação.
- **Validação disponível:** build de produção e TypeScript completo passaram; os mocks preexistentes foram corrigidos. Testes de perfis, updater, guardas, persistência e validadores de release passaram. Evidências e limitações estão nos relatórios ligados abaixo.
- **Smoke adicional:** um secret fictício foi recuperado após salvar, fechar imediatamente e reabrir no Linux. O teste utilizou um bootstrap temporário com `basic_text`, sem alterar a criptografia do produto; Keychain macOS segue sem aceite.
- **DMG de teste entregue:** [build macOS arm64 no GitHub aprovado](https://github.com/calneymgp/force-terminal/actions/runs/37221443395), versão 0.14.5. O DMG foi validado e disponibilizado em página HTML privada na rede Tailscale; não foram criados certificados/secrets ou publicados releases estáveis. A instalação no M5 continua pendente de confirmação do usuário.
- **Pendente para distribuição com updater:** configurar o certificado próprio Force no CI e gerar pacotes com essa identidade estável; executar smoke e conferir a marca no MacBook M5; validar coexistência com Wave, SSH e secrets/Keychain; comprovar uma atualização instalada de N para N+1. O DMG ad hoc já foi entregue e não exige essa configuração. Conta Apple Developer não é requisito do canal `community`.
- **Pendente para a primeira versão:** tarefas e notas persistentes, vínculo entre tarefa e execução e teste do ciclo de atualização com esses domínios quando existirem.

Os nomes internos de variáveis de transporte, protocolos e binários `wavesrv`/`wsh` foram preservados; seus caminhos operacionais agora são Force. O instalador de teste foi entregue; o aceite de atualização depende de pacotes com assinatura estável e do ciclo instalado real, sem publicação na App Store. Consulte [plano aprovado](FOUNDATION-PLAN.md), [roteiro macOS](MACOS.md), [isolamento](ISOLATION-REPORT.md), [updater](UPDATES-REPORT.md), [pipeline](RELEASE-REPORT.md), [modelo tamz-bot](DISTRIBUTION.md) e [revisão](FOUNDATION-REVIEW.md).

O [quadro de aceites](ACCEPTANCE-AUDIT.md) separa código e testes disponíveis das provas ainda necessárias no M5 e no GitHub. Na conferência remota inicial, o fork ainda não tinha workflows, execuções ou releases e os cinco secrets Apple estavam ausentes; essa ausência não impede preparar o canal `community`. O código agora está em `main` e o build de teste foi aprovado; nenhuma release estável foi publicada.

## Direção recomendada

Construir um ambiente simples para conduzir trabalho com agentes: **projetos, tarefas, notas e execuções visíveis**. Preservar a infraestrutura de terminal, SSH, arquivos e transporte do Wave. O primeiro diferencial deve ser conseguir responder: “o que este agente está fazendo, para qual tarefa, o que precisa de mim e como retomo esse trabalho?”.

Assumimos inicialmente Claude/Codex executados como CLIs no terminal. O chat embutido continua disponível como ferramenta auxiliar. A escolha de qual CLI integrar primeiro depende do uso cotidiano; o modelo de tarefas e notas não deve depender dessa escolha.

Ser agent-first significa dar identidade ao agente, relacionar sua execução ao objetivo e apresentar pedidos de atenção. Abrir uma PTY ou manter um processo vivo, por si só, não oferece isso. A estrutura proposta é:

```text
Projeto
  Tarefa — objetivo, estado, notas e histórico
    Agente — perfil de execução e capacidades disponíveis
    Execuções — tentativas e retomadas ao longo do tempo
      Terminal / arquivos / saída / conexão / sessão do CLI
```

Uma tarefa pode existir sem agente nem repositório. Pode acumular várias execuções; no MVP, terá no máximo uma execução ativa. Fechar um bloco de terminal não deve apagar a tarefa nem suas notas.

## O que faz sentido no compilado

- **Preservar o Wave como base.** Evita reimplementar renderer, integração de shell, SSH, editor, acesso a arquivos e reconexão.
- **Persistência lógica separada da execução.** Objetivo, notas, agente escolhido e referências de execução precisam sobreviver ao fechamento do aplicativo e ao fim do processo.
- **Tarefas e scratchpad compartilhado.** Entregam valor diário mesmo antes de existir integração avançada com os CLIs.
- **“Precisa de você”.** É uma boa organização da atenção humana; pode começar como um filtro de tarefas com contador.
- **Adaptadores por agente.** Permitem integrar capacidades verificadas de cada CLI sem espalhar regras específicas por toda a interface.
- **Worktrees para execuções concorrentes que alteram código.** O isolamento faz sentido nessa situação, sem obrigar notas, consultas e tarefas sem código a criar worktrees.

O compilado contém uma tensão: começa propondo tarefas como fundamento, termina propondo o supervisor como primeira entrega e inclui cron/PR automático no MVP. Recomendo resolver isso começando pela organização do trabalho e pela observação das execuções. Recuperação automática e automações dependem dessa base.

## Prioridade de implementação

Os esforços abaixo são relativos, não estimativas de calendário. A comparação considera aproveitar a base existente.

| Ordem | Entrega | Estado e escopo inicial | Esforço | Critério para considerar pronta |
|---|---|---|---|---|
| 0 | Ambiente e referência funcional | **Build completo macOS CI aprovado e DMG de teste entregue; aceite M5 pendente:** comandos completos, perfil temporário e roteiro disponíveis | Pequeno a médio | Aplicativo abre no M5 e os fluxos essenciais funcionam em perfil Force isolado |
| 1a | Identidade Force Terminal | **Marca aplicada; conferência do pacote M5 pendente:** UI, ícones, metadados e destino GitHub Force configurados | Pequeno a médio | Marca aprovada conferida no Finder, Dock, aplicativo e instalador |
| 1b | Distribuição e atualizações Force | **Código implementado; aceite pendente:** isolamento, updater com guardas e pipeline macOS arm64; falta configurar certificado próprio, gerar pacote e testar atualização instalada real | Médio | Versão assinada N atualiza para N+1 somente pelo feed Force, preserva dados e adia quando há trabalho em risco; Wave permanece intacto |
| 2 | Tarefas e notas persistentes | Título, objetivo, estado manual, notas Markdown; projeto e agente opcionais; lista de tarefas | Médio | Criar uma tarefa, escrever notas, fechar/reabrir o aplicativo e recuperar ambos |
| 3 | Vincular tarefa à execução | Associar ou abrir um bloco existente; mostrar terminal, arquivos e referências úteis dentro da tarefa | Médio | Retomar o trabalho pela tarefa sem procurar uma aba pelo nome do processo |
| 4 | Estados observáveis e “Precisa de você” | Informar execução ativa, encerramento, falha, desconexão e pedido de ação confirmado; contador/filtro de atenção | Médio | Cada estado tem uma fonte identificável e uma desconexão não é confundida com conclusão |
| 5 | Primeiro adaptador de agente | Um CLI por vez: iniciar no diretório escolhido, associar identificador de sessão quando disponível, receber eventos e oferecer retomada compatível | Médio a grande | Reiniciar o app permite localizar a execução ou oferecer uma retomada explícita sem duplicá-la |
| 6 | Isolamento Git e links GitHub | Worktree opcional para tarefas que modificam código; branch/diff visíveis; referências de issue/PR | Médio | Duas tarefas concorrentes usam checkouts separados e o usuário consegue revisar o resultado |
| 7 | Automação com um caso validado | Primeiro um comando manual repetível; depois agendamento simples com registros e pausa | Grande | Execuções não se sobrepõem por acidente, falhas ficam visíveis e efeitos externos têm política explícita |
| 8 | Recuperação avançada e colaboração entre agentes | Watchdog específico por CLI, retomada após falha, handoff e delegação | Grande | Recuperação testada em falhas reais, sem perder contexto nem repetir efeitos |

**A primeira versão distribuída deve completar as prioridades 1b–3, com a identidade 1a já aplicada:** atualizações Force, tarefas/notas e uma execução vinculada. O atualizador é parte do lançamento inicial, não uma etapa condicionada a um futuro canal de distribuição. Pode funcionar com o CLI iniciado manualmente e estados manuais. A prioridade 4 melhora a atenção; a 5 torna a integração mais profunda. Cron, supervisor e integração de tarefas com issues/PRs do GitHub podem esperar; GitHub Releases faz parte da distribuição inicial.

### Atualização do aplicativo na primeira versão

O Force Terminal tem um botão permanente de atualização na topbar. Em builds publicados `community` ou `official`, ao abrir o app consulta o feed de metadados e repete a consulta por padrão a cada **10 minutos** (`600000` ms); o botão também permite consultar manualmente, mesmo com consultas automáticas desativadas. Consultas automáticas não baixam nem instalam arquivos. Ao clicar em “Atualizar”, o app faz uma consulta imediata; se não houver versão nova, mostra que está atualizado. Se houver, baixa e prepara instalação/reinício como consequência desse clique, sem segunda confirmação quando não há bloqueios. A interface representa consulta, disponibilidade, progresso, preparação, reinício, erro e estado atualizado. Builds dev/ad hoc não consultam nem aplicam o feed publicado.

O código define `autoDownload = false` e `autoInstallOnAppQuit = false`; downloads automáticos e instalação incidental ao fechar o app ficam desativados. O clique autoriza o ciclo, mas não a interrupção silenciosa de trabalho. A preparação congela entradas temporariamente, aguarda salvamentos, consulta todos os controllers/jobs e exige flush confirmado pelo backend. Comando ativo, terminal sem confirmação atual, falha de salvamento, timeout de renderer ou mudança das views mantém o pacote pronto com reinício adiado. Uma nova tentativa reavalia os bloqueios. Tarefas/notas futuras deverão participar desta guarda de persistência.

O pipeline implementado usa um runner macOS arm64 para publicar por versão os pacotes e metadados em **GitHub Releases de `calneymgp/force-terminal`**, usando o provider GitHub do updater. O fluxo é: tag/versionamento → build e verificações → `task force:package:mac-arm64` → envio dos artefatos e metadados para draft → validação remota → publicação automática da release completa. Releases em preparação ficam como draft até todos os arquivos necessários estarem disponíveis; o usuário consulta apenas versões publicadas. Credenciais ficam nos secrets do CI, sem distribuir tokens de publicação no aplicativo. Releases e metadados Wave nunca são usados pelo Force. A configuração segue o mecanismo de publicação e atualização do [electron-builder](https://www.electron.build/docs/features/auto-update/), respeitando a versão usada pelo projeto.

O updater valida o provider GitHub e o repositório Force antes de consultar. Os defaults são `autoupdate:enabled=true`, `installonquit=false` e `intervalms=600000`. As tarefas de publicação S3/Snap/Winget e o workflow de versionamento dependente da aplicação Wave foram removidos do caminho de distribuição. O pipeline inicial é macOS arm64: `task force:package:mac-arm64` gera DMG/ZIP e o CI valida manifestos, hashes e assets remotos antes de publicar. Windows, Linux e Mac Intel ficam fora deste aceite inicial. Assinatura própria estável e atualização instalada real continuam pendentes; notarização só é exigida no modo opcional `official`. Um build/teste em Linux não comprova esses aceites.

Atualizar o binário não pode apagar tarefas, notas ou seus metadados; após o reinício, esses dados devem continuar disponíveis. Uma sessão de terminal só pode ser recuperada se a infraestrutura existente confirmar que continua recuperável. O produto não deve prometer que processos locais sobreviverão ao reinício do aplicativo, nem aprovar automaticamente prompts de CLI para efetuar uma atualização.

Critérios de aceite do fluxo: testar uma atualização real de uma versão empacotada para outra (sem download até clique, progresso, validação do artefato, instalação e reinício na nova versão), ausência de atualização (incluindo consulta manual), erro de rede com opção de tentar novamente e preservação de tarefas/notas depois da instalação. A consulta periódica não deve iniciar operações sobrepostas nem exibir avisos repetidos quando não houver mudança. Erros de download/verificação mantêm a versão instalada funcionando e não disparam a instalação. A retomada de uma sessão deve ser oferecida somente quando confirmada como recuperável.

## Como simplificar tarefas, notas e navegação

### Tarefas

Começar com título, objetivo, projeto/diretório opcional, agente escolhido, estado e última atualização. Oferecer os estados da tarefa **A fazer, Em andamento, Precisa de você e Concluída**, inicialmente controlados pelo usuário. Arquivar é uma ação separada.

Os estados da execução são outro dado: processo ativo, encerrado, falhou, desconectado ou desconhecido. Um exit code zero indica que o processo terminou sem erro informado; não comprova que o objetivo da tarefa foi cumprido. Um evento do agente pode sugerir “pronta para revisão”; a conclusão da tarefa continua explícita.

Não incluir no MVP subtarefas hierárquicas, dependências, sprints, calendários, estimativas e um quadro Kanban configurável. Uma lista com filtros resolve a organização inicial.

### Notas

Usar um arquivo Markdown por tarefa, aberto nas superfícies de edição e preview existentes. Manter uma única fonte de verdade; o banco guarda a referência e os metadados, sem uma segunda cópia do texto. O arquivo deve ter localização visível. O acesso por um CLI deve ser explícito e compatível com o caminho e o host usados; uma nota local não fica automaticamente disponível para um agente remoto. Envio de contexto e sincronização remota são decisões separadas.

Por padrão, guardar as notas na área de dados do Force Terminal, evitando criar arquivos de gestão em todos os repositórios. Permitir vincular um Markdown existente. Compartilhamento humano/agente precisa detectar alterações externas e evitar sobrescrever uma edição concorrente. Não criar automaticamente `AGENT.md`, `TASK.md`, `HANDOFF.md` e `DECISIONS.md` em cada checkout.

Uma tarefa pessoal pode conter apenas notas. Uma biblioteca independente de notas, backlinks, busca semântica ou um editor próprio pode esperar até haver uma necessidade concreta.

### Navegação

Entrada principal com projetos e tarefas agrupadas em **A fazer, Em andamento, Precisa de você e Concluídas**. Exibir agente, quando escolhido, e última atividade em cada tarefa. Ao abrir uma tarefa, mostrar objetivo, terminal e notas; arquivos permanecem acessíveis no mesmo espaço de trabalho.

Reutilizar os blocos, abas e layouts como mecanismo interno. Evitar reconstruir o layout completo ou criar um canvas. Um painel separado de Inbox só será necessário quando o filtro “Precisa de você” deixar de atender ao volume.

## O que adaptar, esconder ou retirar do Wave

| Elemento atual | Decisão | Motivo |
|---|---|---|
| Terminal, renderer, integração de shell, SSH e helper remoto | Preservar | São a infraestrutura de execução e acesso remoto |
| Blocos, abas, workspaces e layout | Adaptar a navegação | Permitem compor terminal/notas/arquivos sem criar outro sistema de janelas |
| Editor, Markdown e preview de arquivos | Reutilizar | A base já oferece edição, salvamento e visualização |
| RPC, eventos, SQLite e armazenamento de arquivos | Reutilizar com domínio de tarefas próprio | Evita backend e sincronização paralelos; tarefas não devem ser apenas rótulos de processos |
| Armazenamento de secrets existente | Preservar | Não há motivo para construir um cofre novo no MVP |
| Nome, logotipo, ícones de marca e mensagens promocionais Wave | Marca Force aplicada | A identidade oficial está integrada; atribuições e serviços externos mantêm a identificação upstream |
| Feed de atualização e configuração de publicação/distribuição Wave | Updater e pipeline Force implementados; validação de distribuição pendente | Comprovar atualização instalada com pacotes assinados macOS arm64 antes do lançamento |
| Telemetria ligada por padrão e endpoints hospedados pelo Wave | Revisar e desativar por padrão no perfil inicial do fork | O fluxo básico de tarefas e CLIs não precisa desses serviços; dependências de cada recurso devem ser identificadas antes de retirar código |
| Chat Wave AI | Manter opcional e com menor destaque na hipótese CLI-first | Pode ajudar com contexto e arquivos; não equivale ao gerente de execuções proposto |
| Widgets de sistema/processos, dicas, launchers secundários e Tsunami | Esconder da navegação principal inicialmente | Reduzir distração tem custo menor e menos impacto do que apagar módulos com dependências |
| Browser e previews embutidos | Manter disponíveis como ferramentas secundárias | Úteis para verificar resultados; não exigem um novo browser de automação agora |
| Nomes internos `wave*`, módulo Go e protocolos `wsh` | Preservar inicialmente | Uma substituição textual global pode quebrar imports, caminhos remotos e compatibilidade sem melhorar a experiência |
| Licença, NOTICE e atribuições do código herdado | Preservar | Trocar a identidade visual não significa apagar a origem do código |

Trocar os ícones da marca não exige redesenhar todos os ícones funcionais de salvar, arquivos, SSH e configurações. Uma limpeza da navegação também não exige remover o backend correspondente na primeira etapa.

Para dados, recomendo uma instalação Force **isolada da instalação Wave**. Isso significa começar sem os workspaces, configurações e conexões da instalação Wave; uma importação pode ser adicionada depois, explicitamente. Não mover nem apagar diretórios Wave durante o rebranding. Pastas locais, caminhos remotos e namespace de credenciais precisam ser inventariados antes de alterar nomes internos.

## O que não fazer agora

- **Inferir travamento só por silêncio, CPU baixa ou falta de heartbeat.** Mostrar “sem atualização” ou “estado desconhecido” e abrir o terminal. Espera por aprovação, builds silenciosos e rede interrompida podem produzir os mesmos sinais.
- **Enviar Enter, Ctrl+C, matar processos ou retomar automaticamente por heurística.** São ações com efeitos no trabalho. Primeiro oferecer controles explícitos e reunir eventos confiáveis por agente.
- **Exibir porcentagem de progresso, contexto, tokens ou custo inventados.** Mostrar apenas métricas realmente fornecidas pela integração; ausência de dado deve aparecer como indisponível.
- **Prometer sobreviver a qualquer reboot.** Persistir a tarefa é diferente de preservar o processo. A retomada do CLI depende de identificadores e capacidades confirmados para aquela versão.
- **Lançar vários agentes no mesmo checkout para alterar código.** Primeiro implementar isolamento ou restringir a concorrência de escrita.
- **Fazer commit, push, PR e merge automáticos como comportamento padrão.** Na primeira etapa, mostrar resultado/diff e oferecer ações explícitas; um workflow automatizado pode ser escolhido depois.
- **Criar já um scheduler universal.** Cron, GitHub webhooks, eventos de arquivos e delegação exigem tratamento de concorrência, repetição, falhas e autorização. Isso ampliaria demais o MVP.
- **Implementar todos os adaptadores e funcionalidades de chat ao mesmo tempo.** Começar pelo CLI mais usado e tratar cada capacidade como opcional.
- **Criar um novo daemon remoto antes de provar que o helper/job manager existente não atende.** Explorar a infraestrutura atual antes de duplicar gerenciamento de processos.

## O que a inspeção do código confirmou

| Constatação | Fonte local |
|---|---|
| As views são registradas centralmente e já incluem terminal, preview, web e ferramentas secundárias | `frontend/app/block/blockregistry.ts:23` |
| O preview oferece edição/salvamento e tem implementação para Markdown e Monaco | `frontend/app/view/preview/preview-model.tsx:285`, `preview-markdown.tsx:11`, `preview-edit.tsx:39` |
| Jobs têm identidade, conexão, processo, timestamps e resultado de saída | `pkg/waveobj/wtype.go:313` |
| A decisão normal de durabilidade exclui conexões locais e WSL | `pkg/jobcontroller/jobcontroller.go:1363` |
| A reconexão remota depende de o job manager continuar vivo | `pkg/wshrpc/wshremote/wshremote_job.go:280` |
| A interface admite perda de sessão após reboot do host remoto | `frontend/app/block/durable-session-flyover.tsx:205` |
| O chat store examinado mantém conversas em um mapa em memória | `pkg/aiusechat/chatstore/chatstore.go:14` |
| O prompt do chat delimita suas capacidades; a integração de CLIs precisa ser tratada separadamente | `pkg/aiusechat/usechat-prompts.go:38`, `pkg/aiusechat/tools.go:136` |
| Existem operações transacionais no armazenamento de objetos | `pkg/wstore/wstore.go:43` |
| Metadados, perfis, updater e tarefas próprias usam Force Terminal; o pipeline inicial distribui somente macOS arm64 | `package.json`, `electron-builder.config.cjs`, `emain/force-profile.ts`, `emain/updater.ts`, `Taskfile.yml`, `.github/workflows/build-helper.yml` |
| Telemetria vem habilitada na configuração padrão | `pkg/wconfig/defaultconfig/settings.json:32` |

O chat store atual examinado é volátil; persistência própria de conversas seria trabalho adicional. Tarefas, notas e histórico de execuções precisam ter sua própria persistência. Importação de dados e capacidades oficiais dos CLIs exigem verificação específica antes de entrar em uma promessa de produto. Os adaptadores devem declarar o que sabem fazer; parsing de terminal, quando inevitável, deve ser identificado como inferência.

## Identidade aprovada

![Ícone oficial Force Terminal — Raio Lunar](../../assets/force-terminal/brand/icon.png)

A identidade oficial é **Raio Lunar**: raio maciço com anel de poeira lunar, em preto `#000000`, branco `#FFFFFF` e transparência, com versões inversas e compactas. A marca está integrada à UI, às janelas, aos favicons e aos recursos do pacote. A fonte de verdade para aplicações, proporções e arquivos é [BRAND.md](BRAND.md); os vetores oficiais ficam em [`assets/force-terminal/brand/`](../../assets/force-terminal/brand/).

## Próxima fatia recomendada

Entrega imediata escolhida pelo usuário: executar o build manual no GitHub, obter o DMG ad hoc e disponibilizá-lo em uma página HTML com botão de download na rede Tailscale. Não criar certificado/configurar secrets nesta entrega. O pacote de teste tem instalação manual e não ativa o updater; o aceite N → N+1 continua para a etapa assinada posterior.

1. **Concluir o aceite no M5:** executar o roteiro de [MACOS.md](MACOS.md) com terminal, SSH descartável, arquivos e persistência; conferir marca no pacote e coexistência com Wave, incluindo helpers remotos e Keychain. Os comandos e o isolamento já estão implementados.
2. **Gerar o primeiro pacote de teste:** executar o workflow manual para obter DMG/ZIP ad hoc e validar a instalação no M5. Esses artefatos não entram no feed oficial.
3. **Habilitar a distribuição pelo GitHub:** configurar o certificado próprio Force e seu fingerprint, executar o pipeline `community` por tag correspondente à versão e confirmar assinatura, hashes e publicação do conjunto completo. Developer ID/notarização ficam para o canal `official`, se escolhido depois.
4. **Aprovar a atualização instalada:** instalar N e atualizar para N+1 pelo botão, confirmando persistência e adiamento com comando ativo ou falha de salvamento. Só então encerrar 1b. Tarefas/notas futuras deverão participar da mesma preparação antes do reinício.
5. **Definir tarefas e notas persistentes:** título, objetivo, estado e nota Markdown como primeiro domínio novo.
6. **Vincular tarefa à execução:** mostrar terminal e referências úteis a partir da tarefa, com retomada oferecida somente quando confirmada como possível.
7. **Validar o ciclo da primeira versão:** criar tarefa → iniciar CLI → tomar notas → atualizar pelo botão → reabrir na nova versão → recuperar tarefa e notas. O terminal local atual não garante preservar o processo ao fechar o aplicativo.

Esse ciclo é o teste de valor da primeira versão. A próxima integração deve resolver a fricção encontrada nele, antes de ampliar o produto com automações.
