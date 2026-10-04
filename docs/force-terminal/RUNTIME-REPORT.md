# Smoke de runtime Force Terminal

## Pacote macOS arm64: aprovado no GitHub

O [build 37224909810](https://github.com/calneymgp/force-terminal/actions/runs/37224909810), fonte `f824866a6035a17146779879e9f659d646e0f58d`, produziu o DMG ad hoc 0.14.5 com as páginas iniciais apontando para o fork Force. O [smoke 37225675703](https://github.com/calneymgp/force-terminal/actions/runs/37225675703) baixou esse artefato, conferiu o manifesto, montou o DMG somente para leitura, copiou o app sem alterar sua assinatura e validou `codesign --verify --deep --strict` antes de executá-lo em macOS arm64.

O relatório `force-terminal-mac-runtime-smoke/report.json` registra `passed: true` para seis verificações:

- Renderer real, nome Force, caminhos do perfil descartável e updater ad hoc desabilitado.
- Terminal local executando `printf`, com a saída identificada em linha distinta do comando digitado.
- Leitura e escrita de arquivo pelo renderer e backend reais, com conteúdo confirmado no disco.
- Editor visual Monaco: digitação e Cmd+S gravaram um conteúdo diferente, conferido por RPC e leitura do arquivo.
- Encerramento pelo evento nativo de quit do macOS, com saída 0 e `shutdown complete` sem erro de flush.
- Reabertura do mesmo perfil: mesma aba, metadado gravado no banco e arquivo editado recuperados; segundo encerramento também normal.

O perfil de teste usa Zsh `/bin/zsh`, telemetria e consultas automáticas desativadas, renderização DOM do terminal e confirmação de quit desativada. São configurações do perfil descartável, não mudanças dos defaults do aplicativo. O ambiente repassado ao app contém somente variáveis de sistema necessárias e os caminhos/endpoints do teste. Nenhum secret de usuário foi fornecido ao app; este teste não exercita a persistência de secrets ou o Keychain. App e backend encerraram normalmente e o perfil temporário foi removido. O artifact inclui capturas de onboarding, terminal/editor e reabertura, sem logs brutos ou ambiente do runner.

As execuções exploratórias revelaram limites do teste: SIGTERM não acionou o handler Node no app macOS observado; o encerramento aprovado usa Apple Events, que exercitam o quit nativo. O primeiro seletor do Monaco esperava textarea, mas a versão instalada habilita EditContext nativo no Chromium atual; o teste passou a selecionar a superfície de entrada ativa. Em uma execução com o Bash do runner, o terminal permaneceu sem saída durante 45 segundos; outras execuções Bash passaram. Esse resultado permanece inconclusivo, sem declaração de correção do Bash ou dos sinais POSIX.

**Limites:** este smoke comprova execução do pacote macOS arm64 no runner, não no M5 físico. Finder/Dock, quarentena/Gatekeeper após download pelo navegador, SSH, coexistência instalada com Wave, Keychain e atualização assinada N → N+1 continuam sem aceite. A extração via Actions não reproduz a quarentena do download no Mac do usuário.

## Desenvolvimento Linux x64

Executado em 2026-10-04 no host Linux x64 com Xvfb, Electron 41.1.0, `dist/main`, `dist/frontend` e os binários locais `wavesrv.x64` e `wsh-0.14.5-linux.x64`. Este teste usa a versão de desenvolvimento (`app.isPackaged === false`); não valida macOS, pacote instalado ou atualização real.

## Isolamento do teste

- As três variáveis `FORCE_TERMINAL_DATA_HOME`, `FORCE_TERMINAL_CONFIG_HOME` e `FORCE_TERMINAL_CACHE_HOME` apontaram para diretórios descartáveis sob `/tmp/force-runtime-test-*`. O arquivo `settings.json` foi criado antes do lançamento com `telemetry:enabled: false` e `autoupdate:enabled: false`; os dois valores foram lidos de volta ao final.
- Os endpoints `WCLOUD_ENDPOINT`, `WCLOUD_PING_ENDPOINT` e `WAVETERM_WAVEAI_ENDPOINT` foram direcionados a `https://127.0.0.1:1`, sem serviço local. Nenhuma ação de AI, login, atualização remota ou SSH foi iniciada.
- O primeiro lançamento, sem `WCLOUD_ENDPOINT`, iniciou `wavesrv` e encerrou imediatamente com `invalid wcloud endpoint, WCLOUD_ENDPOINT not set or invalid`. Esse era um requisito legado de desenvolvimento. O segundo lançamento com endpoint local permitiu o smoke. Após o teste, o bootstrap foi corrigido para usar os endpoints padrão quando não há override; overrides explícitos inválidos continuam rejeitados. Os testes de `pkg/wcloud` reproduziram a falha e passaram após a correção, sem rede. O smoke gráfico usou o binário anterior à correção.

## Resultado observado

| Verificação | Resultado |
| --- | --- |
| Inicialização do backend | `wavesrv` abriu serviços HTTP e WebSocket em `127.0.0.1`, criou o workspace inicial e sinalizou prontidão. |
| Janela real | CDP encontrou a janela `Force Terminal - T1`, com documento carregado, preload `window.api` disponível e interface renderizada. |
| Caminhos do perfil | `window.api.getDataDir()` e `getConfigDir()` retornaram os diretórios descartáveis Force configurados. Os arquivos de banco, sessão, helper e socket surgiram sob esse perfil. |
| Atualizador em desenvolvimento | `window.api.getUpdaterStatus()` retornou `dev-disabled`; o botão `Updates unavailable in development` estava presente e desabilitado. |
| Terminal local | Com foco no `xterm`, uma entrada enviada por CDP executou `printf`; a marca de teste apareceu em linha de saída própria, além da linha de comando. |
| Arquivo local via RPC real | O renderer leu um arquivo descartável em `/tmp`, chamou `FileWriteCommand` e o leu novamente por `FileReadCommand`; a segunda leitura retornou o conteúdo novo. O arquivo no disco foi lido de volta com o mesmo conteúdo. |
| Guarda de origem da atualização | `FlushForUpdateCommand` chamado pelo `TabRpcClient` do renderer foi recusado com `application update preparation requires Electron coordinator`, como exige a guarda do servidor. |
| Encerramento | Interrupção do wrapper Xvfb do teste encerrou Electron, `wavesrv` e shell filho; a busca pelos PIDs e pelo diretório temporário nos argumentos não encontrou processos remanescentes. |

## Limites

- A escrita de arquivo validou o caminho renderer → RPC → sistema de arquivos. O editor visual, seu save e a reabertura pela UI não foram exercitados neste smoke.
- A origem Electron positiva de `FlushForUpdateCommand`, a preparação de todos os renderers, o download/instalação e o reinício do atualizador não foram exercitados. O estado `dev-disabled` impede esse fluxo neste build.
- SSH remoto e o host macOS arm64 alvo não estavam disponíveis. Este resultado não atesta coexistência com Wave no Mac, empacotamento assinado nem atualização de vN para vN+1.

## Tentativa de persistência de segredo no encerramento normal (Linux x64)

Em 2026-10-04, após recompilar `dist/main` e `dist/bin/wavesrv.x64`, foi executado um smoke isolado com Electron 41.1.0, Xvfb e três diretórios Force temporários (`data`, `config`, `cache`) sob `/tmp`. `HOME` foi preservado. O perfil recebeu `telemetry:enabled: false` e `autoupdate:enabled: false`; os endpoints de cloud, ping e AI foram apontados para `https://127.0.0.1:1`. O Electron iniciou com `--password-store=basic`, sem acesso ao cofre do sistema. Nenhum dado ou segredo do Wave foi usado.

Com `--password-store=basic` sozinho, uma chamada `ElectronEncryptCommand` antes do encerramento já retornou `encryption is not available`. Em duas tentativas, `SetSecretsCommand` para um valor fictício foi confirmado pelo renderer real e `SIGTERM` foi enviado 1 ms e 0 ms depois, respectivamente, ao launcher que o encaminhou ao Electron. O handler do Electron chama `app.quit()`. O Electron encerrou com código 0, mas o backend registrou `shutdown storage flush failed: failed to encrypt secrets: encryption is not available`. A indisponibilidade já existia antes do quit; essas tentativas não medem uma regressão causada pelo flush.

Para exercitar a rota de persistência sem cofre do sistema, uma execução posterior usou um bootstrap JavaScript temporário fora do repositório que chamou `safeStorage.setUsePlainTextEncryption(true)` antes de carregar o `dist/main` real. O backend selecionado foi `basic_text`; esse fallback é inseguro e serve somente para o teste descartável. A sondagem de cifra antes do quit passou. `SetSecretsCommand` foi confirmado e `SIGTERM` foi enviado diretamente ao PID principal do Electron 0 ms depois. Electron e backend saíram normalmente, sem erro de flush; `secrets.enc` existia com 176 bytes, sem leitura nem impressão do conteúdo. Após reabrir o mesmo perfil, `GetSecretsCommand` retornou o valor fictício esperado em comparação booleana. A segunda saída também foi normal.

Os processos criados pelo teste e os diretórios temporários foram removidos após cada tentativa. A execução bem-sucedida comprova a rota renderer → RPC → flush de shutdown → arquivo → reabertura com `basic_text` isolado. Não comprova confidencialidade nesse backend, funcionamento de um armazenamento seguro disponível nem comportamento do pacote macOS.
