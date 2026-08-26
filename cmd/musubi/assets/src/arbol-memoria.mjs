// arbol-memoria.mjs — DE LAS MEMORIAS AL ÁRBOL. La memoria ES la rama.
//
// Reemplaza dos cosas que convivían mal: las memorias eran puntos sorteados dentro de una esfera, y
// las dendritas eran decorativas —su forma salía de un PRNG y no decía nada—. Acá la forma del árbol
// SALE DEL DATO: cada bifurcación parte el grupo por algo real y lo declara, y cada punta es una
// memoria. No hay física, no hay esfera, no hay posición al azar.
//
// Vive aparte de quien lo dibuja por la misma razón que layout.mjs: es matemática pura sobre
// números, así que `node --test` la corre en CI. Y le hace falta, porque de acá sale la POSICIÓN de
// cada memoria del panel: un error acá no se ve como un error, se ve como un dibujo distinto.

// ─────────────────────────────────────────────────────────────────────────────────────────────
// LA REGLA DE CORTE
//
// En cada nodo se parte por el SIGUIENTE SEGMENTO DEL TOPIC mientras eso divida de verdad; cuando
// no divide, por TIEMPO. Una sola regla, y el dato eligió que fueran dos criterios y no uno:
//
//   · el tema divide precioso a las personas — davantis: 812 notas en 72 temas —
//   · y no divide NADA donde escribe la máquina: `destilador` tiene 925 notas en UN tema, el libro
//     mayor 775 en dos. Sin el corte por tiempo, esos racimos quedaban como las bolas que este
//     rediseño vino a sacar.
//
// El criterio elegido VIAJA en el nodo (`criterio`), y por eso la rama significa algo: se puede
// decir «esta bifurcación separa `server/` de `gotchas/`» o «separa lo de julio de lo de agosto».

// FRAC_UNITARIOS: si los grupos de UNA sola nota se llevan más de esta fracción de las MEMORIAS, el
// segmento no es una categoría: es un identificador. `design-corpus` da 955 subtemas para 959 notas
// —el 99,6 % de la memoria en grupos de uno— y ahí el tema no agrupa nada.
//
// Se mide sobre las MEMORIAS y no sobre los grupos, y la diferencia importa: un racimo con dos
// temas gordos y cuarenta sueltos tiene el 95 % de sus grupos unitarios y el 91 % de sus memorias
// bien agrupadas. Contando grupos se rechazaba ese corte y se tiraba la única división buena que
// había.
const FRAC_UNITARIOS = 0.6;

/** segmento `d` del topic de una memoria, o '' si no tiene tanta profundidad. */
const seg = (n, d) => String((n && n.topic) || '').split('/').filter(Boolean)[d] || '';

/**
 * cortarPorTema: agrupa por el segmento `d` del topic.
 *
 * @returns {{ok:boolean, grupos?:Array<Array>, etiquetas?:string[], razon?:string}}
 *   La RAZÓN del rechazo importa y por eso viaja:
 *   · `uno-solo`  — todos comparten este segmento. Tiene sentido probar el SIGUIENTE.
 *   · `identificador` / `incompleto` — probar el siguiente CRUZARÍA PADRES: agruparía
 *     `gordo/sub-0` con `medio/sub-0` porque comparten el segundo segmento, y la rama quedaría
 *     rotulada «sub-0» conteniendo dos temas que no tienen nada que ver.
 */
export function cortarPorTema(memorias, d) {
  const por = new Map();
  for (const n of memorias) {
    const k = seg(n, d);
    if (!k) return { ok: false, razon: 'incompleto' };   // alguien no llega a esa profundidad
    if (!por.has(k)) por.set(k, []);
    por.get(k).push(n);
  }
  if (por.size < 2) return { ok: false, razon: 'uno-solo' };
  const claves = [...por.keys()].sort();       // ORDEN ALFABÉTICO: el árbol tiene que ser el mismo en cada recarga
  const grupos = claves.map((k) => por.get(k));
  const sueltas = grupos.reduce((s, g) => s + (g.length === 1 ? 1 : 0), 0);
  if (sueltas > memorias.length * FRAC_UNITARIOS) return { ok: false, razon: 'identificador' };
  return { ok: true, grupos, etiquetas: claves };
}

