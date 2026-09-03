# Runbook operativo — cerebro central de Musubi

Qué hacer ante cada alerta de [`musubi-alerts.yml`](musubi-alerts.yml). El cerebro es el único punto donde converge la memoria compartida de todos los proyectos: tratá los eventos de DR y disponibilidad como de alta consecuencia.

Diagnóstico rápido (siempre): `musubi doctor` (en el host del cerebro) da un panorama de integridad, esquema y backup; `curl -s localhost:7717/metrics` (o vía tailnet con el bearer) muestra los gauges en vivo; `journalctl -u musubi-brain -n 100` y `systemctl status musubi-backup` cubren daemon y timer.

---

## MusubiDown
**Qué significa:** Prometheus no pudo scrapear el cerebro por >5 min (proceso caído, host caído, o red del tailnet cortada).
**Acción:**
1. `systemctl status musubi-brain` → si está `failed`, `journalctl -u musubi-brain -n 200` para la causa.
2. Verificá el tailnet (`tailscale status`) y que el puerto responda: `curl -sS localhost:7717/readyz`.
3. `systemctl restart musubi-brain` si el proceso murió; confirmá `readyz` en 200.

## MusubiBackupOffhostStale
**Qué significa:** el backup off-host **nunca funcionó** (`age < 0`) o **dejó de shipear** (`> 48h`). El CRÍTICO del baseline: perder el disco = perder toda la memoria compartida.
**Acción:**
1. `systemctl status musubi-backup` y `journalctl -u musubi-backup -n 50`.
2. Revisá `BACKUP_REMOTE` en el EnvironmentFile (`/etc/musubi/musubi.env`): destino válido (rsync/rclone/cp) y credenciales/rutas correctas.
3. Corré el backup a mano: `sudo systemctl start musubi-backup` → debe terminar `success` y borrar `.musubi/backups/.last_offhost_error`.
4. Si es local-only a conciencia, seteá `BACKUP_ALLOW_LOCAL_ONLY=1` (asumiendo el riesgo) — pero preferí configurar un destino off-host real.
5. Verificá restore de tanto en tanto (runbook de restore en [`Server_Brain_Onboarding.md`](../docs/Server_Brain_Onboarding.md)).

## MusubiOutboxDead
**Qué significa:** observaciones `shared` que agotaron los reintentos de sync al central — no se están propagando.
**Acción:**
1. `curl -s localhost:7717/metrics | grep musubi_sync_outbox` para ver el tamaño.
2. Revisá conectividad al destino de sync y su auth; `journalctl` del daemon para el último error.
3. Tras resolver la causa, re-encolá con `musubi sync requeue` (mueve las muertas a pending) y observá que `state="dead"` baje.

## MusubiVectorIndexUntrained
**Qué significa:** hay >10k embeddings pero el IVF no tiene centroides → el recall cae a full-scan exacto (correcto pero más lento a escala).
**Acción:**
1. Suele resolverse solo (el índice se entrena/reconstruye en background). Si persiste, reiniciá el daemon para forzar el arranque caliente del índice.
2. Si acabás de correr `musubi embed backfill`, reiniciá el daemon para que el índice incluya los vectores nuevos.

## MusubiQuotaRejections
**Qué significa:** un principal viene chocando la cuota por-minuto (`service.quota_per_minute`, default 600). Puede ser un agente legítimo intenso o uno desbocado.
**Acción:**
1. Identificá el principal (logs del daemon). Si es legítimo y necesita más, subí `service.quota_per_minute` para él/globalmente.
2. Si es anómalo, revocá su token (editá `principals.yaml` quitando la línea — la **revocación es en caliente**, sin reiniciar).

## MusubiAuthzRejectionsSpike
**Qué significa:** pico sostenido de rechazos de autorización — probing, un cliente con token revocado/vencido, o un rol intentando algo que no le corresponde.
**Acción:**
1. Revisá los logs del daemon por IP/patrón. El lockout anti fuerza-bruta ya frena por IP.
2. Si es un miembro con credencial vieja, reemitile un token (`musubi token new`). Si es hostil, confirmá las ACLs del tailnet.

## MusubiToolErrorRateHigh
**Qué significa:** >20% de las tools/call fallan (sostenido). Suele indicar un problema del motor (base bloqueada/corrupta) o un cliente mal comportado.
**Acción:**
1. `musubi doctor` → integridad/esquema. Si hay corrupción, restaurá del último backup off-host.
2. Mirá `musubi_tool_invocations_total{result="error"}` por tool para aislar cuál falla y correlacioná con los logs.

---

## Alertmanager

**Qué es.** El tramo entre «la regla se evalúa» y «alguien se entera». Antes de S10 no existía:
las reglas de `musubi-alerts.yml` se veían en `http://127.0.0.1:9099/alerts` y no notificaban a
ningún lado.

**Montarlo** (systemd nativo, al estilo del resto del deploy):

