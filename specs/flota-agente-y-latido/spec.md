# Spec — S2 · El agente y el latido

Segundo slice del track **Control de flota**. Fase 0. Depende de S1
(`specs/flota-registro-dispositivos/`), que creó la entidad pero no le dio ningún productor.

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

```go
// internal/memory — el rol nuevo del seam (interfaces chicas, compuestas).
type DeviceStore interface { AltaDevice, DevicePorToken, DevicePorNombre,
                             ListarDevices, LatirDevice, RevocarDevice }

// internal/mcp — administración de la flota, por principal (persona).
musubi_fleet_enroll   // ADMIN. Da de alta y devuelve el token UNA vez.
musubi_fleet_list     // Lista la flota con `online` DERIVADO.
musubi_fleet_revoke   // ADMIN. Kill-switch.

// internal/mcp — la puerta del dispositivo, separada de /mcp.
POST /fleet/heartbeat   // auth: token de DISPOSITIVO. Estampa last_seen.

// cmd/musubi — el productor real.
musubi agent            // late contra el cerebro cada N segundos.
```

---

## La decisión de este slice: **son dos puertas, no una**

Un dispositivo necesita autenticarse para latir. La vía cómoda sería darle un principal en
`principals.yaml` y que use `/mcp` como todo el mundo. **Eso sería el error del track.**

Un agente corre en cada máquina de la flota — la de un cliente, un portátil que viaja, un
Windows con antivirus ajeno. Es la superficie **más expuesta** del sistema. Si su credencial abre
`/mcp`, entonces comprometer *cualquier* máquina de la flota entrega `musubi_recall` sobre la
memoria de la empresa. El plano de monitoreo se convierte en el plano de exfiltración.

Por eso las credenciales viven en **dos almacenes distintos**: las personas en
`principals.yaml`, los dispositivos en la tabla `devices`. La separación es **estructural**, no
una promesa: `/mcp` resuelve contra `PrincipalRegistry` y no tiene forma de llegar a `devices`.
Las pruebas B1 y B2 existen porque «estructural hoy» y «estructural dentro de un año» son cosas
distintas, y unificar los dos lookups «para simplificar» es exactamente el refactor que alguien
va a proponer.

---

## H1 · Las dos puertas no se cruzan

### B1 — Un token de DISPOSITIVO no autentica en `/mcp`

Se enrola un device, se presenta su token a `/mcp` y la respuesta es **401**. Comprometer una
máquina de la flota no puede dar lectura sobre la memoria del equipo.

### B2 — Un token de PRINCIPAL no autentica en `/fleet/heartbeat`

La separación va en los dos sentidos. Un token de persona presentado al endpoint de flota es
401: si valiera, `last_seen` sería escribible por cualquiera con una credencial de lectura, y el
panel mostraría vivas máquinas apagadas.

### B3 — El 401 no es un oráculo

Token desconocido, token revocado y token con formato raro devuelven **la misma** respuesta. Un
mensaje que distinga «no existe» de «revocado» le dice a quien prueba credenciales cuáles
existieron.

---

## H2 · El latido dice la verdad y no rompe nada

### B4 — El latido estampa `last_seen` y nada más

El cuerpo del POST no puede cambiar quién es el dispositivo: la identidad sale del token (A1 de
S1). Ni `id`, ni `name`, ni `project` viajan en el cuerpo — no hay dónde ponerlos.

### B5 — El dispositivo revocado se entera

`RevocarDevice` corta en el acto y el latido siguiente devuelve 401 con un cuerpo que el agente
sabe leer: deja de latir y lo dice por consola. Un kill-switch que el agente no entiende obliga a
ir a apagarlo a mano — que es justo lo que no se puede hacer con una máquina remota.

### B6 — El lockout anti fuerza-bruta cubre la puerta nueva

Mismo `authLimiter` que `/mcp` (5 fallos por IP ⇒ 60 s). Una puerta nueva sin lockout es un
oráculo de fuerza bruta con la tabla entera detrás.

### B7 — El agente sobrevive a que el cerebro no esté

Cerebro caído, DNS que no resuelve, 500: el agente **reintenta** y sigue vivo. Un agente que se
muere con el primer error de red hay que ir a levantarlo a mano a cada máquina.

---

## H3 · La administración respeta la tenencia que ya existe

### B8 — Enrolar y revocar son ADMIN

Mismo gate que `musubi_token_new` (`principalFrom(ctx).isAdmin()`), y por la misma razón: mintear
la credencial de una máquina es la joya de la corona del plano de control.

### B9 — El proyecto sale de la CREDENCIAL, no del cliente

`writeOriginFor` decide a qué proyecto se atribuye el alta, igual que una escritura de memoria.
Un principal acotado (`write=own`) no puede enrolar una máquina en el tenant de otro, aunque lo
declare en los argumentos.

### B10 — Listar no cruza tenants

`musubi_fleet_list` sólo respeta el argumento `project` para un principal `read=all` (sala de
mando/cabina). Un acotado ve el suyo. Sin proyecto resoluble, **error** — no una vista federada
accidental.

### B11 — `online` se calcula al servir, no se guarda

El listado devuelve `online` derivado de `last_seen` con un umbral que el llamador puede elegir.
Sigue sin haber columna (A8 de S1): el campo del JSON se calcula en cada respuesta.

---

## H4 · Lo que este slice NO hace

- **No recolecta métricas del host.** Es S4. El latido dice «estoy viva», no «cómo estoy».
- **No ejecuta nada.** Exec es S5, pantalla S6.
- **No reporta versión ni dirección desde el agente.** Se puede, y es tentador, pero es una
  escritura del device sobre su propia fila: entra con S4, con su propia prueba de que no puede
  tocar la fila de otro.
- **No usa build tags.** La identidad del host (`os.Hostname`, `runtime.GOOS/GOARCH`) es stdlib
  multiplataforma. Los build tags llegan cuando llegue la captura de métricas (S4) y el PTY (S5).
  Estaba anotado como «cross-OS por build tags» en el tasks de S1: era prematuro.
- **Cero dependencias nuevas.**