/**
 * cortarPorTiempo: parte en 2-3 tramos por edad. Es el corte que rescata a los racimos que la
 * máquina escribe, donde el tema es uno solo — y además es honesto: una dendrita que crece con el
 * tiempo tiene el tronco viejo y las puntas nuevas, que es exactamente lo que pasa acá.
 */
export function cortarPorTiempo(memorias) {
  if (memorias.length < 2) return null;
  const orden = [...memorias].sort((a, b) => edad(b) - edad(a) || String(a.id).localeCompare(String(b.id)));
  // Desempate por `id`: sin él, dos notas de la misma edad pueden quedar en cualquier orden y el
  // árbol cambia entre recargas sin que haya cambiado un solo dato.
  if (edad(orden[0]) === edad(orden[orden.length - 1])) return null;   // todas iguales: el tiempo no divide
  const k = orden.length > 240 ? 3 : 2;
  const tam = Math.ceil(orden.length / k);
  const grupos = [];
  for (let i = 0; i < orden.length; i += tam) grupos.push(orden.slice(i, i + tam));
  if (grupos.length < 2) return null;
  return { grupos, etiquetas: grupos.map((g) => rotuloEdad(g)) };
}

const edad = (n) => (typeof (n && n.age_days) === 'number' ? n.age_days : 0);
function rotuloEdad(g) {
  const a = edad(g[0]), b = edad(g[g.length - 1]);
  const dias = (v) => (v >= 365 ? (v / 365).toFixed(1) + ' años' : (v >= 30 ? Math.round(v / 30) + ' meses' : Math.round(v) + ' días'));
  return dias(Math.min(a, b)) + '–' + dias(Math.max(a, b));
}

/**
 * bifurcar: de N grupos a un árbol BINARIO/TERNARIO balanceado por peso.
 *
 * ES LA DIFERENCIA ENTRE «RAMIFICADO» Y «RAMIFICADO COMO EL DIBUJO». Una dendrita real bifurca de a
 * dos o tres; un nodo con 72 hijos —lo que dan los temas de davantis— se ve como un plumero. Acá los
 * grupos se reparten en mitades de peso parecido, recursivamente, así que aparecen niveles
 * intermedios que no existen en el dato pero SÍ respetan su orden: cada corte separa un tramo
 * alfabético (o temporal) de otro, nunca mezcla.
 *
 * @returns {{grupos:Array, etiquetas:string[]}} listo para colgar, con 2-3 hijos como máximo.
 */
export function bifurcar(grupos, etiquetas) {
  const items = grupos.map((g, i) => ({ g, et: etiquetas[i], n: g.length }));
  return armar(items);
}

function armar(items) {
  if (items.length <= 3) return { grupos: items.map((x) => x.g), etiquetas: items.map((x) => x.et), hojas: items };
  // Corte por peso acumulado más cercano a la mitad, SIN reordenar: el orden ya es significativo
  // (alfabético o temporal), y reordenar por tamaño rompería esa lectura.
  const total = items.reduce((s, x) => s + x.n, 0);
  let acum = 0, corte = 0, mejor = Infinity;
  for (let i = 0; i < items.length - 1; i++) {
    acum += items[i].n;
    const d = Math.abs(acum - total / 2);
    if (d < mejor) { mejor = d; corte = i + 1; }
  }
  const izq = armar(items.slice(0, corte)), der = armar(items.slice(corte));
  return { grupos: [izq, der], etiquetas: [rango(items.slice(0, corte)), rango(items.slice(corte))], intermedio: true };
}
const rango = (items) => (items.length === 1 ? items[0].et : items[0].et + '…' + items[items.length - 1].et);

// ─────────────────────────────────────────────────────────────────────────────────────────────
// EL ÁRBOL

/**
 * construirNodo: el esqueleto de un grupo de memorias, recursivo.
 *
 * @returns {{n:number, criterio:string, nivelTema:number, etiqueta:string, hijos:Array, mem:Object|null}}
 *   `criterio` dice POR QUÉ partió ahí: 'tema' | 'tiempo' | 'orden' | 'hoja'. Viaja al tooltip.
 *   `nivelTema` dice QUÉ SEGMENTO del topic separó, cuando el criterio es 'tema'. Sin ese número,
 *   «partió por tema» es ambiguo: un corte en el nivel 0 separa `server/` de `gotchas/`, y uno en
 *   el nivel 1 separa dos subtemas de `server/` — que es una afirmación completamente distinta, y
 *   la primera es falsa para el segundo caso.
 */