```bash
# 1. Binario
ALERTMANAGER_VERSION=0.28.1
curl -fsSL -o /tmp/am.tgz \
  https://github.com/prometheus/alertmanager/releases/download/v${ALERTMANAGER_VERSION}/alertmanager-${ALERTMANAGER_VERSION}.linux-amd64.tar.gz
sudo tar -xzf /tmp/am.tgz -C /usr/local/bin --strip-components=1 \
  alertmanager-${ALERTMANAGER_VERSION}.linux-amd64/alertmanager
sudo useradd --system --no-create-home --shell /usr/sbin/nologin alertmanager || true

# 2. Config y secretos. El archivo de config se puede commitear; los de /etc/musubi NO.
sudo install -d -o alertmanager -g alertmanager /etc/alertmanager /var/lib/alertmanager
sudo install -m 644 deploy/prometheus/alertmanager.yml /etc/alertmanager/alertmanager.yml
sudo install -d -m 750 -o root -g alertmanager /etc/musubi
printf '%s' "$TELEGRAM_BOT_TOKEN" | sudo tee /etc/musubi/telegram_bot_token >/dev/null
printf '%s' "$WATCHDOG_URL"       | sudo tee /etc/musubi/watchdog_url       >/dev/null
sudo chmod 640 /etc/musubi/telegram_bot_token /etc/musubi/watchdog_url
sudo chgrp alertmanager /etc/musubi/telegram_bot_token /etc/musubi/watchdog_url

# 3. Editá el chat_id en /etc/alertmanager/alertmanager.yml (el token va por archivo; el chat_id no es secreto)

# 4. Unit
sudo tee /etc/systemd/system/alertmanager.service >/dev/null <<'UNIT'
[Unit]
Description=Alertmanager (Musubi)
After=network-online.target
Wants=network-online.target

[Service]
User=alertmanager
Group=alertmanager
ExecStart=/usr/local/bin/alertmanager \
  --config.file=/etc/alertmanager/alertmanager.yml \
  --storage.path=/var/lib/alertmanager \
  --web.listen-address=127.0.0.1:9093
Restart=on-failure
RestartSec=5s
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/var/lib/alertmanager

[Install]
WantedBy=multi-user.target
UNIT
sudo systemctl daemon-reload && sudo systemctl enable --now alertmanager

# 5. Prometheus ya apunta a 127.0.0.1:9093 (bloque `alerting:` de prometheus.yml). Recargalo:
curl -X POST http://127.0.0.1:9099/-/reload
```

**Escucha en loopback a propósito.** Alertmanager tiene una API que puede *silenciar* alertas sin
autenticación ninguna. Exponerlo al tailnet sería darle a cualquiera con acceso a la red la
posibilidad de apagar la vigilancia. Si necesitás su UI desde otra máquina, va por túnel SSH.

**Verificarlo de punta a punta** (no alcanza con que el servicio esté `active`):

```bash
# ¿Prometheus lo ve?
curl -s http://127.0.0.1:9099/api/v1/alertmanagers | grep -o '"activeAlertmanagers".*'
# ¿Llega el mensaje? Esto MANDA una alerta de prueba al canal real:
curl -s -XPOST http://127.0.0.1:9093/api/v2/alerts -H 'Content-Type: application/json' -d \
  '[{"labels":{"alertname":"PruebaDeCanal","severity":"info","device":"prueba"},"annotations":{"summary":"Si leés esto, el canal funciona."}}]'
```

Si el binario no llega al teléfono, el problema está entre Alertmanager y el canal, no en Musubi:
`journalctl -u alertmanager -n 50` lo dice.

## MusubiSiempreViva

**Nunca vas a ver esta alerta disparar como un problema: está SIEMPRE en firing, a propósito.**

Todas las demás reglas comparten un punto ciego. Si Prometheus muere, si Alertmanager muere, si
el canal se cae — ninguna dispara, y el silencio se ve exactamente igual que «todo bien». Esta
regla convierte ese silencio en una señal: Alertmanager la rutea al receptor `watchdog`, que
hace un ping periódico a un servicio externo (Healthchecks.io, Dead Man's Snitch, o un cron
propio en otra máquina).

**Es media alarma hasta que exista el otro lado.** Sin un servicio que espere el ping y grite
cuando falte, esta regla no hace absolutamente nada. Si no la vas a completar, es preferible
saberlo a creer que está cubierto.

**Qué hacer si el watchdog externo avisa que dejó de recibir el ping** — en este orden, porque
va de lo más probable a lo más raro:

1. `systemctl status alertmanager prometheus musubi` en el host del cerebro.
2. `curl -s http://127.0.0.1:9099/-/healthy` y `curl -s http://127.0.0.1:9093/-/healthy`.
3. Conectividad de salida del host (el ping sale a internet, no al tailnet).
4. La URL del watchdog: `sudo cat /etc/musubi/watchdog_url` — un archivo vacío o rotado la rompe.

**Mientras el watchdog esté callado, no confíes en la ausencia de otras alertas.**

## PoliticaQueNoCura

Una política de auto-heal actuó más de 3 veces en 6 horas sobre lo mismo. **No está curando:
está tapando.**

Es el modo de fallo propio del auto-heal, y es peor que no tener política: la alerta original
(disco, memoria) deja de dispararse porque la política baja la métrica justo a tiempo, una y otra
vez, y el problema de fondo queda invisible. Un `journalctl --vacuum` cada hora durante una semana
no es mantenimiento — es un servicio escribiendo log sin control que nadie fue a mirar.

```bash
# Qué corrió, cuándo, y con qué resultado. La acción automática está en la MISMA bitácora
# que las de las personas, con el nombre del principal de la política.
musubi_fleet_log --project <proyecto> --limite 50
```

**Qué hacer:** buscar la causa (el servicio que llena el disco, el proceso que come RAM), no
subir el cooldown. Subir el cooldown apaga el aviso y deja el problema.

## PoliticaSinPermiso

Una política quiso actuar y **no pudo**: quedó inerte. Las dos causas, en orden de frecuencia:

- **`rechazada`** — el principal de la política perdió su concesión `exec` sobre esa máquina, o
  el comando dejó de estar en su `fleet_exec_allow`. Típicamente alguien editó `principals.yaml`
  para otra cosa.
- **`sin_principal`** — el principal ya no existe en `principals.yaml` (se revocó o se renombró).
  Es el comportamiento correcto y deliberado: **revocar a alguien apaga también lo que actuaba en
  su nombre**, sin tener que acordarse de un segundo lugar. Pero hay que enterarse.

```bash
grep -A6 "name: <el-principal-de-la-politica>" .musubi/principals.yaml
```

Verificá que tenga `fleet: { exec: [...] }` alcanzando esa máquina y, si tiene
`fleet_exec_allow`, que el comando de la política figure ahí. Recordá que **la sección de
allowlist, una vez presente, es exhaustiva**: una máquina sin entrada propia y sin `"*"` no
permite nada.

