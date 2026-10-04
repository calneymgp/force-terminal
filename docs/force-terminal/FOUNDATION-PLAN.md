# Force Terminal — implementação de 0, 1a e 1b

Plano aprovado em 4 de outubro de 2026. Alvo inicial macOS arm64 (MacBook M5). Preserve as mudanças de marca já existentes. Não publicar releases, instalar credenciais nem alterar produção durante esta implementação.

Direcionamento posterior do usuário: seguir a distribuição do tamz-bot pelo GitHub, sem loja Apple. O canal padrão passa a ser `community`, assinado com certificado próprio estável, sem exigir conta Apple Developer ou notarização. `official` permanece opcional para Developer ID/notarização; `adhoc` continua reservado a testes sem updater. O Force permanece em Electron. O tamz-bot consultado usa Tauri/Rust, portanto seu mecanismo de assinatura do feed não é transplantado para o Electron.

## 1. Isolamento

Perfil de produção `force-terminal`, desenvolvimento `force-terminal-dev`. macOS: dados em `~/Library/Application Support/<perfil>`, configuração em `~/.config/<perfil>`, Electron `electron` e `session` dentro dos dados, cache em `~/Library/Caches/<perfil>`. Overrides `FORCE_TERMINAL_*` permitem teste temporário. Ignorar descoberta e overrides legados Wave; não importar dados. Propagar caminhos resolvidos ao backend pelos nomes internos WAVETERM existentes. Helper remoto compilado usa `~/.force-terminal`, incluindo binário, integração shell, sockets, logs e jobs; não tocar `~/.waveterm`. Preservar nomes técnicos e protocolos. Testar caminhos, overrides, coexistência e secrets.

## 2. Atualizador e preservação

Botão permanente em barras horizontal/vertical; estado inicial pendente, consulta, atualizado, disponível, download com percentual, pronto, instalação e erro. Consultar startup/600000 ms sem download; `autoDownload=false`, `autoInstallOnAppQuit=false`. Clique manual ignora intervalo e executa consultar/baixar/preparar/instalar/reiniciar sem segunda confirmação normal. Operação única entre janelas. Falhas mantêm app instalado. Versão dev informa limitação sem fingir disponibilidade.

Preparação guarda todos os editores/configurações carregados; espera salvamento e confirma dirty=false. Views com alterações não podem ser descartadas pelo cache. Falha, renderer ausente ou timeout 10s bloqueiam. Comando ativo ou estado de terminal desconhecido adiam; integração shell distingue prompt ocioso de comando, shell vivo sozinho não serve. `ready` com motivo permite tentar novamente, nunca matar comandos automaticamente. Persistência deve ser confirmada por RPC `FlushForUpdateCommand` antes de instalar; shutdown aguarda prazo suficiente do servidor. Domínio tarefas/notas fora do escopo, mas extensível pela guarda.

## 3. Ambiente e pipeline

Node22, Go conforme go.mod, Task3. Comandos dev/test-profile/package mac arm64 constroem Electron/frontend, wavesrv, helpers cross-platform exigidos, schema/scaffold. Sem .env ou endpoints Wave automáticos e sem limpeza de dados Wave. Corrigir mocks TypeScript existentes. Pipeline próprio mac arm64: npm ci, build Go readonly, geração controlada; DMG+ZIP+manifest+blockmaps completos; versão tag igual package; hashes e arquitetura validados. Workflow manual gera artefato ad hoc de desenvolvimento sem feed publicado. Tags usam `community` por padrão, com certificado próprio e fingerprint fixo conferidos, sem notarização. `official` é opt-in e conserva Developer ID/notarização. Secrets só CI; draft completo vira release automaticamente após validação. Desativar publicação S3/Snap/Winget/Wave e bump via app Wave. Certificado próprio não é assinatura ad hoc: deve permanecer o mesmo entre N e N+1, e o ciclo real Electron precisa de teste no Mac.

## 4. Aceite e documentação

Testes comportamento isolamento, updater timer/clique/concorrência/falhas/progresso, guardas/save/flush; build prod e tsc completo. Smoke real M5: terminal/SSH descartável/arquivos local remoto/reabrir/coexistência e marca Finder Dock UI. Após assinatura vN->vN+1 sem download antes clique, restart/persistência/adiamento. NÃO marcar smoke mac, assinatura ou update real como concluídos sem execução. Atualizar BRAND/EVOLUTION e roteiro Mac com evidências e pendências.
