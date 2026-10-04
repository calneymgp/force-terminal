# Aceites das prioridades 0, 1a e 1b

Auditoria da árvore local em 4 de outubro de 2026. Host: Linux x86_64; Node 24 instalado. O alvo continua sendo macOS arm64 no MacBook M5. Código e testes locais não substituem os aceites no aparelho.

| Item | Evidência disponível | Prova ainda necessária |
|---|---|---|
| 0 — ambiente completo | Tarefas Force para build do frontend, Electron, servidor, helpers, schemas e scaffold; versões e lockfiles documentados; perfil temporário explícito; TypeScript completo e build de produção passaram | Preparar Node 22, Go, Task 3 e ferramentas Apple no M5; abrir app completo e gerar DMG/ZIP ad hoc |
| 0 — referência funcional | Electron real em perfil descartável Linux; comando local e leitura/escrita via RPC aprovados; roteiro reproduzível disponível | Terminal, SSH descartável, editores locais/remotos e reabertura no M5 |
| 1a — identidade | SVGs Raio Lunar preservados, variantes compacta/detalhada, nomes e metadados Force aplicados; fork correto | Conferir Finder, Dock, janelas, onboarding, About, topbar e instalador no pacote macOS |
| 1b A — isolamento | Resolução central de perfis antes do servidor/lock; overrides Force; defaults remotos compilados Force; migração Wave desativada; testes locais/remotos passaram | Force e Wave instalados simultaneamente, inclusive helpers SSH, secrets e identidade de criptografia, sem compartilhar dados |
| 1b B — atualização | Botão permanente e estado compartilhado; startup/timer de 10 minutos; download e instalação só por ação; IPC, preload, atoms e mocks alinhados; testes de concorrência, progresso e falhas passaram | Exercitar versão instalada assinada e feed publicado completo |
| 1b C — trabalho e persistência | Preparo de todas as views, timeout e bloqueio de abas sujas; estados ativo/desconhecido bloqueiam; flush de arquivos e secrets; shutdown com RPC vivo; testes Go com detector de corrida passaram | Editores reais, comando ativo, adiamento e persistência após atualização no M5 |
| 1b D — distribuição | Workflow macOS arm64, tag igual à versão, modos adhoc/community/official, DMG/ZIP/feed/blockmaps, validação de hashes e draft antes de publicar; política de assinatura própria sem notarização preparada | Executar workflow no GitHub, validar pacote ad hoc e, em etapa posterior autorizada, configurar certificado próprio para atualização |
| 1b — aceite final | Código e pipeline preparados | Instalar N e atualizar para N+1 pelo botão; comprovar versão, reinício, dados e adiamento com trabalho ativo |

## Estado externo conferido

A conferência posterior revalidou os mesmos limites externos: host Linux x86_64, nenhum workflow/execução/release Force no GitHub e os cinco nomes de secrets Apple ausentes. Não há processo de build macOS em andamento para aguardar.

- O remoto `origin` aponta para o fork público `calneymgp/force-terminal`; a branch remota `main` ainda está na referência upstream usada como base.
- As consultas de leitura ao GitHub retornaram zero workflows, execuções e releases. O workflow Force está na árvore local e não foi publicado ou executado nesta etapa.
- Nenhum dos cinco nomes exigidos de secrets Apple estava presente: `FORCE_MACOS_DEVELOPER_ID_P12_B64`, `FORCE_MACOS_DEVELOPER_ID_PASSWORD`, `FORCE_APPLE_ID`, `FORCE_APPLE_APP_SPECIFIC_PASSWORD` e `FORCE_APPLE_TEAM_ID`. Somente nomes foram consultados; valores de credenciais não foram acessados.
- Não há acesso confirmado ao M5 neste ambiente. Nenhum teste Linux é apresentado como validação macOS.

O teste Linux de secrets encontrou `safeStorage` indisponível já antes do encerramento, com `--password-store=basic`. Essa tentativa não comprovou persistência cifrada nem permitiu concluir que o fechamento causa a indisponibilidade. O diagnóstico e as verificações com adaptação exclusiva do teste estão em [RUNTIME-REPORT.md](RUNTIME-REPORT.md).

Com `basic_text` habilitado somente por um bootstrap temporário, o smoke real confirmou salvar → fechar imediatamente → reabrir → recuperar o valor fictício. A adaptação não entrou no produto. A prova cobre a rota Electron/backend e o flush em um ambiente de teste com criptografia disponível; o aceite Keychain continua aberto.

## Verificação adicional do helper macOS

A cross-compilação de `cmd/wsh/main-wsh.go` passou com `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -mod=readonly`, Go 1.26.8 e versão 0.14.5. A inspeção do arquivo confirmou executável Mach-O arm64 e os metadados Go confirmaram `darwin/arm64`. O binário foi gerado apenas em `/tmp`, fora dos artefatos de distribuição.

Essa evidência cobre a compilação do helper para o alvo; não comprova sua execução no Mac ou por SSH. O servidor usa CGO/SQLite e exige o toolchain macOS empregado pelo script oficial. Não foi substituído por uma variante com CGO desabilitado.

## Estado de conclusão

As prioridades **0 e 1a aguardam validação no M5**. A prioridade **1b permanece parcialmente implementada e sem aceite integral** até assinatura estável e atualização instalada real. Notarização é opcional no canal `official`; não bloqueia `community`. Na etapa inicial não houve commit, push ou release.

Direcionamento atual: o usuário escolheu somente um DMG de teste, sem criar certificado/configurar credenciais, com build no GitHub e download em página HTML pela rede Tailscale. Esse pacote ad hoc não ativa atualizações. A entrega do link não substitui os aceites de smoke e atualização instalada.

Para executar os aceites no aparelho, siga [MACOS.md](MACOS.md). Evidências de execução local e seus limites estão em [RUNTIME-REPORT.md](RUNTIME-REPORT.md); detalhes técnicos em [UPDATES-REPORT.md](UPDATES-REPORT.md), [ISOLATION-REPORT.md](ISOLATION-REPORT.md), [RELEASE-REPORT.md](RELEASE-REPORT.md) e [FOUNDATION-REVIEW.md](FOUNDATION-REVIEW.md).
