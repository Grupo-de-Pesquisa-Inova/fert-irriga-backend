# Script de backup automático do PostgreSQL via Docker
# Uso: powershell -ExecutionPolicy Bypass -File scripts/backup.ps1

$BackupDir = Join-Path $PSScriptRoot ".." "backups"
$Timestamp = Get-Date -Format "yyyy-MM-dd_HH-mm-ss"
$BackupFile = Join-Path $BackupDir "fertirriga_$Timestamp.sql"

# Criar diretório de backups se não existir
if (!(Test-Path $BackupDir)) {
    New-Item -ItemType Directory -Path $BackupDir -Force | Out-Null
}

Write-Host "[FertIrriga] Iniciando backup do PostgreSQL..." -ForegroundColor Cyan

# Executar pg_dump via container Docker
docker exec fertirriga-db pg_dump -U fertirriga --format=plain fertirriga > $BackupFile

if ($LASTEXITCODE -eq 0) {
    $Size = (Get-Item $BackupFile).Length / 1KB
    Write-Host "[FertIrriga] Backup criado com sucesso!" -ForegroundColor Green
    Write-Host "  Arquivo: $BackupFile" -ForegroundColor Gray
    Write-Host "  Tamanho: $([math]::Round($Size, 2)) KB" -ForegroundColor Gray

    # Limpar backups antigos (manter últimos 30)
    $OldBackups = Get-ChildItem $BackupDir -Filter "fertirriga_*.sql" | Sort-Object CreationTime -Descending | Select-Object -Skip 30
    if ($OldBackups) {
        $OldBackups | Remove-Item -Force
        Write-Host "  Backups antigos removidos: $($OldBackups.Count)" -ForegroundColor Yellow
    }
} else {
    Write-Host "[FertIrriga] ERRO: Falha ao criar backup!" -ForegroundColor Red
    exit 1
}
