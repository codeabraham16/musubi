// forma.mjs — LO QUE LAS CINCO FORMAS COMPARTEN.
//
// Las cinco variantes dibujan EL MISMO DATO con la MISMA maquinaria: las mismas memorias, el mismo
// corte en secciones, el mismo conteo de hilos, las mismas cuñas de `bifurcar`, los mismos
// invariantes. Lo único que cambia es HACIA DÓNDE CRECE EL TEJIDO.
//
// Eso no es un detalle de implementación, es el punto: si cada boceto tuviera su propio pipeline,
// comparar dos sería comparar dos programas y no dos formas — y la pregunta que hay que contestar
// es cuál se lee mejor, no cuál está mejor programado. Acá la única variable es la colocación.
//
// Lo que NO se negocia en ninguna:
//   · hilos(padre) = Σ hilos(hijos) — un axón no aparece ni desaparece en una bifurcación
//   · toda memoria tiene su botón, y lo que no entra se DECLARA en la leyenda
//   · el impulso sale de un acto, no de un reloj
//   · las relaciones están todas las que tienen sus dos extremos dibujados

import { cargar, seccionar, contarFibras, CEREBROS, cerebroDe, enlaceCon,
         FORMAS_IDS, escalaTinta } from './comun.mjs';
import { armarRaiz } from './datos.mjs';
import { montar } from './escena.mjs';

/** Las cinco, en el orden en que se ofrecen. El conmutador sale de acá. */
export const FORMAS = [
  { id: 'a', nombre: 'El núcleo', sub: 'todo converge en un centro' },
  { id: 'b', nombre: 'Las láminas', sub: 'la profundidad es la altura' },
  { id: 'c', nombre: 'El corte', sub: 'una sola lámina, sin oclusión' },
  { id: 'd', nombre: 'La corona', sub: 'el medio vacío, las relaciones lo cruzan' },
  { id: 'e', nombre: 'La corteza', sub: 'las hojas afuera, los tractos adentro' },
  { id: 'f', nombre: 'El nudo', sub: 'la esfera pareja, atada por dentro' },
  { id: 'g', nombre: 'El colonizado', sub: 'la rama CRECE hacia la memoria' },
];

// UN HILO CADA 6 MEMORIAS, y los mismos números para las cinco. Es la única constante libre de
// todo esto y define la densidad: con 1 el tronco tendría 2.267 hilos y sería una pared sólida;
// con 30, ocho hilos y volvería a ser un caño. Que las cinco compartan estos números es lo que
// hace que la comparación sea sobre la FORMA.
export const HILOS = { porMemoria: 6, maxHoja: 22 };
export const HEBRA = { radioHilo: 0.52, separacion: 2.60, largoNeurona: 17, torsion: 0.6 };

const N = (x) => x.toLocaleString('es');

/**
 * construir: el camino completo, de las notas al dibujo.
 *
 * @param {object} v
 *   v.id          'a'..'e'
 *   v.seccionado  opciones de `seccionar`
 *   v.colocar     (S) => void — LO ÚNICO que distingue una forma de otra
 *   v.montaje     opciones extra para `montar`
 *   v.nota        una línea que dice qué está mostrando esta forma
 */
