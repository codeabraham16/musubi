// comun.test.mjs — los invariantes PUROS del boceto, corriendo en `node --test`.
//
// Están acá y no sólo en la prueba de la página por una razón medida: en headless con
// --virtual-time-budget el navegador **no corre un solo cuadro**, así que cualquier invariante que
// dependa del bucle de render devuelve el mismo valor cuando funciona y cuando está roto. Un
// sabotaje ahí no se distingue de un arnés roto, y un test que no puede fallar no es un test.

import * as THREE from 'three';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { frenteEn, seccionar, colocarLibre, colocarNucleo, radioRall,
         contarFibras, enhebrar, deshilachar, bifurcar, radioHaz, medirEnredo,
         destinoDeHilo, pasoMezcla, rutaSinapsis, colocarCorona,
         colocarNudo, repartirEsfera, crearCamara } from './comun.mjs';

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
/**
 * ramificado: un árbol que bifurca de a 2-3, que es lo que produce el pipeline de verdad.
 *
 * El otro fixture arma nodos con 120 hijas de una — sirve para lo que sirve, pero para el reparto
 * de la esfera es un caso patológico: cortar un rectángulo en 120 pedazos deja tiras finísimas y
 * la medida de «parejo» se va al 79 % en un reparto que sobre el dato real da 13 %. Un umbral
 * calibrado ahí no diría nada del dibujo. Se prueba sobre la forma que el árbol tiene de verdad.
 */
const ramificado = (n) => {
  let id = 0;
  const nodo = (k, prof) => {
    if (k <= 1 || prof > 7) {
      const ms = []; for (let j = 0; j < Math.max(1, k); j++) ms.push(mem(id++, 'r/' + prof));
      return grupo('h' + prof + '-' + id, ms);
    }
    const partes = k % 3 === 0 ? 3 : 2;
    const hijos = [];
    for (let p = 0; p < partes; p++) {
      const c = Math.floor(k / partes) + (p < k % partes ? 1 : 0);
      if (c > 0) hijos.push(nodo(c, prof + 1));
    }
    return { n: k, criterio: 'tema', etiqueta: 't' + prof, hijos };
  };
  const raiz = nodo(n, 0);
  raiz.criterio = 'raiz'; raiz.etiqueta = 'todo'; raiz.mem = null;
  return raiz;
};
/** prep0: seccionado y contado, SIN colocar. Cada forma coloca a su manera y sobre lo mismo. */
function prep0() {
  const S = seccionar(ganglio(), { maxNivel: 8, minCarga: 10 });
  contarFibras(S, OPC_HILOS);
  return S;
}
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

/* ══ LA BIFURCACION ═══════════════════════════════════════════════════════════════════════════
   El cambio de esta vuelta. Medido antes de escribirlo: 183 de los 191 choques entre haces no
   emparentados —el 96 %— eran entre HERMANAS, y son inevitables mientras nazcan todas del mismo
   punto. A distancia cero no hay angulo que separe dos ramas. */

// Padre largo y dos hijas gordas: la bifurcacion tiene lugar de sobra, asi que si se tocan es por
// el reparto y no por falta de espacio.
const PADRE = { largo: 120 };
const DOS = [{ R: 8, largo: 70 }, { R: 6, largo: 60 }];

test('B17 · las hermanas NO nacen en el mismo punto', () => {
  const B = bifurcar(PADRE, DOS, {});
  assert.equal(B.apretada & 1, 0, 'el fixture no deberia estar apretado');
  // Traducido a distancia sobre el haz padre: la separacion tiene que alcanzar para que los dos
  // haces no se toquen ahi mismo. Es el invariante que traduce el 96 % del problema medido.
  const sep = Math.abs(B.cuna[0] - B.cuna[1]) * PADRE.largo;
  assert.ok(sep >= DOS[0].R + DOS[1].R, `nacen a ${sep.toFixed(1)}, necesitan ${DOS[0].R + DOS[1].R}`);
});

test('B18 · el angulo entre hermanas sale del GROSOR, no de una constante', () => {
  const finas = bifurcar(PADRE, [{ R: 0.6, largo: 70 }, { R: 0.6, largo: 60 }], { polarMin: 0 });
  const gordas = bifurcar(PADRE, [{ R: 9, largo: 70 }, { R: 9, largo: 60 }], { polarMin: 0 });
  assert.ok(gordas.polar[gordas.orden[1]] > finas.polar[finas.orden[1]] * 1.5,
    'dos haces gordos tienen que abrirse MAS que dos hilos sueltos');
});

test('B19 · lo que no entra se DECLARA', () => {
  // Padre cortisimo, hijas gordas: no hay manera de darles el aire que piden.
  const B = bifurcar({ largo: 6 }, [{ R: 9, largo: 40 }, { R: 9, largo: 40 }], {});
  assert.equal(B.apretada & 1, 1, 'la bifurcacion tiene que quedar marcada como apretada');
  for (const c of B.cuna) assert.ok(c >= 0 && c <= 1, 'y las cunas siguen sobre el haz padre');
});