## Enrolar un Tier B: la clave de host va PRIMERO

Un Tier B se maneja por `ssh`, y Musubi **nunca** afloja `StrictHostKeyChecking`. Si la clave de
host no está verificada, todo `exec` y toda shell sobre esa máquina fallan con:

```
la clave de host de "usuario@maquina:2222" no está verificada.
```

**Y no hay atajo por entorno.** OpenSSH resuelve `~` con `getpwuid`, **no con `$HOME`**: correr el
cerebro con otro `HOME` no cambia dónde busca `known_hosts` ni las claves. El archivo que importa es
el del **usuario bajo el que corre el servicio** — si es una unidad systemd con `User=musubi`, es
`~musubi/.ssh/known_hosts`, no el tuyo.

```sh
sudo -u musubi ssh-keyscan -p 2222 maquina        # 1. mirá la huella
# 2. verificala por un canal confiable (consola física, otro camino, el proveedor)
sudo -u musubi sh -c 'ssh-keyscan -p 2222 maquina >> ~/.ssh/known_hosts'
```

El paso 2 no es ceremonia: `ssh-keyscan` pregunta por la red, así que confiar en su respuesta sin
verificar es exactamente el MITM contra el que sirve `StrictHostKeyChecking`.

## Probar el camino SSH sin instalar un servidor

Para verificar `exec` y `musubi shell` de punta a punta sin levantar un servicio en el host — un
`sshd` **sin privilegios**, en loopback, que sólo acepta al usuario que lo corre. Es como se cerró
A28; la receta completa está en `specs/flota-shell-contra-sshd-real/tasks.md`.

```sh
apt-get download openssh-server && dpkg-deb -x openssh-server_*.deb raiz
ssh-keygen -q -t ed25519 -f hostkey -N ''
cp ~/.ssh/id_ed25519.pub authorized_keys
# sshd.conf: Port 2222 · ListenAddress 127.0.0.1 · UsePAM no · StrictModes no · AcceptEnv LINES COLUMNS
raiz/usr/sbin/sshd -f sshd.conf -E sshd.log
```

Nada instalado, nada en systemd, nada fuera de loopback. Acordate de sacar la línea de
`known_hosts` al terminar.


## MusubiPushOTLPFallando

El cerebro está empujando la telemetría de flota por OTLP y el destino la rechaza.

1. Mirá el contador y el último éxito:
   `curl -sH "Authorization: Bearer $MUSUBI_TOKEN" http://127.0.0.1:7717/metrics | grep musubi_push`
2. El journal del cerebro dice el motivo la PRIMERA vez de cada clase de error (no una vez por
   tick, a propósito): `journalctl -u musubi-brain | grep -i otlp | tail -20`
3. Los tres motivos frecuentes, en orden de probabilidad:
   - **404** — al Prometheus de destino le falta `--web.enable-otlp-receiver`.
   - **401 / 403** — el token de `fleet.otlp.auth_token_env` no es el que espera el destino.
   - **connection refused** — el destino no está escuchando en esa dirección.

El empuje NO frena al cerebro: mientras esto pasa, `/metrics` sigue sirviendo igual y el scrape
—si lo hay— no se entera. Por eso la alerta existe: sin ella el síntoma es que los gráficos se
quedan quietos y nadie sabe desde cuándo.

## MusubiPushOTLPMudo

El empuje funcionó y dejó de hacerlo, **sin contar fallos**. Eso descarta el destino: si el
destino rechazara, subiría `musubi_push_failures_total` y la alerta sería la otra. Lo que quedó
del otro lado es el empuje quedándose **sin nada que exportar**, y eso tiene tres causas
distintas con tres arreglos distintos.

**El log dice cuál es** (A50, cerrado 2026-08-29). Antes no lo decía y había que deducirlo:

```
journalctl -u musubi-brain --since "-1h" | grep "empuje OTLP:"
```

| Lo que dice el log | Qué pasó | Arreglo |
|---|---|---|
| `el principal ya no está en principals.yaml` | Le borraron la entrada entera. | Devolvésela; **sí cuenta un fallo**, así que puede llegar también `MusubiPushOTLPFallando`. |
| `ya NO tiene ninguna concesión \`metrics\`` | La entrada está, la sección `fleet:` no. Las capacidades de flota **no se derivan del rol**: ni el admin exporta sin la concesión. | `fleet: {metrics: ["*"]}` y esperá un tick. |
| `no alcanza a NINGUNA máquina` | La concesión existe y apunta a proyectos donde no hay ni una máquina — casi siempre un proyecto renombrado. El log dice a qué apunta. | Corregí el alcance, o comprobá que el barrido vea máquinas. |

Ninguno de los tres cuenta como fallo de entrega salvo el primero: `musubi_push_failures_total`
significa «no llegó a destino», y en los otros dos ni se intentó llegar. `musubi_push_datapoints`
sí baja a **0** en los tres, y ése es el gauge para mirar en el tablero.

Si el log no dice ninguna de las tres, el empujador ni está corriendo: revisá que
`fleet.otlp.endpoint` esté declarado (`journalctl -u musubi-brain | grep "empuje OTLP activo"`).

## MusubiPushOTLPNuncaLlego

Hay fallos y **ni un solo éxito desde que arrancó el cerebro**. No se cayó nada: nunca anduvo.
Casi siempre es una de dos cosas, y las dos se arreglan en un minuto:

1. **Falta el flag en el destino.** Prometheus no acepta OTLP por defecto:
   `podman inspect musubi-prometheus --format '{{range .Config.Cmd}}{{.}} {{end}}' | grep otlp`
   Si no aparece `--web.enable-otlp-receiver`, agregalo al compose y `podman compose up -d`.
   Se comprueba en un solo paso — sin el flag esto da 404, con el flag da 200:
   `curl -s -o /dev/null -w "%{http_code}\n" -X POST http://127.0.0.1:9099/api/v1/otlp/v1/metrics -H "Content-Type: application/json" -d "{}"`
