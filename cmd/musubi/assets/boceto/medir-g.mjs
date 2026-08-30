// _medir-g.mjs — F0 del colonizado: medir ANTES de construir encima.
// Uso: node _medir-g.mjs <grafo.json> [clave=valor ...]
import { readFileSync } from 'node:fs';
import { formarColonizado, contarFibras, enhebrar, enCurva, medirEnredo, radioHaz } from './comun.mjs';
import { armarRaiz } from './datos.mjs';

const arg = {};
for (const a of process.argv.slice(3)) { const [k, v] = a.split('='); arg[k] = Number(v); }
const n = (k, d) => (Number.isFinite(arg[k]) ? arg[k] : d);

const datos = JSON.parse(readFileSync(process.argv[2], 'utf8'));
const { raiz } = armarRaiz(datos.neurons, { titulo: 'memoria' });

const t0 = performance.now();
const S = formarColonizado(raiz, {
  radio: n('radio', 285), piso: n('piso', 0.45), cola: n('cola', 0.65), margen: n('margen', 0.9),
  paso: n('paso', 16), di: n('di', 80), dk: n('dk', 18), inercia: n('inercia', 0.4),
  largoMax: n('largoMax', 72), tolerancia: n('tol', 2.5),
  nucleo: 40, radioHilo: 0.52, separacion: 2.60,
});
const msCrecer = performance.now() - t0;

const t1 = performance.now();
const E = enhebrar(S, { radioHilo: 0.52, separacion: 2.60, largoNeurona: 17, torsion: 0.6 });
const msEnhebrar = performance.now() - t1;

const hip = (v) => Math.hypot(v[0], v[1], v[2]);
const hojas = S.filter((s) => s.hoja);

// 1 · densidad: memorias por hoja (histograma) y total de hilos
const cargas = hojas.map((h) => h.memorias.length).sort((a, b) => a - b);
const pct = (v, p) => v[Math.min(v.length - 1, Math.floor(v.length * p))];
const memTot = S.reduce((t, s) => t + s.memorias.length, 0);

// 2 · el erizo: radios de hoja + ángulo entre dir de la sección y la radial
const radios = hojas.map((h) => hip(h.b)).sort((a, b) => a - b);
let angRad = 0;
for (const h of hojas) {
  const r = hip(h.b) || 1;
  const rad = [h.b[0] / r, h.b[1] / r, h.b[2] / r];
  angRad += Math.acos(Math.max(-1, Math.min(1,
    h.dir[0] * rad[0] + h.dir[1] * rad[1] + h.dir[2] * rad[2])));
}
angRad /= Math.max(1, hojas.length);

// 3 · la curva no miente: desviación real contra el modelo, muestreada
//     (emitir ya corta por tolerancia; esto VERIFICA que el corte alcanzó)
// nota: la polilínea no viaja en la sección — se re-verifica contra la propia cuadrática con
// puntos intermedios sintéticos no disponibles acá; el chequeo fino vive en el test C6.

// 4 · solape hermana-hermana en el primer 20 % (el defecto del nudo, medido acá también)
const rad = (s) => radioHaz(Math.max(1, s.fibras || 1), 0.52, 2.60);
let peorSolape = 0, paresMal = 0;
const top = S[0].hijos.map((i) => S[i]);
for (let i = 0; i < top.length; i++) {
  for (let j = i + 1; j < top.length; j++) {
    const A = top[i], B = top[j], pide = rad(A) + rad(B);
    let mn = 1e9;
    for (let u = 0; u <= 24; u++) {
      const pa = enCurva(A, u / 24);
      for (let v = 0; v <= 24; v++) {
        const pb = enCurva(B, v / 24);
        mn = Math.min(mn, Math.hypot(pa[0] - pb[0], pa[1] - pb[1], pa[2] - pb[2]));
      }
    }
    const sol = Math.max(0, 100 * (1 - mn / pide));
    if (sol > 0) paresMal++;
    if (sol > peorSolape) peorSolape = sol;
  }
}

// 5 · enredo comparable con el nudo
const Enr = medirEnredo(S, { muestras: 6 });

// 6 · CDF de la edad (¿historia rachuda para el replay?)
const edades = [];
for (const s of S) for (const m of s.memorias) edades.push(Number(m.age_days) || 0);
edades.sort((a, b) => a - b);

console.log(JSON.stringify({
  cfg: { di: n('di', 80), dk: n('dk', 18), paso: n('paso', 16), inercia: n('inercia', 0.4),
         piso: n('piso', 0.45), cola: n('cola', 0.65) },
  ms: { crecer: +msCrecer.toFixed(0), enhebrar: +msEnhebrar.toFixed(0) },
  secciones: S.length, hojas: hojas.length,
  hilosNucleo: S[0].fibras, eslabones: E.length,
  memorias: memTot, forzados: S.forzados, recortado: S[0].recortado || 0,
  cargaHoja: { p10: pct(cargas, 0.1), p50: pct(cargas, 0.5), p90: pct(cargas, 0.9), max: cargas[cargas.length - 1] },
  radioHoja: { p10: +pct(radios, 0.1).toFixed(0), p50: +pct(radios, 0.5).toFixed(0), p90: +pct(radios, 0.9).toFixed(0) },
  anguloRadial: +angRad.toFixed(3),
  solapeTop: { peor: +peorSolape.toFixed(1), pares: paresMal },
  enredo: +Enr.enredo.toFixed(3),
  edad: { p10: +pct(edades, 0.1).toFixed(0), p50: +pct(edades, 0.5).toFixed(0), p90: +pct(edades, 0.9).toFixed(0), max: +(edades[edades.length - 1] || 0).toFixed(0) },
}));
