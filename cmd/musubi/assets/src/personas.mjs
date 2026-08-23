// personas.mjs — convierte el grafo de MEMORIA en el grafo de PERSONAS: quién escribe en el
// cerebro y quién le manda despachos a quién.
//
// Vive aparte de dashboard.mjs por la misma razón que layout.mjs: es lógica pura sobre datos,
// sin DOM ni WebGL, así que `node --test` la puede correr en CI. Y le hace falta: la mitad de
// lo que hace es arqueología de texto y se rompe en silencio si alguien cambia cómo firma.
//
// DE DÓNDE SALE CADA COSA, que no es lo mismo:
//   · la PERSONA sale del campo `author`, que es un dato real de la base (columna desde v16);
//   · la TERMINAL sale del TEXTO del gist, porque no existe como campo.
// Esa asimetría es el estado actual del sistema, no una preferencia. Mientras la terminal se
// firme a mano, este módulo es un parser y hay que tratarlo como tal.

// Los roles que se reconocen como terminal. Es una lista cerrada A PROPÓSITO: un extractor que
// acepte "cualquier palabra en mayúsculas antes de una flecha" convierte cada título enfático
// en una terminal fantasma, y el grafo se llena de nodos que no existen.
export const ROLES = [
  'PLANIFICADOR', 'REFUTADOR', 'SALA DE MANDO', 'PRINCIPAL', 'EMISARIO',
  'AUDITOR', 'SKILLS', 'CUERPO', 'ALTURA', 'DAVANTIS', 'GIO',
];

// Ordenados de más largo a más corto: si 'GIO' se probara antes que 'PLANIFICADOR', un texto
// con "PLANIFICADOR" nunca matchearía porque no contiene 'GIO'... pero al revés sí importa,
// porque buscamos por inclusión y un rol corto puede estar DENTRO de otro más largo.
const ROLES_LARGO = [...ROLES].sort((a, b) => b.length - a.length);

// personaDe: colapsa un `author` en la persona que hay detrás. Un mismo humano escribe con
// varias credenciales —davantis, davantis-admin, davantis-mando-admin, davantis-altura— y
// dibujarlas como cuatro personas distintas es exactamente el error que este grafo existe para
// no cometer. El corte es el primer guion: es la convención real de los tokens del central.
export function personaDe(author) {
  if (!author) return '';
  const i = author.indexOf('-');
  return (i === -1 ? author : author.slice(0, i)).toLowerCase();
}

const textoDe = (n) => `${n.topic || ''} ${n.gist || ''}`.toUpperCase();

// rolEn: qué terminal nombra este fragmento, o null. Devuelve UNA, la más larga que matchee.
function rolEn(txt) {
  for (const r of ROLES_LARGO) if (txt.includes(r)) return r;
  return null;
}

// Una flecha de despacho: "EMISARIO → PLANIFICADOR", "PRINCIPAL -> **SKILLS**".
// El lado derecho admite `·` porque los despachos a varios se escriben "A · B · C".
const FLECHA = /([A-ZÁÉÍÓÚÑ][A-ZÁÉÍÓÚÑ \-]{2,26})\s*(?:→|->)\s*\*{0,2}([A-ZÁÉÍÓÚÑ][A-ZÁÉÍÓÚÑ \-·]{2,40})/g;

/**
 * extraerPersonas: del grafo de memoria al grafo de personas.
 *
 * @param {{neurons?: Array}} grafo  el payload de /api/graph?lens=memory
 * @returns {{terminales: Array, despachos: Array, sinAutor: number}}
 *   terminales: [{id, notas, persona, autores}]  — `notas` es cuántas la nombran
 *   despachos:  [{de, a, veces}]                 — DIRIGIDO: de → a
 *   sinAutor:   cuántas notas no tienen `author` (legacy pre-v16). Se DECLARA en vez de
 *               esconderse: si el número es alto, el reparto por persona no es de fiar.
 */
