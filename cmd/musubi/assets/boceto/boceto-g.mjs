// boceto-g.mjs — «EL COLONIZADO». La rama CRECE hacia la memoria (space colonization).
//
// Séptima forma y otro paradigma: las seis anteriores COLOCAN el árbol semántico de arriba hacia
// abajo; acá cada HILO de memoria es un ATRACTOR en su parcela del treemap y el tejido crece de
// abajo hacia arriba, bifurcándose donde el dato lo bifurca. Las ramas compiten por los
// atractores, así que se esquivan solas — la anti-colisión que las otras formas parchean a mano
// acá es una propiedad del proceso. Ver el bloque EL COLONIZADO en comun.mjs.
//
// LO QUE SE PAGA, medido en F0 y a juzgar con los ojos: el enredo sube (0,95 contra 0,32 del nudo
// en el central) porque la topología emergente no respeta la adyacencia temática EN VUELO — las
// ramas se tejen entre sí como tejido real. Si el ojo lo lee como maraña y no como tejido, el
// siguiente paso es la colonización jerárquica (crecer por nivel semántico), no otra perilla:
// el barrido de inercia × piso dio filas idénticas, o sea esas perillas no llegan.

import { formarColonizado } from './comun.mjs';
import { construir, HEBRA } from './forma.mjs';

await construir({
  id: 'g',
  nota: 'cada hilo es un atractor; el tejido crece hacia donde hay memoria y se ve crecer',
  // Calibrado en F0 contra los DOS cerebros (2026-08-26): di=80 cubre el espaciado típico entre
  // atractores — con 46 el árbol los levantaba EN SERIE y forzaba el 58 %; dk=18 da la densidad
  // buscada (mediana 6 memorias por hoja, la misma constante porMemoria de todas las formas);
  // piso=0.45 deja cola interior que guía el tramo ciego y rompe la silueta esférica.
  formar: (raiz) => formarColonizado(raiz, {
    radio: 285, piso: 0.45, cola: 0.65, margen: 0.9,
    paso: 16, di: 80, dk: 18, inercia: 0.6,
    largoMax: 72, tolerancia: 2.5,
    nucleo: 40, radioHilo: HEBRA.radioHilo, separacion: HEBRA.separacion,
  }),
  montaje: {
    camara: { az: 0.55, el: 0.28, min: 8, max: 3000 },
    agrupar: 0.66, alfaSinapsis: 0.13, alfaConfianza: 0.34,
  },
});
