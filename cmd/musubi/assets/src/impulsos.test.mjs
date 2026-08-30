// Invariantes del impulso (impulsos.mjs). Corre en CI.
//
// El que manda es I1: SIN EVENTO NO HAY LUZ. Es la regla que este rediseño vino a imponer —antes
// un bucle recorría los axones sin que hubiera pasado nada— y es exactamente el tipo de cosa que
// vuelve sola: alcanza con que alguien agregue un Math.sin(t) "para que se vea vivo" y el panel
// pasa a mentir sin que nada falle.

import test from 'node:test';
import assert from 'node:assert/strict';
import { crearImpulsos, grosorDe, DUR_PULSO, TOPE_POR_NEURONA } from './impulsos.mjs';

// Un árbol de juguete: 3 troncos, 4 tramos cada uno, a distancias 1/2/3/4 del soma.
function bancada(nTroncos = 3, porTronco = 4) {
  const n = nTroncos * porTronco;
  const b = {
    glow: new Float32Array(n), warn: new Float32Array(n),
    dist: new Float32Array(n), tronco: new Int32Array(n),
    alcances: new Float32Array(nTroncos).fill(4),
  };
  for (let ti = 0, i = 0; ti < nTroncos; ti++)
    for (let k = 0; k < porTronco; k++, i++) { b.tronco[i] = ti; b.dist[i] = k + 1; }
  return b;
}
const EV = (extra = {}) => ({ capa: 'trabajo', falla: false, ms: 40, exacta: true, ...extra });
const suma = (a) => a.reduce((x, v) => x + v, 0);

test('I1 · SIN EVENTO NO HAY LUZ: el buffer queda en cero', () => {
  const imp = crearImpulsos(), b = bancada();
  for (let t = 0; t < 5; t += 0.13) {
    const r = imp.escribir(t, b);
    assert.equal(r.encendidas, 0, 'se encendió algo en t=' + t.toFixed(2) + ' sin que pasara nada');
    for (let i = 0; i < b.glow.length; i++) assert.equal(b.glow[i], 0);
    for (const f of r.flash) assert.equal(f, 0);
  }
  assert.equal(imp.vivos(0), 0);
});

test('I2 · el pulso VENCE, y el árbol vuelve a apagarse solo', () => {
  const imp = crearImpulsos(), b = bancada();
  imp.nacer(0, EV(), 0);
  assert.ok(imp.escribir(0.05, b).encendidas > 0, 'el pulso recién nacido tiene que encender algo');
  const r = imp.escribir(DUR_PULSO + 0.01, b);
  assert.equal(r.encendidas, 0, 'pasado DUR_PULSO no puede quedar nada encendido');
  assert.equal(imp.vivos(DUR_PULSO + 0.01), 0);
  for (let i = 0; i < b.glow.length; i++) assert.equal(b.glow[i], 0, 'la instancia ' + i + ' quedó prendida');
});

test('I3 · el frente VIAJA del soma a la punta, en ese orden', () => {
  // Es lo que separa un impulso de un destello: si el brillo no se corriera con el tiempo, el
  // árbol entero parpadearía a la vez y sería una lámpara, no una neurona.
  const imp = crearImpulsos(), b = bancada(1, 8);
  b.alcances[0] = 8;
  for (let i = 0; i < 8; i++) b.dist[i] = i + 1;
  imp.nacer(0, EV({ ms: 0 }), 0);
  const centro = (t) => {
    imp.escribir(t, b);
    let acum = 0, peso = 0;
    for (let i = 0; i < 8; i++) { acum += b.dist[i] * b.glow[i]; peso += b.glow[i]; }
    return peso > 0 ? acum / peso : null;
  };
  const temprano = centro(0.10), medio = centro(0.42), tarde = centro(0.74);
  assert.ok(temprano !== null && medio !== null && tarde !== null, 'el frente se perdió en el camino');
  assert.ok(temprano < medio, 'el frente tiene que avanzar: ' + temprano.toFixed(2) + ' a ' + medio.toFixed(2));
  assert.ok(medio < tarde, 'y seguir avanzando: ' + medio.toFixed(2) + ' a ' + tarde.toFixed(2));
});

