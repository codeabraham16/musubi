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
//   1. EL NEGRO ERA EL CONTORNO, y no era una cuestión de gusto: crecía en unidades de MUNDO.
//      Un hilo de 0,52 con grosor 3 se vuelve una funda de 1,56, y como los hilos de un haz están
//      a 2,4 de distancia, las fundas se TOCAN y el haz entero proyecta una losa maciza sobre lo
//      que tenga detrás. Medido: de cerca apagaba el 7,9 % del contenido con 9.064 píxeles de
//      mancha maciza; de lejos, 1,6 % y ninguna. Por eso no aparecía revisando la vista general.
//      Un contorno se mide EN PANTALLA: 2,5 px de reborde, siempre. Quedó en 2,9 % de cerca y
//      3,3 % de lejos —el mismo número, que es la señal de que ahora es un contorno— con 9 px de
//      mancha en vez de 9.064. Ver `CONTORNO_V`.
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
//   UNA HIPÓTESIS MÍA QUE EL DATO REFUTÓ, y queda anotada para que nadie la reponga: sospeché de
//   la ATMÓSFERA —la rampa de profundidad— porque apagada la escena se ve más vívida. Medido, no
//   es ella: la saturación media del contenido pasa de 0,758 a 0,794 y el brillo de 51,7 a 55,6.
//   Y probé restringir el contorno a la CÁSCARA del haz: cambia la imagen y baja el apagado de
//   59.696 a 59.363 píxeles, un 0,6 %. Ninguna de las dos era el problema.

import { cargar, seccionar, colocarNucleo, contarFibras, medirEnredo } from './comun.mjs';
import { armarRaiz } from './datos.mjs';
import { montar } from './escena.mjs';

const t0 = performance.now();
const datos = await cargar('./grafo-local.json');
const { raiz, colorDe, racimos } = armarRaiz(datos.neurons, { titulo: 'memoria' });

