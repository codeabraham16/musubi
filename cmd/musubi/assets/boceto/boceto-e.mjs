// boceto-e.mjs — «LA CORTEZA». Las hojas afuera, los tractos adentro.
//
// LA APUESTA: que la forma diga dónde está el CONOCIMIENTO y dónde el CAMINO hacia él. El
// crecimiento se tira hacia una cáscara: mientras la rama está adentro, el campo la empuja hacia
// afuera; cuando llega a la superficie deja de empujar y la rama sigue por la tangente, así que el
// tejido se extiende SOBRE la esfera en vez de atravesarla. Resultado: todas las memorias terminan
// en una superficie y todo lo que queda adentro es tracto.
//
// Que es exactamente cómo está armado un cerebro: corteza afuera —los cuerpos celulares, lo que
// sabe— y sustancia blanca adentro —los axones, lo que comunica—. Acá no es una analogía: las
// puntas son memorias y el interior son los hilos que las conectan con el núcleo.
//
// LO QUE SE PAGA, y es el precio más alto de las cinco: una superficie tiene menos lugar que un
// volumen, así que las bifurcaciones se aprietan. Medido sobre el cerebro local, 108 de las 220 no
// consiguen el aire que piden — la mitad. Van declaradas en la leyenda, como siempre; pero es el
// número que hay que mirar antes de elegir esta forma. Y la esfera esconde su propia mitad de
// atrás, así que hay que girar sí o sí.

import { colocarNucleo } from './comun.mjs';
import { construir, HEBRA } from './forma.mjs';

await construir({
  id: 'e',
  nota: 'el campo empuja hasta la cáscara y ahí suelta: las memorias quedan en la superficie',
  seccionado: { maxNivel: 8, minCarga: 10 },
  colocar: (S) => colocarNucleo(S, {
    origen: [0, 0, 0], nucleo: 38, largo: 120, curvatura: 0.11, tropismo: 0, semilla: 11,
    // EL RADIO DE LA CÁSCARA contra el LARGO de las ramas: si la cáscara queda más cerca de lo que
    // mide una rama de nivel 1, los actores la atraviesan antes de ramificar y no hay interior.
    campo: 1, cascara: 235,
    aire: 3.0, naciente: 0.85, aperturaMax: 1.30, polarEje: 0.20, polarMin: 0.85,
    radioHilo: HEBRA.radioHilo, separacion: HEBRA.separacion,
  }),
  montaje: { camara: { az: 0.55, el: 0.20, min: 8, max: 3000 } },
});
