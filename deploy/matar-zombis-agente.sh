#!/usr/bin/env bash
#
# matar-zombis-agente.sh — mata los procesos del agente que quedaron corriendo la imagen ANTERIOR
# después de una actualización, y confirma que quedó vivo el nuevo.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# QUÉ ES UN ZOMBI ACÁ, Y POR QUÉ NO SE ARREGLA VOLVIENDO A ACTUALIZAR
#
# El paso [1] de `cambiar-agente.cmd` para la tarea y mata los procesos del agente; el paso [2]
# RENOMBRA el binario en uso. Cuando alguno sobrevive a ese paso —el envoltorio oculto lanza al
# agente como hijo, y `schtasks /end` termina la tarea, no necesariamente al hijo— queda un
# proceso vivo corriendo una imagen que ya no está en disco.
#
# Ese proceso NO es inofensivo: sigue latiéndole al cerebro con la versión VIEJA y puede ganar la
# carrera de la escritura contra el agente nuevo. El 2026-09-05 eso dejó al cerebro reportando una
# versión que no estaba en ningún archivo de la máquina, y el diagnóstico se fue horas por ahí.
#
# El binario nuevo YA ESTÁ INSTALADO cuando esto pasa —la confirmación del actualizador verifica
# el sha antes de mirar los procesos—, así que volver a correr el actualizador no arregla nada:
# repite el trabajo y vuelve a chocar con el mismo proceso. Lo que falta es matarlo.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# POR QUÉ MATA DESPEGADO, CON OCHO SEGUNDOS DE RETRASO
#
# El comando lo ejecuta un agente de ESA máquina, y el zombi es un agente: puede ser él mismo el
# que levantó este comando del cerebro. Un `Stop-Process` en línea que se lleva a su propio
# ejecutor no devuelve respuesta, y desde acá «no contestó» se lee igual que «el cerebro se cayó».
# Así que se lanza un PowerShell aparte que espera y después mata, la respuesta vuelve enseguida
# con la lista de lo que va a morir, y la verificación se hace en un comando NUEVO —que ya lo
# atiende el agente que sobrevivió.
#
# Uso:  MUSUBI_TOKEN_FILE=/ruta/al/token ./matar-zombis-agente.sh davantis-1
set -euo pipefail

DEVICE="${1:?falta el nombre de la máquina, ej: davantis-1}"
REPO="${MUSUBI_REPO:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
TOOL="${MUSUBI_TOOL:-$REPO/deploy/musubi-tool.sh}"

rojo(){ printf '\033[31m✗ %s\033[0m\n' "$*" >&2; }
ok(){   printf '\033[32m✓ %s\033[0m\n' "$*"; }
paso(){ printf '\n\033[36m▶ %s\033[0m\n' "$*"; }

[[ -x "$TOOL" ]] || { rojo "no encuentro musubi-tool.sh en $TOOL"; exit 1; }
[[ -n "${MUSUBI_TOKEN:-}${MUSUBI_TOKEN_FILE:-}" ]] || {
  rojo "falta la credencial: musubi-tool.sh necesita MUSUBI_TOKEN o MUSUBI_TOKEN_FILE"; exit 1; }

# shellcheck source=deploy/lib-agente-windows.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib-agente-windows.sh"

# El mismo par `llamar`/`esperar_comando` del actualizador, y por el mismo motivo: `fleet_exec`
# ENCOLA y vuelve en el acto cuando el timeout llega al techo de 45 s, así que seguir a la línea
# siguiente es seguir sobre un comando que todavía no corrió.
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
        malo = " ".join(x for x in [(c.get("stderr") or "").strip(), (c.get("error") or "").strip()] if x)
        print("%s\t%s\t%s\t%s" % (c.get("estado",""), c.get("exit_code"),
              (c.get("stdout") or "").strip().replace("\n"," ⏎ ")[:900],
              malo.replace("\n"," ⏎ ")[:500]))
        break' "$cid")"
    if [[ -n "$r" ]]; then
      local est ec out malo
      IFS=$'\t' read -r est ec out malo <<< "$r"
      if [[ "$est" == "terminado" ]]; then
        [[ -n "$out" ]] && echo "   $out"
        if [[ "$ec" != "0" ]]; then
          [[ -n "$malo" ]] && printf '   \033[31m%s\033[0m\n' "$malo" >&2
          return 1
        fi
        return 0
      fi
    fi
    sleep 3; i=$((i+3))
  done
  rojo "el comando $cid no terminó en ${tope}s"
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
  esperar_comando "$cid" $(( $2 + 60 ))
}

ps1(){ python3 -c 'import json,sys; print(json.dumps(["powershell","-NoProfile","-Command",sys.argv[1]]))' "$1"; }

# El veredicto lo da la máquina y no este guion: acá no hay forma de saber qué corre allá.
VEREDICTO="$RESOLVER$CLASIFICAR"'"binario instalado " + $cuando + " | procesos: " + $detalle
if ($todos.Count -eq 0) { "NO hay ningun proceso corriendo desde " + $exe + " ni desde " + $viejo + ": la maquina esta SIN AGENTE. Tarea: " + $estado; exit 1 }
if ($zombis.Count -gt 0) { "quedan " + $zombis.Count + " zombi(s) arrancado(s) antes que el binario"; exit 1 }
if ($nuevos.Count -eq 0) { "ningun proceso arranco despues del binario"; exit 1 }
"limpio: " + $nuevos.Count + " proceso(s) del binario instalado y ningun zombi | " + $estado'

paso "1/3 · mirando qué corre en $DEVICE"
if llamar "$(ps1 "$VEREDICTO")" 60; then
  ok "no hay zombis en $DEVICE: no hay nada que matar"
  exit 0
fi

paso "2/3 · matando SÓLO los arrancados antes del binario instalado"
# LA GUARDA QUE IMPORTA: si no hay ningún proceso del binario nuevo, matar a los viejos deja la
# máquina SIN AGENTE y sin canal para arreglarla. En ese caso no se mata: se arranca la tarea.
llamar "$(ps1 "$RESOLVER$CLASIFICAR"'if ($zombis.Count -eq 0) { "no hay zombis"; exit 0 }
if ($nuevos.Count -eq 0) { "hay " + $zombis.Count + " proceso(s) viejo(s) y NINGUNO del binario nuevo: matarlos deja la maquina SIN AGENTE y sin canal. Arranca la tarea primero (schtasks /run /tn \"Musubi Agente de Flota\"). Procesos: " + $detalle; exit 1 }
$ids = (($zombis | ForEach-Object { $_.Id }) -join ",")
$orden = "Start-Sleep -Seconds 8; Stop-Process -Id " + $ids + " -Force -ErrorAction SilentlyContinue"
Start-Process -FilePath "powershell" -ArgumentList "-NoProfile","-Command",$orden -WindowStyle Hidden
"mueren en 8s los pid " + $ids + "; sobreviven " + $nuevos.Count + " del binario instalado"')" 60 || {
  rojo "no se mató nada — la razón está arriba"
  exit 1
}

paso "3/3 · confirmando que quedó vivo el agente nuevo y ningún zombi"
sleep 20
if llamar "$(ps1 "$VEREDICTO")" 60; then
  ok "$DEVICE limpia: corre el binario instalado y no quedan zombis"
  exit 0
fi
rojo "todavía no está limpia. Si siguen los mismos pid, el agente que ejecuta esto es uno de ellos"
echo "    y no se pudo matar a sí mismo: entrá a la máquina y corré" >&2
echo "    Stop-Process -Id <pid> -Force  ·  luego  schtasks /run /tn \"Musubi Agente de Flota\"" >&2
exit 1
