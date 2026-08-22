import test from 'node:test';
import assert from 'node:assert/strict';
import { extraerPersonas, agruparPorPersona, personaDe } from './personas.mjs';

// Fixture chico y EXPLÍCITO: cada nota está puesta para tensar un invariante distinto, y por
// eso se escriben a mano en vez de generarse. Un fixture generado en bucle hace que todos los
// casos se parezcan, que es justo lo que no sirve para encontrar el que falla.
const n = (id, topic, gist, author) => ({ id, topic, gist, author });

const GRAFO = {
  neurons: [
    n('1', 'terminales/despacho', '📮 EMISARIO → PLANIFICADOR, 2026-08-10. Sobre el eje.', 'gio'),
    n('2', 'terminales/despacho', '📮 EMISARIO → **SKILLS**, 2026-08-13. Acuse.', 'gio'),
    n('3', 'terminales/otro', '📮 PRINCIPAL → EMISARIO, 2026-08-14. Trabajo nocturno.', 'gio'),
    n('4', 'server/deploy', 'SALA DE MANDO → CUERPO, 2026-08-22. Tercera medición.', 'davantis-mando-admin'),
    n('5', 'server/deploy', 'La receta de deploy al central estaba incompleta.', 'davantis-admin'),
    n('6', 'legacy/vieja', 'Una nota sin autor, anterior a la migración v16.', ''),
  ],
};

test('P1 · una terminal se cuenta UNA VEZ por nota, aunque el gist la repita', () => {
  const g = { neurons: [n('x', 'a/b', 'AUDITOR y otra vez AUDITOR y AUDITOR de nuevo', 'gio')] };
  const { terminales } = extraerPersonas(g);
  const aud = terminales.find((t) => t.id === 'AUDITOR');
  assert.equal(aud.notas, 1, 'tres menciones en UNA nota son una nota, no tres');
});

test('P2 · la persona sale de `author`, y las credenciales del mismo humano colapsan', () => {
  assert.equal(personaDe('davantis-mando-admin'), 'davantis');
  assert.equal(personaDe('davantis-altura'), 'davantis');
  assert.equal(personaDe('davantis'), 'davantis');
  assert.equal(personaDe('gio'), 'gio');
  assert.equal(personaDe(''), '', 'sin autor no se inventa una persona');

  const { terminales } = extraerPersonas(GRAFO);
  // SALA DE MANDO la escribió davantis-mando-admin: la persona tiene que ser `davantis`
  const sala = terminales.find((t) => t.id === 'SALA DE MANDO');
  assert.equal(sala.persona, 'davantis');
  const emi = terminales.find((t) => t.id === 'EMISARIO');
  assert.equal(emi.persona, 'gio');
});

test('P3 · el despacho es DIRIGIDO: X → Y no es Y → X', () => {
  const { despachos } = extraerPersonas(GRAFO);
  const buscar = (de, a) => despachos.find((d) => d.de === de && d.a === a);

  assert.ok(buscar('EMISARIO', 'PLANIFICADOR'), 'falta EMISARIO → PLANIFICADOR');
  assert.equal(buscar('PLANIFICADOR', 'EMISARIO'), undefined,
    'la dirección se invirtió: PLANIFICADOR nunca le escribió al EMISARIO en el fixture');

  assert.ok(buscar('PRINCIPAL', 'EMISARIO'), 'falta PRINCIPAL → EMISARIO');
  assert.equal(buscar('EMISARIO', 'PRINCIPAL'), undefined, 'esa dirección no existe en el fixture');
});

test('P4 · una nota SIN flecha no fabrica despachos', () => {
  const g = { neurons: [n('x', 'a/b', 'AUDITOR y PLANIFICADOR trabajaron juntos, sin flecha', 'gio')] };
  const { despachos, terminales } = extraerPersonas(g);
  assert.equal(despachos.length, 0, 'dos roles en la misma nota NO son un despacho');
  assert.equal(terminales.length, 2, 'pero las dos terminales sí existen');
});

