# Force Terminal — pitch de boas-vindas

Protótipo visual independente para avaliar as futuras boas-vindas. Os agentes, tarefas e rotinas desta apresentação são uma demonstração de conceito; não executam trabalho nem representam funcionalidades já integradas ao aplicativo.

Uma apresentação silenciosa de 36 segundos percorre quatro capítulos: direção, agentes e sessões, tarefas e rotinas. Há pausa, replay, navegação por capítulos e controle de posição. As tarefas avançam com a apresentação; voltar ou repetir também restaura seu estado. Com movimento reduzido, a apresentação permanece estática e permite explorar os capítulos manualmente.

Abra `index.html` no navegador. Os recursos são locais: `style.css`, `pitch.js`, os SVGs aprovados copiados da pasta `brand` e GSAP 3.13.0 em `vendor/gsap.min.js`, com seu cabeçalho de licença preservado. Não há fontes remotas, analytics, pedidos a serviços externos ou dependências do aplicativo Electron.

A publicação usa o subdiretório `pitch` da página privada de distribuição já existente. Não exige um novo servidor ou porta.

Validação: os quatro capítulos foram exercitados em Chromium e WebKit, nas larguras de 1440, 900 e 390 pixels; movimento reduzido, progresso das tarefas, pausa, replay e reprodução após o final foram conferidos. O aplicativo e o DMG permanecem independentes desta prévia.
