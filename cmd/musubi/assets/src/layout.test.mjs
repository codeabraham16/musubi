// Invariantes de la física del grafo (layout.mjs). Corre en CI con `npm test`.
//
// POR QUÉ ESTOS Y NO OTROS. Lo que este archivo custodia no es "el layout se ve lindo" —eso no es
// testeable— sino las tres propiedades cuya violación se vería como un cambio estético y por eso
// nadie miraría: que rebanar no cambie la física, que siempre se avance, y que arrancar un
// asentado nuevo no arrastre el anterior.

import test from 'node:test';
import assert from 'node:assert/strict';
import { iterSettle, iterParaCambio, settleStart, settleTick, settlePasada, settlePendiente, BH_MIN } from './layout.mjs';

// ---------- generador determinista ----------
// Sin Math.random: un test de exactitud bit a bit que no reproduce su entrada no sirve para nada.
let _s = 0;
function rnd(){ _s = (_s * 1103515245 + 12345) & 0x7fffffff; return _s / 0x7fffffff; }

function mundo(n, e, semilla = 20260822){
  _s = semilla;
  const ns = [], sy = [];
  for(let i = 0; i < n; i++){
    const a = rnd() * 6.283, b = Math.acos(2 * rnd() - 1), r = Math.cbrt(rnd()) * 100;
    ns.push({ id: 'n' + i, x: r * Math.sin(b) * Math.cos(a), y: r * Math.sin(b) * Math.sin(a), z: r * Math.cos(b), vx: 0, vy: 0, vz: 0 });
  }
  for(let i = 0; i < e; i++) sy.push({ a: (rnd() * n) | 0, b: (rnd() * n) | 0 });
  return { ns, sy };
}
const clon = ns => ns.map(n => ({ id: n.id, x: n.x, y: n.y, z: n.z, vx: 0, vy: 0, vz: 0 }));
const ESFERA = () => ({ rx: 118, ry: 118, rz: 118 });
const ELIPSE = () => ({ rx: 118, ry: 94, rz: 87 });   // radios distintos: delata un swap al pasarlos

// corre `its` iteraciones con el presupuesto dado. ms=Infinity nunca rebana; ms=0 rebana al máximo.
function correr(ns, sy, radios, its, ms){
  settleStart(ns, sy, radios, its);
  let cortes = 0;
  for(let k = 0; k < its; k++){
    let vueltas = 0;
    while(!settlePasada(performance.now(), ms)){
      cortes++;
      assert.ok(++vueltas < 100000, 'settlePasada no converge: se está girando en falso');
    }
  }
  return cortes;
}
const posiciones = ns => ns.map(n => [n.x, n.y, n.z]);

function comparar(A, B){
  let peor = 0, exactas = 0;
  for(let i = 0; i < A.length; i++){
    const d = Math.hypot(A[i][0] - B[i][0], A[i][1] - B[i][1], A[i][2] - B[i][2]);
    if(d === 0) exactas++;
    if(d > peor) peor = d;
  }
  return { peor, exactas };
}

// Tres escalas a propósito: la primera cae POR DEBAJO de BH_MIN y ejercita el bucle exacto O(n²),
// que NO se rebana; las otras dos van por Barnes-Hut, que sí. Un test que sólo mirara la escala
// grande dejaría el camino exacto sin cubrir.
const ESCALAS = [
  ['exacto O(n2)   500 /   700', 500, 700],
  ['Barnes-Hut    1200 /  2000', 1200, 2000],
  ['Barnes-Hut    3000 /  6000', 3000, 6000],
];

