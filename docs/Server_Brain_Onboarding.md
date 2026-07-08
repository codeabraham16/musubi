# Musubi como cerebro central self-hosted (Track S)

Runbook para convertir Musubi en un **cerebro compartido** —memoria + orquestación— que
vive en tu servidor casero, sobre una malla VPN privada, sin nube de terceros. Es el
diferenciador de posicionamiento del Track 12: engram tiene cloud comercial; Musubi te da
lo mismo **self-hosted y privado**. Ver la memoria `roadmap/home-server-musubi` y
`roadmap/track-12-pilares`.

> **Estado del código.** Todo lo necesario ya está en el binario; esto es *configuración*,
> no desarrollo pendiente. Se ejecuta cuando el hardware esté activo.

## Arquitectura

```
   Laptop ─┐                          ┌─ musubi serve (HTTP + bearer + TLS)
   Desktop ─┼─ malla VPN privada ─────┤   ↳ una sola SQLite = memoria + workflow_runs + pizarra
   (agentes)┘   (WireGuard/Tailscale) └─ Servidor casero (el "cerebro")
```

Un único `musubi serve` en el servidor expone **todas** las tools MCP (memoria, grafo,
SDD, workflows, pizarra) sobre HTTP. Cada máquina apunta su cliente MCP a esa URL en vez
de a un binario local: así **comparten la misma memoria y los mismos runs** (S2 + S3) sin
motor nuevo — es la propiedad de que el daemon remoto ya sirve el catálogo completo.

## 1. En el servidor — `musubi serve` (S1, ya implementado)

`.musubi/config.yaml`:

```yaml
service:
  enabled: true
  addr: "0.0.0.0:7717"            # escuchá en la interfaz de la VPN (no loopback)
  auth_token_env: "MUSUBI_TOKEN"  # el bearer token vive en el entorno, nunca en el YAML
  tls_cert_file: "/etc/musubi/cert.pem"
  tls_key_file: "/etc/musubi/key.pem"
  # allow_insecure_token: true    # SOLO si un proxy/VPN termina TLS por delante
```

```bash
export MUSUBI_TOKEN="$(openssl rand -hex 32)"   # generá y guardá el token
musubi serve
```

Gating de seguridad (fail-closed, ya en `internal/mcp/http.go`):
- Bind **no-loopback** exige bearer token *y* TLS (o `allow_insecure_token` explícito).
- `auth_token_env` nombrado pero vacío → se niega a arrancar (no degrada a “sin auth”).
- `/healthz` y `/readyz` para sondas; `/metrics` detrás del token.

## 2. La malla VPN (S4)

Poné el servidor y cada cliente en una red privada (WireGuard o Tailscale). El daemon
**no** se expone a Internet: solo escucha en la interfaz de la VPN. Con Tailscale, `addr`
puede ligar a la IP del tailnet y TLS puede delegarse a `tailscale serve`
(`allow_insecure_token: true` en ese caso, porque el tailnet cifra el transporte).

## 3. En cada máquina cliente — apuntar al cerebro (S2 + S3)

Generá/mergeá el `.mcp.json` con una entrada **remota** (helper `bootstrap.RemoteEntry`):

```json
{
  "mcpServers": {
    "musubi": {
      "type": "http",
      "url": "https://box.tailnet.ts.net:7717/mcp",
      "headers": { "Authorization": "Bearer ${MUSUBI_TOKEN}" }
    }
  }
}
```

El token va por **referencia a variable de entorno** (`${MUSUBI_TOKEN}`): el secreto nunca
toca el archivo, consistente con el principio de Musubi. Exportá `MUSUBI_TOKEN` en el
entorno del cliente. Desde ese momento, el agente de esa máquina lee y escribe en el
cerebro central: la memoria y los flujos SDD/pizarra son compartidos.

## 4. Verificación

```bash
curl -fsS https://box.tailnet.ts.net:7717/readyz                 # 200 ok
curl -fsS -H "Authorization: Bearer $MUSUBI_TOKEN" \
     -X POST https://box.tailnet.ts.net:7717/mcp \
     -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | head   # catálogo completo
```

Luego, desde dos máquinas: guardá una observación en una y recuperala en la otra con
`musubi_recall` — si aparece, la memoria compartida funciona. Repetí con `musubi_sdd
action=start` en una y `action=status` en la otra para confirmar la orquestación compartida.

## Backup y recuperación (DR)

