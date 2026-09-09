#!/usr/bin/env bash
# verificar-cobertura.sh — QUÉ VIGILA REALMENTE CADA MÁQUINA, y qué no.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# LA PREGUNTA QUE NINGÚN TABLERO CONTESTA
#
# `verificar-despliegue.sh` (A73) contesta «¿está la regla cargada?». Esta contesta la de un nivel
# más adentro, que es la que decide si la regla sirve: **¿esa regla vigila a ESTA máquina?**
#
# Las dos son distintas y la diferencia se midió el 2026-09-02, con las 35 reglas desplegadas y
# todas sus métricas presentes:
#
#   TemperaturaAlta        1 serie de 4 máquinas    → 3 máquinas sin vigilancia térmica
#   CargaPorCoreAlta       2 de 4                   → las dos Windows no tienen load average
#   ServicioReiniciandose  54 series, TODAS del servidor → 0 de las 2 Windows
#   ServicioLento          1 serie                  → cubre 1 servicio de 184
#
# Cada uno de esos huecos tiene una razón buena —Windows no tiene load, el SCM no expone
# reinicios, A2 sigue abierto— y ninguna se podía leer desde ningún lado. La regla está cargada,
# su métrica existe, y aun así esa dimensión de esa máquina está a ciegas. Es «verde por el motivo
# equivocado» una vez más, sólo que esta vez no es un bug: es que no había dónde mirarlo.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# UN HUECO SIN DECLARAR ES UN HALLAZGO; UNO DECLARADO ES UNA DECISIÓN
#
# La razón la escribe la REGLA, en su anotación `ausente_en:`, y no este script. Es la misma forma
# que `# despliegue:` de A73, y por el mismo motivo: un catálogo de excepciones que vive en el
# verificador se desincroniza de las reglas y termina perdonando huecos que ya no corresponden.
#
#     ausente_en: "os=windows — el load average es un concepto de UNIX y ahí no existe"
#
# Cada cláusula es `<selector> — <la razón, en prosa>`, y varias se separan con `;`. Selectores:
# `os=<x>`, `tier=<x>`, `device=<x>` (límite estructural: esa máquina no lo va a tener nunca) y
# `sin-declarar` (la serie es opt-in por máquina: aparece cuando alguien la configura).
#
# La razón NO es decoración: es lo único que convierte el informe en algo accionable, y por eso un
# selector sin `—` no cuenta. Un selector desconocido NO se ignora en silencio: rompe la suite
# (TestCadaHuecoDeclaradoUsaUnSelectorQueElVerificadorEntiende) — porque ignorarlo convertiría una
# decisión declarada en un hallazgo, o peor, taparía un hueco de verdad.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# LO QUE ESTO NO MIRA
#
#   · Las alertas que NO son por máquina (las del cerebro: MusubiDown, el empuje, la cuota). No
#     tienen etiqueta `device`, así que la pregunta «¿cubre a esta máquina?» no les aplica.
#   · Si el UMBRAL es el correcto. Mide si hay serie, no si el número sirve.
#   · Las máquinas que no están enroladas. Una máquina fuera de la flota tiene cobertura cero y
#     este script no la ve — se listan aparte con `--tailnet` si tenés tailscale a mano.
# ════════════════════════════════════════════════════════════════════════════════════════════
#
# Uso:
#   MUSUBI_SSH=musubi-server ./deploy/verificar-cobertura.sh
#
# Sale 0 si todo hueco está declarado, 1 si hay alguno sin declarar, 2 si no pudo preguntar.

set -uo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROM_URL="${PROM_URL:-http://127.0.0.1:9099}"
SSH_HOST="${MUSUBI_SSH:-}"

if [ ! -f "$REPO/VERSION" ] || [ ! -d "$REPO/deploy" ]; then
  printf 'no encuentro el repositorio en %s. Corrélo desde el árbol del repo.\n' "$REPO" >&2
  exit 2
fi

