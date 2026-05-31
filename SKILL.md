# Nota CLI - Skill para Agentes

Nota é um CLI de base de conhecimentos alimentado por markdown com busca semântica via ollama.

## Comandos para agentes

### Criar nota
```bash
nota new --content "markdown content here" --tags tag1,tag2 --grupo dev --cat howto
```

### Quick capture
```bash
nota save "texto ou link" --tags tag1,tag2 --grupo dev
echo "conteúdo do pipe" | nota save --tags incident
```

### Buscar notas (retorna JSON)
```bash
nota search "query semântica" --json
nota search "api node express" --json --tags dev
```

### Abrir nota (retorna markdown puro)
```bash
nota open <id> --raw
```

### Listar notas (retorna JSON)
```bash
nota list --json
nota list --tags dev --sort recent --json
nota list --grupo dev --json
```

### Deletar nota
```bash
nota delete <id> --force
```

### Importar arquivos .md
```bash
nota import ./pasta/ --tags dev --grupo docs
nota import arquivo.md --tags readme
```

### Linkar notas
```bash
nota link  # abre fuzzy finder interativo
```

### Ver tags existentes
```bash
nota tags --json
```

### Backup e restore
```bash
nota backup
nota restore ./nota-backup-2026-05-31-100000.json
```

### Ver config
```bash
nota config
```

## Formato de resposta JSON

### search --json
```json
[
  {
    "id": "abc12345",
    "title": "Como criar API em Node",
    "score": 0.95,
    "tags": ["api", "node"]
  }
]
```

### list --json
```json
[
  {
    "id": "abc12345",
    "title": "Como criar API em Node",
    "tags": ["api", "node"],
    "grupo": "dev",
    "categoria": "howto",
    "created_at": "2026-05-31T10:00:00Z",
    "updated_at": "2026-05-31T10:00:00Z",
    "accessed": 3
  }
]
```

### open --raw
Retorna markdown puro do documento no stdout.

## Workflow recomendado para agentes

1. **Pesquisar**: `nota search "query" --json` para encontrar documentos relevantes
2. **Ler**: `nota open <id> --raw` para obter conteúdo completo
3. **Salvar**: `nota save "informação" --tags x,y` para armazenar novas informações
4. **Criar**: `nota new --content "markdown" --tags x --grupo y` para documentos estruturados
5. **Deletar**: `nota delete <id> --force` para remover documentos

## Requisitos

- ollama rodando com modelo `nomic-embed-text:latest`
- Binário `nota` no PATH