test('B20 · el escalon de largo no encoge la escena: su media geometrica es 1', () => {
  for (const k of [2, 3, 4, 7]) {
    const hijas = Array.from({ length: k }, (_, i) => ({ R: 4 - i * 0.3, largo: 60 }));
    const B = bifurcar(PADRE, hijas, {});
    const logs = B.escalon.reduce((t, e) => t + Math.log(e), 0);
    // Sin normalizar, el factor medio por nivel queda por debajo de 1 y componer siete niveles
    // achica la escena entera: un escalon que ademas recorta es un recorte disfrazado de detalle.
    assert.ok(Math.abs(logs) < 1e-9, `k=${k}: producto de escalones = ${Math.exp(logs)}`);
  }
});

test('B21 · el dibujo se DESAMONTONA: la metrica lo dice', () => {
  // EL UMBRAL ESTA CALIBRADO CONTRA EL SABOTAJE, no elegido. Sobre este fixture —que es mucho mas
  // denso que el cerebro real: 350 secciones para 360 memorias, casi todas hojas de una nota, asi
  // que su enredo absoluto no es comparable con el 0,06 que da el dato de verdad— los dos valores
  // que importan son:
  //     con la separacion puesta        2,31
  //     con las hermanas pegadas al eje 13,78   ← el sabotaje de este test
  // El umbral va en 5,0: bien arriba del real y bien abajo del roto.
  //
  // LA PRIMERA VERSION DE ESTE TEST ERA RELATIVA —«con separacion tiene que dar 4x mejor que sin»— y
  // el banco la marco VACUA: el sabotaje degrada LOS DOS brazos y el cociente se sostiene igual. Un
  // test comparativo cuyo control tambien se rompe no compara nada. El control se queda, pero como
  // chequeo de la METRICA: prueba que sabe ver maraña, porque un medidor roto que devolviera cero
  // pasaria cualquier umbral sin que nadie se entere.
  const armar = (o) => {
    const S = seccionar(ganglio(), { maxNivel: 8, minCarga: 10 });
    contarFibras(S, OPC_HILOS);
    colocarNucleo(S, Object.assign({ origen: [0, 0, 0], nucleo: 34, largo: 118, curvatura: 0.12,
      tropismo: 0, semilla: 11, radioHilo: 0.52, separacion: 2.60 }, o));
    enhebrar(S, { radioHilo: 0.52, separacion: 2.60, largoNeurona: 17, torsion: 0.6 });
    return medirEnredo(S, { muestras: 8 });
  };
  const con = armar({ aire: 3, polarMin: 0.85 });
  const sin = armar({ aire: 0.05, polarMin: 0, naciente: 0.02 });   // control: nacen casi juntas
  assert.ok(sin.choques > 800,
    `el control solo se enreda ${sin.choques} veces: la METRICA no ve maraña y el test no prueba nada`);
  assert.ok(con.pares > 100, 'el fixture no tiene pares suficientes');
  assert.ok(con.enredo <= 5.0,
    `enredo = ${con.enredo.toFixed(2)} (choques ${con.choques}, hermanas ${con.entreHermanas})`);
});

test('B22 · el radio del haz es UNA sola formula', () => {
  // `enhebrar` y la colocacion tienen que medir el MISMO grosor: si divergieran, la separacion se
  // calcularia sobre un haz que despues no se dibuja.
  const S = prep();
  enhebrar(S, { radioHilo: 0.52, separacion: 2.60, largoNeurona: 17, torsion: 0.6 });
  for (const x of S) {
    assert.ok(Math.abs(x.Rhaz - radioHaz(x.fibras, 0.52, 2.60)) < 1e-9,
      `la seccion ${x.idx} dibuja ${x.Rhaz} y la formula da ${radioHaz(x.fibras, 0.52, 2.60)}`);
  }
});


/* ── EL HILO ES DE ALGUIEN ────────────────────────────────────────────────────────────────────
   El núcleo se pintaba de un gris propio porque «no le pertenece a nadie». Pero un hilo del
   núcleo no es «un hilo del tronco»: es el hilo de una hoja concreta, y se sabe cuál antes de
   dibujarlo. De ahí salen los dos invariantes de abajo. */

test('B23 · todo hilo termina en una HOJA, y las hojas se reparten el tronco sin pisarse', () => {
  const S = prep();
  const D = destinoDeHilo(S);
  assert.equal(D.length, S[0].fibras,
    `el tronco lleva ${S[0].fibras} hilos y la tabla de destinos tiene ${D.length}`);
  // 1 · ninguno sin destino. El -1 es el valor de fallo justamente para que esto se pueda medir:
  //     si el relleno fuera 0, un hilo huérfano se leería como «va a la sección 0» y pasaría.
  const huerfanos = Array.from(D).filter((x) => x < 0).length;
  assert.equal(huerfanos, 0, `${huerfanos} hilos sin destino`);
  // 2 · y todos los destinos son HOJAS
  const noHoja = Array.from(D).filter((x) => S[x].hijos.length).length;
  assert.equal(noHoja, 0, `${noHoja} hilos terminan en una sección que se sigue partiendo`);
  // 3 · cada hoja se queda exactamente con sus hilos, ni uno más
  const cuenta = new Map();
  for (const d of D) cuenta.set(d, (cuenta.get(d) || 0) + 1);
  for (const x of S) {
    if (x.hijos.length) continue;
    assert.equal(cuenta.get(x.idx) || 0, x.fibras,
      `la hoja ${x.idx} declara ${x.fibras} hilos y la tabla le da ${cuenta.get(x.idx) || 0}`);
  }
});

