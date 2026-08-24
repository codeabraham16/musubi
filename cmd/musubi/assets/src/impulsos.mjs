// impulsos.mjs — el IMPULSO que recorre un árbol dendrítico: cuándo nace, cuánto vive y cuánto
// enciende cada tramo. Sin motor de dibujo, para que `node --test` lo corra en CI.
//
// LA REGLA QUE MANDA, Y ES LA RAZÓN DE QUE ESTO SEA UN MÓDULO APARTE: un pulso = un evento real.
// Acá no hay bucle, ni temporizador, ni nada que fabrique luz. `nacer()` es la ÚNICA puerta, y la
// llama el riel cuando llega una invocación. Si el cerebro está quieto, esto devuelve cero y el
// árbol se apaga. Que eso siga siendo cierto es lo que custodian los tests.
//
// Lo que SÍ es licencia de lectura y se declara: DUR_PULSO y ANCHO_FRENTE no salen de ningún
// dato — son la velocidad a la que el ojo puede seguir el frente. Lo que sale del dato es cuándo
// nace (el `at` del evento), en qué neurona (`principal`), de qué capa (`kind`), si falló
// (`outcome`) y qué tan grueso es (`ms`).

export const DUR_PULSO = 0.85;          // segundos del soma a la última punta
export const ANCHO_FRENTE = 0.22;       // qué porción del árbol abarca el frente a la vez
// Techo por neurona: una ráfaga de sondeo no puede volverse un fogonazo blanco. Se descarta el
// más viejo, que ya casi terminó su recorrido. Sin esto, `davantis-admin` —52.498 llamadas en 7
// días, casi todas sondeo— dejaba su árbol encendido permanentemente y el resto invisible.
export const TOPE_POR_NEURONA = 6;
// ÁMBAR y no rojo: es el color que este sistema ya usa para «aviso» en todo el HUD. Un rojo nuevo
// sería un idioma nuevo para decir lo mismo.
//
// Pero el HUD pinta sobre un fondo opaco y el árbol EMITE sobre un medio aditivo, y ahí un ámbar
// pálido es indistinguible del blanco. Medido en pantalla: con el #f5c451 del HUD, la relación
// azul/rojo del frente pasaba de 0,478 (salió bien) a 0,445 (falló entero) — un 7 %, invisible
// a ojo con las dos capturas puestas al lado. Así que el frente usa el MISMO TONO (41°) llevado
// a saturación plena: es la misma señal, no otra, dicha en el idioma del medio que la muestra.
export const AMBAR = [0.961, 0.769, 0.318];       // #f5c451 — el del HUD, para las leyendas
export const AMBAR_FRENTE = [1.0, 0.60, 0.02];    // mismo tono, saturado — el del impulso

/**
 * grosorDe: de `ms` a un multiplicador de ancho y brillo, en escala LOG.
 * El rango medido en producción va de 0,15 ms (sync_pull) a 60.041 ms (distill). En lineal, todo
 * lo que no sea el peor caso queda en un pelo invisible y el dato no se ve.
 */
export function grosorDe(ms) {
  return 1 + Math.min(1.6, Math.log10(1 + Math.max(0, Number(ms) || 0)) * 0.42);
}

/**
 * crearImpulsos: el registro de pulsos vivos, indexado por tronco.
 *
 * El vencimiento se hace al escribir y no en un temporizador: el reloj de la escena se detiene
 * cuando el panel está en pausa, y un pulso no puede seguir viajando mientras el dibujo está
 * congelado. Con `setTimeout` seguiría corriendo y al volver aparecerían árboles ya apagados.
 */