# ── TODAS LAS CONSULTAS EN UNA SOLA CONEXIÓN ────────────────────────────────────────────────
#
# La primera versión abría un ssh por métrica: veinticinco conexiones, cuatro segundos cada una, y
# el verificador tardaba más de lo que nadie está dispuesto a esperar. Una comprobación lenta se
# corre una vez y después no se corre más, que es la misma muerte que no tenerla.
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Las métricas que las reglas nombran salen del REPO, no de una lista escrita a mano acá: una
# lista a mano se queda vieja el día que alguien agrega una regla, y el verificador la ignoraría.
#
# Y EL MISMO ARGUMENTO UN NIVEL ARRIBA: la lista de ARCHIVOS también era a mano, y también se quedó
# vieja. `musubi-recording.yml` no estaba, así que las métricas de las que dependen las once
# recording rules del SLA no se verificaban — y el SLA es lo que se le factura a un cliente. Si una
# de sus entradas deja de existir, las reglas siguen evaluándose, no dan error, y el `avg30d` sale
# de un promedio sobre nada. Se agrega acá porque es exactamente lo que este bloque dice hacer.
# Ver A93.
#
# Sólo aporta sus ENTRADAS y no sus salidas: la regex pide `_` después de `musubi`, así que
# `musubi:device_up:norm` (dos puntos, no guion bajo) no matchea. Es lo que se quiere — preguntarle
# a Prometheus por una serie que él mismo produce no verifica nada de la cobertura.
METRICAS="$(grep -hoE '\b(musubi|alertmanager|prometheus)_[a-z0-9_]+\b' \
  "$REPO"/deploy/musubi-alerts.yml "$REPO"/deploy/musubi-alerts-flota.yml \
  "$REPO"/deploy/musubi-recording.yml | sort -u)"

{
  printf 'P=%s\n' "$PROM_URL"
  printf 'pedir(){ printf "=== %%s\\n" "$2"; curl -sS -m 15 -G --data-urlencode "query=$1" "$P/api/v1/query"; printf "\\n"; }\n'
  printf 'pedir "%s" "%s"\n' "musubi_fleet_device_up" "__maquinas__"
  for m in $METRICAS; do printf 'pedir "%s" "%s"\n' "count by(device)($m)" "$m"; done
} > "$TMP/consulta.sh"

if [ -n "$SSH_HOST" ]; then
  ssh -o BatchMode=yes -o ConnectTimeout=10 "$SSH_HOST" 'bash -s' < "$TMP/consulta.sh" > "$TMP/todo.txt" 2>/dev/null
else
  bash "$TMP/consulta.sh" > "$TMP/todo.txt" 2>/dev/null
fi

if ! grep -q '"status"' "$TMP/todo.txt" 2>/dev/null; then
  printf 'no se pudo consultar %s\n' "$PROM_URL" >&2
  printf '(desde afuera del servidor hace falta MUSUBI_SSH=<host>: Prometheus escucha en loopback)\n' >&2
  exit 2
fi

python3 - "$REPO" "$TMP" <<'PY'
import json, os, re, sys

repo, tmp = sys.argv[1], sys.argv[2]

# ── Las reglas y las métricas que nombra cada una ────────────────────────────────────────────
# El patrón matchea por PREFIJO y no por «identificador seguido de {». La primera versión exigía
# `{`, `[` o un comparador detrás, y con eso `musubi_fleet_device_load1 / musubi_fleet_device_cpus`
# perdía el load1 — la métrica que decide si CargaPorCoreAlta puede evaluarse. Un extractor que
# pierde métricas no falla: reporta cobertura de más.
RE_METRICA = re.compile(r'\b(?:musubi|alertmanager|prometheus)_[a-z0-9_]+\b')
RE_ALERTA  = re.compile(r'^\s*-\s+alert:\s*(\S+)\s*$')

