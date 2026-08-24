// Invariantes del árbol de memoria (arbol-memoria.mjs). Corre en CI.
//
// De este módulo sale la POSICIÓN de cada memoria del panel. Un error acá no se ve como un error:
// se ve como un dibujo distinto, y nadie sabe si el dibujo cambió porque cambió la memoria o porque
// se rompió algo. Por eso lo que custodian estos tests es, sobre todo, que ninguna memoria se
// pierda, que ninguna se duplique, y que el árbol sea EL MISMO mientras el dato sea el mismo.

import test from 'node:test';
import assert from 'node:assert/strict';
import {
  construirNodo, construirRacimo, cortarNeuronas, cortarPorTema, cortarPorTiempo,
  bifurcar, colocar, horquilla, ladear, rng,
} from './arbol-memoria.mjs';

// ── bancadas ─────────────────────────────────────────────────────────────────────────────────
// PERSONA: se parece a davantis en el central — muchas notas repartidas en varios temas.
function personas(n = 400) {
  const temas = ['server', 'gotchas', 'roadmap', 'cuerpo-musubi', 'ingested', 'project', 'review'];
  const out = [];
  for (let i = 0; i < n; i++) {
    const t = temas[i % temas.length];
    out.push({ id: 'p' + i, topic: t + '/' + t + '-' + (i % 11), age_days: (i % 90) + 1, heat: i % 7 });
  }
  return out;
}
// PLANA: se parece a `design-corpus` — 925 notas en UN tema, y casi un subtema por nota. El tema no
// divide nada; si el árbol no sabe caer al tiempo, esto es una bola.
function plana(n = 300) {
  const out = [];
  for (let i = 0; i < n; i++) out.push({ id: 'd' + i, topic: 'design-corpus/patron-' + i, age_days: i * 0.3 + 1, heat: 0 });
  return out;
}

// CON COLA LARGA: la forma REAL del central — un tema que se lleva casi todo y una cola de raíces
// de una nota (`design-corpus` 925 contra 24 raíces sueltas).
//
// La forma importa y costó encontrarla. Con temas gordos REPARTIDOS en subtemas, la bifurcación
// balancea tan bien que nunca queda un grupo chico, y entonces la fusión no se ejecuta NUNCA: el
// test que la custodia pasaba igual con la fusión sacada. Lo que la fuerza es un tema INDIVISIBLE
// enorme al lado de uno diminuto — ahí el mejor corte posible es [400] contra [5] y el 5 queda
// solo. Es exactamente lo que pasa en el central, no un caso de laboratorio.
// Y tiene que ser UN tema chico, no cinco: con cinco raíces sueltas la bifurcación las junta entre
// ellas hasta pasar el mínimo y otra vez no queda nadie solo. Con dos temas, el único corte posible
// es [400] contra [5] y el 5 no tiene con quién juntarse.
function colaLarga() {
  const out = [];
  for (let i = 0; i < 400; i++) out.push({ id: 'g' + i, topic: 'gordo/uno', age_days: (i % 60) + 1 });
  for (let i = 0; i < 5; i++) out.push({ id: 'z' + i, topic: 'zcola/unico', age_days: i + 1 });
  return out;
}

const hojas = (nodo) => (nodo.hijos.length ? nodo.hijos.flatMap(hojas) : [nodo.mem && nodo.mem.id]);

test('T1 · ninguna memoria se pierde y ninguna se duplica', () => {
  // Es el invariante que manda: el panel dice «2.226 memorias» arriba, y si el árbol se come
  // treinta, el número de la cabecera y el dibujo dejan de hablar de lo mismo sin que nada falle.
  for (const ms of [personas(400), plana(300), personas(7), plana(1)]) {
    const ids = hojas(construirNodo(ms));
    assert.equal(ids.length, ms.length, 'cambió la cantidad de hojas');
    assert.equal(new Set(ids).size, ms.length, 'hay ids repetidos: una memoria quedó en dos puntas');
    for (const m of ms) assert.ok(ids.includes(m.id), 'se perdió ' + m.id);
  }
});

