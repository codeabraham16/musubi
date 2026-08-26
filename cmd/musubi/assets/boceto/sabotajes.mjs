// sabotajes.mjs — CADA INVARIANTE, VISTO FALLAR.
//
// Un test que pasa no prueba nada por sí solo: prueba que pasa. Lo que dice si sirve es verlo
// FALLAR cuando se rompe justo lo que declara defender. Este banco rompe el código a propósito,
// una cosa por vez, y exige el rojo.
//
// DOS TRAMPAS QUE YA NOS MORDIERON, y por eso el banco es más estricto de lo que parece:
//
//   1. UN PATRÓN QUE NO MATCHEA NINGÚN TEST sale con código 0, y «no falló» se lee igual que
//      «el invariante aguantó». Por eso se exige ver `tests 1` en la salida: si el filtro no
//      corrió exactamente un test, el sabotaje se reporta SIN TEST y cuenta como error.
//   2. LA DEFENSA EN PROFUNDIDAD tapa el sabotaje: si lo mismo se valida en dos lugares, romper
//      uno no cambia nada y el test queda vacuo pareciendo sano. Por eso cada sabotaje ataca el
//      invariante que el test DECLARA, no una validación cualquiera cerca.
//
// Correr:  node sabotajes.mjs

import { readFileSync, writeFileSync, copyFileSync, unlinkSync } from 'node:fs';
import { execFileSync } from 'node:child_process';

const OBJ = 'comun.mjs';
const BAK = 'comun.mjs.sabotaje-bak';

