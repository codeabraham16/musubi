# Spec — S7 · Tier B: lo que no puede correr un agente

Abre la Fase 2. Routers, NAS, servers ajenos, Raspberry Pis: máquinas que **no van a tener un
binario de Musubi adentro**, nunca.

---

## Este slice invierte la dirección de todo el track

Desde S2, el diseño se apoya en que **el agente sale a buscar al cerebro**: nada le entra a la
máquina, el latido es del device hacia afuera, los comandos vuelven en la respuesta. Eso era una
decisión de seguridad y sigue siéndolo.

En Tier B **no hay agente**, así que se invierte: **el cerebro sale a buscar al dispositivo.** Y
eso trae la pregunta que el track viene esquivando: ¿dónde viven las credenciales para llegar?

---

## La decisión: Musubi NO guarda credenciales de Tier B

La respuesta fácil es una tabla de llaves SSH. Es lo que hace un RMM comercial y es exactamente
lo que S6 se negó a construir para las contraseñas de pantalla: un volcado de esa tabla es la
flota entera, con permiso de escritura.

**Musubi ejecuta invocando al `ssh` del sistema.** Consecuencias, todas buenas:

- **La credencial nunca entra a Musubi.** Ni el secreto ni una referencia a él: las llaves las
  administra quien opera, con `ssh-agent` y `~/.ssh/config`, como ya lo hace.
- **Se hereda todo lo que el operador ya tiene**: jump hosts, certificados, claves por host,
  `known_hosts`. Reimplementarlo con una librería SSH sería peor y menos capaz.
- **Cero dependencias nuevas.** `golang.org/x/crypto/ssh` sería la 7ª directa.

El costo es real y se declara: **el cerebro necesita `ssh` instalado**, y hay un `fork+exec` por
comando. Para una flota de decenas de máquinas eso no es nada; si algún día son miles, se revisa.

---

## H1 · La compuerta y la bitácora no cambian

### H1a — Es el MISMO `musubi_fleet_exec`, la MISMA compuerta, la MISMA bitácora

Un Tier B se ejecuta con la misma tool, exige la misma `CapExec` sobre esa máquina y deja la misma
línea en la misma bitácora. Que el transporte sea SSH en vez de la cola es un detalle del
transporte, y quien opera no debería tener que saberlo.

### H2b — El destino lo fija el ALTA, no quien llama

`musubi_fleet_exec` recibe el NOMBRE de una máquina, nunca una dirección. Un llamador no puede
apuntar el cerebro a un host arbitrario: sólo puede ejecutar en máquinas que un admin enroló.

Es la misma clase de guarda que el blindaje SSRF de la ingesta, resuelta por diseño en vez de por
lista negra.

---

## H2 · Lo que se ejecuta es lo mismo en los dos tiers

### H2a — Un `argv` significa lo mismo en Tier A y en Tier B

`ssh host echo '$HOME'` **expande** `$HOME`: del otro lado siempre hay una shell. Si no se hace
nada, el mismo comando hace cosas distintas según el tier — y esa es la clase de sorpresa que
convierte un comando de mantenimiento en un incidente.

Cada argumento se **cita para la shell remota**, de modo que el argv llegue entero. Quien quiera
una shell la pide explícita, igual que en Tier A: `["sh","-c","..."]`.

### H2b — `BatchMode` obligatorio: nunca se pide nada por consola

Sin él, un `ssh` que quiere una passphrase o una confirmación se queda esperando una entrada que
en un servidor no existe, y el comando cuelga hasta el timeout.

### H2c — La verificación de host key NO se desactiva

Un RMM que pone `StrictHostKeyChecking=no` es MITM-able por diseño: quien se meta en el medio de
la red recibe los comandos y devuelve lo que quiera. Si el host no está en `known_hosts`, **el
comando falla y lo dice** — con la línea exacta para arreglarlo.

Es la decisión más fácil de aflojar «para que ande», y por eso tiene su propia prueba.

---

## H3 · Un Tier B no late, y eso cambia qué significa «en línea»

### H3a — `last_seen` de un Tier B lo estampa el CEREBRO cuando lo alcanza

No hay latido. «En línea» pasa a significar *«la última vez que pudimos llegar»*, que es lo único
honesto que se puede decir de una máquina que no habla sola.

### H3b — No se exige que esté «en línea» para intentar

En Tier A se rechaza ejecutar sobre una máquina que no late, porque nadie va a levantar el
comando. En Tier B **se intenta y se ve**: el intento ES la prueba de vida.

---

## H4 · Lo que este slice NO hace

- **No hay SNMP, MQTT ni Redfish.** Los tres necesitan una librería (dependencia nueva) o un
  protocolo binario a mano. SSH cubre routers, NAS, Raspberry Pis y servers sin agente — la
  mayoría de lo que hay. Los otros van a `ABIERTO.md` con su slice.
- **No hay métricas de Tier B.** Correr `/proc` sobre SSH es el paso natural y reusa el parseo
  que ya existe, pero exige refactorizar el colector para que tome CONTENIDO en vez de leer
  archivos. Es **S7b**.
- **No hay sondeo periódico.** `last_seen` se estampa cuando alguien ejecuta. Un prober en
  background es **S7b**.
- **No hay pantalla en Tier B.** La matriz de S1 ya lo dice: no hay framebuffer.
- **Cero dependencias nuevas.**
