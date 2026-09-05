# S6b — La procedencia del `rustdesk_id`

Cierra **A13** de `ABIERTO.md`.

## El plan anotado no era viable, y tampoco habría servido

`ABIERTO.md` decía «verificar el `rustdesk_id` contra el relay». Al mirarlo de cerca:

- **hbbs** (el relay OSS de RustDesk) **no expone ninguna API** para eso. Habría que hablarle su
  protobuf, que es reimplementar medio cliente — y una dependencia nueva, o un protocolo binario
  a mano.
- Y aunque la expusiera, **sólo diría qué conexión reclama ese id ahora mismo**. No diría cuál de
  nuestras máquinas es, que es la pregunta que teníamos.

Un motivo anotado en un registro de pendientes también envejece. Éste envejeció mal.

## Lo que sí ataca el caso real: la COLISIÓN

Ese id lo reporta la propia máquina en su latido — entrada no confiable. Si **dos máquinas dicen
ser la misma**, conectarse es una moneda al aire. Y se llega ahí por dos caminos:

- **El malicioso**: alguien comprometió una máquina y declaró el id de otra, para que un operador
  abra la pantalla equivocada. Al declararlo, **colisiona con la verdadera**: ésa es su firma.
- **El benigno, y mucho más frecuente**: dos máquinas clonadas de la misma imagen. RustDesk deriva
  su id de características de la máquina, así que los clones nacen iguales — y hasta hoy eso era
  invisible, aunque conectarse ya fuera una moneda al aire.

Una sola guarda cubre los dos, cuesta una consulta y se puede verificar entera.

- [x] `QuienMasDiceSer`: quién más reporta ese id. **Se DERIVA, no se guarda** — una columna de
      colisión habría que actualizarla en cada alta y cada latido, y el día que alguien olvide una
      ruta miente justo cuando importa. Mismo criterio que el «en línea».
- [x] **`musubi_fleet_screen` SE NIEGA** cuando el id es ambiguo. La alternativa amable —abrir con
      una advertencia— entrega una contraseña de sesión y manda a alguien a una pantalla que puede
      no ser la que cree, que es el daño exacto a evitar. Y el arreglo (regenerar el id en la
      máquina duplicada) hace falta igual.
- [x] El mensaje dice **qué pasa, con quién y cómo se arregla**: quien lo recibe está intentando
      mirar una pantalla y no tiene por qué saber cómo RustDesk asigna sus ids.

### La colisión se mira globalmente, y no se nombra lo ajeno

Acotar la consulta al proyecto de quien pregunta dejaba pasar **el caso peor**: dos tenants con el
mismo id, donde un operador aterriza en la máquina de otra empresa. Pero nombrar una máquina ajena
rompería el aislamiento por proyecto.

Se resuelve contando aparte: los **nombres** de las máquinas del propio alcance, y un **conteo** de
las de afuera. Alcanza para decir «este id es ambiguo, no te fíes» sin decir de quién.

## Y el cambio, que la colisión no cubre

- [x] Migración **35**: `rustdesk_id_previo` y `rustdesk_id_cambiado`. Un id que se mueve solo
      tiene dos explicaciones —se reinstaló la máquina, o alguien miente— y las dos ameritan quedar
      escritas. No bloquea nada; se ve.
- [x] Va en el **mismo UPDATE** que el valor nuevo, con un `CASE`: entre un SELECT y un UPDATE
      separados cabe otro latido de la misma máquina, y el «previo» que se guardaría sería el que
      acaba de escribir el otro.
- [x] El **primer** reporte no es un cambio, es el estreno. Y re-reportar el mismo id tampoco: si
      contara, cada latido pisaría el «previo» con el valor actual y se perdería el dato que importa.

## Se ve antes de necesitarlo

- [x] `musubi_fleet_list` trae `rustdesk_id`, `rustdesk_id_ambiguo`, con quién colisiona y si
      cambió — sólo para quien tiene `screen` sobre esa máquina.
- [x] Columna **pantalla** en `/flota`: el id, `⚠ ambiguo` en ámbar, o `↻` si cambió. Descubrir el
      problema en el momento en que hace falta mirar una máquina es descubrirlo tarde.
- [x] Una máquina con id limpio **no lleva marca**: un aviso que aparece siempre enseña a ignorarlo.

## Un bug que la migración destapó al instante

`columnasDevice` la compartían el `SELECT` y el `INSERT`. Agregar dos columnas rompió el alta con
`16 values for 18 columns` — un error que **no aparece al compilar**, sólo al dar de alta un
dispositivo. Son dos listas con propósitos distintos: el SELECT trae todo lo que se escanea, el
INSERT sólo lo que el alta conoce. Ahora el INSERT nombra las suyas explícitamente.

## Pruebas

**6 nuevas.** **6 sabotajes verificados**, incluido el que acota la colisión al proyecto (pierde el
caso peor) y el que nombra la máquina ajena (rompe el aislamiento).

## Lo que sigue sin cubrirse

Si una máquina comprometida declara un id que **no colisiona con ninguna de las nuestras** —uno
inventado, o el de una máquina ajena a la flota— la colisión no lo detecta. Lo que sí queda es que
el id **cambió**, visible en el inventario y en el panel. Detectarlo con certeza requeriría que el
relay dijera quién es quién, y ya vimos que no puede. Se anota como B12.