test('I4 · TECHO por neurona: una ráfaga de sondeo no es un fogonazo blanco', () => {
  // davantis-admin mete 52.498 llamadas en 7 días, casi todas sondeo. Sin techo, su árbol queda
  // saturado permanentemente y los demás desaparecen debajo del bloom.
  const imp = crearImpulsos();
  for (let k = 0; k < 40; k++) imp.nacer(0, EV({ capa: 'sondeo' }), 0.001 * k);
  assert.equal(imp.vivos(0.04), TOPE_POR_NEURONA, 'el techo no se respetó');
  assert.equal(imp.cuenta().vistos, 40, 'los descartados igual se CUENTAN: si no, el techo esconde volumen');
});

test('I5 · un tronco callado NO se enciende porque pulsó otro', () => {
  // El aislamiento es la mitad del valor del dibujo: lo que se ve es QUIÉN trabaja. Si el pulso
  // se derramara al vecino, el panel diría que trabajaron los dos.
  const imp = crearImpulsos(), b = bancada(3, 4);
  imp.nacer(1, EV(), 0);
  imp.escribir(0.3, b);
  let enUno = 0;
  for (let i = 0; i < b.glow.length; i++) {
    if (b.tronco[i] === 1) enUno += b.glow[i];
    else assert.equal(b.glow[i], 0, 'el tronco ' + b.tronco[i] + ' se encendió sin que nadie lo llamara');
  }
  assert.ok(enUno > 0, 'el tronco que sí pulsó tiene que encenderse');
});

test('I6 · el fogonazo del soma es el ARRANQUE, no un brillo permanente', () => {
  // Es lo que hace que se lea «disparó ESTA neurona» en vez de «apareció una luz en el aire».
  const imp = crearImpulsos(), b = bancada();
  imp.nacer(2, EV(), 0);
  const f0 = imp.escribir(0.01, b).flash;
  assert.ok(f0[2] > 0, 'al nacer, el soma tiene que fogonear');
  assert.equal(f0[0], 0); assert.equal(f0[1], 0);
  const f1 = imp.escribir(0.55, b).flash;
  assert.equal(f1[2], 0, 'con el frente lejos, el soma ya no puede seguir prendido');
});

test('I7 · una falla viaja como AVISO, y sólo donde el frente está', () => {
  const imp = crearImpulsos(), b = bancada(2, 4);
  imp.nacer(0, EV({ falla: true }), 0);
  imp.nacer(1, EV({ falla: false }), 0);
  imp.escribir(0.2, b);
  let avisoOk = 0;
  for (let i = 0; i < b.glow.length; i++) {
    if (b.glow[i] === 0) { assert.equal(b.warn[i], 0, 'un tramo apagado no puede avisar nada'); continue; }
    if (b.tronco[i] === 0) { assert.ok(b.warn[i] > 0.9, 'la falla tiene que teñir el tramo: ' + b.warn[i]); avisoOk++; }
    else assert.equal(b.warn[i], 0, 'una llamada que salió bien no puede pintarse de aviso');
  }
  assert.ok(avisoOk > 0, 'el pulso que falló no encendió nada, así que el test no probó nada');
});

test('I7b · una RÁFAGA de fallas avisa igual de fuerte que una sola', () => {
  // El caso que se escapó: con `aviso` tomado como MÁXIMO y `carga` como SUMA, ocho fallas
  // apiladas daban 1/8 de aviso — el ámbar desaparecía justo cuando más fallas hay. Se vio en
  // pantalla antes que en un test: la relación azul/rojo del frente no se movía ni un punto
  // entre una ráfaga que salió bien y una que falló entera.
  const pico = (n, falla) => {
    const imp = crearImpulsos(), b = bancada(1, 4);
    for (let k = 0; k < n; k++) imp.nacer(0, EV({ falla }), 0);
    imp.escribir(0.2, b);
    return Math.max(...b.warn);
  };
  assert.ok(pico(8, true) > 0.9, 'ocho fallas juntas tienen que avisar entero: ' + pico(8, true).toFixed(3));
  assert.equal(pico(8, false), 0, 'ocho llamadas que salieron bien no pueden avisar nada');
  // Y la proporción se respeta: si sólo la mitad falló, el aviso es a medias y no entero.
  const imp = crearImpulsos(), b = bancada(1, 4);
  for (let k = 0; k < 4; k++) imp.nacer(0, EV({ falla: k % 2 === 0 }), 0);
  imp.escribir(0.2, b);
  const m = Math.max(...b.warn);
  assert.ok(m > 0.3 && m < 0.7, 'mitad y mitad tiene que dar un aviso a medias: ' + m.toFixed(3));
});

