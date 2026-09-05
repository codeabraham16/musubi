# Spec — S5 · Ejecución remota auditada

El plano de terminal. Primera cosa que pasa por la compuerta de S3 y **cambia el estado de una
máquina ajena**.

```go
// internal/fleet — dominio
type Comando struct { ID, DeviceID, ProjectID, Principal string; Argv []string; ... }
type EstadoComando string // pendiente | entregado | terminado | expirado

// internal/mcp
musubi_fleet_exec   // encola y espera (acotado). Requiere CapExec sobre ESA máquina.
musubi_fleet_log    // la bitácora: quién ejecutó qué, dónde y cómo salió.

// agente -> cerebro, por la MISMA puerta del latido
POST /fleet/heartbeat  -> respuesta puede traer comandos pendientes
POST /fleet/result     -> el agente reporta el resultado
```

---

## La forma del canal, y por qué NO es una conexión entrante

Un agente corre en un portátil que viaja, detrás del NAT de la oficina de un cliente. **Nada le
entra.** Poner al agente a escuchar un puerto para recibir comandos sería:

- un puerto abierto en cada máquina de la flota, o sea la superficie que este track viene
  evitando desde S2;
- inútil la mitad de las veces, porque esa máquina no es alcanzable.

Así que el comando **viaja de vuelta en la respuesta del latido**, por el canal que el agente ya
abre él mismo. Cuesta latencia (hasta un intervalo de latido) y no cuesta ni un puerto nuevo.

---

## H1 · La auditoría no depende de que nada salga bien

### F1 — La bitácora se escribe ANTES de ejecutar

El registro «*fulano pidió correr esto en tal máquina*» se crea **al encolar**, no al terminar. Si
el cerebro se cae, si el agente nunca responde, si la máquina se apaga a mitad: **el pedido queda
registrado igual**. Una auditoría que sólo guarda lo que terminó bien no sirve para lo único que
se le pide.

### F2 — La bitácora es PERMANENTE; la salida CADUCA

Son dos cosas y se guardan distinto. *«gio corrió `systemctl restart nginx` en pc-gio el 26/8 a
las 14:32, exit 0»* se conserva. Su **stdout** —que puede traer secretos, claves en un log, datos
de un cliente— se acota y se poda.

### F3 — El resultado sólo lo puede reportar la máquina dueña del comando

El agente reporta con el ID del comando; el cerebro verifica que ese comando **sea de esa
máquina**. Sin esto, cualquier device de la flota podría escribir el resultado de un comando
ajeno y envenenar la bitácora de otro.

---

## H2 · La compuerta manda, y nadie la esquiva

### F4 — Sin `CapExec` sobre ESA máquina no se encola

`PuedeSobreDevice(p, d, CapExec)`: tenencia ∧ concesión ∧ aparato. Un admin sin concesiones no
ejecuta nada, igual que no ve métricas.

### F5 — El comando se entrega SÓLO a la máquina a la que fue dirigido

El agente pide su cola presentando su token; el cerebro devuelve **lo suyo**. La identidad sale
del token, como en todo el track.

### F6 — Revocar corta la cola

Un device revocado no recibe comandos pendientes, aunque estén encolados. El kill-switch de S2
tiene que valer también acá — es la ruta más peligrosa de todas.

---

## H3 · Un comando no puede voltear la máquina que monitorea

### F7 — No hay shell implícito: se ejecuta un **argv**, no una cadena

`["systemctl", "restart", "nginx"]`, no `"systemctl restart nginx"`. Quien quiera una shell la
pide explícito: `["sh", "-c", "..."]`.

No es por inyección —quien llega acá ya está autorizado a ejecutar—, es por **claridad de la
bitácora y de la semántica**. Una cadena que a veces pasa por shell y a veces no es la clase de
ambigüedad que hace que un comando de mantenimiento haga algo distinto en dos máquinas.

### F8 — Todo comando tiene timeout, y al vencer se MATA

Sin esto, un comando que espera una entrada que nunca llega deja al agente ocupado para siempre y
la máquina desaparece del inventario.

### F9 — La salida está ACOTADA

Un `cat` sobre un archivo de log de 4 GB no puede volcar 4 GB ni en el agente ni en el cerebro.
Se corta y **se dice que se cortó** — una salida truncada en silencio es peor que una corta.

### F10 — Un comando VIEJO no se ejecuta

Si el agente estuvo caído una semana, no despierta y corre lo que se encoló el lunes. Todo
comando vence; el vencido se marca `expirado` y **no se entrega**.

Es el peor pie de bala de un sistema de colas: el reinicio de un servicio pedido en una
emergencia que ya pasó, ejecutándose siete días después sobre un estado distinto.

---

## H4 · Lo que este slice NO hace

- **No hay shell interactiva (PTY).** Es bidireccional y con streaming — otro problema. Queda como
  **S5b**, anotado en `ABIERTO.md`. El one-shot cubre lo que se hace el 80 % de las veces:
  correr un script, reiniciar un servicio, mirar un estado.
- **No hay allowlist de comandos por máquina.** Es política y va con S10.
- **No hay ejecución en Tier B/C.** Va con S7/S8.
- **Cero dependencias nuevas.**
