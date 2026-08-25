// comun.mjs — LO QUE COMPARTEN LOS DOS BOCETOS.
//
// Bocetos, no producción: viven fuera del bundle del panel y no los toca `npm run build`.
// Se construyen con el MISMO motor (three 0.169 + esbuild vendorizados en ../node_modules) y con
// DATOS REALES del cerebro local, porque un boceto con datos inventados no contesta la pregunta
// que se le hace: si a 2.245 memorias se les ve la jerarquía o no.
//
// ─────────────────────────────────────────────────────────────────────────────────────────────
// LOS TRES RECLAMOS QUE ESTO ATACA, Y CÓMO
//
//   1. «el moverse está muy tosco y es muy difícil moverse entre ramas»
//      TrackballControls no tiene vertical: la escena tumbea y se pierde el norte. Acá hay órbita
//      amortiguada con `up` fijo MÁS navegación por estructura — clic en una sección y la cámara
//      vuela hasta ella; flechas para subir al padre, bajar al hijo, saltar entre hermanas.
//      Moverse «entre ramas» deja de ser puntería con el mouse y pasa a ser recorrer el árbol.
//
//   2. «pusiste todas las ramas juntas y nada más, quiero que haya jerarquía»
//      Antes cada racimo era una bola de arbolitos sueltos repartidos en una esfera: sin tronco
//      común, sin niveles legibles y SIN UN SOLO RÓTULO. Acá hay UN árbol: tronco → racimos →
//      temas → subtemas, con el nombre real del tema escrito sobre las ramas gruesas y el camino
//      hasta la raíz encendido cuando pasás por encima.
//
//   3. «quiero que las mismas ramas sean por secciones neuronas, no que al final estén las
//      neuronas»
//      Éste es el cambio de fondo y está en `seccionar()`. Antes una neurona era un arbolito
//      entero y las memorias colgaban de sus puntas. Ahora CADA TRAMO ENTRE DOS BIFURCACIONES ES
//      UNA NEURONA, y la rama es una CADENA de neuronas que se pasan la señal.
//
// ─────────────────────────────────────────────────────────────────────────────────────────────
// DE DÓNDE SALE LA ANATOMÍA (y por qué no es adorno)
//
// La cadena no es una metáfora forzada: es cómo está hecho un tracto cerebral de verdad.
//
//   · Una vía real —tacto, visión— es una CADENA de neuronas, no una sola: la de primer orden
//     entra, hace sinapsis, la de segundo orden sigue, hace sinapsis, y así. Cada eslabón es una
//     célula completa con su cuerpo. Eso es «la rama por secciones».
//   · El axón mielinizado está DIVIDIDO en internodos separados por los NODOS DE RANVIER, y el
//     potencial de acción SALTA de nodo a nodo (conducción saltatoria). O sea que las secciones
//     no sólo se ven: son la unidad por la que viaja la señal. El impulso del panel puede saltar
//     nodo a nodo y estar diciendo la verdad.
//   · Los BOTONES EN PASO (`boutons en passant`) son engrosamientos a lo largo del axón que hacen
//     sinapsis de paso, sin esperar al final. Por eso las memorias pueden colgar del recorrido y
//     no solamente de la punta.
//   · La HENDIDURA SINÁPTICA es un hueco real: dos neuronas encadenadas no se tocan. Por eso entre
//     el terminal de una sección y el soma de la siguiente queda un espacio — es lo que deja ver
//     dónde termina una neurona y empieza la otra.
//
// Lo único licenciado es el TRAZO (la panza de la curva entre dos puntos). Todo lo que se cuenta
// —cuántas memorias, en qué rama, de quién, qué grosor— sale del dato.

import * as THREE from 'three';

/* ═══════════════════════════════════════════════════════════════════════════════════════════
   PARTE 1 · DEL ÁRBOL A LAS SECCIONES-NEURONA
   ═══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * seccionar: convierte el árbol de memoria en una CADENA DE NEURONAS.
 *
 * EL CAMBIO PEDIDO, en una frase: la neurona ya no es un arbolito con memorias en las puntas —
 * cada tramo entre dos bifurcaciones ES una neurona, y una rama es la cadena de las que la
 * componen.
 *
 * Cada sección devuelta trae, además de su geometría:
 *   · `padre`/`hijos` — la cadena, que es lo que hace navegable el árbol con las flechas
 *   · `nivel`        — la profundidad, que es lo que hace legible la jerarquía
 *   · `etiqueta`     — el segmento REAL del topic («server», «gotchas», «2026-07»)
 *   · `carga`        — cuántas memorias pasan por ella, de donde sale el grosor por ley de Rall
 *   · `nodos`        — cuántos nodos de Ranvier: la señal salta por ellos
 *   · `memorias`     — las que hace sinapsis EN ESTA sección (botones), no en la punta del árbol
 *
 * @param {object} raiz nodo de `construirNodo` (o el compuesto que arma cada boceto)
 * @returns {Array} secciones en orden de recorrido; la 0 es el tronco
 */
export function seccionar(raiz, opciones) {
  const o = opciones || {};
  // `Number(x) || d` TRATA EL CERO COMO AUSENTE, así que `maxNivel: 0` —«no subdividas nada»—
  // se convertía en 7 sin avisar. Lo destapó un test cuyo fixture pedía justamente eso y recibió
  // 301 secciones donde esperaba 1. Es el mismo patrón de siempre: el valor de fallo se disfraza
  // de valor por defecto. Con `Number.isFinite` el 0 es un 0.
  const num = (v, d) => (Number.isFinite(Number(v)) ? Number(v) : d);
  const maxNivel = num(o.maxNivel, 7);
  // minCarga: por debajo de esto la sección no se parte más y absorbe lo que le queda. Sin este
  // tope el árbol baja hasta la memoria suelta y salen miles de secciones de una nota — que es
  // volver a la maraña, sólo que con más geometría. Con 10, el cerebro local da ~250 secciones.
  const minCarga = num(o.minCarga, 10);
  const secciones = [];

  (function bajar(nodo, padre, nivel) {
    // LA SECCIÓN ES EL NODO. Un nodo del árbol de memoria representa «este grupo de memorias, que
    // se parte así»; como neurona representa «este tramo, que carga tantas y se bifurca acá».
    const s = {
      idx: secciones.length,
      padre, hijos: [], nivel,
      carga: nodo.n,
      etiqueta: nodo.etiqueta || '',
      criterio: nodo.criterio || '',
      // Nodos de Ranvier: entre 2 y 9 según lo que carga. En un axón real el largo del internodo
      // crece con el diámetro de la fibra, así que una rama gorda tiene MENOS nodos por unidad de
      // largo y no más. Por eso va en log y se satura: si fuera lineal, el tronco quedaría con
      // cientos de cuentas y se leería como una cadena de bolitas, no como un axón.
      nodos: Math.max(2, Math.min(9, Math.round(2 + Math.log2(Math.max(2, nodo.n)) * 0.9))),
      memorias: [],
      // se completan al colocar
      a: [0, 0, 0], b: [0, 0, 0], curva: [0, 0, 0], w0: 1, w1: 1, dir: [0, 1, 0], largo: 1,
      hoja: false, racimo: nodo.racimo || (padre >= 0 ? secciones[padre].racimo : ''),
    };
    secciones.push(s);
    if (padre >= 0) secciones[padre].hijos.push(s.idx);

    // HOJA. Dos maneras de serlo, y las dos importan:
    //  · el nodo no se parte más (es una memoria suelta o un grupo indivisible)
    //  · o llegamos al tope de niveles: entonces la sección ABSORBE su subárbol completo y lo
    //    declara. Recortar sin declarar el total es el modo de falla que ya nos mordió.
    const hijos = nodo.hijos || [];
    if (!hijos.length || nivel >= maxNivel || nodo.n <= minCarga) {
      s.hoja = true;
      s.memorias = hojasDe(nodo, 120);           // las que hacen sinapsis acá, como botones
      s.absorbidas = nodo.n;
      s.recortado = hijos.length > 0;            // ← se dice, no se esconde
      return;
    }
    for (const h of hijos) bajar(h, s.idx, nivel + 1);
  })(raiz, -1, 0);

  return secciones;
}