test('B24 · la SUPERFICIE del haz muestra a cada actor en su proporción', () => {
  // Las ranuras de un actor son contiguas y el girasol pone el hilo j a radio ∝ √j: un bloque
  // contiguo es un ANILLO, así que sin mezclar, el actor más grande se queda con toda la cáscara
  // — que es lo único que se ve de un haz gordo. El dibujo afirmaría que el núcleo es suyo.
  const S = prep();
  const D = destinoDeHilo(S);
  const nuc = S[0], f = nuc.fibras, K = pasoMezcla(f);
  // el paso tiene que ser una BIYECCIÓN o se perderían o duplicarían hilos, que es el invariante
  // que sostiene todo el dibujo
  const vistos = new Set();
  for (let j = 0; j < f; j++) vistos.add((j * K) % f);
  assert.equal(vistos.size, f, `el paso ${K} no es una biyección sobre ${f} hilos`);

  const raz = (i) => S[i].racimo || '?';
  const real = new Map(), casc = new Map();
  let nc = 0;
  for (let j = 0; j < f; j++) {
    const r = raz(D[(nuc.ranura || 0) + ((j * K) % f)]);
    real.set(r, (real.get(r) || 0) + 1);
    // la cáscara: el 30 % exterior por ÁREA, que es lo que se ve de un haz de costado
    if ((j + 0.5) / f > 0.70) { casc.set(r, (casc.get(r) || 0) + 1); nc++; }
  }
  assert.ok(real.size >= 3, `el fixture tiene ${real.size} actores y hacen falta 3`);
  let peor = 0, quien = '';
  for (const [r, n] of real) {
    const d = Math.abs(100 * (casc.get(r) || 0) / nc - 100 * n / f);
    if (d > peor) { peor = d; quien = r; }
  }
  // EL UMBRAL ESTÁ CALIBRADO CONTRA EL SABOTAJE, no elegido: sin mezclar, la cáscara da 100 % de
  // un solo actor y el desvío se va a ~60 puntos. Con la mezcla puesta no pasa de 1.
  assert.ok(peor < 6, `«${quien}» ocupa la cáscara con ${peor.toFixed(1)} puntos de desvío`);
});

/* ── LA RELACIÓN VIAJA POR EL ÁRBOL ─────────────────────────────────────────────────────────── */

const dosRamas = () => [
  { idx: 0, padre: -1, a: [0, 0, 0], dir: [0, 1, 0], Rhaz: 5, hijos: [1, 2] },
  { idx: 1, padre: 0, a: [0, 10, 0], dir: [1, 0, 0], Rhaz: 3, hijos: [] },
  { idx: 2, padre: 0, a: [0, 10, 0], dir: [-1, 0, 0], Rhaz: 3, hijos: [] },
];
/** perp: distancia de un punto a la recta que une los dos botones. */
const perp = (q, a, b) => {
  const u = [b[0] - a[0], b[1] - a[1], b[2] - a[2]];
  const L2 = u[0] * u[0] + u[1] * u[1] + u[2] * u[2];
  const w = [q[0] - a[0], q[1] - a[1], q[2] - a[2]];
  const t = L2 ? (w[0] * u[0] + w[1] * u[1] + w[2] * u[2]) / L2 : 0;
  return Math.hypot(w[0] - u[0] * t, w[1] - u[1] * t, w[2] - u[2] * t);
};

test('B25 · la relación arranca y muere EXACTAMENTE en sus dos botones', () => {
  // Una relación que no toca lo que dice unir es peor que no dibujarla: afirma un vínculo entre
  // dos cosas que no son las que se ven en las puntas.
  const S = dosRamas();
  const A = [20, 10, 0], B = [-20, 4, 7];
  for (const beta of [0, 0.5, 0.86, 1]) {
    const r = rutaSinapsis(S, 1, 2, A, B, { beta, muestras: 14 });
    const dA = Math.hypot(r[0][0] - A[0], r[0][1] - A[1], r[0][2] - A[2]);
    const u = r[r.length - 1];
    const dB = Math.hypot(u[0] - B[0], u[1] - B[1], u[2] - B[2]);
    assert.ok(dA < 1e-9, `con beta ${beta} arranca a ${dA} del botón`);
    assert.ok(dB < 1e-9, `con beta ${beta} muere a ${dB} del botón`);
  }
});