2. **El path está mal.** Tiene que terminar en `/api/v1/otlp/v1/metrics`. Un path corto de más
   también da 404 y se ve exactamente igual que lo anterior.


## ServicioFallandoPorDentro

El proceso está vivo y su trabajo sale mal. **`musubi_fleet_service_up` no puede ver esto**:
systemd ve el proceso corriendo tanto si contesta bien como si contesta cualquier cosa. Para un
bot es la única señal que importa.

1. Qué está fallando, con los nombres del dominio de quien reporta:
   `musubi_fleet_services device=<máquina>` → mirá `rendimiento.desglose`. Los conteos por
   resultado (`ok`, `no_puedo`, `vacio`…) los elige el colector, no Musubi.
2. `rendimiento.ventana_seg` dice sobre cuánto tiempo son esos números. Sin eso, «47 fallidas» no
   se puede leer.
3. Si el desglose **suma menos** que `atendidas`, no es un bug: quien reporta no supo clasificar
   todo, y forzarlo a que cierre lo empujaría a inventar una categoría que no midió.

## ColectorDeRendimientoMudo

Un servicio que reportaba rendimiento dejó de hacerlo. **No es lo mismo que «no tuvo trabajo»**, y
esa diferencia es exactamente por qué el colector empuja un 0 en vez de callarse:

| Lo que se ve | Qué pasó |
|---|---|
| `musubi_fleet_service_handled` **existe y vale 0** | El servicio está vivo y no tuvo trabajo. Normal de madrugada. |
| la serie **desapareció** | El colector se murió. Esta alerta. |

1. `systemctl status <la unidad o el cron del colector>` en la máquina que reporta.
2. Si el colector corre por cron, su log: un scrape que falla **no empuja** a propósito —un `0`
   falso dispararía esta misma alerta por el motivo equivocado— así que el silencio es el síntoma
   correcto y el motivo está en el log.
3. La respuesta de la puerta trae `desconocidos`: si el colector apunta a un nombre que nadie
   declaró, ahí va a estar. Se arregla con `musubi_fleet_service_declare`, no tocando el colector.

## ServicioLento

El p95 pasa los 5 segundos sostenido. Ojo con una cosa: **sobre cero unidades atendidas no hay
percentil**, así que esta serie está ausente en los minutos tranquilos por diseño. Un p95 que
aparece y desaparece no es un bug del colector: es que a veces no hubo nada que medir.


## ServicioCaido

Un servicio que la máquina reporta como NO corriendo, y la máquina está viva (si estuviera caída
avisaría `MaquinaCaida` y esta alerta se inhibe sola).

1. Qué dice el inventario: `musubi_fleet_services device=<máquina>` — mirá `estado`, `detalle` y
   `reinicios`. El `detalle` trae el `Result=` de systemd, que es la mitad del diagnóstico.
2. En la máquina: `musubi_fleet_exec device=<máquina> argv=["systemctl","status","<servicio>"]`
   (o `podman ps -a --filter name=<servicio>` si la clase es `podman`).
3. Si el servicio se declaró A MANO y la máquina nunca lo enumeró, el estado va a decir
   `desconocido`: nadie lo está midiendo. Eso no es una caída, es una fila sin dueño.

## ServicioReiniciandose

Está corriendo AHORA y se reinició más de cinco veces en la última hora. `up` no lo puede
mostrar: en cada instante el servicio está arriba.

1. `musubi_fleet_exec device=<máquina> argv=["journalctl","-u","<servicio>","-n","80"]`
2. Las causas frecuentes, en orden: se queda sin memoria y el kernel lo mata (`Result=oom-kill`),
   una dependencia que todavía no está lista al arrancar, o una configuración que no valida.
3. El contador es del supervisor y se reinicia cuando se reinicia la máquina: un pico después de
   un reboot no significa lo mismo que uno en una máquina con semanas de uptime.

## ServicioSinNoticias

La máquina late pero hace más de 30 minutos que no manda el estado de sus servicios.

El agente reenvía el inventario cuando CAMBIA, más un piso periódico (`fleet.InventarioCada`), así
que 30 minutos de silencio son varios reenvíos perdidos. **No sabemos cómo está ese servicio**, y
no saber no es estar bien.

1. ¿Es la máquina entera o un servicio? Si son todos los de esa máquina, el problema es el
   agente: `musubi_fleet_exec device=<máquina> argv=["systemctl","status","musubi-agente"]`.
2. Si el agente corre, mirá su salida: un enumerador que falla avisa una vez por motivo y sigue
   latiendo — el latido llega y el inventario no.
3. Si es UN servicio y los demás llegan, la máquina dejó de enumerarlo: probablemente lo
   deshabilitaron. Un servicio deshabilitado y detenido deja de reportarse a propósito.

## Enrolar un Tier B que NO da shell (bases gestionadas, appliances)

Para lo que no tiene dónde correr un agente ni por dónde entrar por SSH, pero sí publica un
endpoint en formato de exposición de Prometheus. El transporte se llama `exposicion` y lo elige
**la declaración**, no el tier: si la máquina está en `.musubi/flota-exposicion.yaml`, se raspa;
si no, se sondea por SSH como siempre.

**1 · El secreto va al entorno del cerebro, nunca al archivo.**

```
sudoedit /etc/musubi/musubi.env      # agregar:  SB_METRICS_AUTH=Bearer …
sudo systemctl restart musubi-brain
```

**2 · La declaración.** En `$MUSUBI_HOME/.musubi/flota-exposicion.yaml` (ejemplo completo en
`deploy/ejemplos/flota-exposicion.yaml`):

