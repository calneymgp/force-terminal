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

## Pendências de execução

- Build/empacotamento macOS arm64 em runner: concluído conforme execução acima. Instalação e abertura no M5: pendentes.
- Certificado próprio estável Force, fingerprint e assinatura nos apps do build/ZIP/DMG: pendentes de configuração e execução CI. Developer ID, notarização e Gatekeeper são verificações adicionais do modo opcional `official`.
- Smoke terminal, SSH, arquivos, reabertura, coexistência Force/Wave e marca Finder/Dock/UI: pendente de Mac M5.
- Update instalado `vN → vN+1`, persistência e adiamento por comando: pendente de duas releases assinadas.

Preencha com links da execução do Actions e versões efetivamente testadas quando esses passos ocorrerem. Não registre secrets.