/** hojasDe: las memorias que cuelgan de un subárbol, hasta `tope` (declarando si recortó). */
function hojasDe(nodo, tope) {
  const out = [];
  (function bajar(n) {
    if (out.length >= tope) return;
    if (n.mem) { out.push(n.mem); return; }
    for (const h of (n.hijos || [])) bajar(h);
  })(nodo);
  return out;
}

/* ── colocación en el espacio ──────────────────────────────────────────────────────────────── */

const norm = (a) => { const l = Math.hypot(a[0], a[1], a[2]) || 1; return [a[0] / l, a[1] / l, a[2] / l]; };
const cross = (a, b) => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]];
const add = (a, b) => [a[0] + b[0], a[1] + b[1], a[2] + b[2]];
const sub = (a, b) => [a[0] - b[0], a[1] - b[1], a[2] - b[2]];
const mul = (a, k) => [a[0] * k, a[1] * k, a[2] * k];
export const rng = (s) => () => ((s = (s * 1664525 + 1013904223) >>> 0) / 4294967296);

// Ley de Rall: el área de la sección se conserva en cada bifurcación, con exponente 2,5. Es lo que
// hace que el grosor sea DATO y no decoración — una rama que carga 200 memorias nace gorda.
const EXP_RALL = 2.5;
export const radioRall = (n, r0) => r0 * Math.pow(Math.max(1, n), 1 / EXP_RALL);

/**
 * horquilla: reparte `k` direcciones hijas alrededor de `dir`, con el ángulo pesado por la carga.
 * La rama que más carga se desvía MENOS: en una bifurcación real la hija dominante continúa la
 * dirección del padre y la chica es la que se abre. Sin esto, dos hijas de 400 y 3 memorias salen
 * simétricas y la principal deja de leerse como la principal.
 */
export function horquilla(dir, pesos, apertura, r) {
  const d = norm(dir);
  let u = cross(d, [0, 1, 0]);
  if (Math.hypot(u[0], u[1], u[2]) < 0.001) u = cross(d, [1, 0, 0]);
  u = norm(u);
  const v = norm(cross(d, u));
  const tot = pesos.reduce((s, p) => s + Math.max(1, p), 0);
  const fase = r() * Math.PI * 2;
  return pesos.map((p, i) => {
    const frac = Math.max(1, p) / tot;
    const ang = apertura * (1 - frac) * (0.75 + 0.5 * r());
    const th = fase + (Math.PI * 2 * i) / pesos.length;
    const lat = add(mul(u, Math.cos(th)), mul(v, Math.sin(th)));
    return norm(add(mul(d, Math.cos(ang)), mul(lat, Math.sin(ang))));
  });
}

/** ladear: la panza del tramo. Se HEREDA del padre y gira un poco — la tortuosidad de una dendrita
 *  persiste a lo largo del recorrido. Arquear «hacia donde siguen los hijos» da cero en las
 *  bifurcaciones simétricas, que son el caso normal. */
export function ladear(previo, dir, r) {
  const d = norm(dir);
  let base = previo;
  if (!base) {
    let u = cross(d, [0, 1, 0]);
    if (Math.hypot(u[0], u[1], u[2]) < 0.001) u = cross(d, [1, 0, 0]);
    base = norm(u);
  }
  // se le saca la componente paralela: una panza a lo largo del propio tramo no se ve
  const dot = base[0] * d[0] + base[1] * d[1] + base[2] * d[2];
  let perp = norm(sub(base, mul(d, dot)));
  const g = (r() - 0.5) * 0.9;
  const t = norm(cross(d, perp));
  return norm(add(perp, mul(t, g)));
}

/**
 * colocarLibre: coloca las secciones como un árbol orgánico que crece hacia afuera.
 * Es la colocación del BOCETO A. La jerarquía se lee por grosor, largo y rótulo.
 */
export function colocarLibre(secciones, opciones) {
  const o = opciones || {};
  const r = rng((Number(o.semilla) || 11) >>> 0);
  const r0 = Number(o.radioHoja) || 0.34;
  const L0 = Number(o.largo) || 46;
  const apertura = Number(o.apertura) || 0.80;
  const curvatura = Number(o.curvatura) || 0.20;
  const tropismo = Number(o.tropismo) || 0;
  const origen = o.origen || [0, -70, 0];
  const dir0 = norm(o.direccion || [0, 1, 0]);

  const raiz = secciones[0];
  // EL TRONCO VA APARTE del largo base. Con el mismo L0 que las ramas, quedaba un tubo gris de 62
  // unidades por debajo de la primera bifurcación sin nada adentro: la parte más grande y más
  // vacía del cuadro. Es el único tramo que no pertenece a nadie, así que no tiene por qué ocupar
  // como los que sí.
  const LRaiz = Number(o.largoRaiz) || L0 * 0.55;
  raiz.a = origen.slice(); raiz.dir = dir0; raiz.largo = LRaiz;
  raiz.b = add(origen, mul(dir0, LRaiz));
  raiz.w0 = radioRall(raiz.carga, r0) * 1.15; raiz.w1 = radioRall(raiz.carga, r0);
  raiz.curva = [0, 0, 0]; raiz.dist = LRaiz;

  (function bajar(s, curvaPadre) {
    if (!s.hijos.length) return;
    const pesos = s.hijos.map((i) => secciones[i].carga);
    const dirs = horquilla(s.dir, pesos, apertura, r);
    s.hijos.forEach((hi, k) => {
      const h = secciones[hi];
      // El largo baja con la profundidad pero SUBE con lo que carga: una rama gorda tiene que
      // llegar más lejos o sus hijas se apilan una encima de otra.
      // ESTIRÓN DE LOS PRIMEROS NIVELES. Los actores salen todos del mismo punto y con el
      // decaimiento normal quedan cortos, así que sus rótulos —«gio», «davantis», «Musubi»— caen
      // uno encima de otro justo en el arranque, que es la parte del mapa que más importa leer.
      // El estirón es UNIFORME dentro del nivel, así que no cambia el orden relativo de largos:
      // lo que el largo dice sobre la carga se conserva, sólo cambia la escala del nivel.
      const estiron = h.nivel <= 2 ? (h.nivel === 1 ? 1.85 : 1.35) : 1;
      const base = h.nivel === 1 ? L0 : s.largo;
      const l = base * (0.56 + 0.34 * Math.cbrt(h.carga / Math.max(1, s.carga))) * estiron;

      // TROPISMO. Una dendrita real no crece simétrica alrededor de la rama madre: crece hacia su
      // CAMPO, empujada por gradientes químicos, y por eso ocupa un volumen. Sin esto, `horquilla`
      // reparte las hijas en un cono alrededor del padre y el conjunto se abre como abanico: se ve
      // como un árbol de diagrama en 3D, no como tejido llenando un espacio.
      //
      // El empuje es RADIAL —hacia afuera del eje del tronco— y crece con la profundidad: cerca
      // del tronco la rama todavía obedece a la horquilla, y en la periferia ya se abrió al campo.
      // Es una licencia del trazo, como la panza: no afirma nada sobre la memoria, sólo sobre por
      // dónde va la línea entre dos puntos que sí salen del dato.
      let d2 = dirs[k];
      if (tropismo > 0 && h.nivel >= 2) {
        const rx = s.b[0] - origen[0], rz = s.b[2] - origen[2];
        const rl = Math.hypot(rx, rz);
        if (rl > 0.001) {
          const t = tropismo * Math.min(1, (h.nivel - 1) / 4);
          d2 = norm([d2[0] + (rx / rl) * t, d2[1] - t * 0.18, d2[2] + (rz / rl) * t]);
        }
      }
      h.dir = d2; h.largo = l;
      // LA HENDIDURA SINÁPTICA: la hija no nace donde muere el padre. Dos neuronas encadenadas no
      // se tocan, y ese hueco es lo que deja VER dónde termina una sección y empieza la siguiente.
      // Sin él la cadena se lee como un solo tubo largo y el pedido se pierde.
      h.a = add(s.b, mul(d2, l * 0.055));
      h.b = add(h.a, mul(d2, l));
      h.w0 = radioRall(h.carga, r0) * 1.12;
      h.w1 = radioRall(h.carga, r0) * 0.86;
      const bow = ladear(curvaPadre, d2, r);
      h.curva = mul(bow, l * curvatura);
      h.dist = (s.dist || 0) + l;
      bajar(h, bow);
    });
  })(raiz, null);

  return secciones;
}