```yaml
dispositivos:
  supabase-altura:
    url: https://<referencia>.supabase.co/customer/v1/privileged/metrics
    auth_env: SB_METRICS_AUTH
    montaje: /data
```

`auth_env` es el NOMBRE de la variable, no su valor. Una URL con usuario y clave adentro se
**rechaza**: un secreto que ya entró a un archivo versionado no se puede des-filtrar.

**3 · El alta**, con `metrics` y nada más — un endpoint de métricas no ejecuta nada:

```
musubi_fleet_enroll  name=supabase-altura  tier=B  caps=["metrics"]  os=linux
```

**4 · Verificar**: `musubi_fleet_probe device=supabase-altura`. En la fila tiene que decir
`"transporte": "exposicion"` y `"ok": true`.

### Lo que se va a ver, y no es un error

- **`cpu_pct: null` en el primer sondeo.** El porcentaje es una derivada.
- **`cpu_pct: null` siempre, si el intervalo es corto.** Muchos endpoints gestionados **cachean**
  su respuesta: medido contra Supabase, refresca cada **~62 s**. Dos sondeos dentro de esa ventana
  ven el mismo contador y no hay contra qué restar. El `probe_minutes` por defecto son 5 min, así
  que alcanza; si alguien lo baja de 1 minuto, la CPU desaparece y no es un bug.
- **`uptime_seg: 0`.** Ese endpoint no publica `node_boot_time_seconds`. No se completa con el
  reloj del cerebro: los relojes difieren y el número saldría con esa deriva encima.

### Cuando algo falla

| Lo que dice | Dónde mirar |
|---|---|
| `rechazó la credencial (HTTP 401/403)` | La variable de entorno del cerebro, no el token del otro lado |
| `declara auth_env: X y esa variable no está en el entorno` | `/etc/musubi/musubi.env` — y acordate de reiniciar el cerebro |
| `responde pero no publica vitales de host` | La URL apunta a un `/metrics` de aplicación, no del host |
| `el endpoint redirige` | Apuntá a la URL final: no se siguen redirecciones a propósito |
| `aviso_configuracion` en TODAS las filas | El YAML no parsea. Las máquinas se siguen sondeando por su transporte de siempre |

## AlturaEndpointMudo

El scrape del endpoint de métricas de la base de Altura falla hace 5 minutos.

**Lo primero, porque no es obvio:** mientras esto dure, `AlturaPoolerLlenandose`,
`AlturaPoolerCaido` y `AlturaBaseCreciendoRapido` están **mudas**. No fallan — se quedan sin
series con las que dispararse. Esta alerta existe para que ese silencio tenga voz.

1. `curl -s -o /dev/null -w '%{http_code}\n' -H "Authorization: $(cat /etc/prometheus/altura-db.token)" https://<ref>.supabase.co/customer/v1/privileged/metrics`
2. **401/403** → el token venció o se rotó. Está en `/etc/prometheus/altura-db.token` (modo 600) y
   también en `/etc/musubi/musubi.env` para el sondeo de Musubi: **son dos copias y hay que
   cambiar las dos.**
3. **000 / timeout** → red o el proyecto pausado. Miralo en el panel de Supabase.
4. Ojo con confundirlo: que Musubi siga midiendo la máquina no dice nada de esto. Musubi va al
   mismo endpoint por otro camino y con otra credencial.

## AlturaPoolerLlenandose

Los clientes conectados al pooler pasan el 85 % del límite que el propio pooler declara.

**El umbral no está tipeado en ningún lado**: el denominador es
`pgbouncer_config_max_client_connections`, que sale del endpoint. Si Supabase cambia el plan, la
alerta se ajusta sola. Esto reemplaza a la del alerter viejo, que comparaba las conexiones del
lado SERVIDOR contra el límite del lado CLIENTE — dos pools distintos, y por eso nunca sonó.

1. `pgbouncer_used_clients{job="altura-db"}` contra `pgbouncer_config_max_client_connections`.
2. Cuando llegue al 100 %, las conexiones nuevas **se rechazan**: la aplicación ve errores de
   conexión, no lentitud. No hay degradación gradual.
3. Casi siempre es la aplicación no devolviendo conexiones al pool, no la base. Mirá
   `pgbouncer_pools_client_waiting_connections`: si hay clientes esperando, el cuello está en el
   lado servidor (`pool_size`), no en el límite de clientes.

## AlturaPoolerCaido

`pgbouncer_up == 0`: el endpoint contesta y el pooler no.

Es distinto de `AlturaEndpointMudo`, y la diferencia importa: **la base puede estar perfecta y ser
inalcanzable**, porque la aplicación va por el pooler. Un chequeo directo a la base no lo detecta.

1. Panel de Supabase → estado del connection pooler.
2. Si la base responde por el puerto directo (5432) y no por el del pooler (6543), es esto.

## AlturaBaseCreciendoRapido

La base creció más de 20 % en 24 horas.

**No es un umbral de tamaño**, a propósito: el tamaño normal depende de la base y un número
absoluto habría que ajustarlo a mano cada tanto — o sea, caducaría. Lo que se puede afirmar sin
conocer la base es la forma de la curva.

1. Mirá `pg_database_size_bytes{datname="postgres"}` en el gráfico: ¿escalón o pendiente?
2. **Escalón** → una migración, una carga masiva, o un backup restaurado. Suele ser esperado.
3. **Pendiente sostenida** → algo escribe y nadie borra. Tablas de log, de auditoría, o de cola
   sin purga son lo habitual.
4. El disco de esa máquina lo vigila Musubi por separado (`musubi_fleet_device_disk_*` con
   `device="supabase-altura"`). Esta alerta llega **antes**, cuando todavía es una curva.

## MaquinaSinInventario

Una máquina de Tier A late —o sea, el agente corre y llega al cerebro— y no reporta ningún
servicio.

