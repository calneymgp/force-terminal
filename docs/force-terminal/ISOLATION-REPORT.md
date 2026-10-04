# Relatório de isolamento Force Terminal

## Implementação

- `emain/force-profile.ts` resolve perfis separados `force-terminal` e `force-terminal-dev`. Em macOS, dados ficam em `~/Library/Application Support/<perfil>`, configuração em `~/.config/<perfil>` e cache em `~/Library/Caches/<perfil>`. Overrides `FORCE_TERMINAL_DATA_HOME`, `FORCE_TERMINAL_CONFIG_HOME` e `FORCE_TERMINAL_CACHE_HOME` exigem caminhos absolutos e rejeitam as homes Wave conhecidas e symlinks que levem a elas, incluindo `~/.config`, `~/.local/share`, XDG e AppData do Windows.
- `emain/emain-platform.ts` ignora a descoberta legada `WAVETERM_HOME` e os overrides Wave para escolher os caminhos locais. Define `userData` e `sessionData` do Electron dentro dos dados Force antes de `app.whenReady()`.
- `emain/emain-wavesrv.ts` envia os caminhos Force ao servidor pelos nomes internos `WAVETERM_DATA_HOME`, `WAVETERM_CONFIG_HOME` e `WAVETERM_CACHE_HOME`. `pkg/wavebase/wavebase.go` mantém o cache interno após remover a variável do ambiente filho e usa os nomes Force nos defaults de produção e desenvolvimento.
- O helper remoto usa `~/.force-terminal` para binário, socket, jobs e integração shell. Sockets e logs temporários usam o prefixo `/tmp/force-terminal-`, incluindo jobs, conexão SSH, log de connserver e dump SIGUSR1. Nomes técnicos de protocolo como `waveterm.sock` e `WAVETERM_*` foram preservados. Produção e desenvolvimento compartilham a home do helper remoto, conforme o plano.
- `pkg/wcloud/wcloud.go` permite bootstrap de desenvolvimento sem `WCLOUD_ENDPOINT` ou `WCLOUD_PING_ENDPOINT`, usando os endpoints padrão existentes. Overrides explícitos, inclusive vazios, só são aceitos quando usam HTTPS com host válido; nenhum teste faz chamada de rede.

## Evidência

- `npx vitest run emain/force-profile.test.ts`: 7 testes passaram. Eles cobrem os caminhos macOS de produção/desenvolvimento, overrides de teste, rejeição de caminhos relativos/Wave, raízes XDG/AppData e symlinks.
- `go test ./pkg/wavebase ./pkg/remote/... ./pkg/shellexec ./pkg/wshutil ./pkg/wshrpc/wshremote ./pkg/wslconn ./pkg/util/sigutil ./cmd/wsh/...`: passou. Os testes novos cobrem destinos remotos, sockets/logs temporários e o cache Force de produção/desenvolvimento e seu override interno.
- `go test ./pkg/wcloud -run TestDevBootstrap -count=1`: passou. Sem overrides, o teste havia falhado no bootstrap; outro teste havia mostrado que um ping override HTTP inválido era aceito.
- Os testes novos falharam antes das alterações pelos caminhos `~/.waveterm`, `/tmp/waveterm-<uid>`, cache `waveterm` e symlink do perfil padrão.
- `npx tsc --noEmit --pretty false` ainda falha em mocks existentes de `frontend/preview` (campos `version`, `buildtime`, `getPathForFile` e `numthreads`), sem erros relatados nos arquivos de isolamento. Esses arquivos estão fora deste trabalho.

## Limitações

- O smoke real no macOS arm64, coexistência com Wave instalado, SSH/WSL remoto e a execução de um pacote instalado dependem de ambiente próprio e ainda não foram executados.
- A validação de symlink de caminhos Windows não foi exercitada; os testes de resolução rodam em Linux com casos macOS simulados.
- O caminho `pkg/wstore/wstore_dboldmigration.go` ainda contém a referência histórica `~/.waveterm/waveterm.db`, fora do escopo de arquivos autorizado. Uma busca por `TryMigrateOldHistory` encontrou apenas sua definição, sem chamada de startup; vale removê-la ou isolá-la antes de reativar essa migração.
