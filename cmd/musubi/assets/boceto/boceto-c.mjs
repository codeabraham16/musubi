// boceto-c.mjs — «EL CORTE». Todo el tejido en una sola lámina.
//
// LA APUESTA: sacrificar el volumen para que no haya NADA tapado. En el núcleo, mirar una rama
// implica que otras trece están detrás de ella; acá el tejido está aplastado contra un plano, así
// que de frente se ve todo a la vez y ninguna rama esconde a otra. Es la vista que contesta
// «¿cómo está repartida la memoria?» de un vistazo, sin girar.
//
// No es una convención de diagrama: es un CORTE HISTOLÓGICO. Así se mira tejido nervioso de
// verdad — se rebana, se tiñe y se mira plano, justamente porque en volumen no se ve nada.
//
// LO QUE SE PAGA, y hay que decirlo: aplastar mete a todas las ramas en el mismo plano, así que
// las que en el núcleo se esquivaban por profundidad ahora se cruzan de verdad. El aplanado NO se
// hace achatando la escena terminada —eso mandaría a dos hermanas a la misma dirección y volvería
// la maraña que costó quince veces menos enredo— sino plegando el acimut de cada bifurcación: cada
// hermana elige un lado del plano y la separación se conserva. Ver `aplanado` en `colocarNucleo`.

import { colocarNucleo } from './comun.mjs';
import { construir, HEBRA } from './forma.mjs';

await construir({
  id: 'c',
  nota: 'aplastado contra un plano: de frente no hay nada tapado',
  // MENOS NIVELES QUE EN EL NÚCLEO. En un plano el espacio crece con el cuadrado del radio y no
  // con el cubo, así que la misma profundidad amontona mucho más. Ocho niveles acá es una maraña;
  // seis reparte.
  seccionado: { maxNivel: 6, minCarga: 14 },
  colocar: (S) => colocarNucleo(S, {
    origen: [0, 0, 0], nucleo: 34, largo: 178, curvatura: 0.10, tropismo: 0, semilla: 11,
    reparto: 'plano', plano: true,
    aire: 3.4, naciente: 0.85, aperturaMax: 1.45, polarEje: 0.20, polarMin: 0.95,
    radioHilo: HEBRA.radioHilo, separacion: HEBRA.separacion,
  }),
  // DE FRENTE Y DESDE ARRIBA. Un corte se mira perpendicular: con la cámara oblicua el plano se
  // ve en escorzo y se pierde justamente lo que se vino a ganar.
  montaje: { camara: { az: 0, el: 1.35, min: 8, max: 3000 } },
});
