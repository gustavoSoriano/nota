# Nota

CLI knowledge base com busca semântica. Guarda markdown, encontra quando precisa.

## Instalação

```bash
curl -fsSL https://raw.githubusercontent.com/soriano/nota/main/install.sh | bash
```

Instala nota, micro editor, ollama e modelo nomic-embed-text.

## Uso

```bash
nota new                                    # abre editor
nota new -c "conteúdo" --tags api,node      # direto no terminal
nota save "link ou texto rápido" --tags dev  # quick capture
nota search "api node express" --json        # busca semântica
nota list --json                             # lista notas
nota open <id>                              # ver conteúdo
nota edit <id> -c "novo conteúdo"            # editar
nota delete <id> --force                     # deletar
nota link <source_id> <target_id>            # conectar notas
nota tags --json                             # lista tags
nota serve                                   # interface web
nota serve --port 3000                       # porta customizada
nota backup                                  # dump com timestamp
nota restore <arquivo>                       # restaura backup
nota clean                                   # remove tudo
nota config                                  # mostra config
```

## Flags

- `--tags tag1,tag2` - filtrar por tags
- `--notebook nome` - filtrar por notebook
- `--cat categoria` - filtrar por categoria
- `--json` - saída JSON (list, search, tags)
- `--raw` - texto puro (list, search)
- `--force` - sem confirmação (delete)
- `--limit` - máximo de resultados (search, padrão 40)

## Requisitos

- ollama com `nomic-embed-text:latest`

## Build

```bash
make            # build local
make build-all  # todos os SOs
```