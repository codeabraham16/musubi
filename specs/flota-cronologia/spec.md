# Spec — S13 · La cronología de una máquina

Fase 5 (**lo cognitivo**), primer slice. La pregunta que alguien hace de verdad cuando algo anda
mal no es «¿qué comandos corrieron?» ni «¿quién entró?»: es **«¿qué le pasó a esta máquina?»**. Hoy
esa pregunta se responde llamando a tres bitácoras y ordenándolas a mano — o sea que no se
responde.

```
musubi_fleet_cronologia {"device":"altura-db","desde":"2026-08-25T00:00:00Z","hasta":"…"}
  ->  { "hechos":[ {cuando, tipo, plano, principal, referencia, estado, argv, termino, duracion_seg} ],
        "ventana":{…}, "truncado":…, "ocultos_por_permiso":…, "sin_clasificar":…, "no_visto":[…] }
```

Es la **fundación** de la fase 5 y no su final: correlacionar «desde el martes anda lenta» con algo
exige, primero, poder listar qué pasó el martes. Lo que se cruza con la memoria y con el grafo de
código viene después y se apoya en esto.

---

## H1 · De dónde sale un hecho

### C1 — sólo tablas APPEND-ONLY

Las fuentes son `device_commands`, `screen_sessions` y `shell_sessions`: las tres que registran
**cada ocurrencia**.

Quedan afuera `devices.last_seen`, `services.last_report` y `fleet_policy_state.last_fired` — no
por olvido, sino porque guardan el **último** valor. Una política que disparó cuarenta veces el
martes aparecería como UNA línea, con la hora de la última, y la cronología mostraría un martes
tranquilo. **Un renglón que resume cuarenta es peor que un renglón ausente: el ausente se nota.**

### C2 — las políticas SÍ están, y no por excepción

La acción de una política se encola con `EncolarComando`, igual que la de una persona (I16). Cada
disparo real es una fila de `device_commands`, con el principal de la política en la misma columna
y con el mismo peso. Lo único que no se ve es el disparo que **no llegó a encolar nada**.

**No hay columna que diga «esto fue automático»** (registro **A59**): el origen se lee del nombre
del principal, por convención.

### C3 — el tipo lo decide lo que el hecho REVELA, no la tabla

Una fila de `device_commands` cuyo argv es `musubi:pantalla` revela que alguien miró una pantalla.
Se presenta como `canal_pantalla`, en el plano de **entrar**, y pide `screen:view` — aunque viva en
la tabla de comandos.

---

## H2 · La compuerta

### C4 — es POR HECHO, no por la lista

Las tres fuentes tienen tres compuertas: `exec`, `screen:view`, `shell`. Compuertar la lista entera
con UNA capacidad falla **en las dos direcciones**: con la más laxa le muestra a alguien con `exec`
quién tuvo un prompt; con la más estricta le esconde sus propios comandos a quien puede correrlos.

`fleet.CapDeHecho` es una función **total** sobre el tipo de hecho y se pregunta por separado para
cada línea. Dos personas distintas ven dos cronologías distintas de la misma máquina.

### C5 — el default es NO MOSTRAR

Un tipo sin capacidad asociada no se le muestra a nadie, ni al que tenga todo, y se cuenta en
`sin_clasificar`. El día que se agregue un `musubi:algo` nuevo, la cronología lo esconde en vez de
clasificarlo como comando común: si el default fuera `exec`, esa operación aparecería ante todo el
que pueda ejecutar **antes de que nadie decida quién puede verla**.

El silencio de ese fail-closed lo rompe una prueba que lee el FUENTE
(`TestTodaOperacionInternaDelCodigoEstaClasificada`): toda `"musubi:*"` que el cerebro o el agente
nombren tiene que estar clasificada. Se lee el fuente y no una lista declarada porque una lista
declarada es justo lo que alguien se olvida de actualizar.

### C6 — sin NINGÚN plano visible se explica, no se devuelve vacío

Es lo contrario de lo que hacen las otras cuatro lecturas de flota, y es deliberado: **una lista
vacía en una línea de tiempo se lee como una conclusión** («no pasó nada en esa máquina»), no como
una ausencia de datos. Las otras devuelven inventarios, donde el vacío se lee como vacío.

El mensaje nombra las tres capacidades que destrabarían algo. No es fuga de existencia: la tenencia
ya filtró antes, y `musubi_fleet_list` muestra la máquina sin exigir capacidad ninguna.

### C7 — una máquina revocada conserva su cronología

`PuedeVerHistorialDeDevice`, no `PuedeSobreDevice` (**A51**). La revocación es el kill-switch para
**operar**, no para **auditar** — y el momento en que se revoca una máquina suele ser exactamente
el momento en que hace falta leer qué le pasó.

