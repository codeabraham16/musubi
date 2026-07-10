# Modelo de amenazas — cerebro central de Musubi

Alcance: el nodo central (`musubi serve`) que agrega la memoria **compartida** de varios
proyectos sobre una malla privada (Tailscale/WireGuard). El daemon local (`musubi daemon`,
stdio) queda fuera: corre en la máquina del dev, sin superficie de red, bajo confianza local.

## Borde de confianza

- **Confiable:** el transporte de la malla (WireGuard cifra e autentica en tránsito entre
  peers) y el disco del servidor (acceso root/OS del host).
- **NO confiable:** cualquier cliente que presente un token, incluso dentro de la malla; un
  dispositivo de la malla comprometido; un token reusado fuera de la malla.

WireGuard da **confidencialidad e integridad en tránsito dentro de la malla**. NO da
autorización (quién puede hacer qué), ni aislamiento entre proyectos, ni protección si un
token se filtra o un peer se compromete. Esas garantías las provee Musubi por encima.

## Activos

1. La `memory.db` central (memoria compartida de todos los proyectos) — confidencialidad e
   integridad.
2. Los tokens de los principals (credenciales de acceso).
3. La disponibilidad del servicio (es el punto de convergencia de la memoria del equipo).

## Amenazas y mitigaciones

| Amenaza | Mitigación (dónde) |
|---|---|
| Fuga de un token compartido = acceso total | **Identidad por-principal** (16.1c): un token por miembro, revocable individualmente; el archivo guarda el SHA-256, no el token |
| Un miembro lee la memoria de otro proyecto | **Aislamiento por proyecto** derivado de la credencial: `recall` (16.1c-3) y las lecturas de **contenido** —`search_keyword`, `search_semantic`, `memory_expand`— se acotan al `project_id` del principal (T17.1a); la **escritura** también se atribuye por credencial, no por lo que declare el cliente (T17.1b-1). Solo `admin` ve federado. *Pendiente (T17.1b-2): las superficies de metadata/grafo (`recall_facts`, `entity_context`, `recall_code`, `insights`, `conflicts`) todavía consultan federado.* |
| Escalamiento: un `reader` muta memoria | **Authz por rol** (16.1c): `reader` solo tools de lectura; deniega con `codeUnauthorized` |
| Un secreto crudo entra al pozo compartido | **Redacción forzada server-side** en TODO ingest al central —`save_observation` (content + topic_key, **antes** del embedding), `save_fact` y `save_code`— cuando el bind es no-loopback (T17.2), fail-closed, sin importar el `scope` declarado. **Es BEST-EFFORT heurístico** (formas conocidas de secreto + entropía): **reduce, no garantiza** la fuga —un secreto corto o de baja entropía puede escapar—; no confiar en la redacción como única barrera. |
| Fuerza bruta del bearer | **Lockout** (16.1e): N fallos por IP ⇒ bloqueo temporal; comparación en tiempo constante (no filtra por timing) |
| Token en texto plano en tránsito | Fail-closed: bind no-loopback **exige** token; sin TLS, hay que optar explícitamente por `allow_insecure_token` (válido solo si WireGuard/un proxy cubren el cifrado) |
| DNS-rebinding (modo loopback) | Chequeo de Host loopback + Origin local |
| DoS por body gigante / slow-loris | `MaxBytesReader` 4 MiB + timeouts de lectura/escritura |
| Movimiento lateral desde cualquier peer de la malla | **ACLs de Tailscale** (ver abajo): restringir el puerto del brain a principals concretos, no confiar solo en el rango CGNAT |
| Pérdida del disco central | **DR** (16.0b): backup consistente off-host + restore probado |

## ACLs de Tailscale (defensa en profundidad)

La regla de firewall del host abre el puerto a todo el rango de la malla (`100.64.0.0/10`).
Por defecto la **policy de Tailscale es allow-all**, así que cualquier dispositivo del tailnet
alcanza el brain. Restringilo en la [policy del tailnet](https://tailscale.com/kb/1018/acls),
por ejemplo permitiendo el puerto solo desde un tag de dispositivos autorizados:

```jsonc
{
  "tagOwners": { "tag:musubi-client": ["autogroup:admin"] },
  "acls": [
    // Solo los dispositivos etiquetados como musubi-client llegan al brain:7717.
    { "action": "accept", "src": ["tag:musubi-client"], "dst": ["tag:musubi-brain:7717"] }
    // (sin una regla que lo permita, el resto del tailnet NO alcanza el puerto)
  ]
}
```

Estrechar solo la regla de firewall del host no basta: sin ACLs, la policy default de
Tailscale ya deja pasar a todos.

## Riesgos residuales (conocidos)

- **Host comprometido:** root en el servidor lee la `memory.db` y el registro de hashes (no
  los tokens crudos, pero sí puede reemplazarlos). Fuera de alcance de la app.
- **Reuso de token fuera de la malla:** si un token se usa por fuera de WireGuard sin TLS,
  viaja en claro. Mitigar con TLS o no exponer el servicio fuera del tailnet.
- **Confianza en el `project_id` declarado por el sync client:** el ingest preserva el
  `project_id` de origen que envía el cliente (16.1a). Dentro de la malla con tokens
  por-principal es aceptable; un endurecimiento futuro es derivar también el `project_id` de
  ESCRITURA de la credencial, no solo el de lectura.
