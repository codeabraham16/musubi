<#
  diagnostico-disco-windows.ps1 - contesta QUE se esta comiendo el disco de una maquina Windows
  de la flota, y por que dos contenedores no arrancan. SOLO MIDE: no borra, no poda, no compacta.

  --------------------------------------------------------------------------------------------
  PARA QUE EXISTE

  `davantis-1` viene con `DiscoPorLlenarse` puesta, y el numero solo no alcanza para decidir nada.
  Medido el 2026-09-23 desde el cerebro: 26,7 G libres de 232 G (11,5 %), **bajo el umbral del
  15 % el 76 % de los ultimos 30 dias**, y plano en 11,5 % las ultimas 30 horas. En el mes oscilo
  entre 11,4 G y 87,6 G. O sea: NO es una fuga que avanza hacia cero, es una maquina que vive al
  borde y se mueve en decenas de gigas. Eso cambia que hay que preguntar: no «cuando se llena»
  sino «que es lo que sube y baja».

  LA SOSPECHA PRINCIPAL, Y POR QUE SE MIDE ASI. Este repo ya pago esta leccion una vez: podar
  Docker NO devuelve disco. Se podaron 12,1 GB y Windows vio 2,24 — el `.vhdx` de WSL crece y
  **no encoge solo** cuando se borra adentro. Por eso la seccion 3 compara DOS numeros que la
  gente confunde: lo que el archivo ocupa EN EL DISCO contra lo que hay usado ADENTRO. Si el
  primero es mucho mayor que el segundo, el espacio no se recupera podando: hay que COMPACTAR, y
  eso es otra operacion, con la maquina y Docker parados.

  LO QUE ESTE GUION NO PUEDE HACER, dicho de frente: se escribio en Linux, donde no hay PowerShell
  para probarlo. La sintaxis NO se verifico ejecutandola. Todo va entre try/catch para que un
  bloque que falle no se lleve la corrida, pero si algo se rompe, el mensaje es del bloque — no
  hay que leerlo como un hallazgo sobre la maquina.

  --------------------------------------------------------------------------------------------
  COMO SE USA

  Desde la maquina afectada, PowerShell COMO ADMINISTRADOR (los tamanos de carpetas del sistema
  y `hiberfil.sys` lo piden; sin admin igual corre, pero varias lineas van a decir "sin permiso"):

      .\diagnostico-disco-windows.ps1

  Para guardarlo y mandarlo:

      .\diagnostico-disco-windows.ps1 | Tee-Object -FilePath "$env:USERPROFILE\Desktop\disco.txt"

  El hermano de este guion es `diagnostico-cortes-windows.ps1`, que contesta otra pregunta
  (POR QUE se corta la maquina). No se solapan: aquel no mira disco ni Docker.
#>
param([int]$TopCarpetas = 20)

$ErrorActionPreference = "Continue"
$sep = "=" * 92
function Titulo($t) { Write-Host ""; Write-Host $sep; Write-Host "  $t"; Write-Host $sep }
function GB($bytes) { if ($null -eq $bytes) { return "?" } return ("{0,8:N1} G" -f ($bytes / 1GB)) }