test('T2 · el corte da neuronas del tamaño declarado, y ninguna raquítica', () => {
  // Sin la fusión quedaban 155 neuronas de 1-3 memorias entre 183 (medido sobre el central):
  // puntitos sueltos que no se leen como neurona y ensucian el racimo.
  // Va con la bancada de COLA LARGA a propósito: con una regular no aparece ni un grupo chico y la
  // fusión nunca corre, así que el test pasaría igual con la fusión sacada.
  for (const ms of [colaLarga(), personas(600)]) {
    const cortes = cortarNeuronas(construirNodo(ms), 30, 150);
    assert.ok(cortes.length > 1, 'un racimo de ' + ms.length + ' no puede dar una sola neurona');
    const suma = cortes.reduce((s, c) => s + c.n, 0);
    assert.equal(suma, ms.length, 'la suma de las neuronas tiene que ser el racimo entero');
    for (const c of cortes) {
      assert.ok(c.n <= 150 * 2, 'una neurona se pasó del tope aun después de fundir: ' + c.n);
      assert.ok(c.n >= 30, 'quedó una neurona raquítica de ' + c.n + ' memorias');
    }
  }
});

test('T3 · SIEMPRE bifurca de a 2-3, nunca en abanico', () => {
  // Es la diferencia entre «ramificado» y «ramificado como el dibujo». davantis da 72 temas y
  // design-corpus 955 subtemas: colgarlos todos del mismo punto es un plumero, no una dendrita.
  const revisar = (nodo) => {
    assert.ok(nodo.hijos.length <= 3, 'un nodo abrió ' + nodo.hijos.length + ' ramas de una vez');
    nodo.hijos.forEach(revisar);
  };
  revisar(construirNodo(personas(600)));
  revisar(construirNodo(plana(400)));
});

test('T4 · el criterio se DECLARA, y es el que se usó', () => {
  // La rama significa algo sólo si se puede decir qué separa. Un nodo que dice 'tema' tiene que
  // tener hijos con temas distintos; si dijera 'tema' partiendo por tiempo, el tooltip mentiría.
  const raiz = construirNodo(personas(400));
  assert.equal(raiz.criterio, 'tema');
  const vistos = new Set();
  (function ver(n) { vistos.add(n.criterio); n.hijos.forEach(ver); })(raiz);
  assert.ok(vistos.has('tema'), 'un racimo con siete temas tiene que partir por tema en algún lado');
  assert.ok(!vistos.has('__inventado__'));
  // Y lo que declara tiene que ser CIERTO: los hijos de un corte por tema no pueden compartir el
  // segmento que ese corte separó. Ojo con el nivel — un corte en el nivel 0 separa `server/` de
  // `gotchas/`, y uno en el nivel 1 separa dos subtemas DENTRO de `server/`: comparar siempre
  // contra el primer segmento daba un falso positivo, porque el nivel 1 legítimamente comparte el 0.
  const cortes = [];
  (function ver(n) {
    if ((n.criterio === 'tema' || (n.criterio === 'reparto' && n.de === 'tema')) && n.hijos.length > 1) cortes.push(n);
    n.hijos.forEach(ver);
  })(raiz);
  assert.ok(cortes.length > 3, 'hacen falta varios cortes por tema para que el test pruebe algo');
  for (const n of cortes) {
    const segDe = (x) => (x.hijos.length ? x.hijos.flatMap(segDe)
      : [String(x.mem.topic).split('/').filter(Boolean)[n.nivelTema] || '']);
    const lados = n.hijos.map((h) => new Set(segDe(h)));
    for (let i = 0; i < lados.length; i++) {
      for (let j = i + 1; j < lados.length; j++) {
        const cruce = [...lados[i]].filter((t) => lados[j].has(t));
        assert.equal(cruce.length, 0,
          'un corte por TEMA de nivel ' + n.nivelTema + ' dejó «' + cruce[0] + '» de los dos lados');
      }
    }
  }
});

