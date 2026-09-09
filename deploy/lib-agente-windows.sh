# lib-agente-windows.sh — los bloques de PowerShell que TIENEN que ser los mismos en los dos
# guiones que tocan el agente de Windows (`actualizar-agente-windows.sh` y
# `matar-zombis-agente.sh`). No se ejecuta solo: se hace `source`.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# POR QUÉ UN ARCHIVO COMPARTIDO Y NO UNA COPIA EN CADA GUION
#
# El defecto que este repo persigue todo el tiempo es «la misma cautela escrita en un archivo y
# ausente —o distinta— en su hermano de al lado». Ya pasó tres veces con estos mismos guiones:
# `cambiar-agente.cmd` nombraba la carpeta de la app de escritorio para no matarla y el
# actualizador no sabía que existía (A98); el actualizador refrescaba el cambiador y no el
# lanzador (A102); y el 2026-09-05, la clasificación de procesos del actualizador miraba UNA ruta
# donde el cambiador miraba DOS. Duplicar estos bloques es programar la próxima divergencia.
#
# Uso:  source "$(dirname "${BASH_SOURCE[0]}")/lib-agente-windows.sh"

# ── QUÉ CARPETA ES LA DEL AGENTE, CUANDO HAY MÁS DE UN musubi.exe CORRIENDO ──────────────────
#
# La primera versión decía `Get-Process musubi | Select-Object -First 1`. En `davantis-1` eso
# eligió la carpeta EQUIVOCADA y el 2026-09-05 la actualización entera fue a parar a la app de
# escritorio (`AppData\Local\Programs\musubi`) en vez de al agente de flota
# (`AppData\Local\Musubi`). El agente quedó intacto —seguía reportando la versión vieja— y lo
# único que evitó romper la app de escritorio fue una casualidad: su carpeta no tiene
# `device.token`, así que el paso [4] del cambiador falló y su rollback la devolvió.
#
# Lo peor es que este repo YA SABÍA que había dos: `cambiar-agente.cmd` nombra la carpeta de la
# app de escritorio explícitamente, para no matarla con un `taskkill /IM`. La cautela estaba
# escrita en un archivo y ausente en su hermano de al lado.
#
# SE ENUMERA POR LA HOJA DE LA RUTA Y NO POR `Get-Process musubi`, Y ESTO TIENE MEDICIÓN.
#
# musubi-00 corrió los cuatro métodos contra `davantis-1` el 2026-09-05:
#
#     Get-Process musubi                 -> 14   (3 agentes + 11 de la app de escritorio)
#     Get-Process 'musubi.exe'           ->  0
#     Win32_Process Name='musubi.exe'    -> 14
#     Win32_Process por ExecutablePath   ->  3   <- sólo los agentes
#
# O sea que `Get-Process musubi` SÍ devuelve el caso normal — la corrección importa, porque el
# error opuesto sería creer que no sirve para nada. Lo que NO devuelve es el proceso que quedó
# corriendo desde `musubi.exe.viejo`: `Get-Process` matchea `ProcessName`, que es el nombre del
# archivo sin su ÚLTIMA extensión, así que ese proceso se llama `musubi.exe` y no `musubi`. Y
# `Win32_Process` con `Name='musubi.exe'` tampoco lo ve, porque su `Name` es `musubi.exe.viejo`:
# LOS DOS MÉTODOS QUE UNO ELIGE PRIMERO pierden exactamente el caso central. El único robusto es
# filtrar por la RUTA del ejecutable.
#
# Y el caso importa acá arriba, no sólo en la clasificación: si los únicos procesos del agente son
# zombis de `.viejo` —el cambiador renombró, instaló, y `schtasks /run` falló—, un resolver que
# mire `Get-Process musubi` no encuentra la carpeta, y `matar-zombis-agente.sh` queda ciego JUSTO
# en el estado para el que existe.
#
# EL DISCRIMINANTE ES `device.token`, no el orden de los procesos: la carpeta del agente es la
# única que tiene la credencial del dispositivo — es el mismo archivo que el paso [4] del
# cambiador usa para probar el binario nuevo. Y ante 0 o ante 2, SE PARA: elegir a ciegas es
# exactamente lo que produjo este defecto.
RESOLVER='$ErrorActionPreference = "SilentlyContinue"
$cands = @(Get-Process | Where-Object { $_.Path } | Where-Object { (Split-Path $_.Path -Leaf) -eq "musubi.exe" -or (Split-Path $_.Path -Leaf) -eq "musubi.exe.viejo" } | ForEach-Object { Split-Path $_.Path } | Sort-Object -Unique)
if ($cands.Count -eq 0) { "no hay ningun proceso corriendo desde un musubi.exe (ni desde un musubi.exe.viejo): sin el no se sabe donde esta instalado el agente"; exit 1 }
$conToken = @($cands | Where-Object { Test-Path (Join-Path $_ "device.token") })
if ($conToken.Count -eq 0) { "hay " + $cands.Count + " carpeta(s) con musubi.exe corriendo y NINGUNA tiene device.token, asi que ninguna es el agente de flota: " + ($cands -join ", "); exit 1 }
if ($conToken.Count -gt 1) { "hay " + $conToken.Count + " instalaciones con device.token y no se cual es el agente: " + ($conToken -join ", ") + ". No elijo a ciegas"; exit 1 }
$d = $conToken[0]
"carpeta del agente: " + $d
'

