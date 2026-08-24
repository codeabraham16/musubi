// dendritas.mjs — la GEOMETRÍA de un árbol dendrítico, sin motor de dibujo.
//
// Vive aparte de quien la dibuja por la misma razón que layout.mjs: es matemática pura sobre
// números, así que `node --test` la corre en CI. Y le hace falta, porque de acá salen decenas de
// miles de instancias y un error de conteo se ve como «se puso lento» sin que nadie sepa por qué.
//
// LA RAMA ES LA ÚNICA LICENCIA DEL DIBUJO, Y SE DECLARA. Cuántas raíces tiene una neurona y qué
// tan gruesa nace SÍ salen del dato (las notas que esa terminal firmó). Hacia dónde se dobla cada
// rama NO: sale de un PRNG semillado. Por eso la semilla es obligatoria y determinista — con
// Math.random(), dos personas mirando la misma pantalla verían árboles distintos y no podrían
// hablar de lo que ven, y una captura de ayer dejaría de comparar con la de hoy.

const add = (a, b) => [a[0] + b[0], a[1] + b[1], a[2] + b[2]];
const mul = (a, k) => [a[0] * k, a[1] * k, a[2] * k];
const norm = (a) => { const l = Math.hypot(a[0], a[1], a[2]) || 1; return [a[0] / l, a[1] / l, a[2] / l]; };
const cross = (a, b) => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]];

// PRNG semillado. Mismo generador que usaba el lienzo 2D, para que el salto de motor no cambie
// la FORMA de los árboles: si cambiara, sería imposible saber si lo que se ve distinto es el
// render nuevo o un árbol nuevo.
export const rng = (s) => () => ((s = (s * 1664525 + 1013904223) >>> 0) / 4294967296);

// desviar: gira una dirección `ang` radianes hacia un lado cualquiera del plano perpendicular.
function desviar(d, ang, r) {
  const up = Math.abs(d[1]) > 0.9 ? [1, 0, 0] : [0, 1, 0];
  const e1 = norm(cross(d, up)), e2 = norm(cross(d, e1));
  const t = r() * Math.PI * 2;
  const lat = add(mul(e1, Math.cos(t)), mul(e2, Math.sin(t)));
  return norm(add(mul(d, Math.cos(ang)), mul(lat, Math.sin(ang))));
}

/**
 * arbol: los segmentos de UNA neurona.
 *
 * @param {{id:string, notas:number, centro:number[], escala:number, tope:number}} t
 * @returns {{segs:Array, alcance:number, alcanceRama:number}}
 *   segs: [{a:[x,y,z], b:[x,y,z], w0, w1, nivel, dist}] — `w0`/`w1` son el grosor al empezar y
 *   al terminar el tramo: el ADELGAZAMIENTO es lo que hace que una rama se lea como dendrita y
 *   no como un palito, y por eso viaja por segmento y no se deduce del nivel.
 *   `dist` es el camino recorrido DESDE EL SOMA a lo largo de la rama, no en línea recta: es lo
 *   que le permite a un impulso propagarse como un frente por el árbol en vez de como una onda
 *   esférica que encendería a la vez ramas que están a distinto camino.
 */