// maxNivel 8 y minCarga 10: medido sobre el cerebro local, da ~440 secciones. Con minCarga 1 el
// árbol baja hasta la nota suelta y salen miles de tramos de una memoria — la maraña otra vez,
// pero con más geometría. El tope se DECLARA en la ficha de cada sección hoja («absorbe N»).
const S = seccionar(raiz, { maxNivel: 8, minCarga: 10 });
// UNA SOLA FUENTE PARA LA DENSIDAD DE HILOS, y tiene que correr ANTES de colocar: la separación
// entre hermanas se calcula sobre el RADIO REAL de sus haces, y ese radio sale de `fibras`. Las
// mismas opciones van después a `montar` — dos conteos distintos separarían las ramas sobre un
// grosor que después no se dibuja.
const HILOS = { porMemoria: 6, maxHoja: 22 };
const HEBRA = { radioHilo: 0.52, separacion: 2.60, largoNeurona: 17, torsion: 0.6 };
contarFibras(S, HILOS);
/* ── DOS MÉTRICAS QUE NO DICEN LO MISMO, Y HAY QUE MIRAR LAS DOS ───────────────────────────────
     ENREDO    con cuántos haces AJENOS se cruza un haz en promedio, en el ESPACIO
               (`medirEnredo`). Un cruce real no lo arregla ningún ángulo de cámara: los dos haces
               se leen como una sola cosa desde donde te pares.
     AJENAS %  qué fracción de las celdas de pantalla tiene dos secciones NO emparentadas encima,
               sobre 12 puntos de vista. Es lo que el ojo ve de golpe — y se mitiga girando, y con
               el contorno por profundidad.

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
colocarNucleo(S, {
  origen: [0, 0, 0], nucleo: 40, largo: 150, curvatura: 0.12, tropismo: 0, semilla: 11,
  // `aire` es EL parámetro: cuántos radios de haz de negro se exige entre dos hermanas.
  aire: 3.0, naciente: 0.85, aperturaMax: 1.30, polarEje: 0.20,
  // PISO DEL ÁNGULO. `aire` está en radios, así que para dos ramas finas pide un hueco de 2,5
  // unidades: alcanza para que no se toquen y son menos de dos píxeles en pantalla. El enredo
  // bajaba y el ojo seguía viendo una sola cosa. Con el piso, ajenas 23,3 → 22,0.
  polarMin: 0.85,
  // Los MISMOS que van a `enhebrar`: la separación se mide sobre el grosor que se dibuja.
  radioHilo: HEBRA.radioHilo, separacion: HEBRA.separacion,
});

const vista = montar({
  secciones: S, colorDe, titulo: 'memoria',
  sinapsis: datos.synapses,
  // UN HILO CADA 6 MEMORIAS. Es la única constante libre de todo esto y define la densidad: con 1
  // el tronco tendría 2.267 hilos y se vería como una pared sólida; con 30, ocho hilos y volvería
  // a ser un caño. Con 6 el tronco lleva ~370 y todavía se cuentan de a uno acercándose.
  // 0,52 de radio y 2,6 de separación: un hilo de 0,30 medía MENOS DE UN PÍXEL a distancia de
  // encuadre, así que el antialias lo promediaba contra el fondo negro y el haz entero se veía gris.
  // Es submuestreo, no color — se arregla con hilos más gordos y más juntos, no subiendo el brillo
  // hasta que se lave.
  ...HILOS, ...HEBRA,
  fondo: '#04060e', bloom: 0.80,
  nivelesPenacho: 3, escalaPenacho: 0.62,
  camara: { az: 0.55, el: 0.20, min: 8, max: 3000 },   // sin dist: lo encuadra la caja
});

/* ── la leyenda: quién es cada color, y el conteo que hace auditable el dibujo ─────────────── */
const N = (x) => x.toLocaleString('es');
const c = vista.conteos;
// LO QUE NO ENTRÓ SE DICE. `apretada` marca las bifurcaciones donde el haz padre era más corto que
// lo que sus hijas necesitaban para no tocarse (bit 1) o donde el ángulo que hacía falta pasaba el
// tope (bit 2). Callarlas sería afirmar una separación que el dibujo no tiene.
const apretadas = S.filter((x) => x.apretada).length;
const leyenda = document.createElement('div');
leyenda.className = 'leyenda';
leyenda.innerHTML = `
  <div class="titulo">El núcleo</div>
  <div class="sub">las ramas no se dibujan: son los hilos que pasan por ellas</div>
  ${racimos.slice(0, 8).map((r) => `<div class="r"><i style="background:${r.color}"></i>${
    r.nombre}${r.detalle ? `<em>${r.detalle}</em>` : ''} <b>${N(r.n)}</b></div>`).join('')}
  <div class="pie"><span class="cifra">${N(c.hilos)}</span> hilos en el núcleo · <span
    class="cifra">${N(c.neuronas)}</span> neuronas<br>${N(c.secciones)} haces · ${
    N(c.ramitas)} terminales · ${N(c.botones)} botones<br>${
    N(c.sinapsis)} sinapsis en ${N(c.tramosSinapsis)} tramos${
    c.sinapsisRecortadas ? ` · ${N(c.sinapsisRecortadas)} sin extremo` : ''
    }${apretadas ? `<br><span class="dim">${N(apretadas)} bifurcaciones sin el aire que piden</span>` : ''
    }<br><span class="cifra">${N(c.señalables)}</span> se pueden señalar de uno
    <span class="dim">(los ${N(c.secciones)} halos y las ${N(c.sinapsis)} sinapsis no)</span>
    <br><span class="dim">el impulso sale de elegir un haz, no de un reloj</span></div>`;
document.body.appendChild(leyenda);

console.log('[boceto A]', c, ((performance.now() - t0) | 0) + ' ms');
// Se expone para la verificación por píxeles: pausar y medir necesita poder tocar la escena.
globalThis.__boceto = vista;

// LA MEDICIÓN DEL ENREDO VA A PEDIDO (`?enredo`): son ~90.000 pares por 64 pruebas segmento a
// segmento cada uno, varios segundos. Ponerlo en el camino de carga sería pagar el diagnóstico en
// cada apertura de la página.
if (location.search.includes('enredo')) {
  console.log('[enredo]', medirEnredo(S, { muestras: 8 }));
  console.log('[enredo · margen 2]', medirEnredo(S, { muestras: 8, margen: 2 }));
}
