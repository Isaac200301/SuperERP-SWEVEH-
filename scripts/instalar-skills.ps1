# Copia las skills del proyecto (agent-skills/) a .claude/skills/ para que Claude Code las cargue.
# Uso (desde la raíz del repo, en PowerShell):  .\scripts\instalar-skills.ps1
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$src  = Join-Path $root 'agent-skills'
$dst  = Join-Path $root '.claude\skills'
New-Item -ItemType Directory -Force -Path $dst | Out-Null
Get-ChildItem -Path $src -Directory | ForEach-Object {
    $target = Join-Path $dst $_.Name
    New-Item -ItemType Directory -Force -Path $target | Out-Null
    Copy-Item -Path (Join-Path $_.FullName 'SKILL.md') -Destination (Join-Path $target 'SKILL.md') -Force
    Write-Host "OK  $($_.Name)"
}
Write-Host "Listo: $((Get-ChildItem $dst -Directory).Count) skills en $dst"