test('B26 · con agrupamiento la relación SE DESVÍA de la cuerda recta', () => {
  // Es lo que separa «viaja por el tracto» de «atraviesa el tejido por el camino más corto», que
  // fue el reclamo textual: se veía anti-física. Se mide la distancia perpendicular a la cuerda.
  const S = dosRamas();
  const A = [20, 10, 0], B = [-20, 10, 0];
  const desvio = (beta) => rutaSinapsis(S, 1, 2, A, B, { beta, muestras: 14 })
    .reduce((m, q) => Math.max(m, perp(q, A, B)), 0);
  // CONTROL: con beta 0 la ruta ES la cuerda. Sin esto, un medidor que devolviera cualquier cosa
  // pasaría el umbral de abajo sin que nadie se entere.
  assert.ok(desvio(0) < 1e-9, `con beta 0 tendría que ser la cuerda y se desvía ${desvio(0)}`);
  // EL UMBRAL, CALIBRADO: el ancestro común está a 6 unidades de la cuerda y con beta 0,86 la
  // ruta llega a ~7,7. Con el agrupamiento apagado da 0. Va en 3.
  assert.ok(desvio(0.86) > 3,
    `con beta 0,86 se desvía ${desvio(0.86).toFixed(2)}: la relación sigue siendo una cuerda`);
});


/* ── LAS CINCO FORMAS ─────────────────────────────────────────────────────────────────────────
   Las variantes cambian HACIA DÓNDE crece el tejido, no qué se dibuja. Cada una tiene una promesa
   propia, y una promesa que no se puede ver fallar es una decoración. */

const ang3 = (a, b) => Math.acos(Math.max(-1, Math.min(1, a[0] * b[0] + a[1] * b[1] + a[2] * b[2])));
const hojasDe = (S) => S.filter((s) => !s.hijos.length);
/** El ángulo mínimo entre dos hermanas, sobre todo el árbol. Es lo que se pierde al aplanar mal. */
const minHermanas = (S) => {
  let m = Math.PI;
  for (const s of S) {
    if (s.hijos.length < 2) continue;
    for (let i = 0; i < s.hijos.length; i++) {
      for (let j = i + 1; j < s.hijos.length; j++) {
        m = Math.min(m, ang3(S[s.hijos[i]].dir, S[s.hijos[j]].dir));
      }
    }
  }
  return m;
};

test('B27 · LA CORONA pone toda hoja en el anillo, y parejo', () => {
  const S = prep0();
  colocarCorona(S, { radio: 265, hueco: 62, semilla: 29 });
  const hs = hojasDe(S);
  assert.ok(hs.length > 50, `el fixture da ${hs.length} hojas y hacen falta más`);
  // 1 · TODAS en el anillo. Es lo que distingue esta forma del corte: allá la hoja cae donde la
  //     mandó su rama, acá el borde está parejo pase lo que pase con la jerarquía.
  // Y CONTRA EL RADIO QUE SE PIDIÓ, no sólo entre ellas: comparando las hojas nada más, mandarlas
  // a todas a otro radio pasa el test igual — el banco lo marcó VACUO por exactamente eso.
  const rs = hs.map((s) => Math.hypot(s.b[0], s.b[2]));
  const fuera = rs.filter((x) => Math.abs(x - 265) > 1e-6).length;
  assert.equal(fuera, 0,
    `${fuera} hojas fuera del anillo de 265 (van de ${Math.min(...rs).toFixed(1)} a ${Math.max(...rs).toFixed(1)})`);
  // 2 · y a PASOS IGUALES. Sin esto, «parejo» sería una intención y no una propiedad.
  const th = hs.map((s) => Math.atan2(s.b[2], s.b[0])).sort((a, b) => a - b);
  const ds = []; for (let i = 1; i < th.length; i++) ds.push(th[i] - th[i - 1]);
  const esperado = (2 * Math.PI) / hs.length;
  assert.ok(Math.max(...ds) - Math.min(...ds) < esperado * 0.02,
    `el paso angular va de ${Math.min(...ds).toFixed(5)} a ${Math.max(...ds).toFixed(5)}`);
});