/**
 * colocarLaminas: coloca las secciones en CAPAS CONCÉNTRICAS por profundidad.
 * Es la colocación del BOCETO B, y la jerarquía deja de ser algo que hay que inferir del grosor:
 * es la posición. Todo lo que está al nivel 2 está en la lámina 2, siempre.
 *
 * Está calcado de dos cosas reales: el ANÁLISIS DE SHOLL —anillos concéntricos alrededor del soma
 * para medir la complejidad de ramificación a cada distancia— y la LAMINACIÓN CORTICAL, las seis
 * capas de la corteza, cada una con su tipo de célula y su conectividad. La profundidad es un eje
 * organizador de verdad en un cerebro, no una convención de diagrama.
 */
export const ALTURA_LAMINA = (nivel, paso) => nivel * paso * 0.95;

export function colocarLaminas(secciones, opciones) {
  const o = opciones || {};
  const r = rng((Number(o.semilla) || 23) >>> 0);
  const r0 = Number(o.radioHoja) || 0.34;
  const paso = Number(o.paso) || 34;          // separación entre láminas
  const r00 = Number(o.radioBase) || 26;
  const curvatura = Number(o.curvatura) || 0.14;

  // Reparto ANGULAR por subárbol: a cada sección se le da una cuña del círculo proporcional a lo
  // que carga, y sus hijas se reparten DENTRO de esa cuña. Es lo que garantiza que dos ramas
  // distintas nunca se crucen de lámina — el problema de «todas las ramas juntas».
  const raiz = secciones[0];
  raiz.a = [0, 0, 0]; raiz.th0 = 0; raiz.th1 = Math.PI * 2;
  raiz.w0 = radioRall(raiz.carga, r0) * 1.15; raiz.w1 = radioRall(raiz.carga, r0);

  const pos = (nivel, th, tilt) => {
    const rad = nivel === 0 ? 0 : r00 + (nivel - 1) * paso;
    // ALTURA POR NIVEL. Con 0,42 —lo primero que probé— la lámina 6 quedaba a 116 de altura contra
    // 264 de radio: un abanico casi plano, que es justo lo que las láminas venían a evitar. Con
    // 0,95 el conjunto es una cúpula escalonada y cada nivel es un anillo a SU altura, legible
    // desde cualquier lado. Se midió mirando el render, no calculándolo.
    return [Math.cos(th) * rad, ALTURA_LAMINA(nivel, paso) + tilt, Math.sin(th) * rad];
  };

  raiz.b = pos(0, 0, 0);
  raiz.a = [0, -paso * 0.8, 0];
  raiz.dist = 0;

  (function bajar(s) {
    if (!s.hijos.length) return;
    const tot = s.hijos.reduce((a, i) => a + Math.max(1, secciones[i].carga), 0);
    let th = s.th0;
    // Un margen dentro de la cuña: sin él las cuñas hermanas se tocan y la separación se pierde.
    const margen = (s.th1 - s.th0) * 0.06;
    const util = (s.th1 - s.th0) - margen;
    s.hijos.forEach((hi) => {
      const h = secciones[hi];
      const w = (Math.max(1, h.carga) / tot) * util;
      h.th0 = th + margen * 0.5; h.th1 = th + margen * 0.5 + w; th += w;
      const thc = (h.th0 + h.th1) * 0.5;
      h.a = s.b.slice();
      h.b = pos(h.nivel, thc, (r() - 0.5) * paso * 0.10);
      h.dir = norm(sub(h.b, h.a));
      h.largo = Math.hypot(...sub(h.b, h.a));
      h.w0 = radioRall(h.carga, r0) * 1.12;
      h.w1 = radioRall(h.carga, r0) * 0.86;
      const bow = ladear(null, h.dir, r);
      h.curva = mul(bow, h.largo * curvatura);
      h.dist = (s.dist || 0) + h.largo;
      bajar(h);
    });
  })(raiz);

  return secciones;
}

/* ═══════════════════════════════════════════════════════════════════════════════════════════
   PARTE 1b · EL NÚCLEO — dejar de ser un árbol
   ═══════════════════════════════════════════════════════════════════════════════════════════ */

/** enCurva: punto sobre la curva de una sección. Los extremos salen del dato; la panza entre
 *  ellos es la única licencia del trazo, y es la MISMA fórmula que usa el shader — si divergieran,
 *  los hilos flotarían al lado de su haz. */
export function enCurva(s, t) {
  const k = Math.sin(t * Math.PI);
  return [
    s.a[0] + (s.b[0] - s.a[0]) * t + s.curva[0] * k,
    s.a[1] + (s.b[1] - s.a[1]) * t + s.curva[1] * k,
    s.a[2] + (s.b[2] - s.a[2]) * t + s.curva[2] * k,
  ];
}

/** marco: base ortonormal perpendicular a `dir`. Es donde se acuestan los hilos del haz. */
export function marco(dir) {
  const d = norm(dir);
  let u = cross(d, [0, 1, 0]);
  if (Math.hypot(u[0], u[1], u[2]) < 0.001) u = cross(d, [1, 0, 0]);
  u = norm(u);
  return [u, norm(cross(d, u))];
}

