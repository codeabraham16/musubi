// boceto-b.mjs — «LAS LÁMINAS». La misma cadena de neuronas, pero la profundidad ES la posición.
//
// LA APUESTA CONTRARIA A LA DEL BOCETO A: en A la jerarquía se lee por grosor, rótulo y camino
// encendido, pero hay que leerla. Acá no hay nada que leer — todo lo que está al nivel 3 está en
// la lámina 3, siempre, y una rama nunca puede cruzarse con otra porque cada subárbol tiene su
// CUÑA angular y sus hijas se reparten adentro de esa cuña.
//
// No es una convención de diagrama: la profundidad es un eje organizador de verdad en un cerebro.
//
//   · ANÁLISIS DE SHOLL — el método clásico para medir la complejidad de un arbor dendrítico es
//     contar cuántas veces lo cruzan ANILLOS CONCÉNTRICOS alrededor del soma. Los anillos tenues
//     que se ven acá son eso, y de paso sirven de regla: se puede comparar a ojo cuánto ramifica
//     un racimo contra otro a la misma distancia.
//   · LAMINACIÓN CORTICAL — la corteza tiene seis capas, cada una con su tipo de célula y su
//     conectividad. Que la profundidad sea una capa es cómo está armada la corteza de verdad.
//
// LO QUE SE PAGA, y hay que decirlo: pierde lo orgánico. Se parece más a un tejido cultivado en
// placa que a tejido vivo. A cambio, la pregunta «¿de qué cuelga esto?» se contesta sin buscar.

import { cargar, seccionar, colocarLaminas, ALTURA_LAMINA } from './comun.mjs';
import { armarRaiz } from './datos.mjs';
import { montar } from './escena.mjs';

const t0 = performance.now();
const datos = await cargar('./grafo-local.json');
const { raiz, colorDe, racimos } = armarRaiz(datos.neurons, { titulo: 'memoria' });

const S = seccionar(raiz, { maxNivel: 6, minCarga: 12 });
colocarLaminas(S, { paso: 46, radioBase: 34, curvatura: 0.13, radioHoja: 0.30, semilla: 23 });

/* ── los anillos de Sholl: la regla contra la que se lee la profundidad ─────────────────────── */
function anillos(mundo, secciones, THREE) {
  let maxNivel = 0; for (const s of secciones) if (s.nivel > maxNivel) maxNivel = s.nivel;
  for (let nv = 1; nv <= maxNivel; nv++) {
    const rad = 34 + (nv - 1) * 46;
    const g = new THREE.RingGeometry(rad - 0.22, rad + 0.22, 160);
    const m = new THREE.MeshBasicMaterial({
      // Muy tenues a propósito: son una regla, no un elemento del dibujo. Si compiten con las
      // ramas, el boceto pasa a ser sobre los anillos.
      color: 0x9fb4d4, transparent: true, opacity: 0.16,
      side: THREE.DoubleSide, depthWrite: false,
    });
    const o = new THREE.Mesh(g, m);
    o.rotation.x = -Math.PI / 2;
    o.position.y = ALTURA_LAMINA(nv, 46);   // MISMA formula que las secciones, o el anillo miente
    mundo.add(o);
  }
}

const vista = montar({
  secciones: S, colorDe, titulo: 'memoria', ornamento: anillos,
  fondo: '#05070e', niebla: 0.0009,
  camara: { az: 0.7, el: 0.40, min: 14, max: 2600 },    // sin dist: lo encuadra la caja
});

/* ── el selector de lámina: «moverse entre ramas» sin tocar el mouse ───────────────────────── */
let maxNivel = 0; for (const s of S) if (s.nivel > maxNivel) maxNivel = s.nivel;
const porNivel = [];
for (let nv = 0; nv <= maxNivel; nv++) porNivel.push(S.filter((s) => s.nivel === nv));

const sel = document.createElement('div');
sel.className = 'laminas';
sel.innerHTML = `<div class="t">lámina</div>` + porNivel.map((ss, nv) =>
  `<button data-nv="${nv}">${nv}<span>${ss.length}</span></button>`).join('');
document.body.appendChild(sel);
sel.querySelectorAll('button').forEach((b) => b.addEventListener('click', () => {
  const nv = Number(b.dataset.nv);
  // Al elegir una lámina se salta a la sección MÁS CARGADA de ese nivel, no a la primera: la
  // primera es un accidente del orden alfabético, y aterrizar en una rama de 11 memorias hace
  // parecer que el nivel entero está vacío.
  const ss = porNivel[nv]; if (!ss.length) return;
  let mejor = ss[0]; for (const s of ss) if (s.carga > mejor.carga) mejor = s;
  vista.elegir(mejor.idx);
}));

const leyenda = document.createElement('div');
leyenda.className = 'leyenda';
leyenda.innerHTML = `
  <div class="titulo">Boceto B · Las láminas</div>
  <div class="sub">la profundidad es la posición: nivel N ⇒ lámina N</div>
  ${racimos.slice(0, 7).map((r) => `<div class="r"><i style="background:${r.color}"></i>${
    r.nombre} <b>${r.n.toLocaleString('es')}</b></div>`).join('')}
  <div class="pie">${datos.neurons.length.toLocaleString('es')} memorias · ${
    vista.conteos.secciones} secciones-neurona · ${vista.conteos.nodos} nodos de Ranvier · ${
    vista.conteos.botones.toLocaleString('es')} botones</div>`;
document.body.appendChild(leyenda);

console.log('[boceto B]', vista.conteos, ((performance.now() - t0) | 0) + ' ms');
globalThis.__boceto = vista;
