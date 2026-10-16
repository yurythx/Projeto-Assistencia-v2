<#
.SYNOPSIS
  Executa o deploy ou consulta o status do Projeto-Assistencia-v2 na VM do Proxmox (192.168.1.20)
  a partir da sua máquina Windows sem precisar de make, terraform ou ansible locais.
.EXAMPLE
  .\scripts\deploy-vm.ps1
  .\scripts\deploy-vm.ps1 -Action status
  .\scripts\deploy-vm.ps1 -Action logs
#>
[CmdletBinding()]
param(
    [ValidateSet("deploy", "status", "logs", "restart", "seed-admin")]
    [string]$Action = "deploy",
    [string]$VmIp = "192.168.1.20",
    [string]$SshUser = "deploy",
    [string]$SshKey = "$HOME\.ssh\projeto_assistencia_deploy"
)

Write-Host "==> Verificando conectividade com a VM ($($VmIp):22)..." -ForegroundColor Cyan
$conn = Test-NetConnection -ComputerName $VmIp -Port 22 -WarningAction SilentlyContinue
if (-not $conn.TcpTestSucceeded) {
    Write-Host "[ERRO] A VM em $($VmIp) não está respondendo na porta 22 (SSH)." -ForegroundColor Red
    Write-Host "Dica: Verifique se a VM 210 (projeto-assistencia) está ligada no Proxmox (https://192.168.1.13:8006)." -ForegroundColor Yellow
    exit 1
}

switch ($Action) {
    "deploy" {
        Write-Host "==> Atualizando código e executando deploy na VM..." -ForegroundColor Green
        & ssh.exe -i "$SshKey" -o StrictHostKeyChecking=no "$SshUser@$VmIp" "cd /opt/projeto-assistencia-v2 && git pull && ./scripts/deploy.sh --no-pull $VmIp"
        Write-Host "`n[OK] Deploy finalizado! Acesse: http://${VmIp}:3010" -ForegroundColor Cyan
    }
    "status" {
        Write-Host "==> Estado dos containers e health..." -ForegroundColor Green
        & ssh.exe -i "$SshKey" -o StrictHostKeyChecking=no "$SshUser@$VmIp" "cd /opt/projeto-assistencia-v2 && docker compose ps; echo ''; curl -s http://127.0.0.1:8010/health; echo ''"
    }
    "logs" {
        Write-Host "==> Acompanhando logs (Ctrl+C para sair)..." -ForegroundColor Green
        & ssh.exe -i "$SshKey" -o StrictHostKeyChecking=no "$SshUser@$VmIp" "cd /opt/projeto-assistencia-v2 && docker compose logs -f --tail 50 backend-api frontend"
    }
    "restart" {
        Write-Host "==> Reiniciando aplicação..." -ForegroundColor Green
        & ssh.exe -i "$SshKey" -o StrictHostKeyChecking=no "$SshUser@$VmIp" "cd /opt/projeto-assistencia-v2 && docker compose restart backend-api backend-worker frontend"
    }
    "seed-admin" {
        Write-Host "==> Resetando/criando admin local..." -ForegroundColor Green
        & ssh.exe -i "$SshKey" -o StrictHostKeyChecking=no "$SshUser@$VmIp" "cd /opt/projeto-assistencia-v2 && docker compose run --rm --no-deps --entrypoint ./seedadmin backend-api"
    }
}