/**
 * colocarNucleo: los actores salen de un NÚCLEO en todas las direcciones, no de un tronco.
 *
 * EL RECLAMO: «que deje de parecer un árbol». Y no era el trazo — era la topología. Un tronco
 * vertical con la copa arriba tiene suelo y cielo, y eso ES un árbol por más dendrítico que se
 * dibuje cada rama. Un ganglio no tiene arriba: las vías salen del cuerpo en todas direcciones.
 *
 * Las direcciones de primer nivel se reparten con la espiral de Fibonacci sobre la esfera, que es
 * el reparto más separado que existe para k puntos sin privilegiar ningún eje. Con tres actores
 * salen tres brazos a ~109° — la forma de un ganglio, no la de un árbol.
 */
export function colocarNucleo(secciones, opciones) {
  const o = opciones || {};
  const num = (v, d) => (Number.isFinite(Number(v)) ? Number(v) : d);
  const r = rng(num(o.semilla, 11) >>> 0);
  const L0 = num(o.largo, 92);
  const apertura = num(o.apertura, 0.80);
  const curvatura = num(o.curvatura, 0.24);
  const tropismo = num(o.tropismo, 0.38);
  const nucleo = num(o.nucleo, 30);
  const origen = o.origen || [0, 0, 0];
  const fase = num(o.fase, 0.7);

  const raiz = secciones[0];
  // EL NÚCLEO ES UN CUERPO, NO UN TRAMO. Se le da un largo corto y un eje cualquiera: lo que se ve
  // ahí no es un tubo sino el haz completo —todos los hilos juntos— antes de repartirse.
  raiz.dir = norm(o.eje || [0, 1, 0]);
  raiz.largo = nucleo;
  raiz.a = add(origen, mul(raiz.dir, -nucleo * 0.5));
  raiz.b = add(origen, mul(raiz.dir, nucleo * 0.5));
  raiz.curva = [0, 0, 0];
  raiz.dist = nucleo;

  const hijos = raiz.hijos;
  const m = hijos.length || 1;
  hijos.forEach((hi, k) => {
    const h = secciones[hi];
    // FIBONACCI SOBRE LA ESFERA: la altura se reparte pareja en [-1,1] y el ángulo avanza el
    // áureo. Ningún eje queda privilegiado, que es exactamente lo que hay que romper.
    const y = m === 1 ? 0 : 1 - (2 * (k + 0.5)) / m;
    const rad = Math.sqrt(Math.max(0, 1 - y * y));
    const th = k * 2.399963 + fase;
    const d = norm([Math.cos(th) * rad, y, Math.sin(th) * rad]);
    const l = L0 * (0.60 + 0.55 * Math.cbrt(h.carga / Math.max(1, raiz.carga)));
    h.dir = d; h.largo = l;
    // Nace en la SUPERFICIE del núcleo, no en su centro: el haz sale del cuerpo, no lo atraviesa.
    h.a = add(origen, mul(d, nucleo * 0.55));
    h.b = add(h.a, mul(d, l));
    const bow = ladear(null, d, r);
    h.curva = mul(bow, l * curvatura);
    h.dist = nucleo + l;
    bajar(h, bow);
  });

  function bajar(s, curvaPadre) {
    if (!s.hijos.length) return;
    const pesos = s.hijos.map((i) => secciones[i].carga);
    const dirs = horquilla(s.dir, pesos, apertura, r);
    s.hijos.forEach((hi, k) => {
      const h = secciones[hi];
      const l = s.largo * (0.56 + 0.36 * Math.cbrt(h.carga / Math.max(1, s.carga)));
      let d2 = dirs[k];
      // TROPISMO RADIAL EN 3D. En el árbol el empuje era horizontal porque había un eje vertical
      // privilegiado; acá no lo hay, así que el campo empuja hacia afuera del núcleo en la
      // dirección que sea. Es lo que hace que el conjunto llene un volumen y no un abanico.
      if (tropismo > 0 && h.nivel >= 2) {
        const rv = sub(s.b, origen), rl = Math.hypot(rv[0], rv[1], rv[2]);
        if (rl > 0.001) {
          const t = tropismo * Math.min(1, (h.nivel - 1) / 4);
          d2 = norm(add(d2, mul(mul(rv, 1 / rl), t)));
        }
      }
      h.dir = d2; h.largo = l;
      // La hendidura sináptica: la hija no nace donde muere el padre.
      h.a = add(s.b, mul(d2, l * 0.05));
      h.b = add(h.a, mul(d2, l));
      const bow = ladear(curvaPadre, d2, r);
      h.curva = mul(bow, l * curvatura);
      h.dist = (s.dist || 0) + l;
      bajar(h, bow);
    });
  }

  return secciones;
}

/* ═══════════════════════════════════════════════════════════════════════════════════════════
   PARTE 1c · LOS HILOS — «que las ramas estén formadas por hilos de neuronas»
   ═══════════════════════════════════════════════════════════════════════════════════════════

   EL RECLAMO: «se sienten pocas neuronas, las ramas son inventadas». Y tenía razón en las dos
   mitades. Una rama era UN tubo con una textura encima: la geometría no contenía ninguna neurona,
   las dibujaba. Acá la rama no existe como objeto — lo que existe son los HILOS, y la rama es lo
   que se ve cuando pasan muchos juntos.

   ES LO QUE ES UN TRACTO DE VERDAD. El cuerpo calloso no es un tubo: son ~200 millones de axones
   individuales corriendo en paralelo. Un nervio periférico es un haz de fascículos, y cada
   fascículo un haz de fibras. La «rama gruesa» de un cerebro es gruesa PORQUE lleva más fibras.

   Y de ahí sale la conservación, que es lo que hace que el dibujo no pueda mentir:

       hilos(padre) = Σ hilos(hijos)

   Un axón no aparece ni desaparece en una bifurcación: se va con una de las dos ramas. Así que el
   grosor del haz deja de ser una fórmula sobre el dato —la ley de Rall era eso— y pasa a ser el
   dato mismo: contá los hilos del tronco y te da la suma de todas las hojas. */

/**
 * contarFibras: cuántos hilos lleva cada sección, de las hojas hacia arriba.
 *
 * En una hoja el número sale de las memorias que absorbe (un hilo cada `porMemoria`); hacia arriba
 * es la SUMA de las hijas, siempre. `ranura` es dónde arranca cada hija dentro del haz del padre:
 * es lo que hace que el haz se parta en cuñas contiguas al bifurcarse, como un fascículo real, en
 * vez de barajarse.
 */
export function contarFibras(secciones, opciones) {
  const o = opciones || {};
  const num = (v, d) => (Number.isFinite(Number(v)) ? Number(v) : d);
  const porMemoria = Math.max(1, num(o.porMemoria, 6));
  const maxHoja = Math.max(1, num(o.maxHoja, 22));
  // De atrás para adelante: el recorrido es en preorden, así que toda hija tiene índice mayor que
  // su padre y una sola pasada alcanza.
  for (let i = secciones.length - 1; i >= 0; i--) {
    const s = secciones[i];
    if (!s.hijos.length) {
      s.fibras = Math.max(1, Math.min(maxHoja, Math.ceil(s.carga / porMemoria)));
    } else {
      let t = 0;
      for (const h of s.hijos) t += secciones[h].fibras;
      s.fibras = t;                     // ← LA CONSERVACIÓN. No es una estimación: es una suma.
    }
  }
  secciones[0].ranura = 0;
  for (const s of secciones) {
    let off = s.ranura || 0;
    for (const h of s.hijos) { secciones[h].ranura = off; off += secciones[h].fibras; }
  }
  return secciones;
}

