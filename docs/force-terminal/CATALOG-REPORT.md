# Catálogo Force — validação em 4 de outubro de 2026

O código entrega projetos locais/SSH e perfis reutilizáveis; a execução e a retomada de agentes continuam nas prioridades 3–4. O DMG servido anteriormente contém a simplificação do onboarding. O novo pacote do catálogo ainda está em preparação.

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

O workflow [force-smoke-mac.yml](../../.github/workflows/force-smoke-mac.yml) agora executa também esse smoke do catálogo no aplicativo extraído do DMG. O resultado nativo será registrado após a execução; o M5 físico continua com aceite próprio.