test('B28 · y un subárbol ocupa un arco CONTIGUO: las hermanas no se mezclan', () => {
  // El anillo aplana la jerarquía, así que lo único que la sigue contando es el ORDEN. Con las
  // hojas mezcladas sería un listado y no un árbol.
  const S = prep0();
  colocarCorona(S, { radio: 265, hueco: 62, semilla: 29 });
  const orden = new Map();
  const hs = hojasDe(S)
    .map((s) => ({ i: s.idx, th: Math.atan2(s.b[2], s.b[0]) }))
    .sort((a, b) => a.th - b.th);
  hs.forEach((h, k) => orden.set(h.i, k));
  const total = hs.length;
  // las hojas de cada subárbol, por recorrido
  const bajo = (i, acc) => {
    const s = S[i];
    if (!s.hijos.length) { acc.push(orden.get(i)); return acc; }
    for (const h of s.hijos) bajo(h, acc);
    return acc;
  };
  let peor = null;
  for (const s of S) {
    if (!s.hijos.length || s.idx === 0) continue;
    const ks = bajo(s.idx, []).sort((a, b) => a - b);
    // CONTIGUO EN UN CÍRCULO ES MÓDULO 2π, y el test tuvo que aprenderlo: el subárbol que se sienta
    // sobre el ángulo cero aparece partido en dos al ordenar por ángulo, y eso NO es un defecto —
    // es la costura. Se permite exactamente un salto, y sólo si el conjunto toca las dos puntas.
    const huecos = []; for (let i = 1; i < ks.length; i++) huecos.push(ks[i] - ks[i - 1]);
    let saltos = huecos.filter((h) => h > 1).length;
    if (saltos === 1 && ks[0] === 0 && ks[ks.length - 1] === total - 1) saltos = 0;
    if (saltos > 0 && (peor === null || saltos > peor.saltos)) peor = { idx: s.idx, saltos, n: ks.length };
  }
  assert.equal(peor, null,
    peor && `la sección ${peor.idx} tiene sus ${peor.n} hojas partidas en ${peor.saltos + 1} arcos`);
});

test('B29 · APLANAR no junta hermanas', () => {
  // Es LA trampa de esta forma: achatar la rueda de hijas manda a dos hermanas a la misma
  // dirección y devuelve la maraña, disfrazada de corte prolijo. Medido sobre este fixture:
  //     con el abanico plano   0,019 rad de separación mínima
  //     con el paso en cero    0,000                        ← el sabotaje de este test
  // El umbral va en 0,006: bien arriba del roto y bien abajo del real.
  const S = prep0();
  colocarNucleo(S, { origen: [0, 0, 0], nucleo: 34, largo: 150, semilla: 11,
    reparto: 'plano', plano: true, aire: 3, polarMin: 0.9, radioHilo: 0.52, separacion: 2.60 });
  // 1 · aplana de verdad
  let maxY = 0;
  for (const s of S) if (s.idx !== 0) maxY = Math.max(maxY, Math.abs(s.dir[1]));
  assert.ok(maxY < 0.02, `hay ramas a ${maxY.toFixed(3)} fuera del plano`);
  // 2 · y NO junta
  const m = minHermanas(S);
  assert.ok(m > 0.006, `dos hermanas quedaron a ${m.toFixed(5)} rad: es el mismo punto`);
});

test('B30 · LA CÁSCARA deja las hojas EN la superficie', () => {
  // La promesa de «la corteza» es que las memorias quedan afuera y adentro sólo hay tracto. Sin
  // acortar el tramo al punto donde el rayo corta la esfera, el campo tuerce pero no alcanza:
  //     sin campo    13 % de las hojas a menos del 12 % de la cáscara
  //     con campo    96 %                                              ← lo que se afirma
  const arma = (campo) => {
    const S = prep0();
    colocarNucleo(S, { origen: [0, 0, 0], nucleo: 34, largo: 110, semilla: 11,
      campo, cascara: 150, aire: 3, polarMin: 0.85, radioHilo: 0.52, separacion: 2.60 });
    const rs = hojasDe(S).map((s) => Math.hypot(s.b[0], s.b[1], s.b[2]));
    return rs.filter((x) => Math.abs(x - 150) < 150 * 0.12).length / rs.length;
  };
  // CONTROL: sin campo tiene que dar MAL. Sin esto el test pasaría igual con un árbol que ya
  // naciera del tamaño de la cáscara, que es la otra manera de que el número salga bien.
  const sin = arma(0);
  assert.ok(sin < 0.30, `sin campo ya da ${(100 * sin).toFixed(0)} %: el fixture no prueba nada`);
  const con = arma(1);
  assert.ok(con > 0.80, `con campo sólo el ${(100 * con).toFixed(0)} % llega a la cáscara`);
});


/* ── EL NUDO: la fusión ───────────────────────────────────────────────────────────────────────
   Junta el borde parejo de la corona con el trazo orgánico del núcleo. Son dos promesas y cada una
   tiene su forma de romperse, así que van separadas. */