export function extraerPersonas(grafo) {
  const neuronas = (grafo && grafo.neurons) || [];
  const term = new Map();   // rol -> {id, notas, autores:Map<persona,veces>}
  const desp = new Map();   // "de>a" -> veces
  let sinAutor = 0;

  for (const n of neuronas) {
    const txt = textoDe(n);
    const persona = personaDe(n.author);
    if (!persona) sinAutor++;

    // 1) cuántas notas nombran a cada terminal. Se cuenta UNA VEZ por nota y por rol, aunque
    //    el rol aparezca cinco veces en el mismo gist: si no, una nota larga que se repite a
    //    sí misma pesa como cinco notas y la terminal se infla sola.
    // FIRMAR es EMPEZAR el gist con el rol: «📮 EMISARIO → PLANIFICADOR, …». Nombrar una
    // terminal y escribir COMO esa terminal no es lo mismo, y confundirlos da respuestas
    // falsas: `ALTURA` la menciona más gio que Gabriel, así que por menciones el racimo se la
    // llevaba gio — cuando ALTURA es una terminal de Gabriel.
    //
    // Y tiene que ser EMPEZAR, no «aparecer cerca del principio»: con un umbral de los
    // primeros N caracteres, cualquier nota corta que mencione el rol cuenta como firma. Lo
    // medí: con 48 caracteres, las cuatro notas del test firmaban y sólo una lo hacía.
    const cabeza = (n.gist || '').toUpperCase().replace(/^[^A-ZÁÉÍÓÚÑ]+/, '');
    for (const r of ROLES) {
      if (!txt.includes(r)) continue;
      let t = term.get(r);
      if (!t) { t = { id: r, notas: 0, firmadas: 0, calor: 0, autores: new Map(), firmas: new Map() }; term.set(r, t); }
      t.notas++;
      // CALOR = cuántas veces se recuperaron las notas que la nombran (`heat` es access_count).
      // No es lo mismo que `notas`: mide cuánto se LEE lo que escribió, no cuánto escribió.
      // Medido sobre el cerebro local va de 0 (REFUTADOR) a 435 (AUDITOR), o sea tiene rango
      // real y sirve como canal. La recencia NO: las 11 terminales tienen su última nota a
      // menos de medio día, así que animar por recencia pintaría a todas igual.
      // Una nota que nombra a tres terminales suma su calor a las tres, igual que suma 1 nota
      // a las tres: es el mismo criterio de atribución, no un doble conteo.
      t.calor += (typeof n.heat === 'number' && n.heat > 0) ? n.heat : 0;
      // La FIRMA es un hecho del texto y se cuenta SIEMPRE, tenga autor la nota o no: con el
      // 65 % de la memoria local sin `author`, contarla sólo cuando hay autor haría parecer
      // que casi ninguna terminal escribe.
      if (cabeza.startsWith(r)) t.firmadas++;
      if (persona) {
        t.autores.set(persona, (t.autores.get(persona) || 0) + 1);
        if (cabeza.startsWith(r)) t.firmas.set(persona, (t.firmas.get(persona) || 0) + 1);
      }
    }

    // 2) los despachos. Sólo se mira si hay flecha: recorrer la regex sobre las 3 700 notas
    //    cuando el 95 % no tiene ninguna es trabajo tirado.
    if (!txt.includes('→') && !txt.includes('->')) continue;
    FLECHA.lastIndex = 0;
    let m;
    while ((m = FLECHA.exec(txt)) !== null) {
      const de = rolEn(m[1]);
      const a = rolEn(m[2]);
      if (!de || !a || de === a) continue;
      const k = de + '>' + a;
      desp.set(k, (desp.get(k) || 0) + 1);
    }
  }

  const terminales = [...term.values()].map((t) => ({
    id: t.id,
    notas: t.notas,
    firmas: t.firmadas,
    calor: t.calor,
    // La persona sale de quien FIRMA como esa terminal. Sólo si nadie firmó nunca —una
    // terminal que se nombra pero no escribe— se cae a quien más la menciona, que es una
    // respuesta peor pero mejor que ninguna.
    persona: mayoritaria(t.firmas) || mayoritaria(t.autores),
    autores: [...t.autores.entries()].sort((x, y) => y[1] - x[1]).map(([p, v]) => ({ persona: p, notas: v })),
  })).sort((a, b) => b.notas - a.notas || a.id.localeCompare(b.id));

  const despachos = [...desp.entries()]
    .map(([k, veces]) => { const [de, a] = k.split('>'); return { de, a, veces }; })
    .sort((x, y) => y.veces - x.veces || x.de.localeCompare(y.de) || x.a.localeCompare(y.a));

  return { terminales, despachos, sinAutor };
}

function mayoritaria(m) {
  let mejor = '', max = -1;
  // recorrido ordenado por clave para que un empate no dependa del orden de inserción
  for (const p of [...m.keys()].sort()) {
    const v = m.get(p);
    if (v > max) { max = v; mejor = p; }
  }
  return mejor;
}

/**
 * agruparPorPersona: las terminales, repartidas en racimos. El orden es estable (por cantidad
 * de notas y después alfabético) para que el dibujo no baile entre recargas.
 */
export function agruparPorPersona(terminales) {
  const por = new Map();
  for (const t of terminales) {
    const p = t.persona || '(sin autor)';
    if (!por.has(p)) por.set(p, []);
    por.get(p).push(t);
  }
  return [...por.entries()]
    .map(([persona, ts]) => ({
      persona,
      terminales: ts,
      notas: ts.reduce((s, t) => s + t.notas, 0),
    }))
    .sort((a, b) => b.notas - a.notas || a.persona.localeCompare(b.persona));
}