/** puntoHilo: dónde está el hilo `(rho, phi0)` de la sección `s` en el parámetro `t`. */
function puntoHilo(s, t, rho, phi0, torsion, e1, e2) {
  const c = enCurva(s, t);
  const ph = phi0 + torsion * t;
  const co = Math.cos(ph) * rho, si = Math.sin(ph) * rho;
  return [c[0] + e1[0] * co + e2[0] * si,
          c[1] + e1[1] * co + e2[1] * si,
          c[2] + e1[2] * co + e2[2] * si];
}

/**
 * enhebrar: convierte las secciones en ESLABONES — una neurona cada uno.
 *
 * Cada eslabón es una célula completa: soma en `a`, axón mielinizado hasta `b`, terminal en `b`, y
 * un hueco antes del próximo (la hendidura sináptica, que es lo que deja VER dónde termina una y
 * empieza la otra). Un hilo es la cadena de eslabones que lo recorren; una rama, el haz de hilos.
 *
 * Los somas del eslabón 0 de cada hilo caen todos a la misma altura del haz, así que se agrupan en
 * un anillo denso al principio de cada sección: eso es un NÚCLEO DE RELEVO, y también es real —
 * los cuerpos celulares de una vía no están desparramados por el tracto, están juntos en el núcleo
 * donde la vía hace relevo.
 *
 * @returns {Array} eslabones {a,b,curva,r,sec,fib,nivel,orden,dist,largo,nodos,ultimo,dir}
 */
export function enhebrar(secciones, opciones) {
  const o = opciones || {};
  const num = (v, d) => (Number.isFinite(Number(v)) ? Number(v) : d);
  const rFib = num(o.radioHilo, 0.30);
  const sep = num(o.separacion, 3.4);            // separación entre hilos, en radios
  const largoN = num(o.largoNeurona, 17);        // largo objetivo de UNA neurona
  const hendidura = Math.min(0.6, Math.max(0.02, num(o.hendidura, 0.17)));
  const torsion = num(o.torsion, 0.6);
  const maxEsl = Math.max(1, num(o.maxEslabones, 7));
  const esl = [];

  for (const s of secciones) {
    const f = Math.max(1, s.fibras || 1);
    const [e1, e2] = marco(s.dir);
    // EL RADIO DEL HAZ SALE DE CUÁNTOS HILOS LLEVA, POR ÁREA. El doble de hilos no es el doble de
    // ancho: es raíz de dos. Es la misma razón por la que un cable de cien pares no es cien veces
    // más gordo que uno de un par — y es lo que reemplaza a la ley de Rall, que estimaba esto.
    const R = f === 1 ? 0 : rFib * sep * Math.sqrt(f);
    s.Rhaz = Math.max(rFib * 1.6, R);
    const nE = Math.max(1, Math.min(maxEsl, Math.round((s.largo || 1) / largoN)));
    const d0 = (s.dist || 0) - (s.largo || 0);
    for (let j = 0; j < f; j++) {
      // Girasol: los hilos LLENAN el disco en vez de formar un anillo hueco. Un anillo se ve como
      // un caño y delata que adentro no hay nada.
      const rho = f === 1 ? 0 : R * Math.sqrt((j + 0.5) / f);
      const phi0 = j * 2.399963;
      const gi = (s.ranura || 0) + j;
      // ESCALONADO POR HILO, y esto se vio en el render antes de entenderlo: con todas las
      // hendiduras alineadas a lo largo del haz, los huecos y los somas caían en el mismo anillo
      // y el haz se leía como una ORUGA segmentada, no como fibras. En un tracto real los nodos y
      // los cuerpos de fibras vecinas no están sincronizados — no hay ninguna razón para que lo
      // estén. Un desfase determinista por hilo devuelve la textura de fibra.
      const desf = ((gi * 2654435761) % 1000) / 1000;
      // El hilo entra y sale de la sección enteros —de 0 a 1— y los cortes de adentro van
      // corridos por el desfase. Los dos eslabones de las puntas quedan cortos por construcción, y
      // si quedan MUY cortos son astillas: una astilla tiene una hendidura por debajo de lo que se
      // ve, así que la cadena se lee como continua justo ahí. Por debajo de medio paso se funden
      // con el vecino. Lo destapó el test, no el ojo: con el recorte a secas la hendidura mínima
      // daba 0,015 — o sea, ninguna.
      const cortes = [0];
      for (let k = 0; k < nE; k++) cortes.push((k + desf) / nE);
      cortes.push(1);
      const minPaso = 0.5 / nE;
      if (cortes.length > 2 && cortes[1] - cortes[0] < minPaso) cortes.splice(1, 1);
      const u = cortes.length;
      if (u > 2 && cortes[u - 1] - cortes[u - 2] < minPaso) cortes.splice(u - 2, 1);
      for (let k = 0; k + 1 < cortes.length; k++) {
        const t0 = cortes[k], t1 = cortes[k + 1];
        const tb = t1 - (t1 - t0) * hendidura;
        const A = puntoHilo(s, t0, rho, phi0, torsion, e1, e2);
        const B = puntoHilo(s, tb, rho, phi0, torsion, e1, e2);
        // La panza del eslabón es la desviación REAL entre la curva del hilo y la cuerda que lo
        // dibuja, medida en el medio. Así el cilindro recto, arqueado por el shader, cae
        // exactamente sobre la hélice — no «parecido».
        const M = puntoHilo(s, (t0 + tb) / 2, rho, phi0, torsion, e1, e2);
        const largo = Math.hypot(B[0] - A[0], B[1] - A[1], B[2] - A[2]) || 0.001;
        // HACIA DÓNDE MIRA ESTE HILO DENTRO DEL HAZ, y para qué sirve: todos los hilos de un haz
        // son cilindros paralelos, así que cada uno recibe la luz igual que su vecino y el
        // conjunto se ve como PANA PLANA en vez de como un cable redondo. Cada hilo tiene volumen
        // y el haz no tiene ninguno. Guardando la normal del hilo respecto del eje del haz se
        // puede sombrear el haz COMO CUERPO —el de arriba iluminado, el de abajo en sombra— que es
        // lo que le devuelve la redondez. `rho/R` completa: el hilo del borde recibe más que el
        // del centro. Se vio en el render; leyendo el código no aparece.
        const phm = phi0 + torsion * ((t0 + tb) / 2);
        const cm = Math.cos(phm), sm = Math.sin(phm);
        const nrad = rho < 1e-6 ? [0, 0, 0]
          : [e1[0] * cm + e2[0] * sm, e1[1] * cm + e2[1] * sm, e1[2] * cm + e2[2] * sm];
        esl.push({
          a: A, b: B,
          curva: [M[0] - (A[0] + B[0]) / 2, M[1] - (A[1] + B[1]) / 2, M[2] - (A[2] + B[2]) / 2],
          r: rFib, sec: s.idx, fib: gi, nivel: s.nivel, orden: k,
          ultimo: k + 2 === cortes.length,
          dist: d0 + (s.largo || 0) * t0, largo,
          nodos: Math.max(2, Math.min(6, Math.round(largo / Math.max(0.6, rFib * 18)))),
          nrad, borde: R > 0 ? rho / R : 1,
          dir: norm(sub(B, A)),
        });
      }
    }
  }
  return esl;
}

