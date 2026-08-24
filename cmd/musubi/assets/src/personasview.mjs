// personasview.mjs — dibuja la lente PERSONAS: una neurona con dendritas por terminal, los
// despachos como axones dirigidos, y un racimo por persona.
//
// POR QUÉ CANVAS 2D Y NO three.js, teniendo three.js ya en el bundle: lo que hace que esto se
// lea como una dendrita y no como un alambre es el trazo que ADELGAZA hacia la punta. En canvas
// 2D es una propiedad del stroke; en three.js hay que construir geometría por segmento. Para
// ~4 000 segmentos, canvas 2D es menos código, menos memoria y se ve mejor.
//
// El render vive aparte de personas.mjs a propósito: allá es lógica pura que corre en
// `node --test`, acá hay DOM. Mezclarlos deja la lógica sin poder testear.
//
// NADA DE ADORNO: cada cosa que se mueve codifica un dato medido.
//   · la luz que viaja por un axón es un DESPACHO; cuántas viajan a la vez sale de `veces`;
//   · el latido de una neurona sale de su CALOR (cuánto se recupera lo que escribió), y una
//     terminal que nadie consulta se queda quieta — eso también es información;
//   · el giro lento existe para que la profundidad se lea, y se DETIENE en cuanto el usuario
//     está mirando algo (hover, zoom o desplazamiento).
// Se descartó animar por RECENCIA: medido sobre el cerebro local, las 11 terminales tienen su
// nota más nueva a menos de medio día, así que ese canal pinta a todas igual.

const IND = [79, 107, 255], JADE = [63, 208, 163], CYAN = [53, 208, 224];
// Paleta de PERSONAS. El color acá no es decorativo: es la única forma de saber de quién es
// cada racimo. Arranca por el acento de marca y sigue por la familia Aurora; ámbar queda
// último porque en el resto del sistema significa «aviso» y conviene no gastarlo.
const PALETA = [IND, JADE, [160, 107, 255], CYAN, [232, 178, 74]];

// El HUD tiene que pintar la leyenda con EXACTAMENTE el color con que se dibujó cada racimo.
// Se exporta la función y no la tabla para que el índice y el módulo del ciclo vivan en un
// solo lugar: si el HUD repitiera `PALETA[i % 5]` por su cuenta, alcanzaría con agregar un
// color acá para que la leyenda y el dibujo dejaran de coincidir sin que nada se rompa.
export const colorPersona = (i) => { const c = PALETA[i % PALETA.length]; return `rgb(${c[0]},${c[1]},${c[2]})`; };
// Los axones: mismo criterio: el mismo par de colores que usa pintar().
export const COLOR_DESPACHO = `rgb(138,153,255)`;
export const COLOR_CRUCE = `rgb(${CYAN[0]},${CYAN[1]},${CYAN[2]})`;

// PRNG con semilla: el árbol de cada neurona tiene que ser EL MISMO en cada recarga. Con
// Math.random(), dos personas mirando la misma pantalla ven dibujos distintos y no pueden
// hablar de lo que ven — y una captura de ayer deja de comparar con la de hoy.
const rng = (s) => () => ((s = (s * 1664525 + 1013904223) >>> 0) / 4294967296);
// Fase determinista por nombre: dos neuronas no pueden latir al unísono (parecería un efecto
// global y no once relojes distintos), pero tampoco puede depender de Math.random().
const fase = (id) => { let h = 2166136261; for (let i = 0; i < id.length; i++) h = Math.imul(h ^ id.charCodeAt(i), 16777619); return ((h >>> 0) % 1000) / 1000 * 6.2832; };

const add = (a, b) => [a[0] + b[0], a[1] + b[1], a[2] + b[2]];
const mul = (a, k) => [a[0] * k, a[1] * k, a[2] * k];
const norm = (a) => { const l = Math.hypot(a[0], a[1], a[2]) || 1; return [a[0] / l, a[1] / l, a[2] / l]; };
const cross = (a, b) => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]];
const acotar = (v, lo, hi) => Math.max(lo, Math.min(hi, v));

// desviar: gira una dirección `ang` radianes hacia un lado cualquiera del plano perpendicular.
function desviar(d, ang, r) {
  const up = Math.abs(d[1]) > 0.9 ? [1, 0, 0] : [0, 1, 0];
  const e1 = norm(cross(d, up)), e2 = norm(cross(d, e1));
  const t = r() * Math.PI * 2;
  const lat = add(mul(e1, Math.cos(t)), mul(e2, Math.sin(t)));
  return norm(add(mul(d, Math.cos(ang)), mul(lat, Math.sin(ang))));
}

