# Propuesta — El hallazgo no muta entre que se emite y se vota

Track: **Encender el arnés**, F5.

## El agujero, concreto

Un hallazgo se emite en un momento y se juzga en otro, con una ronda de crítica cruzada en el
medio. Entre esos dos momentos **nada garantizaba que el texto siguiera siendo el mismo**.

Peor: `PostPosture` hace `ON CONFLICT ... DO UPDATE SET stance=excluded.stance`. Re-postear con la
misma etiqueta **reemplaza la postura anterior en silencio**, sin error y sin rastro.

El tally es determinista sobre los **votos**, pero no sobre el **texto** que esos votos juzgaban.
Un recuento puede ser perfectamente fiel a una discusión que ya no existe.

## La forma: una tripleta

Un hallazgo congelado es un **id estable**, la **huella de su cuerpo canonizado** y la **huella del
árbol** contra el que se emitió.

**El cuerpo no se guarda, sólo su huella.** No es ahorro de espacio: obliga a que quien verifica
tenga el texto en la mano. Un verificador que puede leer el texto del propio registro no verifica —
se mira al espejo.

## Lo que NO se toca, y por qué

`internal/receipt` ya era genérico: importa sólo `sha256`, `hex`, `json`, `fmt`, `strings` y
`time`; no abre git ni toca disco. Todo el amarre a git vive en `treeFingerprint`, en `cmd/`.

El plan proponía **generalizar la aridad de `Compute`**. No se hizo, y la razón es que la propia
regla del plan es más fuerte: *la función nueva va aparte aunque duplique veinte líneas*. Si la
salida de `Compute` cambiara un solo byte, **todos los recibos vigentes se invalidarían y los push
se bloquearían**. `huellaDe` duplica el esquema de longitud-prefijada a propósito, y
`TestH0ComputeSigueDandoLosMismosBytes` congela la salida con hexes literales **obtenidos
corriéndola antes de tocar nada**.

`Check` y `receiptCheck` tampoco se tocan: el hook pre-push está instalado y vivo.
