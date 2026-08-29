#!/usr/bin/env python3
# ════════════════════════════════════════════════════════════════════════════════════════════
# reportar-bot.py — la salud del bot Alturito20 → MUSUBI (fase 4)
#
# REEMPLAZA a monitoring/infra/collect-bot.py, que empujaba a OpenObserve. Mide EXACTAMENTE lo
# mismo, con el mismo patrón marcador-por-id, y lo manda al cerebro por
# POST /fleet/service-health con el token del DISPOSITIVO.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# QUÉ CAMBIA Y QUÉ NO
#
# NO cambia la medición: mismo SQL, mismo marcador, mismos resultados. Si algo se rompe al
# migrar, se rompe en el transporte y no en el número, que es lo que hace la comparación posible.
#
# SÍ cambia el destino, y con él lo que se puede hacer con el dato: entra al mismo inventario que
# el resto de la flota, sale por /metrics con las otras series (`musubi_fleet_service_handled`,
# `_failed`, `_window_seconds`, `_latency_p95_ms`), se dibuja en el panel al lado de la máquina
# donde corre, y tiene reglas de alerta (`ServicioFallandoPorDentro`, `ColectorDeRendimientoMudo`,
# `ServicioLento`).
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# EL SERVICIO TIENE QUE ESTAR DECLARADO ANTES, y no es un trámite: es la parte del diseño que
# impide que el bot parpadee en el panel.
#
#     musubi_fleet_service_declare device=<esta máquina> nombre=alturito20
#
# La puerta de salud NO CREA servicios a propósito. El camino del latido crea con `declared = 0`
# —podable por ausencia— y el siguiente latido del agente, que enumera systemd y contenedores,
# borraría el bot porque no está en ninguno de los dos. Declarado a mano queda `declared = 1` y
# fuera del alcance de esa poda.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# EMPUJA SIEMPRE, INCLUSO CON n=0 — Y NO EMPUJA SI EL SCRAPE FALLA
#
# Las dos mitades de la misma decisión, heredadas del colector original y ahora sostenidas también
# del lado del cerebro:
#
#   n=0 SE EMPUJA. Es el latido que prueba que el colector vive. Con él, la serie existe y vale
#   cero («el bot no tuvo consultas»); sin colector, la serie DESAPARECE («el colector murió»).
#   `ColectorDeRendimientoMudo` mira esa desaparición.
#
#   UN SCRAPE FALLIDO NO EMPUJA NADA. Mandar un 0 ahí sería afirmar «miré y no pasó nada» cuando
#   la verdad es «no pude mirar», y dispararía la alerta equivocada. El silencio es el síntoma
#   correcto, y el motivo queda en el log.
#
# Config: supabase-pat.env (SB_PAT, SB_PROJECT_REF) + el token del agente.
# Estado: bot-salud-state.json (junto al script).
# ════════════════════════════════════════════════════════════════════════════════════════════
import json
import os
import sys
import time
import urllib.error
import urllib.request

DIR = os.path.dirname(os.path.abspath(__file__))
STATE_FILE = os.path.join(DIR, "bot-salud-state.json")

CEREBRO = os.environ.get("MUSUBI_BRAIN_URL", "http://127.0.0.1:7717").rstrip("/")
SERVICIO = os.environ.get("MUSUBI_BOT_SERVICIO", "alturito20")
TABLA = os.environ.get("MUSUBI_BOT_TABLA", "bot_consultas")
# La ventana la declara el colector porque él sabe cada cuánto corre. Deducirla del otro lado
# ataría el número a un cron que vive en otra máquina.
VENTANA_SEG = int(os.environ.get("MUSUBI_BOT_VENTANA_SEG", "60"))


def leer_env(path, clave):
    """Lee una clave de un archivo .env, igual que el colector original."""
    try:
        for ln in open(path):
            ln = ln.strip()
            if ln.startswith(clave + "="):
                v = ln.split("=", 1)[1].strip()
                if len(v) >= 2 and v[0] in "\"'" and v[-1] == v[0]:
                    v = v[1:-1]
                return v
    except FileNotFoundError:
        pass
    return None