# ── QUÉ MÉTRICAS EXIGE DE VERDAD UNA EXPRESIÓN ──────────────────────────────────────────────
#
# Dos formas de PromQL rompen la lectura ingenua «todas las métricas que nombra», y las dos están
# en uso hoy:
#
#   `A unless on(device) B`   B es la NEGATIVA: la regla dispara cuando B FALTA. Exigirla como
#                             cobertura invierte el sentido — `MaquinaSinInventario` existe
#                             justamente para avisar que `musubi_fleet_service_up` no está.
#   `A or B`                  alcanza con una.
#
# `and` y la aritmética sí exigen las dos: `load5 / cpus` no se puede evaluar sin ninguna de ellas.
def metricas_exigidas(expr):
    izq = re.split(r'\bunless\b', expr)[0]
    metricas = sorted(set(RE_METRICA.findall(izq)))
    return metricas, bool(re.search(r'\bor\b', izq))

# ── UNA REGLA QUE SE AUTO-LIMITA NO NECESITA QUE NADIE LE DECLARE EL HUECO ───────────────────
#
# `musubi_fleet_device_up{tier="A"}` dice en su propia expresión que sólo mira Tier A. Pedirle
# además un `ausente_en: "tier=B — ..."` sería escribir dos veces la misma decisión, y dos copias
# de una decisión se contradicen el día que alguien cambia una.
def alcance_declarado(expr):
    return {c: v for c, v in re.findall(r'\b(device|tier|os)\s*=\s*"([^"]+)"', expr)}

reglas = {}
for archivo in ('musubi-alerts.yml', 'musubi-alerts-flota.yml'):
    actual, buf = None, []
    def cerrar():
        if actual:
            texto = '\n'.join(buf)
            # sólo las líneas útiles: un comentario que nombra una métrica para explicarla no es
            # una dependencia de la regla.
            util = '\n'.join(l for l in texto.split('\n') if not l.strip().startswith('#'))
            ausente = ''
            m = re.search(r'ausente_en:\s*["\']?(.+?)["\']?\s*$', util, re.M)
            if m: ausente = m.group(1)
            expr = util.split('annotations:')[0]
            metricas, cualquiera = metricas_exigidas(expr)
            reglas[actual] = {
                'metricas': metricas,
                'cualquiera': cualquiera,
                'ausente_en': ausente,
                'alcance': alcance_declarado(expr),
            }
    for linea in open(os.path.join(repo, 'deploy', archivo), encoding='utf-8'):
        m = RE_ALERTA.match(linea)
        if m:
            cerrar()
            actual, buf = m.group(1), []
        elif actual is not None:
            buf.append(linea.rstrip('\n'))
    cerrar()

# ── Lo que Prometheus tiene ──────────────────────────────────────────────────────────────────
# Una sola respuesta con secciones `=== <nombre>`, que es como llegó de la única conexión.
crudo = {}
clave = None
for linea in open(os.path.join(tmp, 'todo.txt'), encoding='utf-8'):
    if linea.startswith('=== '):
        clave = linea[4:].strip(); crudo[clave] = []
    elif clave:
        crudo[clave].append(linea)

def resultado(clave):
    try:
        d = json.loads(''.join(crudo.get(clave, [])))
        return d['data']['result'] if d.get('status') == 'success' else []
    except Exception:
        return []

maquinas = {}
for x in resultado('__maquinas__'):
    e = x['metric']
    maquinas[e['device']] = {'tier': e.get('tier', '?'), 'os': e.get('os', '?')}

por_metrica = {}
for metrica in crudo:
    if metrica == '__maquinas__': continue
    por_metrica[metrica] = {x['metric'].get('device') for x in resultado(metrica)
                            if x['metric'].get('device')}

# Una métrica SIN etiqueta `device` en ninguna serie no es por-máquina: la pregunta no le aplica.
def es_por_maquina(m):
    return bool(por_metrica.get(m))

# ── ¿Qué máquinas excusa cada `ausente_en`? ─────────────────────────────────────────────────
SELECTORES = ('os=', 'tier=', 'device=', 'sin-declarar')

