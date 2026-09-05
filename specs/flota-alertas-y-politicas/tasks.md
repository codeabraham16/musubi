# S10 — tareas

## Hecho

### A11 · La poda cuelga de algo que corre
- [x] `RunFlotaScheduler` — el latido propio de la flota, **aparte** del mantenimiento de la
      memoria: colgarla de `maintenance.auto_interval_hours` habría atado la caducidad de datos de
      flota a un número que se toca por razones de memoria (I1).
- [x] `podarSalidasSiToca` — como mucho una vez por hora, no en cada tick.
- [x] `fleet.command_output_retention_days` (default 30; negativo = nunca caducan).
- [x] La fila sobrevive: caduca el CONTENIDO de stdout/stderr, no la auditoría (I6).

### A19 · Sondeo automático — y el umbral que lo hacía inútil
- [x] `sondearProyecto` con paralelismo acotado (4) y un solo barrido en vuelo (I5).
- [x] Saltea Tier A, iOS y lo que no tiene `metrics` concedido (I3).
- [x] **`umbralEnLineaPara`: el umbral de «en línea» es POR TIER** (I2). Es la mitad que faltaba:
      con 90 s fijos, un Tier B sondeado cada 5 min figura caído el 97 % del tiempo y
      `MaquinaCaida` dispara para siempre. Aplicado en el listado, las métricas, `/metrics`, exec
      y pantalla — una sola definición.
- [x] `umbral_segundos` viaja por máquina en el inventario: dos filas con el mismo silencio y
      distinto `online` ya no parecen un bug.

### A12 · Allowlist de comandos
- [x] `fleet_exec_allow` en `principals.yaml`: vive en la CREDENCIAL, no en el aparato (I7).
- [x] Match sobre `argv[0]` EXACTO — sin basename, que dejaría pasar `/tmp/evil/systemctl` (I10).
- [x] Sección ausente ⇒ sin restricción; presente ⇒ **exhaustiva**; lista vacía ⇒ cero comandos (I9).
- [x] Vale para los dos transportes (cola de Tier A y SSH de Tier B): una sola compuerta.
- [x] `musubi_tool_rejections_total{reason="fleet_allowlist"}`, aparte de `authz`.
- [x] Aviso de arranque cuando la allowlist contiene un intérprete (I10b).
- [x] La allowlist efectiva se muestra en el inventario, distinguiendo «sin restricción» de «nada».

### A10 · Políticas (auto-heal)
- [x] `fleet.Politica` + `Dispara` — enum acotado de condiciones, no un mini-lenguaje.
- [x] **Una política actúa con la autoridad de un principal, nunca con la del daemon** (I11).
      Misma compuerta, misma allowlist, misma bitácora. Revocar al principal la apaga.
- [x] Validación de ARRANQUE en dos tiempos: sintaxis sin registro, principal con registro (I12).
- [x] No actúa sobre muestra rancia ni sobre una máquina que late sin medir (I13).
- [x] Cooldown por (política × máquina), marcado ANTES de ejecutar (I14).
- [x] Nacen apagadas (I15). Acciones en la misma bitácora que las personas (I16).
- [x] `musubi_fleet_policy_actions_total{policy,result}` — sin etiqueta de máquina, para no
      entregarle el inventario de un tenant a cualquier scraper.
- [x] Los fallos de configuración se avisan UNA vez (son estados, no eventos); la métrica cuenta
      siempre, porque de ella vive la alerta.

### A4 · Alertmanager
- [x] `deploy/prometheus/alertmanager.yml` — rutas por severidad, agrupado por alerta **y máquina**,
      secretos por archivo (`bot_token_file`, `url_file`), nunca en la config versionada.
- [x] `inhibit_rules`: una máquina caída no dispara además disco+RAM+temperatura; `FlotaSinTelemetria`
      calla al resto de flota.
- [x] `alerting:` deja de ser un comentario en `prometheus.yml`.
- [x] **`MusubiSiempreViva`** — dead-man's switch: siempre en firing, ruteado a un watchdog. Si
      DEJA de llegar, lo roto es la cadena de alertas y el silencio de todo lo demás no significa
      nada (I18).
- [x] `PoliticaQueNoCura`, `PoliticaSinPermiso`, `AllowlistDeFlotaRechazando` (I19).
- [x] `deploy/RUNBOOK.md`: Alertmanager (montaje + verificación de punta a punta) y las tres
      secciones nuevas.
- [x] Pruebas sobre los propios archivos de despliegue: cada `runbook:` apunta a una sección que
      existe, nombres de alerta únicos, `severity` declarada, el switch sigue incondicional y su
      ruta existe, y no hay secretos en claro.

## Pruebas

**41 pruebas nuevas** (8 de dominio, 26 de servidor, 4 sobre los archivos de despliegue, 3 de configuración).
**16 sabotajes verificados uno por uno**: cada uno se aplicó al código, se corrió la prueba, se
confirmó que FALLA, y se restauró el archivo.

Dos pruebas **no valían nada y se reescribieron** — queda anotado porque la lección se repite:

1. `TestUnaMetricaQueNoSePudoMedir` probaba `carga_por_core > 2` con el dato ausente. Con el
   sabotaje puesto (nil leído como 0) pasaba igual: `0 > 2` es falso. Medía el lado inofensivo del
   error. El lado peligroso es la única condición que dispara al BAJAR: `disco_libre_pct < 10` con
   el disco sin medir daría `0 < 10` = VERDADERO, y la política saldría a borrar en una máquina de
   la que no sabemos nada. Reescrita en esa dirección.
2. `TestRevocarAlPrincipalApagaLaPolitica` avanzaba 24 h y dejaba la muestra vieja: la política no
   actuaba, sí, pero por la guarda de rancidez, no por la revocación. Pasaba con y sin el arreglo.
   Ahora avanza lo justo para salir del cooldown y vuelve a latir con la condición cumplida.

## E2E contra procesos reales

Cerebro aislado (`MUSUBI_HOME` temporal, `127.0.0.1:7799` — **verificado antes de arrancar**, que
es la lección de S9). Lo comprobado:

| # | Qué | Resultado |
|---|---|---|
| 2 | `gio` con allowlist `[uptime, df]` sobre `pc-gio` | `uptime` aceptado; `rm -rf /` y `/tmp/evil/uptime` rechazados |
| 3 | El inventario muestra la allowlist efectiva | `comandos_permitidos: ['uptime','df']`, `umbral: 90 s` |
| 4-5 | Latido con RAM al 95 % → la política actúa | `nas · auto-heal · journalctl --vacuum-size=200M`, en la misma tabla que `pc-gio · gio · uptime` |
| 6 | `/metrics` | `musubi_fleet_policy_actions_total{policy="vaciar-journal",result="ok"} 1` |
| 7 | 5 barridos más con la condición cumplida | **1 sola acción**: el cooldown contuvo la tormenta |
| 8 | Se borra `auto-heal` de `principals.yaml` | La política queda inerte en ≤10 s, lo dice el log y lo cuenta `result="sin_principal"` |
| 9 | Arrancar con una política que nombra a alguien inexistente | El servidor **se niega a arrancar** |
| 10 | Allowlist con `bash` | WARN al arranque: «esa entrada no restringe nada» |

El paso 8 destapó un defecto real: **17 WARN idénticos en un minuto**. A 5 min de tick son 288 por
día — el ruido que entierra la línea que importa, y contra el criterio que el propio scheduler ya
usaba. Corregido con `avisarUnaVez` (la métrica sigue contando cada evaluación) y fijado con su
propia prueba y su sabotaje.