// ---------- I1 · rebanar no cambia la física ----------
//
// Es la razón de ser de settlePasada: cortar la iteración en trozos de nodos para no comerse el
// frame. Vale porque la repulsión de cada nodo lee un árbol ya armado y su propia posición —que no
// cambia hasta la integración del final— y escribe sólo su velocidad. Si alguien mueve la
// integración adentro del trozo, o rebana los resortes, esto se rompe.
for(const [etq, n, e] of ESCALAS){
  for(const [geo, radios] of [['esfera', ESFERA], ['elipsoide', ELIPSE]]){
    test(`I1 · rebanar no cambia la fisica — ${etq} (${geo})`, () => {
      const { ns, sy } = mundo(n, e);
      const its = 12;

      const entero = clon(ns);
      correr(entero, sy, radios, its, Infinity);

      const rebanado = clon(ns);
      const cortes = correr(rebanado, sy, radios, its, 0);

      const { peor, exactas } = comparar(posiciones(entero), posiciones(rebanado));
      assert.equal(peor, 0, `rebanar movió los nodos: peor desvío ${peor.toExponential(2)}`);
      assert.equal(exactas, n);

      // Que efectivamente HAYA rebanado por encima de BH_MIN. Sin esto el test pasaría contento
      // aunque el troceado se hubiera desactivado — y no probaría nada.
      if(n >= BH_MIN) assert.ok(cortes > 0, 'con presupuesto 0 y Barnes-Hut tenía que cortar y no cortó');
    });
  }
}

// ---------- I2 · con presupuesto cero igual se avanza ----------
//
// El chequeo de tiempo va DESPUÉS del trozo, no antes. Si fuera antes, con ms=0 la pasada saldría
// sin haber movido un solo nodo y el asentado no terminaría nunca: el panel se quedaría
// "acomodando" para siempre, gastando CPU sin avanzar. Es un cuelgue, no una lentitud.
test('I2 · con presupuesto 0 la pasada avanza y termina', () => {
  const { ns, sy } = mundo(1500, 2500);
  const copia = clon(ns);
  settleStart(copia, sy, ESFERA, 3);
  let vueltas = 0;
  for(let k = 0; k < 3; k++){
    while(!settlePasada(performance.now(), 0)){
      assert.ok(++vueltas < 100000, 'settlePasada gira en falso con presupuesto 0');
    }
  }
  assert.ok(vueltas > 0, 'no cortó nunca: el troceado no se está ejercitando');
});

// ---------- I3 · arrancar un asentado no arrastra el anterior ----------
//
// settleStart llama a bhGrow, que REASIGNA los arrays del árbol. Si no se reinicia la pasada en
// curso, un _setFase=1 a medio recorrer sigue leyendo _setRoot y el índice _setI del grafo
// ANTERIOR sobre buffers nuevos: posiciones basura, o un nodo inexistente.
//
// El test deja una pasada a medias sobre un grafo grande y arranca otra sobre uno distinto; el
// resultado tiene que ser igual al de correr el segundo grafo solo, desde cero.
test('I3 · settleStart reinicia la pasada en curso', () => {
  const grande = mundo(3000, 5000, 111);
  const chico  = mundo(1000, 1500, 222);

  // referencia: el segundo grafo, a solas
  const limpio = clon(chico.ns);
  correr(limpio, chico.sy, ESFERA, 8, Infinity);

  // ahora lo mismo, pero interrumpiendo un asentado del grafo grande a mitad de la repulsión
  const sucio = clon(chico.ns);
  const otro = clon(grande.ns);
  settleStart(otro, grande.sy, ESFERA, 20);
  settlePasada(performance.now(), 0);          // deja la pasada del grafo grande a medias
  correr(sucio, chico.sy, ESFERA, 8, Infinity);

  const { peor } = comparar(posiciones(limpio), posiciones(sucio));
  assert.equal(peor, 0, `el asentado anterior contaminó el nuevo: peor desvío ${peor.toExponential(2)}`);
});

// ---------- I4 · el trabajo es proporcional al cambio ----------
//
// Lo que evita que guardar UNA observación cueste 55 iteraciones sobre todo el grafo (1,45 s de
// CPU) y reacomode de lugar lo que estabas mirando.
test('I4 · iterParaCambio no rehace el mundo por unos pocos nodos', () => {
  const n = 3678;
  assert.equal(iterParaCambio(n, 0, false), iterSettle(n), 'sin layout previo hay que asentar entero');
  assert.equal(iterParaCambio(n, 50, false), iterSettle(n), 'idem aunque haya nodos nuevos');
  assert.equal(iterParaCambio(n, 0, true), 0, 'ya asentado y sin nodos nuevos: no hay nada que hacer');

  const pocos = iterParaCambio(n, 1, true);
  assert.ok(pocos > 0, 'un nodo nuevo hay que ubicarlo');
  assert.ok(pocos < iterSettle(n) / 5, `un solo nodo nuevo costó ${pocos} de ${iterSettle(n)} iteraciones`);
  assert.ok(iterParaCambio(n, 10, true) > pocos, 'más nodos nuevos, más trabajo');
  assert.ok(iterParaCambio(n, 99999, true) <= iterSettle(n), 'nunca más que asentar entero');
});

