<#
  connect-brain-windows.ps1 — conecta ESTE equipo Windows al CEREBRO CENTRAL Musubi.
  Automatiza lo que se puede desde Windows: token en el entorno, el .mcp.json remoto
  del proyecto, y la verificación. El split-tunnel de NordVPN es GUI (no hay CLI), así
  que ese paso se imprime para que lo hagas a mano una sola vez.

  Uso (PowerShell):
    $env:MUSUBI_TOKEN="<token>"; .\connect-brain-windows.ps1 -ProjectDir "C:\ruta\al\proyecto"
    # si no pasás -ProjectDir usa el directorio actual.

  Parámetros:
    -BrainIp    IP del cerebro en el tailnet   (default 100.79.126.62)
    -BrainPort  puerto del cerebro             (default 7717)
#>
param(
  [string]$ProjectDir = (Get-Location).Path,
  [string]$BrainIp   = "100.79.126.62",
  [int]$BrainPort    = 7717
)
$ErrorActionPreference = "Stop"
$brainUrl = "http://${BrainIp}:${BrainPort}/mcp"
function Ok($m){ Write-Host "OK  $m" -ForegroundColor Green }
function Info($m){ Write-Host "--> $m" -ForegroundColor Cyan }
function Die($m){ Write-Host "X   $m" -ForegroundColor Red; exit 1 }

# ── 1. Token ────────────────────────────────────────────────────────────────
$token = $env:MUSUBI_TOKEN
if ([string]::IsNullOrWhiteSpace($token)) {
  $token = Read-Host -AsSecureString "Pega el MUSUBI_TOKEN del cerebro" |
           ForEach-Object { [Runtime.InteropServices.Marshal]::PtrToStringAuto(
             [Runtime.InteropServices.Marshal]::SecureStringToBSTR($_)) }
}
if ([string]::IsNullOrWhiteSpace($token)) { Die "Token vacío." }
# Persistir para futuras sesiones (variable de usuario) + la sesión actual.
setx MUSUBI_TOKEN "$token" | Out-Null
$env:MUSUBI_TOKEN = $token
Ok "MUSUBI_TOKEN guardado (variable de usuario + sesión actual)"

# ── 2. .mcp.json del proyecto (crear o mergear la entrada remota "cerebro") ──
$mcpPath = Join-Path $ProjectDir ".mcp.json"
if (Test-Path $mcpPath) {
  $cfg = Get-Content $mcpPath -Raw | ConvertFrom-Json
} else {
  $cfg = [pscustomobject]@{}
}
if (-not ($cfg.PSObject.Properties.Name -contains "mcpServers")) {
  $cfg | Add-Member -NotePropertyName mcpServers -NotePropertyValue ([pscustomobject]@{})
}
$entry = [pscustomobject]@{
  type    = "http"
  url     = $brainUrl
  headers = [pscustomobject]@{ Authorization = 'Bearer ${MUSUBI_TOKEN}' }
}
# Reemplaza/agrega la entrada 'musubi-cerebro' preservando las demás.
$servers = @{}
foreach ($p in $cfg.mcpServers.PSObject.Properties) { $servers[$p.Name] = $p.Value }
$servers["musubi-cerebro"] = $entry
$cfg.mcpServers = [pscustomobject]$servers
($cfg | ConvertTo-Json -Depth 10) | Set-Content -Path $mcpPath -Encoding utf8
Ok ".mcp.json actualizado (entrada 'musubi-cerebro') en $mcpPath"

# ── 3. Verificación ─────────────────────────────────────────────────────────
Info "Verificando alcance al cerebro..."
try {
  $r = curl.exe -fsS "http://${BrainIp}:${BrainPort}/readyz" 2>$null
  if ($r -match "ready") { Ok "Cerebro alcanzable (/readyz -> $r)" }
  else { throw "readyz sin respuesta esperada" }
} catch {
  Write-Host ""
  Die @"
No se alcanza el cerebro desde este equipo.
Casi seguro es NordVPN (split-tunnel por-app). Hacé esto UNA vez:
  1) NordVPN -> Settings -> Connection -> Protocolo = OpenVPN (UDP)
  2) NordVPN -> Settings -> Split Tunneling -> "Disable VPN for selected apps"
     Agregá:  C:\Program Files\Tailscale\tailscaled.exe
              C:\Windows\System32\curl.exe
              node.exe  (el runtime de Claude Code; ver:  (Get-Command node).Source)
  3) Reconectá NordVPN
  4) Reiniciá Tailscale (bandeja -> Exit -> abrir), esperá "Connected"
     (ORDEN: Tailscale PRIMERO conectado, NordVPN despues)
  5) Volvé a correr este script.
"@
}

# Auth con el token
$body = '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
$auth = curl.exe -fsS -H "Authorization: Bearer $token" -X POST $brainUrl -d $body 2>$null
if ($auth -match '"tools"') { Ok "Autenticación OK: el cerebro devuelve el catálogo de tools" }
else { Die "Llega el readyz pero el token no autentica. Revisá el MUSUBI_TOKEN." }

Write-Host ""
Ok "EQUIPO CONECTADO. Este proyecto ($ProjectDir) ya usa el cerebro central."
Write-Host "Recordá: agregar node.exe al split-tunnel de NordVPN para que Claude Code alcance el cerebro." -ForegroundColor Yellow