def excusadas(ausente_en):
    """Devuelve (máquinas con límite estructural, opt-in sí/no, razón, selectores desconocidos).

    LA DISTINCIÓN ENTRE LAS DOS PRIMERAS ES EL CORAZÓN DEL INFORME.

    `os=windows` es un límite que NO se va a cerrar configurando algo: el load average no existe
    en Windows y no hay nada que activar. Esa máquina nunca va a tener esa dimensión, y decirlo
    una vez alcanza.

    `sin-declarar` es lo contrario: la serie aparece en cuanto alguien la configura. Excusarla
    como si fuera un límite borraría del informe lo que más importa mirar — `ServicioLento` cubre
    hoy 1 servicio de 184, y eso NO es una falla pero tampoco es cobertura. Va a su propio
    casillero, con el número a la vista.
    """
    if not ausente_en:
        return set(), False, '', []
    fuera, desconocidos, opcional = set(), [], False
    razon = ausente_en
    for parte in ausente_en.split(';'):
        # UN TROZO SIN `—` ES PROSA, NO UN SELECTOR ROTO.
        #
        # El separador de cláusulas es `;` y la razón se escribe en castellano, así que tarde o
        # temprano alguien pone un punto y coma adentro de la razón — pasó en la primera versión de
        # CargaPorCoreAlta, y la mitad de la frase se denunció como «selector que no entiendo».
        # Un verificador que castiga la puntuación de la explicación enseña a no explicar.
        if '—' not in parte:
            continue
        sel = parte.split('—')[0].strip()
        if sel == 'sin-declarar':
            opcional = True
        elif sel.startswith(('os=', 'tier=', 'device=')):
            clave, _, valor = sel.partition('=')
            for d, meta in maquinas.items():
                if clave == 'device':
                    if d == valor: fuera.add(d)
                elif meta.get(clave) == valor:
                    fuera.add(d)
        elif sel:
            desconocidos.append(sel)
    return fuera, opcional, razon, desconocidos

# ── El cruce ────────────────────────────────────────────────────────────────────────────────
sin_declarar, declarados, opcionales, malos = [], [], [], []
cubre = {d: [0, 0] for d in maquinas}   # [cubiertas, aplicables]

for alerta in sorted(reglas):
    info = reglas[alerta]
    metricas = [m for m in info['metricas'] if es_por_maquina(m)]
    if not metricas:
        continue                                  # alerta global (del cerebro), no por máquina
    fuera, opcional, razon, desconocidos = excusadas(info['ausente_en'])
    for sel in desconocidos:
        malos.append((alerta, sel))
    for d in sorted(maquinas):
        # La regla que se auto-limita manda: si su expresión dice {tier="A"}, una Tier B no está
        # descubierta — está FUERA DE ALCANCE, y no cuenta ni como hueco ni como cobertura.
        if any(maquinas[d].get(c, d if c == 'device' else None) != v and not (c == 'device' and d == v)
               for c, v in info['alcance'].items()):
            continue
        # `or` se conforma con una; todo lo demás exige todas. Conformarse siempre reportaría
        # cobertura que no existe, y exigir siempre la negaría donde alcanza con una.
        faltan = [m for m in metricas if d not in por_metrica.get(m, set())]
        tiene = (len(faltan) < len(metricas)) if info['cualquiera'] else not faltan
        if tiene:
            cubre[d][0] += 1; cubre[d][1] += 1
        elif d in fuera:
            # Límite estructural: esa máquina no va a tener esa dimensión nunca. No entra en el
            # denominador — contarla como «no vigilada» sería pedirle a Windows un load average.
            declarados.append((d, alerta, razon))
        elif opcional:
            cubre[d][1] += 1
            opcionales.append((d, alerta, razon))
        else:
            cubre[d][1] += 1
            sin_declarar.append((d, alerta, faltan))

