// boceto-a.mjs — «EL NÚCLEO». La memoria como tracto, no como árbol.
//
// ─────────────────────────────────────────────────────────────────────────────────────────────
// LOS TRES RECLAMOS DE ESTA VUELTA, Y QUÉ SE HIZO CON CADA UNO
//
//   1. «que deje de parecer un árbol»
//      No era el trazo: era la TOPOLOGÍA. Un tronco vertical con la copa arriba tiene suelo y
//      cielo, y eso es un árbol por más dendrítica que se dibuje cada rama. Ahora la raíz es un
//      NÚCLEO y los actores salen de él en todas las direcciones, repartidos con la espiral de
//      Fibonacci sobre la esfera. No hay arriba. → `colocarNucleo` en comun.mjs, y hay un
//      invariante que lo mide: el sesgo de las direcciones de primer nivel tiene que dar ~0.
//
//   2. «se sienten pocas neuronas, las ramas son inventadas»
//      Las dos mitades eran ciertas y son la misma. Una rama era UN cilindro con una textura de
//      neurona encima: la geometría no contenía células, las dibujaba. Ahora la rama NO EXISTE
//      como objeto — existen los HILOS, y la rama es lo que se ve cuando pasan muchos juntos.
//      Es lo que es un tracto de verdad: el cuerpo calloso no es un tubo, son doscientos millones
//      de axones en paralelo.
//
//      Y de ahí sale lo que hace que el dibujo no pueda mentir:  hilos(padre) = Σ hilos(hijos).
//      Un axón no aparece ni desaparece en una bifurcación. Así el grosor deja de ser una fórmula
//      sobre el dato —la ley de Rall era exactamente eso— y pasa a ser el dato: contá los hilos
//      del tronco y te da la suma de todas las hojas. La ficha lo dice en cada haz.
//
//   3. «la manera de mover el 3D es muy tosca»
//      Segunda vez que se dice, así que no se afinaron constantes: se reescribió la cámara.
//      Amortiguación por TIEMPO y no por cuadro (era el tirón), inercia al soltar, zoom hacia el
//      cursor y vuelos con duración y curva en vez de una persecución asintótica. El detalle de
//      por qué cada una se siente distinto está en `crearCamara`.
//
// ─────────────────────────────────────────────────────────────────────────────────────────────
// LO QUE SE CONSERVA, PORQUE FUNCIONABA
//
//   · La sección sigue siendo la unidad navegable: flechas para padre/hija/hermanas, clic para
//     volar, migas para volver. Moverse entre ramas no es puntería.
//   · El impulso sale de un ACTO, no de un reloj: sin evento no hay luz, la misma regla del panel.
//   · Cada memoria es un BOTÓN, ahora montado sobre un hilo concreto y no sobre «la rama».
//   · Las relaciones están todas las que tienen sus dos extremos dibujados, y las que no se
//     declaran aparte. Recortar sin decir cuánto es como un dibujo empieza a mentir.
//
// ─────────────────────────────────────────────────────────────────────────────────────────────
// LOS TRES DE ESTA VUELTA — «un negro raro que opaca al resto», «las conexiones no se ven
// naturales», «no termino de saber qué presioné»
//
//   1. EL NEGRO ERA EL CONTORNO, y al final NO se arregló: se sacó. Era el halo por profundidad
//      —la técnica estándar de tractografía— y se le dieron tres vueltas antes de aceptar que la
//      premisa era el problema:
//
//        engordaba en unidades de MUNDO   de cerca apagaba el 7,9 % del contenido, 9.064 px de
//                                         mancha maciza; de lejos 1,6 % y ninguna
//        reborde de 2,5 PÍXELES           la mancha bajó a 9 px, pero medido por OSCURECIMIENTO
//                                         seguía velando 123.766 px con 31/255 de media
//        salto por instancia              123.766 → 78.193 px. Mejor, y todavía un velo
//
//      Lo cerró mirar las dos imágenes al lado: sin contorno, el haz que cruza por detrás aparece
//      ENTERO en vez de con una cuña negra de borde duro comida encima. Un haz de hilos YA es
//      opaco por acumulación y el buffer de profundidad ya resuelve el cruce; lo único que
//      agregaba el halo era pintar fondo sobre los huecos ENTRE hilos, que es justo lo que hace
//      que un haz se lea como fibras. La descripción del usuario era la firma exacta de un
//      oclusor: «negro raro» (pintado del color del fondo), «al moverme desaparece» (depende del
//      punto de vista) y «al lado de las ramas» (pegado a lo que lo proyecta).
//
//      Y el gris del centro era otro: el núcleo se pintaba de un gris propio «porque no le
//      pertenece a nadie». Es falso — es de todos a la vez. Ahora cada hilo lleva el color de la
//      hoja donde termina, que ya se sabe antes de dibujarlo. Ver `destinoDeHilo` y `pasoMezcla`.
//
//   2. LAS CONEXIONES eran cuerdas. Ningún axón atraviesa el tejido por el camino más corto: sale
//      del soma, se mete en un tracto y viaja con los demás. Ahora la relación sube por su rama
//      hasta el ancestro común y baja — agrupamiento jerárquico de aristas, que acá además es lo
//      que pasa de verdad. Ver `rutaSinapsis`.
//
//   3. «QUÉ PRESIONÉ» tenía dos causas y ninguna era la puntería, que ya acertaba. La señal de
//      selección era BRILLO, en una escena donde todo emite y encima hay bloom; y el clic VOLABA
//      LA CÁMARA, así que lo elegido dejaba de estar donde estaba. Ahora un clic elige y no mueve
//      nada, el doble clic vuela, y lo elegido queda marcado con un anillo de tamaño constante en
//      pantalla, puesto en el punto exacto del eje que señalaste (medido: 0 px de error).
//
//   4. LA CALIDAD DEL DIBUJO, que es lo que quedaba cuando el negro se fue:
//      · el haz era una PANA PLANA. Ahora lleva oclusión dentro del haz (un hilo del centro
//        recibe menos luz que uno de la superficie) y el lado del HAZ que mira a la luz, con la
//        normal radial. El `dif` del fragmento ilumina cada hilo por separado y todos son
//        paralelos, así que no podía redondear el conjunto.
//      · el soma era una BOLITA pegada al arranque de cada eslabón, y en un hilo fino eso es un
//        collar de cuentas. Ahora es un huso alineado con la fibra: se lee como engrosamiento, y
//        además es la forma real del cuerpo de una neurona de tracto.
//      · los hilos eran perfectamente rectos y equiespaciados — tejido a máquina. Ahora ondulan
//        DENTRO del haz, desfasados entre vecinos.
//      · lo elegido se lavaba a CIAN BLANCO: la selección se sumaba DESPUÉS del rolloff. Ahora es
//        una ganancia antes, así que sube de brillo conservando el tono.
//
//   UNA HIPÓTESIS MÍA QUE EL DATO REFUTÓ, y queda anotada para que nadie la reponga: sospeché de
//   la ATMÓSFERA —la rampa de profundidad— porque apagada la escena se ve más vívida. Medido, no
//   es ella: la saturación media del contenido pasa de 0,758 a 0,794 y el brillo de 51,7 a 55,6.
//   Y probé restringir el contorno a la CÁSCARA del haz: cambia la imagen y baja el apagado de
//   59.696 a 59.363 píxeles, un 0,6 %. Ninguna de las dos era el problema.