def leer_token():
    """El token del DISPOSITIVO, el mismo que usa el agente.

    NO es el de una persona: no abre /mcp, y sólo sirve para las puertas de /fleet/. Sale de un
    archivo modo 600 para poder rotarlo sin editar nada más, igual que en la unidad del agente.
    """
    v = os.environ.get("MUSUBI_DEVICE_TOKEN", "").strip()
    if v:
        return v
    ruta = os.environ.get("MUSUBI_DEVICE_TOKEN_FILE",
                          os.path.expanduser("~/.config/musubi-agente/token"))
    try:
        return open(ruta).read().strip()
    except OSError as e:
        print("reportar-bot: no se pudo leer el token del dispositivo (%s): %s" % (ruta, e),
              file=sys.stderr)
        return ""


SB_PAT = leer_env(os.path.join(DIR, "supabase-pat.env"), "SB_PAT")
SB_REF = leer_env(os.path.join(DIR, "supabase-pat.env"), "SB_PROJECT_REF")


def sb_query(sql):
    url = "https://api.supabase.com/v1/projects/%s/database/query" % SB_REF
    req = urllib.request.Request(
        url, data=json.dumps({"query": sql}).encode(), method="POST",
        headers={"Authorization": "Bearer " + SB_PAT, "Content-Type": "application/json",
                 "User-Agent": "musubi-reportar-bot/1.0"})
    with urllib.request.urlopen(req, timeout=20) as r:
        return json.load(r)


def cargar_estado():
    try:
        with open(STATE_FILE) as f:
            return json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        return {}


def guardar_estado(st):
    tmp = STATE_FILE + ".tmp"
    with open(tmp, "w") as f:
        json.dump(st, f)
    os.replace(tmp, STATE_FILE)