test('P5 · las notas sin autor se DECLARAN, no se reparten', () => {
  const { sinAutor, terminales } = extraerPersonas(GRAFO);
  assert.equal(sinAutor, 1, 'hay exactamente una nota legacy sin author en el fixture');
  // y ninguna terminal puede haberse quedado con una persona vacía disfrazada
  for (const t of terminales) {
    assert.notEqual(t.persona, undefined);
  }
});

test('P6 · un despacho repetido se cuenta, no se pisa', () => {
  const g = {
    neurons: [
      n('1', 'a/b', 'PRINCIPAL → SKILLS uno', 'gio'),
      n('2', 'a/b', 'PRINCIPAL → SKILLS dos', 'gio'),
      n('3', 'a/b', 'PRINCIPAL → SKILLS tres', 'gio'),
    ],
  };
  const { despachos } = extraerPersonas(g);
  assert.equal(despachos.length, 1);
  assert.equal(despachos[0].veces, 3, 'tres notas con el mismo despacho son veces=3');
});

test('P7 · los racimos agrupan por persona y el orden es estable', () => {
  const { terminales } = extraerPersonas(GRAFO);
  const racimos = agruparPorPersona(terminales);
  const nombres = racimos.map((r) => r.persona);
  assert.ok(nombres.includes('gio') && nombres.includes('davantis'));
  // el orden no puede depender del orden de inserción: dos corridas, el mismo resultado
  const otra = agruparPorPersona(extraerPersonas(GRAFO).terminales).map((r) => r.persona);
  assert.deepEqual(nombres, otra);
  // y los racimos vienen de mayor a menor
  const notas = racimos.map((r) => r.notas);
  assert.deepEqual(notas, [...notas].sort((a, b) => b - a));
});

test('P8 · no se inventan terminales: una palabra en mayúsculas no es un rol', () => {
  const g = { neurons: [n('x', 'a/b', 'ATENCIÓN → URGENTE: esto no son terminales', 'gio')] };
  const { terminales, despachos } = extraerPersonas(g);
  assert.equal(terminales.length, 0, 'ATENCIÓN y URGENTE no están en la lista de roles');
  assert.equal(despachos.length, 0);
});

test('P9 · el grafo vacío no explota', () => {
  for (const g of [null, undefined, {}, { neurons: [] }]) {
    const r = extraerPersonas(g);
    assert.deepEqual(r.terminales, []);
    assert.deepEqual(r.despachos, []);
    assert.equal(r.sinAutor, 0);
  }
});

test('P10 · la persona sale de quien FIRMA, no de quien menciona', () => {
  // Caso real que apareció midiendo: `ALTURA` la menciona más gio que Gabriel, pero la
  // terminal es de Gabriel. Contar menciones se la daba a gio. La firma —el rol en la
  // CABEZA del gist— es lo que dice quién escribe COMO esa terminal.
  const g = {
    neurons: [
      n('1', 'x/y', 'Un análisis largo que habla de ALTURA y su ERP', 'gio'),
      n('2', 'x/y', 'Otra nota que menciona ALTURA de pasada', 'gio'),
      n('3', 'x/y', 'Y una tercera sobre ALTURA', 'gio'),
      n('4', 'x/y', 'ALTURA → SALA DE MANDO, 2026-08-22. Despacho.', 'davantis-altura'),
    ],
  };
  const { terminales } = extraerPersonas(g);
  const alt = terminales.find((t) => t.id === 'ALTURA');
  assert.equal(alt.notas, 4, 'las cuatro notas la nombran');
  assert.equal(alt.firmas, 1, 'pero sólo una la firma');
  assert.equal(alt.persona, 'davantis',
    'la persona tiene que salir de la firma (davantis), no de las 3 menciones de gio');
});

test('P11 · una terminal que nadie firma cae a quien la menciona', () => {
  const g = { neurons: [n('1', 'x/y', 'Una nota que habla del REFUTADOR sin firmarlo', 'gio')] };
  const { terminales } = extraerPersonas(g);
  const r = terminales.find((t) => t.id === 'REFUTADOR');
  assert.equal(r.firmas, 0);
  assert.equal(r.persona, 'gio', 'sin firmas, la mención es la mejor respuesta disponible');
});
