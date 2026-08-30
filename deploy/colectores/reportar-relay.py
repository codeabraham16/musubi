#!/usr/bin/env python3
# ════════════════════════════════════════════════════════════════════════════════════════════
# reportar-relay.py — ¿el relay de RustDesk CONTESTA? → MUSUBI (A36)
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# POR QUÉ HACE FALTA SI EL AGENTE YA VE LOS CONTENEDORES
#
# El agente enumera `musubi-rustdesk-hbbs` y `musubi-rustdesk-hbbr` solo, y `ServicioCaido` ya
# avisa si el proceso muere. Eso cubre la mitad fácil.
#
# Lo que NO cubre es la mitad que rompe de verdad: **un contenedor levantado que no acepta
# conexiones**. `musubi_fleet_service_up` ve el proceso vivo conteste bien o mal — es exactamente
# la distinción que motivó la fase 4—, y para un relay «vivo pero mudo» se ve idéntico a sano
# hasta que alguien no puede abrir una pantalla, que era el síntoma anotado en A36.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# SERVICIO PROPIO Y NO EL DEL CONTENEDOR — SI NO, LOS DOS SE PISAN
#
# Este colector NO reporta sobre `musubi-rustdesk-hbbs`: ese nombre lo reporta el AGENTE en cada
# latido, y dos reportantes sobre la misma fila se sobrescriben por turnos. El último que llega
# gana y la salud queda decidida por una carrera.
#
# Reporta sobre `relay-rustdesk`, declarado a mano, que significa otra cosa: **el relay responde
# en sus puertos**. Dos servicios, dos preguntas, ningún empate.
#
#     musubi_fleet_service_declare device=<esta máquina> nombre=relay-rustdesk
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# QUÉ MIDE, Y POR QUÉ UN TCP DESNUDO ALCANZA
#
# hbbs y hbbr hablan su propio protocolo binario; hacerles un saludo de verdad sería reimplementar
# medio cliente (la misma razón por la que B12 descartó verificar ids contra el relay). Lo que sí
# prueba algo es que el puerto ACEPTE la conexión: un contenedor colgado, un proceso que perdió el
# bind o un cortafuegos nuevo dejan de aceptar, y eso es justo lo que `up` no ve.
#
# Se prueban los TRES puertos TCP publicados. El UDP 21116 NO se prueba: sin handshake no hay
# forma de distinguir «llegó y nadie contestó» de «se perdió el paquete», y una medición que no
# distingue esas dos cosas es ruido con forma de dato.
#
# EL RENDIMIENTO ES «CUÁNTOS PUERTOS ATENDIERON» y la latencia es la del saludo TCP. No es
# decoración: `ServicioLento` sobre el relay significa que el server está saturado o la red del
# tailnet degradada, que es información que hoy no existe en ningún lado.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# EMPUJA SIEMPRE, TAMBIÉN CUANDO FALLA — Y ACÁ ES AL REVÉS QUE EN EL COLECTOR DEL BOT
#
# `reportar-bot.py` NO empuja si el scrape falla, porque no poder mirar la base no dice nada del
# bot. Acá el intento ES la medición: un puerto que no acepta es el resultado, no un fallo del
# colector. Por eso un cero se empuja como `fallado`, y el silencio de este script significa una
# sola cosa —que el script murió—, que es lo que mira `ServicioSinNoticias`.
#
# Config: MUSUBI_RELAY_HOST (default: la IP del tailnet de esta máquina) y el token del agente.
# ════════════════════════════════════════════════════════════════════════════════════════════
import json
import os
import socket
import sys
import time
import urllib.request

SERVICIO = "relay-rustdesk"
CEREBRO = os.environ.get("MUSUBI_URL", "http://127.0.0.1:7717").rstrip("/")
HOST = os.environ.get("MUSUBI_RELAY_HOST", "127.0.0.1")
# Los tres TCP que publica el relay. 21115 y 21116 son de hbbs (prueba de NAT y registro); 21117
# es de hbbr (el relevo propiamente dicho). Si alguno deja de aceptar, media función se cae.
PUERTOS = [21115, 21116, 21117]
TIMEOUT = 4.0
# La ventana que se declara junto a los números. Va explícita porque «3 atendidas» no significa
# nada sin saber en cuánto tiempo, y deducirla del intervalo del cron ata el gráfico a un número
# que vive en otro archivo (crontab).
VENTANA_SEG = int(os.environ.get("MUSUBI_RELAY_VENTANA_SEG", "60"))


