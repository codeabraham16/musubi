<#
  agente-windows.ps1 - instala el AGENTE DE FLOTA de Musubi (Tier A) en este equipo Windows.

  --------------------------------------------------------------------------------------------
  QUE HACE Y QUE NO

  Deja `musubi agent` latiendo contra el cerebro: mide CPU, memoria, disco, carga y uptime de
  ESTE host y los reporta cada 30 s. No abre ningun puerto, no acepta conexiones entrantes, y
  no instala servicio de sistema - corre como TAREA PROGRAMADA del usuario, que arranca al
  iniciar sesion y se reinicia sola si el proceso muere.

  POR QUE TAREA PROGRAMADA Y NO SERVICIO: un servicio de Windows exige elevacion y un envoltorio
  (Go no produce un binario de servicio nativo). Una tarea programada da reinicio automatico y
  arranque desatendido sin nada de eso. Si preferis un servicio, NSSM o WinSW envuelven el mismo
  comando - pero eso es una decision de despliegue, no un requisito del agente.

  --------------------------------------------------------------------------------------------
  EL TOKEN QUE PIDE NO ES EL TUYO

  -DeviceToken es la credencial DEL DISPOSITIVO, que sale de `musubi_fleet_enroll` y se muestra
  UNA sola vez. No es el token de una persona y no sirve para /mcp: solo para POST
  /fleet/heartbeat. Son dos almacenes de credenciales que no se cruzan, ni siquiera por el
  nombre de la variable de entorno (MUSUBI_DEVICE_TOKEN vs MUSUBI_TOKEN).

  Uso:
    .\agente-windows.ps1 -BrainUrl "http://100.79.126.62:7717" -DeviceToken "<el del enroll>"

  Para desinstalar:
    Unregister-ScheduledTask -TaskName "Musubi Agente de Flota" -Confirm:$false
#>
param(
  [Parameter(Mandatory=$true)][string]$BrainUrl,
  [Parameter(Mandatory=$true)][string]$DeviceToken,
  [string]$ExePath = "",
  [string]$InstallDir = "$env:LOCALAPPDATA\Musubi",
  [switch]$SkipFirewall
)
$ErrorActionPreference = "Stop"
$TaskName = "Musubi Agente de Flota"

function Paso($m) { Write-Host "->  $m" -ForegroundColor Cyan }
function Bien($m) { Write-Host "OK  $m" -ForegroundColor Green }
function Mal($m)  { Write-Host "ERR $m" -ForegroundColor Red }

# -- 1. El binario ---------------------------------------------------------------------------
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$destino = Join-Path $InstallDir "musubi.exe"
if ($ExePath -eq "") { $ExePath = Join-Path (Get-Location) "musubi.exe" }
if (-not (Test-Path $ExePath)) { Mal "no encuentro musubi.exe en '$ExePath'. Pasa -ExePath."; exit 1 }
Copy-Item -Force $ExePath $destino
Paso "binario en $destino"
& $destino --version

# -- 2. El token, por ARCHIVO ----------------------------------------------------------------
# Nunca en la linea de comandos de la tarea: eso lo deja visible en el Administrador de tareas
# y en cualquier listado de procesos. El archivo se restringe al usuario actual.
$tokenFile = Join-Path $InstallDir "device.token"
[IO.File]::WriteAllText($tokenFile, $DeviceToken)   # sin BOM y sin salto de linea final
$acl = Get-Acl $tokenFile
$acl.SetAccessRuleProtection($true, $false)         # corta la herencia
$acl.SetAccessRule((New-Object System.Security.AccessControl.FileSystemAccessRule(
  "$env:USERDOMAIN\$env:USERNAME", "FullControl", "Allow")))
Set-Acl $tokenFile $acl
Paso "token guardado, solo legible por $env:USERNAME"

# -- 3. Prueba UN latido antes de crear nada -------------------------------------------------
# Si la credencial o la URL estan mal, se ve ACA y no dentro de una tarea que falla en silencio.
Paso "probando un latido contra $BrainUrl ..."
$env:MUSUBI_BRAIN_URL  = $BrainUrl
$env:MUSUBI_DEVICE_TOKEN = $DeviceToken
& $destino agent --once

