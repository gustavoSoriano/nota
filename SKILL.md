# Nota CLI - Knowledge Base Skill

Nota é um CLI de base de conhecimentos alimentado por markdown com busca FTS5 (full-text search) via SQLite. Zero dependências externas.

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

### Busca (JSON)
```bash
nota search "query" --json
nota search "api node" --json --tags dev --limit 10
nota search "deploy" --json --notebook work --cat howto
nota search "golang" --json --cat backend
```

A busca cobre automaticamente: título, conteúdo, tags, notebook e category.

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
nota list --cat howto --json
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
nota serve              # sobe na :3003
nota serve --port 3000  # porta customizada
```

### Versão e config
```bash
nota --version
nota config
```

## Flags

- `-t, --tags` - tags separadas por vírgula
- `-b, --notebook` - notebook (caderno/grupo)
- `-c, --cat` - category (categoria)
- `--content` - conteúdo direto (new, edit)
- `--json` - saída JSON (search, list, tags)
- `--limit` - máximo de resultados (search, padrão 40)

## Formato JSON

### search --json
```json
[{
  "id": "abc123",
  "title": "...",
  "score": 0.95,
  "tags": ["go", "backend"],
  "notebook": "work",
  "category": "howto",
  "snippet": "...trecho relevante..."
}]
```

### list --json
```json
[{
  "id": "abc123",
  "title": "...",
  "tags": [...],
  "notebook": "...",
  "category": "...",
  "created_at": "...",
  "accessed": 3
}]
```

### open
Markdown puro no stdout.

## Workflow recomendado

1. `nota search "query" --json` → encontra docs relevantes (busca em título, conteúdo, tags, notebook, category)
2. `nota open <id>` → lê conteúdo completo
3. `nota save "info" --tags x` → salva nova info
4. `nota new --content "md" --tags x` → salva doc estruturado
5. `nota edit <id> --content "novo"` → atualiza doc existente
6. `nota delete <id> --force` → remove doc
7. `nota serve` → interface web com editor e preview

## Requisitos
- Binário `nota` no PATH
- Sem dependências externas (sem Ollama, sem serviços externos)