/**
 * deshilachar: la arborización terminal, HILO POR HILO.
 *
 * Antes el penacho brotaba de la punta de la sección: uno por rama. Ahora brota de la punta de
 * CADA HILO, que es lo que hace un axón cuando llega a destino — se abre en un ramillete de
 * terminales. Son cientos de penachos en vez de decenas, y ahí es donde se siente el tejido.
 *
 * Van LISAS, sin estrías: las terminales finas no están mielinizadas. El contraste entre la
 * textura estriada del haz y la lisa de las puntas es real, no un efecto.
 */
export function deshilachar(eslabones, secciones, opciones) {
  const o = opciones || {};
  const r = rng((Number(o.semilla) || 97) >>> 0);
  const niveles = Number(o.niveles) || 3;
  const escala = Number(o.escala) || 0.55;
  const ramitas = [];

  for (const e of eslabones) {
    // Sólo del ÚLTIMO eslabón de un hilo que muere en una sección hoja: si brotara de una sección
    // con hijas, la ramita fina se cruzaría con el haz que sale del mismo punto.
    if (!e.ultimo || secciones[e.sec].hijos.length) continue;
    brotar(e, e.b, e.dir, e.largo * escala, e.r, 0);
  }

  function brotar(e, origen, dir, largo, radio, nivel) {
    if (nivel >= niveles || largo < 0.30) return;
    const k = nivel === 0 ? 3 : 2;
    const dirs = horquilla(dir, new Array(k).fill(1), 0.58 + nivel * 0.44, r);
    for (let i = 0; i < k; i++) {
      const l = largo * (0.66 + 0.34 * r());
      const fin = add(origen, mul(dirs[i], l));
      const bow = ladear(null, dirs[i], r);
      ramitas.push({
        a: origen, b: fin, w0: radio, w1: radio * 0.60,
        curva: mul(bow, l * 0.30), seccion: e.sec, fib: e.fib, nivel,
        dist: e.dist + e.largo + l * (nivel + 1),
      });
      brotar(e, fin, dirs[i], l * 0.70, radio * 0.60, nivel + 1);
    }
  }

  return ramitas;
}

/* ═══════════════════════════════════════════════════════════════════════════════════════════
   PARTE 2 · LA CÁMARA QUE NO ES TOSCA
   ═══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * crearCamara: órbita con inercia, zoom hacia el cursor y vuelos con curva de tiempo.
 *
 * ── LA SEGUNDA VUELTA. «La manera de mover el 3D es muy tosca» se dijo DOS VECES, así que la
 * respuesta no puede ser afinar constantes otra vez. Lo que estaba mal eran cuatro cosas de fondo,
 * y cada una se siente distinto:
 *
 *   1. LA AMORTIGUACIÓN DEPENDÍA DEL CUADRO. `x += (meta-x)*0.14` por cuadro significa que a 144 Hz
 *      la cámara llega 2,4× más rápido que a 60, y que cada cuadro largo —y con 5.000 instancias
 *      los hay— produce un tirón. Es *literalmente* movimiento tosco: la mano va parejo y la imagen
 *      no. Ahora el factor sale del TIEMPO: k = 1 - e^(-λ·dt). Mismo movimiento a cualquier tasa.
 *
 *   2. NO HABÍA INERCIA. Soltar el mouse frenaba en seco. En cualquier visor que se sienta bien la
 *      escena sigue girando y desacelera sola: es lo que permite darle un empujón y mirar, en vez
 *      de tener que arrastrar todo el tiempo.
 *
 *   3. EL ZOOM TIRABA AL CENTRO. Acercarse siempre iba al medio de la pantalla, así que había que
 *      corregir con un paneo cada vez. Acá la rueda acerca HACIA EL CURSOR, que es lo que uno cree
 *      que está haciendo cuando gira la rueda mirando algo.
 *
 *   4. EL VUELO ERA UNA PERSECUCIÓN, no un movimiento. Un lerp hacia el objetivo arranca a máxima
 *      velocidad y frena asintótico: sale de golpe y nunca termina de llegar. Acá el vuelo tiene
 *      duración —sacada del tamaño del salto— y curva suave en los dos extremos, así que arranca
 *      despacio, viaja y llega.
 *
 * Lo que se conserva del diseño anterior porque estaba bien: el estado son tres números —azimut,
 * elevación, distancia— y el `up` es siempre +Y. Trackball no tiene concepto de arriba y por eso la
 * escena terminaba tumbada sin manera de enderezarla.
 */