# ── UNA COMPROBACIÓN QUE NO MIRÓ NADA NO PUEDE DECIR QUE TODO ESTÁ BIEN ─────────────────────
#
# Esta guarda existe porque el script se puso en VERDE sobre cero datos: al juntar las consultas en
# una sola conexión, las que llevaban espacios se partieron en varios argumentos, no volvió ninguna
# serie, y el informe dijo «0/0 · toda ausencia de cobertura está declarada» — la frase más
# tranquilizadora posible sobre nada.
#
# Es exactamente la enfermedad que este verificador vino a cazar, cometida por el verificador. Un
# `sys.exit(0)` alcanzable con el conjunto vacío es un falso verde esperando su turno.
if not maquinas:
    print('no volvió NINGUNA máquina de Prometheus: el informe sería sobre el conjunto vacío', file=sys.stderr)
    sys.exit(2)
aplicables = sum(t for _, t in cubre.values())
if aplicables == 0:
    print('ninguna regla resultó aplicable a ninguna máquina (%d métricas consultadas, %d con series).'
          % (len(por_metrica), sum(1 for v in por_metrica.values() if v)), file=sys.stderr)
    print('Eso no es cobertura perfecta: es que no se leyó nada. Revisá la consulta a Prometheus.', file=sys.stderr)
    sys.exit(2)

# ── El informe ──────────────────────────────────────────────────────────────────────────────
V, R, G, N = '\033[32m', '\033[31m', '\033[90m', '\033[0m'
print('\033[1mverificar-cobertura\033[0m — qué vigila realmente cada máquina\n')
print('  %-15s %-5s %-9s %-9s %s' % ('máquina', 'tier', 'os', 'vigilada', 'huecos declarados'))
for d in sorted(maquinas):
    c, t = cubre[d]
    dec = sum(1 for x in declarados if x[0] == d)
    print('  %-15s %-5s %-9s %-9s %s' % (d, maquinas[d]['tier'], maquinas[d]['os'],
                                         '%d/%d' % (c, t), dec if dec else '—'))

if sin_declarar:
    print('\n\033[1mHUECOS SIN DECLARAR\033[0m — una máquina a ciegas en esa dimensión, y nada dice por qué')
    for d, alerta, faltan in sorted(sin_declarar):
        print('  %s✘%s %-15s %-28s falta %s' % (R, N, d, alerta, ', '.join(faltan)))
    print('\n  Si el hueco es legítimo, declaralo en la regla:')
    print('      ausente_en: "os=windows — <la razón, en prosa>"')

if opcionales:
    print('\n\033[1mCOBERTURA OPCIONAL, SIN ACTIVAR\033[0m — se cierra configurando, no es una falla')
    vistos = {}
    for d, alerta, razon in opcionales:
        vistos.setdefault((alerta, razon), []).append(d)
    for (alerta, razon), ds in sorted(vistos.items()):
        cubiertas = len(maquinas) - len(ds)
        print('  %s○%s %-28s activa en %d de %d — falta en %s' % (G, N, alerta, cubiertas, len(maquinas), ', '.join(sorted(ds))))
        print('    %s%s%s' % (G, razon, N))

if declarados:
    print('\n\033[1mLÍMITES ESTRUCTURALES\033[0m — esa máquina no va a tener esa dimensión nunca')
    vistos = {}
    for d, alerta, razon in declarados:
        vistos.setdefault((alerta, razon), []).append(d)
    for (alerta, razon), ds in sorted(vistos.items()):
        print('  %s·%s %-28s %s' % (G, N, alerta, ', '.join(sorted(ds))))
        print('    %s%s%s' % (G, razon, N))

if malos:
    print('\n\033[1mSELECTORES QUE NO ENTIENDO\033[0m — el hueco NO queda excusado')
    for alerta, sel in malos:
        print('  %s✘%s %-28s %r  (válidos: %s)' % (R, N, alerta, sel, ', '.join(SELECTORES)))

print()
if sin_declarar or malos:
    print('%shay cobertura sin explicar%s — arriba está qué máquina y en qué dimensión' % (R, N))
    sys.exit(1)
print('%stoda ausencia de cobertura está declarada%s (dentro de lo que esto mira; ver el encabezado)' % (V, N))
PY
