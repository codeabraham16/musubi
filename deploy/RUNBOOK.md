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
