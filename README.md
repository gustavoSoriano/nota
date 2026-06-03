# Nota

CLI knowledge base em markdown. Guarda, organiza e encontra notas com busca full-text rápida — sem dependências externas.

## Instalação

```bash
curl -fsSL https://raw.githubusercontent.com/gustavoSoriano/nota/main/install.sh | bash
```

Instala o binário e o editor `micro`. Sem Ollama, sem serviços externos.

## Atualização

```bash
nota update          # atualiza para a versão mais recente
nota update --check  # só verifica se há update disponível
```

## Uso

```bash
# Criar notas
nota new                                         # abre editor
nota new --content "# Título\nConteúdo"          # direto no terminal
nota save "link ou texto rápido" --tags dev       # quick capture

# Busca full-text (título, conteúdo, tags, notebook, category)
nota search "api node express"
nota search "deploy" --tags devops --notebook work
nota search "golang" --cat backend --json
nota search "\"frase exata\""                    # busca por frase

# Listar e filtrar
nota list                                        # lista recentes
nota list --tags dev --sort recent
nota list --notebook work --cat howto --json

# Gerenciar
nota open <id>                                   # ver conteúdo
nota edit <id>                                   # abre editor
nota edit <id> --content "novo conteúdo"         # editar direto (agentes)
nota delete <id> --force
nota link <source_id> <target_id>                # conectar notas

# Metadados
nota tags --json                                 # lista todas as tags com contagem
nota config                                      # mostra configuração

# Backup
nota backup                                      # dump JSON com timestamp
nota restore <arquivo>                           # restaura backup

# Interface web
nota serve                                       # http://localhost:3003
nota serve --port 3000

# Manutenção
nota update                                      # auto-update
nota clean                                       # remove todas as notas
```

## Flags globais

| Flag | Atalho | Descrição |
|---|---|---|
| `--tags tag1,tag2` | `-t` | filtrar por tags |
| `--notebook nome` | `-b` | filtrar por notebook (caderno) |
| `--cat categoria` | `-c` | filtrar por category |
| `--json` | | saída JSON (list, search, tags) |
| `--raw` | | texto puro (list, search) |
| `--force` | | sem confirmação (delete) |
| `--limit N` | | máximo de resultados (search, padrão 40) |

## Busca

A busca usa **FTS5** (SQLite full-text search com BM25) e cobre automaticamente:

- Título da nota
- Conteúdo completo
- Tags
- Notebook e category (via filtros)

```bash
# Busca simples
nota search "kubernetes"

# Filtrar por tag E buscar no conteúdo
nota search "deploy" --tags devops

# Filtrar por notebook
nota search "reunião" --notebook trabalho

# Filtrar por category
nota search "setup" --cat howto

# Combinar filtros
nota search "api" --tags backend --notebook dev --cat rfc

# Frase exata
nota search '"rate limiting"'

# Saída JSON para scripts/agentes
nota search "query" --json
```

## Formato JSON

### `search --json`
```json
[{
  "id": "abc123ef",
  "title": "Título da nota",
  "score": 0.95,
  "tags": ["go", "backend"],
  "notebook": "work",
  "category": "howto",
  "snippet": "...trecho relevante do conteúdo..."
}]
```

### `list --json`
```json
[{
  "id": "abc123ef",
  "title": "Título da nota",
  "content": "...",
  "tags": ["go", "backend"],
  "notebook": "work",
  "category": "howto",
  "created_at": "2026-06-02T10:00:00Z",
  "updated_at": "2026-06-02T10:00:00Z",
  "accessed": 3
}]
```

## Organização

Notas podem ser organizadas em três dimensões:

| Dimensão | Flag | Exemplo |
|---|---|---|
| Tags | `--tags` | `go,backend,api` |
| Notebook | `--notebook` | `work`, `pessoal`, `estudos` |
| Category | `--cat` | `howto`, `rfc`, `postmortem` |

## Build

```bash
make build          # build local
make build-all      # todos os SOs (darwin, linux, windows × amd64/arm64)
make install        # build + instala em /usr/local/bin
make release        # build-all + checksums SHA256
```

## Requisitos

- Go 1.21+ (para build)
- Binário `nota` no PATH (para uso)
- Sem dependências externas em runtime
