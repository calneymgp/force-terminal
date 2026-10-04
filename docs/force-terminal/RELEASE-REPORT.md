# Relatório de release — macOS arm64

Data de preparação: 4 de outubro de 2026. Plataforma local de implementação: Linux x64.

Direcionamento posterior: canal `community` padrão pelo GitHub, seguindo a experiência tamz-bot, com certificado próprio estável e sem notarização. `official` com Developer ID fica opcional. A inspeção do tamz-bot confirmou Tauri/Rust, release pública `v0.5.7`, instaladores, feed e pacote do updater; detalhes em [DISTRIBUTION.md](DISTRIBUTION.md). Não houve publicação de Force nem criação/cópia de credenciais nesta adaptação.

## Evidências locais

- `npm run force:release:test`: 9 testes passaram (incluem URLs resolvidas pelo `GitHubProvider` real, pacote completo, blockmap ausente, hash errado, metadados do feed, versão divergente/pacote alterado, feed ausente, preservação do feed do builder e digest remoto do draft).
- `bash -n` nos dois scripts de build/verificação, `node --check` nos scripts JavaScript, parse YAML do Taskfile/workflow e `git diff --check`: concluídos sem erro.
- Configuração de Electron aponta para DMG e ZIP arm64 e mantém app ID, nome e ícone Force Terminal.
- Workflow manual e fluxo por tag preparados, sem executar release nem usar credenciais.
- Pacote macOS configurado para recriar apenas saídas de build no repositório; verificador preparado para comparar bundle ID, nome, versão, provider/cache, marcador ad hoc e assinatura nos apps do build, ZIP e DMG. Esses checks dependem de execução macOS.
- Nome de artefato macOS ajustado para `force-terminal-darwin-arm64-<versão>` sem espaços; fixture exercita a URL gerada pelo `GitHubProvider` instalado. `latest-mac.yml` é responsabilidade do electron-builder e é apenas validado.
- Gates configurados no workflow: geração controlada sem diff, TypeScript completo, testes Vitest de perfil/updater/guardas, testes Go de persistência/isolamento, comparação dos digests SHA-256 dos assets remotos do draft. Os gates de build e artefatos ad hoc passaram no macOS CI conforme execução abaixo; a publicação assinada e a conferência do draft remoto continuam pendentes.

## Primeiro build macOS executado

- [Run 37221443395](https://github.com/calneymgp/force-terminal/actions/runs/37221443395) do workflow `Force Terminal DMG de teste`: **success** em 4 de outubro de 2026, runner macOS arm64 e Node 22, fonte `60a3f5eb4ab1fd2b551af50f28dddd1ffbf63c3b`.
- Geração sem diff, TypeScript completo, 27 testes Vitest, 14 testes de distribuição e suítes Go passaram no runner. Backend/helper/scaffold/frontend/Electron completos foram compilados e empacotados.
- O verificador macOS montou o DMG, extraiu o ZIP e conferiu arquitetura, metadados Force, provider/cache, marker ad hoc e `codesign --verify --deep --strict` nas três cópias do app. O conjunto DMG/ZIP/feed/blockmaps e manifesto foi validado antes do upload.
- Artifact `force-terminal-mac-arm64-adhoc` (id `11309509403`) foi baixado; o manifesto SHA-256 foi validado novamente no servidor. O DMG tem **201174599 bytes** e SHA-256 `379e441240fe36edb43a8e5031eca3785ed61e6d6dcd24cf35cc9c21f7d1d6d8`.
- O instalador foi entregue por página HTML na rede Tailscale, conforme escolha do usuário. Nenhuma release estável ou credencial foi criada. `main` e a branch Force já contêm a implementação; a execução não comprova abertura no M5 ou atualização instalada.

## Pacote atualizado e executado

O [build 37224909810](https://github.com/calneymgp/force-terminal/actions/runs/37224909810) passou com a correção das páginas iniciais Force. O [smoke 37225675703](https://github.com/calneymgp/force-terminal/actions/runs/37225675703) aprovou abertura, Zsh, arquivos por RPC, editor visual/Cmd+S, quit nativo e persistência na reabertura do app extraído desse DMG. O instalador no link Tailscale foi substituído por esse pacote: **201174686 bytes**, SHA-256 `b9ea8cb16e88c162328c1a1161b852a9e20079ac2f046c644e1fa402faee327e`. A versão continua 0.14.5, ad hoc e sem updater; não houve publicação de release. Detalhes e limites em [RUNTIME-REPORT.md](RUNTIME-REPORT.md).

## Revisão com boas-vindas simplificadas

O [build 37241487184](https://github.com/calneymgp/force-terminal/actions/runs/37241487184), fonte `81da701bdd5769f9bc9a5af323d7522f20eaec21`, passou todos os gates e gerou o pacote com somente o primeiro popup. O [smoke 37242225109](https://github.com/calneymgp/force-terminal/actions/runs/37242225109) aprovou sete verificações, incluindo Continue direto ao app e reabertura sem onboarding. Esse DMG substituiu o instalador no mesmo botão Tailscale: **201180258 bytes**, SHA-256 `5c8fb9ddf3b01c90a961e5272065462ca0b04f3093f4ed55e168432f520e4498`. Manifesto, página/botão, HTTP 200 e range 206 foram conferidos no servidor. A versão permanece 0.14.5 ad hoc, sem publicação de release ou atualização automática.

## Pendências de execução

- Build/empacotamento macOS arm64 em runner: concluído conforme execução acima. Instalação e abertura no M5: pendentes.
- Certificado próprio estável Force, fingerprint e assinatura nos apps do build/ZIP/DMG: pendentes de configuração e execução CI. Developer ID, notarização e Gatekeeper são verificações adicionais do modo opcional `official`.
- Smoke terminal, SSH, arquivos, reabertura, coexistência Force/Wave e marca Finder/Dock/UI: pendente de Mac M5.
- Update instalado `vN → vN+1`, persistência e adiamento por comando: pendente de duas releases assinadas.

Preencha com links da execução do Actions e versões efetivamente testadas quando esses passos ocorrerem. Não registre secrets.
