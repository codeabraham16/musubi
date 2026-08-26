// vista-memoria.mjs — EL COLONIZADO, VESTIDO PARA PRODUCCIÓN.
//
// La lente de memoria del panel deja de dibujarse con las mallas propias de dashboard.mjs y pasa
// a montarse con el MOTOR DEL BOCETO (boceto/escena.mjs + comun.mjs + datos.mjs): el árbol que
// CRECE hacia la memoria, el replay cronológico, carne+luz, el nudo en embudo — lo que el usuario
// aprobó mirándolo. Este módulo es sólo el ADAPTADOR: no re-implementa nada del motor, lo
// configura igual que boceto-g.mjs y le da un host, y expone lo que el chrome del panel necesita
// (racimos para la leyenda, un pulso por evento del riel, el brote del vivo).
//
// Por qué importar desde ../boceto/ y no copiar: el boceto es el laboratorio donde cada decisión
// tiene su medición y su sabotaje (54 invariantes al escribir esto). Una copia divergiría en
// silencio — y el puente inverso ya existía (boceto/datos.mjs importa de ../src/ desde el día
// uno). esbuild resuelve las rutas relativas sin plumbing.

import { formarColonizado, escalaTinta, tonoDe, broteDesdeMemorias,
         PALETA_CYBER, COLOR_MUSUBI_CYBER } from '../boceto/comun.mjs';
import { armarRaiz } from '../boceto/datos.mjs';
import { montar } from '../boceto/escena.mjs';

// La MISMA configuración que boceto-g.mjs, calibrada en F0 contra los dos cerebros. Si se ajusta
// una perilla, se ajusta ALLÁ y esto la hereda — dos configuraciones es como el boceto aprueba
// una cosa y producción muestra otra.
export const OPCIONES_COLONIZADO = {
  radio: 285, piso: 0.45, cola: 0.65, margen: 0.9,
  paso: 16, di: 80, dk: 18, inercia: 0.6,
  largoMax: 72, tolerancia: 2.5,
  nucleo: 40, radioHilo: 0.52, separacion: 2.60,
};
const HILOS = { porMemoria: 6, maxHoja: 22 };
const HEBRA = { radioHilo: 0.52, separacion: 2.60, largoNeurona: 17, torsion: 0.6 };

// El CSS mínimo que la vista necesita (la ficha de navegación, el tooltip 3D, la caja de prueba).
// La fuente de verdad visual es boceto/estilo.css — esto es el subconjunto que viaja en el bundle,
// con la ficha corrida para no pisar la barra inferior del panel.
const CSS_VISTA = `
.vm-host{position:fixed;inset:0;z-index:0}
.vm-host canvas{display:block}
.hud{position:fixed;left:20px;bottom:64px;max-width:min(520px,46vw);z-index:10;
  background:rgba(10,14,24,.92);border:1px solid rgba(120,140,180,.18);border-radius:10px;
  font:12.5px/1.45 'Segoe UI',system-ui,sans-serif;color:#c8d2e4;overflow:hidden}
.hud .migas{display:flex;flex-wrap:wrap;gap:4px;padding:9px 12px 7px;
  border-bottom:1px solid rgba(120,140,180,.12)}
.hud .miga{cursor:pointer;color:#8fa3c4;padding:1px 8px;border:1px solid rgba(120,140,180,.22);
  border-radius:999px;font-size:11px;background:none}
.hud .miga:hover{color:#e8eefb;border-color:rgba(120,140,180,.5)}
.hud .miga.hoy{color:#cfe0ff;border-color:rgba(140,170,255,.55);background:rgba(120,150,220,.12)}
.hud .ficha{padding:10px 12px 4px;display:flex;flex-direction:column;gap:3px}
.hud .ficha .fila{white-space:nowrap;color:#b9c4da}
.hud .ficha .fila.elegido{color:#eef3fc}
.hud .ficha .dim{color:#7d8aa5;font-size:11.5px}
.hud .ficha b{color:#eef3fc}
.hud .acciones{display:flex;gap:8px;padding:8px 12px 10px}
.hud .bt{flex:1;cursor:pointer;background:rgba(120,150,220,.08);border:1px solid rgba(120,140,180,.3);
  color:#c8d2e4;border-radius:8px;padding:5px 8px;font-size:11.5px}
.hud .bt:hover{border-color:rgba(140,170,255,.6);color:#fff}
.hud .bt.on{background:rgba(120,150,220,.22);border-color:rgba(140,170,255,.7)}
.hud .teclas{padding:7px 12px 9px;border-top:1px solid rgba(120,140,180,.12);color:#66738d;font-size:10.5px}
.vm-host .tip{position:fixed;z-index:11;pointer-events:none;display:none;max-width:320px;
  background:rgba(8,12,20,.95);border:1px solid rgba(120,140,180,.28);border-radius:8px;
  padding:7px 10px;font:12px/1.4 'Segoe UI',system-ui,sans-serif;color:#dfe7f5}
.vm-host .tip b{color:#fff}
.vm-host .tip .d{color:#8fa3c4}
.prueba{position:fixed;right:16px;top:64px;z-index:12;background:rgba(8,12,20,.96);
  border:1px solid rgba(120,140,180,.3);border-radius:10px;padding:10px 14px;max-width:560px;
  font:11.5px/1.5 Consolas,monospace;color:#c8d2e4;max-height:82vh;overflow:auto}
.prueba .t{font-weight:700;letter-spacing:.06em;margin-bottom:6px;color:#eef3fc}
.prueba .l{display:flex;justify-content:space-between;gap:18px}
.prueba .ok::before{content:'OK ';color:#4ade80}
.prueba .mal{color:#fb7185}
.prueba .mal::before{content:'X '}
.prueba .dato{color:#7d8aa5}
`;

