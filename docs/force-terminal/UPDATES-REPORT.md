# Atualizador e preservação — Force Terminal

## Implementado

- O botão de atualização permanece visível nas barras horizontal e vertical. Expõe consulta, disponibilidade, download com progresso, pronto, adiamento, erro e limitação da versão de desenvolvimento. O botão é acessível por teclado e mostra o motivo do adiamento.
- A versão de desenvolvimento, incluindo pacote com `force-adhoc.json`, não consulta nem instala atualizações. O feed empacotado é aceito somente se `app-update.yml` apontar para GitHub `calneymgp/force-terminal`.
- Releases `community` usam certificado próprio estável, sem marker ad hoc e sem notarização; o updater Electron e suas guardas permanecem os mesmos. `official` é opção futura com Developer ID/notarização. O modelo de entrega é GitHub Releases, sem npm como canal de instaladores e sem App Store. A atualização real com certificado próprio ainda precisa de teste instalado no Mac.
- Consulta no início e a cada 600000 ms sem baixar. Clique manual consulta ou reutiliza pacote baixado, baixa explicitamente, prepara e reinicia sem confirmação adicional. Operações simultâneas compartilham uma execução. Download ou preparo com falha mantém o app aberto e permite nova tentativa.
- A preparação solicita resposta identificada de todos os renderers de abas e Builder carregados, com timeout de 10 segundos e validação do emissor. Preview, configuração, código e ambiente do Builder aguardam saves e confirmam que não há alterações pendentes. Abas com save malsucedido ou sem resposta permanecem carregadas no cache. A troca de workspace também espera a guarda.
- Durante o preparo de instalação, a entrada nas janelas é suspensa. O servidor verifica controllers/estado de shell e jobs, inclusive de abas descarregadas; um shell ocioso só é aceito com confirmação atual de renderer carregado. Em seguida, `FlushForUpdateCommand` confirma persistência. O servidor e o conjunto de views são verificados novamente antes da instalação. Falhas e adiamentos liberam as janelas. O timeout de shutdown do servidor aumentou para 30 segundos, cobrindo flush, telemetria e margem de encerramento.
- Enquanto o instalador nativo macOS conclui sua verificação, cliques adicionais não iniciam uma segunda instalação. Se ele sinalizar erro após `quitAndInstall()` retornar, o app desfaz a confirmação de saída, libera as janelas congeladas e conserva o download para nova tentativa. O listener de conclusão criado pela tentativa falha é removido sem tocar nos listeners preexistentes do Electron, evitando instalação duplicada na nova tentativa.
- A confirmação de persistência inclui arquivos e secrets. A gravação adiada de secrets é serializada e o flush espera a geração mais recente; falha ou prazo expirado impede o reinício. Um perfil que não utilizou secrets não inicializa nem lê o cofre nessa etapa. No fechamento comum, o backend tenta gravar os dois armazenamentos em paralelo dentro do prazo de cinco segundos, e o Electron mantém o RPC de criptografia disponível até a saída do servidor ou o limite de 30 segundos.

## Verificação

- `npx vitest run emain/updater.test.ts frontend/app/store/update-guard.test.ts`: cobre consulta inicial e intervalo de 600000 ms sem download, clique manual com consulta automática desativada ou ainda em configuração, cliques repetidos durante download, nova tentativa após falha de flush ou erro nativo assíncrono sem baixar novamente, progresso, bloqueio por terminal, timeout e resposta inesperada do renderer, congelamento/liberação, mudança de views, feed incorreto e pacote de desenvolvimento.
- `npx tsc --noEmit --pretty false`: compilação de tipos completa após correção dos mocks de preview.
- `go test -race -mod=readonly ./cmd/server ./pkg/secretstore ./pkg/filestore ./pkg/wshrpc/wshserver ./pkg/updateguard -count=1`: passou após a correção de secrets e do encerramento. Cobre escrita pendente, escrita concorrente, alterações durante flush, falha com retry, timeout, origem RPC e tentativa de persistir secrets mesmo se o flush de arquivos falhar ou demorar.

## Pendente em macOS

- Smoke real com terminal/SSH e editores, reabertura e coexistência com Wave.
- Build assinado e teste ponta a ponta de atualização de vN para vN+1, incluindo ausência de download antes do clique, reinício, persistência e adiamento de comando ativo.