**Tres causas, mismo síntoma.** Se descartan en este orden porque van de la más común a la más
rara:

1. **El agente es viejo.** La enumeración de servicios entró en A42; un binario anterior no la
   tiene. Miralo en el inventario: `musubi_fleet_list` trae `agent_version` por máquina.
   Comparalo con el del cerebro (`musubi version`). Si difieren mucho, es esto — desplegá el
   agente en esa máquina y listo.
2. **Una fuente de inventario está rota.** El agente manda el inventario **completo o no lo
   manda**: si `podman ps` (o `systemctl`, o `Get-Service`) está instalado y falla, aborta el
   lote entero a propósito, porque el cerebro poda por ausencia y media lista da de baja la otra
   mitad. Buscá el aviso en el log del agente de esa máquina:
   `journalctl -u musubi-agente | grep "no se pudieron enumerar"`. Dice cuál fuente y por qué.
3. **El blindaje de la unidad prohíbe la fuente.** Pasó en `musubi-server`: `ProtectHome=read-only`
   impedía que `podman ps` abriera sus locks, y el síntoma era `exit status 1` sin más. Ver A54
   y `deploy/systemd/musubi-agente-contenedores.conf`.

**Lo que NO es:** una máquina caída. Ésa la cubre `MaquinaCaida` y esta regla la excluye a
propósito — una máquina apagada no tiene por qué reportar inventario.

## MaquinaCaida en un equipo Windows: mirá esto PRIMERO

Antes de ir a ver si la máquina está encendida.

El agente de Windows se instala como **tarea programada con disparador «al iniciar sesión»**
(`agente-windows.ps1`, `-AtLogOn`). Eso significa que **el agente vive mientras haya alguien
logueado**. Un equipo que se reinició de madrugada y quedó en la pantalla de bloqueo figura caído
en la flota y está perfectamente vivo.

Costó dos días leerlo mal: `gio` figuraba apagada mientras respondía al ping por el tailnet en
145 ms.

**Cómo distinguirlo en treinta segundos:**

```
ping <ip-tailnet-de-la-maquina>
```

- **Responde** → la máquina está viva y el agente no corre. Iniciá sesión (o usá `-AlArranque`,
  abajo). No hay nada que prender.
- **No responde** → puede estar apagada, fuera del tailnet, o con ICMP bloqueado. Ojo: algunas
  Windows no contestan ping y sí latean, porque el latido es saliente y no necesita ICMP entrante.

**Para que sobreviva a un reinicio**, reinstalá con:

```powershell
.\agente-windows.ps1 -BrainUrl "http://100.79.126.62:7717" -DeviceToken "<el del enroll>" -AlArranque
```

Exige administrador y registra la tarea **al arranque, como SYSTEM**. Decidilo a conciencia:
`musubi_fleet_exec` sobre esa máquina pasa a ejecutarse con privilegios de SYSTEM. Es opt-in por
eso, no por comodidad.

## CadenaDeAlertasFallando

Alertmanager no puede entregar por uno de sus canales. **Que estés leyendo esto significa que otro
canal sí anda** — si estuvieran todos rotos, este aviso tampoco habría salido. Eso lo cubre el
dead-man's switch externo, no esta regla.

```
podman logs --tail 50 musubi-alertmanager 2>&1 | grep -i "notify for alerts failed"
```

El error dice el receptor y la causa. Los tres que se ven:

| Error | Qué pasó |
|---|---|
| `read url_file: open …: no such file or directory` | El receptor apunta a un archivo de secreto que no existe. Es el caso del watchdog: la configuración estaba puesta y el archivo nunca se creó |
| `unexpected status code 4xx` | La credencial venció, o el destino cambió |
| `context deadline exceeded` | El destino no contesta. Si es un servicio externo, mirá su estado |

**El caso del dead-man's switch**, que es el que motivó esta regla: `/etc/musubi/watchdog_url` no
existía y el latido `MusubiSiempreViva` fallaba cada 5 minutos desde hacía 32 horas. Se arma
creando el archivo con la URL de ping de un servicio externo (healthchecks.io, cronitor, o el que
uses), con dueño y modo restringidos como los otros secretos:

```
printf '%s' 'https://<tu-servicio>/ping/<tu-uuid>' | sudo tee /home/musubi/musubi-prometheus/secretos/watchdog_url >/dev/null
sudo chmod 600 /home/musubi/musubi-prometheus/secretos/watchdog_url
podman restart musubi-alertmanager
```

Comprobalo: a los 5 minutos, `podman logs --tail 20 musubi-alertmanager | grep -i watchdog` no
tiene que decir nada, y el servicio externo tiene que mostrar el ping.

## AlertmanagerCaido

El último eslabón no contesta: Prometheus sigue evaluando y las alertas quedan FIRING, pero **no
sale ninguna a Telegram ni al CRM**.

**Leé esto sabiendo cómo llegaste acá.** Si te enteraste por un mensaje, no fue por esta alerta —
esta alerta no se puede entregar a sí misma. Te enteraste por el panel, por
`deploy/verificar-despliegue.sh`, o por el watchdog externo. Esta regla existe para que el estado
sea *visible* sin salir del sistema, no para avisarte: el aviso de verdad es el watchdog externo
(B13 en `specs/control-de-flota/ABIERTO.md`, despriorizado).

```
podman ps -a --filter name=musubi-alertmanager
podman logs --tail 50 musubi-alertmanager
```

| Lo que ves | Qué pasó |
|---|---|
| El contenedor no está en `ps` | Se cayó o nunca arrancó. `podman start musubi-alertmanager` y mirá el log |
| `Reason: Error` al arrancar | La configuración no parsea y **no llegó a levantar**. Ver `ConfiguracionSinRecargar` abajo, mismo arreglo |
| Corre pero `up == 0` | Contesta al `ps` y no al scrape: puerto, red del contenedor, o está trabado |

