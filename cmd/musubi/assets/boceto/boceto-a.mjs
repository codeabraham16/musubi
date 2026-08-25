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

import { cargar, seccionar, colocarNucleo } from './comun.mjs';
import { armarRaiz } from './datos.mjs';
import { montar } from './escena.mjs';

const t0 = performance.now();
const datos = await cargar('./grafo-local.json');
const { raiz, colorDe, racimos } = armarRaiz(datos.neurons, { titulo: 'memoria' });

// maxNivel 8 y minCarga 10: medido sobre el cerebro local, da ~440 secciones. Con minCarga 1 el
// árbol baja hasta la nota suelta y salen miles de tramos de una memoria — la maraña otra vez,
// pero con más geometría. El tope se DECLARA en la ficha de cada sección hoja («absorbe N»).
const S = seccionar(raiz, { maxNivel: 8, minCarga: 10 });
// ── LOS NÚMEROS SALIERON DE MEDIR, NO DE PROBAR HASTA QUE GUSTÓ ────────────────────────────────
// «Que no se amontone» se midió: se proyectan las 441 secciones desde 12 puntos de vista sobre una
// grilla de 6 px y se cuenta qué fracción de las celdas ocupadas tiene DOS O MÁS secciones encima.
// Ese número, y no una impresión, es lo que bajó de 36,0 % a 23,8 %.
//
// Y el barrido refutó dos cosas que yo daba por obvias:
//   · EL TROPISMO EMPEORA. Subirlo de 0,46 a 1,10 llevó el solape de 36 % a 41 %: el empuje radial
//     manda todo a una cáscara, y una cáscara se proyecta sobre sí misma. Va en CERO.
//   · MENOS CURVATURA, NO MÁS. La panza cruza ramas vecinas; 0,34 → 0,12 saca 2,5 puntos.
// Lo que sí ayuda es lo aburrido: abrir la horquilla y estirar los tramos, o sea darles AIRE.
colocarNucleo(S, {
  origen: [0, 0, 0], nucleo: 40, largo: 150,
  apertura: 1.40, curvatura: 0.12, tropismo: 0, semilla: 11,
});

const vista = montar({
  secciones: S, colorDe, titulo: 'memoria',
  sinapsis: datos.synapses,
  // UN HILO CADA 6 MEMORIAS. Es la única constante libre de todo esto y define la densidad: con 1
  // el tronco tendría 2.267 hilos y se vería como una pared sólida; con 30, ocho hilos y volvería
  // a ser un caño. Con 6 el tronco lleva ~370 y todavía se cuentan de a uno acercándose.
  porMemoria: 6, maxHoja: 22,
  // 0,40 de radio y 3,05 de separación, y no 0,30/3,40: un hilo de 0,30 mide MENOS DE UN PÍXEL a
  // distancia de encuadre, así que el antialias lo promedia contra el fondo negro y el haz entero
  // se ve gris apagado. Es submuestreo, no color: se arregla con hilos más gordos y más juntos,
  // no subiendo el brillo hasta que se lave. Medido mirando el render.
  // 0,52 de radio y 2,6 de separación. Estirar la escena baja el solape pero la cámara se aleja en
  // proporción y el hilo queda MÁS FINO en pantalla: se gana separación y se pierde el hilo, que es
  // lo único que este boceto vino a mostrar. Midiendo las dos cosas a la vez aparece un punto que
  // mejora las DOS: 25,5 % de solape (contra 36 %) con hilos de 0,68 px (contra 0,62). No es un
  // empate elegido a ojo — es el único punto del barrido que no cambia una cosa por la otra.
  radioHilo: 0.52, separacion: 2.60, largoNeurona: 17, torsion: 0.6,
  fondo: '#04060e', niebla: 0.0011, bloom: 0.80,
  nivelesPenacho: 3, escalaPenacho: 0.62,
  camara: { az: 0.55, el: 0.20, min: 8, max: 3000 },   // sin dist: lo encuadra la caja
});

/* ── la leyenda: quién es cada color, y el conteo que hace auditable el dibujo ─────────────── */
const N = (x) => x.toLocaleString('es');
const c = vista.conteos;
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
    N(c.sinapsis)} sinapsis${c.sinapsisRecortadas ? ` · ${N(c.sinapsisRecortadas)} sin extremo` : ''
    }<br><span class="cifra">${N(c.señalables)}</span> se pueden señalar de uno
    <span class="dim">(los ${N(c.secciones)} halos y las ${N(c.sinapsis)} sinapsis no)</span>
    <br><span class="dim">el impulso sale de elegir un haz, no de un reloj</span></div>`;
document.body.appendChild(leyenda);

console.log('[boceto A]', c, ((performance.now() - t0) | 0) + ' ms');
// Se expone para la verificación por píxeles: pausar y medir necesita poder tocar la escena.
globalThis.__boceto = vista;