export function crearCamara(camera, dom, opciones) {
  const o = opciones || {};
  const MIN = Number(o.min) || 8, MAX = Number(o.max) || 4000;
  const LIM_EL = 1.45;                 // ~83°: nunca se llega al polo, así que nunca se da vuelta
  const est = {
    az: o.az != null ? o.az : 0.6, el: o.el != null ? o.el : 0.28,
    dist: o.dist || 320, foco: new THREE.Vector3(0, 0, 0),
  };
  const meta = { az: est.az, el: est.el, dist: est.dist, foco: est.foco.clone() };
  const topeEl = (x) => Math.max(-LIM_EL, Math.min(LIM_EL, x));

  let arrastre = null, vuelo = null, reloj = 0;
  let vAz = 0, vEl = 0;                 // velocidad angular, para la inercia
  const _der = new THREE.Vector3(), _arr = new THREE.Vector3(), _z = new THREE.Vector3();

  dom.addEventListener('pointerdown', (ev) => {
    arrastre = { x: ev.clientX, y: ev.clientY, t: 0,
                 pan: ev.button === 1 || ev.button === 2 || ev.shiftKey };
    vuelo = null; vAz = 0; vEl = 0;
    try { dom.setPointerCapture(ev.pointerId); } catch (_) {}
  });
  const soltar = () => {
    // Si la mano se quedó quieta antes de soltar, NO hay inercia: lanzar la escena cuando el
    // usuario ya frenó es de las cosas que más se sienten como que el visor tiene voluntad propia.
    if (arrastre && arrastre.t && ahoraMs() - arrastre.t > 90) { vAz = 0; vEl = 0; }
    arrastre = null;
  };
  dom.addEventListener('pointerup', soltar);
  dom.addEventListener('pointercancel', soltar);
  dom.addEventListener('contextmenu', (ev) => ev.preventDefault());

  const ahoraMs = () => (typeof performance !== 'undefined' ? performance.now() : 0);

  dom.addEventListener('pointermove', (ev) => {
    if (!arrastre) return;
    const dx = ev.clientX - arrastre.x, dy = ev.clientY - arrastre.y;
    arrastre.x = ev.clientX; arrastre.y = ev.clientY;
    vuelo = null;                                // tocar el mouse cancela el vuelo: mandás vos
    if (arrastre.pan) {
      // El paneo va en el PLANO DE LA CÁMARA y escalado por la distancia: si no, cerca se va
      // volando y lejos no se mueve.
      const k = est.dist * 0.0016;
      camera.matrixWorld.extractBasis(_der, _arr, _z);
      meta.foco.addScaledVector(_der, -dx * k).addScaledVector(_arr, dy * k);
      return;
    }
    // LA SENSIBILIDAD SALE DEL ALTO DE LA VENTANA, no de un número por píxel: arrastrar media
    // pantalla tiene que girar lo mismo en un monitor grande que en uno chico. Con radianes por
    // píxel fijos, el mismo gesto gira el doble en 4K y ahí no hay constante que sirva para las dos.
    const s = 2.7 / Math.max(320, dom.clientHeight || 800);
    const daz = -dx * s, del = dy * s;
    meta.az += daz;
    meta.el = topeEl(meta.el + del);
    const t = ahoraMs();
    const dt = Math.max(8, t - (arrastre.t || t)) / 1000;
    arrastre.t = t;
    // Velocidad SUAVIZADA: con la instantánea, un tirón en el último cuadro lanza la escena.
    vAz = vAz * 0.62 + (daz / dt) * 0.38;
    vEl = vEl * 0.62 + (del / dt) * 0.38;
  });

  dom.addEventListener('wheel', (ev) => {
    ev.preventDefault(); vuelo = null;
    const antes = meta.dist;
    // Zoom MULTIPLICATIVO: un paso de rueda cambia la distancia en un porcentaje, así que acercarse
    // se siente igual de fino cerca que lejos. En lineal, cerca da saltos y lejos no avanza.
    meta.dist = Math.max(MIN, Math.min(MAX, meta.dist * Math.exp(ev.deltaY * 0.0011)));
    // ZOOM HACIA EL CURSOR. Al acercarse `d`, el punto bajo el puntero tiene que quedarse donde
    // está: eso se consigue corriendo el foco en el plano de la cámara la fracción de pantalla que
    // el cursor está fuera del centro, por lo que se acortó la distancia.
    const rc = dom.getBoundingClientRect ? dom.getBoundingClientRect() : { left: 0, top: 0, width: 1, height: 1 };
    const nx = ((ev.clientX - rc.left) / Math.max(1, rc.width)) * 2 - 1;
    const ny = -((((ev.clientY - rc.top) / Math.max(1, rc.height))) * 2 - 1);
    const tan = Math.tan(((camera.fov || 50) * Math.PI / 180) / 2);
    const d = antes - meta.dist;
    camera.matrixWorld.extractBasis(_der, _arr, _z);
    meta.foco.addScaledVector(_der, nx * tan * (camera.aspect || 1) * d)
             .addScaledVector(_arr, ny * tan * d);
  }, { passive: false });

  /** suave: cúbica in-out. Arranca de cero y llega a cero — un vuelo con velocidad en los extremos
   *  se ve como un corte, y un lerp asintótico nunca termina de llegar. */
  const suave = (u) => (u < 0.5 ? 4 * u * u * u : 1 - Math.pow(-2 * u + 2, 3) / 2);

  /** volarA: encuadra un punto con una holgura dada. Es lo que hace el clic en una sección. */
  function volarA(p, holgura, azel) {
    const dest = {
      az: azel ? azel[0] : meta.az, el: azel ? topeEl(azel[1]) : meta.el,
      dist: Math.max(MIN, Math.min(MAX, holgura)),
      foco: new THREE.Vector3(p[0], p[1], p[2]),
    };
    const desde = { az: est.az, el: est.el, dist: Math.max(0.001, est.dist), foco: est.foco.clone() };
    // CAMINO CORTO EN EL AZIMUT: sin esto, ir de 350° a 10° da la vuelta entera por el otro lado y
    // se ve como si la escena se hubiera desbocado.
    dest.az = desde.az + Math.atan2(Math.sin(dest.az - desde.az), Math.cos(dest.az - desde.az));
    // LA DURACIÓN SALE DEL SALTO. Un tiempo fijo hace que el salto corto se sienta lento y el largo
    // atropellado; se mide en «cuántas pantallas hay que cruzar», que es lo que percibe el ojo.
    const salto = desde.foco.distanceTo(dest.foco) / Math.max(1, dest.dist)
                + Math.abs(Math.log(dest.dist / desde.dist));
    vuelo = { desde, dest, t: 0, dur: Math.min(1.25, 0.32 + salto * 0.42) };
    meta.az = dest.az; meta.el = dest.el; meta.dist = dest.dist; meta.foco.copy(dest.foco);
    vAz = 0; vEl = 0;
  }

  /**
   * tick: un paso. `dt` en segundos.
   *
   * SE PUEDE PASAR EL dt A PROPÓSITO, y no es un detalle de comodidad: la prueba de navegación
   * corre en headless, donde el reloj casi no avanza entre llamadas. Si el dt saliera siempre del
   * reloj, 120 tick() en un bucle cerrado moverían la cámara cero y el arnés roto se leería igual
   * que la navegación rota. Ya nos pasó con rAF; no se repite.
   */
  function tick(dtDado) {
    let dt = Number(dtDado);
    if (!Number.isFinite(dt) || dt <= 0) {
      const t = ahoraMs() * 0.001;
      dt = reloj ? Math.min(0.05, t - reloj) : 1 / 60;
      reloj = t;
      if (dt <= 0) dt = 1 / 60;
    }

    if (vuelo) {
      vuelo.t = Math.min(1, vuelo.t + dt / vuelo.dur);
      const u = suave(vuelo.t);
      est.az = vuelo.desde.az + (vuelo.dest.az - vuelo.desde.az) * u;
      est.el = vuelo.desde.el + (vuelo.dest.el - vuelo.desde.el) * u;
      // La distancia interpola en GEOMÉTRICO. En lineal, un vuelo de 2000 a 40 pasa el 90 % del
      // tiempo lejísimos y llega de un tirón: el ojo percibe el zoom como proporción, no como resta.
      est.dist = vuelo.desde.dist * Math.pow(vuelo.dest.dist / vuelo.desde.dist, u);
      est.foco.lerpVectors(vuelo.desde.foco, vuelo.dest.foco, u);
      if (vuelo.t >= 1) vuelo = null;
    } else {
      if (!arrastre && (vAz || vEl)) {
        meta.az += vAz * dt;
        meta.el = topeEl(meta.el + vEl * dt);
        const fr = Math.exp(-4.2 * dt);          // frenado exponencial: se detiene sola, sin corte
        vAz *= fr; vEl *= fr;
        if (Math.abs(vAz) < 2e-4) vAz = 0;
        if (Math.abs(vEl) < 2e-4) vEl = 0;
      }
      // ← EL ARREGLO DE FONDO: el factor sale del tiempo, no del cuadro.
      const k = 1 - Math.exp(-15 * dt);
      est.az += (meta.az - est.az) * k;
      est.el += (meta.el - est.el) * k;
      est.dist = Math.max(0.001, est.dist) * Math.pow(meta.dist / Math.max(0.001, est.dist), k);
      est.foco.lerp(meta.foco, k);
    }

    const ce = Math.cos(est.el), se = Math.sin(est.el);
    camera.position.set(
      est.foco.x + Math.sin(est.az) * ce * est.dist,
      est.foco.y + se * est.dist,
      est.foco.z + Math.cos(est.az) * ce * est.dist);
    camera.up.set(0, 1, 0);                       // ← el horizonte, que es lo que faltaba
    camera.lookAt(est.foco);
  }

  return { est, meta, tick, volarA, get volando() { return vuelo != null; } };
}

/* ═══════════════════════════════════════════════════════════════════════════════════════════
   PARTE 3 · RÓTULOS Y MIGAS — «que haya jerarquía» es, en buena parte, que esté escrito
   ═══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * crearRotulos: nombres proyectados sobre las ramas gruesas.
 *
 * Ésta es la mitad barata del reclamo de jerarquía y conviene decirlo: el árbol de antes YA tenía
 * niveles, pero sin un solo rótulo se veía como una maraña. Escribir el segmento real del topic
 * sobre las ramas de los primeros niveles convierte el dibujo en un mapa que se puede leer.
 *
 * Son divs, no sprites: el texto queda nítido a cualquier zoom sin atlas de glifos, y son 30-60
 * elementos, no miles.
 */
