// comun.test.mjs — los invariantes PUROS del boceto, corriendo en `node --test`.
//
// Están acá y no sólo en la prueba de la página por una razón medida: en headless con
// --virtual-time-budget el navegador **no corre un solo cuadro**, así que cualquier invariante que
// dependa del bucle de render devuelve el mismo valor cuando funciona y cuando está roto. Un
// sabotaje ahí no se distingue de un arnés roto, y un test que no puede fallar no es un test.

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { frenteEn, seccionar, colocarLibre, colocarNucleo, radioRall,
         contarFibras, enhebrar, deshilachar } from './comun.mjs';

/* ── EL IMPULSO SE APAGA ─────────────────────────────────────────────────────────────────────
   El invariante de fondo del panel entero: sin evento no hay luz. Si el frente no se apaga, la
   luz queda prendida para siempre y la escena deja de poder decir «acá no está pasando nada». */

test('B1 · el frente VIAJA: a más tiempo, más lejos', () => {
  const a = frenteEn(100, 500, 600, 26);
  const b = frenteEn(400, 500, 600, 26);
  assert.ok(a >= 0 && b >= 0, 'los dos tienen que estar encendidos');
  assert.ok(b > a, `el frente no avanzó: ${a} → ${b}`);
});

test('B2 · el frente SE APAGA al pasarse del destino', () => {
  const vel = 500, max = 600, ancho = 26;
  // justo antes del final: encendido
  const antes = frenteEn(((max + ancho) / vel) * 1000 - 20, vel, max, ancho);
  assert.ok(antes >= 0, `debería seguir encendido y dio ${antes}`);
  // pasado el final: apagado, y apagado quiere decir -1, no 0 — un 0 es una posición válida
  const despues = frenteEn(((max + ancho) / vel) * 1000 + 20, vel, max, ancho);
  assert.equal(despues, -1, `no se apagó: quedó en ${despues}`);
  // y sigue apagado mucho después: nada lo vuelve a encender solo
  assert.equal(frenteEn(1e6, vel, max, ancho), -1);
});

test('B3 · el apagado depende del ANCHO del frente, no sólo del destino', () => {
  const vel = 500, max = 600;
  // con un frente ancho todavía queda cola por dibujar cuando la cabeza pasó el destino
  const t = ((max + 10) / vel) * 1000;
  assert.ok(frenteEn(t, vel, max, 200) >= 0, 'con frente ancho aún tiene que quedar cola');
  assert.equal(frenteEn(t, vel, max, 0), -1, 'con frente de ancho 0 ya tendría que estar apagado');
});

/* ── LA LEY DE RALL ──────────────────────────────────────────────────────────────────────────
   Sigue viva en el boceto B (las láminas), donde el grosor no sale de contar hilos. En el boceto A
   la reemplazó la conservación de hilos, que no estima: suma. */

test('B4 · el grosor es DATO: más carga, más radio, y con rendimiento decreciente', () => {
  const r0 = 0.3;
  assert.ok(radioRall(200, r0) > radioRall(20, r0), 'una rama que carga más tiene que ser más gruesa');
  // Rendimiento decreciente: 10× la carga NO da 10× el radio, o el tronco taparía la escena.
  assert.ok(radioRall(200, r0) < radioRall(20, r0) * 10);
  assert.equal(radioRall(0, r0), radioRall(1, r0), 'una carga de 0 no puede dar radio 0 ni NaN');
});

/* ── LA ESTRUCTURA ──────────────────────────────────────────────────────────────────────────── */

const mem = (i, topic) => ({ id: 'm' + i, topic, age_days: i % 60, importance: 5 });
const unaHoja = (m) => ({ n: 1, criterio: 'hoja', etiqueta: '', hijos: [], mem: m });
const grupo = (etiqueta, ms) => ({ n: ms.length, criterio: 'tema', etiqueta, hijos: ms.map(unaHoja) });
// TALLAS DESPAREJAS a propósito. Con todos los temas del mismo tamaño, `seccionar` baja hasta la
// nota suelta en todos y TODAS las hojas quedan con carga 1: un fixture así no puede distinguir
// «los hilos salen de la carga» de «los hilos son siempre uno», y el test pasa sin probar nada.
// Los grupos por debajo de `minCarga` quedan como hoja gorda; los de arriba se siguen partiendo.
const actor = (nombre, desde, tallas) => {
  let i = desde;
  const hs = tallas.map((t, k) => {
    const ms = [];
    for (let j = 0; j < t; j++) ms.push(mem(i++, 'a/' + k));
    return grupo('t' + k, ms);
  });
  return { n: tallas.reduce((a, b) => a + b, 0), criterio: 'actor',
           etiqueta: nombre, racimo: nombre, hijos: hs };
};
// TRES actores, no dos: el reparto de Fibonacci sobre la esfera con dos puntos deja los dos en el
// mismo meridiano y el sesgo mide alto sin que nada esté mal. Tres es el caso real (gio, davantis,
// Musubi) y es donde el invariante dice algo.
const ganglio = () => ({
  n: 360, criterio: 'raiz', etiqueta: 'todo', mem: null,
  hijos: [actor('x', 0, [8, 40, 72]), actor('y', 200, [6, 30, 54]), actor('z', 400, [9, 21, 120])],
});

