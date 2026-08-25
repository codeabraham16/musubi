// datos.mjs — DEL GRAFO REAL A UN SOLO ÁRBOL CON TRONCO.
//
// Acá está la otra mitad del reclamo «pusiste todas las ramas juntas y nada más». Antes cada
// racimo era un arbolito suelto puesto en una esfera de Fibonacci alrededor del centro: cuatro
// bolas de ramas, sin tronco común y sin nada que dijera qué colgaba de qué. Sin una raíz
// compartida no hay jerarquía que mirar — hay cuatro jerarquías que no se tocan.
//
// Acá hay UN árbol:  memoria → actor → tema → subtema → …
//
// ─────────────────────────────────────────────────────────────────────────────────────────────
// MUSUBI ES UN ACTOR, NO UN CAJÓN DE SASTRE  (corrección pedida por el usuario)
//
// La versión anterior mandaba 1.437 de las 2.267 notas —el 63 %— a dos baldes grises llamados
// «libro mayor» y «(sin atribuir)». Los dos nombres mienten por implicación: el primero suena a
// archivo muerto y el segundo a nota huérfana, y NINGUNA de las dos cosas es cierta.
//
// Lo que hay ahí, medido sobre el cerebro local el 2026-08-25:
//
//   1.042  git-commit (541) + sdd (501)  ·  las escribió el MOTOR. Son notas de Musubi.
//     395  cuerpo-musubi, roadmap, gotchas, server, altura…  ·  notas del proyecto SIN firmar.
//           Sustantivas: «INVENTARIO FÍSICO del server casero (medido por SSH read-only)».
//
// Las dos son de Musubi, así que Musubi es un racimo con nombre y color propios. Pero se guarda
// la distinción UN NIVEL MÁS ABAJO, porque aplanarlas sería el mismo error al revés: lo que
// escribe un hook automáticamente y lo que escribió una persona y no quedó sellado no son la
// misma clase de conocimiento.
//
// UNA HIPÓTESIS MÍA QUE EL DATO REFUTÓ, y queda anotada para que nadie la reponga: supuse que las
// 395 sin firma eran ANTERIORES al sello de autor. No. La nota más vieja CON autor tiene 44,8 días
// (`c5looptest/write`, davantis), y de las 395 sin firma **351 —el 89 %— son POSTERIORES a eso**.
// El campo existía y quedaron sin él igual. O sea que hay un agujero de sellado vivo, no un
// residuo histórico, y arreglarlo NO es cosa del dibujo.

import { construirNodo } from '../src/arbol-memoria.mjs';
import { grupoDeNeurona, ordenarRacimos, GRUPO_LIBRO, GRUPO_SIN_ATRIBUIR } from '../src/personas.mjs';
import { PALETA, COLOR_MUSUBI, COLOR_TRONCO } from './comun.mjs';

export const RACIMO_MUSUBI = 'Musubi';
const RAMA_MOTOR = 'lo que escribió el motor';
const RAMA_SIN_FIRMA = 'sin firmar';

/**
 * armarRaiz: agrupa las memorias por actor y devuelve la raíz única.
 *
 * @returns {{raiz:object, racimos:Array, colorDe:Function}}
 */
export function armarRaiz(neuronas, opciones) {
  const o = opciones || {};
  const ns = (neuronas || []).filter(Boolean);

  // El racimo sale de la MISMA función que usa el panel — que los bocetos y la producción
  // difieran en de quién es una nota convertiría la comparación en una discusión sobre los datos.
  // Lo único que cambia acá es a DÓNDE van los dos baldes que no son personas.
  const porRacimo = new Map();
  const musubi = { motor: [], sinFirma: [] };
  for (const n of ns) {
    const g = grupoDeNeurona(n);
    const clave = (g && g.clave) || GRUPO_SIN_ATRIBUIR;
    if (clave === GRUPO_LIBRO) { musubi.motor.push(n); continue; }
    if (clave === GRUPO_SIN_ATRIBUIR) { musubi.sinFirma.push(n); continue; }
    if (!porRacimo.has(clave)) porRacimo.set(clave, []);
    porRacimo.get(clave).push(n);
  }

  const cuentas = {}; for (const [k, v] of porRacimo) cuentas[k] = v.length;
  const personas = ordenarRacimos([...porRacimo.keys()], cuentas);

  const colores = new Map();
  personas.forEach((nombre, i) => colores.set(nombre, PALETA[i % PALETA.length]));
  colores.set(RACIMO_MUSUBI, COLOR_MUSUBI);

  const hijos = personas.map((nombre) => {
    const sub = construirNodo(porRacimo.get(nombre), 0, nombre);
    sub.etiqueta = nombre; sub.racimo = nombre;
    return sub;
  });

  // MUSUBI VA ÚLTIMO aunque sea el más grande, por la misma regla que ya aplica el panel: las
  // personas primero. Ordenar por tamaño lo pondría al frente y afirmaría que trabajó más que
  // nadie, cuando lo que pasa es que un hook escribe una nota por cada commit.
  const ramasMusubi = [];
  if (musubi.motor.length) {
    const t = construirNodo(musubi.motor, 0, RAMA_MOTOR);
    t.etiqueta = RAMA_MOTOR; ramasMusubi.push(t);
  }
  if (musubi.sinFirma.length) {
    const t = construirNodo(musubi.sinFirma, 0, RAMA_SIN_FIRMA);
    t.etiqueta = RAMA_SIN_FIRMA; ramasMusubi.push(t);
  }
  const nMusubi = musubi.motor.length + musubi.sinFirma.length;
  if (nMusubi) {
    hijos.push({
      n: nMusubi, criterio: 'actor', etiqueta: RACIMO_MUSUBI,
      racimo: RACIMO_MUSUBI, hijos: ramasMusubi, mem: null,
    });
  }

  const raiz = {
    n: ns.length, criterio: 'actor', etiqueta: o.titulo || 'memoria',
    hijos, mem: null, racimo: '',
  };

  // El color se HEREDA del racimo hacia abajo: una rama de nivel 5 tiene que seguir diciendo de
  // quién es sin que haya que trazar el camino hasta arriba con el ojo. El tronco va apagado —
  // es lo único que no pertenece a nadie, y pintarlo claro lo volvía el objeto más brillante.
  const colorDe = (s) => (s.nivel === 0 ? COLOR_TRONCO : (colores.get(s.racimo) || COLOR_MUSUBI));

  const racimos = personas.map((nombre) => ({ nombre, color: colores.get(nombre), n: cuentas[nombre] }));
  if (nMusubi) racimos.push({
    nombre: RACIMO_MUSUBI, color: COLOR_MUSUBI, n: nMusubi,
    detalle: `${musubi.motor.length} del motor · ${musubi.sinFirma.length} sin firmar`,
  });

  return { raiz, colorDe, racimos };
}
