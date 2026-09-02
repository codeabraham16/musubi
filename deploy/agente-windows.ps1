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
  LA CONSECUENCIA DE ESA ELECCION, QUE HAY QUE SABER ANTES Y NO DESPUES

  El disparador por defecto es AL INICIAR SESION. O sea: **si la maquina esta encendida y nadie
  inicio sesion, el agente NO corre**. Un equipo que se reinicio de madrugada y quedo en la
  pantalla de bloqueo figura CAIDO en la flota, y esta perfectamente vivo.

  No es teorico: paso, y se leyo mal durante dos dias. La maquina `gio` figuraba apagada mientras
  respondia al ping por el tailnet en 145 ms. La maquina andaba; el agente no estaba corriendo.

  Con -AlArranque la tarea se registra AL ARRANQUE del sistema y corre como SYSTEM, sin necesidad
  de que nadie inicie sesion. Es opt-in y no el default a proposito, porque **cambia lo que la
  flota puede hacer en esta maquina**: `musubi_fleet_exec` sobre ella pasaria a ejecutarse como
  SYSTEM en vez de como el usuario. Esa es una decision de seguridad, no una comodidad, y quien
  la toma tiene que verla escrita.

  --------------------------------------------------------------------------------------------
  EL TOKEN QUE PIDE NO ES EL TUYO

  -DeviceToken es la credencial DEL DISPOSITIVO, que sale de `musubi_fleet_enroll` y se muestra
  UNA sola vez. No es el token de una persona y no sirve para /mcp: solo para POST
  /fleet/heartbeat. Son dos almacenes de credenciales que no se cruzan, ni siquiera por el
  nombre de la variable de entorno (MUSUBI_DEVICE_TOKEN vs MUSUBI_TOKEN).

  Uso:
    .\agente-windows.ps1 -BrainUrl "http://100.79.126.62:7717" -DeviceToken "<el del enroll>"

  Sin la ventana de consola (recomendado en un escritorio):
    .\agente-windows.ps1 -BrainUrl "..." -DeviceToken "..." -Oculto

  Para desinstalar:
    Unregister-ScheduledTask -TaskName "Musubi Agente de Flota" -Confirm:$false
#>
param(
  [Parameter(Mandatory=$true)][string]$BrainUrl,
  [Parameter(Mandatory=$true)][string]$DeviceToken,
  # Destinos que esta maquina tiene que poder alcanzar (A67): `host:puerto`, separados por coma,
  # hasta 4. Sin esto la maquina NO emite `musubi_fleet_device_reach_up`, y eso es lo correcto:
  # "nadie le pidio que mirara" no es "no llega". Es el unico punto de vista que puede responder
  # si un cliente alcanza el relay; el sondeo desde el servidor mira el lugar donde siempre anda.
  [string]$Alcance = "",
  [string]$ExePath = "",
  [string]$InstallDir = "$env:LOCALAPPDATA\Musubi",
  [switch]$SkipFirewall,
  # Registra la tarea AL ARRANQUE y como SYSTEM, para que el agente no dependa de que alguien
  # inicie sesion. Exige administrador. Ver el bloque de arriba: cambia lo que la flota puede
  # hacer en esta maquina.
  [switch]$AlArranque,
  # SIN VENTANA DE CONSOLA, siguiendo como el usuario que inicia sesion.
  #
  # ------------------------------------------------------------------------------------------
  # ESTO NO ES COMODIDAD: LA VENTANA MATA AGENTES
  #
  # Medido el 2026-08-30. La maquina `gio` llevaba TRES DIAS fuera de la flota y el motivo estaba
  # en el resultado de su tarea: LastTaskResult 3221225786 = 0xC000013A = STATUS_CONTROL_C_EXIT.
  # Nadie la apago ni fallo la red: alguien cerro la consola negra porque estorbaba. Una pieza de
  # infraestructura que vive dentro de una ventana molesta se apaga sola, tarde o temprano.
  #
  # POR QUE NO SE RESUELVE CON -AlArranque, que tambien la esconde: correr como SYSTEM pone al
  # agente en la SESION 0, y ahi no hay a quien dibujarle. El eje de consentimiento (A57) deja de
  # funcionar en esa maquina: `puede_preguntar` pasa a false y `pide` se endurece a `prohibido`.
  # Esconder la ventana no deberia costar la capacidad de preguntarle a la persona.
  #
  # Con -Oculto el agente sigue siendo VOS, en TU sesion, y puede avisarte. Lo que no cambia es
  # que muere al cerrar sesion: eso lo arregla -AlArranque y su costo.
  [switch]$Oculto
)
$ErrorActionPreference = "Stop"
$TaskName = "Musubi Agente de Flota"