test('I7c · un pulso NO sobrevive a la reconstruccion del bosque', () => {
  // Los pulsos guardan el ÍNDICE de su tronco. Al rehacerse el grafo ese índice puede pasar a
  // nombrar otro árbol, y el pulso que sobreviva enciende la neurona equivocada durante lo que le
  // quede de vida: el panel le atribuye la llamada a la persona que no fue. Es la falla que este
  // dibujo existe para no cometer, y no da error de ninguna clase.
  const imp = crearImpulsos(), b = bancada(3, 4);
  imp.nacer(0, EV(), 0);
  assert.ok(imp.escribir(0.1, b).encendidas > 0, 'el pulso tiene que estar vivo antes de limpiar');
  imp.limpiar();
  assert.equal(imp.vivos(0.1), 0, 'no puede quedar ningun pulso vivo');
  assert.equal(imp.escribir(0.1, b).encendidas, 0, 'y el arbol tiene que quedar apagado');
  // Los contadores NO se tocan: son el total de lo que paso, y ponerlos en cero al cambiar de
  // lente borraria la cuenta de eventos que no encontraron neurona.
  imp.nacer(-1, EV(), 0);
  assert.deepEqual(imp.cuenta(), { vistos: 2, sinTronco: 1 });
});

test('I12 · el CAMPO es la actividad viva, y sin evento no hay campo', () => {
  // El halo alrededor de una neurona sale de acá. La regla es la misma de siempre: si no pasó nada,
  // no hay campo. Un halo permanente —por calor, por ejemplo— convertiría el reposo en luz y el
  // panel dejaría de poder decir «acá no está pasando nada», que es la mitad de lo que dice.
  const imp = crearImpulsos(), b = bancada(3, 4);
  const quieto = imp.escribir(0.2, b).campo;
  for (const c of quieto) assert.equal(c, 0, 'hay campo sin que haya pasado nada');
  imp.nacer(1, EV(), 0);
  const uno = imp.escribir(0.2, b).campo;
  assert.ok(uno[1] > 0, 'la neurona que pulsó tiene que tener campo');
  assert.equal(uno[0], 0); assert.equal(uno[2], 0);
  // Y SUMA: dos llamadas a la vez dan más campo que una. Si tomara el máximo, una ráfaga y un
  // evento suelto se verían igual, que es justo lo que hay que poder distinguir.
  const imp2 = crearImpulsos();
  imp2.nacer(1, EV(), 0); imp2.nacer(1, EV(), 0); imp2.nacer(1, EV(), 0);
  assert.ok(imp2.escribir(0.2, b).campo[1] > uno[1] * 2.4, 'tres llamadas tienen que dar mucho más campo que una');
  // Y se apaga solo cuando el último pulso vence.
  assert.equal(imp.escribir(DUR_PULSO + 0.01, b).campo[1], 0, 'el campo tiene que apagarse con el pulso');
});

