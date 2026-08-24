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

// ─────────────────────────────────────────────────────────────────────────────
// DE UN EVENTO EN VIVO A UNA NEURONA
//
// El riel del central emite una invocación por cada llamada a una tool:
//   {seq, at, tool, outcome, ms, principal, project, kind}
// El `principal` es una identidad de TOKEN, y las neuronas de este grafo son TERMINALES, que
// salen de la firma del gist. Son DOS POBLACIONES DISTINTAS que sólo se solapan en parte, y
// unirlas a ojo es la misma trampa que confundir firmar con mencionar.
//
// Por eso el mapeo es una TABLA DECLARADA y no una regla inferida. `personaDe()` sirve para
// colapsar credenciales de un humano, pero aplicada a los principales INVENTA personas:
// `b1-adjudicador` daría una persona «b1» y `crm-cabina` una persona «crm», y no son personas:
// son servicios. De quién es cada uno no se deduce del dato — `crm-cabina` ni siquiera declara
// proyecto. Mientras no se declare, NO pulsan, y el panel dice cuántos eventos quedaron sin
// neurona en vez de repartirlos a dedo.
// La protección contra inventar personas NO es un guardia aparte: es que esta tabla sea una
// LISTA CERRADA, igual que `ROLES`. `crm-cabina` no enciende nada porque no hay entrada `crm`,
// no porque haya un `if` que lo frene. Lo aprendí sacándole el guardia que tenía antes: el test
// pasaba igual, o sea el guardia era defensa en profundidad tapando el invariante.
export const ACTORES = {
  'davantis-mando-admin': 'SALA DE MANDO',
  'davantis-mando': 'SALA DE MANDO',
  'davantis-altura': 'ALTURA',
  'davantis': 'DAVANTIS',
  'gio': 'GIO',
};

/**
 * DUEÑOS: de quién es un principal que NO tiene terminal.
 *
 * Es una tabla APARTE de ACTORES y no una entrada más, porque responde otra pregunta. ACTORES
 * dice «esta credencial ES esta terminal»; ésta dice «esta credencial es DE esta persona». Un
 * servicio no es una terminal: no escribe, no firma, no tiene dendritas. Meterlo en ACTORES lo
 * haría encender la neurona de alguien que sí escribe, y el volumen de un poller se sumaría al
 * trabajo de un humano.
 *
 * ACÁ SÓLO ENTRA LO QUE ALGUIEN ESCRIBIÓ, con su cita. Lo que se deduce del nombre —cualquier
 * `davantis-*`— NO entra: para eso está el repliegue por persona, que se dibuja PUNTEADO
 * justamente para no afirmar con la misma tinta lo declarado y lo inferido.
 *
 * · `crm-cabina` → la APP del CRM. «la identidad con la que la APP del CRM lee el cerebro […]
 *   POR QUÉ NO LLEVA project_id: el project_id existe para ATRIBUIR lo que un principal ESCRIBE.
 *   Una cabina no escribe.» (arquitectura/cabina-crm, 2026-07-13). El `project_id` vacío NO era
 *   un dueño desconocido: era una decisión de diseño. Yo lo había leído al revés.
 * · `b1-adjudicador` → el adjudicador nocturno del server. «~/b1-workspace/mcp.json apunta a
 *   localhost:7717/mcp con Bearer del principal davantis b1-adjudicador»
 *   (server/b1-adjudicador-nocturno, 2026-07-23).
 * · `davantis-crm` → el DAEMON del repo del CRM, que sí escribe. «Son dos identidades con dos
 *   trabajos distintos y hay que no confundirlas» (misma nota que crm-cabina).
 * · `davantis-admin` → la credencial admin de la persona que administra el cerebro.
 */
export const DUENOS = {
  'crm-cabina': 'davantis',
  'b1-adjudicador': 'davantis',
  'davantis-crm': 'davantis',
  'davantis-admin': 'davantis',
};

/**
 * fusionarActores: el CENSO de quién llama, convertido en nodos del grafo.
 *
 * El censo (/api/actores) y las terminales son DOS POBLACIONES: una sale del token, la otra de
 * la firma del gist. Fusionarlas tiene tres casos y ninguno se resuelve adivinando:
 *
 *   1. principal DECLARADO en ACTORES y con terminal en el grafo → NO nace un nodo: el volumen
 *      se le suma a esa terminal. Son la misma identidad con dos naturalezas (escribe y llama),
 *      y dibujarla dos veces sería contar dos veces a la misma persona.
 *   2. principal no declarado pero cuya PERSONA sí lo está (`davantis-crm` → davantis) → nodo
 *      propio de tipo actor, en el racimo de esa persona, marcado `exacta:false` porque la
 *      atribución sale de la convención de nombres, no de una declaración.
 *   3. ni el principal ni su persona declarados (`b1-adjudicador`, `crm-cabina`) → nodo propio
 *      en el racimo SERVICIOS, sin persona. De quién son NO se deduce del dato: `crm-cabina` ni
 *      siquiera trae proyecto. Se queda ahí hasta que alguien lo declare, y el panel dice
 *      cuántos hay en vez de repartirlos a dedo.
 *
 * `personaDe()` NO se usa para nombrar el racimo del caso 3, y es el punto entero: aplicada a
 * `crm-cabina` devuelve «crm», que no es una persona sino el prefijo de un servicio. Ese es
 * exactamente el error que este grafo existe para no cometer.
 */
