<#
  connect-brain-windows.ps1 — conecta ESTE equipo Windows al CEREBRO CENTRAL Musubi, DESDE 0.
  Un comando: instala/verifica Tailscale, aplica el fix de firewall que destraba NordVPN,
  persiste el token, cablea el .mcp.json del proyecto, y VERIFICA de verdad (con node, el
  mismo runtime que usa Claude Code) que el cerebro responde y autentica.

  Uso (PowerShell — se auto-eleva a admin si hace falta):
    $env:MUSUBI_TOKEN="<token>"; .\connect-brain-windows.ps1 -ProjectDir "C:\ruta\al\proyecto"
    # si no pasás -ProjectDir usa el directorio actual.

  Parámetros:
    -BrainIp           IP de TU cerebro en el tailnet   (REQUERIDO: no hay default)
    -BrainPort         puerto del cerebro               (default 7717)
    -TailscaleAuthKey  (opcional) auth key para unir la malla sin abrir el navegador
    -SkipFirewall      no tocar el firewall (si ya lo configuraste a mano)
#>
param(
  [string]$ProjectDir      = (Get-Location).Path,
  [string]$BrainIp         = "",
  [int]$BrainPort          = 7717,
  [string]$TailscaleAuthKey = "",
  [switch]$SkipFirewall
)
$ErrorActionPreference = "Stop"
$brainReadyz = "http://${BrainIp}:${BrainPort}/readyz"
$brainUrl    = "http://${BrainIp}:${BrainPort}/mcp"
$tailnetCidr = "100.64.0.0/10"   # rango CGNAT de Tailscale

function Ok($m){   Write-Host "OK  $m" -ForegroundColor Green }
function Info($m){ Write-Host "--> $m" -ForegroundColor Cyan }
function Warn($m){ Write-Host "!   $m" -ForegroundColor Yellow }
function Die($m){  Write-Host "X   $m" -ForegroundColor Red; exit 1 }

# -BrainIp es REQUERIDO: apunta a TU cerebro central (antes defaulteaba a la infra del autor,
# lo que rompía la adopción por terceros — Track 16 F4).
if ([string]::IsNullOrWhiteSpace($BrainIp)) {
  Die "Falta -BrainIp: pasá la IP de TU cerebro central de Musubi (ej: -BrainIp 100.x.y.z)."
}