test('T5 · cuando el tema no divide, el árbol cae al TIEMPO', () => {
  // El caso real de `destilador`: 925 notas en UN tema y casi un subtema por nota. Sin este
  // camino el racimo entero es un solo nodo, o sea la bola que este rediseño vino a sacar.
  const ms = plana(300);
  assert.equal(cortarPorTema(ms, 0).ok, false, 'un solo tema no puede considerarse una división');
  assert.equal(cortarPorTema(ms, 0).razon, 'uno-solo');
  assert.equal(cortarPorTema(ms, 1).ok, false, 'un subtema por nota es un identificador, no una división');
  assert.equal(cortarPorTema(ms, 1).razon, 'identificador');
  // Y con dos temas gordos y cuarenta sueltos el corte SÍ vale: el 91 % de las memorias queda bien
  // agrupado. Medir el rechazo por cantidad de GRUPOS en vez de por memorias tiraba esa división.
  assert.equal(cortarPorTema(colaLarga(), 0).ok, true, 'una cola larga no invalida un corte bueno');
  const raiz = construirNodo(ms);
  assert.equal(raiz.criterio, 'tiempo');
  assert.equal(hojas(raiz).length, 300);
  const cortes = cortarNeuronas(raiz, 30, 150);
  assert.ok(cortes.length >= 2, 'tiene que dar varias neuronas, no una bola: dio ' + cortes.length);
});

test('T6 · el mismo dato da EL MISMO árbol', () => {
  // Con Math.random(), dos personas mirando la misma pantalla ven dibujos distintos y no pueden
  // hablar de lo que ven; y una captura de ayer deja de comparar con la de hoy.
  const ms = personas(300);
  const uno = construirRacimo(ms, { centro: [0, 0, 0], radio: 60, semilla: 42 });
  const dos = construirRacimo([...ms], { centro: [0, 0, 0], radio: 60, semilla: 42 });
  assert.equal(uno.neuronas.length, dos.neuronas.length);
  assert.equal(uno.total, dos.total);
  for (const [id, p] of uno.posiciones) {
    const q = dos.posiciones.get(id);
    assert.ok(q, 'falta ' + id + ' en la segunda corrida');
    assert.deepEqual([p.x, p.y, p.z], [q.x, q.y, q.z], 'la memoria ' + id + ' se movió sin que cambiara el dato');
  }
  // Y el ORDEN de entrada no puede cambiar el árbol: las memorias llegan de una consulta SQL, y
  // un ORDER BY distinto no es un cambio en la memoria.
  const tres = construirRacimo([...ms].reverse(), { centro: [0, 0, 0], radio: 60, semilla: 42 });
  assert.equal(tres.neuronas.length, uno.neuronas.length, 'reordenar la entrada cambió el árbol');
});

test('T7 · el grosor sigue la ley de Rall: lo que carga más nace más gordo', () => {
  // Antes el adelgazamiento era un 0,62 fijo por nivel, o sea decoración. Con Rall el grosor es
  // DATO: mirando el dibujo se ve por dónde está el grueso de lo escrito.
  const { segs } = colocar(construirNodo(personas(400)), { escala: 1, semilla: 7 });
  for (const s of segs) {
    assert.ok(s.w1 <= s.w0 + 1e-9, 'un tramo terminó más gordo de lo que empezó: ' + s.w0 + ' -> ' + s.w1);
    assert.ok(s.w1 > 0, 'ningún grosor puede llegar a cero: un tramo invisible cuesta igual');
  }
  // Y ENTRE niveles: el nivel 0 nace mucho más gordo que el 5.
  const w = (n) => { const g = segs.filter((x) => x.nivel === n); return g.reduce((a, x) => a + x.w0, 0) / (g.length || 1); };
  assert.ok(w(5) < w(0) * 0.6, 'el nivel 5 tiene que nacer bastante más fino que el 0: ' + w(0).toFixed(3) + ' vs ' + w(5).toFixed(3));
});