El cerebro central es el **único punto donde converge la memoria compartida** de todos los
proyectos: perder su `memory.db` sin backup off-host es perder todo. `install-musubi-brain.sh`
instala un **backup diario off-host** (`musubi-backup.timer`).

**Cómo funciona.** El timer corre `musubi backup` (que usa `VACUUM INTO` → snapshot
*transaccionalmente consistente*, sin lockear el daemon ni depender del CLI `sqlite3`) y
shipa el snapshot a `BACKUP_REMOTE`. Config en `/etc/musubi/musubi.env`:

```
BACKUP_REMOTE=user@host:/srv/backups/musubi   # off-host: rsync, rclone-remote:path, o dir local
BACKUP_METHOD=rsync                            # rsync | rclone | cp
BACKUP_RETENTION_DAYS=14                        # purga snapshots LOCALES > N días
```

> Dejá `BACKUP_REMOTE` **configurado**: vacío significa que el snapshot queda solo en el
> mismo disco y no protege contra la pérdida del host.

**Operar / verificar.**

```bash
systemctl list-timers musubi-backup.timer     # próxima corrida
sudo systemctl start musubi-backup.service     # backup manual ya
journalctl -u musubi-backup.service -n 30      # log del último backup
```

**Restore (procedimiento probado).** Con el servicio detenido, reemplazá la base por el
snapshot y validá la integridad ANTES de rearrancar:

```bash
sudo systemctl stop musubi-brain.service
SNAP=/ruta/al/memory.db.YYYYMMDD-HHMMSS         # traé el snapshot desde BACKUP_REMOTE
# Validá el snapshot antes de confiar en él:
sqlite3 "$SNAP" 'PRAGMA integrity_check;'       # debe decir: ok   (o usá 'musubi doctor')
# Guardá la base actual por las dudas y restaurá (el VACUUM INTO no trae -wal/-shm):
cd /home/musubi/musubi-brain/.musubi
sudo mv memory.db memory.db.corrupta 2>/dev/null || true
sudo rm -f memory.db-wal memory.db-shm
sudo -u musubi cp "$SNAP" memory.db
sudo systemctl start musubi-brain.service
curl -fsS http://127.0.0.1:7717/readyz          # verificá que levantó
```

- `musubi export` (snapshot JSON) y `musubi doctor` (valida integridad / repara) siguen
  disponibles como herramientas complementarias de diagnóstico.

## Identidad por-principal (opcional)

Por defecto el central usa **un único bearer** (`MUSUBI_TOKEN`) para todo el equipo: una fuga
= compromiso total, sin revocación selectiva. Para un equipo, activá el **registro de
principals**: cada miembro tiene su propio token, con proyecto y rol, revocable borrando una
línea. El archivo guarda el **SHA-256** del token, nunca el token crudo.

Gestionalo con **`musubi token`** (genera el token, guarda solo su SHA-256 y mantiene el
`.musubi/principals.yaml`):

```bash
# Alta: imprime el token UNA vez — entregáselo al miembro por un canal seguro.
musubi token new --name alice --project crm-musubi --role writer
musubi token list                 # nombre / rol / proyecto (nunca el token ni el hash)
musubi token revoke --name alice  # baja; reiniciá musubi-brain.service para aplicar
```

El archivo resultante (600, fuera de control de versiones):

```yaml
principals:
  - name: alice
    token_sha256: "<sha256-del-token>"   # el servidor solo ve el hash
    project_id: crm-musubi               # aísla su recall a este proyecto (con 16.1c-3)
    role: writer                         # reader (solo lectura) | writer (lee+escribe) | admin
```

- **Roles:** `reader` solo puede tools de lectura; `writer` lee y escribe; `admin` todo.
- **Backward-compat:** sin archivo de registro, sigue el modo de un único token. El
  `MUSUBI_TOKEN` legacy sigue válido (como `admin`) aun con registro presente.

**Redacción forzada (automática en el central):** como el central escucha en un bind
no-loopback (es infra compartida), **redacta secretos en TODO ingest** independientemente del
`scope` que declare el cliente — un secreto crudo no puede entrar al pozo compartido ni
mandando `scope=local`. Es fail-closed (no se desactiva en no-loopback).

## Notas de operación

- **Un solo escritor lógico**: el daemon serializa las tools que mutan; no corras dos
  `serve` sobre la misma DB.
- **Local + remoto conviven**: podés tener un `musubi` remoto (cerebro) y un stdio local
  con nombres distintos en el mismo `.mcp.json` si querés memoria de proyecto local además
  del cerebro central.