// ---------- I5 · settleTick informa cuándo terminó ----------
//
// El panel guarda las posiciones buenas cuando esto dice `termino`. Si mintiera, POS quedaría con
// un layout a medio asentar y la próxima visita arrancaría de ahí.
//
// EL PRESUPUESTO CHICO NO ES DECORATIVO. Con `Infinity` el asentado entero entra en un solo tick,
// así que `termino` sale true de todas formas y un settleTick que cantara victoria siempre pasaría
// el test sin despeinarse — comprobado: esa versión del test no detectaba el sabotaje. Hacen falta
// varios ticks para que haya un "todavía no" que se pueda mentir.
test('I5 · settleTick avisa que termino, y solo entonces', () => {
  const { ns, sy } = mundo(1500, 2500);
  const copia = clon(ns);
  settleStart(copia, sy, ESFERA, 10);

  let ticks = 0, termino = false;
  while(!termino){
    const p = settleTick(1);
    assert.ok(p.trabajo, 'dijo que no hizo nada y todavía quedaban iteraciones');
    assert.equal(p.termino, settlePendiente() === 0,
      `tick ${ticks}: dijo termino=${p.termino} con ${settlePendiente()} iteraciones pendientes`);
    termino = p.termino;
    assert.ok(++ticks < 10000, 'no termina nunca');
  }
  assert.ok(ticks > 1, 'terminó en un solo tick: el caso que importa no se probó');
  assert.equal(settlePendiente(), 0);

  const despues = settleTick(1);
  assert.deepEqual(despues, { trabajo: false, termino: false }, 'ya no queda nada que asentar');
});

// ---------- el centrado por RACIMO ----------
// Custodia la propiedad que hace que «agrupar por persona» sea un dibujo y no una leyenda: los
// nodos con ancla convergen HACIA SU ANCLA. Y la contracara, que es la que evita una regresión
// silenciosa en la lente código: un nodo SIN ancla se comporta exactamente como antes.

function mundoConAnclas(n, anclas, semilla = 20260824){
  const { ns, sy } = mundo(n, n, semilla);
  ns.forEach((a, i) => { const g = anclas[i % anclas.length]; a.gx = g[0]; a.gy = g[1]; a.gz = g[2]; a._g = i % anclas.length; });
  return { ns, sy };
}

test('L-RACIMO-1 · los nodos con ancla se juntan alrededor de la suya', () => {
  const anclas = [[-120, 0, 0], [120, 0, 0]];
  const { ns, sy } = mundoConAnclas(300, anclas);
  const dist0 = ns.map(a => Math.hypot(a.x - a.gx, a.y - a.gy, a.z - a.gz));
  const medio0 = dist0.reduce((s, d) => s + d, 0) / dist0.length;

  settleStart(ns, sy, () => ({ rx: 200, ry: 170, rz: 200 }), 120);
  while (settlePendiente() > 0) settleTick(50);

  const dist1 = ns.map(a => Math.hypot(a.x - a.gx, a.y - a.gy, a.z - a.gz));
  const medio1 = dist1.reduce((s, d) => s + d, 0) / dist1.length;
  assert.ok(medio1 < medio0, `los nodos tenían que acercarse a su ancla: ${medio0.toFixed(1)} -> ${medio1.toFixed(1)}`);

  // Y lo que de verdad importa: los DOS racimos quedan separados. Que cada nodo se acerque a su
  // ancla no alcanza si los dos grupos terminan superpuestos — que es exactamente lo que pasaba
  // sin esta fuerza, con los cuatro racimos reales apilados en una esfera pareja.
  const cen = anclas.map((_, g) => { const gs = ns.filter(a => a._g === g); return gs.reduce((s, a) => [s[0] + a.x / gs.length, s[1] + a.y / gs.length, s[2] + a.z / gs.length], [0, 0, 0]); });
  const sep = Math.hypot(cen[0][0] - cen[1][0], cen[0][1] - cen[1][1], cen[0][2] - cen[1][2]);
  assert.ok(sep > 120, `los dos racimos tenían que separarse; sus centros quedaron a ${sep.toFixed(1)}`);
});