test('B31 · el reparto de la esfera es PAREJO y COMPACTO', () => {
  // Es la mitad «corona» de la fusión. Se mide con la distancia al vecino más cercano: si el
  // reparto es parejo, todas se parecen; si es grumoso, la dispersión se dispara.
  const S = seccionar(ramificado(700), { maxNivel: 8, minCarga: 10 });
  contarFibras(S, OPC_HILOS);
  const D = repartirEsfera(S);
  const hs = S.filter((s) => !s.hijos.length).map((s) => D[s.idx]);
  assert.ok(hs.length > 50, `el fixture da ${hs.length} hojas y hacen falta más`);
  const vec = [];
  for (let i = 0; i < hs.length; i++) {
    let m = 9;
    for (let j = 0; j < hs.length; j++) {
      if (i === j) continue;
      const e = Math.hypot(hs[i][0] - hs[j][0], hs[i][1] - hs[j][1], hs[i][2] - hs[j][2]);
      if (e < m) m = e;
    }
    vec.push(m);
  }
  // 1 · NINGUNA PARCELA VACÍA: dos hojas en el mismo punto serían una hoja invisible.
  assert.ok(Math.min(...vec) > 1e-3, `dos hojas caen a ${Math.min(...vec)} una de otra`);
  // 2 · y PAREJO. El umbral está calibrado contra el sabotaje: cortando siempre por el mismo lado
  //     salen tiras y el coeficiente de variación se dispara. Medido sobre el cerebro local: 15 %
  //     con el corte por arco, muy por encima de 60 % cortando siempre igual.
  const mv = vec.reduce((a, b) => a + b, 0) / vec.length;
  const dv = Math.sqrt(vec.reduce((a, x) => a + (x - mv) * (x - mv), 0) / vec.length);
  assert.ok(dv / mv < 0.35, `el vecino más cercano varía un ${(100 * dv / mv).toFixed(0)} %`);
  // 3 · y COMPACTO: un subárbol no puede quedar desparramado por toda la esfera, o el borde deja
  //     de contar la jerarquía — que es exactamente para lo que sirve.
  let peor = 0, quien = null;
  for (const s of S) {
    if (!s.hijos.length || s.idx === 0) continue;
    const dd = [];
    (function b(i) {
      const x = S[i];
      if (!x.hijos.length) { dd.push(D[i]); return; }
      for (const h of x.hijos) b(h);
    })(s.idx);
    if (dd.length < 3) continue;
    let c = [0, 0, 0];
    for (const q of dd) c = [c[0] + q[0], c[1] + q[1], c[2] + q[2]];
    const l = Math.hypot(c[0], c[1], c[2]) || 1;
    c = [c[0] / l, c[1] / l, c[2] / l];
    let mx = 0;
    for (const q of dd) {
      mx = Math.max(mx, Math.acos(Math.max(-1, Math.min(1, q[0] * c[0] + q[1] * c[1] + q[2] * c[2]))));
    }
    // el casquete que le tocaría por área:  2π(1 − cos α) = 4π · fracción
    const ideal = Math.acos(Math.max(-1, 1 - 2 * (dd.length / hs.length)));
    if (mx / ideal > peor) { peor = mx / ideal; quien = { idx: s.idx, n: dd.length }; }
  }
  assert.ok(peor < 2.4,
    `la sección ${quien && quien.idx} ocupa ${peor.toFixed(1)}× el casquete que le toca`);
});

test('B32 · el imán deja la hoja EN el radio pedido', () => {
  // Es la otra mitad: sin esto la hoja cae donde la dejó su cadena de largos y el borde vuelve a
  // ser disparejo, que es justo lo que la corona tenía para aportar.
  const arma = (im) => {
    const S = prep0();
    colocarNudo(S, { origen: [0, 0, 0], nucleo: 40, largo: 130, curvatura: 0.12, tropismo: 0,
      semilla: 11, radio: 250, 'imán': im, aire: 3.0, naciente: 0.85, aperturaMax: 1.30,
      polarEje: 0.20, polarMin: 0.85, radioHilo: 0.52, separacion: 2.60 });
    const rs = S.filter((s) => !s.hijos.length).map((s) => Math.hypot(s.b[0], s.b[1], s.b[2]));
    const m = rs.reduce((a, b) => a + b, 0) / rs.length;
    return Math.sqrt(rs.reduce((a, x) => a + (x - m) * (x - m), 0) / rs.length);
  };
  // CONTROL: sin imán el borde TIENE que ser disparejo. Sin esto el test pasaría igual con un
  // árbol que ya naciera esférico, que es la otra manera de que el número salga bien.
  const sin = arma(0);
  assert.ok(sin > 3, `sin imán la dispersión ya es ${sin.toFixed(1)}: el fixture no prueba nada`);
  const con = arma(0.8);
  assert.ok(con < 1, `con imán las hojas todavía se reparten ±${con.toFixed(1)} en radio`);
});


