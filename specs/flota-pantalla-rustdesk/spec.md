# Spec — S6 · La pantalla, sobre RustDesk self-hosted

Cierra la Fase 1. **No se construye un motor de pantalla: se orquesta uno.** Captura, encoding y
NAT traversal son meses de trabajo y un campo minado de seguridad; RustDesk ya los resolvió y su
relay es software libre que corre en tu VPS.

Lo que Musubi aporta es lo que RustDesk **no** tiene: inventario, la compuerta por
persona-y-máquina, y una bitácora de quién miró qué pantalla.

---

## El problema real: Musubi no puede interceptar el protocolo de RustDesk

RustDesk autentica con **ID + contraseña**. Musubi no está en el medio de esa conversación, así
que no puede "autorizar" una sesión en el sentido literal. Hay dos formas de resolverlo:

**(a) Musubi guarda la contraseña permanente de cada máquina** y se la entrega a quien pase la
compuerta. Descartada: convierte la base de Musubi en un llavero de acceso total a toda la flota,
y un volcado de esa tabla es la flota entera. Además la contraseña sobrevive a la sesión, así que
"tuviste acceso una vez" pasa a ser "tenés acceso para siempre".

**(b) La contraseña se ACUÑA por sesión, dura poco, y Musubi no la guarda.** Es la elegida.

El flujo, y cada paso reusa algo que ya existe:

1. Alguien pide una sesión → la **compuerta de S3** decide (`CapScreen` sobre ESA máquina).
2. Musubi genera una contraseña al azar y la manda a la máquina **por el canal de comandos de
   S5** — el mismo que ya está probado y auditado.
3. El agente la aplica en RustDesk y **programa su propio vencimiento**.
4. Musubi devuelve la contraseña a quien la pidió, **una sola vez**, y la olvida.
5. La fila de la sesión queda: quién, qué máquina, cuándo. **Nunca el secreto.**

---

## H1 · No hay un secreto de larga vida en ningún lado

### G1 — Musubi NUNCA guarda la contraseña de pantalla

Ni en claro ni hasheada. Se genera, viaja dos veces (a la máquina y a quien la pidió) y se
descarta. La fila de la sesión registra que hubo acceso, no cómo entrar.

Hashearla no serviría de nada —quien verifica es RustDesk, no Musubi— y guardarla en claro sería
un llavero de la flota entera en una tabla.

### G2 — La contraseña CADUCA, y el vencimiento no depende del cerebro

El agente recibe la contraseña **y su TTL**, la aplica, y programa él mismo el reemplazo por una
al azar que nadie conoce.

Que el vencimiento viva en el agente y no en el cerebro es el invariante: si el cerebro se cae,
si la red se corta, si alguien apaga Musubi — **la sesión se cierra igual**. Una caducidad que
depende de que otra máquina siga viva no es una caducidad.

### G3 — Se muestra una sola vez

Igual que el token de enrolamiento. No hay forma de recuperarla: se pide una sesión nueva.

---

## H2 · La compuerta manda, otra vez

### G4 — Sin `CapScreen` sobre ESA máquina no hay sesión

`PuedeSobreDevice(p, d, CapScreen)`. Un admin de la memoria sin concesiones no mira ninguna
pantalla, igual que no ejecuta ni ve métricas.

### G5 — Un Tier B nunca tiene pantalla

La matriz de S1 sigue rigiendo: un router administrado por SNMP no tiene framebuffer. No es
política, es que no existe la cosa.

### G6 — Revocar la máquina corta la sesión

El kill-switch vale también acá: el device revocado no recibe el comando y su cola se corta.

---

## H3 · Queda escrito quién miró

### G7 — Toda sesión se registra ANTES de acuñar nada

Misma regla que F1 de S5. Si el agente nunca aplica la contraseña, el **pedido** queda igual.

### G8 — La bitácora de sesiones no la ve cualquiera

Sólo las máquinas sobre las que tenés `screen`. Saber quién mira la pantalla de un servidor es
información sensible por sí sola.

---

## H4 · Lo que Musubi NO hace, y hay que decirlo

### G9 — El consentimiento lo aplica RustDesk, no Musubi

Un device puede exigir que la persona sentada frente a la máquina apruebe la conexión
(*attended*). **Eso lo hace RustDesk**, con su propia configuración. Musubi guarda la política
como metadato del inventario y el despliegue la configura — pero **no la impone**, y afirmar lo
contrario sería vender una garantía que no existe.

### G10 — Musubi no ve ni graba la sesión

El video va directo entre los dos clientes de RustDesk (P2P, o por el relay si el NAT no deja).
Musubi no está en ese camino. Lo que sabe es que la sesión **se autorizó**, no lo que pasó dentro.

---

## H5 · Lo que este slice NO hace

- **No instala RustDesk en las máquinas.** El despliegue de `hbbs`/`hbbr` y el instalador del
  cliente van como material de `deploy/`, no como código.
- **No hay grabación de sesión.** Es un slice propio si hace falta, y una decisión legal antes que
  técnica.
- **Cero dependencias nuevas.**