export function construirNodo(memorias, d = 0, etiqueta = '') {
  if (memorias.length === 1) return { n: 1, criterio: 'hoja', nivelTema: -1, etiqueta, hijos: [], mem: memorias[0] };

  let corte = null, criterio = '';
  // Se baja de segmento SÓLO mientras la razón sea `uno-solo`, o sea mientras todos compartan el
  // segmento actual. Ahí el siguiente sigue estando dentro del mismo padre y el corte es legítimo.
  // Si el rechazo fue por otra cosa, bajar cruzaría padres y rotularía una rama con un nombre que
  // dos temas distintos comparten.
  for (let k = d; k < d + 4; k++) {
    const c = cortarPorTema(memorias, k);
    if (c.ok) { corte = c; criterio = 'tema'; d = k; break; }
    if (c.razon !== 'uno-solo') break;
  }
  if (!corte) { corte = cortarPorTiempo(memorias); if (corte) criterio = 'tiempo'; }
  if (!corte) {
    // ÚLTIMO RECURSO, y se declara: mismo tema y misma edad. Partir por `id` es arbitrario, pero
    // devolver el grupo entero sería peor — quedaría un nodo con 200 hojas en abanico. Que el
    // criterio diga 'orden' es lo que impide que esto pase por una división con sentido.
    const orden = [...memorias].sort((a, b) => String(a.id).localeCompare(String(b.id)));
    const m = Math.ceil(orden.length / 2);
    corte = { grupos: [orden.slice(0, m), orden.slice(m)], etiquetas: ['', ''] };
    criterio = 'orden';
  }

  const bif = bifurcar(corte.grupos, corte.etiquetas);
  const prof = criterio === 'tema' ? d + 1 : d;
  const hijos = bif.grupos.map((g, i) => (Array.isArray(g)
    ? construirNodo(g, prof, bif.etiquetas[i])
    : anidado(g, prof, bif.etiquetas[i], criterio, criterio === 'tema' ? d : -1)));
  return { n: memorias.length, criterio, nivelTema: criterio === 'tema' ? d : -1, etiqueta, hijos, mem: null };
}

// anidado: un hijo que la bifurcación dejó como sub-árbol intermedio (no es un grupo de memorias
// sino otro reparto). Se aplana a un nodo con el mismo criterio del padre.
// Un nodo 'reparto' HEREDA de qué criterio y de qué nivel viene el corte que lo generó. No es
// cosmetico: es lo unico que permite decir «este brazo separa server/ de gotchas/» cuando entre el
// nodo que decidio y las ramas hay dos o tres niveles de balanceo que el dato no tiene.
function anidado(sub, d, etiqueta, criterioPadre, nivelPadre) {
  const hijos = sub.grupos.map((g, i) => (Array.isArray(g)
    ? construirNodo(g, d, sub.etiquetas[i])
    : anidado(g, d, sub.etiquetas[i], criterioPadre, nivelPadre)));
  return { n: hijos.reduce((s, h) => s + h.n, 0), criterio: 'reparto',
           de: criterioPadre, nivelTema: nivelPadre, etiqueta, hijos, mem: null };
}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// EL CORTE EN NEURONAS

/**
 * cortarNeuronas: baja por el árbol hasta que los subárboles entran en `max`, y esos son las
 * NEURONAS. Después funde las que quedaron por debajo de `min` con su hermana.
 *
 * POR QUÉ SE FUNDEN. Sin esto quedaban 155 neuronas raquíticas de 1-3 memorias entre las 183
 * (medido sobre el central): puntitos sueltos que no se leen como neurona y ensucian el racimo. Y
 * fundirlas NO esconde nada — las memorias siguen todas, en la misma rama de al lado.
 */
