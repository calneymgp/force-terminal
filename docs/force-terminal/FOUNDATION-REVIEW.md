# Revisão independente da fundação Force Terminal

Revisão de leitura da árvore de trabalho em `codex/force-foundation-20261004`, com alterações de outros agentes ainda em curso. Referência: `FOUNDATION-PLAN.md` aprovado em 2026-10-04. Nenhum build de distribuição, release ou teste no M5 foi executado nesta revisão.

**Veredito deste escopo:** os riscos abaixo foram corrigidos na árvore local, incluindo a persistência de secrets identificada na auditoria posterior. A validação no M5 e a atualização assinada seguem pendentes.

**Evidência posterior à revisão:** o [build macOS arm64 37221443395](https://github.com/calneymgp/force-terminal/actions/runs/37221443395) passou e verificou o app, ZIP e DMG com assinatura ad hoc. O DMG de teste foi entregue via Tailscale; detalhes em [RELEASE-REPORT.md](RELEASE-REPORT.md). Isso não encerra o smoke no M5 nem a atualização instalada N → N+1. Developer ID e notarização são requisitos somente do canal opcional `official`; o canal `community` ainda requer prova com certificado próprio estável.

## Achados resolvidos na árvore revisada

### [P1 resolvido] O encerramento podia expirar antes do flush final

- **Spec:** seção 2: “Persistência deve ser confirmada por RPC `FlushForUpdateCommand` antes de instalar; shutdown aguarda prazo suficiente do servidor.”
- **Gatilho:** telemetria de encerramento lenta, por exemplo rede que deixa a requisição pendente até o timeout de 15 segundos, durante a instalação de update.
- **Situação anterior:** `cmd/server/main-server.go` iniciava o contexto de cinco segundos antes de telemetria de até 15 segundos, fazia o flush depois e ignorava seu erro. `emain/emain.ts` forçava saída após 15 segundos.
- **Correção revisada:** `cmd/server/main-server.go:84-91` agora inicia o contexto imediatamente antes de `FlushForUpdate`, aguarda flush concorrente e registra falha; telemetria vem depois. `emain/emain.ts:318-323` dá 30 segundos, superior aos cinco segundos do flush mais até 15 segundos da rede e 500 ms finais. O risco específico foi fechado por inspeção estática. A falha de flush durante o shutdown ainda é registrada em log e depende da barreira RPC pré-instalação para negar update caso a persistência anterior não tenha sido confirmada.

### [P1 resolvido] Nomes dos assets do GitHub impediam download do updater

- **Situação anterior:** o template `${productName}` gerava DMG e ZIP com `Force Terminal` no nome, mas `GitHubProvider.resolveFiles` de `electron-updater` substitui espaços por hífens na URL de download. Uma release publicada com nomes originais faria o update buscar outro asset e receber 404.
- **Correção revisada:** `electron-builder.config.cjs:52` define `mac.artifactName` com `${name}`, que resolve para `force-terminal`. No `app-builder-lib` instalado, `PlatformPackager.artifactPatternConfig` prioriza a configuração da plataforma sobre o template global. `scripts/force-release.mjs` e `scripts/force-verify-mac.sh` exigem o mesmo prefixo; o teste de release chama o `GitHubProvider` real e compara as URLs resolvidas aos nomes de ZIP/DMG. O feed gerado pelo builder é preservado e validado.

### [P1 resolvido] Falha nativa na instalação podia deixar editores congelados

- **Situação anterior:** com `autoInstallOnAppQuit=false`, `MacUpdater.quitAndInstall` inicia uma checagem nativa assíncrona. Uma falha nessa etapa emitia `error` depois do retorno de `quitAndInstall`, quando `setUserConfirmedQuit(true)` já estava ativo e as views permaneciam congeladas.
- **Correção revisada:** `emain/updater.ts:128-139` mantém as views da tentativa de instalação; ao receber erro, restaura o estado de quit, libera as views e retorna a `ready` com motivo. O teste `releases frozen editors after a native installation error and retries without downloading again` cobre essa transição. O `MacUpdater` instalado encaminha `nativeUpdater.error` ao emitter usado pelo handler (`node_modules/electron-updater/out/MacUpdater.js:18-21`).

## Achado adicional resolvido

### [P1 resolvido] Secrets recém-alterados podiam se perder no reinício ou fechamento

- **Gatilho:** salvar um secret e atualizar ou fechar antes de vencer o debounce de um segundo.
- **Situação anterior:** a barreira de update confirmava apenas arquivos, enquanto o escritor de secrets ainda podia estar pendente. No fechamento comum, o processo principal encerrava o RPC de criptografia imediatamente após sinalizar o servidor.
- **Correção revisada:** `pkg/secretstore/secretstore.go` serializa snapshots e confirma a geração mais recente antes de liberar a barreira. O RPC de update inclui esse flush e rejeita falhas. O shutdown do backend tenta gravar arquivos e secrets em paralelo sob o mesmo prazo de cinco segundos; o Electron fecha o RPC somente na saída final. Testes Go com detector de corrida passaram, incluindo concorrência, timeout, falha seguida de retry e um flush de arquivos lento que não impede a tentativa de persistir secrets.
- **Limite:** a criptografia e coexistência instaladas no macOS ainda precisam de teste. A evidência de runtime disponível é registrada em [RUNTIME-REPORT.md](RUNTIME-REPORT.md).

### [P2 resolvido] Retry após erro nativo acumulava callbacks de instalação

- **Gatilho:** iniciar a instalação, receber `nativeUpdater.error` durante a checagem do Squirrel.Mac e clicar novamente em tentar instalar.
- **Evidência:** `node_modules/electron-updater/out/MacUpdater.js:243-249` registra `nativeUpdater.on("update-downloaded", ...)` em cada `quitAndInstall` e não remove o listener após erro. Uma reprodução local só de leitura chamou `MacUpdater.prototype.quitAndInstall` duas vezes com um erro nativo entre elas; um único evento `update-downloaded` acionou `handleUpdateDownloaded` **duas vezes**. O handler Force em `emain/updater.ts:128-139` permite esse retry; o teste atual mocka `autoUpdater` e não exercita os listeners internos do `MacUpdater`.
- **Correção revisada:** `emain/updater.ts:93-101,158-171,259-268` compara os listeners nativos antes/depois da tentativa e remove somente os adicionados por ela quando há erro. O teste com emissor de eventos mantém o listener preexistente, dispara erro, faz retry e confirma uma só chamada nativa de instalação. `npx vitest run emain/updater.test.ts` passou 17/17 após essa alteração. A instalação real no M5 ainda precisa de smoke.

## Conferências sem achado adicional

- O perfil local resolve dados, configuração, cache, `userData` e `sessionData` sob `force-terminal` ou `force-terminal-dev`. Overrides `FORCE_TERMINAL_*` absolutos que apontem para perfis Wave conhecidos, inclusive por symlink no sistema em uso, são rejeitados. A migração de banco antigo retorna erro antes de abrir `~/.waveterm`.
- Os caminhos operacionais do helper remoto examinados para binário, integração de shell, sockets, jobs e logs usam `~/.force-terminal` ou nomes `/tmp/force-terminal-*`. O nome `waveterm.sock` dentro do diretório Force e os nomes de protocolo foram mantidos conforme o plano.
- A versão atual da preparação do updater congela a entrada nos renderers, libera o bloqueio em falhas, consulta os bloqueadores antes e depois do flush e detecta mudança do conjunto de views. `GetUpdateBlockersCommand` exige origem RPC `electron`; controladores de shell só são considerados ociosos quando renderer e estado de runtime concordam. Jobs ativos ou de estado desconhecido bloqueiam.
- As tarefas próprias `force:*` chamam o script Force; nele as variáveis legadas `WAVETERM_ENVFILE` e `WCLOUD_*` herdadas são removidas. Os comandos antigos `electron:*` permanecem como fluxo legado explícito e não foram tratados como execução da tarefa Force.
- O pipeline oficial só aceita tag `v` igual à versão do pacote, exige entradas de assinatura e notarização e valida assinatura Developer ID, Gatekeeper e staple do app no diretório de build, ZIP e DMG. O manifesto e feed exigem DMG, ZIP e respectivos blockmaps com hashes e tamanhos; a release continua draft até conferir também os hashes SHA-256 dos seis assets remotos. O runner `macos-15` é arm64 conforme a [tabela oficial dos runner images](https://github.com/actions/runner-images/blob/main/README.md). Não há achado adicional de publicação nesta leitura.
- Na revisão final delimitada, `pkg/wcloud/wcloud.go` permite bootstrap de desenvolvimento sem variáveis herdadas, valida apenas overrides explícitos HTTPS e os remove do ambiente após cache. Os defaults upstream continuam como decisão de escopo do plano. `scripts/force-build-mac.sh` limpa saídas de build antes do pacote e recria o scaffold ignorado pelo Git; não aponta para perfis ou dados do usuário.
- O pacote ad hoc usa assinatura ad hoc (`identity: "-"`), desabilita notarização e inclui marker que desabilita o updater oficial. `scripts/force-verify-mac.sh` confere assinatura em todos os modos, identidade/versionamento no `Info.plist`, configuração do feed e cache Force e presença ou ausência correta do marker em app, ZIP e DMG. A expectativa `force-terminal-updater` confere com o cálculo da versão instalada de `app-builder-lib` a partir de `package.json.name`.

## Validação e limites

- `go test ./pkg/updateguard`: passou. Após os últimos patches, `npx vitest run emain/updater.test.ts` passou 17 testes e `npm run force:release:test` passou nove, incluindo o `GitHubProvider` real. A integração final confirmou 27 testes Vitest, `npx tsc --noEmit`, suítes Go relevantes e compilação do backend Linux. O build completo de produção passou; após a última alteração somente no processo principal, esse alvo foi recompilado com a configuração do electron-vite e passou. Não foi executado teste de pacote macOS aqui.
- Revisão estática de `force-profile`, `emain-platform`, `emain-wavesrv`, `wavebase`, consumidores SSH/WSL, `updateguard`, `updateflush`, `wshserver/update.go`, bindings RPC e updater. Nenhum teste macOS foi executado neste host Linux.
- Permanecem **pendentes**, sem status de aceite: smoke real no Mac M5 para terminal/SSH/arquivos/reabertura/coexistência e marca no Finder/Dock; assinatura e notarização Developer ID; ciclo instalado vN → vN+1.