export function crearVista(canvas) {
  const cx = canvas.getContext('2d');
  // `f` grande = perspectiva suave. No es gusto: MEDIDO. Con f=1120 el encuadre quedaba en
  // dist≈1900 para un radio de escena de 538, o sea la profundidad era el 28 % de la distancia;
  // la punta MÁS CERCANA tocaba el borde y todo lo del medio proyectaba al 59 %, dejando la
  // escena en 540 px de los 940 disponibles. Alejando la cámara y alargando la focal en la
  // misma proporción, la escala deja de depender tanto de la profundidad y el dibujo llena el
  // cuadro; la sensación de 3D se sostiene con el degradado de profundidad, el orden del pintor
  // y el giro, no con la deformación. `dist` la calcula el encuadre; el valor inicial es un piso.
  const cam = { yaw: 0.20, pitch: -0.24, dist: 1180, f: 2800, zoom: 1 };
  // ZOOM_MAX alto a propósito: el pedido es poder meterse en las neuronas CHICAS. `SALA DE
  // MANDO` tiene 10 notas y a escala 1 es un punto de 3 px; recién cerca de 25× se le ven las
  // ramas. Lo que hace que eso no cueste un frame es el LOD + el descarte por pantalla.
  const ZOOM_MIN = 0.4, ZOOM_MAX = 40;
  // Niveles EXTRA de dendrita que se generan de más, para que haya qué mirar al acercarse. No
  // se dibujan a escala 1: el LOD los deja fuera del recorrido, no sólo del trazo.
  const NIVELES_EXTRA = 2;

  let W = 0, H = 0, DATOS = null, AX = [], NODOS = [], IDX = new Map();
  // Encuadre: el centro de la escena en el mundo, su radio, y dónde cae el centro del ÁREA
  // ÚTIL en pantalla — que NO es el centro del lienzo, porque el HUD tapa las dos columnas.
  // RH: radio en el plano XZ (invariante al giro). RV: media altura. Van separados porque el
  // ancho y el alto de la pantalla no son el mismo número y un radio único desperdicia el mayor.
  let CENTRO = [0, 0, 0], RH = 600, RV = 300, OX = 0, OY = 0;
  // Desplazamiento en PANTALLA. Existe para que la rueda pueda acercar HACIA EL PUNTERO: sin
  // esto el zoom siempre tira al centro de la escena y llegar a una neurona del borde es
  // imposible por más que el zoom llegue a 40×.
  let PANX = 0, PANY = 0;
  let foco = null, arrastrando = false, modo = 'rotar', lx = 0, ly = 0, vivo = false;
  let reloj = 0;            // segundos de animación; sólo avanza si el panel no está pausado
  let calorMax = 1;
  let viaje = null;         // transición suave hacia una neurona (doble click)
  const buf = [];
  const suave = !matchMedia('(prefers-reduced-motion: reduce)').matches;

  // ── el impulso ──
  // Cuánto tarda un frente en ir del soma a la última punta, y qué porción del árbol abarca.
  // Estos DOS números no salen de ningún dato y hay que decirlo: son la velocidad de LECTURA,
  // elegida para que el ojo pueda seguir el frente. Lo que SÍ sale del dato es cuándo nace el
  // impulso (una invocación real del riel), en qué neurona (el `principal`), de qué capa
  // (`kind`), si falló (`outcome`) y qué tan grueso es (`ms`).
  const DUR_PULSO = 0.85;
  const ANCHO_FRENTE = 0.22;
  // Techo por neurona: una ráfaga de sondeo no puede volverse un fogonazo blanco. Se descarta
  // el más viejo, que ya casi terminó su recorrido.
  const PULSOS_POR_NEURONA = 6;
  let pulsosVistos = 0, pulsosSinNeurona = 0;

  function medir() {
    const dpr = Math.min(devicePixelRatio || 1, 2);
    W = canvas.clientWidth; H = canvas.clientHeight;
    canvas.width = Math.round(W * dpr); canvas.height = Math.round(H * dpr);
    cx.setTransform(dpr, 0, 0, dpr, 0, 0);
    encuadrar();
  }

  // areaUtil: el rectángulo del lienzo que NO tapa el HUD. Se MIDE del DOM en vez de
  // hardcodear los 290 px de la grilla: por debajo de 1080 px los rieles se ocultan
  // (@media en dashboard.html) y ahí el área útil es el lienzo entero. Un número escrito a
  // mano acá diría que hay HUD donde no lo hay, y la escena quedaría chica sin motivo.
  function areaUtil() {
    const m = 20;
    let x0 = m, y0 = m, x1 = W - m, y1 = H - m;
    const vis = (sel) => {
      const el = document.querySelector(sel);
      if (!el) return null;
      const r = el.getBoundingClientRect();
      return (r.width > 1 && r.height > 1) ? r : null;   // display:none da 0×0
    };
    const l = vis('#hud .rail.l'); if (l) x0 = Math.max(x0, l.right + m);
    const r = vis('#hud .rail.r'); if (r) x1 = Math.min(x1, r.left - m);
    const h = vis('#hud .hdr');    if (h) y0 = Math.max(y0, h.bottom + m);
    const c = vis('#hud .center'); if (c) y1 = Math.min(y1, c.top - m);
    // Si el HUD llegara a comerse todo (ventana muy chica), no se devuelve un rectángulo
    // negativo: se cae al lienzo entero, que se ve mal pero se ve.
    if (x1 - x0 < 120 || y1 - y0 < 120) return { x0: 0, y0: 0, x1: W, y1: H };
    return { x0, y0, x1, y1 };
  }

  // Hasta qué inclinación se garantiza el encuadre. Más allá el usuario ya está mirando la
  // escena desde arriba y que una punta roce el borde es barato; encuadrar para el pitch
  // máximo (±1,2 rad) obligaría a alejar tanto la cámara que en reposo la escena entraría en
  // un tercio del cuadro. Se elige el compromiso, no el peor caso.
  const PITCH_ENCUADRE = 0.5;
  let semiW = 200, semiH = 200;

  // encuadrar: aleja la cámara lo justo para que la escena entre en el área útil.
  //
  // Se resuelve POR EJE y CON la perspectiva adentro, que es donde se equivocaba antes. Un
  // punto a radio horizontal Rh cae en pantalla a f·Rh/z, y el peor caso no es el centro de la
  // escena sino su borde CERCANO, a z = dist − Rh. Encuadrar con f·R/dist —ignorando ese
  // acercamiento— y encima usar el radio de la esfera 3D (que infla el horizontal con la
  // profundidad) daba una escena metida en la mitad del cuadro. Despejando f·Rh/(dist−Rh) ≤
  // semiancho sale la fórmula de abajo.
  //
  // Rh es invariante al yaw (es el radio en el plano XZ), así que el encuadre no baila cuando
  // la escena gira sola ni cuando se la arrastra.
  function encuadrar() {
    if (!W || !H) return;
    const a = areaUtil();
    OX = (a.x0 + a.x1) / 2; OY = (a.y0 + a.y1) / 2;
    semiW = Math.max((a.x1 - a.x0) / 2 * 0.96, 60);
    semiH = Math.max((a.y1 - a.y0) / 2 * 0.96, 60);
    // al inclinar, lo que era profundidad pasa a ocupar alto: por eso Rh entra acá también
    const alto = RV * Math.cos(PITCH_ENCUADRE) + RH * Math.sin(PITCH_ENCUADRE);
    cam.dist = Math.max(240, RH + cam.f * RH / semiW, RH + cam.f * alto / semiH);
  }

  // La escala a la profundidad del centro de la escena. Es la referencia para el zoom hacia el
  // puntero: no existe UN factor válido para todas las profundidades a la vez, así que se toma
  // el plano medio, que es donde el ojo cree que está mirando.
  const escalaCentro = () => cam.f / (cam.dist / cam.zoom);

  // construir: de {terminales, despachos} a geometría 3D. Se llama sólo cuando cambian los
  // datos, no por frame: son decenas de miles de segmentos y rehacerlos a 60 fps sería absurdo.
  function construir(datos, racimos) {
    DATOS = datos; AX = []; NODOS = []; IDX = new Map();
    PANX = 0; PANY = 0; cam.zoom = 1; viaje = null;
    if (!racimos.length) return;

    // El tamaño del racimo lo fija cuántas TERMINALES tiene, no cuántas notas: es el volumen
    // que hay que repartir en la esfera. La constante sale del boceto aprobado (8 terminales →
    // 190, 3 → 150); lo que importa no es el número sino la PROPORCIÓN entre el racimo y sus
    // neuronas, porque el encuadre normaliza la escala después. Con racimos más anchos las
    // neuronas quedan relativamente chicas y el dibujo se ve tímido.
    const radioRacimo = (r) => 88 + 36 * Math.sqrt(r.nodos.length);
    // Y se colocan en fila PEGADAS: cada una a su radio más un aire fijo de la siguiente. La
    // versión anterior repartía un ancho fijo en proporción a las notas, así que dos racimos
    // desparejos quedaban lejísimos con vacío en el medio.
    const AIRE = 92;
    let x = 0;
    const centros = racimos.map((r, i) => {
      if (i > 0) x += radioRacimo(racimos[i - 1]) + AIRE + radioRacimo(r);
      return [x, 0, 0];
    });
    const medio = centros.length ? (centros[0][0] + centros[centros.length - 1][0]) / 2 : 0;
    for (const c of centros) c[0] -= medio;

    calorMax = 1;
    racimos.forEach((rac, ri) => {
      const col = PALETA[ri % PALETA.length];
      const c = centros[ri];
      const R = radioRacimo(rac);
      rac.nodos.forEach((t, i) => {
        // esfera de Fibonacci: reparte sin apelmazar y es determinista
        const k = i + 0.5, phi = Math.acos(1 - 2 * k / rac.nodos.length);
        const th = Math.PI * (1 + Math.sqrt(5)) * k;
        // Si una terminal se llama como la persona —GIO, DAVANTIS— va casi en el centro de su
        // racimo. Es la que la representa, y verla orbitando en el borde junto a las otras
        // hacía que el racimo no tuviera núcleo.
        const rr = t.id.toLowerCase() === rac.persona ? R * 0.18 : R;
        const esActor = t.tipo === 'actor';
        // El CALOR es cuánto se recupera lo que alguien escribió. Un actor no escribe, así que
        // no tiene calor y NO late en reposo. No es una carencia del dibujo: una neurona que
        // sólo se enciende cuando llama es exactamente lo que un servicio es.
        const calor = (!esActor && Number.isFinite(t.calor)) ? t.calor : 0;
        if (calor > calorMax) calorMax = calor;
        // DOS UNIDADES DISTINTAS, dos fórmulas. Las notas de una terminal van de 1 a 232 y con
        // raíz quedan bien; las llamadas de un actor van de 1 a 166.371 —cinco órdenes de
        // magnitud— y con raíz el poller taparía la pantalla. Convertir una en otra para tener
        // una sola fórmula sería inventar una equivalencia entre escribir y llamar.
        const nd = {
          id: t.id, tipo: esActor ? 'actor' : 'terminal',
          notas: esActor ? 0 : t.notas, calor, firmas: t.firmas || 0, persona: rac.persona, col,
          llamadas: esActor ? { calls: t.calls, sondeo: t.sondeo, trabajo: t.trabajo, tools: t.tools, proyecto: t.proyecto } : (t.llamadas || null),
          exacta: esActor ? t.exacta !== false : true,
          r: esActor ? Math.max(3.4, Math.log10(1 + (t.calls || 0)) * 3.4)
                     : Math.max(4.2, Math.sqrt(t.notas) * 1.15),
          fase: fase(t.id),
          seg: [], finNivel: [0], alcance: 0, alcanceRama: 1, pulsos: [],
          pos: [c[0] + Math.cos(th) * Math.sin(phi) * rr,
                c[1] + Math.cos(phi) * rr * 0.78,
                c[2] + Math.sin(th) * Math.sin(phi) * rr],
        };
        NODOS.push(nd); IDX.set(t.id, nd);
      });
    });

    // las dendritas
    let semilla = 7;
    for (const nd of NODOS) {
      const r = rng(semilla += 977);
      if (nd.tipo === 'actor') { corona(nd); continue; }
      const raices = Math.max(4, Math.min(7, Math.round(Math.log(nd.notas + 1) * 1.6)));
      nd.profBase = nd.notas > 150 ? 5 : (nd.notas > 55 ? 4 : 3);
      const L0 = nd.r * (nd.notas > 120 ? 2.5 : 2.9);
      for (let i = 0; i < raices; i++) {
        const k = i + 0.5, phi = Math.acos(1 - 2 * k / raices);
        const th = Math.PI * (1 + Math.sqrt(5)) * k;
        const d0 = norm([Math.cos(th) * Math.sin(phi), Math.cos(phi), Math.sin(th) * Math.sin(phi)]);
        crecer(add(nd.pos, mul(d0, nd.r * 0.85)), d0, L0, Math.max(1.5, nd.r * 0.3),
               nd.profBase + NIVELES_EXTRA, 0, nd, r, nd.r * 0.85);
      }
      // Ordenar por nivel y anotar dónde termina cada uno: así el frame recorre un PREFIJO del
      // arreglo y las ramas finas ni se visitan cuando no se ven. Ordenar una vez al construir
      // es lo que hace que el LOD no cueste nada por frame.
      nd.seg.sort((a, b) => a.nivel - b.nivel);
      const hondo = nd.profBase + NIVELES_EXTRA;
      nd.finNivel = new Array(hondo + 1).fill(0);
      for (const s of nd.seg) for (let l = s.nivel; l <= hondo; l++) nd.finNivel[l]++;
      // alcance: hasta dónde llega la copa. Es el radio que usa el descarte por pantalla.
      for (const s of nd.seg) {
        const d = Math.hypot(s.b[0] - nd.pos[0], s.b[1] - nd.pos[1], s.b[2] - nd.pos[2]);
        if (d > nd.alcance) nd.alcance = d;
      }
    }

    // los axones
    for (const d of DATOS.despachos) {
      const A = IDX.get(d.de), B = IDX.get(d.a);
      if (!A || !B) continue;
      const rr = rng(d.de.length * 131 + d.a.length * 17 + d.veces);
      const m = mul(add(A.pos, B.pos), 0.5);
      // el control se separa del eje para que la ida y la vuelta entre dos terminales no se
      // dibujen una encima de la otra y parezcan un solo despacho
      const ctrl = add(m, [(rr() - 0.5) * 150, -70 - d.veces * 4.5 - rr() * 60, (rr() - 0.5) * 150]);
      const P = [];
      for (let i = 0; i <= 26; i++) {
        const t = i / 26, u = 1 - t;
        P.push([u * u * A.pos[0] + 2 * u * t * ctrl[0] + t * t * B.pos[0],
                u * u * A.pos[1] + 2 * u * t * ctrl[1] + t * t * B.pos[1],
                u * u * A.pos[2] + 2 * u * t * ctrl[2] + t * t * B.pos[2]]);
      }
      // El axón queda como CAMINO, no como flujo: su grosor sigue diciendo cuántos despachos
      // hubo (`veces`), que es un dato histórico real. Lo que se fue es el bucle de luces que
      // lo recorría sin que hubiera pasado nada — eso era una animación sin referente.
      AX.push({
        de: d.de, a: d.a, veces: d.veces, P,
        cruza: A.persona !== B.persona,
        Q: new Array(P.length),
      });
    }

    // El centro de la escena son los NODOS. Los arcos de los axones suben bastante por encima
    // de ellos, así que meterlos acá corría el centro hacia arriba y dejaba a las neuronas
    // sentadas en la mitad de abajo del cuadro.
    const lo = [1e9, 1e9, 1e9], hi = [-1e9, -1e9, -1e9];
    for (const nd of NODOS) for (let i = 0; i < 3; i++) {
      if (nd.pos[i] < lo[i]) lo[i] = nd.pos[i];
      if (nd.pos[i] > hi[i]) hi[i] = nd.pos[i];
    }
    CENTRO = [(lo[0] + hi[0]) / 2, (lo[1] + hi[1]) / 2, (lo[2] + hi[2]) / 2];
    // El encuadre se calcula sobre los NODOS y sus etiquetas, NO sobre la última punta de la
    // última dendrita. Es la decisión del boceto aprobado y es deliberada: ahí los árboles de
    // `principal` y `auditor` se salen del cuadro, y por eso el dibujo se lee frondoso. Metiendo
    // las puntas adentro, el mismo grafo proyecta un 40 % más chico y queda tímido — comparé las
    // dos contra el boceto. Lo que NO puede salirse es un nodo o su nombre: eso sí es información.
    let rh = 0, rv = 0;
    const PAD = 46;   // el bulbo del nodo y su etiqueta debajo
    for (const nd of NODOS) {
      const d = Math.hypot(nd.pos[0] - CENTRO[0], nd.pos[2] - CENTRO[2]);
      if (d + PAD > rh) rh = d + PAD;
      const v = Math.abs(nd.pos[1] - CENTRO[1]);
      if (v + PAD > rv) rv = v + PAD;
    }
    RH = Math.max(120, rh);
    RV = Math.max(80, rv);
    encuadrar();
  }

  // `dist` es el camino recorrido DESDE EL SOMA hasta el final de este segmento, medido a lo
  // largo de la rama y no en línea recta. Es lo que le permite al impulso viajar como viaja de
  // verdad —un frente que se propaga por el árbol— en vez de como una onda esférica que
  // encendería a la vez ramas que están a distinto camino.
  function crecer(p, d, largo, w, prof, nivel, nodo, r, dist) {
    // El corte por ancho baja a 0,06: las ramas finas AHORA sirven, porque a 20× se ven. Lo que
    // impide que cuesten es el LOD, no dejar de generarlas.
    if (prof <= 0 || w < 0.06 || nodo.seg.length > 9000) return;
    const hijos = r() < 0.28 ? 3 : 2;
    for (let i = 0; i < hijos; i++) {
      const d2 = desviar(d, 0.38 + r() * 0.42, r);
      const l2 = largo * (0.66 + r() * 0.16);
      const q = add(p, mul(d2, l2));
      const hasta = dist + l2;
      if (hasta > nodo.alcanceRama) nodo.alcanceRama = hasta;
      nodo.seg.push({ a: p, b: q, w, nivel, dist: hasta });
      crecer(q, d2, l2, w * 0.6, prof - 1, nivel + 1, nodo, r, hasta);
    }
  }

  /**
   * corona: la forma de un ACTOR. Radios rectos desde el soma, uno por cada TOOL DISTINTA que
   * esa credencial llama — que es un dato del censo, no un adorno con número redondo.
   *
   * No lleva árbol dendrítico y es deliberado: una dendrita en este grafo representa memoria
   * escrita, y un actor no escribe nada. Dibujarle ramas lo haría parecer lo que no es. Lo que
   * sí necesita es camino por donde salga el impulso, porque si no una llamada de un servicio
   * sería un destello de un píxel.
   */
  function corona(nd) {
    const n = Math.max(2, Math.min(12, nd.llamadas ? nd.llamadas.tools || 2 : 2));
    const L = nd.r * 2.4;
    for (let i = 0; i < n; i++) {
      const k = i + 0.5, phi = Math.acos(1 - 2 * k / n);
      const th = Math.PI * (1 + Math.sqrt(5)) * k;
      const d0 = norm([Math.cos(th) * Math.sin(phi), Math.cos(phi), Math.sin(th) * Math.sin(phi)]);
      const a = add(nd.pos, mul(d0, nd.r * 0.9)), b = add(nd.pos, mul(d0, nd.r * 0.9 + L));
      nd.seg.push({ a, b, w: Math.max(0.9, nd.r * 0.22), nivel: 0, dist: nd.r * 0.9 + L });
    }
    nd.alcanceRama = nd.r * 0.9 + L;
    nd.alcance = nd.alcanceRama;
    nd.profBase = 0;
    nd.finNivel = new Array(NIVELES_EXTRA + 1).fill(nd.seg.length);
  }

  /**
   * pulsar: nace UN impulso, porque pasó UNA cosa. No hay otra forma de que aparezca un pulso
   * en este lienzo — no existe ningún bucle que los fabrique.
   *
   * @param {{terminal:string, capa:string, falla:boolean, ms:number, exacta:boolean}} ev
   * @returns {boolean} si encontró neurona. Un false NO se traga: se cuenta y se muestra.
   */
  function pulsar(ev) {
    pulsosVistos++;
    const nd = ev && ev.terminal ? IDX.get(ev.terminal) : null;
    if (!nd) { pulsosSinNeurona++; return false; }
    if (nd.pulsos.length >= PULSOS_POR_NEURONA) nd.pulsos.shift();
    nd.pulsos.push({
      t0: reloj,
      capa: ev.capa === 'sondeo' ? 'sondeo' : 'trabajo',
      falla: !!ev.falla,
      // `ms` a grosor en escala log: el rango medido va de 0,15 ms (sync_pull) a 60.041 ms
      // (distill). En lineal, todo lo que no sea el peor caso es un pelo invisible.
      grosor: 1 + Math.min(1.6, Math.log10(1 + Math.max(0, Number(ev.ms) || 0)) * 0.42),
      exacta: ev.exacta !== false,
    });
    return true;
  }

  function proy(p) {
    // Se gira alrededor del CENTRO de la escena, no del origen del mundo: con dos racimos de
    // tamaños distintos el centro geométrico no cae en 0, y girar sobre el origen mandaba
    // media escena fuera de cuadro en cuanto se arrastraba.
    const px = p[0] - CENTRO[0], py = p[1] - CENTRO[1], pz = p[2] - CENTRO[2];
    const cy = Math.cos(cam.yaw), sy = Math.sin(cam.yaw);
    const x = px * cy - pz * sy, z = px * sy + pz * cy, y = py;
    const cp = Math.cos(cam.pitch), sp = Math.sin(cam.pitch);
    const y2 = y * cp - z * sp, z2 = y * sp + z * cp;
    // OJO: el zoom mueve la CÁMARA, no la distancia focal. Dividir las dos por `zoom` se
    // cancela en el centro de la escena y el zoom no hace nada justo donde estás mirando.
    const zc = z2 + cam.dist / cam.zoom;
    const s = cam.f / Math.max(zc, 40);
    // El centro de proyección es el del ÁREA ÚTIL, no el del lienzo: si fuera W/2 la escena
    // queda centrada DEBAJO de las tarjetas del HUD, que es exactamente lo que pasaba.
    return { x: OX + PANX + x * s, y: OY + PANY + y2 * s, s, z: zc };
  }

  function pintar() {
    cx.clearRect(0, 0, W, H);
    if (!NODOS.length) {
      cx.fillStyle = 'rgba(154,157,171,.7)';
      cx.font = "13px 'JetBrains Mono', ui-monospace, monospace";
      cx.textAlign = 'center';
      cx.fillText('todavía no hay despachos entre terminales en esta memoria', OX || W / 2, OY || H / 2);
      return;
    }
    buf.length = 0;
    // Profundidad NORMALIZADA dentro de la escena: 0 la cara cercana, 1 la lejana. Antes se
    // medía contra la distancia de cámara, que no es una propiedad de la escena sino del
    // encuadre — al alejar la cámara para achatar la perspectiva, el degradado se planchaba y
    // todo quedaba igual de brillante. Contra el radio propio, el efecto es el mismo con
    // cualquier focal.
    const zCerca = cam.dist / cam.zoom - RH, zLargo = Math.max(2 * RH, 1);
    const prof01 = (z) => acotar((z - zCerca) / zLargo, 0, 1);
    // LOD: cada duplicación del zoom habilita un nivel más de rama. Los niveles extra existen
    // justamente para esto y no se recorren hasta que hay con qué verlos.
    const extra = acotar(Math.round(Math.log2(Math.max(cam.zoom, 1))), 0, NIVELES_EXTRA);

    for (const nd of NODOS) {
      const P = proy(nd.pos);
      // Descarte por pantalla ANTES de proyectar la copa: a 20× casi todas las neuronas están
      // fuera de cuadro, y proyectar sus miles de ramas para tirarlas una por una es el gasto
      // que haría inusable el zoom profundo.
      const rr = nd.alcance * P.s + 40;
      if (P.x + rr < 0 || P.x - rr > W || P.y + rr < 0 || P.y - rr > H) { nd._sx = null; continue; }
      nd._P = P;

      // Los impulsos vivos de ESTA neurona. Se vencen acá y no en un temporizador: el reloj de
      // la escena se detiene cuando el panel esta en pausa, y un pulso no puede seguir viajando
      // mientras el dibujo esta congelado.
      const vivos = nd.pulsos;
      while (vivos.length && reloj - vivos[0].t0 > DUR_PULSO) vivos.shift();
      // El frente de cada pulso, precalculado por NODO: meterlo adentro del bucle de segmentos
      // lo repetiria diez mil veces por cuadro para dar siempre lo mismo.
      const frentes = [];
      for (let k = 0; k < vivos.length; k++) {
        const pu = vivos[k];
        const u = (reloj - pu.t0) / DUR_PULSO;
        frentes.push({
          en: u * nd.alcanceRama,
          radio: ANCHO_FRENTE * nd.alcanceRama,
          // se apaga hacia el final del recorrido: llega a la punta y se disipa, no desaparece
          fuerza: (pu.capa === 'trabajo' ? 1 : 0.3) * (1 - u * u) * (pu.exacta ? 1 : 0.65),
          falla: pu.falla, grosor: pu.grosor,
        });
      }
      nd._flash = 0;
      for (const f of frentes) {   // el soma fogonea mientras el frente todavia esta encima
        if (f.en < f.radio) nd._flash = Math.max(nd._flash, f.fuerza * (1 - f.en / f.radio));
      }

      buf.push({ z: P.z, t: 2, nd, P });
      const lim = nd.finNivel[Math.min(nd.finNivel.length - 1, nd.profBase + extra)];
      for (let i = 0; i < lim; i++) {
        const s = nd.seg[i];
        const A = proy(s.a), B = proy(s.b);
        // una rama de menos de medio píxel no se ve pero cuesta un stroke igual
        if (Math.abs(A.x - B.x) + Math.abs(A.y - B.y) < 0.8) continue;
        // ¿pasa un frente por este tramo justo ahora?
        let carga = 0, gordo = 1, falla = false;
        for (let k = 0; k < frentes.length; k++) {
          const f = frentes[k];
          const d = Math.abs(s.dist - f.en) / f.radio;
          if (d >= 1) continue;
          const forma = (1 - d) * (1 - d);
          carga += forma * f.fuerza;
          if (forma > 0.35) { gordo = Math.max(gordo, f.grosor); falla = falla || f.falla; }
        }
        buf.push({ z: (A.z + B.z) / 2, t: 0, A, B, w: s.w, col: nd.col, nodo: nd.id,
                   carga: carga > 0.01 ? Math.min(1.6, carga) : 0, gordo, falla });
      }
    }

    for (const ax of AX) {
      for (let i = 0; i < ax.P.length; i++) ax.Q[i] = proy(ax.P[i]);
      for (let i = 0; i < ax.Q.length - 1; i++) {
        const A = ax.Q[i], B = ax.Q[i + 1];
        buf.push({ z: (A.z + B.z) / 2, t: 1, A, B, ax, ult: i === ax.Q.length - 2 });
      }
    }
    buf.sort((a, b) => b.z - a.z);   // pintor: lo lejano primero

    for (const it of buf) {
      if (it.t === 0) {
        const off = foco && foco !== it.nodo;
        const p = 1 - 0.78 * prof01(it.A.z);
        const c = it.col;
        cx.lineCap = 'round';
        // 1) la rama en REPOSO
        cx.strokeStyle = `rgba(${c[0]},${c[1]},${c[2]},${((off ? 0.055 : 0.3) * p).toFixed(3)})`;
        cx.lineWidth = Math.max(0.35, it.w * it.A.s * 1.15);
        cx.beginPath(); cx.moveTo(it.A.x, it.A.y); cx.lineTo(it.B.x, it.B.y); cx.stroke();
        // 2) y encima, el FRENTE, si justo pasa por acá. Se dibuja como una segunda pasada y no
        //    cambiando el color de la primera: superponer es lo que hace que el cruce de ramas
        //    acumule luz, que es lo que da el aspecto eléctrico.
        if (it.carga > 0 && !off) {
          // hacia el blanco segun la carga: un impulso fuerte satura, uno debil apenas tiñe
          const k = Math.min(1, it.carga);
          const r2 = Math.round(c[0] + (255 - c[0]) * k * 0.9);
          const g2 = Math.round(c[1] + (255 - c[1]) * k * 0.9);
          const b2 = Math.round(c[2] + (255 - c[2]) * k * 0.75);
          // una falla no se pinta de rojo a secas: se pinta ámbar, que es el color que este
          // sistema ya usa para «aviso» en todo el HUD. Un rojo nuevo seria un idioma nuevo.
          const col = it.falla ? [245, 196, 81] : [r2, g2, b2];
          const base = Math.max(0.6, it.w * it.A.s * 1.15);
          // ADITIVO. Es lo único parecido a un bloom que canvas 2D sabe hacer, y es lo que
          // hace que el impulso se lea como luz y no como una línea pintada: donde dos ramas
          // encendidas se cruzan, el brillo SE SUMA, igual que en la referencia.
          const antes = cx.globalCompositeOperation;
          cx.globalCompositeOperation = 'lighter';
          // 1) el halo, ancho y tenue: sin esto una rama fina encendida sigue siendo una rama
          //    fina. Medido: con sólo el núcleo, un impulso movía el brillo del cuadro 0,3 %.
          cx.strokeStyle = `rgba(${col[0]},${col[1]},${col[2]},${(0.20 * k * p).toFixed(3)})`;
          cx.lineWidth = Math.max(3.2, base * it.gordo * 5.5);
          cx.beginPath(); cx.moveTo(it.A.x, it.A.y); cx.lineTo(it.B.x, it.B.y); cx.stroke();
          // 2) el núcleo, angosto y saturado
          cx.strokeStyle = `rgba(${col[0]},${col[1]},${col[2]},${(Math.min(1, it.carga) * p).toFixed(3)})`;
          cx.lineWidth = Math.max(1.5, base * it.gordo * 2.1);
          cx.beginPath(); cx.moveTo(it.A.x, it.A.y); cx.lineTo(it.B.x, it.B.y); cx.stroke();
          cx.globalCompositeOperation = antes;
        }
      } else if (it.t === 1) {
        const toca = foco && (foco === it.ax.de || foco === it.ax.a);
        const off = foco && !toca;
        const p = 1 - 0.6 * prof01(it.A.z);
        const c = it.ax.cruza ? CYAN : [138, 153, 255];
        // El axón en REPOSO es tenue: lo que se ve es la luz que lo recorre. Antes el brillo
        // base subía con `veces` y competía con el pulso, que es el que trae ese mismo dato.
        const base = toca ? 0.62 : (off ? 0.04 : 0.27);
        cx.strokeStyle = `rgba(${c[0]},${c[1]},${c[2]},${(base * p).toFixed(3)})`;
        cx.lineWidth = Math.max(0.5, (0.7 + it.ax.veces * 0.16) * it.A.s * (toca ? 1.5 : 1));
        cx.beginPath(); cx.moveTo(it.A.x, it.A.y); cx.lineTo(it.B.x, it.B.y); cx.stroke();
        if (it.ult) {   // el despacho tiene DIRECCIÓN y se tiene que ver aunque esté quieto
          const an = Math.atan2(it.B.y - it.A.y, it.B.x - it.A.x);
          const L = Math.max(4, 7 * it.B.s * (toca ? 1.5 : 1));
          cx.fillStyle = `rgba(${c[0]},${c[1]},${c[2]},${((base + 0.14) * p).toFixed(3)})`;
          cx.beginPath();
          cx.moveTo(it.B.x, it.B.y);
          cx.lineTo(it.B.x - Math.cos(an - 0.42) * L, it.B.y - Math.sin(an - 0.42) * L);
          cx.lineTo(it.B.x - Math.cos(an + 0.42) * L, it.B.y - Math.sin(an + 0.42) * L);
          cx.closePath(); cx.fill();
        }
      } else {
        const nd = it.nd, P = it.P, off = foco && foco !== nd.id;
        // LATIDO: la amplitud sale del CALOR, o sea de cuánto se recupera lo que esa terminal
        // escribió. Escala logarítmica porque el reparto es muy desparejo (0 a 435 medidos): en
        // lineal, AUDITOR late y las otras diez se ven congeladas. Calor 0 ⇒ amplitud 0: una
        // terminal que nadie consulta se queda quieta, y eso es lo que hay que ver.
        const amp = Math.log(1 + nd.calor) / Math.log(1 + calorMax);
        const lat = 1 + 0.17 * amp * Math.sin(reloj * 1.9 + nd.fase);
        // El fogonazo del soma es el ARRANQUE del impulso: dura lo que el frente tarda en
        // despegarse del cuerpo. Es lo que hace que se lea «disparó ESTA neurona» y no
        // «apareció una luz en el aire».
        const flash = nd._flash || 0;
        const R = Math.max(2.2, nd.r * P.s * 1.5) * lat * (1 + 0.9 * flash), c = nd.col;
        // El halo tiene TECHO. Es proporcional al nucleo hasta cierto punto y despues crece
        // fijo: sin tope, acercandose se convierte en una mancha que se come las dendritas,
        // que es justo lo que uno fue a mirar de cerca.
        const HALO = R + Math.min(R * 1.6, 38);
        const g = cx.createRadialGradient(P.x, P.y, 0, P.x, P.y, HALO);
        g.addColorStop(0, `rgba(${c[0]},${c[1]},${c[2]},${off ? 0.1 : 0.22 + 0.14 * amp * (lat - 1) / 0.17})`);
        g.addColorStop(1, `rgba(${c[0]},${c[1]},${c[2]},0)`);
        cx.fillStyle = g; cx.beginPath(); cx.arc(P.x, P.y, HALO, 0, 6.2832); cx.fill();
        if (nd.tipo === 'actor') {
          // ANILLO, no disco. Un actor LLAMA y no escribe: el hueco del centro es la
          // diferencia, y se lee de un vistazo sin necesidad de leyenda. Es la distinción
          // ◉/◯ del plan, dibujada en vez de rotulada.
          cx.strokeStyle = `rgba(${c[0]},${c[1]},${c[2]},${off ? 0.3 : 0.95})`;
          cx.lineWidth = Math.max(1.2, R * 0.34);
          // Punteado cuando la atribución NO es exacta: `davantis-crm` cae en el racimo de
          // davantis por la convención del nombre, no porque alguien lo haya declarado. El
          // dibujo tiene que poder decir «esto lo deduje» sin decirlo con la misma tinta.
          if (nd.exacta === false) cx.setLineDash([Math.max(2, R * 0.5), Math.max(2, R * 0.42)]);
          cx.beginPath(); cx.arc(P.x, P.y, R, 0, 6.2832); cx.stroke();
          cx.setLineDash([]);
          if (flash > 0.01) {   // el disparo llena el hueco: es el único momento en que se ve lleno
            cx.fillStyle = `rgba(255,255,255,${(0.75 * flash).toFixed(3)})`;
            cx.beginPath(); cx.arc(P.x, P.y, R * 0.52 * flash, 0, 6.2832); cx.fill();
          }
        } else {
          cx.fillStyle = `rgba(${c[0]},${c[1]},${c[2]},${off ? 0.28 : 0.95})`;
          cx.beginPath(); cx.arc(P.x, P.y, R, 0, 6.2832); cx.fill();
          cx.fillStyle = `rgba(255,255,255,${off ? 0.06 : Math.min(1, 0.22 + 0.78 * flash).toFixed(3)})`;
          cx.beginPath(); cx.arc(P.x, P.y, R * (0.42 + 0.18 * flash), 0, 6.2832); cx.fill();
        }
        nd._sx = P.x; nd._sy = P.y; nd._sr = R;
        if (!off) {   // se etiquetan TODAS: una terminal chica sin nombre es un punto anónimo
          // El destaque sale de la unidad de CADA UNO: notas para quien escribe, llamadas para
          // quien llama. Usar notas para los dos dejaría a todos los actores en el mismo gris.
          const fuerte = nd.tipo === 'actor' ? (nd.llamadas && nd.llamadas.calls >= 1000) : nd.notas >= 60;
          cx.fillStyle = foco === nd.id ? '#ECEAE3'
            : (fuerte ? 'rgba(236,234,227,.62)' : 'rgba(236,234,227,.34)');
          const esAct = nd.tipo === 'actor';
          cx.font = `${Math.max(8, Math.min(esAct ? 12 : 16, (esAct ? 9 : 11) * P.s * 1.3))}px 'JetBrains Mono', ui-monospace, monospace`;
          cx.textAlign = 'center';
          // SE LE SACA EL PREFIJO DE LA PERSONA cuando está dentro del racimo de esa persona.
          // `davantis-lienzo-corpus-reader` son 29 caracteres al lado de `davantis-crm` y
          // `davantis-admin`: las tres etiquetas se pisaban y no se leía ninguna. El racimo ya
          // dice de quién es, así que el prefijo es información repetida ocupando el lugar de
          // la que distingue. En SERVICIOS va entero: ahí el nombre completo ES el dato.
          const et = (esAct && nd.persona && nd.id.startsWith(nd.persona + '-'))
            ? nd.id.slice(nd.persona.length + 1) : nd.id;
          cx.fillText(et.toLowerCase(), P.x, P.y + R + (esAct ? 11 : 14));
        }
      }
    }
  }

  // ── interacción ──
  const nodoEn = (e) => {
    const rc = canvas.getBoundingClientRect();
    const mx = e.clientX - rc.left, my = e.clientY - rc.top;
    let mejor = null, dm = 1e9;
    for (const nd of NODOS) {
      if (nd._sx == null) continue;
      const d = Math.hypot(mx - nd._sx, my - nd._sy);
      if (d < Math.max(16, nd._sr * 1.9) && d < dm) { dm = d; mejor = nd; }
    }
    return mejor;
  };

  canvas.addEventListener('pointerdown', (e) => {
    arrastrando = true;
    // Con shift o con el botón del medio se DESPLAZA en vez de girar. Hace falta: acercándose
    // a 20× la neurona que buscabas se va de cuadro y sin desplazamiento no hay cómo alcanzarla.
    modo = (e.shiftKey || e.button === 1) ? 'pan' : 'rotar';
    lx = e.clientX; ly = e.clientY; viaje = null; soltarFoco();
    try { canvas.setPointerCapture(e.pointerId); } catch (_) { /* sin captura se sigue pudiendo */ }
  });
  canvas.addEventListener('pointerup', () => { arrastrando = false; });
  canvas.addEventListener('pointermove', (e) => {
    if (arrastrando) {
      if (modo === 'pan') { PANX += e.clientX - lx; PANY += e.clientY - ly; }
      else {
        cam.yaw += (e.clientX - lx) * 0.006;
        cam.pitch = acotar(cam.pitch + (e.clientY - ly) * 0.005, -1.2, 1.2);
      }
      lx = e.clientX; ly = e.clientY;
      return;
    }
    const mejor = nodoEn(e);
    const id = mejor ? mejor.id : null;
    if (id !== foco) { foco = id; if (alFocar) alFocar(mejor, DATOS, e.clientX, e.clientY); }
  });
  canvas.addEventListener('pointerleave', () => { soltarFoco(); });
  // Al mover la camara lo que habia bajo el puntero deja de estar ahi, asi que el detalle
  // que quedaba abierto pasa a describir otra cosa. Se cierra: un tooltip que sobrevive al
  // gesto miente con datos ciertos.
  function soltarFoco() { if (foco !== null) { foco = null; if (alFocar) alFocar(null, DATOS); } }
  canvas.addEventListener('wheel', (e) => {
    e.preventDefault();
    const rc = canvas.getBoundingClientRect();
    const mx = e.clientX - rc.left, my = e.clientY - rc.top;
    const antes = escalaCentro();
    cam.zoom = acotar(cam.zoom * (e.deltaY > 0 ? 1 / 1.12 : 1.12), ZOOM_MIN, ZOOM_MAX);
    // El punto bajo el puntero se queda donde está: se despeja el desplazamiento que hace falta
    // para eso. Sin esto la rueda tira siempre al centro y las neuronas del borde son
    // inalcanzables por más que el zoom llegue.
    const k = escalaCentro() / antes;
    PANX = mx - OX - (mx - OX - PANX) * k;
    PANY = my - OY - (my - OY - PANY) * k;
    viaje = null; soltarFoco();
  }, { passive: false });

  // Doble click: la forma directa de llegar a una neurona chica. En una, la centra y se acerca
  // lo justo para que su copa llene el cuadro; en el vacío, vuelve a la vista completa.
  canvas.addEventListener('dblclick', (e) => {
    const nd = nodoEn(e);
    if (!nd) { viaje = { zoom: 1, panx: 0, pany: 0, t: 0 }; return; }
    const alcance = Math.max(nd.alcance, nd.r * 3);
    const zObj = acotar(0.62 * Math.min(semiW, semiH) * cam.dist / (cam.f * alcance), 1, ZOOM_MAX);
    // Dónde caería la neurona con ese zoom y sin desplazamiento: ése es el desplazamiento que
    // hay que aplicar para dejarla en el centro del área útil.
    const zA = cam.zoom, pX = PANX, pY = PANY;
    cam.zoom = zObj; PANX = 0; PANY = 0;
    const P = proy(nd.pos);
    const destino = { zoom: zObj, panx: OX - P.x, pany: OY - P.y, t: 0 };
    cam.zoom = zA; PANX = pX; PANY = pY;
    viaje = destino;
  });

  let alFocar = null;
  addEventListener('resize', () => { if (vivo) medir(); });

  return {
    // datos: {terminales, despachos} de extraerPersonas; racimos: agruparPorPersona(...)
    cargar(datos, racimos) { construir(datos, racimos); },
    activar(v) { vivo = v; if (v) medir(); },
    activa() { return vivo; },
    // frame: lo llama el bucle del dashboard. No abre su propio requestAnimationFrame para no
    // tener DOS bucles peleándose por el mismo frame cuando la lente está apagada.
    frame(motion) {
      if (!vivo) return;
      if (motion && suave) {
        reloj += 1 / 60;
        // El giro es continuo, no después de N frames quietos: la profundidad se lee moviéndose.
        // Pero se DETIENE cuando el usuario está mirando algo —hover, acercado o desplazado—,
        // porque ahí girar deja de ayudar y saca de cuadro lo que estabas leyendo.
        const inspeccionando = arrastrando || foco || cam.zoom > 1.35 || PANX || PANY;
        if (!inspeccionando) cam.yaw += 0.0018;
      }
      if (viaje) {   // transición suave del doble click: llegar de un salto desorienta
        viaje.t = Math.min(1, viaje.t + 0.075);
        const k = 1 - Math.pow(1 - viaje.t, 3);
        cam.zoom += (viaje.zoom - cam.zoom) * k * 0.5;
        PANX += (viaje.panx - PANX) * k * 0.5;
        PANY += (viaje.pany - PANY) * k * 0.5;
        if (viaje.t >= 1) { cam.zoom = viaje.zoom; PANX = viaje.panx; PANY = viaje.pany; viaje = null; }
      }
      pintar();
    },
    onFoco(fn) { alFocar = fn; },
    // pulsar: la ÚNICA puerta por la que entra un impulso. Si nadie la llama, el lienzo no
    // tiene de dónde sacar un pulso — que es exactamente la garantía que se quiere.
    pulsar,
    // Cuántos eventos llegaron y cuántos no encontraron neurona. Se declara en pantalla en vez
    // de repartirlos a dedo: un servicio sin dueño declarado no puede encender la neurona de
    // otro sólo para que la pantalla no quede quieta.
    cuentaPulsos() { return { vistos: pulsosVistos, sinNeurona: pulsosSinNeurona }; },
  };
}