const OPC_HILOS = { porMemoria: 6, maxHoja: 22 };
const OPC_HEBRA = { radioHilo: 0.3, separacion: 3.4, largoNeurona: 17, torsion: 0.6 };
/** prep: el camino completo del boceto A — seccionar, contar hilos, colocar en el núcleo. */
function prep(opc) {
  const S = seccionar(ganglio(), { maxNivel: 8, minCarga: 10 });
  contarFibras(S, OPC_HILOS);
  colocarNucleo(S, Object.assign({ origen: [0, 0, 0], nucleo: 34, largo: 118, semilla: 11 }, opc));
  return S;
}

// UNA HOJA GORDA: 300 memorias en un solo tema indivisible, o sea POR ENCIMA del tope de 120
// botones por sección. Sin esto el corte nunca se activa y el test no puede verlo — que es
// exactamente lo que el banco de sabotajes destapó: la primera versión de B5 pasaba con el recorte
// roto, porque en el fixture nunca llegaba a recortar.
const hojaGorda = () => {
  const ms = [];
  for (let i = 0; i < 300; i++) ms.push(mem(i, 'unico/tema'));
  return { n: ms.length, criterio: 'raiz', etiqueta: 'todo', racimo: 'z',
           hijos: ms.map(unaHoja), mem: null };
};

test('B5 · ninguna memoria se dibuja DOS VECES, y lo que no entra se DECLARA', () => {
  const S = seccionar(ganglio(), { maxNivel: 8, minCarga: 10 });
  const vistas = new Set();
  let repes = 0;
  for (const s of S) for (const m of s.memorias) { if (vistas.has(m.id)) repes++; vistas.add(m.id); }
  assert.equal(repes, 0, 'una memoria dibujada dos veces cuenta doble en la escena');
  assert.equal(vistas.size, 360, `faltan memorias: ${vistas.size} de 360`);

  // Y ACÁ EL CASO QUE IMPORTA: cuando la sección lleva más de lo que puede dibujar, el recorte
  // TIENE QUE DECLARARSE. Recortar en silencio es la manera callada de mentir: la escena se ve
  // completa y no lo está. `absorbidas` es lo que la ficha muestra como «absorbe N».
  // maxNivel 0: el nodo raíz mismo se convierte en hoja y ABSORBE sus 300. Es la única forma de
  // llegar al recorte en un fixture chico — `seccionar` no bifurca (eso lo hace `construirNodo`),
  // sólo sigue los hijos que le dan, así que con 300 hijos sueltos salen 300 hojas de una memoria
  // y el tope nunca se toca. La primera versión de este test caía justo en ese hueco.
  const G = seccionar(hojaGorda(), { maxNivel: 0, minCarga: 10 });
  const hs = G.filter((s) => s.hoja);
  const dibujadas = hs.reduce((a, s) => a + s.memorias.length, 0);
  const declaradas = hs.reduce((a, s) => a + (s.absorbidas || 0), 0);
  assert.ok(dibujadas < 300, `el fixture no llegó a recortar (dibujó ${dibujadas}): el test no prueba nada`);
  assert.equal(declaradas, 300,
    `se dibujaron ${dibujadas} de 300 y sólo se declararon ${declaradas}: el resto desaparece en silencio`);
});

test('B6 · la carga de un nodo es la SUMA de sus hijas: el grosor no puede mentir', () => {
  const S = seccionar(ganglio(), { maxNivel: 8, minCarga: 10 });
  for (const s of S) {
    if (!s.hijos.length) continue;
    const suma = s.hijos.reduce((a, i) => a + S[i].carga, 0);
    assert.equal(suma, s.carga, `la sección ${s.idx} carga ${s.carga} pero sus hijas suman ${suma}`);
  }
});

/* ══ LOS HILOS ═══════════════════════════════════════════════════════════════════════════════
   El cambio pedido de esta vuelta: «que las ramas estén formadas por hilos de neuronas». Si el
   número de hilos no se conservara en las bifurcaciones, el grosor volvería a ser una estimación
   —una fórmula sobre el dato— y la rama volvería a ser inventada, que es literalmente el reclamo. */