test('L-RACIMO-2 · un nodo SIN ancla no cae en la fuerza de racimo', () => {
  // Es la contracara de L-RACIMO-1 y guarda la lente CÓDIGO, que no tiene racimos: si la fuerza
  // fuerte se aplicara a todos, ese grafo se comprimiría contra el origen y el cambio se leería
  // como «quedó un poco distinto», que es justo lo que nadie mira.
  //
  // La primera versión de este test comparaba un mundo sin `gx` contra otro con `gx=0` esperando
  // que fueran IDÉNTICOS. Estaba mal: con la constante propia, `gx=0` significa «anclado en el
  // origen» y usa la fuerza fuerte a propósito. Lo que hay que probar es lo contrario — que los
  // dos caminos son DISTINTOS —, porque eso es lo único que demuestra que la ausencia de ancla
  // toma el camino viejo.
  const sinAncla = mundo(200, 200, 7777);
  const conAncla0 = mundo(200, 200, 7777);
  conAncla0.ns.forEach(a => { a.gx = 0; a.gy = 0; a.gz = 0; });

  for (const M of [sinAncla, conAncla0]) {
    settleStart(M.ns, M.sy, () => ({ rx: 200, ry: 170, rz: 200 }), 120);
    while (settlePendiente() > 0) settleTick(50);
  }
  const radio = (ns) => ns.reduce((s, a) => s + Math.hypot(a.x, a.y, a.z), 0) / ns.length;
  const rSin = radio(sinAncla.ns), rCon = radio(conAncla0.ns);
  assert.ok(rCon < rSin * 0.75,
    `anclar al origen tiene que compactar mucho más que el centrado suave: sin ancla ${rSin.toFixed(1)}, anclado ${rCon.toFixed(1)}`);
});

test('L-RACIMO-3 · con volumen propio los racimos DEJAN de pisarse', () => {
  // El invariante que hace que «agrupar por persona» se vea: holgura = separación entre centros
  // dividida por la suma de radios. Por debajo de 1 los racimos se superponen y en pantalla queda
  // una esfera pareja — que es exactamente lo que pasaba con el tirón solo (medido: 0,38).
  const anclas = [[-150, 0, 0], [150, 0, 0]];
  const RGR = 60;
  const { ns, sy } = mundo(400, 400, 24680);
  ns.forEach((a, i) => { const g = i % 2; a.gx = anclas[g][0]; a.gy = anclas[g][1]; a.gz = anclas[g][2]; a.gr = RGR; a._g = g; });

  settleStart(ns, sy, () => ({ rx: 300, ry: 260, rz: 300 }), 140);
  while (settlePendiente() > 0) settleTick(80);

  // Ningún nodo puede quedar fuera de la esfera de su racimo: es el techo, no una sugerencia.
  for (const a of ns) {
    const d = Math.hypot(a.x - a.gx, a.y - a.gy, a.z - a.gz);
    assert.ok(d <= RGR + 1e-6, `un nodo se escapó de su racimo: ${d.toFixed(2)} > ${RGR}`);
  }
  const cen = [0, 1].map(g => { const gs = ns.filter(a => a._g === g); return gs.reduce((s, a) => [s[0] + a.x / gs.length, s[1] + a.y / gs.length, s[2] + a.z / gs.length], [0, 0, 0]); });
  const rad = [0, 1].map(g => { const gs = ns.filter(a => a._g === g); return gs.reduce((s, a) => s + Math.hypot(a.x - cen[g][0], a.y - cen[g][1], a.z - cen[g][2]), 0) / gs.length; });
  const sep = Math.hypot(cen[0][0] - cen[1][0], cen[0][1] - cen[1][1], cen[0][2] - cen[1][2]);
  const holgura = sep / (rad[0] + rad[1]);
  assert.ok(holgura > 1, `los racimos se pisan: holgura ${holgura.toFixed(2)} (separación ${sep.toFixed(0)}, radios ${rad.map(r => r.toFixed(0)).join('+')})`);
});