### C8 — dos máquinas con el mismo nombre no se desempatan solas

El nombre sólo es único dentro de su proyecto. Para una credencial `read: all` que alcanza varios
tenants, elegir una —la primera, la última— devolvería la cronología de una máquina que no es la
que se preguntó, sin decirlo. Se pide `project`.

---

## H3 · La ventana

### C9 — es obligatoria y no tiene modo «todo»

Una cronología sin ventana es una bitácora más, y el tope la cortaría por lo más reciente sin
decirlo. Default 24 h, máximo 30 días.

### C10 — va en el `WHERE`, no en un filtro posterior

La forma cómoda sería reusar las bitácoras existentes —que reciben un tope y no una ventana—, pedir
de más y filtrar en Go. Es exactamente cómo se fabrica una respuesta que miente: pedir las últimas
200 filas de una máquina ocupada y filtrar «el martes» **devuelve vacío**, y ese vacío se lee como
«el martes no pasó nada».

Con la ventana en el `WHERE`, el tope corta lo que sobra DENTRO de la ventana pedida — un corte que
se puede declarar (`truncado`) en vez de uno que se disimula.

### C11 — semiabierta `[desde, hasta)`

Con las dos puntas cerradas, «00 a 12» y «12 a 24» cuentan dos veces el hecho de las 12 y sumar los
tramos da un total que no existe.

### C12 — se normaliza a la granularidad del ALMACENAMIENTO

Las fechas se guardan en RFC3339 **sin fracción de segundo**. Una ventana que termina «ahora»
termina en `22:29:58.7`, que al formatearse se convierte en `22:29:58` — y con el borde superior
abierto, el comando encolado en ese mismo segundo **queda afuera**.

El síntoma es el que más engaña: alguien reinicia un servicio, entra a mirar, y ve la cronología
vacía. Las dos puntas se redondean hacia **afuera**; una punta sin fracción no se mueve, y eso
conserva el mosaico de C11.

**Este bug existió y lo encontró el CONTROL POSITIVO del barrido de aislamiento** —la aserción de
que el admin federado SÍ tiene que ver el dato—, no la aserción de fuga.

### C13 — la ventana que vuelve es la que se aplicó

Contestar con la pedida mientras se aplica otra hace irreproducible una investigación: alguien
copia el `desde` de la respuesta, lo vuelve a pedir, y le vuelven hechos distintos.

---

## H4 · Lo que la respuesta declara

### C14 — los tres contadores dicen cosas distintas

`truncado` = hubo más adentro de la ventana (subí `limite`). `ocultos_por_permiso` = hubo más y no
los podés ver (pedí la capacidad). `sin_clasificar` = hubo más y este cerebro no sabe qué son
(actualizalo). Un solo número los confundiría, y cada uno se arregla distinto.

### C15 — `no_visto` viaja SIEMPRE, también en una respuesta llena

Un registro que no aclara contra qué NO protege es peor que ninguno, porque alguien deja de buscar
el de verdad. Los cinco huecos declarados: no hay serie temporal (**B5**), no hay logs del host, no
hay historial de salud de servicios, no hay historial de disparos de política que no encolaron
comando, no hay contenido de sesiones (**A14**, **B10**).

### C16 — nulls, no ceros

`termino` y `duracion_seg` viajan en null cuando el hecho sigue en curso. Un `0` se dibuja como
«duró nada» y lo que pasa es que no terminó — el mismo cero mentiroso que persigue todo el track
(**A39**), esta vez en el eje del tiempo. `argv` va null en los hechos que no son comandos: una
lista vacía se leería como «corrió algo sin argumentos».

### C17 — el argv nunca lleva la contraseña, y el saneo vive en el DOMINIO

`fleet.ArgvDeBitacora` es la única implementación. Estaba escrita en la tool de la bitácora y la
cronología habría sido la segunda copia — y la copia que se queda vieja es siempre la del camino
que se usa menos. Ahora `ocultarArgvDePantalla` delega, y el saneo se aplica al **construir el
hecho**, así que ninguna superficie futura puede olvidarse de llamarlo.

---

## Lo que este slice NO hace

- **No cruza con la memoria ni con el grafo de código.** Ése es el resto de la fase 5 y se apoya en
  esto.
- **No consulta Prometheus.** Musubi guarda el presente; la serie la guarda Prometheus (**B5**), y
  eso está declarado en `no_visto` en vez de disimulado con un `last_*`.
- **No infiere causas.** Devuelve hechos correlacionados en una ventana; quién concluye es quien
  lee. Una tool que adivinara la causa sería exactamente lo que este repo evita.