# ── 0. Elevar a administrador (el fix de firewall lo necesita) ────────────────
$isAdmin = ([Security.Principal.WindowsPrincipal] `
  [Security.Principal.WindowsIdentity]::GetCurrent()
).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin -and -not $SkipFirewall) {
  Info "Elevando a administrador (para las reglas de firewall del tailnet)…"
  $argList = @(
    "-NoProfile","-ExecutionPolicy","Bypass","-File","`"$PSCommandPath`"",
    "-ProjectDir","`"$ProjectDir`"","-BrainIp",$BrainIp,"-BrainPort",$BrainPort
  )
  if ($TailscaleAuthKey) { $argList += @("-TailscaleAuthKey","`"$TailscaleAuthKey`"") }
  # el token viaja por el entorno del proceso elevado, no por la línea de comandos
  Start-Process powershell -Verb RunAs -ArgumentList $argList `
    -Environment @{ MUSUBI_TOKEN = $env:MUSUBI_TOKEN }
  exit 0
}

# ── 1. Token ──────────────────────────────────────────────────────────────────
$token = $env:MUSUBI_TOKEN
if ([string]::IsNullOrWhiteSpace($token)) {
  $token = Read-Host -AsSecureString "Pega el MUSUBI_TOKEN del cerebro" |
           ForEach-Object { [Runtime.InteropServices.Marshal]::PtrToStringAuto(
             [Runtime.InteropServices.Marshal]::SecureStringToBSTR($_)) }
}
if ([string]::IsNullOrWhiteSpace($token)) { Die "Token vacío." }
setx MUSUBI_TOKEN "$token" | Out-Null          # persiste para futuras sesiones
$env:MUSUBI_TOKEN = $token                       # y para la actual
Ok "MUSUBI_TOKEN guardado (variable de usuario + sesión actual)"

# ── 2. Tailscale: instalar si falta + unir a la malla ─────────────────────────
$tsExe = "C:\Program Files\Tailscale\tailscale.exe"
if (-not (Test-Path $tsExe) -and -not (Get-Command tailscale -ErrorAction SilentlyContinue)) {
  Info "Tailscale no está: instalando…"
  if (Get-Command winget -ErrorAction SilentlyContinue) {
    winget install --id Tailscale.Tailscale -e --silent `
      --accept-source-agreements --accept-package-agreements
  } else {
    $msi = Join-Path $env:TEMP "tailscale-setup.msi"
    Invoke-WebRequest "https://pkgs.tailscale.com/stable/tailscale-setup-latest.msi" -OutFile $msi
    Start-Process msiexec.exe -Wait -ArgumentList "/i `"$msi`" /qn"
  }
  Start-Sleep -Seconds 3
}
if (-not (Test-Path $tsExe)) {
  $tsCmd = (Get-Command tailscale -ErrorAction SilentlyContinue).Source
  if ($tsCmd) { $tsExe = $tsCmd } else { Die "No encuentro tailscale.exe tras instalar." }
}
# ¿ya está en la malla?
$tsUp = $false
try { & $tsExe status 2>$null | Out-Null; if ($LASTEXITCODE -eq 0) { $tsUp = $true } } catch {}
if (-not $tsUp) {
  Info "Uniendo este equipo a la malla Tailscale…"
  if ($TailscaleAuthKey) { & $tsExe up --authkey $TailscaleAuthKey }
  else { & $tsExe up }    # abre el navegador para login
}
Ok "Tailscale presente"

# ── 3. FIX DE FIREWALL: dejar pasar el tailnet pese a NordVPN ──────────────────
#   Este es el paso que destraba la convivencia con NordVPN en Windows: reglas de
#   Windows Firewall que PERMITEN explícitamente el rango del tailnet (100.64.0.0/10)
#   en ambos sentidos, ganándole al filtro WFP de NordVPN. Idempotente.
if (-not $SkipFirewall) {
  foreach ($r in @(
    @{ Name="TS-Allow-Tailnet-Out"; Dir="Outbound" },
    @{ Name="TS-Allow-Tailnet-In";  Dir="Inbound"  }
  )) {
    Get-NetFirewallRule -DisplayName $r.Name -ErrorAction SilentlyContinue |
      Remove-NetFirewallRule -ErrorAction SilentlyContinue
    New-NetFirewallRule -DisplayName $r.Name -Direction $r.Dir -Action Allow `
      -RemoteAddress $tailnetCidr -Profile Any -Enabled True | Out-Null
  }
  Ok "Firewall: tailnet ($tailnetCidr) permitido (reglas TS-Allow-Tailnet-In/Out)"
} else {
  Warn "Firewall: omitido (-SkipFirewall)"
}

# ── 4. .mcp.json del proyecto (agrega/reemplaza la entrada remota "cerebro") ──
$mcpPath = Join-Path $ProjectDir ".mcp.json"
if (Test-Path $mcpPath) { $cfg = Get-Content $mcpPath -Raw | ConvertFrom-Json }
else { $cfg = [pscustomobject]@{} }
if (-not ($cfg.PSObject.Properties.Name -contains "mcpServers")) {
  $cfg | Add-Member -NotePropertyName mcpServers -NotePropertyValue ([pscustomobject]@{})
}
$entry = [pscustomobject]@{
  type    = "http"
  url     = $brainUrl
  headers = [pscustomobject]@{ Authorization = 'Bearer ${MUSUBI_TOKEN}' }
}
$servers = @{}
foreach ($p in $cfg.mcpServers.PSObject.Properties) { $servers[$p.Name] = $p.Value }
$servers["musubi-cerebro"] = $entry
$cfg.mcpServers = [pscustomobject]$servers
($cfg | ConvertTo-Json -Depth 10) | Set-Content -Path $mcpPath -Encoding utf8
Ok ".mcp.json actualizado (entrada 'musubi-cerebro') en $mcpPath"