export const RACIMO_SERVICIOS = '(servicios)';

export function fusionarActores(terminales, censo) {
  const filas = (censo && censo.actores) || [];
  const porId = new Map((terminales || []).map((t) => [t.id, t]));
  const actores = [];
  let sinDeclarar = 0;

  for (const f of filas) {
    const principal = String((f && f.principal) || '').toLowerCase();
    if (!principal) continue;
    const llamadas = {
      principal,
      calls: Math.max(0, Number(f.calls) || 0),
      sondeo: Math.max(0, Number(f.sondeo) || 0),
      trabajo: Math.max(0, Number(f.trabajo) || 0),
      errores: Math.max(0, Number(f.errors) || 0) + Math.max(0, Number(f.denied) || 0),
      tools: Math.max(0, Number(f.tools) || 0),
      proyecto: String(f.project || ''),
    };

    // Caso 1: declarado y con terminal viva en el grafo.
    const declarada = ACTORES[principal];
    const term = declarada && porId.get(declarada);
    if (term) {
      // Un mismo humano tiene varias credenciales: se ACUMULAN sobre la terminal en vez de
      // pisarse. `davantis-mando` y `davantis-mando-admin` son las dos SALA DE MANDO.
      const prev = term.llamadas || { calls: 0, sondeo: 0, trabajo: 0, errores: 0, tools: 0, principales: [] };
      term.llamadas = {
        calls: prev.calls + llamadas.calls,
        sondeo: prev.sondeo + llamadas.sondeo,
        trabajo: prev.trabajo + llamadas.trabajo,
        errores: prev.errores + llamadas.errores,
        // Las tools distintas NO se suman: dos credenciales que llaman las mismas cinco tools
        // no tocan diez. Sin las filas crudas no se puede unir el conjunto, así que se queda
        // el máximo, que es el piso correcto y nunca miente para arriba.
        tools: Math.max(prev.tools, llamadas.tools),
        principales: [...(prev.principales || []), principal],
      };
      continue;
    }

    // Casos 2 y 3: nodo propio. El dueño DECLARADO gana sobre el que sugiere el nombre, y la
    // diferencia se conserva en `exacta` en vez de perderse: lo declarado se dibuja entero y lo
    // inferido punteado. Si las dos coinciden, igual manda la declaración — es la que alguien
    // se hizo cargo de escribir.
    const persona = personaDe(principal);
    const declarado = DUENOS[principal] || '';
    const porNombre = ACTORES[persona] ? persona : '';
    const suya = declarado || porNombre;
    if (!suya) sinDeclarar++;
    actores.push({
      id: principal, principal, tipo: 'actor',
      persona: suya || RACIMO_SERVICIOS,
      exacta: !!declarado,
      ...llamadas,
    });
  }

  actores.sort((a, b) => b.calls - a.calls || a.id.localeCompare(b.id));
  return { terminales: terminales || [], actores, sinDeclarar };
}

/**
 * mapaDeEncendido: qué neurona enciende cada principal. Se arma UNA vez al cargar el grafo y se
 * consulta por cada evento del riel — a 0,6 eventos/s da igual, pero a una ráfaga no.
 *
 * Devuelve DOS mapas y no uno porque son dos preguntas distintas: `directo` es «esta credencial
 * tiene su neurona» y `porPersona` es «esta credencial es de alguien que tiene neurona». La
 * segunda es un repliegue declarado y se marca inexacto; mezclarlas en un solo mapa haría
 * imposible distinguir el hecho de la convención.
 *
 * SÓLO ENTRA LO QUE EXISTE EN EL DIBUJO. Una terminal declarada en ACTORES que no está en el
 * grafo no se incluye: encender un nodo que nadie dibujó es un pulso que se pierde en silencio,
 * y el contador de «eventos sin neurona» deja de contar lo que dice contar. Lo aprendí con un
 * test que declaraba justo eso y pasaba igual, porque la tabla declarada seguía respondiendo
 * por atrás cuando el mapa decía que no.
 */
