#!/usr/bin/env bash
# release.sh — publica uma nova versão do nota
# Uso: ./release.sh 0.2.0
set -euo pipefail

VERSION="${1:-}"

# ── Validações ────────────────────────────────────────────────────────────────

if [ -z "$VERSION" ]; then
  echo "Uso: ./release.sh <versão>"
  echo "Exemplo: ./release.sh 0.2.0"
  exit 1
fi

# Remover 'v' se o usuário passar 'v0.2.0'
VERSION="${VERSION#v}"

if ! echo "$VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "Versão inválida: '$VERSION' — use o formato X.Y.Z (ex: 0.2.0)"
  exit 1
fi

TAG="v${VERSION}"

# Verificar se estamos no branch main
BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$BRANCH" != "main" ]; then
  echo "Aviso: você está no branch '$BRANCH', não em 'main'."
  read -r -p "Continuar mesmo assim? [y/N] " confirm
  if [ "${confirm,,}" != "y" ]; then
    echo "Cancelado."
    exit 1
  fi
fi

# Verificar se há mudanças não commitadas
if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "Há mudanças não commitadas. Faça commit antes de publicar."
  git status --short
  exit 1
fi

# Verificar se a tag já existe
if git tag | grep -q "^${TAG}$"; then
  echo "Tag ${TAG} já existe. Use uma versão diferente."
  exit 1
fi

# ── Build local para validar ──────────────────────────────────────────────────

echo "→ Validando build (VERSION=${VERSION})..."
make build VERSION="$VERSION"
echo "  Build OK"

# ── Commit, tag e push ────────────────────────────────────────────────────────

echo "→ Criando tag ${TAG}..."
git tag -a "$TAG" -m "Release ${TAG}"

echo "→ Enviando para o GitHub..."
git push origin "$BRANCH"
git push origin "$TAG"

echo ""
echo "✓ Release ${TAG} publicado!"
echo ""
echo "  O GitHub Actions vai compilar e publicar os binários automaticamente."
echo "  Acompanhe em: https://github.com/gustavoSoriano/nota/actions"
echo ""
echo "  Quando concluído, usuários podem atualizar com:"
echo "    nota update"
