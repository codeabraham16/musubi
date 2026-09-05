#!/usr/bin/env bash
#
# actualizar-agente-windows.sh — actualiza el agente de una máquina Windows de la flota.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# EL SHA NO SE ESCRIBE A MANO: SE DERIVA DE LA COMPILACIÓN
#
# La primera versión de este guion tenía el sha256 y la versión escritos como constantes, y
# quedaron viejos TRES VECES en una tarde — la rama se mueve, el binario preparado deja de ser el
# de la rama, y nadie avisa. Un valor copiado de una compilación anterior se ve idéntico a uno
# correcto: es la misma familia que este repo persigue.
#
# Así que el guion COMPILA y toma el sha de lo que acaba de compilar. No puede quedar viejo.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# SE COMPILA EN UN WORKTREE LIMPIO, NO EN EL ÁRBOL DE TRABAJO
#
# `construir.sh` le pone el sufijo `-sucio` a un binario armado con cambios sin commitear, y con
# razón: ese binario no se puede reconstruir después, porque el commit que anuncia NO es el código
# que corre. Un agente de producción no puede ser `-sucio`. El worktree en un commit da un árbol
# limpio por construcción sin tocar lo que estés editando.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# EL SERVIDOR HTTP SALE DE ACÁ Y NO DE musubi-server
#
# Esta máquina está en el tailnet, así que no hace falta abrir un listener sin autenticación en
# el host del cerebro. Se sirve UN directorio con DOS archivos, bindeado a la IP del tailnet y no
# a 0.0.0.0, y se apaga con `trap` incluso si el guion muere a la mitad.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# LA CARPETA SALE DEL BINARIO EN USO, NO DE %LOCALAPPDATA% (A71, y lo pagué midiendo)
#
# La primera versión de este guion usaba `$env:LOCALAPPDATA\Musubi`. NO FUNCIONA, y falla de la
# peor manera: el agente corre como SYSTEM, así que ahí `LOCALAPPDATA` es
# `C:\WINDOWS\system32\config\systemprofile\AppData\Local` —no la instalación—, y esa carpeta
# NO EXISTE. Medido en `gio` el 2026-09-04: la instalación real es
# `C:\Users\meirn\AppData\Local\Musubi`, y el `curl` a la ruta de SYSTEM muere con
# `(23) client returned ERROR on write` DESPUÉS de haber bajado los bytes — o sea que el error no
# dice «carpeta equivocada», dice algo sobre escritura, y manda a buscar permisos.
#
# `cambiar-agente.cmd` ya usa `%~dp0` por esta misma razón y lo tiene documentado; este guion lo
# ignoraba. La carpeta se deriva del PATH DEL PROCESO que está corriendo.
#
# CORRECCIÓN DEL 2026-09-05: «el path del proceso» NO alcanza, y afirmarlo acá como invariante fue
# lo que produjo el incidente. En `davantis-1` corren DOCE procesos `musubi.exe` en DOS carpetas
# —el agente de flota y la app de escritorio, que levanta sus propios `daemon` y `cerebro`— y
# `Select-Object -First 1` eligió la equivocada: la actualización entera fue a parar a la app.
# Ahora la carpeta se elige por `device.token`, y ante 0 o ante 2 el guion SE PARA. Ver el RESOLVER
# más abajo.
#
# Uso:  MUSUBI_TOKEN_FILE=/ruta/al/token ./actualizar-agente-windows.sh gio [commit]
set -euo pipefail

DEVICE="${1:?falta el nombre de la máquina, ej: gio}"
REF="${2:-HEAD}"
REPO="${MUSUBI_REPO:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
TOOL="${MUSUBI_TOOL:-$REPO/deploy/musubi-tool.sh}"
PUERTO="${PUERTO:-8899}"

rojo(){ printf '\033[31m✗ %s\033[0m\n' "$*" >&2; }
ok(){   printf '\033[32m✓ %s\033[0m\n' "$*"; }
paso(){ printf '\n\033[36m▶ %s\033[0m\n' "$*"; }