const SABOTAJES = [
  {
    test: 'B1',
    que: 'el frente deja de avanzar (devuelve siempre lo mismo)',
    de: '  const f = (Math.max(0, ms) / 1000) * vel;',
    a: '  const f = 1;',
  },
  {
    test: 'B2',
    que: 'el frente NUNCA se apaga: la luz queda prendida para siempre',
    de: '  return f > max + ancho ? -1 : f;',
    a: '  return f;',
  },
  {
    test: 'B3',
    que: 'el apagado ignora el ancho del frente y corta la cola',
    de: '  return f > max + ancho ? -1 : f;',
    a: '  return f > max ? -1 : f;',
  },
  {
    test: 'B4',
    que: 'el grosor deja de salir de la carga',
    de: 'export const radioRall = (n, r0) => r0 * Math.pow(Math.max(1, n), 1 / EXP_RALL);',
    a: 'export const radioRall = (n, r0) => r0 * 2;',
  },
  {
    test: 'B5',
    que: 'el recorte NO se declara: lo que no entra desaparece en silencio',
    de: '      s.absorbidas = nodo.n;',
    a: '      s.absorbidas = s.memorias.length;',
  },
  {
    test: 'B5',
    que: 'el cero vuelve a leerse como ausente en maxNivel',
    de: '  const maxNivel = num(o.maxNivel, 7);',
    a: '  const maxNivel = Number(o.maxNivel) || 7;',
  },
  {
    test: 'B6',
    que: 'la carga deja de ser la suma real del subárbol',
    de: '      carga: nodo.n,',
    a: '      carga: nodo.n + (nodo.hijos && nodo.hijos.length ? 3 : 0),',
  },
  {
    test: 'B7',
    que: 'el penacho brota también de hilos que no terminan en una hoja',
    de: '    if (!e.ultimo || secciones[e.sec].hijos.length) continue;',
    a: '    if (!e.ultimo) continue;',
  },
  {
    test: 'B8',
    que: 'un NaN se cuela en la geometría del hilo (desaparece sin error)',
    de: '  const co = Math.cos(ph) * rw, si = Math.sin(ph) * rw;',
    a: '  const co = Math.cos(ph) * rw, si = Math.sin(ph) * rw * (t > 0.9 ? NaN : 1);',
  },
  {
    test: 'B9',
    que: 'el núcleo vuelve a medir lo mismo que un actor',
    de: '  raiz.largo = largoNucleo;',
    a: '  raiz.largo = L0 * 3;',
  },
  {
    test: 'B10',
    que: 'el árbol deja de ser determinista',
    de: 'export const rng = (s) => () => ((s = (s * 1664525 + 1013904223) >>> 0) / 4294967296);',
    a: 'export const rng = () => () => Math.random();',
  },

  /* ── LA BIFURCACIÓN: que las hermanas no nazcan del mismo punto ──────────────────────────── */
  {
    test: 'B17',
    que: 'las hermanas vuelven a nacer todas de la punta del padre',
    de: '    cuna[i] = Math.max(1 - naciente, 1 - atras / L);',
    a: '    cuna[i] = 1;',
  },
  {
    test: 'B18',
    que: 'el ángulo vuelve a ser una constante que ignora el grosor',
    de: '    return 2 * Math.asin(Math.min(1, pide / (2 * ref)));   // clamp: aca es donde saldria NaN',
    a: '    return 0.5;',
  },
  {
    test: 'B19',
    que: 'el apretón no se declara: se aprieta en silencio',
    de: '  const esc = (necesita > disponible && necesita > 0) ? disponible / necesita : 1;',
    a: '  const esc = 1;',
  },
  {
    test: 'B20',
    que: 'el escalón deja de normalizarse y cada nivel encoge la escena',
    de: '  for (let i = 0; i < k; i++) escalon[i] = Math.exp(escalon[i] - suma / k);',
    a: '  for (let i = 0; i < k; i++) escalon[i] = Math.exp(escalon[i]);',
  },
  {
    test: 'B21',
    que: 'todas las hermanas salen pegadas al eje del padre: vuelve la maraña',
    de: '    polar[i] = q === 0 ? (k === 1 ? 0 : polar0) : anillo;',
    a: '    polar[i] = polar0;',
  },
  {
    test: 'B22',
    que: 'el radio del haz se calcula con DOS fórmulas que divergen',
    de: '    s.Rhaz = radioHaz(f, rFib, sep);                   // el radio del HAZ, con piso, compartido',
    a: '    s.Rhaz = Math.max(rFib * 1.6, R) * 1.2;',
  },

  /* ── EL HILO ES DE ALGUIEN, Y LA RELACION VIAJA POR EL ARBOL ─────────────────────────────── */
  {
    test: 'B23',
    que: 'casi ningun hilo llega a saber a que hoja va',
    de: '    for (let j = 0; j < (s.fibras || 1); j++) dest[r0 + j] = s.idx;',
    a: '    dest[r0] = s.idx;',
  },
  {
    test: 'B24',
    que: 'los actores vuelven a salir en anillos: el mas grande se queda con toda la cascara',
    de: '  if (f <= 2) return 1;',
    a: '  if (f > 0) return 1;',
  },
  {
    test: 'B25',
    que: 'la relacion deja de tocar sus botones: arranca cerca, no encima',
    de: '  const C = [P[0], P[0], ...P, P[n - 1], P[n - 1]];',
    a: '  const C = [...P];',
  },
  {
    test: 'B26',
    que: 'la relacion vuelve a ser una cuerda recta que atraviesa el tejido',
    de: '    for (let c = 0; c < 3; c++) P[i][c] = beta * P[i][c] + (1 - beta) * (pA[c] + (pB[c] - pA[c]) * t);',
    a: '    for (let c = 0; c < 3; c++) P[i][c] = pA[c] + (pB[c] - pA[c]) * t;',
  },

  /* ── LAS CINCO FORMAS ────────────────────────────────────────────────────────────────────── */
  {
    test: 'B27',
    que: 'las hojas de la corona dejan de estar en el anillo',
    de: '    : R);',
    a: '    : hueco + (R - hueco) * 0.5);',
  },
  {
    test: 'B28',
    que: 'el orden de las hojas se mezcla: un subarbol queda partido en arcos sueltos',
    de: '    if (!s.hijos.length) { hojas.push(i); return; }',
    a: '    if (!s.hijos.length) { if (i % 3) hojas.push(i); else hojas.unshift(i); return; }',
  },
  {
    test: 'B29',
    que: 'el abanico plano pierde su paso: todas las hermanas salen en la misma direccion',
    de: '      const pasoP = Math.min(anilloDe(B), apMaxPlano / brazos);',
    a: '      const pasoP = 0;',
  },
  {
    test: 'B30',
    que: 'el tramo deja de cortarse en la cascara y la atraviesa',
    de: '          const t = (-bd + Math.sqrt(disc)) / 1.05;',
    a: '          const t = l * 4;',
  },

  /* ── EL NUDO: la fusión ──────────────────────────────────────────────────────────────────── */
  {
    test: 'B31',
    que: 'el treemap corta siempre por el mismo lado: las parcelas salen como tiras',
    de: '      if (arcoLat >= arcoLon) {',
    a: '      if (true) {',
  },
  {
    test: 'B32',
    que: 'la hoja deja de aterrizar en el radio: el borde vuelve a ser disparejo',
    de: '        if (raiz2 >= 0) l = Math.max(l * 0.25, (-qd + Math.sqrt(raiz2)) / 1.05);',
    a: '        if (raiz2 >= 0) l = l;',
  },


  {
    test: 'B33',
    que: 'vuelve la rampa: las hermanas nacen apuntando para el mismo lado y se pliegan',
    de: '  const rampa = Math.max(0, Math.min(1, num(o.rampa, 0)));',
    a: '  const rampa = 1;',
  },

  /* ── EL COLOR ────────────────────────────────────────────────────────────────────────────── */
  {
    test: 'D1',
    que: 'un actor vuelve a venir mas saturado que los otros y pesa mas sin significar nada',
    de: "export const PALETA = ['#5bc8ba', '#c85b93', '#5b85c8', '#9c5bc8', '#5badc8', '#6d5bc8',",
    a: "export const PALETA = ['#2dd4bf', '#f472b6', '#5b85c8', '#9c5bc8', '#5badc8', '#6d5bc8',",
  },
  {
    test: 'D2',
    que: 'el jitter vuelve al TONO: el haz se lee como confeti y se ensucia la identidad',
    de: '  return { h: 0, s: 0, l: (v - 0.5) * 0.34 };',
    a: '  return { h: (v - 0.5) * 0.085, s: 0, l: (v - 0.5) * 0.34 };',
  },

  /* ── LA CAMARA ───────────────────────────────────────────────────────────────────────────── */
  {
    test: 'C3',
    que: 'la rueda deja de ser progresiva y salta al valor final en el primer paso',
    de: '    meta.dist = Math.max(MIN, Math.min(MAX, meta.dist * Math.exp(ev.deltaY * 0.0011)));',
    a: '    meta.dist = Math.max(MIN, Math.min(MAX, meta.dist * Math.exp(ev.deltaY * 0.011)));',
  },
  {
    test: 'C4',
    que: 'el zoom deja de ir hacia el cursor: el punto se corre de abajo del puntero',
    de: '    meta.foco.addScaledVector(_der, nx * tan * (camera.aspect || 1) * d)',
    a: '    meta.foco.addScaledVector(_der, 0)',
  },


  /* ── LOS HILOS: lo que se agregó en la vuelta anterior ───────────────────────────────────── */
  {
    test: 'B11',
    que: 'un hilo aparece de la nada en cada bifurcación',
    de: '      s.fibras = t;                     // ← LA CONSERVACIÓN. No es una estimación: es una suma.',
    a: '      s.fibras = t + 1;',
  },
  {
    test: 'B12',
    que: 'las ranuras se pisan: dos hijas creen tener los mismos hilos',
    de: '    for (const h of s.hijos) { secciones[h].ranura = off; off += secciones[h].fibras; }',
    a: '    for (const h of s.hijos) { secciones[h].ranura = off; off += 1; }',
  },
  {
    test: 'B13',
    que: 'el grosor deja de contarse: toda hoja lleva un solo hilo',
    de: '      s.fibras = Math.max(1, Math.min(maxHoja, Math.ceil(s.carga / porMemoria)));',
    a: '      s.fibras = 1;',
  },
  {
    test: 'B14',
    que: 'desaparece la hendidura: las neuronas de un hilo se tocan',
    de: '        const tb = t1 - (t1 - t0) * hendidura;',
    a: '        const tb = t1;',
  },
  {
    test: 'B15',
    que: 'vuelve a ser un árbol: todos los actores salen para arriba',
    de: '  const y = m === 1 ? 0 : 1 - (2 * (k + 0.5)) / m;',
    a: '  const y = 0.9;',
  },
  {
    test: 'E1',
    que: 'el enlace a otra forma PIERDE el cerebro y te devuelve al local en silencio',
    de: '  return !c || c === CEREBROS[0] ? href : `${href}?cerebro=${c.id}`;',
    a: '  return href;',
  },
  {
    test: 'E2',
    que: 'la lista deja de ser cerrada: un id cualquiera se convierte en un archivo a pedir',
    de: "  return CEREBROS.find((c) => c.id === q) || CEREBROS[0];",
    a: "  return q ? { id: q, nombre: q, archivo: `./grafo-${q}.json` } : CEREBROS[0];",
  },
  {
    test: 'F1',
    que: 'la alfa deja de repartirse: seis veces más relaciones, seis veces más luz, centro lavado',
    de: '  return Math.max(TINTA_SINAPSIS.piso, TINTA_SINAPSIS.referencia / n);',
    a: '  return 1;',
  },
  {
    test: 'F2',
    que: 'se va el piso: con muchas relaciones la capa se desvanece y dice que no hay ninguna',
    de: '  return Math.max(TINTA_SINAPSIS.piso, TINTA_SINAPSIS.referencia / n);',
    a: '  return TINTA_SINAPSIS.referencia / n;',
  },
  {
    test: 'G1',
    que: 'las hijas vuelven a nacer DENTRO del núcleo y los actores se atraviesan',
    de: '    return Math.min(tapa, panza);',
    a: '    return nucleo * 0.55;',
  },
  {
    test: 'G2',
    que: 'el núcleo vuelve a ser un disco: más ancho que largo, o sea una cara cortada',
    de: '  const largoNucleo = Math.max(nucleo, 2 * rNucleo);',
    a: '  const largoNucleo = nucleo;',
  },
];