Titulo "1 - LOS VOLUMENES: CUANTO HAY Y CUANTO QUEDA"
try {
  Get-PSDrive -PSProvider FileSystem -ErrorAction Stop |
    Where-Object { $null -ne $_.Used -and ($_.Used + $_.Free) -gt 0 } |
    ForEach-Object {
      $tot = $_.Used + $_.Free
      $pct = 100 * $_.Free / $tot
      $marca = if ($pct -lt 15) { "  <-- BAJO EL UMBRAL DE LA ALERTA (15%)" } else { "" }
      Write-Host ("  {0,-4} total {1}  usado {2}  libre {3}  ({4,5:N1}% libre){5}" -f `
        $_.Name, (GB $tot), (GB $_.Used), (GB $_.Free), $pct, $marca)
    }
} catch { Write-Host "  no pude enumerar volumenes: $_" -ForegroundColor Yellow }

Titulo "2 - LAS $TopCarpetas CARPETAS MAS GRANDES DE C:\  (puede tardar unos minutos)"
Write-Host "  Se recorre UN nivel bajo C:\ y otro bajo las tres mas grandes. Lo que no se puede leer"
Write-Host "  se informa como 'sin permiso' y NO se cuenta: una carpeta inaccesible que aparezca con"
Write-Host "  0 G no es una carpeta vacia."
Write-Host ""
function TamanoDe($ruta) {
  try {
    $items = @(Get-ChildItem -LiteralPath $ruta -Recurse -File -Force -ErrorAction SilentlyContinue)
    if ($items.Count -eq 0) { return 0 }
    return ($items | Measure-Object -Property Length -Sum).Sum
  } catch { return $null }
}
$nivel1 = @()
try {
  foreach ($d in @(Get-ChildItem -LiteralPath 'C:\' -Directory -Force -ErrorAction SilentlyContinue)) {
    $t = TamanoDe $d.FullName
    $nivel1 += [pscustomobject]@{ Ruta = $d.FullName; Bytes = $t }
  }
} catch { Write-Host "  no pude recorrer C: : $_" -ForegroundColor Yellow }
$ordenadas = @($nivel1 | Where-Object { $null -ne $_.Bytes } | Sort-Object Bytes -Descending)
foreach ($x in @($ordenadas | Select-Object -First $TopCarpetas)) {
  Write-Host ("  {0}  {1}" -f (GB $x.Bytes), $x.Ruta)
}
$sinPermiso = @($nivel1 | Where-Object { $null -eq $_.Bytes })
if ($sinPermiso.Count -gt 0) {
  Write-Host ""
  Write-Host ("  sin permiso para medir ({0}): {1}" -f $sinPermiso.Count, (($sinPermiso | ForEach-Object { $_.Ruta }) -join ", ")) -ForegroundColor Yellow
}

Titulo "3 - EL .VHDX: LO QUE OCUPA EN DISCO CONTRA LO QUE TIENE USADO ADENTRO"
Write-Host "  ESTE ES EL PAR DE NUMEROS QUE IMPORTA. Si 'ocupa' es MUCHO mayor que 'usado adentro',"
Write-Host "  el espacio NO vuelve podando: el .vhdx crece y no encoge solo. Hace falta COMPACTAR,"
Write-Host "  que es otra operacion y va con Docker y WSL PARADOS."
Write-Host ""
$vhdx = @()
foreach ($raiz in @("$env:LOCALAPPDATA\Docker", "$env:LOCALAPPDATA\Packages", "$env:LOCALAPPDATA\wsl", "C:\ProgramData\DockerDesktop")) {
  if (Test-Path -LiteralPath $raiz) {
    try { $vhdx += @(Get-ChildItem -LiteralPath $raiz -Recurse -Filter *.vhdx -Force -ErrorAction SilentlyContinue) } catch {}
  }
}
if ($vhdx.Count -eq 0) {
  Write-Host "  no se encontro ningun .vhdx en las rutas conocidas."
  Write-Host "  Eso puede significar que Docker no usa WSL2 aca, o que esta en otra ruta: mirar"
  Write-Host "  Docker Desktop > Settings > Resources > Disk image location."
} else {
  foreach ($f in @($vhdx | Sort-Object Length -Descending)) {
    Write-Host ("  ocupa {0}   {1}" -f (GB $f.Length), $f.FullName)
  }
}
Write-Host ""
Write-Host "  -- usado ADENTRO, segun el propio Docker --"
$docker = Get-Command docker -ErrorAction SilentlyContinue
if ($null -eq $docker) {
  Write-Host "  el comando docker no esta en el PATH: o no esta instalado, o Docker Desktop esta parado." -ForegroundColor Yellow
} else {
  try { docker system df 2>&1 | ForEach-Object { Write-Host "  $_" } }
  catch { Write-Host "  docker system df fallo: $_" -ForegroundColor Yellow }
  Write-Host ""
  Write-Host "  -- el desglose, que dice QUE es reclamable --"
  try { docker system df -v 2>&1 | Select-Object -First 60 | ForEach-Object { Write-Host "  $_" } }
  catch { Write-Host "  docker system df -v fallo: $_" -ForegroundColor Yellow }
}

Titulo "4 - LOS DOS CONTENEDORES QUE LA FLOTA REPORTA CAIDOS"
Write-Host "  Medido desde el cerebro el 2026-09-23:"
Write-Host "    agora-searx                      arriba en 1 de 279 muestras en 14 dias (0,4 %)"
Write-Host "    supabase_edge_runtime_altura-erp arriba 29 % del tiempo; ultima vez el 09-21"
Write-Host "  La pregunta no es 'estan caidos' —eso ya se sabe— sino POR QUE no vuelven solos."
Write-Host ""
if ($null -ne $docker) {
  foreach ($n in @("agora-searx", "supabase_edge_runtime_altura-erp")) {
    Write-Host "  -- $n --"
    try {
      $insp = docker inspect $n --format '{{.State.Status}}|{{.State.ExitCode}}|{{.State.Error}}|{{.HostConfig.RestartPolicy.Name}}|{{.State.FinishedAt}}|{{.RestartCount}}' 2>&1
      if ($LASTEXITCODE -ne 0) {
        Write-Host "     no existe como contenedor (docker inspect fallo). Puede haber sido borrado," -ForegroundColor Yellow
        Write-Host "     y entonces lo que sigue declarado en la flota es una declaracion huerfana." -ForegroundColor Yellow
      } else {
        $p = "$insp".Split("|")
        Write-Host ("     estado={0}  exit={1}  restart-policy={2}  reinicios={3}" -f $p[0], $p[1], $p[3], $p[5])
        Write-Host ("     termino en: {0}" -f $p[4])
        if ($p[2]) { Write-Host ("     error: {0}" -f $p[2]) -ForegroundColor Yellow }
        if ($p[3] -eq "no") {
          Write-Host "     OJO: restart-policy=no. Con esta maquina reiniciando seguido (16 de 16 cortes" -ForegroundColor Yellow
          Write-Host "     fueron reboot), un contenedor sin politica de reinicio NO vuelve nunca solo." -ForegroundColor Yellow
        }
        Write-Host "     -- ultimas 15 lineas del log --"
        docker logs --tail 15 $n 2>&1 | ForEach-Object { Write-Host "       $_" }
      }
    } catch { Write-Host "     fallo consultando $n : $_" -ForegroundColor Yellow }
    Write-Host ""
  }
}

Titulo "5 - LOS SOSPECHOSOS DE SIEMPRE, QUE SE MIDEN PORQUE SON BARATOS"
foreach ($par in @(
    @("C:\hiberfil.sys", "hibernacion — se apaga con 'powercfg /h off' y libera su tamano entero"),
    @("C:\pagefile.sys", "archivo de paginacion — NO conviene tocarlo a ciegas"),
    @("C:\swapfile.sys", "swap de apps modernas"),
    @("C:\Windows\SoftwareDistribution\Download", "cache de Windows Update — se puede vaciar"),
    @("C:\Windows\Temp", "temporales del sistema"),
    @("$env:TEMP", "temporales del usuario"),
    @("$env:LOCALAPPDATA\Temp", "temporales locales"))) {
  $ruta = $par[0]; $que = $par[1]
  try {
    if (Test-Path -LiteralPath $ruta) {
      $item = Get-Item -LiteralPath $ruta -Force -ErrorAction Stop
      $b = if ($item.PSIsContainer) { TamanoDe $ruta } else { $item.Length }
      Write-Host ("  {0}  {1,-48} {2}" -f (GB $b), $ruta, $que)
    } else {
      Write-Host ("  {0,10}  {1,-48} (no existe)" -f "-", $ruta)
    }
  } catch { Write-Host ("  {0,10}  {1,-48} sin permiso" -f "?", $ruta) -ForegroundColor Yellow }
}

Titulo "QUE HACER CON ESTO"
Write-Host @'
  El orden importa, y la primera pregunta decide todo lo demas:

  1. MIRA LA SECCION 3. Si el .vhdx ocupa mucho mas de lo que Docker dice tener usado adentro,
     el disco NO se recupera podando. Podar y despues COMPACTAR son dos pasos, y saltearse el
     segundo es exactamente como este repo perdio 10 G una vez: 12,1 G podados, 2,24 G recuperados.

  2. Si la seccion 2 pone arriba una carpeta que no esperabas, ese es el hallazgo y lo de Docker
     era ruido. Mirala antes de tocar nada.

  3. En la seccion 4, si `restart-policy` dice `no`, ese contenedor no es un problema de Docker:
     es que nadie le pidio que volviera. Con esta maquina reiniciando cada pocas horas, eso
     explica por si solo la mitad de las alertas de servicio.

  NO CORRAS NADA DE ESTO TODAVIA. Este guion mide; lo que se borra o se compacta se decide con
  los numeros a la vista, no antes.
'@
