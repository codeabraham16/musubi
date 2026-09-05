<#
  diagnostico-cortes-windows.ps1 - junta, en una corrida, todo lo que hace falta para saber POR QUE
  se corta una maquina Windows. Se lee, no se ejecuta nada destructivo.

  --------------------------------------------------------------------------------------------
  PARA QUE EXISTE

  `davantis-1` lleva quince cortes en diez dias. El evento 41 con BugcheckCode 0 y CERO errores
  WHEA descarta al sistema operativo: no llego a escribir nada. Lo que queda es fuente, corriente
  de pared, termica o placa — y ninguna de esas se ve desde la flota.

  Peor: la flota no distingue «la maquina se apago» de «el agente no esta corriendo», porque la
  tarea programada arranca AL INICIAR SESION. Una maquina que reinicio de madrugada y quedo en la
  pantalla de bloqueo figura CAIDA estando perfectamente viva. Ya paso con `gio`, y se leyo mal
  durante tres dias. La seccion 2 de este guion es la que responde esa pregunta primero, porque si
  la respuesta es «nadie inicio sesion» todo lo demas sobra.

  --------------------------------------------------------------------------------------------
  COMO SE USA

  Desde la maquina afectada, PowerShell COMO ADMINISTRADOR (el registro del sistema lo pide):

      .\diagnostico-cortes-windows.ps1

  Para guardarlo y mandarlo:

      .\diagnostico-cortes-windows.ps1 | Tee-Object -FilePath "$env:USERPROFILE\Desktop\cortes.txt"
#>
param([int]$Dias = 30)

$ErrorActionPreference = "Continue"
$sep = "=" * 78
function Titulo($t) { Write-Host ""; Write-Host $sep; Write-Host "  $t"; Write-Host $sep }

