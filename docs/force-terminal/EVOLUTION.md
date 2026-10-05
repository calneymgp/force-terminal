# Force Terminal: evolução e estado do produto

Análise do compilado de ideias fornecido pelo usuário e do checkout `bc6de9a138dfe8d37f53c46d2f05ff3594d74a2e` do Wave. Atualizada em 5 de outubro de 2026, incluindo a direção Agent-first e a retomada após desligar o notebook. Esta página distingue o que entrou no instalador de teste, o código em validação e os aceites ainda necessários. Projetos, perfis e runtime Claude local foram entregues; execução e retomada passaram com CLI fictício no Linux/macOS, sem aceite de conversa real ou SSH.

## Estado em 5 de outubro de 2026

- **Concluído:** identidade oficial Raio Lunar e aplicação da marca Force Terminal à interface, ícones, janelas, favicons e metadados do pacote. Os arquivos e regras oficiais estão em [BRAND.md](BRAND.md).
- **Implementado:** perfis Force locais próprios, cache e namespaces de helper remoto separados, sem descoberta/importação automática dos dados Wave. Produção usa `force-terminal`; desenvolvimento usa `force-terminal-dev`; o helper remoto usa `~/.force-terminal`.
- **Implementado:** botão permanente de atualização, consulta de metadados ao abrir e por padrão a cada 10 minutos, download somente após clique, progresso e reinício condicionado à confirmação de persistência e ausência de trabalho em risco. Builds de desenvolvimento/ad hoc exibem indisponibilidade de atualização oficial.
- **Persistência corrigida:** o preparo para atualização aguarda também secrets recém-alterados, incluindo gravações concorrentes. No fechamento comum, o backend tenta gravar arquivos e secrets antes de sair, mantendo a conexão de criptografia disponível durante essa etapa.
- **Distribuição pelo GitHub:** seguir o modelo do tamz-bot, mantendo Electron. Tags usam o modo `community` com certificado próprio estável e sem notarização; `official` com Developer ID/notarização fica opcional. Execução manual gera artefatos ad hoc sem updater. A release só é publicada após conferir pacotes e hashes. Nenhuma release Force foi publicada nesta implementação.
- **Validação disponível:** build de produção e TypeScript completo passaram; os mocks preexistentes foram corrigidos. Testes de perfis, updater, guardas, persistência e validadores de release passaram. Evidências e limitações estão nos relatórios ligados abaixo.
- **Smoke adicional:** um secret fictício foi recuperado após salvar, fechar imediatamente e reabrir no Linux. O teste utilizou um bootstrap temporário com `basic_text`, sem alterar a criptografia do produto; Keychain macOS segue sem aceite.
- **DMG de teste entregue e instalado:** versão 0.14.5, disponibilizada em página HTML privada na rede Tailscale. O usuário confirmou instalação e abertura no notebook, com o símbolo Force nas boas-vindas. Isso confirma esse fluxo inicial; terminal, SSH, coexistência com Wave e demais superfícies da marca ainda precisam de aceite no aparelho. Não foram criados certificados/secrets ou publicados releases estáveis.
- **Pacote macOS executado:** o [build atualizado 37224909810](https://github.com/calneymgp/force-terminal/actions/runs/37224909810) corrigiu as páginas iniciais para o fork Force e substituiu o DMG no link privado. O [smoke 37225675703](https://github.com/calneymgp/force-terminal/actions/runs/37225675703) aprovou abertura, comando Zsh, leitura/escrita, editor visual/Cmd+S, encerramento nativo e persistência ao reabrir. Esses testes usam perfil descartável no runner macOS; não substituem os aceites no M5, SSH, coexistência, Keychain ou atualização assinada.
- **Pendente para distribuição com updater:** configurar o certificado próprio Force no CI e gerar pacotes com essa identidade estável; executar smoke e conferir a marca no MacBook M5; validar coexistência com Wave, SSH e secrets/Keychain; comprovar uma atualização instalada de N para N+1. O DMG ad hoc já foi entregue e não exige essa configuração. Conta Apple Developer não é requisito do canal `community`.
- **Prévia visual independente:** o [pitch animado de boas-vindas](../../assets/force-terminal/onboarding-pitch/index.html) foi publicado como HTML no Tailscale. Mostra uma proposta de direção, agentes/sessões, tarefas e rotinas; ainda não está incorporado ao DMG nem implementa esses recursos.
- **Onboarding simplificado e entregue:** somente o primeiro popup de boas-vindas, com o símbolo aprovado. Continue salva a conclusão e abre o aplicativo diretamente, sem pedido adicional de estrela, tour de recursos, animação ou popups automáticos de novidades herdados do Wave. O [build 37241487184](https://github.com/calneymgp/force-terminal/actions/runs/37241487184) substituiu o DMG no Tailscale; o [smoke 37242225109](https://github.com/calneymgp/force-terminal/actions/runs/37242225109) confirmou esse fluxo e a reabertura sem onboarding, além de terminal, arquivos/editor e persistência. A prévia animada fica arquivada como conceito. Investir em um novo onboarding somente depois que as funcionalidades Agent-first estiverem disponíveis.
- **Primeira versão Agent-first em teste:** execução local pelo botão **Novo agente** e persistência da sessão lógica com ação **Reconectar agente** estão no DMG. Conversa real, reboot físico e execução de agentes SSH permanecem pendentes. Tarefas e notas ficam vinculadas a esse trabalho; sua existência não será requisito para iniciar um agente.
- **Correção do acesso ao primeiro agente em validação:** criar um projeto passa a selecioná-lo e revelar a seção Agentes, com botão textual **Novo agente**. Antes, o usuário precisava clicar no projeto após salvar; o smoke do aplicativo reproduziu a seção escondida. Se o cadastro for salvo mas a seleção falhar, **Tentar selecionar** reutiliza o registro existente. Cadastro e seleção não iniciam processos. O pacote servido será substituído somente após validação do novo DMG; execução Codex e agentes SSH continuam pendentes.
- **Runtime local entregue no DMG de teste:** instância persistente, perfil congelado, terminal lógico estável e ações explícitas Novo agente, Reconectar agente e Nova sessão. O adaptador Claude prepara um UUID e solicita retomada por esse ID exato; esse registro permanece como identidade solicitada, sem afirmar confirmação pelo fornecedor. CLI fictício validou criação sem tarefa, reabertura sem execução automática e retomada no mesmo bloco/diretório no Linux e macOS. O [build 37259821901](https://github.com/calneymgp/force-terminal/actions/runs/37259821901), fonte `5e0922a443f439b8cc936b787790d577d484d31c`, foi aprovado pelo [smoke 37260967299](https://github.com/calneymgp/force-terminal/actions/runs/37260967299): 7 checks essenciais, 8 do catálogo e 9 do runtime passaram no aplicativo extraído do DMG. Substituiu o instalador na [página privada Tailscale](https://calneyserver.tail802eab.ts.net:8443/force-terminal/), preservando somente o primeiro popup e a reabertura sem onboarding. SSH e execução Codex permanecem indisponíveis. Detalhes, hash e limites em [AGENT-RUNTIME-REPORT.md](AGENT-RUNTIME-REPORT.md).
- **Catálogo implementado e entregue:** painel de projetos locais/SSH e biblioteca de perfis com título, ícone, system prompt e preferência de CLI. Cadastro, edição, arquivo/reativação e seleção persistem no SQLite Force. Smokes reais no Linux e no DMG macOS aprovados; erros preservam o formulário e alterações pendentes bloqueiam o reinício do updater. Retry de criação após resposta perdida evita duplicação. O [build 37246104954](https://github.com/calneymgp/force-terminal/actions/runs/37246104954), aprovado pelo [smoke 37247460436](https://github.com/calneymgp/force-terminal/actions/runs/37247460436), substituiu o DMG na [página privada Tailscale](https://calneyserver.tail802eab.ts.net:8443/force-terminal/). Evidências e limites em [CATALOG-REPORT.md](CATALOG-REPORT.md).
- **Privacidade inicial:** telemetria desativada por padrão nos novos perfis; configurações explícitas existentes continuam respeitadas. O teste confirmou o valor efetivo no renderer.
- **Infraestrutura de agentes SSH em desenvolvimento:** o helper oferece consulta, preparo e limpeza por protocolo versionado, sem executar o CLI; os jobs transportam o diretório tipado até o processo. A chave de host aceita e a sessão SSH são capturadas após autenticação, e um waiter de conexão antiga não pode encerrar a conexão nova. APIs genéricas de jobs recusam anexação ao terminal Force pelo vínculo persistente; criação/anexação usam uma transação para evitar jobs órfãos em caso de recusa. Dez pacotes passaram com detector de concorrência na integração Linux; geração e TypeScript também passaram. Ainda faltam vincular a resposta do helper àquela conexão, implementar a reserva remota e integrar execução/reconexão antes de habilitar agentes SSH. O instalador servido permanece no build de runtime local descrito acima. Evidências em [SSH-PREPARE-REPORT.md](SSH-PREPARE-REPORT.md), [SSH-RPC-PREPARE-REPORT.md](SSH-RPC-PREPARE-REPORT.md) e [SSH-IDENTITY-REPORT.md](SSH-IDENTITY-REPORT.md).

**O que mudou no DMG atual:** além da marca Force, dados isolados e catálogo persistente, o painel oferece Novo agente, Reconectar agente e Nova sessão para Claude local. A reconexão mantém instância, bloco, diretório e perfil e solicita o UUID salvo; o teste com CLI fictício não comprova contexto de conversa real após desligar o notebook. O Claude Code deve estar instalado no Mac. Mantemos apenas o primeiro popup; terminal comum, abas, workspaces, SSH comum e editor ainda usam a experiência herdada. Execução de agentes SSH/Codex, tarefas/notas e rotinas permanecem próximas entregas. O pacote ad hoc desativa o feed de atualização. A prévia animada fica arquivada.

Os nomes internos de variáveis de transporte, protocolos e binários `wavesrv`/`wsh` foram preservados; seus caminhos operacionais agora são Force. O instalador de teste foi entregue; o aceite de atualização depende de pacotes com assinatura estável e do ciclo instalado real, sem publicação na App Store. Consulte [plano aprovado](FOUNDATION-PLAN.md), [roteiro macOS](MACOS.md), [isolamento](ISOLATION-REPORT.md), [updater](UPDATES-REPORT.md), [pipeline](RELEASE-REPORT.md), [modelo tamz-bot](DISTRIBUTION.md) e [revisão](FOUNDATION-REVIEW.md).

O [quadro de aceites](ACCEPTANCE-AUDIT.md) separa código e testes disponíveis das provas ainda necessárias no M5 e no GitHub. Na conferência remota inicial, o fork ainda não tinha workflows, execuções ou releases e os cinco secrets Apple estavam ausentes; essa ausência não impede preparar o canal `community`. O código agora está em `main` e o build de teste foi aprovado; nenhuma release estável foi publicada.

## Direção confirmada: Agent-first

Construir um ambiente para conduzir trabalho com agentes a partir de **projetos e agentes persistentes**. O caso de uso principal é trabalhar em **três projetos simultâneos**, locais ou por SSH, com vários agentes em cada um — por exemplo DevOps, ETL de dados e Marketing. Preservar a infraestrutura de terminal, SSH, arquivos e transporte do Wave. O primeiro diferencial deve responder: “em qual projeto este agente trabalha, qual é seu papel, o que precisa de mim e como reconecto ao mesmo trabalho?”.

Assumimos inicialmente agentes executados como CLIs no terminal, começando por um adaptador compatível com o uso cotidiano. A organização de projetos, perfis e instâncias não deve depender de um único fornecedor. O chat embutido continua disponível como ferramenta auxiliar.

Na experiência principal, selecionar um projeto e clicar em **Novo agente** substitui começar por “Novo terminal”. O usuário escolhe um perfil e o Force prepara o destino, diretório e CLI. O terminal é a superfície de execução daquele agente; sua identidade permanece mesmo quando o processo termina. A estrutura de navegação será:

```text
Projeto — raiz local ou conexão SSH + raiz remota
  Agente no projeto — identidade persistente, derivada de um perfil
    Sessão lógica — contexto e identificador da conversa do CLI
    Execuções e retomadas — processos, terminais e jobs ao longo do tempo
    Tarefas e notas — objetivos, decisões e referências desse trabalho

Perfis de agente — biblioteca reutilizável de papéis
  Título / ícone / system prompt / adaptador CLI / configuração
```

**Projeto:** nome, ícone opcional e destino de trabalho. Um projeto local aponta para uma pasta; um remoto aponta para uma conexão SSH e um diretório naquele host. Não precisa ser um repositório Git. O destino e o diretório ficam claros ao criar ou reconectar o agente.

**Perfil:** título, ícone e system prompt, além do CLI/adaptador e opções necessárias para iniciá-lo. Exemplos: DevOps, ETL de dados e Marketing. Um perfil pode ser reutilizado nos três projetos. O adaptador precisa confirmar como aplica o system prompt; não tratar comandos ou flags não suportados como uma integração pronta.

**Agente no projeto:** uma instância persistente com ID próprio, perfil/configuração versionados, projeto, sessão do CLI, terminal lógico, última atividade e estado de conexão. Usar o perfil DevOps em dois projetos cria duas instâncias com contextos distintos. Alterar o perfil não deve mudar silenciosamente a configuração de uma conversa já iniciada.

**Execução:** cada início, reconexão ou retomada mantém sua referência ao agente. No MVP, cada instância terá no máximo uma execução ativa; um projeto pode ter vários agentes ativos. Fechar um bloco de terminal não apaga o agente, sua sessão lógica, tarefas ou notas. Tarefas podem existir sem agente e ser atribuídas depois; criar uma tarefa não é pré-requisito para **Novo agente**.

## O que faz sentido no compilado

- **Preservar o Wave como base.** Evita reimplementar renderer, integração de shell, SSH, editor, acesso a arquivos e reconexão.
- **Persistência lógica separada da execução.** Projetos, perfis, instâncias de agentes, sessões do CLI, objetivos e notas precisam sobreviver ao fechamento do aplicativo e ao fim do processo.
- **Tarefas e scratchpad ligados ao agente/projeto.** Organizam o trabalho sem obrigar o usuário a criar uma tarefa antes de iniciar um agente.
- **“Precisa de você”.** É uma boa organização da atenção humana; pode começar como um filtro de tarefas com contador.
- **Adaptadores por agente.** Permitem integrar capacidades verificadas de cada CLI sem espalhar regras específicas por toda a interface.
- **Worktrees para execuções concorrentes que alteram código.** O isolamento faz sentido nessa situação, sem obrigar notas, consultas e tarefas sem código a criar worktrees.

O compilado inicial priorizava tarefas como fundamento; a escolha posterior do usuário coloca projetos e agentes na entrada principal. A ordem passa a ser: cadastrar projetos/perfis → iniciar agentes no projeto → reconectar ao mesmo trabalho → enriquecer com tarefas/notas e rotinas. Retomada manual básica faz parte dessa primeira experiência; watchdog, recuperação automática e delegação avançada continuam posteriores.

## Prioridade de implementação

Os esforços abaixo são relativos, não estimativas de calendário. A comparação considera aproveitar a base existente.

| Ordem | Entrega | Estado e escopo inicial | Esforço | Critério para considerar pronta |
|---|---|---|---|---|
| 0 | Ambiente e referência funcional | **Build macOS aprovado; usuário confirmou instalação/abertura:** demais fluxos M5 pendentes; comandos completos, perfil temporário e roteiro disponíveis | Pequeno a médio | Terminal, SSH, arquivos e reabertura funcionam no M5 em perfil Force isolado |
| 1a | Identidade Force Terminal | **Marca aplicada e vista nas boas-vindas instaladas:** demais superfícies M5 pendentes | Pequeno a médio | Marca aprovada conferida no Finder, Dock, aplicativo e instalador |
| 1b | Distribuição e atualizações Force | **Código implementado; aceite pendente:** isolamento, updater com guardas e pipeline macOS arm64; falta configurar certificado próprio, gerar pacote e testar atualização instalada real | Médio | Versão assinada N atualiza para N+1 somente pelo feed Force, preserva dados e adia quando há trabalho em risco; Wave permanece intacto |
| 2 | Projetos e perfis de agentes persistentes | **Implementado e entregue no DMG; smokes Linux/macOS aprovados:** catálogo local/SSH, perfis, CRUD, arquivo/reativação e seleção persistente; uso no M5 ainda por conferir | Médio | Manter três projetos, reutilizar perfis DevOps/ETL/Marketing e recuperar os cadastros ao reabrir |
| 3 | Novo agente dentro do projeto | **Local entregue em teste:** instância/terminal persistentes, perfil congelado, argv Claude e reserva de escrita; CLI fictício aprovado no Linux/macOS. SSH, M5 e conversa real pendentes | Médio a grande | Iniciar vários agentes nos projetos pelo botão Novo agente, com contextos separados e sem criar uma tarefa antes |
| 4 | Reconectar agente após fechar/desligar | **Local entregue em teste:** mesmo bloco e UUID exato, reconciliação de processo e proteção contra repetição no Linux/macOS; reboot físico, conversa real e job SSH pendentes | Grande | Após desligar/religar o notebook, um clique recupera o mesmo trabalho e a mesma sessão quando suportada; falhas não criam duplicatas ou uma conversa nova silenciosamente |
| 5 | Tarefas e notas do projeto/agente | Título, objetivo, estado manual e Markdown, relacionados ao agente e às suas execuções quando necessário | Médio | Criar tarefa/nota, fechar/reabrir e recuperá-las no mesmo projeto/agente |
| 6 | Estados observáveis e “Precisa de você” | Execução ativa, encerramento, falha, desconexão, retomada disponível e pedido de ação confirmado; filtro de atenção | Médio | Cada estado tem uma fonte identificável; desconexão não é conclusão e silêncio não é travamento |
| 7 | Isolamento Git e links GitHub | Ampliar isolamento de escrita, worktrees e revisão de branch/diff; referências de issue/PR | Médio | Agentes de código concorrentes usam checkouts separados e o resultado pode ser revisado; a proteção básica de concorrência já se aplica ao iniciar agentes |
| 8 | Rotinas com um caso validado | Associar rotina a projeto/perfil; primeiro ação manual repetível, depois agendamento com registros e pausa | Grande | A rotina usa o destino e agente definidos, não se sobrepõe por acidente e deixa falhas visíveis |
| 9 | Recuperação avançada e orquestração | Watchdog por CLI, retomada automática configurada, handoff e delegação entre agentes | Grande | Recuperação e colaboração testadas em falhas reais, sem perder contexto nem repetir efeitos |

**A primeira entrega que muda o uso cotidiano deve completar 2–4:** projetos, perfis, Novo agente e Reconectar agente, com um CLI integrado e retomada comprovada. O fechamento de 1b continua necessário para distribuir com atualização instalada, mas a configuração de assinatura não precisa bloquear o desenvolvimento Agent-first nem novos pacotes de teste. Tarefas/notas vêm na prioridade 5; rotinas, supervisor e orquestração avançada ficam posteriores. A alteração visual das boas-vindas pode acompanhar essa entrega, apresentando apenas capacidades realmente disponíveis.

### Persistência e “Reconectar agente”

**Requisito confirmado pelo usuário, com implementação local em validação:** desligar o notebook, voltar ao Force e clicar em **Reconectar agente** para abrir o mesmo trabalho: projeto, diretório, conexão SSH quando existir, perfil do agente e conversa anterior do CLI. Restaurar abas ou reabrir uma shell sem contexto não encerra este item. O CLI fictício verifica os argumentos e as identidades locais; a conversa real e os cenários SSH ainda precisam de implementação/aceite próprios.

Persistir continuamente no armazenamento Force, sem depender apenas de um fechamento limpo: IDs do projeto/instância/terminal lógico, referência e versão do perfil, destino local ou conexão SSH, diretório atual confirmado pela integração de shell, CLI/adaptador/versão, identificador de sessão do CLI, referências de job/execução e última atualização confirmada. Se o diretório atual não puder ser observado, guardar a raiz configurada e identificar a limitação; não adivinhar o caminho. Referenciar as credenciais no mecanismo existente, sem copiá-las para esse registro. Preservar também tarefas/notas e o histórico visual persistido quando disponível, sem usar o texto do terminal como substituto da conversa do agente.

O terminal lógico conserva sua identidade, vínculo ao agente e localização na interface. Se for necessário recriar uma PTY, ela ocupa esse mesmo espaço e registra uma nova execução vinculada à sessão; não vira um agente sem relação com o anterior.

Ao clicar em **Reconectar agente**:

1. Carregar o projeto, a instância e o último destino/diretório confirmado; restabelecer a conexão local/SSH e validar que o diretório existe.
2. Se o backend/helper confirmar que o job anterior está vivo e pertence à instância, reanexar esse job e seu terminal, sem iniciar outro agente.
3. Se o processo terminou, consultar o adaptador sobre a sessão gravada. Quando suportado, iniciar um novo processo no mesmo destino/diretório e retomar o **identificador exato da conversa**. Um comando semelhante a `--resume` só será usado conforme a API/versão verificada do CLI; não escolher automaticamente a “última conversa” de um host compartilhado.
4. Conferir a retomada e atualizar o estado. Cliques repetidos, múltiplas janelas ou uma conexão em curso não podem criar processos/sessões duplicados.
5. Se o CLI não suporta retomada, o histórico desapareceu ou o destino está indisponível, manter o agente e seu contexto registrados, indicar a causa e permitir tentar novamente. **Nova sessão** é uma ação explícita; falha de retomada não deve abrir uma conversa vazia como se fosse a anterior.

O system prompt e as opções efetivas da sessão ficam versionados. A retomada usa a configuração compatível com a conversa anterior; aplicar um perfil editado ou um prompt diferente requer uma escolha explícita e suporte confirmado do adaptador. Não prometer persistência de estado interno que o CLI não oferece.

| Situação | Comportamento esperado |
|---|---|
| Fechar e reabrir o Force | Recuperar catálogo, instâncias e referências; reconectar job confirmado vivo ou oferecer retomada do CLI |
| Desligar/religar o notebook | Recuperar identidade e contexto persistidos; o processo local terminou e poderá ser recriado para retomar a mesma conversa, conforme suporte do CLI |
| Notebook desligado, job SSH ainda vivo no servidor | Reconectar ao mesmo host/diretório e reanexar o mesmo job confirmado, sem duplicá-lo |
| Servidor SSH reiniciou ou job foi perdido | Registrar a perda do processo; retomar a conversa do CLI se seu histórico/identificador continuarem válidos no host |
| CLI sem suporte a retomada ou histórico ausente | Preservar o registro do agente; mostrar a limitação e oferecer nova sessão separadamente |

**Aceites obrigatórios:** manter três projetos simultâneos, com uma combinação local/SSH e vários agentes DevOps/ETL/Marketing; confirmar que hosts, diretórios, perfis e conversas não se misturam. Testar fechamento/reabertura, desligamento real do notebook, queda de rede, job remoto ainda vivo e reinício do host remoto. Para cada retomada suportada, comprovar o mesmo ID de instância e de sessão do CLI, contexto anterior acessível e ausência de execução duplicada. Dados salvos devem sobreviver também a encerramento abrupto, até o último checkpoint confirmado; restaurar scrollback isoladamente não comprova retomada da conversa.

### Atualização do aplicativo na primeira versão

O Force Terminal tem um botão permanente de atualização na topbar. Em builds publicados `community` ou `official`, ao abrir o app consulta o feed de metadados e repete a consulta por padrão a cada **10 minutos** (`600000` ms); o botão também permite consultar manualmente, mesmo com consultas automáticas desativadas. Consultas automáticas não baixam nem instalam arquivos. Ao clicar em “Atualizar”, o app faz uma consulta imediata; se não houver versão nova, mostra que está atualizado. Se houver, baixa e prepara instalação/reinício como consequência desse clique, sem segunda confirmação quando não há bloqueios. A interface representa consulta, disponibilidade, progresso, preparação, reinício, erro e estado atualizado. Builds dev/ad hoc não consultam nem aplicam o feed publicado.

O código define `autoDownload = false` e `autoInstallOnAppQuit = false`; downloads automáticos e instalação incidental ao fechar o app ficam desativados. O clique autoriza o ciclo, mas não a interrupção silenciosa de trabalho. A preparação congela entradas temporariamente, aguarda salvamentos, consulta todos os controllers/jobs e exige flush confirmado pelo backend. Comando ativo, terminal sem confirmação atual, falha de salvamento, timeout de renderer ou mudança das views mantém o pacote pronto com reinício adiado. Uma nova tentativa reavalia os bloqueios. Projetos, perfis, instâncias/sessões de agentes e tarefas/notas deverão participar desta guarda de persistência quando forem implementados.

O pipeline implementado usa um runner macOS arm64 para publicar por versão os pacotes e metadados em **GitHub Releases de `calneymgp/force-terminal`**, usando o provider GitHub do updater. O fluxo é: tag/versionamento → build e verificações → `task force:package:mac-arm64` → envio dos artefatos e metadados para draft → validação remota → publicação automática da release completa. Releases em preparação ficam como draft até todos os arquivos necessários estarem disponíveis; o usuário consulta apenas versões publicadas. Credenciais ficam nos secrets do CI, sem distribuir tokens de publicação no aplicativo. Releases e metadados Wave nunca são usados pelo Force. A configuração segue o mecanismo de publicação e atualização do [electron-builder](https://www.electron.build/docs/features/auto-update/), respeitando a versão usada pelo projeto.

O updater valida o provider GitHub e o repositório Force antes de consultar. Os defaults são `autoupdate:enabled=true`, `installonquit=false` e `intervalms=600000`. As tarefas de publicação S3/Snap/Winget e o workflow de versionamento dependente da aplicação Wave foram removidos do caminho de distribuição. O pipeline inicial é macOS arm64: `task force:package:mac-arm64` gera DMG/ZIP e o CI valida manifestos, hashes e assets remotos antes de publicar. Windows, Linux e Mac Intel ficam fora deste aceite inicial. Assinatura própria estável e atualização instalada real continuam pendentes; notarização só é exigida no modo opcional `official`. Um build/teste em Linux não comprova esses aceites.

Atualizar o binário não pode apagar projetos, perfis, instâncias, identificadores de sessão do CLI, tarefas ou notas; após o reinício, esses dados devem continuar disponíveis. A ação Reconectar agente segue o fluxo de retomada acima: reanexar processo confirmado vivo ou retomar a conversa por um adaptador compatível. O produto não deve prometer que processos locais sobreviverão ao reinício do aplicativo, nem aprovar automaticamente prompts de CLI para efetuar uma atualização.

Critérios de aceite do fluxo: testar uma atualização real de uma versão empacotada para outra (sem download até clique, progresso, validação do artefato, instalação e reinício na nova versão), ausência de atualização (incluindo consulta manual), erro de rede com opção de tentar novamente e preservação do catálogo de projetos/agentes, referências de sessão e tarefas/notas depois da instalação, conforme esses domínios forem entregues. A consulta periódica não deve iniciar operações sobrepostas nem exibir avisos repetidos quando não houver mudança. Erros de download/verificação mantêm a versão instalada funcionando e não disparam a instalação. A interface deve distinguir retomar a mesma conversa de criar uma sessão nova.

## Como organizar agentes, tarefas, notas e navegação

### Tarefas

Começar com título, objetivo, projeto/instância de agente opcionais, estado e última atualização. Oferecer os estados da tarefa **A fazer, Em andamento, Precisa de você e Concluída**, inicialmente controlados pelo usuário. Arquivar é uma ação separada. Tarefas enriquecem o trabalho iniciado pelo agente; não são a entrada obrigatória nem a identidade da sessão.

Os estados da execução são outro dado: processo ativo, encerrado, falhou, desconectado ou desconhecido. Um exit code zero indica que o processo terminou sem erro informado; não comprova que o objetivo da tarefa foi cumprido. Um evento do agente pode sugerir “pronta para revisão”; a conclusão da tarefa continua explícita.

Não incluir no MVP subtarefas hierárquicas, dependências, sprints, calendários, estimativas e um quadro Kanban configurável. Uma lista com filtros resolve a organização inicial.

### Notas

Usar um arquivo Markdown por tarefa, aberto nas superfícies de edição e preview existentes. Manter uma única fonte de verdade; o banco guarda a referência e os metadados, sem uma segunda cópia do texto. O arquivo deve ter localização visível. O acesso por um CLI deve ser explícito e compatível com o caminho e o host usados; uma nota local não fica automaticamente disponível para um agente remoto. Envio de contexto e sincronização remota são decisões separadas.

Por padrão, guardar as notas na área de dados do Force Terminal, evitando criar arquivos de gestão em todos os repositórios. Permitir vincular um Markdown existente. Compartilhamento humano/agente precisa detectar alterações externas e evitar sobrescrever uma edição concorrente. Não criar automaticamente `AGENT.md`, `TASK.md`, `HANDOFF.md` e `DECISIONS.md` em cada checkout.

Uma tarefa pessoal pode conter apenas notas. Uma biblioteca independente de notas, backlinks, busca semântica ou um editor próprio pode esperar até haver uma necessidade concreta.

### Navegação

Entrada principal com **projetos e seus agentes**. Permitir alternar entre três ou mais projetos sem perder os agentes dos demais; cada item mostra o destino local/SSH e os agentes vinculados com título, ícone, estado e última atividade. Um painel de perfis permite cadastrar e reutilizar DevOps, ETL de dados, Marketing e outros papéis.

Ao selecionar um projeto, a ação principal é **Novo agente**. Ao selecionar um agente já existente e desconectado, a ação principal é **Reconectar agente**. Abrir o agente mostra seu terminal lógico, arquivos, contexto e tarefas/notas vinculadas. Iniciar, reconectar, encerrar execução e arquivar a instância são ações distintas; encerrar um processo não remove o agente do projeto.

O filtro **Precisa de você** pode reunir agentes e tarefas com um pedido confirmado. Estados de tarefas continuam disponíveis em sua própria lista. Um terminal avulso permanece como ferramenta secundária para comandos manuais, sem exigir que todo terminal tenha um perfil de agente.

Reutilizar os blocos, abas e layouts como mecanismo interno. Evitar reconstruir o layout completo ou criar um canvas. Um painel separado de Inbox só será necessário quando o filtro “Precisa de você” deixar de atender ao volume.

## O que adaptar, esconder ou retirar do Wave

| Elemento atual | Decisão | Motivo |
|---|---|---|
| Terminal, renderer, integração de shell, SSH e helper remoto | Preservar | São a infraestrutura de execução e acesso remoto |
| Blocos, abas, workspaces e layout | Adaptar para projetos e agentes | Novo agente/Reconectar agente são as ações principais; terminal/notas/arquivos continuam compostos pelos mecanismos existentes |
| Editor, Markdown e preview de arquivos | Reutilizar | A base já oferece edição, salvamento e visualização |
| RPC, eventos, SQLite e armazenamento de arquivos | Reutilizar com domínios persistentes de projeto, perfil, instância e sessão | Evita backend paralelo; a identidade do agente não pode ser apenas o rótulo ou PID do terminal |
| Armazenamento de secrets existente | Preservar | Não há motivo para construir um cofre novo no MVP |
| Nome, logotipo, ícones de marca e mensagens promocionais Wave | Marca Force aplicada | A identidade oficial está integrada; atribuições e serviços externos mantêm a identificação upstream |
| Feed de atualização e configuração de publicação/distribuição Wave | Updater e pipeline Force implementados; validação de distribuição pendente | Comprovar atualização instalada com pacotes assinados macOS arm64 antes do lançamento |
| Telemetria e endpoints hospedados pelo Wave | Telemetria desativada por padrão; revisar dependências de cada endpoint antes de retirar código | O fluxo básico de tarefas e CLIs não precisa desses serviços |
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
- **Confundir persistência do agente com sobrevivência do processo.** Retomada após desligar o notebook faz parte do MVP, mas depende do registro persistido e das capacidades confirmadas do CLI. Um PID/PTY local não sobrevive ao desligamento; reconectar deve restaurar a conversa suportada ou explicar a limitação.
- **Lançar vários agentes no mesmo checkout para alterar código sem isolamento.** Vários agentes por projeto fazem parte do uso principal; para escrita concorrente, usar worktrees/diretórios separados ou restringir a concorrência. Agentes de consulta ou papéis em recursos distintos não precisam criar worktrees por padrão.
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
| Telemetria desativada na configuração padrão Force | `pkg/wconfig/defaultconfig/settings.json` |
| Catálogo de projetos/perfis usa entidades próprias, versão otimista e chave de criação estável | `pkg/service/forceservice/forceservice.go`, `pkg/waveobj/forcecatalog.go`, `db/migrations-wstore/000012_force_catalog.up.sql` |

O chat store atual examinado é volátil; persistência própria desse chat seria trabalho adicional e não substitui a retomada da conversa de um CLI. Projetos, perfis, instâncias, referências de sessão, tarefas/notas e histórico de execuções precisam de persistência própria. Importação de dados e capacidades oficiais dos CLIs exigem verificação específica antes de entrar em uma promessa de produto. Os adaptadores devem declarar o que sabem fazer; parsing de terminal, quando inevitável, deve ser identificado como inferência.

## Identidade aprovada

![Ícone oficial Force Terminal — Raio Lunar](../../assets/force-terminal/brand/icon.png)

A identidade oficial é **Raio Lunar**: raio maciço com anel de poeira lunar, em preto `#000000`, branco `#FFFFFF` e transparência, com versões inversas e compactas. A marca está integrada à UI, às janelas, aos favicons e aos recursos do pacote. A fonte de verdade para aplicações, proporções e arquivos é [BRAND.md](BRAND.md); os vetores oficiais ficam em [`assets/force-terminal/brand/`](../../assets/force-terminal/brand/).

## Próxima fatia recomendada

A entrega do DMG de teste foi concluída e o usuário confirmou instalação/abertura. A prévia animada também foi entregue como HTML independente. A próxima mudança perceptível deve ser a navegação por projetos/agentes e a retomada do trabalho. A escolha de não criar certificado/configurar secrets continua válida; o aceite N → N+1 permanece para a etapa assinada posterior.

1. **Projetos e biblioteca de perfis:** implementados, validados no pacote macOS e entregues no Tailscale; conferir no M5. Perfil reutilizável permanece separado da futura instância vinculada ao projeto.
2. **Novo agente e primeiro adaptador:** criar a instância no projeto selecionado, aplicar a configuração suportada, abrir o terminal no destino/diretório correto e registrar a sessão do CLI assim que seu identificador estiver confirmado. Proteger a concorrência de escrita desde essa etapa.
3. **Persistência e Reconectar agente:** implementar checkpoints duráveis, reanexação de job vivo e retomada por ID exato; preservar a identidade visual/lógica ao recriar o processo e impedir duplicatas. A falta de suporte deve aparecer de forma clara, com nova sessão como ação separada.
4. **Validar o fluxo cotidiano:** iniciar agentes DevOps/ETL/Marketing em três projetos → alternar entre eles → fechar/reabrir → desligar/religar o notebook → reconectar à mesma conversa local/remota quando suportada. Testar também SSH sem rede, job remoto vivo e host reiniciado.
5. **Tarefas/notas e atenção:** vincular objetivos e Markdown ao projeto/agente; oferecer Precisa de você a partir de sinais confirmados. Só depois acrescentar uma primeira rotina repetível/agendada.

Em paralelo, concluir os aceites restantes de [MACOS.md](MACOS.md), marca instalada, isolamento/coexistência, SSH e Keychain. Quando a configuração de assinatura for autorizada, habilitar o canal GitHub e comprovar N → N+1, incluindo o flush dos novos domínios e a retomada posterior. Não considerar 1b concluída antes desse teste.

O teste de valor Agent-first é retomar um agente reconhecível no projeto correto e continuar sua conversa, sem procurar uma aba nem iniciar o trabalho novamente. A próxima integração deve resolver a fricção encontrada nesse ciclo antes de ampliar o produto com automações.
