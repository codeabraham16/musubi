// latido.mjs — el LATIDO del pie del riel en vivo: si el sondeo sigue llegando y hace cuánto llegó
// el último. Sin DOM y con la hora como parámetro, para que `node --test` y la prueba de Go
// (cmd/musubi/panel_latido_node_test.go) lo recorran segundo a segundo sin dormir.
//
// POR QUÉ DEJÓ DE SER «SONDEOS DEL ÚLTIMO MINUTO». El latido contaba los sondeos de los últimos 60 s
// y con cero decía «sin sondeo». Con la bajada espaciada (internal/mcp/bajada_ritmo.go) una máquina
// quieta pide cada 300 s, y hasta ~450 s cuando el candado cambia de dueño (el tope, más un tick de
// 30 s, más el lease de 120 s si el dueño muere sin soltarlo). Con los 3 sondeadores que el central
// vio en 24 h (medido el 2026-09-27), eso es un sondeo cada ~100 s: simulado con fases al azar, la
// ventana de un minuto apagaba la lámpara el 51,6 % del tiempo con el sistema sano, y el 80 % con
// un solo sondeador.
//
// EL UMBRAL ES 10 MIN: le gana al peor hueco sano (450 s) con 150 s de margen, y ante un corte real
// la lámpara se apaga a los 10 min del último sondeo. La rapidez que se pierde la devuelve el TEXTO,
// que dice hace cuánto llegó el último: un corte se lee en cuanto esa antigüedad pasa de lo normal,
// antes de que la lámpara se apague. Y un enlace caído lo sigue diciendo en el acto el encabezado
// del riel: esto mide si los clientes sondean, no si hay enlace.

export const UMBRAL_LATIDO_MS = 10 * 60 * 1000;

/**
 * anotarSondeo: la marca del último sondeo después de ver uno.
 *
 * La hora es la del EVENTO (`at`, la que estampó el cerebro al terminar la llamada), no la de
 * llegada: el backlog que el relay manda al abrir la página trae sondeos de hasta horas atrás, y
 * estampados al llegar prendían la lámpara al abrir —un minuto con la ventana vieja, diez con ésta—
 * aunque nadie sondeara hacía rato. Un `at` ilegible o ausente cae a la llegada; uno del FUTURO (el
 * reloj del central adelantado respecto del navegador) se recorta a la llegada, para no dar una
 * antigüedad negativa. Nunca retrocede: un sondeo viejo que llega tarde no tapa a uno más nuevo.
 */
export function anotarSondeo(ultimo, at, llegada) {
  const t = Date.parse(at);
  const marca = Number.isFinite(t) ? Math.min(t, llegada) : llegada;
  return Math.max(Number(ultimo) || 0, marca);
}

/**
 * latido: lo que dice el pie del riel a la hora `ahora`, con `ultimo` la marca de anotarSondeo (0
 * si todavía no llegó ninguno). Prendido mientras el último sondeo tenga UMBRAL_LATIDO_MS o menos.
 * La antigüedad va en las mismas unidades gruesas que la de las filas (s → m → h): pasado el primer
 * minuto, el texto cambia a lo sumo una vez por minuto.
 */
export function latido(ultimo, ahora) {
  if (!(ultimo > 0)) return { vive: false, texto: 'sin sondeo' };
  const edad = Math.max(0, ahora - ultimo);
  const vive = edad <= UMBRAL_LATIDO_MS;
  const hace = antiguedad(edad);
  return { vive, texto: vive ? 'sondeo · hace ' + hace : 'sin sondeo desde hace ' + hace };
}

// antiguedad: la forma más corta que sigue siendo exacta, igual que hace() en dashboard.mjs.
function antiguedad(ms) {
  const s = Math.round(ms / 1000);
  if (s < 60) return s + 's';
  const m = Math.round(s / 60);
  if (m < 60) return m + 'm';
  return Math.round(m / 60) + 'h';
}