Mientras dure, el único canal es mirar Prometheus. Todo lo que se dispare en ese lapso **no se
reenvía al recuperarse**: Alertmanager entrega lo que está firing cuando vuelve, no el historial.

## ConfiguracionSinRecargar

Alguien escribió una configuración inválida y la recarga la rechazó. **El proceso no se cayó**:
sigue corriendo con la configuración VIEJA. Ese es el punto — el target está `up`, las alertas
evalúan, todo parece normal, y sin embargo lo que corre no es lo que dice el repo.

Primero, cuál de los dos:

```
curl -s http://127.0.0.1:9099/api/v1/query?query=prometheus_config_last_reload_successful
curl -s http://127.0.0.1:9099/api/v1/query?query=alertmanager_config_last_reload_successful
```

El que devuelve `0` es el que rechazó. El log dice la línea exacta:

```
podman logs --tail 30 musubi-alertmanager 2>&1 | grep -i "error\|invalid"
podman logs --tail 30 musubi-prometheus   2>&1 | grep -i "error\|invalid"
```

Corregí el archivo y recargá. **Validá ANTES de recargar**, que es lo que evita volver acá:

```
podman run --rm -v $PWD/deploy/prometheus:/c prom/alertmanager amtool check-config /c/alertmanager.yml
promtool check rules deploy/musubi-alerts*.yml
```

Ojo con la trampa de esta alerta: **corregir el archivo no la apaga**. La métrica sigue en `0`
hasta que la recarga *tenga éxito*, así que después de editar hay que recargar de verdad
(`podman kill -s HUP` o reiniciar el contenedor) y recién ahí se apaga.

## MaquinaQueNoAlcanzaSuDestino

Esa máquina **no llega** a alguno de los puertos que le declararon en `MUSUBI_ALCANCE`. La sonda
la toma el agente en cada latido, así que lo que la alerta afirma es «desde ESA máquina no se
llega» — no «el destino está caído».

**La distinción es el punto entero de esta alerta.** Nació de un caso real: el relay de RustDesk se
veía sano por las tres vías que existían —los dos contenedores arriba y sus tres puertos
contestando— y ningún cliente lograba registrarse. El chequeo sondeaba desde el propio servidor,
que es el único lugar desde el que siempre anda.

Primero, cuál destino falla:

```bash
./musubi-tool.sh musubi_fleet_list '{}' | grep -A3 no_alcanza
```

Después, en orden de frecuencia:

- **La red de esa máquina.** Un VPN que captura todo el tráfico es la causa más común y la más
  difícil de ver desde afuera: la máquina sigue latiendo contra el cerebro y todo parece normal.
  Si el VPN tiene *split tunneling*, excluir la aplicación o el rango del tailnet
  (`100.64.0.0/10`) resuelve sin apagarlo.
- **El destino caído.** Se distingue solo: si TODAS las máquinas dejan de alcanzarlo a la vez, es
  el destino; si es una sola, es esa máquina.
- **Un destino mal escrito.** El formato es `host:puerto`. Los inválidos se descartan al arrancar
  el agente y avisa una vez por su salida estándar, así que un typo se ve como una serie que
  **nunca aparece**, no como un `0`.

Una máquina sin destinos configurados **no emite la serie** y no puede disparar esta alerta.
Ausente no es falso: significa que nadie le pidió que mirara.

## AgenteDesactualizado

El agente de esa máquina corre un **release distinto** del que corre el cerebro. La serie compara
el núcleo semver (`0.130.0`), no el commit: dos binarios del mismo release construidos de commits
distintos son lo normal y **no** disparan esto.

**Por qué importa, con el caso que lo abrió.** El 2026-09-01 el cerebro corría `0.130.0` y los dos
Windows `v0.106.0`, veinticuatro versiones atrás. Se descubrió de casualidad, mirando otra cosa. El
costo fue concreto: A67 se había desplegado el día anterior y **no podía correr en las dos máquinas
para las que se escribió**, porque su binario no tenía la capacidad. Un agente atrasado se veía
idéntico a uno al día.

Qué versión corre cada una:

```bash
./musubi-tool.sh musubi_fleet_list '{}' | grep -E 'name|agent_version'
```

La versión **no viaja como etiqueta de Prometheus** a propósito: la serie se re-etiquetaría sola en
cada actualización y las viejas quedarían huérfanas. Por eso la métrica es un booleano y el detalle
sale de la tool.

Para actualizar:

- **Linux (el propio servidor).** El cerebro y el agente comparten ejecutable: `deploy/redesplegar-cerebro.sh`.
- **Windows.** Se cruza el binario nuevo a la máquina y se corre `cambiar-agente.cmd`, que lo
  reemplaza con prueba de latido y vuelta atrás. Ojo con el zombi: si un agente viejo quedó vivo
  desde `musubi.exe.viejo`, gana la carrera del latido y la máquina sigue figurando en la versión
  anterior aunque el log del cambio diga «exitoso».

**Lo que esta alerta no puede ver:** una capacidad que entró al cerebro sin tocar el archivo
`VERSION`. La comparación mide lo que VERSION declara, y VERSION lo bumpea una persona.

## ScrapeQueElRepoDeclaraYNoExiste

Prometheus **no tiene configurado** un job que el repo declara. No es un target caído: `up == 0`
significa «no contesta», y acá el job ni siquiera existe, así que **ninguna regla que use sus
métricas puede dispararse**. Se ve en verde.

Es el caso exacto que abrió A73. El job `alertmanager` no estaba desplegado, así que
`alertmanager_notifications_failed_total` no existía y `CadenaDeAlertasFallando` —la alerta que
vigila que las alertas se entreguen— llevaba días cargada sin poder sonar.

```bash
# qué jobs tiene de verdad
curl -s 'http://127.0.0.1:9099/api/v1/targets?state=any' \
  | python3 -c 'import sys,json;print(sorted({t["labels"]["job"] for t in json.load(sys.stdin)["data"]["activeTargets"]}))'
```