export function crearRotulos(host) {
  const capa = document.createElement('div');
  capa.style.cssText = 'position:fixed;inset:0;pointer-events:none;z-index:5;overflow:hidden';
  host.appendChild(capa);
  const vivos = [];
  const _v = new THREE.Vector3();

  function set(items) {
    while (vivos.length > items.length) capa.removeChild(vivos.pop().el);
    while (vivos.length < items.length) {
      const el = document.createElement('div');
      el.style.cssText = 'position:absolute;transform:translate(-50%,-50%);white-space:nowrap;'
        + 'font:600 11px/1.1 ui-sans-serif,system-ui,sans-serif;letter-spacing:.04em;'
        + 'text-shadow:0 1px 3px #000,0 0 10px #000;transition:opacity .18s';
      capa.appendChild(el); vivos.push({ el });
    }
    items.forEach((it, i) => {
      const v = vivos[i]; v.p = it.p; v.rango = it.rango || 1e9;
      v.prio = it.prio || 0;
      v.imprescindible = !!it.imprescindible;
      v.el.textContent = it.texto;
      v.el.style.color = it.color || '#dbe6f5';
      v.el.style.fontSize = (it.tam || 11) + 'px';
      v.ancho = it.texto.length * (it.tam || 11) * 0.56;
    });
    // Una sola vez: se ordena por prioridad para que el desapilado de abajo sea determinista y
    // siempre gane el rótulo más importante, no el que la iteración tocó primero.
    vivos.sort((a, b) => (b.prio || 0) - (a.prio || 0));
  }

  // DESAPILADO. Sin esto los rótulos de los primeros niveles se amontonan en el arranque del árbol
  // —«libro mayor», «davantis» y «(sin atribuir)» encimados— y el mapa se vuelve ilegible justo en
  // la parte que más importa. Es codicioso y por prioridad: se coloca el más importante y el que
  // caiga sobre uno ya puesto se calla. Son ~60 rótulos: el O(n²) no se siente.
  const puestos = [];
  function tick(camera, dist) {
    puestos.length = 0;
    for (const v of vivos) {
      if (!v.p) { v.el.style.opacity = 0; continue; }
      _v.set(v.p[0], v.p[1], v.p[2]).project(camera);
      // Se ocultan los de atrás de la cámara y los que quedan fuera de su rango de zoom: mostrar
      // los rótulos de nivel 5 desde lejos es volver a la maraña, pero de texto.
      if (_v.z >= 1 || dist >= v.rango) { v.el.style.opacity = 0; continue; }
      const x = (_v.x * 0.5 + 0.5) * innerWidth;
      let y = (-_v.y * 0.5 + 0.5) * innerHeight;
      const choca = (yy) => puestos.some((q) =>
        Math.abs(yy - q.y) < 17 && Math.abs(x - q.x) < (v.ancho + q.w) * 0.5 + 8);
      if (choca(y)) {
        // UN RÓTULO IMPRESCINDIBLE NO SE ESCONDE: SE CORRE. Los actores («gio», «davantis»,
        // «Musubi») salen todos del mismo punto, así que dos de ellos colisionan casi siempre — y
        // con el desapilado a secas desaparecía uno, que en la práctica es afirmar que ese actor
        // no está. Se prueban desplazamientos verticales crecientes y sólo si NINGUNO entra se
        // calla; para los rótulos de tema, que son decenas, esconder sigue siendo lo correcto.
        let libre = null;
        if (v.imprescindible) {
          for (let dy = 18; dy <= 90 && libre === null; dy += 18) {
            if (!choca(y - dy)) libre = y - dy;
            else if (!choca(y + dy)) libre = y + dy;
          }
        }
        if (libre === null) { v.el.style.opacity = 0; continue; }
        y = libre;
      }
      puestos.push({ x, y, w: v.ancho });
      v.el.style.opacity = 1;
      v.el.style.left = x + 'px';
      v.el.style.top = y + 'px';
    }
  }
  return { set, tick, capa };
}

/* ═══════════════════════════════════════════════════════════════════════════════════════════
   PARTE 4 · DATOS REALES
   ═══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * frenteEn: dónde está el frente del impulso `ms` después de salir, o -1 si ya pasó.
 *
 * Vive acá, en la parte pura, por una razón concreta: **el invariante que importa del impulso es
 * que SE APAGA**, y hay que poder verlo fallar bajo sabotaje. Adentro del bucle de render sólo se
 * puede probar con un navegador completo, y en headless con --virtual-time-budget no corre ni un
 * cuadro — o sea que el sabotaje no se distinguiría del arnés roto. Acá lo corre `node --test`.
 *
 * Sin el apagado, la luz se queda prendida para siempre y el panel deja de poder decir «acá no
 * está pasando nada», que es la mitad de lo que dice.
 *
 * @param {number} ms   milisegundos desde que salió
 * @param {number} vel  unidades por segundo
 * @param {number} max  distancia del destino
 * @param {number} ancho ancho del frente
 * @returns {number} la distancia del frente, o -1 si ya terminó
 */
export function frenteEn(ms, vel, max, ancho) {
  const f = (Math.max(0, ms) / 1000) * vel;
  // Se apaga al pasarse del destino MÁS el ancho del frente: cortarlo justo en el destino deja
  // media onda dibujada y se ve como un parpadeo.
  return f > max + ancho ? -1 : f;
}

/** cargar: el grafo del cerebro local, tal cual lo sirve el panel. */
export async function cargar(url) {
  const r = await fetch(url || './grafo-local.json');
  if (!r.ok) throw new Error('no se pudo leer el grafo: ' + r.status);
  return r.json();
}

// PALETA DE PERSONAS, ordenada por SEPARACIÓN DE TONO y no por la del panel.
//
// El primer intento reusó el orden del panel: gio quedó turquesa (172°) y davantis violeta (258°),
// con Musubi en índigo (232°) en el medio. Medido en el render: davantis y Musubi se leían como el
// mismo color, y como Musubi es el 63 % de la memoria, davantis desaparecía adentro. Acá los tres
// actores están a ~90° de tono unos de otros, que es lo que hace falta para distinguirlos cuando
// además hay bloom encima lavando la saturación.
export const PALETA = ['#2dd4bf', '#f472b6', '#fbbf24', '#4ade80', '#a78bfa', '#22d3ee',
                       '#fb923c', '#f87171', '#a3e635', '#e879f9', '#38bdf8', '#facc15'];

// MUSUBI tiene color propio, y esa es la corrección: antes sus 1.437 notas iban a dos grises que
// se leían como «archivo muerto» y «nota huérfana». Azul, lejos del turquesa de gio y del rosa de
// davantis — Musubi es un actor, pero no es una persona, y la diferencia tiene que verse.
export const COLOR_MUSUBI = '#4d8df5';
// EL NUCLEO: lo unico que no pertenece a nadie. Antes iba gris apagado a proposito —para no darle
// color de actor— y con la forma nueva eso lo convirtio en un grumo sucio justo en el centro del
// cuadro, que es donde converge todo. Plata fria: sigue sin ser el color de nadie, pero se lee
// como el cuerpo del que salen las vias en vez de como una mancha.
export const COLOR_TRONCO = '#7d90ae';
