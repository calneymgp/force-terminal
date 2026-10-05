# Catálogo Force — validação em 4 de outubro de 2026

O código e o DMG de teste entregam projetos locais/SSH e perfis reutilizáveis; a execução e a retomada de agentes continuam nas prioridades 3–4. A instalação mantém apenas o primeiro popup, sem animação ou tour.

## Evidência local

- Backend: `go test -mod=readonly -count=1 ./pkg/service/forceservice ./pkg/waveobj ./pkg/wstore ./pkg/service` passou. `waveobj` e `service` não possuem testes próprios.
- Contratos: schemas e Go derivados não mudaram; a segunda geração TypeScript não produziu alterações.
- `npx tsc --noEmit`, `npm run build:prod` e compilação do servidor real passaram.
- [Smoke real](../../scripts/force-catalog-smoke.mjs): oito checks aprovados em Electron e backend reais, com dados Force temporários no Linux. Resultados locais: `/tmp/force-catalog-smoke-final-20261004/report.json`.

O smoke criou três projetos (dois locais e uma referência SSH), salvou perfis DevOps/ETL/Marketing, rejeitou uma pasta inexistente sem descartar o formulário e comprovou o bloqueio de reinício pelo IPC real de preparo de atualização. Envios repetidos criaram um único cadastro; falha simulada ao salvar seleção manteve o projeto anterior selecionado. Tab/Shift+Tab permaneceram no editor. Janelas com 1440 e 900 pixels comportaram o painel sem sobreposição. Uma edição originada na segunda janela atualizou ambas. IDs, versões, arquivamento/reativação e seleção sobreviveram ao encerramento normal e à reabertura.

Testes adicionais do serviço simularam resposta perdida e retry após reabrir o banco: a mesma chave devolveu o objeto original, sem novo cadastro, update ou evento. Payload divergente, objeto editado/arquivado e chave inválida foram rejeitados. Não foi testada uma corrida entre duas primeiras criações com a mesma chave.

## Limites

Salvar projeto SSH guarda somente a referência; os testes não abriram conexão remota. Nenhum CLI de agente foi iniciado nem suas conversas ou credenciais foram lidas. O perfil configurado usa diretórios Force temporários; isso não substitui a validação de coexistência, SSH e Keychain no aparelho. A telemetria veio desativada pelo novo padrão, sem override no teste.

O novo fluxo ainda utiliza abas e terminais herdados. Perfil é configuração, não instância executando. Desligamento abrupto, reboot, conversa de CLI, reconexão do agente e atualização assinada N→N+1 não estão comprovados nesta entrega.

## Pacote macOS entregue

O [build 37246104954](https://github.com/calneymgp/force-terminal/actions/runs/37246104954) empacotou o commit `f0c935a0a38508fa091ba7dfb6cbe0abfd84939e`. Manifesto, referências, arquitetura e hashes foram conferidos pelo workflow e novamente após baixar os artefatos.

O [smoke 37247460436](https://github.com/calneymgp/force-terminal/actions/runs/37247460436) passou em Darwin arm64 com o aplicativo extraído do mesmo DMG: sete checks de onboarding, terminal, arquivos/editor, quit e reabertura; mais os oito checks do catálogo descritos acima. O roteiro foi ajustado no commit `603b780d` para reconhecer o marcador de execução mesmo quando o terminal quebra uma linha; a tentativa anterior falhou nessa comparação, com o comando já executado. O pacote não precisou de alteração para essa correção do teste.

Distribuição privada: [página com botão](https://calneyserver.tail802eab.ts.net:8443/force-terminal/), build `37246104954`. DMG ad hoc `0.14.5`, 201196571 bytes, SHA-256 `d82d47f8b816a978acf2e5c72059c6dad00eefae3ab503c7b3fae3de73b09d5a`. HTTP 200, range 206, trailer UDIF `koly` e o hash do arquivo servido foram conferidos. Nenhum certificado/secret, release estável, porta ou Funnel foi criado.

O M5 físico, SSH real, execução/resume de CLI e atualização assinada continuam com aceites próprios pendentes.