El arreglo es copiar `deploy/prometheus/prometheus.yml` al servidor y recargar. **Copiálo con
`cat >` o `install`, nunca con `sed -i`**: el archivo está bind-monteado en el contenedor y `sed -i`
reemplaza el inodo — el contenedor se queda leyendo el archivo anterior, que ya no tiene nombre, y
la recarga contesta `200` sobre el archivo equivocado.

```bash
curl -X POST http://127.0.0.1:9099/-/reload
MUSUBI_SSH=musubi-server ./deploy/verificar-despliegue.sh
```

## ReglasDeFlotaSinDesplegar

## ReglasDelCerebroSinDesplegar

Las dos son la misma cosa mirada desde cada lado: **lo que Prometheus tiene cargado no es lo que
declara el repo**. Cada archivo de reglas vigila el conteo del OTRO, cruzado a propósito — un
archivo que declara su propio conteo se despliega junto con el conteo, las dos mitades se mueven a
la vez y la comprobación no falla nunca.

Primero, qué falta y en qué dirección:

```bash
MUSUBI_SSH=musubi-server ./deploy/verificar-despliegue.sh
```

Ese informe distingue tres cosas que se ven parecidas y se arreglan distinto: un archivo **sin
desplegar**, uno **desplegado a medias**, y reglas **cargadas que el repo ya no tiene** (quedaron
de un despliegue anterior). También deja sin denunciar los archivos que están parkeados a
propósito, que lo declaran en su propia línea `# despliegue:`.

Después, copiar y recargar:

```bash
# desde la máquina que tiene el repo
scp deploy/musubi-alerts.yml deploy/musubi-alerts-flota.yml \
    musubi-server:/tmp/
ssh musubi-server 'for f in musubi-alerts.yml musubi-alerts-flota.yml; do
  cat "/tmp/$f" > "$HOME/musubi-prometheus/rules/$f"; done
  curl -sS -X POST http://127.0.0.1:9099/-/reload'
MUSUBI_SSH=musubi-server ./deploy/verificar-despliegue.sh
```

**`cat >` y no `cp`**, por lo mismo que arriba: conserva el inodo del bind-mount.

**Si la alerta suena y el despliegue está bien**, lo que se pudrió es el número: alguien agregó una
regla y no actualizó el conteo del archivo que la custodia. Eso lo detecta la suite
(`TestCadaArchivoDeReglasCustodiaElConteoDelOtro`) antes de llegar a producción, así que si suena
en producción es que se desplegó sin correr las pruebas.

## MaquinaSeReiniciaSola

La máquina se reinició **dos o más veces en 24 horas** sin que nadie lo pidiera.

Esta regla existe porque el caso real pasó desapercibido. El 2026-09-03, `davantis-1` llevaba
**trece apagones sucios en diez días** y la flota no había dicho nada: `musubi_fleet_device_uptime_seconds`
se exportaba desde siempre y ninguna regla lo miraba. `MaquinaCaida` sí disparó cada vez, y ahí
está la trampa —**se resolvía sola a los pocos minutos**, cuando la máquina volvía—. Trece avisos
que aparecen y se apagan solos se leen como ruido de red. El patrón sólo existe si alguien lo
cuenta.

**Lo primero: distinguir un reinicio LIMPIO de un corte.** No es lo mismo y se pregunta distinto
según el sistema.

En Windows, el evento 41 es el que importa, y **su `BugcheckCode` es el que decide**:

```powershell
Get-WinEvent -FilterHashtable @{LogName='System'; Id=41; StartTime=(Get-Date).AddDays(-10)} |
  ForEach-Object { "$($_.TimeCreated)  BugcheckCode=$($_.Properties[0].Value)" }
```

- **`BugcheckCode` distinto de 0** → hubo pantalla azul. Windows se cayó y dejó su minidump en
  `C:\Windows\Minidump`. La causa está adentro: driver, kernel, memoria. Ese volcado se lee.
- **`BugcheckCode=0`** → **no hubo pantalla azul**. Windows no llegó a caerse: lo cortaron. La
  causa está **afuera** del sistema operativo —fuente, corriente de pared, térmica, un cuelgue
  duro que no alcanzó ni a hacer bugcheck— y no hay ningún log de Windows que lo vaya a decir,
  porque cuando se va la corriente no se escribe nada.

En Linux: `journalctl --list-boots` y `last -x reboot shutdown`. Un arranque sin un `shutdown`
que lo preceda es un corte.

**Lo segundo: descartar hardware con lo que sí queda registrado.**

```powershell
Get-WinEvent -FilterHashtable @{LogName='System'; ProviderName='Microsoft-Windows-WHEA-Logger'; StartTime=(Get-Date).AddDays(-10)}
```

WHEA registra los errores de hardware que el procesador SÍ alcanza a reportar. **Que esté vacío no
absuelve a la máquina**: un corte de corriente no le da tiempo a escribir nada. Sirve en positivo
(si hay algo, ahí está la causa), no en negativo.

**Lo tercero, si el sistema operativo no tiene la respuesta**, es lo de afuera, en este orden por
frecuencia: la fuente bajo carga, una regleta o un cable flojo, la temperatura. La térmica es la
única de las tres que la flota podría ver sola —`musubi_fleet_device_temperature_celsius`— y hoy
**la reporta una sola máquina**, así que en el resto hay que ir a mirarla a mano (HWiNFO, `sensors`).

**Lo que NO es:** una máquina caída ahora mismo. Ésa es `MaquinaCaida`, y esta regla la excluye a
propósito para no dar dos avisos por el mismo evento. La ventana de 24 h se guarda la cuenta: si
la máquina está abajo, el aviso llega cuando vuelve, que es cuando se puede hacer algo.