export function crearImpulsos() {
  /** @type {Map<number, Array>} */
  const porTronco = new Map();
  let vistos = 0, sinTronco = 0;

  return {
    /**
     * nacer: UN evento real, UN impulso. Devuelve si encontró tronco.
     * Un `false` NO se traga: se cuenta, porque un principal sin neurona es un dueño sin declarar.
     */
    nacer(tronco, ev, ahora) {
      vistos++;
      // OJO: `Number(null)` es 0, no NaN. Coercionar acá mandaba todo evento sin neurona al
      // tronco 0 —el más grande, porque los racimos van ordenados por volumen— y el panel decía
      // que trabajó la terminal que más escribe cada vez que un principal no estaba declarado.
      const ti = tronco;
      if (!Number.isInteger(ti) || ti < 0) { sinTronco++; return false; }
      let lista = porTronco.get(ti);
      if (!lista) { lista = []; porTronco.set(ti, lista); }
      if (lista.length >= TOPE_POR_NEURONA) lista.shift();
      lista.push({
        t0: Number(ahora) || 0,
        trabajo: (ev && ev.capa) !== 'sondeo',
        falla: !!(ev && ev.falla),
        grosor: grosorDe(ev && ev.ms),
        // `exacta:false` = el evento cayó en la neurona por la persona, no por credencial propia.
        // Se ve más tenue en vez de mentir que fue una atribución directa.
        exacta: !(ev && ev.exacta === false),
      });
      return true;
    },

    cuenta() { return { vistos, sinTronco }; },

    /**
     * limpiar: tira los pulsos vivos, sin tocar los contadores.
     *
     * Se llama cuando se rehace el bosque, y NO es higiene: los pulsos guardan el ÍNDICE de su
     * tronco, y al reconstruirse el grafo ese índice puede pasar a nombrar a otro árbol. Un pulso
     * que sobreviva a la reconstrucción enciende, durante lo que le quede de vida, la neurona
     * equivocada — o sea el panel le atribuye la llamada a la persona que no fue, que es
     * exactamente lo que este dibujo existe para no hacer.
     *
     * Los contadores NO se reinician: son el total de lo que pasó, y ponerlos en cero al cambiar
     * de lente borraría la cuenta de eventos que no encontraron neurona.
     */
    limpiar() { porTronco.clear(); },

    /** vivos: cuántos pulsos siguen viajando. Cero ⇒ no hay nada que redibujar. */
    vivos(ahora) {
      let n = 0;
      for (const lista of porTronco.values()) {
        for (const p of lista) if (ahora - p.t0 <= DUR_PULSO) n++;
      }
      return n;
    },

    /**
     * frentes: dónde está el frente de cada pulso de UN tronco, ahora mismo.
     * Se precalcula por tronco a propósito: meterlo adentro del bucle de instancias lo repetiría
     * doce mil veces por cuadro para dar siempre lo mismo.
     */
    frentes(tronco, ahora, alcanceRama) {
      const lista = porTronco.get(Number(tronco));
      if (!lista || !lista.length) return [];
      // EL VENCIMIENTO VIVE ACA Y EN NINGUN OTRO LADO. Antes había además un `u > 1` más abajo, y
      // esa redundancia hacía que el test del vencimiento pasara igual con este `shift` sacado:
      // la segunda defensa tapaba al sabotaje y el invariante quedaba sin custodiar.
      // La lista está ordenada por `t0`, así que sacar de adelante alcanza.
      while (lista.length && ahora - lista[0].t0 > DUR_PULSO) lista.shift();
      const alc = Math.max(0.001, Number(alcanceRama) || 1);
      const out = [];
      for (const p of lista) {
        const u = (ahora - p.t0) / DUR_PULSO;
        if (u < 0) continue;   // el reloj de la escena se congela en pausa; nunca al revés
        out.push({
          en: u * alc,
          radio: ANCHO_FRENTE * alc * p.grosor,
          // se apaga hacia el final del recorrido: llega a la punta y se disipa, no desaparece
          fuerza: (p.trabajo ? 1 : 0.3) * (1 - u * u) * (p.exacta ? 1 : 0.65) * p.grosor,
          falla: p.falla,
        });
      }
      return out;
    },

    /**
     * escribir: el brillo de CADA instancia de dendrita, en un solo barrido.
     *
     * @param {number} ahora  reloj de la escena, en segundos
     * @param {{glow:Float32Array, warn:Float32Array, dist:Float32Array, tronco:Int32Array,
     *          alcances:Float32Array}} b
     * @returns {{encendidas:number, flash:Float32Array}} `flash` es el fogonazo por SOMA: el
     *   arranque del impulso, que es lo que hace que se lea «disparó ESTA neurona» y no «apareció
     *   una luz en el aire».
     */
    escribir(ahora, b) {
      const nTroncos = b.alcances.length;
      const flash = new Float32Array(nTroncos);
      // Un frente por tronco, calculado UNA vez. `null` en un tronco callado es la salida rápida
      // del bucle grande: con nueve actores y once troncos, lo normal es que casi todos lo estén.
      const cache = new Array(nTroncos);
      let hayAlguno = false;
      for (let ti = 0; ti < nTroncos; ti++) {
        const fs = this.frentes(ti, ahora, b.alcances[ti]);
        cache[ti] = fs.length ? fs : null;
        if (fs.length) {
          hayAlguno = true;
          for (const f of fs) {   // el soma fogonea mientras el frente todavía está encima
            if (f.en < f.radio) flash[ti] = Math.max(flash[ti], f.fuerza * (1 - f.en / f.radio));
          }
        }
      }
      let encendidas = 0;
      const n = b.glow.length;
      if (!hayAlguno) { b.glow.fill(0); b.warn.fill(0); return { encendidas: 0, flash }; }
      for (let i = 0; i < n; i++) {
        const fs = cache[b.tronco[i]];
        if (!fs) { b.glow[i] = 0; b.warn[i] = 0; continue; }
        const d = b.dist[i];
        let carga = 0, aviso = 0;
        for (let k = 0; k < fs.length; k++) {
          const f = fs[k], dd = d < f.en ? f.en - d : d - f.en;
          if (dd >= f.radio) continue;
          const c = f.fuerza * (1 - dd / f.radio);
          // SE SUMA, no se toma el máximo: donde dos frentes se cruzan el brillo se acumula,
          // igual que el blending aditivo del shader. Es lo que lo hace leer como luz.
          carga += c;
          // El aviso SE SUMA igual que la carga. Tomar el máximo y dividirlo por la suma parece
          // lo mismo con un pulso y no lo es con varios: con ocho fallas apiladas daba 1/8 y el
          // ámbar quedaba invisible justo cuando más fallas hay. Medido en pantalla: la relación
          // azul/rojo del frente no se movía ni un punto entre una ráfaga que salió bien y una
          // que falló entera.
          if (f.falla) aviso += c;
        }
        b.glow[i] = carga;
        // La proporción es honesta: todo falló ⇒ 1, la mitad ⇒ 0,5, nada ⇒ 0.
        b.warn[i] = carga > 0 ? Math.min(1, aviso / carga) : 0;
        if (carga > 0.004) encendidas++;
      }
      return { encendidas, flash };
    },
  };
}