test('T8 · `dist` se mide A LO LARGO DE LA RAMA, no en línea recta', () => {
  // Es lo que hace que el impulso se propague como un frente por el árbol. Con distancia euclídea,
  // dos ramas a distinto camino se encienden a la vez y el pulso se ve como una onda esférica
  // saliendo del soma, no como algo que recorre la neurona.
  const { segs } = colocar(construirNodo(personas(400)), { escala: 1, semilla: 7, centro: [0, 0, 0] });
  let masLargo = 0;
  for (const s of segs) {
    const recto = Math.hypot(s.b[0], s.b[1], s.b[2]);
    assert.ok(s.dist >= recto - 1e-6, 'dist (' + s.dist.toFixed(2) + ') menor que la recta (' + recto.toFixed(2) + ')');
    if (s.dist > recto + 0.5) masLargo++;
  }
  assert.ok(masLargo > segs.length * 0.2, 'el camino tiene que ser MÁS largo que la recta en buena parte del árbol');
});

test('T9 · dos memorias distintas no caen en el mismo punto', () => {
  // Si las puntas se apilan, dos memorias se dibujan una encima de la otra: el hover devuelve
  // siempre la misma y la otra es inalcanzable, aunque el conteo diga que están las dos.
  const { posiciones } = construirRacimo(personas(300), { centro: [0, 0, 0], radio: 60, semilla: 5 });
  const vistos = new Map();
  let choques = 0;
  for (const [id, p] of posiciones) {
    const k = p.x.toFixed(2) + ',' + p.y.toFixed(2) + ',' + p.z.toFixed(2);
    if (vistos.has(k)) choques++;
    vistos.set(k, id);
  }
  assert.equal(choques, 0, choques + ' memorias quedaron apiladas en el mismo punto');
});

test('T10 · toda memoria del racimo recibe posición, y el árbol no se va al infinito', () => {
  const ms = personas(500);
  const { posiciones, neuronas } = construirRacimo(ms, { centro: [10, 0, -5], radio: 60, escala: 1, semilla: 3 });
  assert.equal(posiciones.size, ms.length, 'faltaron posiciones: ' + (ms.length - posiciones.size));
  let lejos = 0;
  for (const [, p] of posiciones) {
    const d = Math.hypot(p.x - 10, p.y, p.z + 5);
    if (!Number.isFinite(d)) assert.fail('una posición salió NaN');
    if (d > 60 * 6) lejos++;
  }
  assert.equal(lejos, 0, lejos + ' memorias quedaron a más de seis radios del centro del racimo');
  assert.ok(neuronas.every((n) => n.segs.length > 0), 'una neurona quedó sin una sola rama');
});

test('T11 · la horquilla abre en un plano, y la rama que pesa más se desvía menos', () => {
  // Es lo que se ve como una horquilla y no como un manojo; y que el peso mande el ángulo hace que
  // mirando el dibujo se lea por dónde sigue el grueso del árbol.
  const r = rng(1);
  const dirs = horquilla([0, 1, 0], [90, 10], 0.7, r);
  const ang = (d) => Math.acos(Math.max(-1, Math.min(1, d[1])));
  assert.ok(ang(dirs[0]) < ang(dirs[1]), 'la rama de 90 tiene que abrirse menos que la de 10');
  for (const d of dirs) assert.ok(Math.abs(Math.hypot(d[0], d[1], d[2]) - 1) < 1e-9, 'la dirección no es unitaria');
});

test('T12 · bifurcar respeta el ORDEN de los grupos', () => {
  // El orden es alfabético o temporal, o sea SIGNIFICA algo: un corte tiene que separar un tramo
  // de otro, nunca intercalar. Si mezcla, la rama deja de poder rotularse.
  const gs = ['a', 'b', 'c', 'd', 'e', 'f', 'g'].map((k, i) => new Array(i + 1).fill(k));
  const b = bifurcar(gs, ['a', 'b', 'c', 'd', 'e', 'f', 'g']);
  const orden = [];
  (function ver(x) { if (Array.isArray(x)) { orden.push(x[0]); return; } x.grupos.forEach(ver); })(b);
  assert.deepEqual(orden, ['a', 'b', 'c', 'd', 'e', 'f', 'g'], 'la bifurcación intercaló los grupos');
});