test('B11 · ningún hilo aparece ni desaparece en una bifurcación', () => {
  const S = prep();
  let bifurcaciones = 0;
  for (const s of S) {
    if (!s.hijos.length) continue;
    bifurcaciones++;
    const suma = s.hijos.reduce((a, i) => a + S[i].fibras, 0);
    assert.equal(suma, s.fibras,
      `la sección «${s.etiqueta || s.idx}» lleva ${s.fibras} hilos y sus hijas suman ${suma}`);
  }
  assert.ok(bifurcaciones >= 3, 'el fixture no tiene bifurcaciones: el test no prueba nada');
  // Y el corolario, que es lo que se muestra en la leyenda: el tronco lleva, contados, todos los
  // hilos que mueren en las hojas.
  const enHojas = S.filter((s) => !s.hijos.length).reduce((a, s) => a + s.fibras, 0);
  assert.equal(S[0].fibras, enHojas,
    `el núcleo dice ${S[0].fibras} hilos y las hojas suman ${enHojas}`);
});

test('B12 · los hilos del padre se reparten SIN SOLAPARSE entre las hijas', () => {
  const S = prep();
  for (const s of S) {
    if (!s.hijos.length) continue;
    // Las ranuras tienen que ser contiguas y arrancar donde arranca el padre: es lo que hace que
    // el haz se parta en cuñas al bifurcarse, como un fascículo, en vez de barajarse.
    let esperado = s.ranura;
    for (const i of s.hijos) {
      assert.equal(S[i].ranura, esperado,
        `la hija ${i} arranca en la ranura ${S[i].ranura} y debería arrancar en ${esperado}`);
      esperado += S[i].fibras;
    }
    assert.equal(esperado, s.ranura + s.fibras, 'las hijas no cubren el haz del padre');
  }
});

test('B13 · más memorias, más hilos: el grosor SE CUENTA', () => {
  const S = prep();
  const hs = S.filter((s) => !s.hijos.length && s.carga > 0);
  assert.ok(hs.length >= 4);
  const flaca = hs.reduce((m, s) => (s.carga < m.carga ? s : m), hs[0]);
  const gorda = hs.reduce((m, s) => (s.carga > m.carga ? s : m), hs[0]);
  assert.ok(gorda.carga > flaca.carga, 'el fixture no tiene hojas de tamaños distintos');
  // ESTRICTO, no `>=`. Con `>=` el sabotaje que pone todas las hojas en un hilo pasa igual (1 ≥ 1)
  // y el test se vuelve una afirmación sobre nada: es el mismo agujero que ya tuvo B5.
  assert.ok(gorda.fibras > flaca.fibras,
    `la hoja de ${gorda.carga} memorias lleva ${gorda.fibras} hilos y la de ${flaca.carga} lleva ${flaca.fibras}`);
  // Y ninguna sección puede quedar sin hilo: una rama de cero hilos no se dibuja, y no dibujarla
  // es afirmar que esas memorias no están.
  for (const s of S) assert.ok(s.fibras >= 1, `la sección ${s.idx} quedó con ${s.fibras} hilos`);
});

test('B14 · LA CADENA NO SE TOCA: hay hendidura sináptica entre neurona y neurona', () => {
  const S = prep();
  const F = enhebrar(S, OPC_HEBRA);
  assert.ok(F.length > S.length, `${F.length} eslabones para ${S.length} secciones: no se enhebró`);
  // Dos neuronas encadenadas no se tocan, y ese hueco es lo que deja VER dónde termina una y
  // empieza la otra. Sin él la cadena se lee como un solo tubo largo y el pedido se pierde.
  let pares = 0, minimo = Infinity;
  for (let i = 0; i + 1 < F.length; i++) {
    const a = F[i], b = F[i + 1];
    if (a.sec !== b.sec || a.fib !== b.fib || b.orden !== a.orden + 1) continue;
    pares++;
    minimo = Math.min(minimo, Math.hypot(b.a[0] - a.b[0], b.a[1] - a.b[1], b.a[2] - a.b[2]));
  }
  assert.ok(pares > 0, 'ningún hilo tiene dos neuronas seguidas: el test no prueba nada');
  assert.ok(minimo > 0.05, `la hendidura más chica mide ${minimo.toFixed(3)}: las neuronas se tocan`);
});