export function cortarNeuronas(raiz, min = 30, max = 150) {
  const fuera = [];
  (function bajar(nodo) {
    if (nodo.n <= max || !nodo.hijos.length) { fuera.push(nodo); return; }
    for (const h of nodo.hijos) bajar(h);
  })(raiz);

  // LA FUSIÓN VIVE ACÁ Y EN NINGÚN OTRO LADO. Cada neurona por debajo de `min` se funde con su
  // vecina: la de adelante, o la de atrás si es la última. Se fusiona con la VECINA y no con
  // cualquiera porque el orden es el del árbol, así que una neurona fundida sigue siendo un tramo
  // contiguo del tema o del tiempo — no un cajón de sastre.
  //
  // Antes esto estaba en dos lugares —un bucle y un remate para la última— y con eso el test que
  // custodia la fusión pasaba igual con el bucle sacado: el remate lo tapaba. Mismo modo de falla
  // que el vencimiento del pulso en `impulsos.mjs`.
  const out = fuera.slice();
  for (let i = 0; i < out.length;) {
    if (out.length > 1 && out[i].n < min) {
      const a = i + 1 < out.length ? i : i - 1;      // fundir con la de adelante, o con la de atrás si soy la última
      out.splice(a, 2, fundir(out[a], out[a + 1]));
      i = Math.max(0, a - 1);                        // el resultado puede seguir siendo chico: se vuelve a mirar
    } else i++;
  }
  return out;
}

const fundir = (a, b) => ({
  n: a.n + b.n, criterio: 'fundido', nivelTema: -1, hijos: [a, b], mem: null,
  etiqueta: [a.etiqueta, b.etiqueta].filter(Boolean).join(' + '),
});

// ─────────────────────────────────────────────────────────────────────────────────────────────
// LA GEOMETRÍA

// PRNG semillado. Mismo generador que usaban las dendritas decorativas, para que el cambio de
// origen de la forma no se confunda con un cambio de render. Con Math.random(), dos personas
// mirando la misma pantalla verían árboles distintos y no podrían hablar de lo que ven.
export const rng = (s) => () => ((s = (s * 1664525 + 1013904223) >>> 0) / 4294967296);

const norm = (a) => { const l = Math.hypot(a[0], a[1], a[2]) || 1; return [a[0] / l, a[1] / l, a[2] / l]; };
const cross = (a, b) => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]];
const add = (a, b) => [a[0] + b[0], a[1] + b[1], a[2] + b[2]];
const mul = (a, k) => [a[0] * k, a[1] * k, a[2] * k];

/**
 * horquilla: las direcciones de los hijos de UNA bifurcación.
 *
 * Se abren en un PLANO alrededor de la dirección del padre, no hacia lados cualquiera: eso es lo
 * que se ve como una horquilla y no como un manojo. El plano lo elige el PRNG y rota en cada nivel,
 * así el árbol ocupa volumen en vez de quedar aplastado.
 *
 * El ángulo de cada hijo es INVERSO a su peso: la rama que carga más memorias se desvía menos y
 * queda como continuación del tronco, y la liviana se abre. Es lo que hace una dendrita real, y
 * además es dato: mirando el dibujo se ve por dónde está el grueso de lo escrito.
 */
export function horquilla(dir, pesos, apertura, r) {
  const up = Math.abs(dir[1]) > 0.9 ? [1, 0, 0] : [0, 1, 0];
  const e1 = norm(cross(dir, up)), e2 = norm(cross(dir, e1));
  const giro = r() * Math.PI * 2;
  const eje = norm(add(mul(e1, Math.cos(giro)), mul(e2, Math.sin(giro))));
  const total = pesos.reduce((s, p) => s + p, 0) || 1;
  const n = pesos.length;
  return pesos.map((p, i) => {
    const lado = n === 1 ? 0 : (i / (n - 1)) * 2 - 1;          // -1 … +1
    const ang = apertura * lado * (1 - 0.55 * (p / total));    // el que pesa más se abre menos
    return norm(add(mul(dir, Math.cos(ang)), mul(eje, Math.sin(ang))));
  });
}

/**
 * ladear: hacia qué lado se arquea un tramo, heredando del padre.
 *
 * Devuelve un vector UNITARIO y PERPENDICULAR a `dir`. Se parte del lado del padre, se le saca la
 * parte que ya no es perpendicular (porque el tramo giró) y se lo rota un poco alrededor del eje.
 * Girar poco es lo que hace que la curva PERSISTA: una rama entera se arquea del mismo lado y se va
 * torciendo despacio, en vez de zigzaguear tramo a tramo.
 *
 * Sin padre —o si el padre quedó degenerado, que pasa cuando el giro fue de 180°— se elige un
 * perpendicular con el PRNG. Semillado, como todo acá.
 */