# ── QUIÉN ESTÁ CORRIENDO EL BINARIO NUEVO Y QUIÉN QUEDÓ DE ANTES ─────────────────────────────
#
# Deja puestas cuatro variables: $exe, $escrito (LastWriteTime del binario), $nuevos (arrancados
# DESPUÉS de que ese archivo se escribiera) y $zombis (arrancados antes). Necesita $d.
#
# SE ENUMERA POR RUTA Y NO POR NOMBRE, y las dos rutas, y esto se pagó midiendo:
#
#  1. `Get-Process musubi` NO encuentra al zombi que corre desde `musubi.exe.viejo`. El nombre de
#     proceso de Windows es el del archivo sin su última extensión, así que ese proceso se llama
#     `musubi.exe` y no `musubi`. La primera confirmación usaba `Get-Process musubi` y por
#     construcción no podía ver a la mitad de los zombis que decía custodiar.
#  2. El paso [2] del cambiador RENOMBRA el binario en uso, así que un agente anterior sigue vivo
#     con su imagen ahora llamada `.viejo`. `cambiar-agente.cmd` mata las DOS rutas —lo tiene
#     escrito y medido desde el 2026-09-02—; la confirmación del actualizador miraba una sola.
#  3. `-eq $exe` y no `-like`: la app de escritorio tiene su propio `musubi.exe` en otra carpeta y
#     no se toca. Es la misma razón por la que el cambiador no usa `taskkill /IM`.
#
# Y LA HORA SE IMPRIME EN ISO. El 2026-09-05 la salida decía «09/05/2026 01:14:38» y no había
# forma de saber si era el 9 de mayo o el 5 de septiembre: el formato sale de la config regional
# de la máquina remota. Un dato que no se puede leer sin adivinar no sirve para diagnosticar.
CLASIFICAR='$exe = Join-Path $d "musubi.exe"
$viejo = $exe + ".viejo"
if (-not (Test-Path $exe)) { "no hay musubi.exe en " + $d; exit 1 }
$escrito = (Get-Item $exe).LastWriteTime
$todos  = @(Get-Process -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $exe -or $_.Path -eq $viejo })
$nuevos = @($todos | Where-Object { $_.StartTime -gt $escrito })
$zombis = @($todos | Where-Object { $_.StartTime -lt $escrito })
$cuando = $escrito.ToString("yyyy-MM-dd HH:mm:ss")
$detalle = (($todos | Sort-Object StartTime | ForEach-Object { "pid " + $_.Id + " arrancado " + $_.StartTime.ToString("yyyy-MM-dd HH:mm:ss") + " desde " + $_.Path }) -join " | ")
$tarea = (schtasks /query /tn "Musubi Agente de Flota" /fo list 2>&1 | Out-String)
$estado = (($tarea -split "`r?`n" | Where-Object { $_ -match "Status|Estado" }) -join " ").Trim()
'
