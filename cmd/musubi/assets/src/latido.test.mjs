// Invariantes del latido (latido.mjs). Corre en CI con `npm test`. La misma función la ejecuta en
// node cmd/musubi/panel_latido_node_test.go, que es la que lleva los sabotajes del arnés.
//
// Los huecos salen de la bajada espaciada (internal/mcp/bajada_ritmo.go): 300 s entre dos pedidos
// de una máquina quieta, y hasta 450 s cuando el candado de la bajada cambia de dueño.

import test from 'node:test';
import assert from 'node:assert/strict';
import { anotarSondeo, latido, UMBRAL_LATIDO_MS } from './latido.mjs';

const T0 = Date.parse('2026-09-27T12:00:00.000Z');
const S = 1000;
const iso = (ms) => new Date(ms).toISOString();

// recorrer: la lámpara segundo a segundo, como tictac, con cada sondeo llegando a su hora (y con su
// `at` igual a la llegada, salvo que el caso diga otra cosa).
function recorrer(sondeos, desde, hasta) {
  const pend = [...sondeos].sort((a, b) => a.llegada - b.llegada);
  const out = [];
  let ultimo = 0, i = 0;
  for (let t = desde; t <= hasta; t += S) {
    while (i < pend.length && pend[i].llegada <= t) {
      const s = pend[i++];
      ultimo = anotarSondeo(ultimo, 'at' in s ? s.at : iso(s.llegada), s.llegada);
    }
    out.push({ t, ...latido(ultimo, t) });
  }
  return out;
}

const cada = (paso, n, desde = T0) => Array.from({ length: n }, (_, k) => ({ llegada: desde + k * paso }));

test('L1 · con la bajada espaciada a 300 s la lámpara no se apaga nunca', () => {
  const r = recorrer(cada(300 * S, 13), T0, T0 + 3600 * S);
  const apagados = r.filter((m) => !m.vive);
  assert.equal(apagados.length, 0, 'se apagó en ' + apagados.length + ' de ' + r.length + ' segundos con un sondeo cada 5 min');
});

test('L2 · el peor hueco sano (450 s: tope, tick y lease) tampoco la apaga', () => {
  const llegadas = [0, 300, 750, 1050].map((s) => ({ llegada: T0 + s * S }));
  const r = recorrer(llegadas, T0, T0 + 1050 * S);
  assert.ok(r.every((m) => m.vive), 'se apagó en el hueco de 450 s');
});

test('L3 · un corte real la apaga a los 10 min del último sondeo, y no antes', () => {
  const ultimo = T0 + 600 * S;
  const r = recorrer(cada(300 * S, 3), T0, ultimo + 1200 * S);
  const primeroApagado = r.find((m) => !m.vive);
  assert.ok(primeroApagado, 'con 20 min de silencio la lámpara no se apagó nunca');
  const tardo = primeroApagado.t - ultimo;
  assert.ok(tardo > UMBRAL_LATIDO_MS && tardo <= UMBRAL_LATIDO_MS + S,
    'se apagó ' + tardo / S + ' s después del último sondeo; el umbral es ' + UMBRAL_LATIDO_MS / S + ' s');
  assert.ok(r.filter((m) => m.t >= primeroApagado.t).every((m) => !m.vive), 'volvió a prenderse sin ningún sondeo nuevo');
});

test('L4 · la hora es la del evento: el backlog viejo no la prende al abrir la página', () => {
  const r = recorrer([{ llegada: T0, at: iso(T0 - 20 * 60 * S) }], T0, T0 + 10 * S);
  assert.ok(r.every((m) => !m.vive), 'un sondeo de hace 20 min, llegado con el backlog, prendió la lámpara');
  assert.equal(r[0].texto, 'sin sondeo desde hace 20m');
});

test('L5 · un reloj del central adelantado no da antigüedad negativa, y un sondeo viejo no tapa a uno nuevo', () => {
  const adelantado = anotarSondeo(0, iso(T0 + 30 * S), T0);
  assert.equal(adelantado, T0, 'un `at` del futuro se recorta a la llegada');
  assert.deepEqual(latido(adelantado, T0), { vive: true, texto: 'sondeo · hace 0s' });
  const nuevo = anotarSondeo(0, iso(T0), T0);
  assert.equal(anotarSondeo(nuevo, iso(T0 - 3600 * S), T0 + S), nuevo, 'un sondeo viejo que llega tarde hizo retroceder la marca');
  assert.equal(anotarSondeo(0, undefined, T0), T0, 'sin `at` cuenta la llegada');
  assert.equal(anotarSondeo(0, 'no es una hora', T0), T0, 'con un `at` ilegible cuenta la llegada');
});

test('L6 · el texto no dice «sin sondeo» con la lámpara prendida, y dice hace cuánto', () => {
  assert.deepEqual(latido(0, T0), { vive: false, texto: 'sin sondeo' });
  const r = recorrer(cada(300 * S, 2), T0, T0 + 1500 * S);
  for (const m of r) {
    if (m.vive) assert.match(m.texto, /^sondeo · hace \d+[smh]$/);
    else assert.match(m.texto, /^sin sondeo desde hace \d+[smh]$/);
  }
  assert.equal(latido(T0, T0 + 45 * S).texto, 'sondeo · hace 45s');
  assert.equal(latido(T0, T0 + 4 * 60 * S).texto, 'sondeo · hace 4m');
});
