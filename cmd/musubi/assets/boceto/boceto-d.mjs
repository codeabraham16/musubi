// boceto-d.mjs — «LA CORONA». El medio vacío, y las relaciones lo cruzan.
//
// LA APUESTA: hacer protagonistas a las CONEXIONES en vez de a la ramificación. En las otras
// cuatro formas la relación viaja por el mismo tejido que todo lo demás y se lee como una capa
// más. Acá las hojas se reparten PAREJO sobre un anillo y el medio queda VACÍO: cualquier relación
// entre dos hojas lejanas sube hasta su ancestro común —que está cerca del centro— y lo cruza. Lo
// único que hay ahí adentro son relaciones, así que se ve de un vistazo quién habla con quién.
//
// La diferencia con «el corte», que también es plano: allá una hoja cae donde la mandó su rama, y
// los actores grandes ocupan más borde. Acá cada memoria terminal tiene el mismo pedazo de
// circunferencia, y el orden de recorrido mantiene juntas a las hermanas — el anillo sigue
// contando la jerarquía aunque esté aplanado.
//
// Es la lógica del diagrama de conectoma circular —la forma estándar para mostrar conectividad
// entre regiones— pero con fibras de verdad en vez de cintas: el arco que cruza el medio es la
// misma relación que en las otras formas, ruteada por el mismo árbol.
//
// LO QUE SE PAGA: la profundidad deja de ser distancia recorrida y pasa a ser posición radial, así
// que dos ramas de largos muy distintos se dibujan iguales. Es el mismo precio que las láminas, y
// es el intercambio contrario al del núcleo — que conserva las distancias y paga con oclusión.

import { colocarCorona } from './comun.mjs';
import { construir } from './forma.mjs';

await construir({
  id: 'd',
  nota: 'las hojas parejas en un anillo; lo que cruza el medio son relaciones',
  // MÁS PROFUNDIDAD QUE EN LAS OTRAS, y acá no cuesta: el anillo reparte las hojas parejo, así que
  // el doble de hojas es el doble de resolución del borde y no el doble de amontonamiento.
  seccionado: { maxNivel: 8, minCarga: 8 },
  colocar: (S) => colocarCorona(S, { radio: 268, hueco: 62, curvatura: 0.10, semilla: 29 }),
  montaje: {
    camara: { az: 0.15, el: 1.32, min: 8, max: 3000 },
    // LAS RELACIONES SE VEN MÁS ACÁ, y no es una perilla suelta: es de lo que trata esta forma. Con
    // el mismo alfa que en el núcleo, lo único que hay en el medio del cuadro sería casi invisible.
    // Y `agrupar` más bajo las separa del árbol: pegadas al andamio no se distinguirían de él.
    agrupar: 0.62, alfaSinapsis: 0.15, alfaConfianza: 0.38,
  },
});
