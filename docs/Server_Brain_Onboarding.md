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

## Notas de operación

- **Backups**: la SQLite del servidor es la fuente única; respaldala (el `musubi export`
  produce un snapshot JSON, y `musubi_doctor` valida integridad).
- **Un solo escritor lógico**: el daemon serializa las tools que mutan; no corras dos
  `serve` sobre la misma DB.
- **Local + remoto conviven**: podés tener un `musubi` remoto (cerebro) y un stdio local
  con nombres distintos en el mismo `.mcp.json` si querés memoria de proyecto local además
  del cerebro central.
