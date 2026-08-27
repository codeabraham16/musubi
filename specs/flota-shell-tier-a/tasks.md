# S5c — Shell interactiva en Tier A, y el puerto de un Tier B

Cierra **A25** de `ABIERTO.md` y un hueco que apareció al empezar.

## El hueco que apareció primero: un Tier B en un puerto no estándar era inalcanzable

`gio@nas:2222` es la forma que cualquiera escribe —la que usan scp, rsync, git y media internet—
y `ssh` **no la entiende**: para él eso es un hostname entero. El error resultante era

```
ssh: Could not resolve hostname 127.0.0.1:2222: Name or service not known
```

o sea que mandaba a depurar el DNS de un host que estaba perfecto. Es exactamente la clase de
mensaje engañoso contra la que existe `explicarFalloSSH`, y esta vez lo producíamos nosotros al
armar mal la invocación. Y mover el 22 es lo primero que hace cualquiera con un NAS expuesto.

- [x] `destinoYPuertoSSH`, aplicada en los **dos** caminos (one-shot e interactiva). Arreglar uno
      solo sería peor que ninguno: la máquina andaría para ejecutar comandos y no para abrir una
      terminal, sin ninguna pista de por qué.
- [x] **No se toca ADB**: ahí `host:puerto` ES el serial del dispositivo, y partirlo rompería todos
      los Android por red.
- [x] IPv6 pelada (`fe80::1`) NO se parte; entre corchetes con puerto (`[fe80::1]:2222`) sí. Lo que
      parece un puerto y no lo es (`:0`, `:99999`, `:web`) se queda pegado: inventar un `-p 0`
      produciría un error de ssh todavía más raro que el original.

## A25 · La shell llega a las máquinas con agente

**La diferencia con Tier B cabe en una línea: a un Tier A no le entra nadie.** Está detrás de un
NAT, sin puertos abiertos, y sólo sabe SALIR hacia el cerebro — es toda su razón de ser. Así que
el canal es un **encuentro**: el cerebro deja la sesión esperando, avisa por la cola de comandos
(`musubi:shell`, el mismo canal por el que ya viajan los exec y las sesiones de pantalla), y el
agente se conecta desde su lado.

- [x] `fleet.CanalAgente`: dos buffers, uno por sentido, con la misma contrapresión. Los campos se
      llaman por su **destino** (`haciaLaMaquina`, `haciaLaPersona`) porque «entrada» y «salida»
      significan lo contrario según de qué lado se pare uno, y ésa es la confusión más fácil de
      cometer en todo el track.
- [x] Dos rutas nuevas del lado del agente, que **autentican dispositivos** (al revés que el relay
      de personas, que está en el archivo de al lado).
- [x] El pty local se pide con **`script`**, no con ioctls sobre `/dev/ptmx` — mismo criterio con
      el que el track invoca al `ssh` y al `stty` del sistema. Y con `/dev/null` como archivo de
      transcripción **a propósito**: `script` graba, y grabar lo que alguien teclea es la decisión
      legal que este track dejó sin tomar. Es usar su pty sin usar su grabadora.
- [x] La demora se **dice**: un Tier A se entera en su próximo latido, así que el prompt puede
      tardar hasta un intervalo (30 s por defecto). Un prompt que tarda medio minuto sin
      explicación se lee como un cuelgue, y quien lo sufre corta y reintenta — abriendo otra sesión.

### La guarda que importa

**¿Esta sesión es de ESTA máquina?** Por el canal del agente viaja *todo lo que la persona teclea*,
contraseñas de sudo incluidas. Sin esa línea, cualquier máquina de la flota con un token válido
—o sea, cualquiera que alguien comprometa— recoge las teclas de la sesión abierta en otra. Es la
peor fuga posible del track, y el sabotaje la demuestra literalmente: el cuerpo de la respuesta
trae `"contraseña-de-sudo\n"`.

## Lo que el e2e dejó a la vista, y ahora se dice

`id -un` dentro de una shell de Tier A devolvió **`davantis`**: la shell corre como **el usuario
que ejecuta el agente**. Si el agente corre como servicio de systemd, es una shell de root.
Conceder `shell` sobre una máquina es conceder ese usuario, entero — y eso no estaba escrito en
ningún lado. Ahora está en la descripción de la tool y en `musubi shell --help`.

## Dos defectos que sólo aparecieron corriéndolo

1. **El resultado volvía sin `ComandoID`.** No falla ruidosamente: el cerebro responde 403 («ese
   comando no es de esta máquina»), el agente logea y sigue, y el comando queda `entregado` PARA
   SIEMPRE — la bitácora nunca registra que la sesión terminó. Leyendo el código no se ve, porque
   la función que devuelve el resultado está en otro archivo que la que lo inicializa. Fijado con
   una prueba que recorre **todas** las ramas de `ejecutar`.

2. **Cada sesión dejaba un zombie.** Se mataba el `script` y no se lo cosechaba. No consume CPU ni
   memoria y no se nota en semanas — hasta que la máquina se queda sin PIDs. A ojo parecía un pty
   que no había muerto; `ps` decía `[script] <defunct>`.

## Pruebas

**11 nuevas.** **9 sabotajes verificados.**

Y una prueba propia que **nombraba una guarda y ejercitaba otra** — la cuarta de este tipo en el
track, con una causa nueva: ponía una sesión de un Tier B y la pedía con el token de un Tier A,
pero eso lo rechaza la guarda de *pertenencia*, que corre antes. Pasaba con la aserción de tipo
sacada porque nunca llegaba a ejecutarse. Para aislarla hay que construir el estado imposible a
mano. En el camino, la misma prueba tenía un `w.Code < 400` que confundía **un rechazo deliberado
con un panic convertido en 500** — ahora exige el 410 exacto.

## E2E

Cerebro aislado + **agente real** latiendo cada 2 s, sobre esta misma máquina como Tier A.

| Qué | Resultado |
|---|---|
| Prompt del pty remoto | El prompt real del usuario, **con color ANSI y glifos powerline intactos** — el relay no interpreta un byte (T9) |
| `tty` | `/dev/pts/1` — un pty de verdad |
| `id -un` | `davantis` — corre como el usuario del agente |
| Aviso de demora | Viaja en la respuesta de apertura |
| Cierre | 204, sesión `cerrada` a los 28 s en la bitácora |
| Bitácora de comandos | `esta-pc · op · musubi:shell c8ee12ea… · entregado` |

## Lo que sigue sin verificarse

**A28 (correr la shell contra un `sshd` real) no se pudo hacer acá: esta máquina no tiene
`openssh-server` instalado**, sólo el cliente. Instalar un servidor SSH es levantar un servicio de
red, y ésa es una decisión del operador, no mía. Queda en `ABIERTO.md` con la receta exacta para
que sea cuestión de cinco minutos cuando se quiera.
