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
nota search "api node express"              # busca semântica
nota list                                   # lista notas recentes
nota open                                   # fuzzy finder → visualiza
nota edit                                   # fuzzy finder → edita
nota delete                                 # fuzzy finder → deleta
nota link                                   # conecta duas notas
nota tags                                   # lista tags
nota backup                                 # dump com timestamp
nota restore <arquivo>                      # restaura backup
nota clean                                  # remove tudo
nota config                                 # mostra config
```

## Flags

- `--tags tag1,tag2` - filtrar por tags
- `--grupo nome` - filtrar por grupo
- `--cat categoria` - filtrar por categoria
- `--json` - saída JSON (list, search, tags)
- `--raw` - markdown puro (open)
- `--force` - sem confirmação (delete)

## Agentes

Comandos com `--json` e `--raw` retornam texto parseável. Skill incluída em `SKILL.md`.

## Requisitos

- ollama com `nomic-embed-text:latest`

## Build

```bash
make            # build local
make build-all  # todos os SOs
```