export function mapaDeEncendido(terminales, actores) {
  const directo = new Map(), porPersona = new Map();
  const hay = new Set((terminales || []).map((t) => t.id));
  for (const [principal, term] of Object.entries(ACTORES)) {
    if (!hay.has(term)) continue;
    directo.set(principal, { id: term, tipo: 'terminal' });
    // El repliegue sale de la entrada de la PERSONA (una clave sin guion), no de la primera
    // credencial que empiece con ese nombre. La diferencia es concreta y me mordió: iterando la
    // tabla, `davantis-mando-admin` aparece antes que `davantis`, así que un `davantis-*`
    // desconocido replegaba a SALA DE MANDO en vez de a DAVANTIS. El orden de un objeto no
    // puede decidir a quién se le atribuye trabajo.
    if (personaDe(principal) === principal) porPersona.set(principal, { id: term, tipo: 'terminal' });
  }
  for (const a of actores || []) directo.set(a.principal, { id: a.id, tipo: 'actor' });
  return { directo, porPersona };
}

// El mapa que se usa cuando NO hay censo: exactamente la tabla declarada, sin filtrar por lo
// dibujado. Es el comportamiento que había antes del censo, conservado a propósito para que
// «no llegó el censo» degrade a menos neuronas y nunca a neuronas inventadas.
function mapaDeclarado() {
  const directo = new Map(), porPersona = new Map();
  for (const [principal, term] of Object.entries(ACTORES)) {
    directo.set(principal, { id: term, tipo: 'terminal' });
    if (personaDe(principal) === principal) porPersona.set(principal, { id: term, tipo: 'terminal' });
  }
  return { directo, porPersona };
}

/**
 * neuronaDeEvento: qué neurona enciende una invocación, o null si no se puede saber.
 *
 * CUANDO HAY MAPA, EL MAPA MANDA. No hay un segundo camino que consulte la tabla declarada por
 * detrás: si el mapa dice que esa neurona no está dibujada, el evento queda sin neurona y se
 * cuenta como tal. Un repliegue silencioso a la tabla convierte «no se pudo atribuir» en un
 * pulso invisible sobre un nodo inexistente.
 *
 * El repliegue por PERSONA —un `davantis-*` sin neurona propia pulsa en la de `davantis`— es una
 * REGLA DECLARADA, no una inferencia: la credencial es de esa persona y la persona tiene neurona.
 * Se devuelve `exacta:false` para que el dibujo pueda mostrarlo distinto y no afirmar de más.
 */
export function neuronaDeEvento(ev, mapa) {
  const principal = String((ev && ev.principal) || '').toLowerCase();
  if (!principal) return null;
  const m = mapa || mapaDeclarado();

  const directa = m.directo.get(principal);
  if (directa) {
    return { terminal: directa.id, tipo: directa.tipo, persona: personaDe(principal), principal, exacta: true };
  }
  const persona = personaDe(principal);
  const propia = m.porPersona.get(persona);
  if (!propia) return null;
  return { terminal: propia.id, tipo: propia.tipo, persona, principal, exacta: false };
}

/**
 * clasificarEvento: las dos capas y si falló. `kind` lo decide el SERVIDOR (viene en el evento),
 * no este módulo: acá sólo se normaliza. Medido sobre 7 días, el 98,2 % es `sondeo` — por eso
 * las dos capas tienen que verse distintas o el trabajo real queda enterrado bajo el ruido.
 */
export function clasificarEvento(ev) {
  const kind = String((ev && ev.kind) || '').toLowerCase();
  const outcome = String((ev && ev.outcome) || '').toLowerCase();
  const ms = Number(ev && ev.ms);
  return {
    capa: kind === 'sondeo' ? 'sondeo' : 'trabajo',
    falla: outcome !== '' && outcome !== 'ok',
    ms: Number.isFinite(ms) && ms >= 0 ? ms : 0,
    tool: String((ev && ev.tool) || ''),
  };
}

// ─────────────────────────────────────────────────────────────────────────────
// DE UNA MEMORIA A SU GRUPO: quién la escribió, o por qué no es de nadie.
//
// Esto es lo que reemplaza al agrupamiento por DOMINIO en la escena principal. Y hay un dato
// que obliga a que sean TRES clases y no una: medido sobre las 2.217 neuronas del cerebro
// local el 2026-08-24, sólo 802 (36,2 %) traen `author`. Agrupar por persona a secas dejaría
// el 62 % en una mancha gris llamada «sin autor», que es menos informativa que el dominio que
// vino a reemplazar.
//
// La salida es que 1.027 de esas 1.415 huérfanas (72 %) NO son la memoria de nadie: son
// `git-commit` y `sdd/`, los dos géneros que ESCRIBE EL PROPIO MOTOR. Musubi ya los nombra
// LIBRO MAYOR en internal/config/config.go:355 — «se leen y se citan, pero nadie puede pedir un
// veredicto que los REEMPLACE» —, así que acá se los trata igual: un racimo propio, declarado,
// que no compite entre personas. Mismo criterio que el racimo `(servicios)` de los actores.