test('I13 · una llamada vale LO MISMO repartida en cinco neuronas que en una', () => {
  // Un evento no sabe en qué neurona de la persona cayó, así que enciende todo su racimo. Si cada
  // una recibiera la fuerza entera, una persona cuyo árbol quedó cortado en cinco neuronas
  // brillaría cinco veces más que una cortada en una — por la MISMA llamada. El dibujo estaría
  // diciendo «trabajó más» cuando lo único distinto es la forma de su árbol.
  const luz = (nNeuronas) => {
    const imp = crearImpulsos(), b = bancada(nNeuronas, 4);
    for (let ti = 0; ti < nNeuronas; ti++) imp.nacer(ti, EV({ reparto: 1 / nNeuronas }), 0);
    imp.escribir(0.2, b);
    return suma(b.glow);
  };
  const una = luz(1), cinco = luz(5);
  assert.ok(Math.abs(cinco - una) / una < 0.02,
    'la misma llamada tiene que dar la misma luz total: 1 neurona ' + una.toFixed(3) + ' vs 5 ' + cinco.toFixed(3));
  // Y el campo, igual: la suma sobre el racimo es la misma.
  const campoDe = (nNeuronas) => {
    const imp = crearImpulsos(), b = bancada(nNeuronas, 4);
    for (let ti = 0; ti < nNeuronas; ti++) imp.nacer(ti, EV({ reparto: 1 / nNeuronas }), 0);
    return suma(imp.escribir(0.2, b).campo);
  };
  assert.ok(Math.abs(campoDe(5) - campoDe(1)) / campoDe(1) < 0.02, 'el campo total también tiene que ser el mismo');
  // Sin `reparto` no se rompe nada: vale 1, que es el caso de una neurona sola.
  const imp = crearImpulsos(), b = bancada(1, 4);
  imp.nacer(0, EV(), 0); imp.escribir(0.2, b);
  assert.ok(Math.abs(suma(b.glow) - una) / una < 0.02, 'sin reparto tiene que comportarse como reparto 1');
});

test('I8 · un principal SIN tronco se cuenta, no se traga', () => {
  // Es la señal de «hay un dueño sin declarar». Tragársela deja el panel diciendo que todo está
  // atribuido cuando no lo está.
  const imp = crearImpulsos();
  assert.equal(imp.nacer(-1, EV(), 0), false);
  assert.equal(imp.nacer(null, EV(), 0), false);
  assert.deepEqual(imp.cuenta(), { vistos: 2, sinTronco: 2 });
  assert.equal(imp.vivos(0), 0, 'un evento sin tronco no puede dejar un pulso vivo en ningún lado');
});

test('I9 · el TRABAJO REAL se distingue del sondeo por brillo, no sólo por color', () => {
  // Criterio del plan: en la ventana medida, el 98,1 % es sondeo. Si las dos capas brillaran
  // igual, el 1,9 % que importa quedaría enterrado.
  const brillo = (capa) => {
    const imp = crearImpulsos(), b = bancada(1, 4);
    imp.nacer(0, EV({ capa, ms: 10 }), 0);
    imp.escribir(0.15, b);
    return suma(b.glow);
  };
  const t = brillo('trabajo'), s = brillo('sondeo');
  assert.ok(t > s * 2, 'el trabajo real tiene que destacarse: ' + t.toFixed(3) + ' vs sondeo ' + s.toFixed(3));
  assert.ok(s > 0, 'pero el sondeo tampoco puede ser invisible: es el latido del sistema');
});

test('I10 · `ms` entra en escala LOG, o el rango real no se ve', () => {
  // Medido en producción: de 0,15 ms (sync_pull) a 60.041 ms (distill). En lineal eso es 400.000x
  // y todo lo que no sea el peor caso queda en un pelo invisible.
  const a = grosorDe(0.15), b = grosorDe(60041);
  assert.ok(b > a, 'una llamada más lenta tiene que verse más');
  assert.ok(b / a < 3, 'la relación no puede ser lineal: salió ' + (b / a).toFixed(1) + 'x');
  assert.equal(grosorDe(undefined), grosorDe(0), 'sin ms se trata como cero, no como NaN');
  assert.ok(Number.isFinite(grosorDe(-5)));
});

test('I11 · dos frentes que se cruzan SUMAN el brillo', () => {
  // Es el equivalente del blending aditivo del shader, y es lo que hace que una ráfaga se lea
  // como luz acumulándose en vez de como una línea repintada.
  const b = bancada(1, 4);
  const uno = crearImpulsos(); uno.nacer(0, EV(), 0); uno.escribir(0.2, b);
  const solo = suma(b.glow);
  const dos = crearImpulsos(); dos.nacer(0, EV(), 0); dos.nacer(0, EV(), 0); dos.escribir(0.2, b);
  const juntos = suma(b.glow);
  assert.ok(juntos > solo * 1.9, 'dos frentes encima tienen que sumar: ' + solo.toFixed(3) + ' a ' + juntos.toFixed(3));
});
