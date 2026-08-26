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
         FORMAS_IDS, escalaTinta, hashCadena, deHash, crecerDelta, emitirBrote,
         PALETA_CYBER, COLOR_MUSUBI_CYBER, tonoDe } from './comun.mjs';
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
  const tono = tonoDe(globalThis.location ? location.search : '');
  const datos = await cargar(cerebro.archivo);
  // #brotar: la demo del vivo RETIENE las 6 memorias más nuevas — brotarlas sin retenerlas
  // primero las dibujaría dos veces, que es justo lo que el banco prohíbe
  let retenidas = [];
  if (globalThis.location && location.hash === '#brotar' && datos.neurons) {
    const porEdad = [...datos.neurons].sort((x, y2) => (x.age_days || 0) - (y2.age_days || 0));
    retenidas = porEdad.slice(0, 6);
    const fuera = new Set(retenidas.map((m2) => m2.id));
    datos.neurons = datos.neurons.filter((m2) => !fuera.has(m2.id));
  }
  const { raiz, colorDe, racimos } = armarRaiz(datos.neurons, tono === 'cyber'
    ? { titulo: 'memoria', paleta: PALETA_CYBER, colorMusubi: COLOR_MUSUBI_CYBER }
    : { titulo: 'memoria' });

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
    fondo: tono === 'cyber' ? '#05070F' : '#0C1020', bloom: tono === 'cyber' ? 1.05 : 0.80,
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
  barra.innerHTML = FORMAS.map((f) => `<a href="${enlaceCon(`./boceto-${f.id}.html`, cerebro.id, tono)}"${
    f.id === v.id ? ' class="hoy" aria-current="page"' : ''}><b>${f.nombre}</b><span>${
    f.sub}</span></a>`).join('');
  document.body.appendChild(barra);

  // EL CONMUTADOR DE CEREBRO. Va aparte del de formas porque es otro eje: aquél cambia el DIBUJO
  // sobre el mismo dato, éste cambia el DATO con el mismo dibujo. Juntarlos en una barra sola
  // insinuaría que «el central» es una forma más.
  const cbarra = document.createElement('nav');
  cbarra.className = 'cerebros';
  cbarra.innerHTML = CEREBROS.map((c) => `<a href="${enlaceCon(`./boceto-${v.id}.html`, c.id, tono)}"${
    c.id === cerebro.id ? ' class="hoy" aria-current="page"' : ''}>${c.nombre}</a>`).join('')
    + '<span class="sep"></span>'
    + ['sobrio', 'cyber'].map((t) => `<a href="${enlaceCon(`./boceto-${v.id}.html`, cerebro.id, t)}"${
      t === tono ? ' class="hoy" aria-current="page"' : ''}>${t}</a>`).join('');
  document.body.appendChild(cbarra);

  console.log('[boceto ' + v.id + ' · ' + cerebro.id + ']', c, ((performance.now() - t0) | 0) + ' ms');
  // Se expone para la verificación por píxeles: pausar y medir necesita poder tocar la escena.
  globalThis.__boceto = vista;

  /* ── EL VIVO: guardás una nota y la rama brota ─────────────────────────────────────────────
     Sólo para la forma que CRECE (S.estado existe). Cada 45 s se re-pide el MISMO grafo; sin ids
     nuevos no pasa NADA — ni evento, ni rebuild, ni luz. Con ids nuevos, las memorias se
     convierten en atractores (la parcela sale del topic si ya existe, o de la del actor — y un
     atractor viejo NUNCA se recoloca), crecen con crecerDelta (la madera vieja no se mueve, G9),
     y brotan en pantalla con el mismo reloj del replay. Contra un dump estático el diff siempre
     da vacío: el lazo es honesto, no simulado.

     #brotar es la DEMO verificable: retiene las 6 memorias más nuevas de la carga y las suelta a
     los 4 segundos — el mismo camino de código que el vivo, con delta garantizado. */
  if (S.estado) {
    const estado = S.estado;
    const brotarMemorias = (nuevas) => {
      if (!nuevas.length) return null;
      const porActor = new Map();
      for (const m of nuevas) {
        // la parcela: la del topic si ya existe; si el topic es nuevo, la del actor del gist
        let celda = estado.topicCelda.get(m.topic);
        let racimo = null;
        // a qué actor pertenece: si el topic ya se dibuja, su bosque es el del actor que lo
        // dibuja — la MISMA decisión que tomó armarRaiz, leída del estado en vez de re-decidida.
        let bosqueIdx = -1;
        if (celda) {
          for (let bi = 0; bi < estado.bosques.length; bi++) {
            const B = estado.bosques[bi];
            if (B.atrs.some((a2) => a2.mems.some((mm) => mm.topic === m.topic))) { bosqueIdx = bi; racimo = B.racimo; break; }
          }
        }
        if (bosqueIdx < 0) {
          // topic nuevo: al bosque más grande de su... sin autor no hay más dato — al mayor.
          bosqueIdx = 0; racimo = estado.bosques[0] && estado.bosques[0].racimo;
          celda = celda || (estado.racimoInfo.get(racimo) || { celda: [0, 1, 0, Math.PI] }).celda;
        }
        if (!porActor.has(bosqueIdx)) porActor.set(bosqueIdx, []);
        const o2 = estado.opciones || {};
        const R = Number(o2.radio) || 285, piso = Number(o2.piso) || 0.45, cola = Number(o2.cola) || 0.65;
        const h = hashCadena(m.id);
        const cc = (celda[0] + celda[1]) / 2 + (deHash(h, 2) - 0.5) * 0.9 * (celda[1] - celda[0]);
        const ff = (celda[2] + celda[3]) / 2 + (deHash(h, 3) - 0.5) * 0.9 * (celda[3] - celda[2]);
        const r = R * (piso + (1 - piso) * Math.pow(deHash(h, 1), cola));
        const sn = Math.sqrt(Math.max(0, 1 - cc * cc));
        porActor.get(bosqueIdx).push({ id: m.id, mems: [m],
          pos: [Math.cos(ff) * sn * r, cc * r, Math.sin(ff) * sn * r] });
      }
      let tot = { eslabones: 0, botones: 0, sinLugar: 0 };
      for (const [bi, atrs2] of porActor) {
        const B = estado.bosques[bi];
        if (!B) continue;
        const r2 = crecerDelta(B.bosque, atrs2, estado.opciones);
        const br = emitirBrote(B.bosque, atrs2, r2.consumidoPor, r2.nodosNuevos, estado.opciones);
        const res = vista.brotar(br);
        tot.eslabones += res.eslabones; tot.botones += res.botones; tot.sinLugar += res.sinLugar;
        for (const m of nuevas) estado.idsVistos.add(m.id);
      }
      // el brote se DECLARA en la leyenda — lo vivo se cuenta, no se insinúa
      const pie = leyenda.querySelector('.pie');
      if (pie) {
        pie.insertAdjacentHTML('beforeend',
          `<br><span class="cifra">+${N(tot.botones)}</span> brotaron en vivo${
            tot.sinLugar ? ` · <span class="dim">${N(tot.sinLugar)} sin lugar hasta recargar</span>` : ''}`);
      }
      return tot;
    };

    if (location.hash === '#brotar') {
      // demo: las 6 retenidas en la carga brotan a los 4 s (el replay ya terminó)
      setTimeout(() => {
        const res = brotarMemorias(retenidas);
        console.log('[brotar demo]', res);
      }, 4000);
    } else {
      setInterval(async () => {
        try {
          const d2 = await cargar(cerebro.archivo + '?ahora=' + Date.now());
          const nuevas = (d2.neurons || []).filter((m2) => m2 && m2.id && !estado.idsVistos.has(m2.id));
          if (nuevas.length) console.log('[vivo]', brotarMemorias(nuevas));
        } catch (e) { console.warn('[vivo] sin refresco:', e.message); }
      }, 45000);
    }
  }
  return { vista, S, datos, racimos, cerebro };
}