export async function construir(v) {
  const t0 = performance.now();
  // DE QUÉ CEREBRO. Sale de la URL y no de una constante: cambiar de cerebro tiene que costar un
  // clic, o en la práctica se mira uno solo y se opina sobre ése — que es lo mismo que pasaba con
  // las formas antes del conmutador.
  const cerebro = cerebroDe(globalThis.location ? location.search : '');
  const datos = await cargar(cerebro.archivo);
  const { raiz, colorDe, racimos } = armarRaiz(datos.neurons, { titulo: 'memoria' });

  // DOS CAMINOS AL MISMO CONTRATO. Las formas a–f COLOCAN el árbol semántico (seccionar decide
  // la topología, colocar la geometría); el colonizado lo CRECE (la topología emerge del
  // crecimiento y v.formar la emite ya en el contrato). Lo de después no distingue cuál fue.
  let S;
  if (v.formar) {
    S = v.formar(raiz, datos);
  } else {
    S = seccionar(raiz, v.seccionado || { maxNivel: 8, minCarga: 10 });
    // ANTES de colocar, siempre: la separación entre hermanas se calcula sobre el RADIO REAL de
    // sus haces, y ese radio sale de `fibras`. Dos conteos distintos separarían las ramas sobre
    // un grosor que después no se dibuja.
    contarFibras(S, HILOS);
    v.colocar(S);
  }
  contarFibras(S, HILOS);

  // LA TINTA DE LAS RELACIONES SE REPARTE. La forma declara la alfa que quiere para el cerebro de
  // referencia; acá se divide entre las relaciones que este cerebro traiga. Ver `escalaTinta`.
  const tinta = escalaTinta((datos.synapses || []).length);
  const montaje = Object.assign({}, v.montaje || {});
  if (montaje.alfaSinapsis != null) montaje.alfaSinapsis *= tinta;
  if (montaje.alfaConfianza != null) montaje.alfaConfianza *= tinta;

  const vista = montar(Object.assign({
    secciones: S, colorDe, titulo: 'memoria', sinapsis: datos.synapses,
    ...HILOS, ...HEBRA,
    // FONDO AZUL-NOCHE, NO NEGRO PURO. Sale de la marca de Musubi, y no es capricho: sobre negro
    // absoluto los azules oscuros —que acá son un actor entero— se hunden hasta desaparecer, y el
    // panel deja de tener un piso contra el cual medir. #0C1020 da ese piso sin levantar la escena.
    fondo: '#0C1020', bloom: 0.80,
    nivelesPenacho: 3, escalaPenacho: 0.62,
  }, montaje));

  const info = FORMAS.find((f) => f.id === v.id) || FORMAS[0];
  const c = vista.conteos;
  const apretadas = S.filter((x) => x.apretada).length;

  const leyenda = document.createElement('div');
  leyenda.className = 'leyenda';
  // LO QUE NO ENTRÓ SE DICE, en las cinco por igual. `apretada` marca las bifurcaciones donde el
  // haz padre era más corto que lo que sus hijas necesitaban para no tocarse, o donde el ángulo
  // que hacía falta pasaba el tope. Callarlas sería afirmar una separación que el dibujo no tiene.
  leyenda.innerHTML = `
    <div class="titulo">${info.nombre} <em>${cerebro.nombre}</em></div>
    <div class="sub">${v.nota || info.sub}</div>
    ${racimos.slice(0, 8).map((r) => `<div class="r"><i style="background:${r.color}"></i>${
      r.nombre}${r.detalle ? `<em>${r.detalle}</em>` : ''} <b>${N(r.n)}</b></div>`).join('')}
    <div class="pie"><span class="cifra">${N(c.hilos)}</span> hilos en el núcleo · <span
      class="cifra">${N(c.neuronas)}</span> neuronas<br>${N(c.secciones)} haces · ${
      N(c.ramitas)} terminales · ${N(c.botones)} botones<br>${
      N(c.sinapsis)} sinapsis en ${N(c.tramosSinapsis)} tramos${
      tinta < 1 ? ` <span class="dim">al ${Math.round(100 * tinta)} % de tinta</span>` : ''}${
      c.sinapsisRecortadas ? ` · ${N(c.sinapsisRecortadas)} sin extremo` : ''
      }${S.forzados ? ` · ${N(S.forzados)} forzadas al final` : ''
      }${apretadas ? `<br><span class="dim">${N(apretadas)} bifurcaciones sin el aire que piden</span>` : ''
      }<br><span class="cifra">${N(c.señalables)}</span> se pueden señalar de uno
      <span class="dim">(los ${N(c.secciones)} halos y las ${N(c.sinapsis)} sinapsis no)</span></div>`;
  document.body.appendChild(leyenda);

  // EL CONMUTADOR. Comparar cinco formas exige poder saltar entre ellas sin volver a buscar la URL:
  // si cambiar de boceto cuesta, en la práctica se mira uno solo y se opina sobre ése.
  const barra = document.createElement('nav');
  barra.className = 'formas';
  barra.innerHTML = FORMAS.map((f) => `<a href="${enlaceCon(`./boceto-${f.id}.html`, cerebro.id)}"${
    f.id === v.id ? ' class="hoy" aria-current="page"' : ''}><b>${f.nombre}</b><span>${
    f.sub}</span></a>`).join('');
  document.body.appendChild(barra);

  // EL CONMUTADOR DE CEREBRO. Va aparte del de formas porque es otro eje: aquél cambia el DIBUJO
  // sobre el mismo dato, éste cambia el DATO con el mismo dibujo. Juntarlos en una barra sola
  // insinuaría que «el central» es una forma más.
  const cbarra = document.createElement('nav');
  cbarra.className = 'cerebros';
  cbarra.innerHTML = CEREBROS.map((c) => `<a href="${enlaceCon(`./boceto-${v.id}.html`, c.id)}"${
    c.id === cerebro.id ? ' class="hoy" aria-current="page"' : ''}>${c.nombre}</a>`).join('');
  document.body.appendChild(cbarra);

  console.log('[boceto ' + v.id + ' · ' + cerebro.id + ']', c, ((performance.now() - t0) | 0) + ' ms');
  // Se expone para la verificación por píxeles: pausar y medir necesita poder tocar la escena.
  globalThis.__boceto = vista;
  return { vista, S, datos, racimos, cerebro };
}
