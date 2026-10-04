# Force Terminal — identidade oficial

Identidade Raio Lunar aprovada em 4 de outubro de 2026.

## Nome e símbolo

O nome público do aplicativo é **Force Terminal**. O identificador do pacote e executável Linux é `force-terminal`; o identificador do aplicativo é `io.github.calneymgp.force-terminal`. Force Agent foi o nome dos estudos de logo e não substitui o nome do aplicativo.

O próprio raio maciço é o corpo central. O anel inclinado em −18° é uma faixa de poeira com grãos e trilhas vetoriais, que passa atrás e à frente do raio. Os vazios preservam a silhueta elétrica.

## Paleta e aplicações

Somente preto `#000000`, branco `#FFFFFF` e transparência.

| Aplicação | Arquivo | Regra |
| --- | --- | --- |
| Símbolo detalhado em fundo escuro | `assets/force-terminal/brand/symbol-white.svg` | Branco com borda preta |
| Símbolo detalhado em fundo claro | `assets/force-terminal/brand/symbol-black.svg` | Preto com borda branca |
| UI e ícones pequenos | `symbol-compact-white.svg` / `symbol-compact-black.svg` | Anel simplificado; preferir de 24 a 64 px |
| Nome com símbolo | `logo-white.svg` / `logo-black.svg` | Force Terminal em contornos vetoriais |
| Ícone de desktop grande | `icon.svg` | Símbolo branco em base preta com borda branca |
| Ícone de desktop pequeno | `icon-compact.svg` | Mesma base; anel simplificado |

Os nomes abreviados na tabela são relativos a `assets/force-terminal/brand/`. O símbolo detalhado deve ser usado a partir de 128 px; os ícones rasterizados abaixo de 256 px usam a geometria compacta. As letras dos logotipos são contornos derivados da fonte Inter existente no repositório.

Mantenha as proporções. Reserve ao menos 8% da largura do símbolo como espaço livre e use o par de cores que tenha maior contraste com o fundo. Não acrescente gradientes, sombras, estrelas, planetas circulares ou partículas fora da faixa orbital.

## Fontes e arquivos derivados

Os SVGs oficiais ficam em `assets/force-terminal/brand/`. Os estudos em `assets/force-agent/` e os rascunhos antigos não são usados pelo aplicativo.

A UI consome `frontend/app/asset/logo.svg` (compacto, 48 px) e `logo-detailed.svg` (128 px). O diretório `public/logos/` contém os recursos estáticos usados pela aplicação. Essas cópias vetoriais correspondem às fontes oficiais; uma futura alteração do desenho deve atualizar as respectivas cópias em conjunto.

`npm run brand:icons` usa `scripts/generate-force-icons.py` e regenera:

- PNG em 16, 24, 32, 48, 64, 128, 256, 512 e 1024 px em `build/icons/`;
- `build/icon.ico` para Windows e `build/icon.icns` para macOS;
- `public/logos/force-terminal-icon.png` e a prévia `assets/force-terminal/brand/icon.png`;
- aliases de nome herdado usados pelo scaffold dos widgets e pelas prévias.

Pré-requisitos de renderização: `python3 -m pip install -r scripts/brand-requirements.txt`.

## Integração e limites desta etapa

A marca foi aplicada à interface, aos ícones de janelas Electron, aos recursos dos instaladores e aos metadados do pacote. O publish do electron-builder aponta para os releases de `calneymgp/force-terminal`; esta configuração não publica uma versão por si só.

O bootstrap usa perfis próprios `force-terminal` e `force-terminal-dev`; o helper remoto usa `~/.force-terminal`. Variáveis internas de transporte, protocolos e nomes dos binários `wavesrv`/`wsh` foram preservados. Não há importação automática dos dados Wave. A coexistência real, incluindo secrets/Keychain, ainda precisa de validação no M5.

A documentação local usa a marca Force Terminal e tem o alvo de configuração GitHub Pages do fork. O pipeline macOS arm64 gera artefatos ad hoc para teste; releases `community` usam certificado próprio estável, sem notarização, e `official` mantém a opção Developer ID/notarização. O usuário escolheu o primeiro DMG de teste por página privada Tailscale. A conferência da marca instalada ainda precisa de teste no Mac. Veja [MACOS.md](MACOS.md).

Wave AI e a documentação/serviços externos do Wave mantêm o nome upstream. Licença Apache-2.0, créditos e avisos de copyright de Command Line Inc. permanecem intactos. O fluxo de atualização pela topbar foi implementado com guardas de persistência e adiamento; o ciclo real com pacote assinado permanece pendente. A conferência visual da marca no Finder, Dock e aplicativo instalado no M5 também permanece pendente.
