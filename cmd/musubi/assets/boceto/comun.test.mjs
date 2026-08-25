// comun.test.mjs — los invariantes PUROS del boceto, corriendo en `node --test`.
//
// Están acá y no sólo en la prueba de la página por una razón medida: en headless con
// --virtual-time-budget el navegador **no corre un solo cuadro**, así que cualquier invariante que
// dependa del bucle de render devuelve el mismo valor cuando funciona y cuando está roto. Un
// sabotaje ahí no se distingue de un arnés roto, y un test que no puede fallar no es un test.

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { frenteEn, seccionar, colocarLibre, colocarNucleo, radioRall,
         contarFibras, enhebrar, deshilachar, bifurcar, radioHaz, medirEnredo,
         destinoDeHilo, pasoMezcla, rutaSinapsis, colocarCorona } from './comun.mjs';

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
