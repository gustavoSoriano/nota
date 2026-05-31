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

## Stack

- **Linguagem**: Go
- **Arquitetura**: SOLID + CLEAN Architecture
- **Storage**: SQLite local (`~/.nota/data.db`)
- **Busca semântica**: ollama com nomic-embed-text
- **TUI**: Charmbracelet (bubbletea, bubbles, lipgloss, huh)
- **Distribuição**: Go binary compilado para todos os SOs via goreleaser

## Uso por agentes

Agentes usam via skill que ensina o CLI. Não é MCP server.

---

## Comandos

### `nota new [--tags tag1,tag2] [--grupo grupo] [--cat categoria]`

Abre o `$EDITOR` do sistema para escrever o markdown. Metadata (tags, grupo, categoria) passada por flags. Arquivo salvo com YAML frontmatter gerado automaticamente.

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

### `nota open [filtro]`

Mesmo fuzzy finder do edit, mas mostra o documento no terminal (read-only). Agentes usam bastante esse comando.

### `nota delete [filtro]`

Fuzzy finder → seleciona → pede confirmação → remove.

### `nota list [--tags tag1,tag2] [--grupo grupo] [--cat categoria] [--sort recent|accessed|alpha]`

Lista paginada dos últimos 20 documentos. Aceita filtros por tags, grupo, categoria. Ordenação por recent, mais acessados, alfabética.

### `nota search "query" [--tags tag1,tag2] [--grupo grupo] [--cat categoria]`

Busca semântica usando embeddings (ollama + nomic-embed-text). Retorna lista rankeada por relevância com preview. Permite filtros adicionais.

### `nota link`

Fuzzy finder pra selecionar duas notas e criar relação. Ao abrir uma nota (`nota open`), mostra notas linkadas. Cria um grafo de conhecimento entre os documentos.

### `nota tags`

Lista todas as tags existentes com contagem de uso. Útil pra descobrir e padronizar vocabulário.

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
