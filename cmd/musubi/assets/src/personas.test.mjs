import test from 'node:test';
import assert from 'node:assert/strict';
import {
  extraerPersonas, agruparPorPersona, personaDe,
  neuronaDeEvento, clasificarEvento,
  fusionarActores, mapaDeEncendido, RACIMO_SERVICIOS,
} from './personas.mjs';

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

// ── el pulso en vivo: un evento real del riel enciende UNA neurona, o ninguna ──

test('P13 · un principal declarado enciende SU terminal, y se marca exacta', () => {
  const n = neuronaDeEvento({ principal: 'davantis-altura', tool: 'musubi_sync_pull', kind: 'sondeo' });
  assert.equal(n.terminal, 'ALTURA');
  assert.equal(n.persona, 'davantis');
  assert.equal(n.exacta, true, 'hay tabla para este principal: la atribución es exacta');

  const m = neuronaDeEvento({ principal: 'davantis-mando-admin' });
  assert.equal(m.terminal, 'SALA DE MANDO', 'el sufijo -mando-admin es la sala de mando');
});

test('P14 · un servicio sin dueño declarado NO enciende nada ni inventa una neurona', () => {
  // El nombre de la terminal tiene que salir de la TABLA, nunca derivarse del principal.
  // Derivarlo —`persona.toUpperCase()` y listo— haría que `b1-adjudicador` encendiera una
  // neurona «B1» que no existe en el grafo, y `crm-cabina` una «CRM». Son servicios reales
  // cuyo dueño no se deduce del dato, y se prefiere no dibujar antes que dibujar una mentira.
  for (const p of ['b1-adjudicador', 'crm-cabina', 'algun-bot-nuevo', 'auditor-x']) {
    const n = neuronaDeEvento({ principal: p });
    assert.equal(n, null, `${p} no puede encender nada: no está declarado`);
  }
  assert.equal(neuronaDeEvento({ principal: '' }), null);
  assert.equal(neuronaDeEvento({}), null);
  assert.equal(neuronaDeEvento(null), null);
});

test('P15 · una credencial sin neurona propia PULSA EN SU PERSONA, y se declara inexacta', () => {
  // `davantis-admin` es el 28 % del tráfico medido y no tiene terminal propia. Cae en DAVANTIS
  // por REGLA DECLARADA (la credencial es de esa persona y la persona tiene neurona), pero
  // `exacta:false` existe para que el dibujo no afirme más de lo que sabe.
  const n = neuronaDeEvento({ principal: 'davantis-admin' });
  assert.equal(n.terminal, 'DAVANTIS');
  assert.equal(n.persona, 'davantis');
  assert.equal(n.exacta, false, 'es un repliegue, no una coincidencia: hay que poder distinguirlo');

  const g = neuronaDeEvento({ principal: 'gio' });
  assert.equal(g.terminal, 'GIO');
  assert.equal(g.exacta, true);
});

test('P16 · la capa la decide el SERVIDOR (`kind`), no el panel', () => {
  const s = clasificarEvento({ kind: 'sondeo', outcome: 'ok', ms: 0.25, tool: 'musubi_sync_pull' });
  assert.equal(s.capa, 'sondeo');
  assert.equal(s.falla, false);
  assert.equal(s.ms, 0.25);

  const t = clasificarEvento({ kind: 'trabajo', outcome: 'ok', ms: 2893, tool: 'musubi_save_observation' });
  assert.equal(t.capa, 'trabajo', 'todo lo que no es sondeo es trabajo');

  // un kind desconocido NO puede caer en «sondeo»: el sondeo se dibuja tenue, y esconder ahí
  // un evento que no sabemos qué es sería perder trabajo real en el ruido.
  assert.equal(clasificarEvento({ kind: 'loquesea' }).capa, 'trabajo');
  assert.equal(clasificarEvento({}).capa, 'trabajo');

  assert.equal(clasificarEvento({ outcome: 'error' }).falla, true);
  assert.equal(clasificarEvento({ outcome: 'rechazo' }).falla, true);
  assert.equal(clasificarEvento({ ms: -5 }).ms, 0, 'un ms negativo no puede volverse un grosor');
  assert.equal(clasificarEvento({ ms: 'x' }).ms, 0, 'ni un ms que no es número');
});

