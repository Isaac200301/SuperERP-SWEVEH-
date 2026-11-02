#!/usr/bin/env bash
# Copia las skills del proyecto (agent-skills/) a .claude/skills/ para que Claude Code las cargue.
# Uso (desde la raíz del repo):  bash scripts/instalar-skills.sh
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
mkdir -p "$root/.claude/skills"
for d in "$root"/agent-skills/*/; do
  n="$(basename "$d")"
  mkdir -p "$root/.claude/skills/$n"
  cp "$d/SKILL.md" "$root/.claude/skills/$n/SKILL.md"
  echo "OK  $n"
done
echo "Listo: $(ls "$root/.claude/skills" | wc -l) skills en $root/.claude/skills"
