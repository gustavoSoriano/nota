# Nota CLI - Knowledge Base Skill

Nota é um CLI de base de conhecimentos alimentado por markdown com busca semântica via ollama.

## Quando usar

- O usuário pede pra salvar, buscar, ou gerenciar anotações/conhecimento
- O usuário menciona notes, documentos, knowledge base, base de conhecimento
- O usuário quer armazenar links, how-tos, RFCs, post-mortems, transcrições, etc.
- Você mesmo (agente) precisa guardar ou recuperar informações do conhecimento do usuário

## Comandos para agentes (non-interactive)

### Criar nota
```bash
nota new --content "markdown content" --tags tag1,tag2 --notebook dev --cat howto
nota save "texto ou link" --tags tag1,tag2 --notebook dev
```

### Busca semântica (JSON)
```bash
nota search "query" --json
nota search "api node" --json --tags dev --limit 10
```

### Abrir nota (markdown puro)
```bash
nota open <id>
```

### Editar nota (sem abrir editor, pra agentes)
```bash
nota edit <id> --content "novo conteúdo"
```

### Listar notas (JSON)
```bash
nota list --json
nota list --tags dev --sort recent --json
nota list -b dev --json
```

### Deletar nota
```bash
nota delete <id> --force
```

### Ver tags
```bash
nota tags --json
```

### Backup e restore
```bash
nota backup
nota restore <arquivo>
```

### Interface web
```bash
nota serve              # sobe na :8080
nota serve --port 3000  # porta customizada
```

### Versão e config
```bash
nota --version
nota config
```

## Flags

- `-t, --tags` - tags separadas por vírgula
- `-b, --notebook` - notebook
- `-c, --cat` - category
- `--content` - conteúdo direto (new, edit)
- `--json` - saída JSON (search, list, tags)
- `--limit` - máximo de resultados (search, padrão 40)

## Formato JSON

### search --json
```json
[{"id": "abc123", "title": "...", "score": 0.95, "tags": ["..."]}]
```

### list --json
```json
[{"id": "abc123", "title": "...", "tags": [...], "notebook": "...", "category": "...", "created_at": "...", "accessed": 3}]
```

### open
Markdown puro no stdout.

## Workflow recomendado

1. `nota search "query" --json` → encontra docs relevantes
2. `nota open <id>` → lê conteúdo completo
3. `nota save "info" --tags x` → salva nova info
4. `nota new --content "md" --tags x` → salva doc estruturado
5. `nota edit <id> --content "novo"` → atualiza doc existente
6. `nota delete <id> --force` → remove doc
7. `nota serve` → interface web com editor e preview

## Requisitos
- ollama com `nomic-embed-text:latest`
- binário `nota` no PATH