import { colocarNucleo, medirEnredo } from './comun.mjs';
import { construir, HEBRA } from './forma.mjs';

/* ── DOS MÉTRICAS QUE NO DICEN LO MISMO, Y HAY QUE MIRAR LAS DOS ───────────────────────────────
     ENREDO    con cuántos haces AJENOS se cruza un haz en promedio, en el ESPACIO
               (`medirEnredo`). Un cruce real no lo arregla ningún ángulo de cámara: los dos haces
               se leen como una sola cosa desde donde te pares.
     AJENAS %  qué fracción de las celdas de pantalla tiene dos secciones NO emparentadas encima,
               sobre 12 puntos de vista. Es lo que el ojo ve de golpe — y se mitiga girando.

   Lo que destapó medir el enredo: **183 de los 191 choques —el 96 %— eran entre HERMANAS**, y son
   inevitables mientras nazcan todas del mismo punto. A distancia cero no hay ángulo que separe.
   Ninguna cantidad de `apertura` podía tocar eso; por eso `apertura` ya no existe.

     colocación                    ajenas %   enredo   px/hilo
     vieja (apertura fija 1,40)       18,4     0,866      0,68
     bifurcar (aire 3, pM 0,85)       22,0     0,059      0,81   ← ésta

   Cuesta 3,6 puntos de solape y compra 15× menos interpenetración, y encima deja los hilos más
   gruesos en pantalla. Dos cosas más que el barrido refutó: el TROPISMO empeora (el empuje es el
   mismo para todas las hermanas, así que las junta justo después de separarlas) y MENOS curvatura
   separa más que más. */
const { S } = await construir({
  id: 'a',
  nota: 'las ramas no se dibujan: son los hilos que pasan por ellas',
  seccionado: { maxNivel: 8, minCarga: 10 },
  colocar: (Sec) => colocarNucleo(Sec, {
    origen: [0, 0, 0], nucleo: 40, largo: 150, curvatura: 0.12, tropismo: 0, semilla: 11,
    // `aire` es EL parámetro: cuántos radios de haz de negro se exige entre dos hermanas.
    aire: 3.0, naciente: 0.85, aperturaMax: 1.30, polarEje: 0.20,
    // PISO DEL ÁNGULO. `aire` está en radios, así que para dos ramas finas pide un hueco de 2,5
    // unidades: alcanza para que no se toquen y son menos de dos píxeles en pantalla. El enredo
    // bajaba y el ojo seguía viendo una sola cosa. Con el piso, ajenas 23,3 → 22,0.
    polarMin: 0.85,
    // Los MISMOS que van a `enhebrar`: la separación se mide sobre el grosor que se dibuja.
    radioHilo: HEBRA.radioHilo, separacion: HEBRA.separacion,
  }),
  montaje: { camara: { az: 0.55, el: 0.20, min: 8, max: 3000 } },   // sin dist: lo encuadra la caja
});

// LA MEDICIÓN DEL ENREDO VA A PEDIDO (`?enredo`): son ~90.000 pares por 64 pruebas segmento a
// segmento cada uno, varios segundos. Ponerlo en el camino de carga sería pagar el diagnóstico en
// cada apertura de la página.
if (location.search.includes('enredo')) {
  console.log('[enredo]', medirEnredo(S, { muestras: 8 }));
  console.log('[enredo · margen 2]', medirEnredo(S, { muestras: 8, margen: 2 }));
}