# ESTE GUION CORRE EN LA MÁQUINA DE DESARROLLO, NO EN musubi-server, y el error de correrlo en
# el lugar equivocado no se parecía a eso: era un `No such file or directory` del shell, que manda
# a mirar la ruta. Acá se dice.
[[ -d "$REPO/.git" ]] || {
  rojo "$REPO no es el repo de Musubi."
  echo "  Este guion COMPILA el agente y lo SIRVE por el tailnet, así que corre en la máquina que" >&2
  echo "  tiene el repo y el compilador de Go —la de desarrollo—, NO en musubi-server." >&2
  exit 1
}
command -v go >/dev/null || { rojo "no hay Go acá: este guion compila el agente. Corrélo en la máquina de desarrollo"; exit 1; }
[[ -x "$TOOL" ]] || { rojo "no encuentro musubi-tool.sh en $TOOL"; exit 1; }
[[ -n "${MUSUBI_TOKEN:-}${MUSUBI_TOKEN_FILE:-}" ]] || {
  rojo "falta la credencial: musubi-tool.sh necesita MUSUBI_TOKEN o MUSUBI_TOKEN_FILE"
  echo "  En la máquina de desarrollo suele venir del perfil (~/.bashrc), así que si esto salta," >&2
  echo "  o estás en otra máquina o el shell no cargó el perfil." >&2
  exit 1
}
command -v tailscale >/dev/null || { rojo "sin tailscale no sé por qué IP servir"; exit 1; }
IP="$(tailscale ip -4 | head -1)"
[[ -n "$IP" ]] || { rojo "tailscale no devolvió una IP v4"; exit 1; }

# Si el puerto ya está tomado, no se pisa: podría ser otra actualización en curso, y dos
# servidores sirviendo binarios distintos en el mismo puerto es peor que ninguno.
if (exec 3<>/dev/tcp/"$IP"/"$PUERTO") 2>/dev/null; then
  rojo "algo ya escucha en $IP:$PUERTO — no lo piso. Cerralo o pasá otro PUERTO="
  exit 1
fi

TMP="$(mktemp -d)"; WT="$TMP/wt"; SRV="$TMP/servir"
SRVPID=""
limpiar(){
  [[ -n "$SRVPID" ]] && kill "$SRVPID" 2>/dev/null || true
  git -C "$REPO" worktree remove --force "$WT" 2>/dev/null || true
  git -C "$REPO" worktree prune 2>/dev/null || true
  rm -rf "$TMP"
}
trap limpiar EXIT

paso "1/6 · compilando el agente de Windows desde un árbol limpio en $REF"
git -C "$REPO" worktree add --detach "$WT" "$REF" >/dev/null 2>&1 || {
  rojo "no pude crear el worktree en $REF"; exit 1; }
COMMIT="$(git -C "$WT" rev-parse --short HEAD)"
mkdir -p "$SRV"
( cd "$WT" && GOOS=windows GOARCH=amd64 ./deploy/construir.sh flota "$SRV/musubi.exe" ) >"$TMP/build.log" 2>&1 || {
  rojo "la compilación falló:"; tail -20 "$TMP/build.log" >&2; exit 1; }
VERSION="$(grep -oE '0\.[0-9]+\.[0-9]+-[A-Za-z0-9.-]+' "$TMP/build.log" | head -1)"
SHA="$(sha256sum "$SRV/musubi.exe" | cut -d' ' -f1)"
case "$VERSION" in
  *-sucio) rojo "el binario salió como «$VERSION»: el árbol del worktree no estaba limpio. No se despliega un binario que no se puede reconstruir."; exit 1 ;;
  "")      rojo "no pude leer la versión de la compilación"; tail -10 "$TMP/build.log" >&2; exit 1 ;;
esac
cp "$REPO/deploy/cambiar-agente.cmd" "$SRV/"
ok "$VERSION · sha256 $SHA · $(stat -c %s "$SRV/musubi.exe") bytes · commit $COMMIT"

paso "2/6 · sirviendo esos DOS archivos en $IP:$PUERTO (sólo mientras corre este guion)"
( cd "$SRV" && exec python3 -m http.server "$PUERTO" --bind "$IP" >/dev/null 2>&1 ) &
SRVPID=$!
for _ in 1 2 3 4 5 6 7 8 9 10; do
  curl -fsI --max-time 2 "http://$IP:$PUERTO/musubi.exe" >/dev/null 2>&1 && break
  sleep 0.5
done
curl -fsI --max-time 3 "http://$IP:$PUERTO/musubi.exe" >/dev/null || { rojo "el servidor no levantó"; exit 1; }
ok "sirviendo"