export function ladear(previo, dir, r) {
  const up = Math.abs(dir[1]) > 0.9 ? [1, 0, 0] : [0, 1, 0];
  const e1 = norm(cross(dir, up)), e2 = norm(cross(dir, e1));
  let base;
  if (previo) {
    const proy = previo[0] * dir[0] + previo[1] * dir[1] + previo[2] * dir[2];
    const p = [previo[0] - dir[0] * proy, previo[1] - dir[1] * proy, previo[2] - dir[2] * proy];
    base = Math.hypot(p[0], p[1], p[2]) > 1e-6 ? norm(p) : null;
  }
  if (!base) { const t = r() * Math.PI * 2; base = norm(add(mul(e1, Math.cos(t)), mul(e2, Math.sin(t)))); return base; }
  // rotación chica alrededor de `dir`: la curva se tuerce, no salta
  const ang = (r() - 0.5) * 0.9;
  const lat = norm(cross(dir, base));
  return norm(add(mul(base, Math.cos(ang)), mul(lat, Math.sin(ang))));
}

// EXP_RALL: la ley de Rall — el radio del padre elevado a `e` es la suma de los de los hijos
// elevados a `e`. Es la física de una dendrita real (y la regla de Da Vinci para árboles), y es lo
// que hace que el grosor sea DATO: una rama que carga 200 memorias nace gorda y una que carga 3
// nace fina. Antes el adelgazamiento era un 0,62 fijo por nivel, o sea decoración.
const EXP_RALL = 2.5;
const radioDe = (n, r0) => r0 * Math.pow(Math.max(1, n), 1 / EXP_RALL);

/**
 * colocar: del esqueleto a segmentos en 3D.
 *
 * @returns {{segs:Array, puntas:Array, alcance:number, alcanceRama:number, rSoma:number}}
 *   segs: [{a,b,w0,w1,nivel,dist,criterio,etiqueta,mem}] — `dist` es el camino DESDE EL SOMA a lo
 *   largo de la rama, no en línea recta: es lo que le permite al impulso propagarse como un frente
 *   por el árbol en vez de como una onda esférica.
 *   puntas: [{id,x,y,z,mem}] — una por memoria. Es la posición que el panel usa para dibujarla.
 */
export function colocar(raiz, opciones) {
  const o = opciones || {};
  const centro = o.centro || [0, 0, 0];
  const escala = Math.max(0.001, Number(o.escala) || 1);
  const apertura = Number(o.apertura) || 0.62;
  const r = rng((Number(o.semilla) || 1) >>> 0);
  const curvatura = Number(o.curvatura) === 0 ? 0 : (Number(o.curvatura) || 0.22);
  const r0 = Math.max(0.05, Number(o.radioHoja) || 0.30) * escala;
  const L0 = (Number(o.largo) || 9) * escala;

  const segs = [], puntas = [];
  let alcanceRama = 0, alcance = 0;

  function bajar(nodo, origen, dir, largo, nivel, dist, curvaPadre) {
    if (!nodo.hijos.length) {                      // una MEMORIA: la punta
      puntas.push({ id: nodo.mem && nodo.mem.id, x: origen[0], y: origen[1], z: origen[2], mem: nodo.mem });
      return;
    }
    const pesos = nodo.hijos.map((h) => h.n);
    const dirs = horquilla(dir, pesos, apertura, r);
    nodo.hijos.forEach((h, i) => {
      // El largo baja con la profundidad pero sube con lo que carga la rama: una rama gorda tiene
      // que llegar más lejos o sus hojas se apilan unas encima de otras.
      const l2 = largo * (0.58 + 0.34 * Math.cbrt(h.n / Math.max(1, nodo.n)));
      const fin = add(origen, mul(dirs[i], l2));
      const hasta = dist + l2;
      const idx = segs.length;
      segs.push({
        a: origen, b: fin, w0: radioDe(nodo.n, r0), w1: radioDe(h.n, r0),
        nivel, dist: hasta, criterio: nodo.criterio, etiqueta: h.etiqueta || '',
        mem: h.hijos.length ? null : (h.mem && h.mem.id) || null,
        dir: dirs[i], largo: l2, curva: [0, 0, 0],
      });
      // LA PANZA DEL TRAMO. Se calcula bajando y heredando la del padre, no mirando a los hijos:
      // la primera versión curvaba «hacia donde siguen los hijos» y daba CERO en una bifurcación
      // simétrica —que es el caso normal—, así que sólo se curvaba el 11 % de los tramos.
      //
      // Lo que curva una dendrita real es la TORTUOSIDAD: la rama no va derecho entre dos puntos,
      // se arquea, y el arqueo persiste a lo largo del recorrido en vez de sortearse de nuevo en
      // cada tramo. Por eso la dirección de la panza se hereda y se gira un poco, y no se elige.
      //
      // ESTA ES LA ÚNICA LICENCIA DEL DIBUJO, y se declara: los EXTREMOS de cada tramo salen del
      // dato —qué memoria, en qué rama, con qué grosor—; lo licenciado es el camino que la línea
      // toma ENTRE esos dos puntos. Es una propiedad del trazo, no una afirmación sobre la memoria.
      const bow = ladear(curvaPadre, dirs[i], r);
      segs[idx].curva = mul(bow, l2 * curvatura);
      if (hasta > alcanceRama) alcanceRama = hasta;
      const dd = Math.hypot(fin[0] - centro[0], fin[1] - centro[1], fin[2] - centro[2]);
      if (dd > alcance) alcance = dd;
      bajar(h, fin, dirs[i], l2, nivel + 1, hasta, bow);
    });
  }

  // Las ramas nacen en la SUPERFICIE del soma, no en su centro: arrancando en el centro el primer
  // tramo queda enterrado adentro del cuerpo y la neurona se ve como una bola con palitos saliendo
  // de la nada.
  const rSoma = radioDe(raiz.n, r0) * 1.25;
  const dir0 = norm(o.direccion || [0, 1, 0]);
  bajar(raiz, add(centro, mul(dir0, rSoma)), dir0, L0, 0, rSoma, null);
  return { segs, puntas, alcance: Math.max(alcance, rSoma), alcanceRama: Math.max(alcanceRama, rSoma), rSoma };
}