test('B33 · las hermanas apuntan a SU parcela desde que nacen', () => {
  // 🔴 EL RECLAMO: «que en vez de estar para adentro sean un poco más sueltas». Se veía como ramas
  // que salen, se doblan y se pliegan sobre sí mismas hasta formar un puño.
  //
  // La causa era la RAMPA del imán: la fuerza crecía con la profundidad, así que dos hermanas
  // nacían apuntando casi para el mismo lado y recién se despegaban varios niveles después. Es la
  // misma falla que `bifurcar` con otro disfraz — lo que pasa AL NACER manda.
  //
  // Se mide el ángulo entre hermanas, y el PERCENTIL 10 y no la mediana: la mediana puede estar
  // perfecta y el 10 % más apretado seguir pegado, que es justo lo que se ve como amontonado.
  const p10 = (rampa) => {
    // SOBRE EL FIXTURE RAMIFICADO, no el otro: el de 120 hijas de una satura el anillo de
    // `bifurcar` y ahí el ángulo entre hermanas lo fija el tope, no el imán — los dos brazos dan
    // 0,224 idénticos y el test no puede ver nada. El árbol real bifurca de a 2-3.
    const S = seccionar(ramificado(700), { maxNivel: 8, minCarga: 10 });
    contarFibras(S, OPC_HILOS);
    colocarNudo(S, { origen: [0, 0, 0], nucleo: 40, largo: 130, curvatura: 0.12, tropismo: 0,
      semilla: 11, radio: 285, 'imán': 0.92, rampa, aire: 3.0, naciente: 0.85,
      aperturaMax: 1.60, polarEje: 0.20, polarMin: 1.60, radioHilo: 0.52, separacion: 2.60 });
    const a = [];
    for (const s of S) {
      if (s.hijos.length < 2) continue;
      for (let i = 0; i < s.hijos.length; i++) {
        for (let j = i + 1; j < s.hijos.length; j++) {
          const u = S[s.hijos[i]].dir, v = S[s.hijos[j]].dir;
          a.push(Math.acos(Math.max(-1, Math.min(1, u[0] * v[0] + u[1] * v[1] + u[2] * v[2]))));
        }
      }
    }
    a.sort((x, y) => x - y);
    return a[Math.floor(a.length * 0.1)];
  };
  // CONTROL: con la rampa puesta TIENE que apretar. Sin esto el test pasaría igual con un árbol
  // que ya naciera abierto por otra razón, que es la otra manera de que el número salga bien.
  const conRampa = p10(1);
  const sinRampa = p10(0);
  assert.ok(conRampa < sinRampa * 0.6,
    `la rampa no aprieta: ${conRampa.toFixed(3)} contra ${sinRampa.toFixed(3)} sin ella`);
  // EL UMBRAL, CALIBRADO CONTRA EL SABOTAJE. Sobre el cerebro local: 0,128 con rampa y 0,657 sin
  // ella. Sobre este fixture los valores son otros, así que se exige la RELACIÓN y además un piso
  // absoluto que el sabotaje no alcanza.
  assert.ok(sinRampa > 0.30,
    `sin rampa el decil más apretado sigue a ${sinRampa.toFixed(3)} rad: nacen juntas igual`);
});


/* ── LA CÁMARA ────────────────────────────────────────────────────────────────────────────────
   «El movimiento en 3D está muy roto, es muy raro mover las neuronas» — dicho TRES veces. Las dos
   primeras lo afiné a ciegas (amortiguación, inercia, zoom hacia el cursor) y ninguna era la
   causa: la escena giraba siempre alrededor del mismo punto, así que acercarse a una rama lejana
   convertía el arrastre en un arco enorme que cruzaba la pantalla. Sin un invariante, eso se
   «arregla» tres veces sin arreglarse. Acá está. */

/** unDom: lo mínimo que `crearCamara` necesita, y que además deja disparar los eventos a mano. */
function unDom() {
  const h = {};
  return {
    clientHeight: 800, clientWidth: 1200,
    addEventListener: (n, f) => { (h[n] = h[n] || []).push(f); },
    setPointerCapture: () => {},
    getBoundingClientRect: () => ({ left: 0, top: 0, width: 1200, height: 800 }),
    disparar: (n, ev) => { for (const f of (h[n] || [])) f(Object.assign({ preventDefault() {} }, ev)); },
  };
}
const unaCamara = () => {
  const c = new THREE.PerspectiveCamera(50, 1.5, 0.1, 5000);
  c.updateMatrixWorld(true);
  return c;
};

test('C1 · cambiar el PIVOTE no mueve la cámara', () => {
  // Es la mitad que hace invisible al arreglo. Si mover el eje de giro corriera la vista, cada vez
  // que apoyás el dedo la escena daría un salto — peor que el defecto que se venía a arreglar.
  const cámara = unaCamara();
  const cam2 = crearCamara(cámara, unDom(), { az: 0.6, el: 0.3, dist: 300 });
  cam2.tick(1); cam2.tick(1);                       // que la cámara llegue a su lugar
  const antes = cámara.position.clone();
  cam2.fijarPivote([48, -26, 17]);
  cam2.tick(1 / 60);
  const corrio = antes.distanceTo(cámara.position);
  assert.ok(corrio < 0.5, `la vista se movió ${corrio.toFixed(1)} unidades al cambiar el pivote`);
  // Y el pivote SÍ cambió: sin esto el test pasaría con una función que no hace nada.
  assert.ok(cam2.est.foco.distanceTo(new THREE.Vector3(48, -26, 17)) < 1e-6,
    `el pivote quedó en ${cam2.est.foco.toArray().map((x) => x.toFixed(1))}`);
});