# CADA PASO SE ESPERA, NO SE SUPONE — Y ES EL DEFECTO QUE ESTE GUION YA COMETIÓ.
#
# `musubi_fleet_exec` BLOQUEA hasta 45 s (`esperaMaxExec`) y, si el timeout del comando llega o
# supera ese techo, lo ENCOLA y vuelve en el acto con `estado: pendiente`. Eso es correcto de su
# parte —esperar más daría un timeout de HTTP, que se lee como «el cerebro no anda»— pero para
# este guion significa que seguía a la línea siguiente MIENTRAS el comando todavía no había
# corrido, y al terminar bajaba el servidor HTTP con el `trap`.
#
# Medido el 2026-09-04: el agente levantó la descarga 6 s después, con el servidor YA APAGADO, y
# `Invoke-WebRequest` murió con «Unable to connect». El guion no se enteró de nada: había salido
# en verde. Un despliegue que reporta éxito sobre un paso que ni siquiera corrió es peor que uno
# que falla.
#
# Así que se encola con un timeout generoso —para que el comando no lo mate a él— y después se
# ESPERA su resultado en la bitácora, que es la única fuente que sabe si terminó.
esperar_comando(){ # $1 = command_id · $2 = segundos de paciencia
  local cid="$1" tope="$2" i=0
  while (( i < tope )); do
    local r
    r="$("$TOOL" musubi_fleet_log "{\"device\":\"$DEVICE\",\"limite\":20,\"project\":\"musubi\"}" 2>/dev/null \
      | python3 -c '
import json,sys
try: d=json.load(sys.stdin)
except Exception: sys.exit()
for c in d.get("comandos", []):
    if c.get("command_id") == sys.argv[1]:
        # stderr Y error VAN, no sólo stdout. Sin ellos, un PowerShell que rompe por sintaxis, un
        # Start-Process que no encuentra el .exe, o el timeout del ejecutor —que escribe `error` y
        # deja `exit_code` en None— salían como un «exit=1» MUDO, con la línea de salida vacía.
        # El JSON que este guion ya descarga los trae (methods_exec.go:481-486); los tiraba acá.
        malo = " ".join(x for x in [(c.get("stderr") or "").strip(), (c.get("error") or "").strip()] if x)
        print("%s\t%s\t%s\t%s" % (c.get("estado",""), c.get("exit_code"),
              (c.get("stdout") or "").strip().replace("\n"," ⏎ ")[:500],
              malo.replace("\n"," ⏎ ")[:500]))
        break' "$cid")"
    if [[ -n "$r" ]]; then
      local est ec out malo
      IFS=$'\t' read -r est ec out malo <<< "$r"
      if [[ "$est" == "terminado" ]]; then
        [[ -n "$out" ]] && echo "   $out"
        if [[ "$ec" != "0" ]]; then
          [[ -n "$malo" ]] && printf '   \033[31m%s\033[0m\n' "$malo" >&2
          rojo "el paso terminó con exit=$ec — no se sigue"
          return 1
        fi
        return 0
      fi
    fi
    sleep 3; i=$((i+3))
  done
  rojo "el comando $cid no terminó en ${tope}s. NO se sigue: los pasos que vienen suponen que éste salió bien"
  return 1
}

llamar(){ # $1 argv como JSON · $2 timeout del comando
  local j r cid est
  j="$(python3 -c '
import json,sys
print(json.dumps({"device": sys.argv[1], "argv": json.loads(sys.argv[2]),
                  "timeout_seg": int(sys.argv[3]), "project": "musubi"}))' "$DEVICE" "$1" "$2")"
  r="$("$TOOL" musubi_fleet_exec "$j")" || { rojo "el cerebro rechazó el pedido"; echo "$r" >&2; return 1; }
  read -r cid est < <(printf '%s' "$r" | python3 -c '
import json,sys
d=json.load(sys.stdin); print(d.get("command_id",""), d.get("estado",""))')
  [[ -n "$cid" ]] || { rojo "el cerebro no devolvió un command_id"; echo "$r" >&2; return 1; }
  echo "   encolado $cid ($est)"
  # LA PACIENCIA SALE DEL PRESUPUESTO QUE SE PIDIÓ, no de un 180 fijo. Con el 300 del paso 3, el
  # guion se rendía a los 180 s CON EL COMANDO TODAVÍA CORRIENDO, y el `trap limpiar` bajaba el
  # servidor HTTP debajo de un `fetch` a medio bajar. Es el mismo modo de falla que las líneas de
  # arriba dicen haber arreglado, entrando por la puerta de al lado.
  esperar_comando "$cid" $(( $2 + 60 ))
}
ps1(){ python3 -c 'import json,sys; print(json.dumps(["powershell","-NoProfile","-Command",sys.argv[1]]))' "$1"; }

# `RESOLVER` (qué carpeta es la del agente) y `CLASIFICAR` (quién corre el binario nuevo y quién
# quedó de antes) viven en el lib porque `matar-zombis-agente.sh` necesita EXACTAMENTE los mismos.
# Tenerlos duplicados es programar la próxima divergencia, y ya nos pasó tres veces con estos
# mismos guiones — el lib lo cuenta en su cabecera.
# shellcheck source=deploy/lib-agente-windows.sh
source "$REPO/deploy/lib-agente-windows.sh"


# ────────────────────────────────────────────────────────────────────────────────────────────
# LA DESCARGA LA HACE `musubi.exe fetch`, NO PowerShell — Y ES UN SOLO CAMINO A PROPÓSITO
#
# En `davantis-1` la lista blanca de NordVPN autoriza POR ARCHIVO (A31): el único ejecutable que
# alcanza el tailnet es `musubi.exe` en su ruta exacta. `Invoke-WebRequest` corre desde
# `powershell.exe` y muere con «Unable to connect» MIENTRAS el agente de esa misma máquina le late
# al cerebro sin problema — medido el 2026-09-05.
#
# `musubi fetch` resuelve eso porque la descarga la hace el proceso que SÍ está autorizado. Su
# allowlist acepta loopback o `100.64.0.0/10` y revalida en cada redirect (cmd/musubi/fetch.go),
# así que no afloja nada: sigue sin poder salir del tailnet.
#
# SE INVOCA CON `Start-Process -RedirectStandardOutput` Y NO CON `cmd /c ... > archivo`. La
# primera versión usaba `cmd`, y PowerShell la rechazó con «The string is missing the
# terminator» — para pasarle a `cmd` una ruta entre comillas DENTRO de una cadena de PowerShell
# hacen falta cuatro comillas seguidas, y ahí el parser no sabe cuál cierra. `Start-Process`
# toma la ruta y los argumentos como valores, no como texto a re-parsear, así que no hay
# anidamiento que equivocar. Y de paso da el ExitCode del proceso, que `cmd /c` se tragaba.
#
# SE USA EN LAS DOS MÁQUINAS Y NO SÓLO EN LA QUE LO NECESITA. Tener dos caminos de descarga
# —`Invoke-WebRequest` acá, `fetch` allá— es exactamente la forma que este repo persigue: el que se
# usa menos es el que se rompe sin que nadie se entere. La idea es de la sesión musubi-aa.
paso "3/6 · bajando el binario a $DEVICE con su propio fetch, y verificando el sha EN DESTINO"
llamar "$(ps1 "$RESOLVER"'$n = Join-Path $d "musubi-nuevo.exe"
$r = Start-Process -FilePath (Join-Path $d "musubi.exe") -ArgumentList "fetch","http://'"$IP:$PUERTO"'/musubi.exe" -RedirectStandardOutput $n -NoNewWindow -Wait -PassThru
if ($r.ExitCode -ne 0) { Remove-Item $n -Force -EA SilentlyContinue; "fetch salio con " + $r.ExitCode; exit 1 }
if (-not (Test-Path $n)) { "fetch no dejo archivo"; exit 1 }
$h=(Get-FileHash $n -Algorithm SHA256).Hash.ToLower()
if ($h -ne "'"$SHA"'") { Remove-Item $n -Force; "SHA DISTINTO: $h -- BORRADO"; exit 1 }
"sha ok, bytes: " + (Get-Item $n).Length')" 300

paso "4/6 · refrescando cambiar-agente.cmd (el de la máquina puede ser viejo)"
# Mismo motivo que el paso 3: `curl.exe` tampoco está en la lista blanca de `davantis-1`.
llamar "$(ps1 "$RESOLVER"'$c = Join-Path $d "cambiar-agente.cmd.nuevo"
$r = Start-Process -FilePath (Join-Path $d "musubi.exe") -ArgumentList "fetch","http://'"$IP:$PUERTO"'/cambiar-agente.cmd" -RedirectStandardOutput $c -NoNewWindow -Wait -PassThru
if ($r.ExitCode -ne 0 -or -not (Test-Path $c) -or (Get-Item $c).Length -lt 100) { Remove-Item $c -Force -EA SilentlyContinue; "el cambiador no bajo entero"; exit 1 }
Move-Item -Force $c (Join-Path $d "cambiar-agente.cmd")
"cambiar-agente.cmd: " + (Get-Item (Join-Path $d "cambiar-agente.cmd")).Length + " bytes"')" 120

paso "5/6 · migrando el lanzador si trae la forma vieja del token (A102)"
# EL LANZADOR TAMBIÉN SE DESPLIEGA, Y NADIE LO ACTUALIZABA.
#
# `agente.cmd` lo escribe SÓLO el instalador. Este guion refresca el binario y `cambiar-agente.cmd`
# —con el argumento «el de la máquina puede ser viejo» escrito acá al lado— y nunca tocaba el
# lanzador. Resultado medido en davantis-1 el 2026-09-05: la máquina seguía con
# `set /p MUSUBI_DEVICE_TOKEN=<"...\device.token"`, la forma que `agente-windows.ps1:248` reemplazó
# hace tiempo. El arreglo estaba en el repo y no había llegado a la máquina.
#
# Las tres consecuencias son de hoy y están vivas: la credencial queda EN EL ENTORNO del proceso
# —el mecanismo exacto de A88, donde arreglar el archivo no le llega a un proceso que ya arrancó—;
# la rotación en caliente no puede completarse porque `set /p` corre UNA sola vez al arrancar; y
# `set /p` lee sólo la PRIMERA LÍNEA, así que el formato multi-token que existe para dar fallback a
# una rotación nunca le llega al agente (se cruza con A101).
#
# Se migra EN EL LUGAR y no se regenera: el lanzador lleva valores propios de la máquina
# (`MUSUBI_BRAIN_URL`, `MUSUBI_ALCANCE`, la ruta del token, la ruta del binario) que este guion no
# conoce. Se cambia la línea del token y NADA más, con respaldo al lado.
llamar "$(ps1 "$RESOLVER"'$lan = Join-Path $d "agente.cmd"
if (-not (Test-Path $lan)) { "no hay agente.cmd en " + $d + ": esta maquina no la instalo agente-windows.ps1, no la toco"; exit 1 }
$txt = Get-Content $lan -Raw
if ($txt -match "(?im)^\s*set\s+MUSUBI_DEVICE_TOKEN_FILE\s*=") { "el lanzador ya pasa la RUTA del token: nada que migrar"; exit 0 }
$m = [regex]::Match($txt, "(?im)^\s*set\s*/p\s+MUSUBI_DEVICE_TOKEN\s*=\s*<\s*""?([^""\r\n]+?)""?\s*$")
if (-not $m.Success) { "el lanzador no tiene ni la forma vieja ni la nueva; NO lo toco. Contenido:"; Get-Content $lan; exit 1 }
$ruta = $m.Groups[1].Value.Trim()
Copy-Item -Force $lan ($lan + ".antes-de-la-ruta")
$nuevo = [regex]::Replace($txt, "(?im)^\s*set\s*/p\s+MUSUBI_DEVICE_TOKEN\s*=\s*<.*$", ("set MUSUBI_DEVICE_TOKEN_FILE=" + $ruta))
Set-Content -Encoding ASCII -Path $lan -Value $nuevo
"lanzador migrado a la RUTA del token (" + $ruta + "); respaldo en agente.cmd.antes-de-la-ruta"')" 60

paso "6/6 · lanzando el cambiador DESPEGADO (el paso 2 del .cmd mata al agente que corre esto)"
llamar "$(ps1 "$RESOLVER"'$cmdPath = Join-Path $d "cambiar-agente.cmd"
if (-not (Test-Path $cmdPath)) { "no esta el cambiador en " + $cmdPath; exit 1 }
Start-Process -FilePath "cmd.exe" -ArgumentList "/c",$cmdPath -WindowStyle Hidden
"lanzado desde " + $d')" 30

paso "esperando que $DEVICE vuelva a latir con $VERSION"
for i in $(seq 1 20); do
  sleep 15
  V="$("$TOOL" musubi_fleet_list '{"project":"musubi"}' 2>/dev/null | python3 -c '
import json,sys
try: d=json.load(sys.stdin)
except Exception: sys.exit()
for m in d.get("devices", []):
    if m.get("name")==sys.argv[1]:
        print(m.get("agent_version") or "")' "$DEVICE")"
  printf '   [%3ds] %s reporta: %s\n' "$((i*15))" "$DEVICE" "${V:-(sin respuesta)}"
  if [[ "$V" == "$VERSION" ]]; then
    # EL VERDE NO LO PUEDE DAR LA VERSIÓN SOLA (A98). El paso [4] del cambiador corre
    # `musubi.exe agent --once` para PROBAR el binario nuevo, y ese latido de prueba YA escribe la
    # versión nueva en la fila del cerebro. O sea que si el paso [5] falla —`schtasks /run` no
    # arranca la tarea, por ejemplo porque la instancia anterior todavía se está apagando y está
    # registrada con `-MultipleInstances IgnoreNew`— la máquina queda SIN AGENTE y este guion la
    # declara actualizada igual. El verde lo daría un campo que la prueba también escribe.
    paso "confirmando EN LA MÁQUINA que quedó un agente vivo, no sólo una versión escrita"
    if llamar "$(ps1 "$RESOLVER$CLASIFICAR"'$sha = (Get-FileHash $exe -Algorithm SHA256).Hash.ToLower()
if ($sha -ne "'"$SHA"'") { "el musubi.exe de la carpeta NO es el que se instalo (sha " + $sha + "): el cambiador no llego a ponerlo o algo lo piso"; exit 1 }
if ($todos.Count -eq 0) { "NO hay ningun proceso corriendo desde " + $exe + " ni desde " + $viejo + " -- la version nueva la escribio el latido de PRUEBA del cambiador, no un agente vivo. Tarea: " + $estado; exit 1 }
if ($nuevos.Count -eq 0) { "hay " + $todos.Count + " proceso(s) del agente y TODOS arrancaron ANTES de que el binario se escribiera (" + $cuando + "): corren la imagen VIEJA, que ya no esta en disco, y la version que reporta la fila es la de ellos. Procesos: " + $detalle + " | Tarea: " + $estado; exit 1 }
if ($zombis.Count -gt 0) {
  # UN ZOMBI VIVO HACE FALLAR, NO ADVERTIR. Corre la imagen ANTERIOR —que ya no esta en disco— y
  # sigue latiendo: puede ganar la proxima escritura de la fila y dejar al cerebro reportando la
  # version vieja sobre una maquina que si se actualizo. Paso el 2026-09-05 y confundio el
  # diagnostico durante horas. Ademas significa que el paso [1] del cambiador no logro matarlo.
  # EL BINARIO YA ESTA PUESTO: esto no se arregla volviendo a correr el actualizador.
  "quedaron " + $zombis.Count + " zombi(s) del binario anterior. Binario instalado " + $cuando + ". Procesos: " + $detalle
  exit 1
}
"agente NUEVO vivo: " + $nuevos.Count + " proceso(s), todos arrancados despues de instalar el binario, y sin zombis | " + $estado')" 60; then
      ok "ACTUALIZADA a $VERSION, con agente vivo confirmado en la máquina"
      exit 0
    fi
    # ESTE MENSAJE NO DIAGNOSTICA: EL DE ARRIBA YA LO HIZO, Y MIDIENDO.
    #
    # La versión anterior afirmaba acá «el cambiador probó el binario y la tarea no arrancó» y
    # mandaba a correr `schtasks /run`. Eso es UNA de las cuatro razones por las que la
    # confirmación puede fallar, y el 2026-09-05, con la máquina en otra de las cuatro —quedaban
    # zombis—, la línea de PowerShell dijo «lo que falta es matar esos procesos» y esta línea la
    # tapó con una causa falsa y un remedio que EMPEORA el caso: `schtasks /run` agrega un proceso
    # más y deja al zombi vivo. Un mensaje genérico encima de uno preciso es peor que ninguno.
    rojo "la versión llegó pero la confirmación EN LA MÁQUINA falló. La razón exacta es la línea de arriba"
    echo "    · si dice ZOMBIS         →  ./deploy/matar-zombis-agente.sh $DEVICE" >&2
    echo "    · si dice que NO hay ningún proceso  →  la tarea no arrancó; en la máquina:" >&2
    echo "        schtasks /run /tn \"Musubi Agente de Flota\"" >&2
    echo "    · si dice que el sha NO coincide     →  mirá $DEVICE:<carpeta>\\cambio.log (¿hubo rollback?)" >&2
    exit 1
  fi
done
rojo "no llegó a $VERSION en 5 min. Mirá el log EN LA MÁQUINA:"
echo "    <carpeta-de-instalacion>\\cambio.log   (dice si hizo ROLLBACK y por qué)"
echo "    La carpeta NO es %LOCALAPPDATA%: el agente corre como SYSTEM. Sale del binario en uso;"
echo "    en gio es C:\\Users\\meirn\\AppData\\Local\\Musubi (medido el 2026-09-04)."
echo "  Y ojo con el zombi: un agente viejo vivo desde musubi.exe.viejo gana la carrera del latido."
exit 1
