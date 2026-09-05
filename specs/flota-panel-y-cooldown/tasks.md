# S9b + S10b — que el auto-heal se vea y sobreviva un reinicio

> Deuda directa de S10, atendida antes de abrir capacidades nuevas. S10 dejó al cerebro
> ejecutando comandos en máquinas ajenas sin una persona detrás; le faltaban las dos cosas que
> hacen que eso sea defendible en la práctica: **que se vea** y **que no se rearme solo**.
>
> Cierra A21, A23 y A24 de `specs/control-de-flota/ABIERTO.md`.

## A24 · El cooldown sobrevive un reinicio

- [x] Migración **33**: tabla `fleet_policy_state`, clave compuesta `(policy, device_id)`.
- [x] `CooldownsDePoliticas` / `MarcarDisparoDePolitica` / `PodarEstadoDePoliticas` en el motor.
- [x] El mapa en memoria sigue siendo el camino caliente y se **siembra** al arrancar; la tabla es
      su respaldo durable. Una lectura por par y por tick serían 200 consultas para un dato que
      sólo cambia cuando algo dispara.
- [x] Un fallo al persistir **no cancela la acción**: el comando ya está decidido y auditado, y el
      costo es un cooldown que no sobrevive al próximo reinicio. Cancelar por no poder anotar
      sería dejar el problema sin atender para proteger la anotación.
- [x] La limpieza de filas huérfanas cuelga de la misma cadencia que la poda de salidas.
      **Una lista vacía de políticas no borra nada**: «no hay políticas» es también lo que se ve
      con un YAML a medio editar, y ese borrado sería irreversible.

**Por qué importaba.** El cooldown es lo único que separa «una política que corrige algo» de una
tormenta de comandos idénticos. Viviendo sólo en memoria, duraba lo que durara el proceso — y
reiniciar no es un evento raro en momentos tranquilos: es lo primero que alguien hace cuando algo
va mal, que es exactamente cuando las políticas están disparando.

## A23 · El auto-heal se ve

- [x] `musubi_fleet_list` devuelve `politicas_activas` y, con `exec`, el detalle: `nombre`,
      `principal`, `condicion`, `hacer`, `cooldown_min`, `ultimo_disparo` y **`puede_actuar`**.
- [x] **`puede_actuar` es el campo que más importa.** Una política cuyo principal perdió la
      concesión —o cuyo comando se cayó de la allowlist— se ve EXACTAMENTE IGUAL que una que
      funciona: las dos figuran en la lista y ninguna hace nada visible hasta que la condición se
      cumple. Es una alarma apagada. Se calcula con la MISMA cadena de guardas que
      `evaluarPolitica`, no con una aproximación: un indicador que dijera «sí» donde la política
      dice «no» sería peor que no tenerlo.
- [x] **El detalle exige `exec`; el conteo no.** Qué comando corre en un servidor se gatea igual
      que la bitácora. Pero que exista algo automático encima no es un secreto: ocultarlo dejaría
      a quien sólo tiene `metrics` viendo cambiar su máquina sin ninguna pista.
- [x] Columna «automático» en `/flota`, con tres estados distinguibles (`—`, `⚙ N` sin detalle,
      `⚙ nombre` con `⚠` ámbar si está inerte) y el detalle completo en el `title`.
- [x] `esc()` para el nombre y el argv: salen de un archivo de configuración y se interpolan en un
      atributo. No es una amenaza probable, pero interpolar texto ajeno sin escapar es una
      costumbre que en algún momento se paga.

## A21 · Se llega y se vuelve

- [x] Enlace `▤ flota` en el panel del cerebro, y `← cerebro` de vuelta en el de flota.
- [x] **Va en `dashboard.html`, no en el bundle.** El motivo anotado en `ABIERTO.md` —«habría que
      tocar el bundle WebGL»— era **incorrecto**: la CI reconstruye `dashboard.bundle.js` desde
      `src/` y compara bytes, pero la cáscara HTML no entra en esa verificación. El bundle quedó
      intacto, y hay una prueba que lo exige.

## Un hallazgo que no venía en el plan

**Un `principals_file` relativo se resolvía contra el CWD del proceso, no contra el workspace.**

`principals_file: ".musubi/principals.yaml"` es lo que cualquiera escribe a mano. Resuelto contra
el directorio de trabajo —que en un servicio de systemd es `/`, porque el unit de
`install-musubi-brain.sh` no fija `WorkingDirectory`— apunta a un archivo que no existe. Y
`loadPrincipals` con un archivo inexistente **no falla**: devuelve el registro **legacy**. El
cerebro arranca bien, sirve bien, y toda la identidad por-miembro se degrada en silencio a **un
solo bearer admin-federado que ve todos los proyectos**.

Hay un WARNING para binds no-loopback y `strict_tenancy` lo rechaza, pero apoyar el aislamiento
entre tenants sobre que alguien lea un warning de arranque no es apoyarlo en nada.

Ahora una ruta relativa cuelga del workspace; una absoluta se respeta tal cual (es la vía para
tener el registro en `/etc`). Con su prueba y su sabotaje.

Lo destapó el e2e, y sólo porque había una política que validar: el servidor se negó a arrancar
al leer otro registro. **Sin políticas habría arrancado, y nadie se habría enterado.**

## Pruebas

**13 nuevas.** **9 sabotajes verificados** uno por uno (aplicar, correr, confirmar el fallo,
restaurar). Una prueba preexistente falló y con razón: `TestMigrationV11OutboxSchema` fija la
versión del esquema para obligar a reconocer cada migración — se documentó la 33 y se subió.

## E2E — y la lección de método que se repitió

Cerebro aislado (`MUSUBI_HOME` temporal, `127.0.0.1:7799`).

| Qué | Resultado |
|---|---|
| El inventario nombra lo automático | `nas` y `pc-gio`: `⚙ vaciar-journal · si mem_pct > 90.0 → journalctl --vacuum-size=200M · como auto-heal · [PUEDE]` |
| Vida A: dispara y persiste | 4 acciones; `fleet_policy_state = ('vaciar-journal','2026-08-27T04:25:14Z')` |
| **Reinicio real** (pid 86052 → 86253) | `cooldowns recuperados del reinicio pares=1` |
| Vida B: 3 barridos con la condición cumplida | **4 acciones — no volvió a actuar.** Métricas del proceso nuevo: ninguna |
| Se saca `journalctl` de la allowlist en caliente | `puede_actuar` pasa a `False` en ≤10 s → se dibuja `⚠ INERTE` |
| ¿El indicador dice la verdad? | 4 → 4 acciones: no actuó, como decía. `result="rechazada"` 6 |
| El WARN de la política inerte | **1 sola línea**, no 6 (el arreglo de ruido de S10 aguanta) |

**La primera medición del reinicio no valía nada, y por segunda vez en el track por la misma
causa: apuntaba a otro proceso.** El servidor «nuevo» murió con `address already in use` y estuve
leyendo el viejo, que por supuesto tenía los cooldowns en memoria. Dos errores encadenados: un
`( cmd & echo $! )` que guardaba el PID del subshell y no el del servidor, y después un `pkill -f`
cuyo patrón coincidía con la línea de comando de mi propio shell y lo mató.

**Antes de creerle a una medición, verificar de qué proceso salió.** Ahora se mata por PID sacado
del puerto, se comprueba que el puerto queda libre, y se compara el PID de las dos vidas.