$id = [Security.Principal.WindowsIdentity]::GetCurrent()
$admin = (New-Object Security.Principal.WindowsPrincipal $id).IsInRole(
           [Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $admin) {
  Write-Host "AVISO: no sos administrador. El registro del sistema puede venir incompleto y" -ForegroundColor Yellow
  Write-Host "       eso se lee como 'no hubo eventos', que es distinto de 'no los pude ver'." -ForegroundColor Yellow
}

Titulo "1 - QUIEN ES ESTA MAQUINA Y HACE CUANTO ESTA ENCENDIDA"
$os = Get-CimInstance Win32_OperatingSystem
$cs = Get-CimInstance Win32_ComputerSystem
"   equipo:     {0}  ({1} {2})" -f $env:COMPUTERNAME, $cs.Manufacturer, $cs.Model
"   windows:    {0}  build {1}" -f $os.Caption, $os.BuildNumber
"   arranco:    {0}" -f $os.LastBootUpTime
"   encendida:  {0:N1} horas" -f ((Get-Date) - $os.LastBootUpTime).TotalHours
"   ahora:      {0}" -f (Get-Date)

Titulo "2 - EL AGENTE DE MUSUBI: CORRE O NO, Y POR QUE NO"
# ESTA SECCION VA SEGUNDA Y NO ULTIMA A PROPOSITO. Si la maquina esta encendida y el agente no
# corre, la flota la muestra caida y NO hubo ningun corte que investigar.
$t = Get-ScheduledTask -TaskName "Musubi Agente de Flota" -ErrorAction SilentlyContinue
if (-not $t) {
  Write-Host "   NO existe la tarea 'Musubi Agente de Flota'." -ForegroundColor Red
  "   El agente no esta instalado como tarea, o tiene otro nombre. Mira: Get-ScheduledTask *usubi*"
} else {
  $info = $t | Get-ScheduledTaskInfo
  "   estado:          {0}" -f $t.State
  "   ultima corrida:  {0}" -f $info.LastRunTime
  "   resultado:       {0}" -f $info.LastTaskResult
  "   disparadores:    {0}" -f (($t.Triggers | ForEach-Object { $_.CimClass.CimClassName }) -join ", ")
  "   corre como:      {0}" -f $t.Principal.UserId
  # El valor puede llegar como entero CON SIGNO: 3221225786 no entra en Int32 y aparecería como
  # -1073741510. Comparar contra el positivo fallaría en silencio, que es peor que no mirarlo.
  $res = [int64]$info.LastTaskResult
  if ($res -lt 0) { $res += 4294967296 }
  switch ($res) {
    3221225786 { Write-Host "   >> 3221225786 = 0xC000013A = alguien CERRO LA VENTANA de consola." -ForegroundColor Yellow
                 Write-Host "      Reinstala con -Oculto: una pieza de infraestructura dentro de una ventana que" -ForegroundColor Yellow
                 Write-Host "      molesta se apaga sola, tarde o temprano." -ForegroundColor Yellow }
    267009     { "   >> 267009 = la tarea esta corriendo ahora mismo. Bien." }
    0          { "   >> 0 = la ultima corrida termino sin error (puede haber terminado, igual)." }
  }
  if ($info.LastRunTime -lt $os.LastBootUpTime) {
    Write-Host "   >> LA TAREA NO CORRIO DESDE EL ULTIMO ARRANQUE." -ForegroundColor Red
    Write-Host "      Su disparador es AL INICIAR SESION y nadie inicio sesion. La maquina esta" -ForegroundColor Red
    Write-Host "      viva y la flota la ve muerta: no hay ningun corte que investigar aca." -ForegroundColor Red
    Write-Host "      Arreglo permanente: reinstalar con -AlArranque (corre como SYSTEM al arrancar)." -ForegroundColor Red
    Write-Host "      OJO: eso hace que musubi_fleet_exec corra como SYSTEM y apaga el eje de" -ForegroundColor Red
    Write-Host "      consentimiento en esta maquina. Es una decision de seguridad, no una comodidad." -ForegroundColor Red
  }
}
"   proceso musubi:  {0}" -f ((Get-Process musubi -ErrorAction SilentlyContinue | Measure-Object).Count)

Titulo "3 - LOS CORTES: EVENTO 41, CON TODOS SUS CAMPOS"
# El evento 41 se emite en el arranque SIGUIENTE y describe el apagon anterior. Sus campos son lo
# unico que separa las causas, y `Get-EventLog` NO los muestra: hay que leer el XML.
$e41 = Get-WinEvent -FilterHashtable @{LogName='System'; Id=41; StartTime=(Get-Date).AddDays(-$Dias)} -ErrorAction SilentlyContinue
if (-not $e41) { "   sin eventos 41 en $Dias dias (o sin permiso para leerlos)" }
else {
  "   {0} corte(s) sucio(s) en {1} dias" -f @($e41).Count, $Dias
  ""
  "   {0,-21} {1,8} {2,10} {3,11}  {4}" -f "cuando","bugcheck","boton","horas prev","lectura"
  $prev = $null
  foreach ($e in ($e41 | Sort-Object TimeCreated)) {
    $x = [xml]$e.ToXml()
    $d = @{}
    if ($x.Event.EventData -and $x.Event.EventData.Data) {
      foreach ($n in $x.Event.EventData.Data) { $d[$n.Name] = $n.'#text' }
    }
    $bug = $d['BugcheckCode']; $btn = $d['PowerButtonTimestamp']
    $gap = if ($prev) { "{0:N1}" -f ($e.TimeCreated - $prev).TotalHours } else { "-" }
    # LA LECTURA VA AL LADO DEL DATO, porque el numero solo no dice nada a quien no lo vio antes.
    $lec = if ($bug -ne '0' -and $bug) { "hubo pantalla azul: mira el volcado" }
           elseif ($btn -and $btn -ne '0') { "el BOTON de encendido, o el firmware apago" }
           else { "corte LIMPIO de energia: el SO no llego a escribir nada" }
    "   {0,-21} {1,8} {2,10} {3,11}  {4}" -f $e.TimeCreated.ToString("yyyy-MM-dd HH:mm:ss"), $bug, $btn, $gap, $lec
    $prev = $e.TimeCreated
  }
}

Titulo "4 - LO QUE DESCARTA (O NO) AL HARDWARE QUE WINDOWS SI VE"
$whea = Get-WinEvent -FilterHashtable @{LogName='System'; ProviderName='Microsoft-Windows-WHEA-Logger'; StartTime=(Get-Date).AddDays(-$Dias)} -ErrorAction SilentlyContinue
"   errores WHEA (CPU/memoria/bus): {0}" -f (($whea | Measure-Object).Count)
if (-not $whea) { "   >> cero WHEA con corte limpio deja afuera casi todo lo que el SO puede ver:" ; "      queda fuente, corriente de pared, termica o placa." }
$term = Get-WinEvent -FilterHashtable @{LogName='System'; Id=86,87; StartTime=(Get-Date).AddDays(-$Dias)} -ErrorAction SilentlyContinue
"   eventos termicos del procesador (86/87): {0}" -f (($term | Measure-Object).Count)
$kern = Get-WinEvent -FilterHashtable @{LogName='System'; Id=6008; StartTime=(Get-Date).AddDays(-$Dias)} -ErrorAction SilentlyContinue
"   apagados inesperados (6008): {0}" -f (($kern | Measure-Object).Count)

Titulo "5 - FAST STARTUP Y EL VOLCADO: DOS COSAS QUE ENSUCIAN EL DIAGNOSTICO"
$hib = (Get-ItemProperty "HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Power" -Name HiberbootEnabled -ErrorAction SilentlyContinue).HiberbootEnabled
"   Fast Startup (HiberbootEnabled): {0}" -f $(if ($null -eq $hib) { "no declarado" } elseif ($hib -eq 1) { "ENCENDIDO" } else { "apagado" })
if ($hib -eq 1) {
  Write-Host "   >> Encendido produce eventos 41 que NO son cortes y esconde reinicios reales:" -ForegroundColor Yellow
  Write-Host "      un 'apagar' es en realidad una hibernacion parcial. Apagalo mientras dure el" -ForegroundColor Yellow
  Write-Host "      diagnostico:  powercfg /h off      (tambien libera el hiberfil.sys del disco)" -ForegroundColor Yellow
}
$cd = (Get-ItemProperty "HKLM:\SYSTEM\CurrentControlSet\Control\CrashControl" -ErrorAction SilentlyContinue)
"   CrashDumpEnabled: {0}  (0=ninguno 1=completo 2=kernel 3=mini 7=automatico)" -f $cd.CrashDumpEnabled
"   AutoReboot:       {0}" -f $cd.AutoReboot
$pf = Get-CimInstance Win32_PageFileUsage -ErrorAction SilentlyContinue
"   pagefile en uso:  {0}" -f $(if ($pf) { "{0}  {1} MB" -f $pf.Name, $pf.AllocatedBaseSize } else { "NINGUNO" })
if (-not $pf) {
  Write-Host "   >> SIN pagefile no se puede escribir NINGUN volcado, asi que un cuelgue real" -ForegroundColor Yellow
  Write-Host "      tampoco dejaria rastro. Eso hace indistinguible 'corte de luz' de 'cuelgue'." -ForegroundColor Yellow
}

Titulo "6 - MEMORIA Y ENERGIA AHORA"
$mem = Get-CimInstance Win32_OperatingSystem
"   RAM: {0:N1} GB total, {1:N1} GB libre  ({2:N0} % usada)" -f ($mem.TotalVisibleMemorySize/1MB), ($mem.FreePhysicalMemory/1MB), (100-($mem.FreePhysicalMemory/$mem.TotalVisibleMemorySize*100))
$bat = Get-CimInstance Win32_Battery -ErrorAction SilentlyContinue
"   bateria/UPS detectada: {0}" -f $(if ($bat) { "$($bat.Name) - estado $($bat.BatteryStatus)" } else { "ninguna (equipo de escritorio sin UPS visible)" })
$plan = powercfg /getactivescheme 2>$null
"   plan de energia: {0}" -f ($plan -replace '^.*\(', '' -replace '\)$', '')

Titulo "QUE HACER CON ESTO"
@"
   Si la seccion 2 dice que la tarea no corrio desde el ultimo arranque, el problema de HOY es
   ese y no el hardware: la maquina esta viva y la flota la ve muerta.

   Si la seccion 3 muestra cortes limpios (bugcheck 0, boton 0) con WHEA en cero, el orden mas
   barato para atacarlos es:
     1. apagar Fast Startup (powercfg /h off) y volver a medir una semana;
     2. otro tomacorriente, en otro circuito, y el cable de alimentacion bien puesto;
     3. la fuente. Que aguante 30-40 h y corte sin avisar es el patron clasico de capacitores
        viejos, y es la pieza mas barata de descartar prestandola.

   La temperatura NO la reporta esta maquina, asi que la hipotesis termica hoy no se puede ni
   confirmar ni descartar. Si queres cerrarla, HWiNFO64 o LibreHardwareMonitor registrando a
   archivo durante un dia alcanza.
"@