def reportar(token, rendimiento):
    """POST /fleet/service-health.

    LA MÁQUINA SALE DEL TOKEN y el cuerpo no tiene por dónde decirla: es la misma garantía que el
    latido y el resultado de un comando. Por eso este script no necesita saber en qué máquina corre.
    """
    cuerpo = {"servicios": [{
        "nombre": SERVICIO,
        "salud": {
            "tomada": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            # `corriendo` porque el colector acaba de hablar con la base del bot. Si no pudiera,
            # no llegaría hasta acá: el scrape fallido sale antes, sin empujar.
            "estado": "corriendo",
            "rendimiento": rendimiento,
        },
    }]}
    req = urllib.request.Request(
        CEREBRO + "/fleet/service-health", data=json.dumps(cuerpo).encode(), method="POST",
        headers={"Authorization": "Bearer " + token, "Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=15) as r:
        res = json.load(r)
    # LOS `desconocidos` SE MIRAN. El error más probable de este camino es apuntar a un servicio
    # que nadie declaró —un typo, o que se olvidó el paso del `service_declare`—, y su síntoma sin
    # esto sería un panel que nunca cambia y un cron en verde.
    if res.get("desconocidos"):
        print("reportar-bot: el cerebro no conoce %s. Declaralo con "
              "musubi_fleet_service_declare y volvé a correr." % res["desconocidos"],
              file=sys.stderr)
        return 1
    return 0


def main():
    if not (SB_PAT and SB_REF):
        print("reportar-bot: falta SB_PAT/SB_PROJECT_REF en supabase-pat.env", file=sys.stderr)
        return 1
    token = leer_token()
    if not token:
        return 1

    st = cargar_estado()
    last = st.get("last_id")

    # Primera corrida: se siembra el marcador en el max(id) actual y se empuja el latido en cero.
    # NO se vuelca la historia: un backfill entrando por el camino del reporte periódico
    # descoloca cualquier tasa, y el cerebro lo rechaza (ventana > un día).
    if last is None:
        try:
            rows = sb_query("select coalesce(max(id),0) mx from %s" % TABLA)
            last = int(rows[0]["mx"])
        except Exception as e:
            print("reportar-bot: no pude sembrar el marcador: %s" % e, file=sys.stderr)
            return 0
        st["last_id"] = last
        guardar_estado(st)
        reportar(token, {"ventana_seg": VENTANA_SEG, "atendidas": 0, "fallidas": 0})
        print("reportar-bot: sembrado last_id=%d" % last, flush=True)
        return 0

    # El max(id) avanza sobre TODO lo nuevo (incluidas las de prueba) para no re-escanearlas; los
    # conteos las excluyen. Mismo SQL que el colector original, a propósito: si algo se rompe al
    # migrar, que se rompa en el transporte y no en el número.
    sql = (
        "select "
        " coalesce(max(id), %d) mx, "
        " count(*) filter (where not coalesce(prueba,false)) n, "
        " count(*) filter (where resultado='ok'       and not coalesce(prueba,false)) ok, "
        " count(*) filter (where resultado='no_puedo' and not coalesce(prueba,false)) no_puedo, "
        " count(*) filter (where resultado='vacio'    and not coalesce(prueba,false)) vacio, "
        " count(*) filter (where resultado='error'    and not coalesce(prueba,false)) err, "
        " coalesce(percentile_cont(0.95) within group (order by ms) "
        "   filter (where not coalesce(prueba,false)),0)::int ms_p95, "
        " coalesce(max(ms) filter (where not coalesce(prueba,false)),0) ms_max "
        "from %s where id > %d" % (last, TABLA, last)
    )
    try:
        rows = sb_query(sql)
    except Exception as e:
        # NO SE EMPUJA. Un 0 acá afirmaría «miré y no pasó nada» cuando la verdad es «no pude
        # mirar», y dispararía la alerta equivocada. El silencio es el síntoma correcto.
        print("reportar-bot: el scrape falló, no se empuja: %s" % e, file=sys.stderr)
        return 0

    r = rows[0] if rows else {}
    mx = int(r.get("mx") or last)
    n = int(r.get("n") or 0)
    ok = int(r.get("ok") or 0)
    no_puedo = int(r.get("no_puedo") or 0)
    vacio = int(r.get("vacio") or 0)
    err = int(r.get("err") or 0)

    rendimiento = {
        "ventana_seg": VENTANA_SEG,
        "atendidas": n,
        # FALLIDAS ES UN SUBCONJUNTO, no un total aparte: sólo los `error`. `no_puedo` y `vacio`
        # son respuestas del bot, no fallas suyas — meterlas acá haría que una búsqueda sin
        # resultados cuente como error y la tasa no signifique nada. El cerebro rechaza el reporte
        # entero si fallidas supera a atendidas, así que la cuenta tiene que cerrar.
        "fallidas": min(err, n),
        "desglose": {"ok": ok, "no_puedo": no_puedo, "vacio": vacio, "error": err},
    }
    # LA LATENCIA SÓLO SI HUBO ALGO QUE MEDIR. Sobre cero unidades no hay percentil, y el cerebro
    # rechaza el reporte si viene igual: un 0 ahí hunde el promedio justo en los minutos tranquilos.
    if n > 0:
        rendimiento["latencia_p95_ms"] = int(r.get("ms_p95") or 0)
        rendimiento["latencia_max_ms"] = int(r.get("ms_max") or 0)

    try:
        code = reportar(token, rendimiento)
    except urllib.error.HTTPError as e:
        print("reportar-bot: el cerebro devolvió %s: %s" % (e.code, e.read()[:300]), file=sys.stderr)
        return 1
    except Exception as e:
        print("reportar-bot: no se pudo entregar: %s" % e, file=sys.stderr)
        return 1

    # El marcador avanza SÓLO si el reporte llegó. Avanzarlo antes perdería esas consultas para
    # siempre en la primera falla de red: la próxima corrida arrancaría después de ellas.
    st["last_id"] = mx
    guardar_estado(st)
    return code


if __name__ == "__main__":
    sys.exit(main())
