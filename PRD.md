# Nota - CLI Knowledge Base

## Visão

CLI de base de conhecimentos alimentado por arquivos markdown. Feito pra guardar e encontrar informações rapidamente. Não é ferramenta de autoria - você escreve como quiser, o nota salva e recupera quando precisar.

## Contexto de uso

Uso intenso em T.I.: reuniões, transcrições, links de ferramentas, anotações, post-mortems, RFCs, conversas no Slack, how-tos, documentações. Tudo num lugar só, searchable por semântica.

## Execução

- Binário único, funciona em qualquer S.O.
- Executável de qualquer lugar da máquina
- Tudo via linha de comando
- Requer ollama rodando com modelo nomic-embed-text

## Instalação

### Opção 1: Install script (recomendado)

```bash
curl -fsSL https://raw.githubusercontent.com/seu-user/nota/main/install.sh | bash
```

Detecta SO e arquitetura, baixa binário correto, move pra `/usr/local/bin/`, pronto.

### Opção 2: Go install

```bash
go install github.com/seu-user/nota@latest
```

### Opção 3: Download manual

Baixa o binário do GitHub Releases, move pro PATH:

```bash
mv nota /usr/local/bin/
```

### Setup na primeira execução

Ao rodar `nota` pela primeira vez:
- Cria `~/.nota/` com `data.db` e `config.json`
- Detecta ollama e modelo nomic-embed-text
- Se ollama não encontrado, mostra instrução de instalação
- Instala micro editor se não tiver nenhum editor configurado
- Salva micro como `$EDITOR` no config do nota

## Stack

- **Linguagem**: Go
- **Arquitetura**: SOLID + CLEAN Architecture
- **Storage**: SQLite local (`~/.nota/data.db`)
- **Busca semântica**: ollama com nomic-embed-text
- **TUI**: Charmbracelet (bubbletea, bubbles, lipgloss, huh)
- **Editor**: micro (instalado automaticamente, substituível via config)
- **Distribuição**: Go binary compilado para todos os SOs via goreleaser

## Duas interfaces

O nota tem dois modos de saída:

- **TUI interativo** (padrão): fuzzy finder, navegação com setas, preview, cores. Pra uso humano no terminal.
- **Machine-readable** (flags `--json`, `--raw`): saída estruturada parseável. Pra agentes e scripts.

Quando o TUI não faz sentido (pipe, `--json`, `--raw`), ele não é renderizado. O comando detecta automaticamente se stdout é um TTY.

## Uso por agentes

Agentes usam via skill que ensina o CLI. Não é MCP server. Comandos preferidos por agentes:

```bash
nota search "query" --json        # array JSON com resultados
nota open <id> --raw              # markdown puro do documento
nota save "texto" --tags x,y      # salva rápido sem TUI
nota new --content "md aqui" --tags x  # cria direto sem editor
nota list --json                  # lista paginada em JSON
nota delete <id> --force          # deleta sem fuzzy finder
```

---

## Comandos

### `nota new [--tags tag1,tag2] [--grupo grupo] [--cat categoria] [--content "markdown"]`

Abre o `$EDITOR` do sistema para escrever o markdown. Metadata (tags, grupo, categoria) passada por flags. Arquivo salvo com YAML frontmatter gerado automaticamente.

Flag `--content` cria direto sem abrir editor (uso por agentes).

Frontmatter exemplo:
```yaml
---
id: abc123
title: Como criar API em Node
tags: [api, node, express]
grupo: dev
categoria: howto
created_at: 2026-05-31T10:00:00Z
updated_at: 2026-05-31T10:00:00Z
accessed: 0
---
```

### `nota save "texto ou link" [--tags tag1,tag2] [--grupo grupo] [--cat categoria]`

Quick capture. Salva sem abrir editor. Ideal pra links rápidos do Slack, browser.

Suporta pipe:
```bash
echo "slack copy cola" | nota save --tags incident
pbpaste | nota save
```

### `nota import <caminho> [--tags tag1,tag2] [--grupo grupo]`

Importa arquivo .md ou pasta inteira de .mds. Flags opcionais pra adicionar metadata em massa.

### `nota edit [filtro]`

Abre fuzzy finder interativo com busca em tempo real (título, tags, conteúdo). Filtra, navega com setas, enter seleciona e abre no `$EDITOR`. Se receber filtro como argumento, pré-filtra a lista.

### `nota open <id> [--raw]`

Fuzzy finder pra selecionar documento e mostrar no terminal (read-only, scrollável). Com `--raw`, imprime markdown puro no stdout sem TUI (uso por agentes).

### `nota delete <id> [--force]`

Fuzzy finder → seleciona → pede confirmação → remove. Com `--force` + id, deleta direto sem fuzzy finder nem confirmação (uso por agentes).

### `nota list [--tags tag1,tag2] [--grupo grupo] [--cat categoria] [--sort recent|accessed|alpha] [--json]`

Lista paginada dos últimos 20 documentos. Aceita filtros por tags, grupo, categoria. Ordenação por recent, mais acessados, alfabética. Com `--json`, retorna array JSON parseável.

### `nota search "query" [--tags tag1,tag2] [--grupo grupo] [--cat categoria] [--json]`

Busca semântica usando embeddings (ollama + nomic-embed-text). Retorna TUI interativa rankeada por relevância com preview. Permite filtros adicionais. Com `--json`, retorna array JSON parseável (uso por agentes).

### `nota link`

Fuzzy finder pra selecionar duas notas e criar relação. Ao abrir uma nota (`nota open`), mostra notas linkadas. Cria um grafo de conhecimento entre os documentos.

### `nota tags [--json]`

Lista todas as tags existentes com contagem de uso. Com `--json`, retorna array JSON parseável.

### `nota backup`

Gera dump com timestamp: `nota-backup-2026-05-31-100000.tar.gz`. Contém banco SQLite + arquivos .md exportados.

### `nota restore <arquivo>`

Restaura de backup. Pede confirmação antes de sobrescrever dados existentes.

### `nota clean`

Remove tudo. Pede confirmação dupla.

### `nota config`

Mostra configurações atuais (editor, endpoint ollama, path do storage).

---

## Estrutura de diretórios

```
~/.nota/
├── data.db          # SQLite (documentos, embeddings, FTS5, links)
├── config.json      # Configurações do usuário
└── backups/         # Backups gerados
```

## Modelo de dados

Cada documento no SQLite:

| Campo | Tipo | Descrição |
|---|---|---|
| id | string | UUID único |
| title | string | Primeira linha do markdown ou fallback |
| content | text | Conteúdo markdown completo |
| tags | JSON | Array de tags |
| grupo | string | Agrupamento lógico |
| categoria | string | Categoria (howto, rfc, postmortem, etc) |
| embedding | blob | Embedding vetorial (nomic-embed-text) |
| created_at | datetime | Data de criação |
| updated_at | datetime | Data de última edição |
| accessed | int | Contador de acessos |

## Metadata automática

Toda nota recebe automaticamente:
- `id` - UUID gerado
- `title` - extraído da primeira linha do markdown
- `created_at` - timestamp de criação
- `updated_at` - timestamp de edição (atualizado no `nota edit`)
- `accessed` - incrementado a cada `nota open`

## Funcionalidades futuras (não no MVP)

- `nota stats` - estatísticas de uso
- `nota pin` - fixar notas importantes
- `nota merge` - mesclar duas notas
- Integração com clipboard monitoring