test('B15 · EL NÚCLEO NO TIENE ARRIBA: ya no es un árbol', () => {
  const S = prep();
  const d1 = S.filter((s) => s.nivel === 1);
  assert.ok(d1.length >= 3, 'el fixture necesita al menos tres actores');
  // Un árbol tiene suelo y copa: todas sus ramas de primer nivel salen para el mismo lado, así que
  // el promedio de sus direcciones apunta fuerte a +Y y su módulo da ~1. En un ganglio se reparten
  // por la esfera y se CANCELAN. El módulo del promedio es exactamente esa medida.
  const p = [0, 1, 2].map((k) => d1.reduce((a, s) => a + s.dir[k], 0) / d1.length);
  const sesgo = Math.hypot(p[0], p[1], p[2]);
  assert.ok(sesgo < 0.45, `las direcciones apuntan todas para el mismo lado (sesgo ${sesgo.toFixed(2)})`);
  // Y llenan las tres dimensiones parecido: una escena plana también deja de ser un ganglio.
  const ext = [0, 1, 2].map((k) => Math.max(...S.map((s) => s.b[k])) - Math.min(...S.map((s) => s.b[k])));
  assert.ok(Math.min(...ext) > Math.max(...ext) * 0.35,
    `la caja es ${ext.map((x) => x.toFixed(0)).join('×')}: la escena quedó achatada`);
});

test('B7 · el penacho sale SÓLO de las puntas de hilo de las hojas', () => {
  const S = prep();
  const F = enhebrar(S, OPC_HEBRA);
  const RAM = deshilachar(F, S, { niveles: 2, escala: 0.55, semilla: 97 });
  assert.ok(RAM.length > S.length, `sólo ${RAM.length} ramitas: no hay penacho`);
  for (const x of RAM) {
    assert.equal(S[x.seccion].hijos.length, 0,
      `una ramita brotó de la sección ${x.seccion}, que tiene hijas: se cruzaría con el haz`);
  }
});

test('B8 · nada sale NaN: una instancia con NaN desaparece SIN error', () => {
  const S = prep();
  const fin = (v) => v.every(Number.isFinite);
  for (const s of S) {
    assert.ok(fin(s.a) && fin(s.b) && fin(s.curva), `sección ${s.idx} con NaN en su geometría`);
    assert.ok(Number.isFinite(s.largo) && Number.isFinite(s.dist));
  }
  const F = enhebrar(S, OPC_HEBRA);
  for (const e of F) {
    assert.ok(fin(e.a) && fin(e.b) && fin(e.curva) && fin(e.dir), `eslabón con NaN en la sección ${e.sec}`);
    assert.ok(Number.isFinite(e.dist) && Number.isFinite(e.largo) && Number.isFinite(e.r));
  }
  for (const x of deshilachar(F, S, { niveles: 2, escala: 0.55, semilla: 97 })) {
    assert.ok(fin(x.a) && fin(x.b) && fin(x.curva), 'ramita con NaN');
  }
});

test('B9 · el núcleo es MÁS CORTO que cualquier actor', () => {
  const S = prep();
  const actores = S.filter((s) => s.nivel === 1);
  assert.ok(actores.length >= 3);
  for (const a of actores) {
    assert.ok(S[0].largo < a.largo,
      `el núcleo (${S[0].largo}) no puede ser más largo que el actor «${a.etiqueta}» (${a.largo})`);
  }
});

test('B10 · el árbol es DETERMINISTA: dos corridas dan lo mismo', () => {
  const uno = prep(), dos = prep();
  assert.equal(uno.length, dos.length);
  for (let i = 0; i < uno.length; i++) {
    assert.deepEqual(uno[i].b, dos[i].b, `la sección ${i} se movió entre dos corridas iguales`);
    assert.equal(uno[i].fibras, dos[i].fibras, `la sección ${i} cambió de cantidad de hilos`);
  }
  // Y los hilos también: si `enhebrar` sorteara algo, el dibujo cambiaría entre recargas y
  // comparar dos capturas dejaría de decir nada.
  const a = enhebrar(uno, OPC_HEBRA), b = enhebrar(dos, OPC_HEBRA);
  assert.equal(a.length, b.length);
  assert.deepEqual(a[a.length - 1].b, b[b.length - 1].b);
});

test('B16 · el boceto B sigue colocando: la ley de Rall no quedó huérfana', () => {
  const S = seccionar(ganglio(), { maxNivel: 8, minCarga: 10 });
  colocarLibre(S, { origen: [0, -78, 0], largo: 62, largoRaiz: 30, semilla: 11, tropismo: 0.42 });
  for (const s of S) assert.ok(Number.isFinite(s.w0) && s.w0 > 0, `sección ${s.idx} sin radio`);
});
