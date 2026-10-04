# Distribuição Force pelo GitHub

Direcionamento do usuário em 4 de outubro de 2026: adotar a experiência de distribuição do tamz-bot, sem App Store e sem tornar conta Apple Developer um bloqueio inicial.

## Modelo verificado no tamz-bot

O código está em `/home/calney/Labfy/tamz-bot`. `package.json` usa Tauri 2, Solid/TypeScript e Rust; não é Electron e o pacote é privado para npm. `scripts/release.sh` roda testes, versiona, cria tag e dispara o workflow no repositório público [tamz-bot-releases](https://github.com/calneymgp/tamz-bot-releases).

O [workflow de release](https://github.com/calneymgp/tamz-bot-releases/blob/main/.github/workflows/release.yml) gera DMG universal e instalador Windows, assina o app Mac com certificado próprio, assina o updater Tauri com minisign, carrega arquivos em draft e publica o conjunto completo. Não exige Developer ID ou notarização. A release `v0.5.7` consultada contém instaladores, `latest.json`, pacote de atualização e suas assinaturas.

`src-tauri/tauri.conf.json` aponta o updater para `releases/latest/download/latest.json`. `src-tauri/src/commands.rs` implementa consulta, download/instalação e `app.restart()`. A consulta automática é ao abrir e a cada quatro horas; o clique no botão instala a versão encontrada.

## Aplicação ao Force

O Force permanece Electron + React/TypeScript + Go. O modelo de entrega é o mesmo: versão/tag → runner macOS → pacotes → GitHub Releases → botão de atualização. As dependências npm são ferramentas de desenvolvimento; o usuário baixa o instalador, sem instalar npm.

| Modo | Assinatura | Notarização Apple | Destino e updater |
|---|---|---|---|
| `adhoc` | Ad hoc, para teste | Não | Artefatos manuais; updater indisponível |
| `community` — padrão por tag | Certificado próprio estável Force | Não | GitHub Releases; updater nativo Electron, pendente de prova instalada |
| `official` — opcional | Developer ID Application | Sim | GitHub Releases com verificações Apple adicionais |

No Force, `electron-updater` utiliza DMG + ZIP, `latest-mac.yml` e blockmaps; não utiliza o `latest.json` nem a chave minisign do Tauri. A consulta permanece ao abrir e a cada dez minutos, sem download automático. O clique prepara persistência e verifica trabalho ativo antes de reiniciar.

## Entrega atual: DMG de teste

O usuário escolheu somente o instalador de teste com página HTML privada pelo Tailscale. O [build macOS arm64 37221443395](https://github.com/calneymgp/force-terminal/actions/runs/37221443395) passou e o DMG 0.14.5 foi entregue. Não foram criados certificados ou secrets. Este pacote usa assinatura ad hoc e instalação manual, com atualização automática desabilitada.

## Próxima etapa: distribuição com atualização

- Configurar um certificado próprio Force persistente e seu fingerprint no CI. Não reutilizar ou copiar as credenciais TAMZ. Não gerar uma assinatura ad hoc diferente a cada versão para o canal de atualização.
- Gerar os pacotes em runner macOS; o servidor Linux pode preparar código e disparar CI, mas não substitui o toolchain macOS do backend.
- Testar instalação e atualização de N para N+1 no Mac usando o mesmo certificado. Os arquivos de distribuição devem manter a assinatura correspondente à identidade instalada.
- Sem notarização, a primeira abertura pode exibir o aviso do macOS. O canal `official` continua disponível se quisermos tratar isso depois; nenhuma loja Apple faz parte deste fluxo.

A [documentação do Electron](https://www.electronjs.org/docs/latest/api/auto-updater#macos) exige assinatura para atualizar no macOS. O [código do Squirrel.Mac](https://github.com/Squirrel/Squirrel.Mac/blob/main/Squirrel/SQRLCodeSignature.m) compara o pacote recebido com o requisito de assinatura da aplicação instalada. Isso sustenta preparar o caminho com certificado estável; não substitui a validação real do Force, que ainda está pendente.