/**
 * crearVistaMemoria: monta el colonizado sobre el grafo que el panel ya bajó de /api/graph.
 *
 * @param {{neurons:Array, synapses:Array}} grafo  tal cual lo devuelve el endpoint
 * @param {object} o  { host?: Element }
 * @returns {{vista, racimos, S, pulsoHacia, brotarMemorias, mostrar, get visible}}
 */
export function crearVistaMemoria(grafo, o) {
  const opciones = o || {};
  if (!document.getElementById('vm-css')) {
    const st = document.createElement('style');
    st.id = 'vm-css';
    st.textContent = CSS_VISTA;
    document.head.appendChild(st);
  }
  const host = document.createElement('div');
  host.className = 'vm-host';
  // Va como PRIMER hijo del body: mismo plano que el canvas #brain (z-index 0), debajo de todo
  // el chrome del panel, que vive en capas superiores.
  (opciones.host || document.body).prepend(host);

  const tono = tonoDe(location.search);
  const { raiz, colorDe, racimos } = armarRaiz(grafo.neurons, tono === 'cyber'
    ? { titulo: 'memoria', paleta: PALETA_CYBER, colorMusubi: COLOR_MUSUBI_CYBER }
    : { titulo: 'memoria' });
  const S = formarColonizado(raiz, OPCIONES_COLONIZADO);

  // LA TINTA ES PRESUPUESTO — el paso que en el boceto hace forma.mjs y acá hace el adaptador:
  // la alfa declarada es para el cerebro de referencia y se reparte entre las relaciones reales.
  const tinta = escalaTinta((grafo.synapses || []).length);

  const vista = montar({
    host,
    secciones: S, colorDe, titulo: 'memoria', sinapsis: grafo.synapses,
    ...HILOS, ...HEBRA,
    fondo: tono === 'cyber' ? '#04060D' : '#0C1020',
    bloom: tono === 'cyber' ? 1.15 : 0.80,
    umbralBloom: tono === 'cyber' ? 0.60 : 0.74,
    neon: tono === 'cyber' ? 1 : 0,
    agrupar: 0.66,
    alfaSinapsis: 0.13 * tinta, alfaConfianza: 0.34 * tinta,
    nivelesPenacho: 3, escalaPenacho: 0.62,
    fichaCompacta: true,
  });

  // racimo → sección raíz de ese actor, para que un evento del riel se vuelva un pulso que viaja
  // por el árbol REAL hasta su dueño.
  const RAIZ_DE = new Map();
  for (const i of S[0].hijos) {
    if (S[i].racimo != null && !RAIZ_DE.has(S[i].racimo)) RAIZ_DE.set(S[i].racimo, i);
  }

  return {
    vista, racimos, S,
    get estado() { return S.estado; },
    /** un evento real del riel enciende el camino hasta el actor que lo produjo */
    pulsoHacia(racimo) {
      const i = RAIZ_DE.get(racimo);
      vista.lanzarPulso(i != null ? i : 0);
    },
    /** el vivo: memorias nuevas del delta brotan en el árbol — la misma lógica del boceto */
    brotarMemorias(nuevas) {
      let tot = { eslabones: 0, botones: 0, sinLugar: 0 };
      for (const { brote } of broteDesdeMemorias(S.estado, nuevas)) {
        const r = vista.brotar(brote);
        tot.eslabones += r.eslabones; tot.botones += r.botones; tot.sinLugar += r.sinLugar;
      }
      return tot;
    },
    mostrar(v) {
      // dormir de verdad: el frame del motor se salta todo cuando el canvas está hidden
      vista.renderer.domElement.hidden = !v;
      host.style.display = v ? '' : 'none';
    },
    get visible() { return !vista.renderer.domElement.hidden; },
  };
}
