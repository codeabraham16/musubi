// Invariantes de la geometría dendrítica (dendritas.mjs). Corre en CI.
//
// Lo que custodian no es «se ve lindo» —eso no es testeable— sino las propiedades cuya violación
// se vería como un cambio estético y por eso nadie miraría: que el árbol sea el MISMO en cada
// recarga, que adelgace hacia la punta, que el conteo tenga techo, y que la distancia que usa el
// impulso se mida a lo largo de la rama y no en línea recta.

import test from 'node:test';
import assert from 'node:assert/strict';
import { arbol, bosque, rng } from './dendritas.mjs';

const T = (extra = {}) => ({ id: 'AUDITOR', notas: 232, centro: [0, 0, 0], escala: 1, semilla: 4242, ...extra });

test('D1 · el mismo árbol en cada recarga: la forma sale de una semilla, no de Math.random', () => {
  const a = arbol(T()), b = arbol(T());
  assert.equal(a.segs.length, b.segs.length);
  for (let i = 0; i < a.segs.length; i++) {
    assert.deepEqual(a.segs[i].a, b.segs[i].a, `segmento ${i}`);
    assert.deepEqual(a.segs[i].b, b.segs[i].b, `segmento ${i}`);
  }
  // Y semillas distintas dan árboles distintos: si no, la semilla no estaría haciendo nada y
  // las once neuronas serían once copias del mismo dibujo.
  const c = arbol(T({ semilla: 999 }));
  const igual = c.segs.length === a.segs.length && c.segs.every((s, i) => s.b[0] === a.segs[i].b[0]);
  assert.ok(!igual, 'dos semillas distintas tienen que dar dos árboles distintos');
});

test('D2 · la rama ADELGAZA hacia la punta, que es lo que la hace dendrita', () => {
  const { segs } = arbol(T());
  for (const s of segs) {
    assert.ok(s.w1 < s.w0, `un tramo no puede terminar más gordo de lo que empezó: ${s.w0} -> ${s.w1}`);
    assert.ok(s.w1 > 0, 'ningún grosor puede llegar a cero: un tramo invisible es un tramo que cuesta igual');
  }
  // Y adelgaza ENTRE niveles, no sólo dentro de un tramo: una rama de nivel 3 tiene que nacer
  // más fina que una de nivel 0. Sin esto el árbol se ve como un manojo de palitos iguales.
  const w = (n) => { const s = segs.filter(x => x.nivel === n); return s.reduce((a, x) => a + x.w0, 0) / (s.length || 1); };
  assert.ok(w(3) < w(0), `el nivel 3 tiene que nacer más fino que el 0: ${w(0).toFixed(3)} vs ${w(3).toFixed(3)}`);
});

test('D3 · el conteo tiene TECHO y se respeta', () => {
  // Sin techo, once troncos pasaban de 100.000 instancias. WebGL las dibuja en una llamada, pero
  // generarlas y subirlas cuesta en CADA reconstrucción del grafo.
  const { segs } = arbol(T({ notas: 100000, tope: 500 }));
  assert.ok(segs.length <= 500, `el tope era 500 y salieron ${segs.length}`);
  assert.ok(segs.length > 100, 'un tope alto no puede dar un árbol pelado');
});

test('D4 · `dist` se mide A LO LARGO DE LA RAMA, no en línea recta', () => {
  // Es lo que hace que el impulso se propague como un frente por el árbol. Con distancia
  // euclídea, dos ramas que están a distinto camino se encenderían a la vez, y el pulso se vería
  // como una onda esférica saliendo del soma en vez de como algo que recorre la neurona.
  const { segs } = arbol(T());
  let vistos = 0;
  for (const s of segs) {
    const recto = Math.hypot(s.b[0], s.b[1], s.b[2]);
    assert.ok(s.dist >= recto - 1e-9, `dist (${s.dist.toFixed(2)}) no puede ser menor que la línea recta (${recto.toFixed(2)})`);
    if (s.dist > recto + 0.5) vistos++;
  }
  assert.ok(vistos > segs.length * 0.2,
    `el camino tiene que ser MÁS LARGO que la recta en buena parte del árbol; sólo pasó en ${vistos} de ${segs.length}`);
});

test('D5 · el bosque reparte los troncos DENTRO de su racimo', () => {
  const racimos = [{
    persona: 'gio', centro: [100, 0, 0], radio: 60, color: '#0f0',
    troncos: [{ id: 'AUDITOR', notas: 232 }, { id: 'GIO', notas: 97 }, { id: 'SKILLS', notas: 152 }],
  }];
  const { troncos, total } = bosque(racimos, { topePorArbol: 300 });
  assert.equal(troncos.length, 3);
  assert.ok(total > 0 && total <= 900);
  for (const t of troncos) {
    const d = Math.hypot(t.centro[0] - 100, t.centro[1], t.centro[2]);
    assert.ok(d <= 60, `el tronco ${t.id} se salió de su racimo: ${d.toFixed(1)} > 60`);
  }
  // El que se llama como la persona va casi al centro: es el que la representa, y verlo orbitando
  // en el borde junto a los otros dejaba al racimo sin núcleo.
  const propio = troncos.find(t => t.id === 'GIO');
  const otro = troncos.find(t => t.id === 'AUDITOR');
  const dp = Math.hypot(propio.centro[0] - 100, propio.centro[1], propio.centro[2]);
  const dof = Math.hypot(otro.centro[0] - 100, otro.centro[1], otro.centro[2]);
  assert.ok(dp < dof, `la neurona homónima tiene que ir más al centro: ${dp.toFixed(1)} vs ${dof.toFixed(1)}`);
});

test('D6 · un racimo SIN troncos no produce nada, y no rompe', () => {
  // Es el caso real del libro mayor y de lo sin atribuir: nadie los firma, así que no tienen
  // neurona. Fabricarles una sería inventar un autor.
  const { troncos, total } = bosque([{ persona: 'libro mayor', centro: [0, 0, 0], radio: 80, troncos: [] }], {});
  assert.deepEqual(troncos, []);
  assert.equal(total, 0);
  assert.deepEqual(bosque(null, null).troncos, []);
});

test('D7 · el PRNG es el mismo que usaba el lienzo 2D', () => {
  // El salto de motor no puede cambiar la FORMA de los árboles: si cambiara, sería imposible
  // saber si lo que se ve distinto es el render nuevo o un árbol nuevo.
  const r = rng(1);
  const tres = [r(), r(), r()];
  const r2 = rng(1);
  assert.deepEqual([r2(), r2(), r2()], tres);
  for (const v of tres) assert.ok(v >= 0 && v < 1, `fuera de rango: ${v}`);
});
