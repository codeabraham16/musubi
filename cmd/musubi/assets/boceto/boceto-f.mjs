// boceto-f.mjs — «EL NUDO». La fusión: el trazo del núcleo, el borde de la corona.
//
// Es lo que se pidió juntar, y lo que se puede juntar de verdad:
//
//   el núcleo   isotropía —no hay arriba—, volumen y un trazo ORGÁNICO: la rama se abre donde el
//               dato la abre. Paga oclusión, y un borde disparejo.
//   la corona   borde PAREJO y hueco central, así que lo único que cruza el medio son las
//               relaciones y pasan a ser el dibujo. Paga ser plana y perder lo orgánico.
//
// EL DENDROGRAMA ESFÉRICO NO ERA LA FUSIÓN. Lo construí entero antes de aceptarlo: sale un
// estallido de cuerdas rectas desde el centro, porque una hoja poco profunda salta de su radio a
// la cáscara de una sola tirada. Gana el borde parejo y pierde justo lo que hacía bueno al núcleo.
//
// La fusión que SÍ lo es: el treemap esférico decide DÓNDE va cada hoja y el crecimiento del
// núcleo decide CÓMO llega. Cada rama se abre como en el núcleo —con sus cuñas, su aire entre
// hermanas y su curvatura— y un imán se la va llevando hacia su parcela, cada vez más fuerte a
// medida que baja. Ver `repartirEsfera` y `colocarNudo`.
//
// Y se llama el nudo porque eso es lo que se ve: lo que ata la esfera pasa por adentro. En Musubi
// el nudo (結び) no es un adorno que se dibuja — aparece cuando hay vínculo, y acá lo hay.
//
// LO QUE SE PAGA: una esfera esconde su propia mitad de atrás, así que girar deja de ser opcional.

import { colocarNudo } from './comun.mjs';
import { construir, HEBRA } from './forma.mjs';

await construir({
  id: 'f',
  nota: 'el trazo del núcleo, el borde parejo de la corona; lo que la ata cruza por el medio',
  seccionado: { maxNivel: 8, minCarga: 10 },
  colocar: (S) => colocarNudo(S, {
    origen: [0, 0, 0], nucleo: 40, largo: 130, curvatura: 0.12, tropismo: 0, semilla: 11,
    // EL IMÁN VA ALTO Y SIN RAMPA, que es lo contrario de lo que yo suponía: con 0,80 y rampa la
    // hermana nace apuntando casi para el mismo lado que su vecina. Ver `rampa` en `colocarNucleo`.
    // EL DESTINO ORIENTA, PERO NO PEGA. Con la hoja clavada al radio la silueta es una esfera
    // perfecta —y eso ya era «la corteza»—. Lo que aporta el treemap es el reparto PAREJO EN
    // ÁNGULO; la distancia la sigue poniendo el crecimiento, así que la silueta queda irregular.
    radio: 285, pegar: false, 'imán': 0.45,
    // Y LAS DOS SE SUBEN JUNTAS: `polarMin` sola no llega, la frena `aperturaMax`. El barrido lo
    // destapó por el delator de siempre — 1,30 · 1,45 · 1,60 devolvían FILAS IDÉNTICAS y sólo
    // subía el contador de apretadas, o sea la perilla no llegaba y nada más prometía.
    aire: 3.0, naciente: 0.85, aperturaMax: 1.60, polarEje: 0.20, polarMin: 1.60,
    radioHilo: HEBRA.radioHilo, separacion: HEBRA.separacion,
  }),
  montaje: {
    camara: { az: 0.55, el: 0.28, min: 8, max: 3000 },
    // Las relaciones son la mitad de esta forma, como en la corona: se separan del andamio para
    // que se vean. Pegadas al árbol no se distinguirían de él.
    agrupar: 0.66, alfaSinapsis: 0.13, alfaConfianza: 0.34,
  },
});