const original = readFileSync(OBJ, 'utf8');
copyFileSync(OBJ, BAK);
let errores = 0;

console.log('BANCO DE SABOTAJES · ' + SABOTAJES.length + ' invariantes\n');

for (const s of SABOTAJES) {
  const veces = original.split(s.de).length - 1;
  if (veces !== 1) {
    console.log(`  ✗ ${s.test}  ANCLA AMBIGUA (${veces} coincidencias) — ${s.que}`);
    errores++;
    continue;
  }
  writeFileSync(OBJ, original.replace(s.de, s.a), 'utf8');
  let salida = '';
  let fallo = false;
  try {
    salida = execFileSync(process.execPath,
      ['--test', '--test-name-pattern', '^' + s.test + ' ', 'comun.test.mjs'],
      { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] });
  } catch (e) {
    salida = String(e.stdout || '') + String(e.stderr || '');
    fallo = true;
  }
  writeFileSync(OBJ, original, 'utf8');

  // ¿CORRIÓ EXACTAMENTE UN TEST? Sin esto, un patrón que no matchea nada sale con código 0 y
  // «no falló» se lee igual que «el invariante aguantó». Ya nos pasó.
  const m = /^# tests (\d+)$/m.exec(salida) || /tests (\d+)/.exec(salida);
  const corridos = m ? Number(m[1]) : 0;
  if (corridos !== 1) {
    console.log(`  ✗ ${s.test}  SIN TEST (corrieron ${corridos}) — ${s.que}`);
    errores++;
  } else if (!fallo) {
    console.log(`  ✗ ${s.test}  VACUO: el sabotaje NO lo hizo fallar — ${s.que}`);
    errores++;
  } else {
    console.log(`  ✓ ${s.test}  falla como debe — ${s.que}`);
  }
}

try { unlinkSync(BAK); } catch (_) {}
// PARANOIA: el archivo tiene que haber quedado EXACTAMENTE como estaba. Un banco de sabotajes que
// deja el sabotaje puesto es la peor herramienta posible.
if (readFileSync(OBJ, 'utf8') !== original) {
  console.log('\n🔴 EL ARCHIVO NO QUEDÓ COMO ESTABA');
  process.exit(2);
}
console.log(`\n${SABOTAJES.length - errores}/${SABOTAJES.length} invariantes verificados fallando`);
console.log('comun.mjs restaurado byte a byte');
process.exit(errores ? 1 : 0);
