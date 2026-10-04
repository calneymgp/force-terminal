# Implementação — FOUNDATION-PLAN.md

Base: checkout main com alterações anteriores de rebranding preservadas. Branch de trabalho `codex/force-foundation-20261004`. Host de execução Linux x64, Node24 instalado, Go e Xvfb disponíveis, Task ausente; validação real macOS não é possível neste host.

## Revisão prévia e decisões

| Entregas/interface | Verificação e decisão |
|---|---|
| Isolamento / ambiente | Caminhos separados devem preceder smoke. Namespace remoto compilado Force obrigatório. |
| Atualizador / persistência | UI aguarda preparo dos renderers e RPC FlushForUpdateCommand antes de instalação. Falha bloqueia. |
| Pipeline / ambiente | Build dev ad hoc sem feed; após direcionamento tamz-bot, tags `community` exigem certificado próprio estável e conjunto completo; Developer ID/notarização somente no modo opcional `official`. |
| Atualizador / pipeline | Apenas GitHub Force; pacote mac ZIP necessário para update. |
| Marca / código | Marca já aplicada; não redesenhar e preservar copyrights. |

Ruling: usar checkout existente em branch própria; não criar outro worktree que perderia os arquivos não versionados e a marca aprovada.
Ruling inicial: executar partes independentes com ownership exclusivo em paralelo, conforme instrução do usuário; integração/revisão final pela raiz. A etapa inicial não fez commits/push/publicação. No direcionamento posterior, o usuário solicitou build no GitHub e escolheu apenas DMG de teste com página HTML privada via Tailscale; isso autoriza enviar o código e executar o workflow manual. Não criar certificado/configurar credenciais ou publicar release estável nessa entrega.
Ruling: evidência mac M5, Keychain e atualização assinada permanece pendente; a implementação não substitui o smoke exigido.

## Entregas

- Isolamento local/remoto: implementado. Perfis Force são resolvidos no bootstrap, com validação dos overrides; defaults compilados do helper usam `.force-terminal`; migração Wave desativada. Ver [ISOLATION-REPORT.md](ISOLATION-REPORT.md).
- Atualizador/UI/guardas: implementado. Botão permanente, consulta sem download automático, ciclo manual coordenado, saves de todas as views, bloqueadores de terminal/jobs e confirmação de flush. Ver [UPDATES-REPORT.md](UPDATES-REPORT.md).
- Ambiente/pipeline: implementado. Tarefas macOS arm64, pacote ad hoc sem feed e fluxo por tag com assinatura estável, draft e validação local/remota. O direcionamento posterior adiciona `community` sem notarização; `official` permanece opcional. Ver [MACOS.md](MACOS.md) e [RELEASE-REPORT.md](RELEASE-REPORT.md).
- Flush backend, integração e documentação: implementados. A revisão corrigiu o prazo de shutdown consumido antes do flush. O smoke revelou dependência de endpoint legado em dev; o bootstrap agora funciona sem esses overrides.

## Evidência integrada

- TypeScript completo e build de produção passaram no host Linux.
- Os 27 testes do perfil/updater/guardas passaram, incluindo timer, cliques concorrentes, clique durante configuração inicial, retry após falha do flush e erro nativo de assinatura sem manter janelas congeladas ou acumular chamadas de instalação.
- Os nove testes de release passaram, incluindo resolução real do GitHub provider com nomes sem espaços, feed do builder preservado, artefatos ausentes e hashes inválidos.
- Suítes Go de persistência, bloqueadores, origem RPC, perfis e caminhos locais/remotos passaram. O teste de bootstrap cloud sem overrides também passou sem rede.
- Regeneração de schema não produziu diff; configuração YAML, sintaxe dos scripts e `git diff --check` passaram. O trecho de pré-requisitos do workflow foi exercitado localmente, sem o teste de arquitetura do Mac: tag divergente e credenciais de assinatura ausentes bloquearam com saída 1.
- Electron real abriu com perfil temporário sob Xvfb; terminal e leitura/escrita de arquivo via RPC funcionaram. Ver [RUNTIME-REPORT.md](RUNTIME-REPORT.md).
- A verificação posterior cross-compilou o helper `wsh` para darwin/arm64, com Go 1.26.8 e as opções do script Force; o executável Mach-O arm64 foi confirmado. Não é evidência de execução macOS nem substitui o build do servidor CGO no Mac.
- Revisão independente e integração final: [FOUNDATION-REVIEW.md](FOUNDATION-REVIEW.md).

Verificação final após a última correção do updater: TypeScript completo e os 27 testes passaram novamente. O processo principal foi recompilado em modo produção usando a configuração resolvida do electron-vite; frontend e preload já haviam passado no build completo e não mudaram nessa correção. O backend Linux foi recompilado com as correções de shutdown e bootstrap. Essas evidências não substituem o pacote ou a execução macOS.

## Aceites ainda abertos

A auditoria posterior identificou a gravação adiada de secrets fora da barreira de persistência. Essa lacuna foi corrigida no RPC de update e no fechamento comum: escrita serializada por geração, confirmação do snapshot mais recente, tentativa paralela dos dois armazenamentos no shutdown e RPC de criptografia mantido até a saída final. As cinco suítes Go afetadas passaram com detector de corrida; o backend Linux e o processo principal foram recompilados. Consulte [quadro de aceites](ACCEPTANCE-AUDIT.md) para o estado de cada entrega.

O smoke adicional confirmou salvar um secret fictício, fechar imediatamente (0 ms após o RPC) e recuperá-lo ao reabrir o mesmo perfil. Foi necessário habilitar `basic_text` em um bootstrap temporário exclusivo do teste, porque o `safeStorage` do Linux estava indisponível antes do fechamento. O produto não recebeu esse fallback; a prova confirma a rota e o flush quando a criptografia está disponível, e não valida Keychain macOS. Os processos e perfis descartáveis foram removidos.

O host Linux não comprova execução no M5, assinatura macOS, marca no Finder/Dock, SSH remoto, Keychain ou coexistência instalada com Wave. O workflow foi implementado e validado localmente, mas não executado no GitHub nesta etapa. Para o canal `community`, faltam configurar o certificado próprio Force e comprovar a atualização instalada N → N+1; conta Apple e notarização deixam de ser requisitos desse canal. Por isso, 0 e 1a aguardam aceite no M5 e 1b não está integralmente concluído. Não houve commit, push ou publicação de release.
