<p align="center">
  <a href="https://github.com/calneymgp/force-terminal">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="./assets/force-terminal/brand/logo-white.svg">
      <source media="(prefers-color-scheme: light)" srcset="./assets/force-terminal/brand/logo-black.svg">
      <img alt="Force Terminal" src="./assets/force-terminal/brand/logo-black.svg" width="420">
    </picture>
  </a>
</p>

# Force Terminal

Fork do [Wave Terminal](https://github.com/wavetermdev/waveterm) para evoluir em direção a um ambiente de trabalho com agentes, tarefas e notas. A infraestrutura existente de terminal, SSH, arquivos, widgets e modelos de IA continua sendo a base do aplicativo.

A identidade oficial é o **Raio Lunar**: o próprio raio representa o corpo central, envolvido por um anel de poeira. O aplicativo usa versões em preto e branco, com contorno de cor inversa.

## Estado do projeto

A marca Force Terminal está aplicada à interface, aos ícones de janelas e aos recursos de empacotamento de macOS, Windows e Linux. A configuração de atualização aponta para os releases deste fork.

Perfis Force isolados, o botão permanente de atualização com proteção do trabalho e o pipeline macOS arm64 estão implementados. A validação no MacBook M5 e a atualização instalada com assinatura Apple continuam pendentes. Gestão de tarefas, notas e execuções de agentes seguem o [plano de evolução](docs/force-terminal/EVOLUTION.md). Nenhum instalador ou release foi publicado nesta etapa.

## Desenvolvimento

O primeiro alvo é macOS arm64. Instale Node 22, Go da versão em `go.mod`, Task 3 e as ferramentas de compilação do macOS. Consulte [MACOS.md](docs/force-terminal/MACOS.md) para os pré-requisitos, perfis de teste, empacotamento e roteiro de validação. Para executar o aplicativo completo:

```sh
task force:deps
task force:dev
```

`task force:test-profile` cria um perfil temporário explícito; `task force:package:mac-arm64` gera DMG e ZIP ad hoc para instalação manual. Esses comandos compilam backend, helpers e scaffold sem carregar `.env` legado. Os diretórios de dados e configuração são próprios do Force; os identificadores internos `wavesrv`/`wsh` e protocolos são preservados. [BUILD.md](BUILD.md) documenta comandos herdados e outros alvos, fora do aceite inicial.

## Identidade e ícones

- [Identidade oficial e regras de uso](docs/force-terminal/BRAND.md)
- [Fontes vetoriais da marca](assets/force-terminal/brand)
- [Símbolo branco](assets/force-terminal/brand/symbol-white.svg) · [Símbolo preto](assets/force-terminal/brand/symbol-black.svg)

Para regenerar PNG, ICO e ICNS a partir dos SVGs oficiais:

```sh
python3 -m pip install -r scripts/brand-requirements.txt
npm run brand:icons
```

## Origem e licença

Force Terminal é derivado do Wave Terminal, desenvolvido por Command Line Inc. e seus contribuidores, sob a licença [Apache-2.0](LICENSE). Os avisos de copyright e [agradecimentos às dependências](ACKNOWLEDGEMENTS.md) são preservados.

A documentação técnica original permanece disponível em [docs.waveterm.dev](https://docs.waveterm.dev/). Integrações com serviços externos do Wave continuam identificadas como recursos upstream.
