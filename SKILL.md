# Nota CLI - Knowledge Base Skill

Nota é um CLI de base de conhecimentos alimentado por markdown com busca semântica via ollama.

## Quando usar

- O usuário pede pra salvar, buscar, ou gerenciar anotações/conhecimento
- O usuário menciona notes, documentos, knowledge base, base de conhecimento
- O usuário quer armazenar links, how-tos, RFCs, post-mortems, transcrições, etc.
- Você mesmo (agente) precisa guardar ou recuperar informações do conhecimento do usuário

## Comandos para agentes

### Criar nota (sem abrir editor)
```bash
nota new --content "markdown content" --tags tag1,tag2 --grupo dev --cat howto
```

### Quick capture (texto curto, link)
```bash
nota save "texto ou link" --tags tag1,tag2 --grupo dev
nota save "texto" -t api -g dev
echo "conteúdo do pipe" | nota save --tags incident
```

### Busca semântica (JSON)
```bash
nota search "query" --json
nota search "api node" --json --tags dev --limit 10
```

### Abrir nota (markdown puro)
```bash
nota open <id> --raw
```

### Editar nota (sem abrir editor, pra agentes)
```bash
nota edit <id> --content "novo conteúdo"
```

### Listar notas (JSON)
```bash
nota list --json
nota list --tags dev --sort recent --json
nota list -g dev --json
```

### Deletar nota
```bash
nota delete <id> --force
```

### Importar .md
```bash
nota import ./pasta/ --tags dev
nota import arquivo.md --tags readme
```

### Linkar notas
```bash
nota link
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

### Versão e config
```bash
nota --version
nota config
```

## Flags

- `-t, --tags` - tags separadas por vírgula
- `-g, --grupo` - grupo
- `-c, --cat` - categoria
- `--content` - conteúdo direto (new, edit)
- `--json` - saída JSON (search, list, tags)
- `--raw` - markdown puro (open)
- `--force` - sem confirmação (delete)
- `--limit` - máximo de resultados (search, padrão 20)

## Formato JSON

### search --json
```json
[{"id": "abc123", "title": "...", "score": 0.95, "tags": ["..."]}]
```

### list --json
```json
[{"id": "abc123", "title": "...", "tags": [...], "grupo": "...", "categoria": "...", "created_at": "...", "accessed": 3}]
```

### open --raw
Markdown puro no stdout.

## Workflow recomendado

1. `nota search "query" --json` → encontra docs relevantes
2. `nota open <id> --raw` → lê conteúdo completo
3. `nota save "info" --tags x` → salva nova info
4. `nota new --content "md" --tags x` → salva doc estruturado
5. `nota edit <id> --content "novo"` → atualiza doc existente
6. `nota delete <id> --force` → remove doc

## Requisitos
- ollama rodando com `nomic-embed-text:latest`
- binário `nota` no PATH