# EL FIREWALL SE TOCA SOLO SI HIZO FALTA, Y RECIEN DESPUES DE FALLAR.
#
# NordVPN filtra por WFP y bloquea el rango del tailnet; el sintoma es un WSAEACCES
# ("access to a socket in a way forbidden by its access permissions") que NO menciona
# firewall por ningun lado y manda a depurar la red. El arreglo son dos reglas que permiten
# 100.64.0.0/10 explicitamente, y le ganan al filtro.
#
# Por que despues y no antes: agregar reglas de firewall a una maquina que no las necesita es
# dejar puesta una excepcion permanente para arreglar un problema que no tenia. Se prueba,
# y solo si el sintoma aparece se aplica el arreglo.
if ($LASTEXITCODE -ne 0 -and -not $SkipFirewall) {
  $soyAdmin = ([Security.Principal.WindowsPrincipal] `
    [Security.Principal.WindowsIdentity]::GetCurrent()
  ).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
  if (-not $soyAdmin) {
    Mal "el latido fallo y no soy administrador, asi que no puedo tocar el firewall."
    Mal "Reabri PowerShell como administrador y volve a correr esto."
    exit 1
  }
  Paso "el latido fallo; aplicando el permiso de firewall para el tailnet (100.64.0.0/10)"
  foreach ($r in @(@{N="TS-Allow-Tailnet-Out";D="Outbound"}, @{N="TS-Allow-Tailnet-In";D="Inbound"})) {
    Get-NetFirewallRule -DisplayName $r.N -ErrorAction SilentlyContinue |
      Remove-NetFirewallRule -ErrorAction SilentlyContinue
    New-NetFirewallRule -DisplayName $r.N -Direction $r.D -Action Allow `
      -RemoteAddress "100.64.0.0/10" -Profile Any -Enabled True | Out-Null
  }
  Paso "reintentando el latido ..."
  & $destino agent --once
}

if ($LASTEXITCODE -ne 0) {
  Mal "el latido de prueba fallo; NO se crea la tarea."
  Mal "Una tarea que late contra un cerebro inalcanzable no monitorea nada y ADEMAS"
  Mal "te deja creyendo que si."
  Write-Host ""
  # ESTE CASO SE DESCUBRIO A LOS GOLPES Y CUESTA HORAS SI NO SE LO NOMBRA.
  #
  # NordVPN (y otros clientes VPN con filtrado por proceso) bloquean ejecutables SIN FIRMAR
  # aunque la red este perfecta. La firma del sintoma es exacta: curl.exe al MISMO host y
  # puerto devuelve HTTP 200, y este binario devuelve WSAEACCES. No es la red, no es el
  # firewall de Windows, no es el routing: es QUIEN abre el socket.
  #
  # Y las reglas de split tunneling son POR RUTA, asi que autorizar el .exe en una carpeta no
  # autoriza la copia instalada en otra. Por eso se imprime la ruta final y no una generica.
  if ($LASTEXITCODE -ne 0 -and $error[0] -match "forbidden|10013" -or $true) {
    Mal "SI EL ERROR DICE 'forbidden by its access permissions' (WSAEACCES):"
    Mal "  Es un VPN o un filtro que bloquea ESTE PROGRAMA, no la red."
    Mal "  Comprobalo:  curl.exe -sS -o NUL -w \"%{http_code}\" $BrainUrl/readyz"
    Mal "  Si curl da 200 y esto falla, es filtrado por proceso."
    Mal "  Arreglo en NordVPN: Settings > Split tunneling > Bypass VPN > Add app:"
    Mal "    $destino"
    Mal "  La regla es POR RUTA: autoriza EXACTAMENTE esa, no otra copia del .exe."
  }
  exit 1
}
Bien "el cerebro registro el latido"

# -- 4. La tarea programada ------------------------------------------------------------------
$lanzador = Join-Path $InstallDir "agente.cmd"
@"
@echo off
set MUSUBI_BRAIN_URL=$BrainUrl
set /p MUSUBI_DEVICE_TOKEN=<"$tokenFile"
"$destino" agent
"@ | Set-Content -Encoding ASCII $lanzador

if (Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue) {
  Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
}
$accion   = New-ScheduledTaskAction -Execute $lanzador
$gatillo  = New-ScheduledTaskTrigger -AtLogOn
$ajustes  = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries `
              -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit 0 `
              -MultipleInstances IgnoreNew
Register-ScheduledTask -TaskName $TaskName -Action $accion -Trigger $gatillo -Settings $ajustes `
  -Description "Mide este host y late contra el cerebro Musubi. Solo sale; no escucha." | Out-Null
Start-ScheduledTask -TaskName $TaskName
Start-Sleep -Seconds 5
$estado = (Get-ScheduledTask -TaskName $TaskName).State
Bien "tarea '$TaskName' creada y $estado"
Write-Host ""
Write-Host "Comprobalo desde el cerebro con musubi_fleet_list." -ForegroundColor Yellow