export function arbol(t) {
  const notas = Math.max(0, Number(t && t.notas) || 0);
  const centro = (t && t.centro) || [0, 0, 0];
  const escala = Math.max(0.001, Number(t && t.escala) || 1);
  // TOPE DURO de segmentos por árbol. No es prudencia decorativa: 11 troncos sin techo pasaban
  // de 100.000 instancias, y aunque WebGL las dibuje en una sola llamada, generarlas y subirlas
  // cuesta en cada reconstrucción del grafo.
  const tope = Math.max(16, Number(t && t.tope) || 2000);

  // El soma escala con las notas y las raíces con su logaritmo: 232 notas no pueden dar 232
  // raíces (sería un erizo) pero tampoco las mismas que 11.
  const rSoma = Math.max(1.4, Math.sqrt(notas) * 0.42) * escala;
  const raices = Math.max(4, Math.min(9, Math.round(Math.log(notas + 1) * 1.7)));
  const prof = notas > 150 ? 6 : (notas > 55 ? 5 : 4);
  const L0 = rSoma * (notas > 120 ? 2.4 : 2.9);
  const w0 = Math.max(0.28, rSoma * 0.30);

  const segs = [];
  let alcanceRama = rSoma;
  const r = rng((Number(t && t.semilla) || 1) >>> 0);

  function crecer(p, d, largo, w, nivel, dist) {
    if (nivel >= prof || w < 0.035 * escala || segs.length >= tope) return;
    const hijos = r() < 0.28 ? 3 : 2;
    for (let i = 0; i < hijos; i++) {
      if (segs.length >= tope) return;
      const d2 = desviar(d, 0.38 + r() * 0.42, r);
      const l2 = largo * (0.66 + r() * 0.16);
      const q = add(p, mul(d2, l2));
      const wf = w * 0.62;
      const hasta = dist + l2;
      if (hasta > alcanceRama) alcanceRama = hasta;
      segs.push({ a: p, b: q, w0: w, w1: wf, nivel, dist: hasta });
      crecer(q, d2, l2, wf, nivel + 1, hasta);
    }
  }

  for (let i = 0; i < raices; i++) {
    const k = i + 0.5, phi = Math.acos(1 - 2 * k / raices), th = Math.PI * (1 + Math.sqrt(5)) * k;
    const d0 = norm([Math.cos(th) * Math.sin(phi), Math.cos(phi), Math.sin(th) * Math.sin(phi)]);
    crecer(add(centro, mul(d0, rSoma * 0.85)), d0, L0, w0, 0, rSoma * 0.85);
  }

  let alcance = rSoma;
  for (const s of segs) {
    const d = Math.hypot(s.b[0] - centro[0], s.b[1] - centro[1], s.b[2] - centro[2]);
    if (d > alcance) alcance = d;
  }
  return { segs, alcance, alcanceRama, rSoma };
}

/**
 * bosque: los árboles de todos los troncos, con sus somas repartidos DENTRO del racimo.
 *
 * Los troncos se colocan en una esfera de Fibonacci alrededor del centro del racimo, y el que se
 * llama como la persona va casi al medio: es el que la representa, y verlo orbitando en el borde
 * junto a los otros dejaba al racimo sin núcleo.
 *
 * @returns {{troncos:Array, total:number}} `total` es cuántos segmentos hay en TODO el bosque —
 *   el número que decide el tamaño del buffer de instancias, y el que hay que mirar cuando algo
 *   se pone lento.
 */
export function bosque(racimos, opciones) {
  const o = opciones || {};
  const tope = Math.max(16, Number(o.topePorArbol) || 2000);
  const out = [];
  let total = 0;
  let semilla = 7;
  for (const rac of racimos || []) {
    const ts = rac.troncos || [];
    if (!ts.length) continue;
    const R = Math.max(1, Number(rac.radio) || 1);
    ts.forEach((t, i) => {
      const k = i + 0.5, phi = Math.acos(1 - 2 * k / ts.length), th = Math.PI * (1 + Math.sqrt(5)) * k;
      const propio = String(t.id || '').toLowerCase() === String(rac.persona || '').toLowerCase();
      const rr = propio ? R * 0.12 : R * 0.62;
      const centro = [
        rac.centro[0] + Math.cos(th) * Math.sin(phi) * rr,
        rac.centro[1] + Math.cos(phi) * rr,
        rac.centro[2] + Math.sin(th) * Math.sin(phi) * rr,
      ];
      const a = arbol({ id: t.id, notas: t.notas, centro, escala: Number(o.escala) || 1, tope, semilla: (semilla += 977) });
      total += a.segs.length;
      out.push({ id: t.id, persona: rac.persona, notas: t.notas, centro, color: rac.color, ...a });
    });
  }
  return { troncos: out, total };
}
