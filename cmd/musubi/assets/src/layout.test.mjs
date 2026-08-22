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