# ── 5. Verificación REAL con node (el runtime que usa Claude Code) ────────────
#   Clave: verificamos con node.exe, NO con curl.exe. Bajo NordVPN el split-tunnel
#   excluye node (lo usa Claude Code) pero NO siempre a curl → verificar con curl
#   da falsos negativos. node es la prueba que importa.
$node = (Get-Command node -ErrorAction SilentlyContinue).Source
if (-not $node) { Die "Falta node.exe (runtime de Claude Code). Instalá Node.js y reintentá." }

Info "Verificando alcance + auth al cerebro (con node)…"
$checkJs = @'
const http = require("http");
const [ready, url, token] = process.argv.slice(2);
function get(u){return new Promise((res)=>{http.get(u,r=>{let d="";r.on("data",c=>d+=c);r.on("end",()=>res({c:r.statusCode,d}));}).on("error",e=>res({c:0,d:e.message}));});}
function post(u,tok,body){return new Promise((res)=>{const {hostname,port,pathname}=new URL(u);const req=http.request({hostname,port,path:pathname,method:"POST",headers:{"Content-Type":"application/json","Authorization":"Bearer "+tok}},r=>{let d="";r.on("data",c=>d+=c);r.on("end",()=>res({c:r.statusCode,d}));});req.on("error",e=>res({c:0,d:e.message}));req.write(body);req.end();});}
(async()=>{
  const rz=await get(ready);
  if(rz.c!==200){console.log("REACH_FAIL "+rz.c+" "+rz.d);process.exit(2);}
  console.log("REACH_OK "+rz.d);
  const tl=await post(url,token,JSON.stringify({jsonrpc:"2.0",id:1,method:"tools/list"}));
  if(tl.c===200 && tl.d.includes('"tools"')){console.log("AUTH_OK");process.exit(0);}
  console.log("AUTH_FAIL "+tl.c+" "+tl.d.slice(0,160));process.exit(3);
})();
'@
$checkFile = Join-Path $env:TEMP "musubi-brain-check.js"
Set-Content -Path $checkFile -Value $checkJs -Encoding utf8
$out = & $node $checkFile $brainReadyz $brainUrl $token 2>&1
$code = $LASTEXITCODE
Remove-Item $checkFile -ErrorAction SilentlyContinue
$out | ForEach-Object { Write-Host "    $_" -ForegroundColor DarkGray }

if ($code -eq 0) {
  Ok "Cerebro alcanzable Y autenticando. node llega al tailnet con NordVPN activo."
  Write-Host ""
  Ok "EQUIPO CONECTADO. Este proyecto ($ProjectDir) ya usa el cerebro central."
} else {
  Write-Host ""
  Die @"
node NO alcanza el cerebro. El firewall ya está permitido; falta el split-tunnel de NordVPN.
Hacé esto UNA vez (es GUI, no hay CLI):
  1) NordVPN -> Settings -> Connection -> Protocolo = OpenVPN (UDP)   (NordLynx ignora el split-tunnel)
  2) NordVPN -> Settings -> Split Tunneling -> "Disable VPN for selected apps", agregá:
        C:\Program Files\Tailscale\tailscaled.exe
        $node   (node.exe = el runtime de Claude Code)
  3) NordVPN: Disconnect -> esperar -> Connect  (reaplica el split-tunnel limpio)
  4) Tailscale: bandeja -> Exit -> reabrir, esperá "Connected"  (Tailscale PRIMERO, NordVPN DESPUÉS)
  5) Volvé a correr este script.
Regla: cada cambio en la lista de split-tunnel reconecta NordVPN -> reconectá limpio después.
"@
}
