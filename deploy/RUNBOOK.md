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
destino rechazara, subiría `musubi_push_failures_total` y la alerta sería la otra.

La causa conocida es que el principal del empuje **perdió su concesión `metrics`** en una recarga
en caliente de `principals.yaml` (que se relee cada 10 s). El servidor exige esa concesión al
arrancar, pero no vuelve a mirarla, así que el empuje se queda sin máquinas que exportar y manda
cero puntos sin quejarse (A50).

1. `grep -A6 "name: <el principal de fleet.otlp.principal>" .musubi/principals.yaml`
2. Tiene que tener `fleet: metrics: [...]` con al menos una máquina. Si alguien se la sacó,
   devolvésela y esperá un tick.

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
