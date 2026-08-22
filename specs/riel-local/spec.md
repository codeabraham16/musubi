# Spec — El riel también ve lo local

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

```go
// internal/mcp — el vertedero: un archivo por proceso, una linea JSON por evento.
type spoolLocal struct{ ... }
func nuevoSpool(dir string, pid int, tope int64) *spoolLocal
func (s *spoolLocal) escribir(e LiveEvent)   // best-effort, NUNCA devuelve error al llamador
func (s *spoolLocal) cerrar()                 // borra el archivo propio

// cmd/musubi — el lector: sigue el directorio y emite los eventos nuevos.
type lectorSpool struct{ ... }
func nuevoLector(dir string) *lectorSpool
func (l *lectorSpool) leerNuevos() []LiveEvent
func (l *lectorSpool) podar(ahora time.Time)  // saca archivos de procesos muertos
```

Ubicación: `.musubi/live/<pid>.jsonl`. Procedencia en el evento: `origen: "local" | "central"`.

---

## H1 · El presente se ve, y se ve entero

### A1 — Un evento local aparece en el riel sin pasar por el ledger

Una tool ejecutada por un daemon stdio llega al panel **sin depender del volcado del ledger**. Es la
razón de ser: el ledger vuelca cada 10 s y estampa la hora del INSERT, así que no puede sostener un
presente. La prueba mide la separación entre eventos, no sólo que lleguen: dos tools ejecutadas con
20 ms de diferencia tienen que llegar con marcas distintas.

### A2 — Con N daemons escribiendo a la vez no se pierde ni se mezcla nada

Medido: hay **7 daemons stdio vivos** en esta máquina. Con un archivo por proceso no hay contención,
y la prueba lo exige: N escritores concurrentes producen exactamente N×M eventos, cada uno parseable
como una línea JSON completa. Una línea entrelazada es una pérdida silenciosa — se lee como un
evento corrupto y se descarta sin que nadie se entere.

### A3 — El panel que arranca tarde ve lo que ya estaba

El daemon vive antes que el panel. Un panel que arranca después lee lo que hay en el spool en vez de
empezar en blanco, igual que el `backlog` del central.

### A4 — La procedencia viaja con el evento

Cada evento dice si es `local` o `central`. Sin eso el riel afirma algo falso: mezcla el trabajo de
esta máquina con el de toda la empresa y no hay forma de distinguirlos. Es el mismo principio que ya
rige en el feed del central, donde el evento lleva `principal` y `project`.

---

## H2 · Ver no puede romper

### A5 — Un spool que falla no hace fallar la tool

Misma garantía que el ledger (L2) y que el feed en vivo. Disco lleno, directorio sin permisos,
archivo borrado bajo los pies: `escribir` traga el error y la tool responde igual. Una tool que
empieza a fallar porque su telemetría falla es peor que no tener telemetría.

### A6 — El spool no crece sin fin

Cada proceso acota su propio archivo. Al cruzar el tope, se trunca y sigue. Un feed no necesita
historia —para eso está el ledger—, y un archivo que crece para siempre en `.musubi/` es una bomba
de relojería en una máquina que corre 7 daemons.

### A7 — Un daemon que muere no deja basura eterna

El daemon borra su archivo al cerrar. Y como morir de golpe existe, el lector **poda**: un archivo
cuyo PID ya no vive se elimina. Sin esto, cada daemon muerto deja un archivo que el panel relee para
siempre — que es exactamente la forma del bug de los `-watch` huérfanos que motivó medir todo esto.

### A8 — El sondeo no tapa el trabajo

`work`, `phase` y `sync_status` ya se clasifican `sondeo`. Con 36.516 llamadas de cada una en un día
contra 136 de trabajo real, un riel que las mezclara sería ilegible. La clasificación es la misma
del central: **una sola copia**, para que no se desincronicen.

---

## H3 · Lo que este cambio NO hace

- **No toca el ledger.** Historia y presente siguen separados.
- **No enciende el spool en el central.** `musubi serve` ya tiene suscriptores por HTTP; escribir a
  disco además sería 100.000 líneas diarias para nadie.
- **No pinta el grafo con eventos locales.** El grafo dibuja la memoria local; un evento sólo
  dispara un pulso.