test('P12 · el calor suma el `heat` de las notas, y sin heat es 0 y no NaN', () => {
  // El calor es lo que hace latir a la neurona en el dibujo. Si una nota vieja no trae `heat`,
  // sumarla como undefined convierte el total en NaN — y un NaN en el radio del latido no se
  // ve como un error: se ve como una neurona que dejó de latir.
  const g = {
    neurons: [
      { id: '1', topic: 'x/y', gist: 'AUDITOR revisó algo', author: 'gio', heat: 5 },
      { id: '2', topic: 'x/y', gist: 'AUDITOR otra vez', author: 'gio', heat: 3 },
      { id: '3', topic: 'x/y', gist: 'AUDITOR sin heat', author: 'gio' },
      { id: '4', topic: 'x/y', gist: 'SKILLS nunca se leyó', author: 'gio', heat: 0 },
    ],
  };
  const { terminales } = extraerPersonas(g);
  const aud = terminales.find((t) => t.id === 'AUDITOR');
  assert.equal(aud.calor, 8, 'suma 5+3 y la nota sin heat aporta 0');
  const sk = terminales.find((t) => t.id === 'SKILLS');
  assert.equal(sk.calor, 0, 'heat 0 es un calor de 0, no ausencia');
  assert.ok(Number.isFinite(aud.calor) && Number.isFinite(sk.calor), 'jamás NaN');
});

test('P11 · una terminal que nadie firma cae a quien la menciona', () => {
  const g = { neurons: [n('1', 'x/y', 'Una nota que habla del REFUTADOR sin firmarlo', 'gio')] };
  const { terminales } = extraerPersonas(g);
  const r = terminales.find((t) => t.id === 'REFUTADOR');
  assert.equal(r.firmas, 0);
  assert.equal(r.persona, 'gio', 'sin firmas, la mención es la mejor respuesta disponible');
});

// ─────────────────────────────────────────────────────────────────────────────
// F2 · EL CENSO DE ACTORES ENTRA AL GRAFO
//
// El censo dice quién LLAMA; las terminales dicen quién ESCRIBE. Los tests de abajo cubren la
// costura entre las dos poblaciones, que es donde este grafo se puede poner a mentir.

// Un censo de juguete con la forma exacta de /api/actores.
const act = (principal, calls, extra = {}) => ({
  principal, calls, sondeo: 0, trabajo: calls, errors: 0, denied: 0, tools: 1, ...extra,
});

test('P17 · un principal declarado NO nace como nodo aparte: se le suma a su terminal', () => {
  const { terminales } = extraerPersonas(GRAFO);
  const { actores } = fusionarActores(terminales, { actores: [act('davantis-mando-admin', 900)] });
  assert.equal(actores.length, 0, 'la identidad ya está dibujada como terminal; un segundo nodo la cuenta dos veces');
  const mando = terminales.find((t) => t.id === 'SALA DE MANDO');
  assert.equal(mando.llamadas.calls, 900, 'el volumen tiene que llegar a la terminal, no perderse');
});

test('P18 · varias credenciales de la misma terminal se ACUMULAN', () => {
  const { terminales } = extraerPersonas(GRAFO);
  fusionarActores(terminales, {
    actores: [act('davantis-mando-admin', 900, { tools: 12 }), act('davantis-mando', 100, { tools: 5 })],
  });
  const mando = terminales.find((t) => t.id === 'SALA DE MANDO');
  assert.equal(mando.llamadas.calls, 1000, 'las dos credenciales son la misma sala de mando');
  assert.deepEqual(mando.llamadas.principales.sort(), ['davantis-mando', 'davantis-mando-admin']);
  // Las tools distintas NO se suman: 12 y 5 no son 17 tools, porque los conjuntos se solapan y
  // desde acá no se sabe cuánto. El máximo es el piso correcto; sumar mentiría para arriba.
  assert.equal(mando.llamadas.tools, 12, 'las tools distintas no se suman entre credenciales');
});