// Los dos géneros que escribe el motor. Es una LISTA CERRADA y sólo tiene lo que Musubi produce
// por su cuenta. Los géneros que inventa cada equipo —actas, bitácoras, correspondencia— son
// convención del DESPLIEGUE y viven en `conflicts.ledger_prefixes` del config; el panel no los
// ve, así que caen en «sin atribuir» y se cuentan ahí. Es una limitación conocida, no un olvido:
// meterlos acá sería hardcodear la costumbre de un usuario adentro del producto, que es
// exactamente lo que ese config existe para no hacer.
export const GENEROS_DEL_MOTOR = ['git-commit', 'sdd'];
export const GRUPO_LIBRO = 'libro mayor';
export const GRUPO_SIN_ATRIBUIR = '(sin atribuir)';

/**
 * grupoDeNeurona: a qué racimo va una memoria.
 *
 * EL GÉNERO GANA SOBRE EL AUTOR, y es deliberado. Un `git-commit` es el registro del repo aunque
 * la fila traiga quién lo firmó: si el autor mandara, el día que se rellene ese campo para los
 * 524 commits el racimo entero se mudaría de golpe al de una persona. Con el género primero, el
 * agrupamiento no depende de un backfill.
 *
 * @returns {{clave: string, tipo: 'persona'|'libro'|'sin'}}
 */
export function grupoDeNeurona(n) {
  const topic = String((n && n.topic) || '');
  const dominio = String((n && n.domain) || '');
  for (const g of GENEROS_DEL_MOTOR) {
    if (dominio === g || topic === g || topic.startsWith(g + '/')) {
      return { clave: GRUPO_LIBRO, tipo: 'libro' };
    }
  }
  const persona = personaDe(n && n.author);
  if (persona) return { clave: persona, tipo: 'persona' };
  return { clave: GRUPO_SIN_ATRIBUIR, tipo: 'sin' };
}

/**
 * ordenarRacimos: las personas primero y por tamaño; después el LIBRO MAYOR; último lo que no se
 * pudo atribuir. Los dos racimos que no son personas van al final SIEMPRE, aunque pesen más —
 * ordenarlos por tamaño entre las personas afirmaría que lo son. Es la misma regla que
 * `agruparPorPersona` aplica al racimo de servicios, y por el mismo motivo.
 */
export function ordenarRacimos(claves, cuenta) {
  const rango = (k) => (k === GRUPO_SIN_ATRIBUIR ? 2 : k === GRUPO_LIBRO ? 1 : 0);
  return [...claves].sort((a, b) =>
    rango(a) - rango(b) || (cuenta[b] || 0) - (cuenta[a] || 0) || String(a).localeCompare(String(b)));
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
 * agruparPorPersona: los nodos del grafo, repartidos en racimos. El orden es estable (por
 * cantidad de notas y después alfabético) para que el dibujo no baile entre recargas.
 *
 * Cada racimo trae `nodos`, que son terminales Y actores mezclados: en el dibujo son vecinos
 * de la misma persona y no dos capas separadas. El TIPO va en cada nodo (`tipo`), que es donde
 * tiene que estar — separarlos en dos listas obligaría a cada consumidor a volver a unirlos.
 *
 * El racimo SERVICIOS va SIEMPRE ÚLTIMO, aunque tenga más volumen que varias personas. No es
 * una persona: ordenarlo entre ellas por tamaño afirmaría que lo es, y de quién son esos
 * servicios es justamente lo que todavía no está declarado.
 */
export function agruparPorPersona(terminales, actores) {
  const por = new Map();
  const meter = (clave, nodo) => {
    if (!por.has(clave)) por.set(clave, []);
    por.get(clave).push(nodo);
  };
  for (const t of terminales || []) meter(t.persona || '(sin autor)', { ...t, tipo: 'terminal' });
  for (const a of actores || []) meter(a.persona || RACIMO_SERVICIOS, a);

  return [...por.entries()]
    .map(([persona, ns]) => ({
      persona,
      nodos: ns,
      // `notas` sigue siendo el peso EDITORIAL del racimo: cuánto se escribió. Los actores no
      // escriben, así que no suman acá — un poller con 160.000 llamadas no hace grande a nadie
      // en la memoria. Su volumen se ve en el tamaño de SU nodo, que es donde significa algo.
      notas: ns.reduce((s, x) => s + (x.tipo === 'terminal' ? x.notas : 0), 0),
      llamadas: ns.reduce((s, x) => s + (x.tipo === 'actor' ? x.calls : (x.llamadas ? x.llamadas.calls : 0)), 0),
    }))
    .sort((a, b) => {
      if (a.persona === RACIMO_SERVICIOS) return 1;
      if (b.persona === RACIMO_SERVICIOS) return -1;
      return b.notas - a.notas || a.persona.localeCompare(b.persona);
    });
}