test('T15 · la panza es PERPENDICULAR al tramo y PERSISTE a lo largo de la rama', () => {
  // Perpendicular: una componente en la dirección del tramo no lo arquea, lo ALARGA — la punta se
  // despegaría de la memoria que representa, que es lo único que el dibujo no puede hacer.
  //
  // Y persiste: lo que curva una dendrita real es tortuosidad, no zigzag. Si el lado se sorteara de
  // nuevo en cada tramo, la rama temblaría en vez de arquearse. La primera versión curvaba «hacia
  // donde siguen los hijos» y daba CERO en una bifurcación simétrica —el caso normal—, así que sólo
  // se curvaba el 11 % de los tramos.
  const r = rng(3);
  const dir = [0, 1, 0];
  const a = ladear(null, dir, r);
  assert.ok(Math.abs(Math.hypot(a[0], a[1], a[2]) - 1) < 1e-9, 'tiene que ser unitario');
  assert.ok(Math.abs(a[0] * dir[0] + a[1] * dir[1] + a[2] * dir[2]) < 1e-9, 'tiene que ser perpendicular');
  // Heredando, el lado se parece al del padre aunque el tramo haya girado.
  const giro = [0.26, 0.94, 0.21];
  const l = Math.hypot(...giro), d2 = giro.map((v) => v / l);
  let prev = a, juntos = 0;
  for (let k = 0; k < 40; k++) {
    const b = ladear(prev, d2, r);
    assert.ok(Math.abs(b[0] * d2[0] + b[1] * d2[1] + b[2] * d2[2]) < 1e-9, 'perdió la perpendicularidad');
    if (prev[0] * b[0] + prev[1] * b[1] + prev[2] * b[2] > 0.6) juntos++;
    prev = b;
  }
  assert.ok(juntos > 30, 'el lado tiene que PERSISTIR: sólo ' + juntos + ' de 40 se parecieron al anterior');
  // Y en el árbol entero: todas las panzas perpendiculares, todas presentes.
  const { segs } = colocar(construirNodo(personas(300)), { escala: 1, semilla: 7 });
  let sinCurva = 0, torcidas = 0;
  for (const s2 of segs) {
    if (Math.hypot(s2.curva[0], s2.curva[1], s2.curva[2]) < 1e-9) sinCurva++;
    if (Math.abs(s2.curva[0] * s2.dir[0] + s2.curva[1] * s2.dir[1] + s2.curva[2] * s2.dir[2]) > 1e-6) torcidas++;
  }
  assert.equal(torcidas, 0, torcidas + ' panzas no eran perpendiculares a su tramo');
  assert.equal(sinCurva, 0, sinCurva + ' tramos quedaron rectos de ' + segs.length);
});

test('T13 · un racimo vacío no rompe, y uno de una sola memoria da una neurona', () => {
  assert.deepEqual(construirRacimo([], {}).neuronas, []);
  assert.equal(construirRacimo(null, {}).total, 0);
  const uno = construirRacimo([{ id: 'x', topic: 'a/b', age_days: 1 }], { radio: 40 });
  assert.equal(uno.neuronas.length, 1);
  assert.equal(uno.posiciones.size, 1);
});

test('T14 · el corte por tiempo ordena de viejo a nuevo', () => {
  // Una dendrita que crece con el tiempo tiene el tronco viejo y las puntas nuevas. Al revés se ve
  // igual de bien y dice exactamente lo contrario.
  const ms = plana(100);
  const c = cortarPorTiempo(ms);
  assert.ok(c && c.grupos.length >= 2);
  const media = (g) => g.reduce((s, m) => s + m.age_days, 0) / g.length;
  for (let i = 1; i < c.grupos.length; i++) {
    assert.ok(media(c.grupos[i - 1]) > media(c.grupos[i]),
      'el tramo ' + (i - 1) + ' tiene que ser MÁS VIEJO que el ' + i);
  }
});