test('P19 · un servicio sin dueño va al racimo SERVICIOS, y su racimo NO sale de personaDe()', () => {
  const { terminales } = extraerPersonas(GRAFO);
  const { actores, sinDeclarar } = fusionarActores(terminales, {
    actores: [act('crm-cabina', 400), act('b1-adjudicador', 50)],
  });
  assert.equal(actores.length, 2);
  assert.equal(sinDeclarar, 2, 'el panel tiene que poder decir cuántos quedaron sin dueño');
  for (const a of actores) {
    assert.equal(a.persona, RACIMO_SERVICIOS, `${a.id} no tiene dueño declarado`);
    assert.equal(a.exacta, false, 'sin declaración, la atribución no es exacta');
    // El delator concreto: `personaDe('crm-cabina')` es «crm», que no es una persona sino el
    // prefijo de un servicio. Si ese string aparece como racimo, el grafo inventó una persona.
    assert.notEqual(a.persona, personaDe(a.principal));
  }
});

test('P20 · con censo, un servicio enciende SU nodo — que es lo que antes no podía pasar', () => {
  const { terminales } = extraerPersonas(GRAFO);
  const { actores } = fusionarActores(terminales, { actores: [act('crm-cabina', 400)] });
  const mapa = mapaDeEncendido(terminales, actores);

  const n = neuronaDeEvento({ principal: 'crm-cabina' }, mapa);
  assert.ok(n, 'con nodo propio, el evento tiene dónde caer');
  assert.equal(n.terminal, 'crm-cabina');
  assert.equal(n.tipo, 'actor', 'es un actor, no una terminal: no escribe, llama');

  // Y sin censo sigue sin encender nada, que era el comportamiento anterior. La diferencia la
  // hace el DATO, no una regla nueva que reparta a dedo.
  assert.equal(neuronaDeEvento({ principal: 'crm-cabina' }, mapaDeEncendido(terminales, [])), null);
});

test('P21 · al mapa sólo entra lo que EXISTE en el dibujo', () => {
  // Un grafo donde GIO no aparece: ninguna nota la nombra.
  const g = { neurons: [n('1', 'x/y', 'AUDITOR revisó algo', 'davantis-admin')] };
  const { terminales } = extraerPersonas(g);
  const mapa = mapaDeEncendido(terminales, []);
  assert.equal(mapa.directo.has('gio'), false, 'GIO está declarada en ACTORES pero no está dibujada');
  // Encender un nodo que nadie dibujó es un pulso que se pierde en silencio, y el contador de
  // «eventos sin neurona» deja de contar lo que dice contar.
  assert.equal(neuronaDeEvento({ principal: 'gio' }, mapa), null);
});

test('P22 · el censo apagado degrada a la tabla declarada, no a cero', () => {
  const { terminales } = extraerPersonas(GRAFO);
  const { actores, sinDeclarar } = fusionarActores(terminales, null);
  assert.deepEqual(actores, []);
  assert.equal(sinDeclarar, 0);
  const mapa = mapaDeEncendido(terminales, actores);
  const n = neuronaDeEvento({ principal: 'davantis-mando-admin' }, mapa);
  assert.equal(n && n.terminal, 'SALA DE MANDO', 'sin censo, lo declarado sigue encendiendo');
});

test('P23 · el racimo SERVICIOS va último aunque pese más, y no compite por notas', () => {
  const { terminales } = extraerPersonas(GRAFO);
  const { actores } = fusionarActores(terminales, {
    actores: [act('crm-cabina', 166371), act('b1-adjudicador', 9000)],
  });
  const racimos = agruparPorPersona(terminales, actores);
  const ultimo = racimos[racimos.length - 1];
  assert.equal(ultimo.persona, RACIMO_SERVICIOS, 'un racimo que no es una persona no se ordena entre personas');
  assert.equal(ultimo.nodos.length, 2);
  assert.equal(ultimo.notas, 0, 'los actores no escriben: no pueden pesar en el eje editorial');
  assert.equal(ultimo.llamadas, 175371, 'su volumen se cuenta, pero en su propia unidad');
  // Y las personas de arriba conservan su orden por notas: el racimo de servicios se saca de la
  // competencia, no la altera.
  const personas = racimos.slice(0, -1).map((r) => r.notas);
  assert.deepEqual(personas, [...personas].sort((a, b) => b - a));
});