# SE AVISA AL PRINCIPIO, NO AL FINAL.
#
# Registrar una tarea con disparador "al iniciar sesion" exige administrador. Descubrirlo en el
# ULTIMO paso significa haber copiado el binario, escrito el token y probado el latido para nada
# -- y peor: deja la maquina a medio instalar, con el agente puesto y sin nada que lo arranque.
# El chequeo cuesta tres lineas y va antes de tocar el disco.
$soyAdminInicial = ([Security.Principal.WindowsPrincipal] `
  [Security.Principal.WindowsIdentity]::GetCurrent()
).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $soyAdminInicial) {
  Write-Host "ERR Esta PowerShell NO es de administrador." -ForegroundColor Red
  Write-Host "ERR Registrar la tarea programada lo exige, y sin tarea el agente no arranca solo." -ForegroundColor Red
  Write-Host "ERR Abri PowerShell como administrador y volve a correr esta misma linea." -ForegroundColor Red
  exit 1
}

function Paso($m) { Write-Host "->  $m" -ForegroundColor Cyan }
function Bien($m) { Write-Host "OK  $m" -ForegroundColor Green }
function Mal($m)  { Write-Host "ERR $m" -ForegroundColor Red }
# Aviso es lo que NO impide instalar pero hay que enterarse. Va en amarillo y no en rojo a
# proposito: mezclarlo con los errores es como se aprende a no leer ninguno de los dos.
function Aviso($m) { Write-Host "!   $m" -ForegroundColor Yellow }

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
# CON -AlArranque, EL AGENTE NO ES VOS: ES SYSTEM. Y ESTE ENDURECIMIENTO LO DEJABA AFUERA.
#
# El $false de SetAccessRuleProtection dice "corta la herencia y NO copies las reglas heredadas",
# asi que despues de la linea de arriba queda UNA sola regla: la del usuario que instalo. Con la
# tarea corriendo como SYSTEM, el agente no puede leer su propio token y muere al arrancar.
#
# Medido el 2026-09-02 en `gio`: la tarea decia "Attempted to run", volvia a Ready al instante, y
# la maquina quedaba fuera de la flota sin un solo mensaje de error.
#
# Y EL INSTALADOR NO LO VEIA, que es la parte que importa: su prueba de latido pasa el token por
# VARIABLE DE ENTORNO y corriendo como el usuario, asi que nunca abre el archivo. Probaba un
# camino distinto del que la tarea iba a usar, y por eso daba verde sobre una instalacion muerta.
#
# Negarle el archivo a SYSTEM tampoco era una defensa real: SYSTEM puede tomar posesion de
# cualquier archivo de la maquina cuando quiera. Lo unico que lograba era romper el arranque.
if ($AlArranque) {
  $acl.SetAccessRule((New-Object System.Security.AccessControl.FileSystemAccessRule(
    "NT AUTHORITY\SYSTEM", "FullControl", "Allow")))
}
Set-Acl $tokenFile $acl
if ($AlArranque) {
  Paso "token guardado, legible por $env:USERNAME y por SYSTEM (la tarea corre como SYSTEM)"
} else {
  Paso "token guardado, solo legible por $env:USERNAME"
}

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

# -- 3b. El script de cambio de binario ------------------------------------------------------
# VIAJA CON EL INSTALADOR, no se escribe a mano en cada maquina.
#
# Hasta el 2026-09-02 este archivo existia SOLO en las dos maquinas Windows, creado a mano y sin
# versionar. Nadie lo revisaba, no tenia pruebas, y sus dos fallas -tomar la carpeta de
# %LOCALAPPDATA% y no matar al proceso viejo- sobrevivieron meses porque no habia donde verlas.
# Se copia del lado del instalador si esta presente; si no, se avisa y se sigue.
$cambiador = Join-Path $PSScriptRoot "cambiar-agente.cmd"
if (Test-Path $cambiador) {
  Copy-Item -Force $cambiador (Join-Path $InstallDir "cambiar-agente.cmd")
  Paso "cambiar-agente.cmd instalado"
} else {
  Aviso "no vino cambiar-agente.cmd al lado del instalador: esta maquina no va a poder actualizarse sola"
}

# -- 4. La tarea programada ------------------------------------------------------------------
$lanzador = Join-Path $InstallDir "agente.cmd"
# La linea sale VACIA cuando no se paso -Alcance: un `set MUSUBI_ALCANCE=` a secas dejaria la
# variable definida y vacia, que el agente tendria que distinguir de ausente. Mejor no escribirla.
$lineaAlcance = ""
if ($Alcance -ne "") { $lineaAlcance = "`r`nset MUSUBI_ALCANCE=$Alcance" }
@"
@echo off
set MUSUBI_BRAIN_URL=$BrainUrl$lineaAlcance
set /p MUSUBI_DEVICE_TOKEN=<"$tokenFile"
"$destino" agent
"@ | Set-Content -Encoding ASCII $lanzador

if (Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue) {
  Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
}
# EL LANZADOR OCULTO, CON UN DETALLE QUE NO ES OPCIONAL: `Run(..., 0, True)`.
#
# El tercer argumento es "esperar a que termine". Con False, el .vbs lanza el agente y sale, la
# tarea se da por CUMPLIDA en el acto y el `-RestartCount 999` de arriba deja de protegerla: si el
# agente muere, nadie lo levanta porque para el planificador ya termino bien. Con True, wscript se
# queda esperando, la tarea sigue en Running y el reinicio automatico sigue valiendo.
#
# Es exactamente la clase de detalle que convierte un arreglo en una regresion silenciosa: la
# ventana desaparece --que es lo que se ve-- y la red de contencion tambien --que es lo que no--.
if ($Oculto) {
  $shim = Join-Path $InstallDir "agente-oculto.vbs"
  @"
' Lanza el agente SIN ventana. El 0 es el estilo de ventana (oculta); el True es esperar a que
' termine, y sin el la tarea se daria por cumplida al instante y perderia su reinicio automatico.
CreateObject("WScript.Shell").Run """$lanzador""", 0, True
"@ | Set-Content -Encoding ASCII $shim
  Paso "lanzador oculto en $shim"
  $accion = New-ScheduledTaskAction -Execute "wscript.exe" -Argument "//B ""$shim"""
} else {
  $accion = New-ScheduledTaskAction -Execute $lanzador
}
$ajustes  = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries `
              -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit 0 `
              -MultipleInstances IgnoreNew

# EL DISPARADOR ES LA DIFERENCIA ENTRE "figura caida" Y "esta caida".
#
# Al iniciar sesion (default): el agente vive mientras haya alguien logueado. Una maquina
# encendida en la pantalla de bloqueo no reporta, y desde el cerebro se ve identica a una
# apagada.
#
# Al arranque como SYSTEM (-AlArranque): el agente vive mientras la maquina viva. Cuesta
# elevacion y le da a `musubi_fleet_exec` privilegios de SYSTEM sobre este equipo.
if ($AlArranque) {
  if (-not $soyAdminInicial) {
    Mal "-AlArranque exige administrador: registrar una tarea como SYSTEM no se puede sin elevacion."
    Mal "Abri PowerShell como administrador, o saca -AlArranque para la tarea por sesion."
    exit 1
  }
  $gatillo   = New-ScheduledTaskTrigger -AtStartup
  $principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
  Register-ScheduledTask -TaskName $TaskName -Action $accion -Trigger $gatillo -Settings $ajustes `
    -Principal $principal `
    -Description "Mide este host y late contra el cerebro Musubi. Solo sale; no escucha. Corre como SYSTEM al arranque." | Out-Null
  Bien "tarea registrada AL ARRANQUE como SYSTEM: sobrevive a un reinicio sin que nadie inicie sesion"
  Write-Host "  OJO: musubi_fleet_exec sobre esta maquina ahora corre como SYSTEM." -ForegroundColor Yellow
} else {
  $gatillo = New-ScheduledTaskTrigger -AtLogOn
  Register-ScheduledTask -TaskName $TaskName -Action $accion -Trigger $gatillo -Settings $ajustes `
    -Description "Mide este host y late contra el cerebro Musubi. Solo sale; no escucha." | Out-Null
  Write-Host "  El agente corre AL INICIAR SESION: si esta maquina se reinicia y nadie se loguea," -ForegroundColor Yellow
  Write-Host "  va a figurar CAIDA en la flota estando viva. Para que sobreviva un reinicio," -ForegroundColor Yellow
  Write-Host "  reinstala con -AlArranque (exige admin y corre como SYSTEM)." -ForegroundColor Yellow
}
Start-ScheduledTask -TaskName $TaskName
Start-Sleep -Seconds 5
$estado = (Get-ScheduledTask -TaskName $TaskName).State
Bien "tarea '$TaskName' creada y $estado"
Write-Host ""
Write-Host "Comprobalo desde el cerebro con musubi_fleet_list." -ForegroundColor Yellow