test('C2 · arrastrar es 1 a 1: no hay retraso entre la mano y la imagen', () => {
  // La amortiguación es buenísima para el frenado y los vuelos, y es exactamente lo que NO puede
  // aplicarse al gesto en curso: mete cinco cuadros entre lo que hacés y lo que ves, y eso no se
  // lee como suavidad sino como que la escena viene atrás tuyo.
  const cámara = unaCamara();
  const dom = unDom();
  const cam = crearCamara(cámara, dom, { az: 0, el: 0.2, dist: 300 });
  cam.tick(1); cam.tick(1);
  dom.disparar('pointerdown', { clientX: 600, clientY: 400, button: 0, pointerId: 1 });
  dom.disparar('pointermove', { clientX: 700, clientY: 400, button: 0, pointerId: 1 });
  const pedido = cam.meta.az;
  cam.tick(1 / 60);
  assert.ok(Math.abs(cam.est.az - pedido) < 1e-9,
    `pediste ${pedido.toFixed(4)} y la cámara fue a ${cam.est.az.toFixed(4)}: viene atrás`);
  // CONTROL: soltando, el suavizado TIENE que volver — si no, este test pasaría con la
  // amortiguación borrada del todo, que es la otra manera de que no haya retraso.
  dom.disparar('pointerup', { pointerId: 1 });
  cam.meta.az += 1.0;
  cam.tick(1 / 60);
  assert.ok(Math.abs(cam.est.az - cam.meta.az) > 0.05,
    'sin arrastre la cámara ya no suaviza: el frenado y los vuelos van a dar cortes');
});


test('C3 · diez pasos de rueda ACUMULAN, aunque haya pivote', () => {
  // 🔴 EL BUG QUE ESTE TEST EXISTE PARA QUE NO VUELVA. Enganché el pivote a la rueda «para que no
  // se aleje de la geometría», y `fijarPivote` deriva el destino de DONDE ESTÁ la cámara: pisaba la
  // distancia recién fijada con la vieja, así que cada paso de rueda cancelaba su propio zoom.
  // Medido: 300 → 287,8 en diez pasos, cuando tiene que dar 300 → 80,1. Se sintió como «al hacer
  // scroll se daña todo», y era exactamente eso.
  const camina = (conPivote) => {
    const cámara = unaCamara(), dom = unDom();
    const cam = crearCamara(cámara, dom, Object.assign({ az: 0.6, el: 0.3, dist: 300 },
      conPivote ? { puntoBajo: () => [12, -4, 9] } : {}));
    for (let i = 0; i < 40; i++) cam.tick(1 / 60);
    for (let i = 0; i < 10; i++) {
      dom.disparar('wheel', { deltaY: -120, clientX: 600, clientY: 400 });
      cam.tick(1 / 60);
    }
    return cam.meta.dist;
  };
  const esperado = 300 * Math.exp(-1200 * 0.0011);
  // 1 · sin pivote tiene que dar exacto — es el control de que la cuenta del zoom está bien
  assert.ok(Math.abs(camina(false) - esperado) < 0.5,
    `sin pivote la rueda da ${camina(false).toFixed(1)} y debería dar ${esperado.toFixed(1)}`);
  // 2 · y CON pivote tiene que dar lo mismo: el pivote no puede tocar el zoom
  assert.ok(Math.abs(camina(true) - esperado) < 0.5,
    `con pivote la rueda da ${camina(true).toFixed(1)} y debería dar ${esperado.toFixed(1)}`);
});

test('C4 · una vez agarrado, mover el mouse no cambia la distancia', () => {
  // El pivote se fija AL APOYAR EL DEDO y una sola vez. Refijándolo en cada movimiento, la
  // distancia —y con ella la velocidad del paneo y el paso del zoom— cambia mientras girás, que es
  // la otra manera de que el gesto se sienta con voluntad propia.
  const cámara = unaCamara(), dom = unDom();
  // EL PUNTO TIENE QUE DEPENDER DEL CURSOR, como en la realidad: con uno fijo, refijar el pivote en
  // cada movimiento es un no-op después del primero y el sabotaje no muerde. El banco lo marcó
  // VACUO por exactamente eso.
  const cam = crearCamara(cámara, dom,
    { az: 0.6, el: 0.3, dist: 300, puntoBajo: (x, y) => [x * 0.05, -4, y * 0.05] });
  for (let i = 0; i < 40; i++) cam.tick(1 / 60);
  dom.disparar('pointerdown', { clientX: 600, clientY: 400, button: 0, pointerId: 1 });
  cam.tick(1 / 60);
  // CONTROL: apoyar el dedo SÍ tiene que haber movido el pivote. Sin esto el test pasaría con la
  // función desconectada, que es la otra manera de que la distancia no cambie.
  assert.ok(Math.abs(cam.est.dist - 300) > 1,
    'apoyar el dedo no movió el pivote: el test no está probando nada');
  const antes = cam.est.dist;
  for (let i = 0; i < 8; i++) {
    dom.disparar('pointermove', { clientX: 600 + i * 20, clientY: 400 + i * 7, button: 0, pointerId: 1 });
    cam.tick(1 / 60);
  }
  assert.ok(Math.abs(cam.est.dist - antes) < 1e-6,
    `la distancia cambió de ${antes.toFixed(2)} a ${cam.est.dist.toFixed(2)} girando`);
});