// ─────────────────────────────────────────────────────────────────────────────────────────────
// LA PUERTA

/**
 * construirRacimo: de las memorias de UN racimo a sus neuronas, con todo puesto.
 *
 * Las neuronas se reparten en una esfera de Fibonacci alrededor del centro del racimo y cada una
 * crece hacia AFUERA: es lo que hace que el borde del racimo sean puntas de rama y no una
 * superficie esférica. La esfera de Fibonacci reparte los SOMAS; lo que se ve en el borde es dónde
 * llegaron las ramas, que es distinto en cada neurona porque depende de cuánto carga.
 *
 * @returns {{neuronas:Array, posiciones:Map, total:number}}
 */
export function construirRacimo(memorias, opciones) {
  const o = opciones || {};
  const centro = o.centro || [0, 0, 0];
  const radio = Math.max(1, Number(o.radio) || 60);
  const min = Number(o.min) || 30, max = Number(o.max) || 150;
  const ms = (memorias || []).filter(Boolean);
  if (!ms.length) return { neuronas: [], posiciones: new Map(), total: 0 };

  const raiz = construirNodo(ms, 0, o.etiqueta || '');
  const cortes = cortarNeuronas(raiz, min, max);

  const neuronas = [], posiciones = new Map();
  let semilla = (Number(o.semilla) || 7) >>> 0, total = 0;
  cortes.forEach((nodo, i) => {
    const k = i + 0.5, phi = Math.acos(1 - 2 * k / cortes.length), th = Math.PI * (1 + Math.sqrt(5)) * k;
    const dir = [Math.cos(th) * Math.sin(phi), Math.cos(phi), Math.sin(th) * Math.sin(phi)];
    // Los somas van en la mitad interior del racimo para que las ramas tengan a dónde crecer sin
    // salirse: el radio del racimo lo ocupa el ÁRBOL, no el reparto de somas. Y el radio crece con
    // la raíz cúbica del índice, o sea VOLUMÉTRICO: en una cáscara los somas quedan todos a la
    // misma distancia y el racimo se lee, otra vez, como una esfera hueca.
    const soma = add(centro, mul(dir, radio * 0.44 * Math.cbrt(k / cortes.length)));
    const g = colocar(nodo, {
      centro: soma, direccion: dir, escala: Number(o.escala) || 1,
      semilla: (semilla += 977), apertura: o.apertura, radioHoja: o.radioHoja, largo: o.largo,
    });
    for (const p of g.puntas) if (p.id != null) posiciones.set(p.id, { x: p.x, y: p.y, z: p.z, neurona: i });
    total += g.segs.length;
    neuronas.push({ i, etiqueta: nodo.etiqueta || '', criterio: nodo.criterio, memorias: nodo.n, centro: soma, ...g });
  });
  return { neuronas, posiciones, total };
}
