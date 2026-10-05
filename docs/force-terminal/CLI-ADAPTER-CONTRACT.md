# Contrato verificado dos primeiros adaptadores CLI

Inspeção somente leitura em 4 de outubro de 2026: Codex CLI `0.160.0` e Claude Code `2.1.289` instalados no servidor. Foram consultados help/version, binários e fontes oficiais; nenhuma conversa, credencial ou sessão de usuário foi lida ou iniciada. Esses executáveis ainda não estão integrados ao Force. O catálogo guarda a preferência do perfil, sem disparar um processo.

## Claude Code em terminal interativo

O CLI permite escolher um UUID antes do início com `--session-id`, aplicar instruções adicionais com `--append-system-prompt-file` e retomar o ID explícito com `--resume`. `--system-prompt-snapshot on` registra o prompt inicial; compactações podem alterar o contexto que o fornecedor mantém. Referência: [CLI Claude Code](https://code.claude.com/docs/en/cli-reference).

O Force deverá persistir instância, UUID e snapshot versionado antes de abrir o processo, colocar o arquivo de instruções no mesmo host onde o CLI roda e confirmar a identidade da conversa. A retomada reutiliza aquele host/usuário/histórico e o ID exato. Falha de retomada preserva o agente e não chama o início de uma conversa nova. A execução com histórico ausente ainda precisa de teste real; mensagens encontradas no binário não provam esse comportamento por si.

## Codex

O início da TUI não oferece `--session-id` no help da versão instalada. Duas interfaces documentadas fornecem identificação estruturada: `codex exec --json` emite `thread.started` com `thread_id` e permite retomada por ID, porém opera em turnos sem a TUI; o app-server oferece `thread/start` e `thread/resume` e retorna identidade explícita. O CLI local marca app-server como experimental. Referências: [modo não interativo](https://developers.openai.com/codex/non-interactive-mode/) e [app-server](https://developers.openai.com/codex/app-server/).

`developer_instructions` é a configuração documentada para instruções adicionais; `model_instructions_file` substitui instruções base. Uma integração deverá isolar a configuração efetiva por instância, guardar a versão usada e comprová-la ao retomar. Não inferir uma capacidade de prompt por thread que não esteja no contrato verificado. Referência: [configuração Codex](https://developers.openai.com/codex/config-reference/).

## Invariantes comuns da implementação seguinte

- Não selecionar `last`, a conversa mais recente do host ou uma conversa inferida do scrollback.
- Guardar ID exato, versão do adaptador, destino, usuário/contexto do CLI, raiz e prompt efetivos; não copiar credenciais para esse registro.
- Não trocar `CODEX_HOME`, home ou histórico do CLI entre início e retomada sem uma escolha explícita e suporte comprovado.
- Confirmar job vivo antes de reanexar; confirmar ausência/encerramento antes de criar outra execução.
- Serializar início/retomada por instância no backend, inclusive entre janelas.
- Provar o mesmo ID e contexto com CLI controlado de teste e, depois, com a integração real; o help não encerra o aceite de reconexão do EVOLUTION.md.