def leer_token():
    """El token del DISPOSITIVO, el mismo que usa el agente. No abre /mcp."""
    v = os.environ.get("MUSUBI_DEVICE_TOKEN", "").strip()
    if v:
        return v
    ruta = os.environ.get("MUSUBI_DEVICE_TOKEN_FILE",
                          os.path.expanduser("~/.config/musubi-agente/token"))
    try:
        return open(ruta).read().strip()
    except OSError as e:
        print("reportar-relay: no se pudo leer el token (%s): %s" % (ruta, e), file=sys.stderr)
        return ""


def probar(host, puerto):
    """Devuelve (atendio, milisegundos). El reloj arranca ANTES de resolver el nombre a propósito:
    un DNS lento también es el relay tardando en atender, desde donde mira quien lo usa."""
    t0 = time.monotonic()
    try:
        with socket.create_connection((host, puerto), timeout=TIMEOUT):
            pass
        return True, (time.monotonic() - t0) * 1000.0
    except OSError:
        return False, (time.monotonic() - t0) * 1000.0


def main():
    token = leer_token()
    if not token:
        return 2

    atendidas, fallidas, latencias, caidos = 0, 0, [], []
    for p in PUERTOS:
        ok, ms = probar(HOST, p)
        if ok:
            atendidas += 1
            latencias.append(ms)
        else:
            fallidas += 1
            caidos.append(p)

    # EL ESTADO SALE DE LOS PUERTOS, no de si el script anduvo. `fallado` con uno solo caído y no
    # con todos: hbbs y hbbr son procesos distintos, y que ande la mitad es una falla completa
    # para quien quiere abrir una pantalla — el registro sin relevo no sirve, y al revés tampoco.
    estado = "corriendo" if fallidas == 0 else "fallado"

    rendimiento = {
        "ventana_seg": VENTANA_SEG,
        "atendidas": atendidas,
        "fallidas": fallidas,
    }
    # LAS LATENCIAS SÓLO SI HUBO ALGUNA. Un p95 de 0 ms sobre cero conexiones no es un percentil
    # bajo: es la ausencia de uno, y mandarlo como 0 haría que el gráfico dibuje al relay más
    # rápido que nunca justo cuando está muerto.
    if latencias:
        latencias.sort()
        # p95 sobre tres muestras es el máximo, y se dice así en vez de disimularlo con una
        # interpolación que fingiría una precisión que tres puntos no tienen.
        rendimiento["latencia_p95_ms"] = int(round(latencias[-1]))
        rendimiento["latencia_max_ms"] = int(round(latencias[-1]))
    if caidos:
        rendimiento["desglose"] = {("puerto_%d" % p): 1 for p in caidos}

    # SE REGISTRA ANTES DE REPORTAR. Si el POST falla, el diagnóstico local no se pierde: la
    # medición vale más que el transporte, y el caso en que más importa saber qué puerto se cayó
    # es justo aquel en que el cerebro tampoco contesta.
    if caidos:
        print("reportar-relay: NO atienden los puertos %s de %s" % (caidos, HOST), file=sys.stderr)

    cuerpo = {"servicios": [{
        "nombre": SERVICIO,
        "salud": {
            "tomada": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "estado": estado,
            "rendimiento": rendimiento,
        },
    }]}
    req = urllib.request.Request(
        CEREBRO + "/fleet/service-health", data=json.dumps(cuerpo).encode(), method="POST",
        headers={"Authorization": "Bearer " + token, "Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=15) as r:
            res = json.load(r)
    except Exception as e:
        print("reportar-relay: no se pudo reportar: %s" % e, file=sys.stderr)
        return 1
    # LOS `desconocidos` SE MIRAN: el error más probable es que falte el service_declare, y su
    # síntoma sin esto sería un panel que nunca cambia y un cron en verde.
    if res.get("desconocidos"):
        print("reportar-relay: el cerebro no conoce %s. Declaralo con musubi_fleet_service_declare."
              % res["desconocidos"], file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
