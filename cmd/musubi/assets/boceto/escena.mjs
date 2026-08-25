// escena.mjs — LA ANATOMÍA DIBUJADA Y LA NAVEGACIÓN. Compartido por los dos bocetos.
//
// Lo que cambia entre A y B es DÓNDE van las secciones (la colocación) y la atmósfera. Lo que es
// igual es qué ES una sección: una neurona completa, con su cuerpo, su vaina y sus nodos.
//
// LA PIEZA DE CADA SECCIÓN, de proximal a distal:
//
//     ◍━━━╋━━━╋━━━╋━━━▷    ·  ◍━━━╋━━━▷
//     soma  nodos de Ranvier   ↑
//           en la vaina        la hendidura: la siguiente neurona NO la toca
//
//   · SOMA — el cuerpo celular, al principio de la sección. Su tamaño sale de la ley de Rall
//     sobre lo que carga la rama, así que es dato.
//   · VAINA DE MIELINA — el tubo, con estrías: cada estría es un internodo.
//   · NODOS DE RANVIER — los estrangulamientos entre internodos. En un axón real la señal SALTA
//     de nodo a nodo (conducción saltatoria), así que son la unidad por la que viaja el impulso y
//     no una textura.
//   · TERMINAL / BOTONES — donde la sección hace sinapsis. En las secciones hoja, cada botón es
//     UNA MEMORIA.
//   · HENDIDURA SINÁPTICA — el hueco antes de la siguiente sección. Es lo que hace que la cadena
//     se lea como una cadena de neuronas y no como un tubo largo.

import * as THREE from 'three';
import { EffectComposer } from 'three/examples/jsm/postprocessing/EffectComposer.js';
import { RenderPass } from 'three/examples/jsm/postprocessing/RenderPass.js';
import { UnrealBloomPass } from 'three/examples/jsm/postprocessing/UnrealBloomPass.js';
import { SMAAPass } from 'three/examples/jsm/postprocessing/SMAAPass.js';
import { crearCamara, crearRotulos, frenteEn, enCurva,
         contarFibras, enhebrar, deshilachar } from './comun.mjs';

export { enCurva };

// EL COLOR DEL IMPULSO. Ámbar SATURADO, no el pálido del HUD: sobre mezcla aditiva y con bloom
// encima, un ámbar claro se lava a blanco y deja de leerse como ámbar. Ya pasó una vez.
const AMBAR = new THREE.Vector3(1.0, 0.60, 0.02);

/* ── LA ATMOSFERA ─────────────────────────────────────────────────────────────────────────────
   LA NIEBLA ESTABA MUERTA, y hacia meses que lo estaba. `scene.fog = FogExp2(...)` estaba puesto,
   se veia en el codigo y se pasaba por configuracion (`niebla: 0.0011`), pero NO LLEGABA A NINGUN
   MATERIAL: los seis son ShaderMaterial, y three los crea con `this.fog = false` (three.module.js
   :12312) y ademas hay que incluir <fog_fragment> a mano. Ninguno lo hacia. O sea que la unica
   señal de profundidad que habia era `prof`, que va por NIVEL DEL ARBOL y no por distancia: una
   hoja pegada al ojo se dibujaba apagada y un brazo de nivel 1 a 500 unidades, a pleno.

   Y revivirla tampoco alcanzaba, por algo medido: el rango de profundidad de la escena entera es
   apenas 2,3x entre lo mas cerca y lo mas lejos. Una densidad fija tiene UNA escala buena; sobre
   un rango de 2,3x no tiene recorrido para decir nada.

   Por eso la atmosfera se ancla a LA ORBITA: los dos extremos de la rampa salen de donde esta la
   camara y de cuanto mide la escena, asi que siempre hay contraste de profundidad — mirando el
   cuerpo entero y tambien metido adentro de una hoja.

   Y el orden importa: primero se va la SATURACION y despues la luminancia. Es perspectiva aerea de
   verdad; al reves —apagar sin desaturar— se ve como bajar el brillo, no como distancia. */
const ATM_DECL = `
uniform vec3 uFondo; uniform vec2 uAtm;
varying float vProf;
vec3 atmosfera(vec3 c){
  float a = smoothstep(uAtm.x, uAtm.y, vProf);
  float lum = dot(c, vec3(0.2126, 0.7152, 0.0722));
  c = mix(c, vec3(lum), a * 0.55);
  return mix(c, uFondo, a * 0.60);
}`;
const ATM_VARY = `varying float vProf;`;

/* ── shaders ───────────────────────────────────────────────────────────────────────────────── */

const VAINA_V = `
// ATRIBUTOS EMPAQUETADOS, y no por prolijidad: WebGL garantiza 16 atributos de vértice y esta
// malla llegó a 17 —position, normal, las CUATRO filas de instanceMatrix y diez escalares—. El
// síntoma no fue un error visible: fue «Too many attributes (aLargo)» por consola y la malla
// entera SIN DIBUJARSE, que se lee igual que una decisión de diseño. Lo destapó el cazador de
// errores en pantalla, no la consola.
//   aTNS  = (taper, nodos,  sel, nivel)
//   aDLVF = (dist,  largo,  vol, familia)
attribute vec3 aColor; attribute vec3 aCurva;
attribute vec4 aTNS; attribute vec4 aDLVF;
uniform float uFrente; uniform float uAncho;
varying vec3 vC; varying float vY; varying float vNod; varying float vSel; varying float vNiv;
varying vec3 vNrm; varying vec3 vView; varying float vPul; varying float vVol; varying float vFam;
${ATM_VARY}
void main(){
  vVol = aDLVF.z; vFam = aDLVF.w;
  vY = position.y + 0.5;
  // El adelgazamiento va acá y no en la matriz: una matriz sola no puede tener dos radios.
  float k = mix(1.0, aTNS.x, vY);
  vec3 p = vec3(position.x * k, position.y, position.z * k);
  vec4 wp = instanceMatrix * vec4(p, 1.0);
  // La panza se aplica DESPUÉS de la matriz de instancia: es un desplazamiento en el mundo, no una
  // deformación del cilindro, así que no se estira con la escala.
  wp.xyz += aCurva * sin(vY * 3.14159265);
  vec4 mv = modelViewMatrix * wp;
  vNrm = normalize(normalMatrix * mat3(instanceMatrix) * normal);
  vView = normalize(-mv.xyz);
  vC = aColor; vNod = aTNS.y; vSel = aTNS.z; vNiv = aTNS.w;
  // EL FRENTE DEL IMPULSO se calcula POR VÉRTICE contra la distancia recorrida desde la raíz, no
  // por instancia: así la luz entra por un extremo de la sección y sale por el otro, en vez de que
  // el tramo entero se encienda de golpe. Es lo que hace que se vea VIAJAR.
  float d = aDLVF.x - aDLVF.y * (1.0 - vY);
  vPul = uFrente < 0.0 ? 0.0 : max(0.0, 1.0 - abs(d - uFrente) / uAncho);
  vProf = -mv.z;
  gl_Position = projectionMatrix * mv;
}`;

const VAINA_F = `
precision highp float;
// LA LUZ. Un solo direccional en espacio de cámara —arriba, a la izquierda, hacia adelante—, que
// es la posición clásica de la luz clave. Antes la escena era fresnel puro: fresnel dibuja el
// BORDE de un cuerpo, no su forma, así que un cilindro quedaba como una silueta iluminada y por
// eso el conjunto se leía plano. Con un término difuso, el tubo tiene lado iluminado y lado en
// sombra, y ahí aparece el 3D. Va en espacio de cámara y no de mundo a propósito: acompaña al
// que mira, así que ninguna rama queda nunca completamente negra al girar.
const vec3 LUZ = normalize(vec3(-0.45, 0.72, 0.52));

varying vec3 vC; varying float vY; varying float vNod; varying float vSel; varying float vNiv;
varying vec3 vNrm; varying vec3 vView; varying float vPul; varying float vVol; varying float vFam;
uniform vec3 uAmbar;
${ATM_DECL}
void main(){
  // LOS INTERNODOS. la funcion fract sobre el largo da las estrías de la vaina; el valor va a 1 en el
  // estrangulamiento, que es donde está el nodo de Ranvier.
  float banda = abs(fract(vY * vNod) - 0.5) * 2.0;
  float nodo  = smoothstep(0.70, 1.0, banda);
  // La vaina brilla en el internodo y se apaga en el nodo: es el contraste que hace que la sección
  // se LEA como seccionada aunque esté lejos y los nodos ya no se distingan como geometría.
  float mielina = 0.55 + 0.45 * (1.0 - nodo);
  // Fresnel: el borde encendido es lo que le da volumen a un tubo sin luces en la escena.
  vec3 nn = normalize(vNrm);
  float fres = pow(1.0 - abs(dot(nn, normalize(vView))), 2.2);
  // Semi-lambert (0,5+0,5·N·L) y no lambert crudo: con lambert la mitad de cada tubo queda en
  // negro absoluto y las ramas se cortan por la mitad. El envolvente conserva la forma sin perder
  // la parte que mira para el otro lado.
  float dif = 0.5 + 0.5 * dot(nn, LUZ);
  // La profundidad se lee también en el color: las ramas hondas se apagan. Sin esto, el nivel 6
  // compite con el tronco por la atención y la jerarquía se pierde. 0,74 y no 0,62: con hilos
  // finos, la mitad honda del dibujo caía por debajo de lo que un panel oscuro deja ver.
  float prof = mix(1.0, 0.74, clamp(vNiv / 7.0, 0.0, 1.0));
  // MÁS LUZ DE BASE que en la versión de tubos, y no es gusto: un hilo mide menos de un píxel a
  // distancia de encuadre, así que el antialias lo promedia contra el fondo negro y lo apaga. Un
  // tubo gordo no tenía ese problema. Lo que se compensa es el submuestreo, no el color.
  // El equilibrio se midio en el render, en dos pasadas: con 0,42+0,78 (los tubos) los hilos
  // quedaban por debajo de lo que un panel oscuro deja ver —un hilo mide menos de un pixel a
  // distancia de encuadre y el antialias lo promedia contra el negro—; con 0,62+0,92 el turquesa
  // se lavaba a blanco. Lo que hace falta no es mas brillo sino mas UMBRAL de bloom, que es donde
  // se estaba perdiendo: aca se compensa el submuestreo y el bloom deja de florecer la rama entera.
  // vVol es el sombreado A NIVEL DE HAZ: sin el, todos los hilos paralelos reciben la luz igual y
  // el cable se ve como pana plana. Multiplica el termino base y NO el impulso — el impulso tiene
  // que verse igual de fuerte del lado en sombra, o dejaria de decir por donde viaja.
  vec3 base = (vC * mielina * prof * (0.74 + 1.30 * dif) + vC * fres * 0.66) * vVol;
  // ROLLOFF SUAVE en vez de recorte duro. Sin esto hay que elegir entre dos males: con ganancia
  // alta el turquesa saturado se pasa de 1 en dos canales y el haz cercano se lava a CIAN BLANCO
  // —se pierde el hilo, que es lo unico que importa mostrar—; con ganancia baja las puntas se
  // hunden en el negro. La compresion deja subir la ganancia sin que nada llegue a 1: comprime los
  // altos y casi no toca los bajos, y conserva el TONO, que un recorte por canal destruye.
  vec3 c = base / (1.0 + 0.34 * base);
  c += vC * vSel * 1.15;                       // la sección elegida y su camino a la raíz
  // CONDUCCIÓN SALTATORIA: el impulso no brilla parejo, brilla EN LOS NODOS. En un axón real la
  // corriente entra por los nodos de Ranvier y el internodo sólo la transmite, así que la luz
  // salta de estrangulamiento en estrangulamiento. Es la animación y es el hecho, a la vez.
  float enNodo = smoothstep(0.55, 1.0, banda);
  float p = vPul * vPul;
  // El frente recorre TODO el árbol —es una onda desde la raíz, y eso es lo que es— pero pega más
  // fuerte en el camino de la sección elegida: así la animación contesta «por dónde se llega hasta
  // acá» en vez de ser sólo un efecto bonito.
  c += uAmbar * p * (0.55 + 1.9 * enNodo) * (0.32 + 1.15 * vSel);
  // AISLAR. El atributo aFam vale 1 para lo que estas mirando, 0,45 para el camino del que cuelga
  // y 0,08 para el resto. Apagar en vez de esconder es a proposito: esconder contesta «¿como es
  // esta rama?» pero borra «¿donde esta?», y la segunda es la mitad de para que sirve un mapa.
  c *= vFam;
  // La atmosfera va DESPUES del impulso a proposito: un frente que viaja hacia el fondo tiene que
  // apagarse con la distancia como todo lo demas, o la animacion se despega del espacio.
  c = atmosfera(c);
  gl_FragColor = vec4(c, min(1.0, 0.90 + 0.10 * vSel + p * 0.4) * (0.35 + 0.65 * vFam));
}`;

/* ── EL CONTORNO POR PROFUNDIDAD ──────────────────────────────────────────────────────────────
   Es LA pieza que hace legible un haz denso, y no es un efecto: es la tecnica estandar de la
   tractografia (halos dependientes de la profundidad, Everts et al.). El problema que resuelve se
   midio antes de escribirla:

     · el 24 % de los pixeles ocupados tiene DOS O MAS secciones distintas encima;
     · pero el 83 % de esos pixeles tiene un salto de profundidad mayor a cuatro diametros de hilo,
       con una MEDIANA de 29 unidades.

   O sea: casi todo lo que se ve amontonado NO esta amontonado — esta a distintas profundidades y no
   hay ninguna señal que lo diga. La niebla no puede decirlo: el rango de profundidad de la escena
   entera es apenas 2,3x entre lo mas cerca y lo mas lejos, asi que cualquier atenuacion por
   distancia tiene casi nada de recorrido.

   COMO FUNCIONA: cada hilo se dibuja DOS veces. Primero una vaina mas gorda, del color del fondo,
   EMPUJADA HACIA ATRAS una distancia fija en espacio de camara; despues el hilo de verdad. La
   vaina gorda tapa lo que este mas atras que ese empujon y deja un hueco oscuro alrededor del hilo
   de adelante. Lo que este MAS CERCA que el empujon no se tapa — y eso es justamente lo que salva
   a los hilos vecinos del mismo haz, que estan a 1,4 unidades unos de otros: con un halo comun se
   comerian entre si y el haz se volveria una mancha negra con cuatro hilos encima. Por eso el halo
   tiene que DEPENDER DE LA PROFUNDIDAD y no ser un contorno a secas. */
const CONTORNO_V = `
attribute vec3 aCurva; attribute vec4 aTNS; attribute vec4 aDLVF;
uniform float uGrosor; uniform float uSalto;
void main(){
  float y = position.y + 0.5;
  // Un hilo apagado NO proyecta contorno: si lo hiciera, seguiria tapando a la rama que estas
  // mirando y aislar dejaria de servir — verias un hueco negro delante de lo que querias ver.
  // Colapsar la instancia a cero no dibuja ni un fragmento; apagar el color no alcanza porque el
  // contorno escribe PROFUNDIDAD, que es lo que tapa.
  float k = aDLVF.w < 0.5 ? 0.0 : mix(1.0, aTNS.x, y) * uGrosor;
  vec3 p = vec3(position.x * k, position.y, position.z * k);
  vec4 wp = instanceMatrix * vec4(p, 1.0);
  // La panza se aplica IGUAL que en la vaina real. Si no, el contorno no queda concentrico con su
  // hilo y se ve como una sombra corrida — el mismo error de espacios que ya nos mordio dos veces.
  wp.xyz += aCurva * sin(y * 3.14159265);
  vec4 mv = modelViewMatrix * wp;
  // La camara mira hacia -Z, asi que restar aleja. Este numero ES el invariante del efecto: separa
  // lo que esta a mas de uSalto por detras y no toca a los vecinos de adentro del haz.
  mv.z -= uSalto;
  gl_Position = projectionMatrix * mv;
}`;

const CONTORNO_F = `
precision highp float;
uniform vec3 uFondo;
void main(){ gl_FragColor = vec4(uFondo, 1.0); }`;

const CUERPO_V = `
attribute vec3 aColor; attribute vec2 aSF;   // (seleccion, familia)
varying vec3 vC; varying float vSel; varying vec3 vNrm; varying vec3 vView; varying float vFam;
${ATM_VARY}
void main(){
  vFam = aSF.y;
  vec4 wp = instanceMatrix * vec4(position, 1.0);
  vec4 mv = modelViewMatrix * wp;
  vNrm = normalize(normalMatrix * mat3(instanceMatrix) * normal);
  vView = normalize(-mv.xyz);
  vC = aColor; vSel = aSF.x; vProf = -mv.z;
  gl_Position = projectionMatrix * mv;
}`;

const CUERPO_F = `
precision highp float;
const vec3 LUZ = normalize(vec3(-0.45, 0.72, 0.52));
varying vec3 vC; varying float vSel; varying vec3 vNrm; varying vec3 vView; varying float vFam;
${ATM_DECL}
void main(){
  vec3 nn = normalize(vNrm);
  float fres = pow(1.0 - abs(dot(nn, normalize(vView))), 1.7);
  float dif = 0.5 + 0.5 * dot(nn, LUZ);
  // 0,22+0,62 y no 0,30+0,85: con el valor anterior el soma del tronco —que es el mas grande de
  // todos por la ley de Rall— saturaba a blanco puro y se comia el centro de la escena. Un cuerpo
  // celular tiene que leerse como volumen, no como una lampara.
  vec3 c = (vC * (0.14 + 0.55 * dif + 0.52 * fres) + vC * vSel * 1.3) * vFam;
  gl_FragColor = vec4(atmosfera(c), 1.0);
}`;

// EL PENACHO. Shader aparte y no un parámetro del de la vaina, porque la diferencia es real: las
// dendritas finas NO están mielinizadas — no tienen vaina ni nodos de Ranvier. Dibujarlas lisas no
// es simplificar, es dibujar lo que son, y el contraste entre la textura estriada del tronco y la
// lisa de las puntas es justamente lo que hace que se lea como tejido.
const PENACHO_V = `
attribute vec3 aColor; attribute vec3 aCurva;
attribute vec3 aSDF;   // (seleccion, distancia desde la raiz, familia)
uniform float uFrente; uniform float uAncho; uniform float uT; uniform float uVaiven;
varying vec3 vC; varying float vY; varying float vSel; varying float vPul; varying float vFam;
varying vec3 vNrm; varying vec3 vView;
${ATM_VARY}
void main(){
  vFam = aSDF.z;
  vY = position.y + 0.5;
  float k = mix(1.0, 0.45, vY);
  vec3 p = vec3(position.x * k, position.y, position.z * k);
  vec4 wp = instanceMatrix * vec4(p, 1.0);
  wp.xyz += aCurva * sin(vY * 3.14159265);
  // EL VAIVÉN, y va SÓLO en el penacho. Es tejido fino: en una preparación real las dendritas
  // terminales se mueven y el tronco no. La amplitud crece con vY —la punta se mueve, la base
  // queda anclada donde nace— y la fase sale de la distancia recorrida, así que ramitas vecinas
  // no oscilan al unísono, que es lo que delataría que es un seno y no tejido.
  //
  // Es DECORACIÓN y se declara como tal: no afirma nada sobre la memoria. Por eso no toca ni el
  // tronco ni los somas ni los botones — todo lo que sí afirma algo se queda quieto.
  float f = aSDF.y * 0.055;
  wp.xyz += vec3(sin(uT * 0.62 + f), sin(uT * 0.44 + f * 1.7) * 0.5, cos(uT * 0.55 + f))
            * vY * vY * uVaiven;
  vec4 mv = modelViewMatrix * wp;
  vNrm = normalize(normalMatrix * mat3(instanceMatrix) * normal);
  vView = normalize(-mv.xyz);
  vC = aColor; vSel = aSDF.x;
  vPul = uFrente < 0.0 ? 0.0 : max(0.0, 1.0 - abs(aSDF.y - uFrente) / uAncho);
  vProf = -mv.z;
  gl_Position = projectionMatrix * mv;
}`;

const PENACHO_F = `
precision highp float;
const vec3 LUZ = normalize(vec3(-0.45, 0.72, 0.52));
varying vec3 vC; varying float vY; varying float vSel; varying float vPul; varying float vFam;
varying vec3 vNrm; varying vec3 vView;
uniform vec3 uAmbar;
${ATM_DECL}
void main(){
  vec3 nn = normalize(vNrm);
  float fres = pow(1.0 - abs(dot(nn, normalize(vView))), 2.0);
  float dif = 0.5 + 0.5 * dot(nn, LUZ);
  // La punta se APAGA hacia el final. Una ramita que termina con el mismo brillo que empieza se
  // ve cortada; el desvanecido es lo que da la sensación de que sigue más allá de lo dibujado.
  float punta = mix(1.0, 0.38, vY);
  vec3 c = vC * (0.30 + 0.55 * dif + 0.62 * fres) * punta;
  c += vC * vSel * 0.9;
  c += uAmbar * vPul * vPul * 1.5;
  gl_FragColor = vec4(atmosfera(c * vFam), (0.62 + 0.38 * vSel) * punta * (0.30 + 0.70 * vFam));
}`;

// EL HALO DEL SOMA. Un cartel que siempre encara a la cámara, detrás del cuerpo celular. En las
// reconstrucciones reales el soma es lo único que se ve incandescente, y sin esto un icosaedro
// mate a media distancia se pierde entre las ramas que salen de él.
const HALO_V = `
attribute vec3 aColor; attribute float aR; attribute vec2 aSF;
varying vec3 vC; varying vec2 vUv; varying float vSel; varying float vFam;
void main(){
  vC = aColor; vUv = uv * 2.0 - 1.0; vSel = aSF.x; vFam = aSF.y;
  vec4 c = modelViewMatrix * instanceMatrix * vec4(0.0, 0.0, 0.0, 1.0);
  gl_Position = projectionMatrix * (c + vec4(position.x * aR, position.y * aR, 0.0, 0.0));
}`;

const HALO_F = `
precision highp float;
varying vec3 vC; varying vec2 vUv; varying float vSel; varying float vFam;
void main(){
  float d = length(vUv);
  if (d > 1.0) discard;
  float a = pow(1.0 - d, 2.8);
  gl_FragColor = vec4(vC * a * (1.0 + vSel * 1.4) * vFam, a * 0.55 * vFam);
}`;

// LA SINAPSIS. Une DOS BOTONES, o sea dos memorias que el cerebro relacionó — no dos ramas. Es
// donde el axón de una neurona toca la dendrita de otra, que es lo que muestra la tercera imagen
// de referencia. En reposo casi no se ven: a 584 arcos cruzando el árbol, dibujarlas con presencia
// convierte la escena en una madeja y tapa la ramificación, que es lo que se vino a mirar. Se leen
// por ACUMULACIÓN, como una corriente de fondo, y se encienden las que tocan lo que elegiste.
const SIN_V = `
attribute vec3 aColor; attribute vec3 aCurva; attribute float aConf; attribute vec2 aSF;
varying vec3 vC; varying float vY; varying float vSel; varying float vConf; varying float vFam;
void main(){
  vFam = aSF.y;
  vY = position.y + 0.5;
  vec4 wp = instanceMatrix * vec4(position, 1.0);
  wp.xyz += aCurva * sin(vY * 3.14159265);
  vC = aColor; vSel = aSF.x; vConf = aConf;
  gl_Position = projectionMatrix * modelViewMatrix * wp;
}`;

const SIN_F = `
precision highp float;
varying vec3 vC; varying float vY; varying float vSel; varying float vConf; varying float vFam;
void main(){
  // Se desvanece en los DOS extremos: un arco de brillo parejo choca contra el botón y se ve como
  // un palo clavado. Naciendo y muriendo tenue, el contacto se lee como contacto.
  float extremo = sin(vY * 3.14159265);
  // La de base es MUY tenue: 584 arcos cruzando el árbol con presencia lo convierten en una
  // madeja y tapan la ramificación, que es lo que se vino a mirar. Se leen por acumulación.
  float a = (0.035 + 0.13 * vConf + 0.80 * vSel) * extremo;
  gl_FragColor = vec4(vC * (0.5 + 1.6 * vSel), a * vFam);
}`;

/* ── EL PASE DE IDENTIDAD ─────────────────────────────────────────────────────────────────────
   «Lo importante es poder seleccionar CUALQUIERA, no un monton.»

   Lo que habia contestaba otra pregunta. El raycast analitico prueba la CURVA de cada seccion con
   una tolerancia del ancho del haz: elegir ahi es elegir un haz de 111 hilos y 537 memorias — un
   monton, literalmente. Y no habia forma de señalar un hilo ni una nota: los 19.270 eslabones y
   los 2.267 botones no eran direccionables. Medido: 441 cosas elegibles sobre 45.000 dibujadas,
   el 1 %.

   El pase de identidad contesta la pregunta correcta —QUE SE VE EXACTAMENTE EN ESTE PIXEL— porque
   la contesta el mismo rasterizador que dibujo la imagen. Cada instancia lleva su id en RGB, se
   redibuja la escena en un recorte de 21x21 pixeles alrededor del cursor y se lee ese pedacito.
   Lo que agarras es lo que ves, incluido cual de los cien hilos del haz.

   TRES TRAMPAS, y las tres estan tapadas acá:
     · El desplazamiento de vertices vive en el VERTEX SHADER (el afinado, la panza, el vaiven del
       penacho). Un pase de id que no lo replique pinta el id sobre la geometria SIN deformar y
       agarra donde el tubo NO esta. Por eso cada malla tiene su propio shader de id y no se usa
       `scene.overrideMaterial`, que reemplaza el material pero no el posicionado.
     · El id NO puede viajar como float por un varying: en el fragment shader `mediump` tiene ~10
       bits de mantisa y 46.000 no entra. Va como vec3 de bytes normalizados, que es exacto.
     · El pase NO pasa por el composer: el bloom promediaria ids vecinos y el SMAA los
       interpolaria, asi que cada pixel de borde saldria con un id que no es de nadie. */
const ID_F = `
precision highp float;
varying vec3 vIdRGB;
void main(){ gl_FragColor = vec4(vIdRGB, 1.0); }`;

// La vaina: mismo afinado y misma panza que el shader visual, o el id no cae donde se ve el hilo.
const ID_VAINA_V = `
attribute vec3 aCurva; attribute vec4 aTNS; attribute vec3 aIdRGB;
varying vec3 vIdRGB;
void main(){
  vIdRGB = aIdRGB;
  float y = position.y + 0.5;
  float k = mix(1.0, aTNS.x, y);
  vec4 wp = instanceMatrix * vec4(position.x * k, position.y, position.z * k, 1.0);
  wp.xyz += aCurva * sin(y * 3.14159265);
  gl_Position = projectionMatrix * modelViewMatrix * wp;
}`;

// El soma y el boton no se deforman: alcanza con la matriz de instancia.
const ID_PLANO_V = `
attribute vec3 aIdRGB;
varying vec3 vIdRGB;
void main(){
  vIdRGB = aIdRGB;
  gl_Position = projectionMatrix * modelViewMatrix * instanceMatrix * vec4(position, 1.0);
}`;

// El penacho SI se mueve: el vaiven es parte de donde esta la ramita en este cuadro. Sin
// replicarlo, el id de una ramita queda medio ancho corrido de la ramita que se ve.
const ID_PEN_V = `
attribute vec3 aCurva; attribute vec3 aSDF; attribute vec3 aIdRGB;
uniform float uT; uniform float uVaiven;
varying vec3 vIdRGB;
void main(){
  vIdRGB = aIdRGB;
  float y = position.y + 0.5;
  float k = mix(1.0, 0.45, y);
  vec4 wp = instanceMatrix * vec4(position.x * k, position.y, position.z * k, 1.0);
  wp.xyz += aCurva * sin(y * 3.14159265);
  float f = aSDF.y * 0.055;
  wp.xyz += vec3(sin(uT * 0.62 + f), sin(uT * 0.44 + f * 1.7) * 0.5, cos(uT * 0.55 + f))
            * y * y * uVaiven;
  gl_Position = projectionMatrix * modelViewMatrix * wp;
}`;

/* ── EL CAZADOR DE ERRORES ────────────────────────────────────────────────────────────────────
   Un shader que no compila NO se ve como un error: se ve como «esa malla no está». three lo grita
   por consola y la consola no sale en una captura, así que un `const LUZ` sin tipo —que ya pasó—
   se lee igual que una decisión de diseño. Esto lo trae a la pantalla, que es donde miro. */
let bannerErr = null;
function cazarErrores(host) {
  const mostrar = (txt) => {
    if (!bannerErr) {
      bannerErr = document.createElement('div');
      bannerErr.style.cssText = 'position:fixed;left:0;right:0;top:0;z-index:99;max-height:42vh;'
        + 'overflow:auto;background:rgba(60,8,12,.96);border-bottom:2px solid #f87171;color:#ffd9d9;'
        + 'font:12px/1.45 ui-monospace,monospace;padding:10px 14px;white-space:pre-wrap';
      host.appendChild(bannerErr);
    }
    bannerErr.textContent += txt + String.fromCharCode(10);
  };
  const orig = console.error.bind(console);
  console.error = (...a) => { orig(...a); mostrar(a.map(String).join(' ').slice(0, 1400)); };
  addEventListener('error', (e) => mostrar('EXCEPCIÓN: ' + (e.message || e)));
  addEventListener('unhandledrejection', (e) => mostrar('PROMESA: ' + (e.reason && e.reason.message || e.reason)));
}

/* ── construcción de la escena ─────────────────────────────────────────────────────────────── */

const _m = new THREE.Matrix4(), _q = new THREE.Quaternion(), _p = new THREE.Vector3(),
      _s = new THREE.Vector3(), _d = new THREE.Vector3(), _c = new THREE.Color(),
      _UP = new THREE.Vector3(0, 1, 0), _UP_X = new THREE.Vector3(1, 0, 0);

/**
 * montar: arma la escena completa a partir de las secciones ya colocadas.
 *
 * @param {object} cfg  {secciones, host, fondo, colorDe, camara, ornamento, titulo, subtitulo}
 */
export function montar(cfg) {
  const S = cfg.secciones;
  const host = cfg.host || document.body;
  cazarErrores(host);

  const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false });
  renderer.setPixelRatio(Math.min(devicePixelRatio, 2));
  renderer.setSize(innerWidth, innerHeight);
  host.appendChild(renderer.domElement);
  const scene = new THREE.Scene();
  scene.background = new THREE.Color(cfg.fondo || '#05070d');
  // NO se pone scene.fog: estaba puesto y NO HACIA NADA (ningun ShaderMaterial la consume), asi que
  // era codigo que parecia vivo. La profundidad la da `atmosfera()` adentro de cada shader.
  const camera = new THREE.PerspectiveCamera(52, innerWidth / innerHeight, 0.5, 9000);
  const cam = crearCamara(camera, renderer.domElement, cfg.camara || {});

  // BLOOM. Las reconstrucciones reales se ven así porque el tejido teñido EMITE: sin un halo
  // alrededor de lo brillante, un tubo de color plano se lee como plástico. El umbral alto (0,55)
  // es lo que evita que se convierta en niebla — sólo florecen los somas y el frente del impulso,
  // no la rama entera.
  const _dbs = new THREE.Vector2(); renderer.getDrawingBufferSize(_dbs);
  const composer = new EffectComposer(renderer,
    new THREE.WebGLRenderTarget(_dbs.x, _dbs.y, { samples: 4 }));
  composer.setSize(_dbs.x, _dbs.y);
  composer.addPass(new RenderPass(scene, camera));
  // UMBRAL 0,74 y no 0,55. Con 0,55 florecia el color propio de cada haz —un turquesa saturado
  // pasa ese umbral sin ayuda de nadie— y el resultado era una rama lavada a blanco donde ya no se
  // distinguia un hilo de otro: el bloom borraba justamente lo que este cambio vino a mostrar.
  // Con 0,74 florecen los somas, el nucleo y el frente del impulso, que es lo que tiene que emitir.
  const bloom = new UnrealBloomPass(new THREE.Vector2(_dbs.x, _dbs.y),
    cfg.bloom != null ? cfg.bloom : 0.72, 0.58,
    cfg.umbralBloom != null ? cfg.umbralBloom : 0.74);
  composer.addPass(bloom);
  composer.addPass(new SMAAPass(_dbs.x, _dbs.y));

  const mundo = new THREE.Group(); scene.add(mundo);

  /* ══ LOS HILOS ═══════════════════════════════════════════════════════════════════════════
     «Se sienten pocas neuronas, las ramas son inventadas.» Las dos mitades eran ciertas: una rama
     era UN cilindro con una textura de neurona encima — la geometría no contenía células, las
     dibujaba. Acá la rama no existe como objeto. Existen los HILOS, y la rama es lo que se ve
     cuando pasan muchos juntos, igual que un tracto real.

     `contarFibras` reparte los hilos de abajo hacia arriba y el número del padre es la SUMA de sus
     hijas — un axón no aparece ni desaparece en una bifurcación. Así el grosor deja de ser una
     fórmula sobre el dato (eso era la ley de Rall) y pasa a ser el dato: los hilos del tronco son,
     contados, todas las hojas del árbol.                                                        */
  contarFibras(S, { porMemoria: cfg.porMemoria || 6, maxHoja: cfg.maxHoja || 22 });
  const FIB = enhebrar(S, {
    radioHilo: cfg.radioHilo || 0.30, separacion: cfg.separacion || 3.4,
    largoNeurona: cfg.largoNeurona || 17, torsion: cfg.torsion || 0.6,
  });
  // Rango de eslabones de cada sección: `enhebrar` los emite agrupados, así que alcanza con los
  // bordes. Es lo que permite colgar cada memoria de UN hilo concreto y no de «la rama».
  const RANGO = new Map();
  FIB.forEach((e, i) => {
    const r = RANGO.get(e.sec);
    if (r) r[1] = i + 1; else RANGO.set(e.sec, [i, i + 1]);
  });

  /* ── 1 · los axones: un cilindro por NEURONA ────────────────────────────────────────────── */
  const n = S.length, nF = Math.max(1, FIB.length);
  const gVaina = new THREE.CylinderGeometry(1, 1, 1, 6, 1, true);
  // DOS vec4 EN VEZ DE OCHO ESCALARES. No es estetica: con ocho, esta malla pedia 17 atributos de
  // vertice y WebGL garantiza 16 — el shader NO COMPILABA y las 16.437 neuronas no se dibujaban,
  // que en pantalla se ve igual que «decidimos no dibujar los hilos».
  //   TNS  = (taper, nodos, seleccion, nivel)
  //   DLVF = (distancia, largo, volumen, familia)
  const AC = new Float32Array(nF * 3), ACUR = new Float32Array(nF * 3),
        TNS = new Float32Array(nF * 4), DLVF = new Float32Array(nF * 4);
  for (let k = 0; k < nF; k++) DLVF[k * 4 + 3] = 1;      // familia: todo visible al arrancar
  gVaina.setAttribute('aColor', new THREE.InstancedBufferAttribute(AC, 3));
  gVaina.setAttribute('aTNS', new THREE.InstancedBufferAttribute(TNS, 4));
  gVaina.setAttribute('aCurva', new THREE.InstancedBufferAttribute(ACUR, 3));
  gVaina.setAttribute('aDLVF', new THREE.InstancedBufferAttribute(DLVF, 4));
  // Los uniforms del impulso son UN SOLO objeto compartido por la vaina y el penacho: si fueran
  // dos, un desfase de un cuadro entre ellos partiría el frente en dos justo en la unión.
  // uAtm son los DOS EXTREMOS de la rampa de profundidad, en unidades de vista, y se recalculan
  // cada cuadro desde la orbita. Un solo objeto compartido por los cuatro materiales: dos objetos
  // separados se desincronizan en un cuadro y la escena se parte en dos atmosferas.
  const uAtm = { uFondo: { value: new THREE.Color(cfg.fondo || '#05070d') },
                 uAtm: { value: new THREE.Vector2(1, 2) } };
  const uPulso = { uFrente: { value: -1 }, uAncho: { value: 26 }, uAmbar: { value: AMBAR },
                   uT: { value: 0 }, uVaiven: { value: cfg.vaiven != null ? cfg.vaiven : 0.9 },
                   uFondo: uAtm.uFondo, uAtm: uAtm.uAtm };
  // OPACO, y no es un detalle de rendimiento. Con 5.000 cilindros transparentes el orden de
  // dibujo decide qué tapa a qué, así que el haz se veía como una nube lechosa en vez de como
  // fibras: la transparencia es justamente lo que borra la individualidad de cada hilo, que es lo
  // único que este cambio vino a mostrar.
  const mVaina = new THREE.ShaderMaterial({ vertexShader: VAINA_V, fragmentShader: VAINA_F,
    uniforms: uPulso, side: THREE.DoubleSide });
  const vainas = new THREE.InstancedMesh(gVaina, mVaina, nF);
  vainas.frustumCulled = false; mundo.add(vainas);

  // El contorno comparte la MISMA geometria y el MISMO instanceMatrix que la vaina: son los mismos
  // hilos, dibujados una vez mas gordos y corridos hacia atras. Compartir el atributo en vez de
  // copiarlo evita 16.437 matrices duplicadas y, sobre todo, evita que las dos versiones se
  // desincronicen si algun dia las instancias se recalculan.
  const uCont = {
    uGrosor: { value: cfg.grosorContorno != null ? cfg.grosorContorno : 3.0 },
    uSalto:  { value: cfg.saltoContorno  != null ? cfg.saltoContorno  : 11.0 },
    uFondo:  { value: new THREE.Color(cfg.fondo || '#05070d') },
  };
  const contorno = new THREE.InstancedMesh(gVaina,
    new THREE.ShaderMaterial({ vertexShader: CONTORNO_V, fragmentShader: CONTORNO_F,
      uniforms: uCont, side: THREE.DoubleSide }), nF);
  contorno.instanceMatrix = vainas.instanceMatrix;
  contorno.frustumCulled = false; contorno.renderOrder = -1; mundo.add(contorno);

  /* ── 2 · los somas: UN CUERPO CELULAR POR NEURONA ───────────────────────────────────────── */
  // Detalle 1 y no 2: son miles, y a este tamaño en pantalla la diferencia entre 80 y 320 caras no
  // se ve — el costo sí.
  const gSoma = new THREE.IcosahedronGeometry(1, 1);
  const SC = new Float32Array(nF * 3), SSF = new Float32Array(nF * 2);
  for (let k = 0; k < nF; k++) SSF[k * 2 + 1] = 1;
  gSoma.setAttribute('aColor', new THREE.InstancedBufferAttribute(SC, 3));
  gSoma.setAttribute('aSF', new THREE.InstancedBufferAttribute(SSF, 2));
  // Se fabrican DOS materiales con el MISMO objeto de uniforms en vez de clonar uno: clonar un
  // ShaderMaterial deep-copia los uniforms (cloneUniforms), asi que los botones se quedarian con la
  // atmosfera del primer cuadro — y eso no se ve como error, se ve como que los botones no respiran.
  const cuerpoMat = () => new THREE.ShaderMaterial({ vertexShader: CUERPO_V,
    fragmentShader: CUERPO_F, uniforms: uAtm });
  const mCuerpo = cuerpoMat();
  const somas = new THREE.InstancedMesh(gSoma, mCuerpo, nF);
  somas.frustumCulled = false; mundo.add(somas);

  /* ── 4 · los botones: UNA MEMORIA CADA UNO ──────────────────────────────────────────────── */
  let totBot = 0; for (const s of S) totBot += s.memorias.length;
  const gBot = new THREE.SphereGeometry(1, 6, 5);
  const BC = new Float32Array(totBot * 3), BSF = new Float32Array(Math.max(1, totBot) * 2);
  for (let k = 0; k < Math.max(1, totBot); k++) BSF[k * 2 + 1] = 1;
  gBot.setAttribute('aColor', new THREE.InstancedBufferAttribute(BC, 3));
  gBot.setAttribute('aSF', new THREE.InstancedBufferAttribute(BSF, 2));
  const botones = new THREE.InstancedMesh(gBot, cuerpoMat(), Math.max(1, totBot));
  botones.frustumCulled = false; mundo.add(botones);
  const BOT_DE = new Int32Array(Math.max(1, totBot));
  // Que MEMORIA es cada boton. Sin esto se puede señalar una nota y no se puede decir cual es,
  // que es exactamente la mitad de para que sirve poder señalarla.
  const BOT_MEM = new Array(Math.max(1, totBot)).fill(null);
  // DÓNDE ESTÁ CADA MEMORIA. Se llena al colocar los botones y es lo único que sabe unir una
  // relación del dato con un punto del dibujo. Se guarda acá y no en el objeto de la memoria: un
  // campo inyectado en objetos que otro código recrea es la bomba que dejó 586 sinapsis
  // dibujándose en la nada durante semanas.
  const POSMEM = new Map();
  const MEM_SEC = new Map();          // memoria → sección que la lleva, para encender por selección

  /* ── 5 · EL PENACHO TERMINAL, AHORA HILO POR HILO ───────────────────────────────────────── */
  // Antes brotaba uno por sección: decenas. Ahora brota de la punta de CADA HILO, que es lo que
  // hace un axón al llegar a destino — se abre en un ramillete de terminales. Son cientos, y ahí
  // es donde aparece la sensación de tejido que el dibujo no tenía.
  const RAM = deshilachar(FIB, S, { niveles: cfg.nivelesPenacho || 3,
                                    escala: cfg.escalaPenacho || 0.55,
                                    semilla: cfg.semillaPenacho || 97 });
  const gPen = new THREE.CylinderGeometry(1, 1, 1, 5, 1, true);
  const nR = Math.max(1, RAM.length);
  const PC = new Float32Array(nR * 3), PCUR = new Float32Array(nR * 3),
        PSDF = new Float32Array(nR * 3);
  for (let k = 0; k < nR; k++) PSDF[k * 3 + 2] = 1;
  gPen.setAttribute('aColor', new THREE.InstancedBufferAttribute(PC, 3));
  gPen.setAttribute('aCurva', new THREE.InstancedBufferAttribute(PCUR, 3));
  gPen.setAttribute('aSDF', new THREE.InstancedBufferAttribute(PSDF, 3));
  const mPen = new THREE.ShaderMaterial({ vertexShader: PENACHO_V, fragmentShader: PENACHO_F,
    uniforms: uPulso, transparent: true, side: THREE.DoubleSide, depthWrite: false });
  const penacho = new THREE.InstancedMesh(gPen, mPen, Math.max(1, RAM.length));
  penacho.frustumCulled = false; mundo.add(penacho);
  const PEN_DE = new Int32Array(Math.max(1, RAM.length));

  /* ── 6 · EL HALO DEL SOMA ───────────────────────────────────────────────────────────────── */
  const gHalo = new THREE.PlaneGeometry(1, 1);
  const HC = new Float32Array(n * 3), HR = new Float32Array(n), HSF = new Float32Array(n * 2);
  for (let k = 0; k < n; k++) HSF[k * 2 + 1] = 1;
  gHalo.setAttribute('aColor', new THREE.InstancedBufferAttribute(HC, 3));
  gHalo.setAttribute('aR', new THREE.InstancedBufferAttribute(HR, 1));
  gHalo.setAttribute('aSF', new THREE.InstancedBufferAttribute(HSF, 2));
  const halos = new THREE.InstancedMesh(gHalo,
    new THREE.ShaderMaterial({ vertexShader: HALO_V, fragmentShader: HALO_F,
      transparent: true, blending: THREE.AdditiveBlending, depthWrite: false }), n);
  halos.frustumCulled = false; halos.renderOrder = 2; mundo.add(halos);

  /* ── llenado ────────────────────────────────────────────────────────────────────────────── */
  const COLSEC = S.map((s) => new THREE.Color(cfg.colorDe(s)));
  const LUZW = (() => { const v = [-0.45, 0.72, 0.52], l = Math.hypot(v[0], v[1], v[2]);
                        return [v[0] / l, v[1] / l, v[2] / l]; })();
  const _u1 = new THREE.Vector3(), _u2 = new THREE.Vector3(), _u3 = new THREE.Vector3();

  /** tintar: el color de UN hilo. La sección dice de quién es; el hilo, cuál es.
   *  En una reconstrucción teñida dos células vecinas del mismo tipo se distinguen porque cada una
   *  tomó el colorante distinto — sin eso, mil hilos del mismo dueño se funden en una mancha y el
   *  haz vuelve a leerse como un tubo, que es exactamente lo que veníamos a romper. El corrimiento
   *  es determinista por hilo, así que el dibujo no cambia entre dos corridas. */
  function tintar(sec, fib, orden) {
    _c.copy(COLSEC[sec]);
    const h1 = ((fib * 2654435761) % 1000) / 1000;
    const h2 = ((fib * 40503 + orden * 7919) % 1000) / 1000;
    _c.offsetHSL((h1 - 0.5) * 0.085, 0, (h2 - 0.5) * 0.20);
    return _c;
  }

  /* 1 · UN AXÓN Y UN SOMA POR NEURONA */
  const FIB_SEC = new Int32Array(nF);
  let totNodos = 0;
  FIB.forEach((e, i) => {
    FIB_SEC[i] = e.sec;
    totNodos += e.nodos;
    tintar(e.sec, e.fib, e.orden);
    AC[i * 3] = _c.r; AC[i * 3 + 1] = _c.g; AC[i * 3 + 2] = _c.b;
    SC[i * 3] = _c.r; SC[i * 3 + 1] = _c.g; SC[i * 3 + 2] = _c.b;
    // El axón se afina hacia el terminal: es lo que hace que la punta se lea como punta.
    TNS[i * 4] = 0.80;
    // LUZ DE MUNDO, no de camara, y a proposito: si el sombreado del haz acompañara al que mira,
    // el cable se veria igual desde todos lados y no habria volumen que percibir. Fijo en el
    // mundo, girar alrededor de una rama muestra su lado iluminado y su lado en sombra, que es
    // exactamente la informacion que faltaba.
    const dd = e.nrad[0] * LUZW[0] + e.nrad[1] * LUZW[1] + e.nrad[2] * LUZW[2];
    DLVF[i * 4 + 2] = (0.46 + 0.54 * (0.5 + 0.5 * dd)) * (0.80 + 0.20 * e.borde);
    TNS[i * 4 + 1] = e.nodos; TNS[i * 4 + 3] = e.nivel;
    DLVF[i * 4] = e.dist; DLVF[i * 4 + 1] = e.largo;
    ACUR[i * 3] = e.curva[0]; ACUR[i * 3 + 1] = e.curva[1]; ACUR[i * 3 + 2] = e.curva[2];

    _p.set(e.a[0], e.a[1], e.a[2]);
    _d.set(e.b[0] - e.a[0], e.b[1] - e.a[1], e.b[2] - e.a[2]);
    const L = _d.length() || 0.001;
    _q.setFromUnitVectors(_UP, _u1.copy(_d).divideScalar(L));
    _m.compose(_u2.copy(_p).addScaledVector(_d, 0.5), _q, _s.set(e.r, L, e.r));
    vainas.setMatrixAt(i, _m);

    // EL SOMA VA EN EL ARRANQUE de cada neurona. Los del eslabón 0 caen todos a la misma altura
    // del haz, así que se agrupan en un anillo denso al principio de cada sección: eso es un
    // NÚCLEO DE RELEVO, y también es real — los cuerpos celulares de una vía no están
    // desparramados por el tracto, están juntos donde la vía hace relevo.
    const rs = e.r * (e.orden === 0 ? 2.1 : 1.8);
    _m.compose(_p, _q, _s.set(rs, rs, rs));
    somas.setMatrixAt(i, _m);
  });

  /* 2 · LOS BOTONES: cada memoria montada sobre UN HILO, no sobre «la rama» */
  let iB = 0;
  S.forEach((s, i) => {
    const mm = s.memorias;
    if (!mm.length) return;
    const rg = RANGO.get(i);
    if (!rg) return;
    // Los terminales de la sección: uno por hilo. La memoria k viaja por el hilo k % hilos, así
    // que las memorias de una hoja se reparten entre sus hilos en vez de amontonarse en uno.
    const term = [];
    for (let k = rg[0]; k < rg[1]; k++) if (FIB[k].ultimo) term.push(FIB[k]);
    if (!term.length) return;
    const porHilo = Math.ceil(mm.length / term.length);
    for (let k = 0; k < mm.length; k++) {
      const e = term[k % term.length];
      const u = (Math.floor(k / term.length) + 1) / (porHilo + 1);
      const kk = Math.sin(u * Math.PI);
      const qx = e.a[0] + (e.b[0] - e.a[0]) * u + e.curva[0] * kk;
      const qy = e.a[1] + (e.b[1] - e.a[1]) * u + e.curva[1] * kk;
      const qz = e.a[2] + (e.b[2] - e.a[2]) * u + e.curva[2] * kk;
      // Colgado AFUERA del hilo, en ángulo áureo: `boutons en passant`, que en un axón real hacen
      // sinapsis de paso sin esperar al final.
      _u1.set(e.dir[0], e.dir[1], e.dir[2]);
      _u2.copy(Math.abs(_u1.y) > 0.9 ? _UP_X : _UP).cross(_u1).normalize();
      _u3.copy(_u1).cross(_u2).normalize();
      const th = k * 2.399963;
      const rad = e.r * 1.75;
      const px = qx + _u2.x * Math.cos(th) * rad + _u3.x * Math.sin(th) * rad;
      const py = qy + _u2.y * Math.cos(th) * rad + _u3.y * Math.sin(th) * rad;
      const pz = qz + _u2.z * Math.cos(th) * rad + _u3.z * Math.sin(th) * rad;
      // El tamaño sale de la IMPORTANCIA de la memoria: es dato, no ruido decorativo.
      const imp = Math.max(1, Number(mm[k].importance) || 5);
      const rb = e.r * (0.62 + 0.11 * imp);
      _m.compose(_p.set(px, py, pz), _q.identity(), _s.set(rb, rb, rb));
      botones.setMatrixAt(iB, _m);
      if (mm[k] && mm[k].id) { POSMEM.set(mm[k].id, [px, py, pz]); MEM_SEC.set(mm[k].id, i); }
      BOT_MEM[iB] = mm[k] || null;
      tintar(i, e.fib, 3);
      BC[iB * 3] = _c.r; BC[iB * 3 + 1] = _c.g; BC[iB * 3 + 2] = _c.b;
      BOT_DE[iB] = i; iB++;
    }
  });

  /* 3 · EL HALO, uno por NÚCLEO DE RELEVO (o sea por sección), no por neurona */
  // Un billboard aditivo por cada una de las miles de neuronas lavaría la escena a blanco. El halo
  // marca dónde la vía hace relevo, que es la información que hace falta a distancia.
  S.forEach((s, i) => {
    _c.copy(COLSEC[i]);
    HC[i * 3] = _c.r; HC[i * 3 + 1] = _c.g; HC[i * 3 + 2] = _c.b;
    // El halo del NUCLEO va mas chico en proporcion: lleva 459 hilos apretados, asi que ya emite
    // por acumulacion, y con el mismo factor que una rama fina salia un bloque blanco tapando el
    // centro del cuadro. Lo que necesita destacarse ahi es la forma, no el brillo.
    HR[i] = (s.Rhaz || 1) * (i === 0 ? 1.15 : 2.4);
    _p.set(s.a[0], s.a[1], s.a[2]);
    _m.compose(_p, _q.identity(), _s.set(1, 1, 1));
    halos.setMatrixAt(i, _m);
  });

  RAM.forEach((x, i) => {
    _p.set(x.a[0], x.a[1], x.a[2]);
    _d.set(x.b[0] - x.a[0], x.b[1] - x.a[1], x.b[2] - x.a[2]);
    const l = _d.length() || 0.001;
    _q.setFromUnitVectors(_UP, _d.clone().normalize());
    _m.compose(_p.clone().addScaledVector(_d, 0.5), _q, _s.set(x.w0, l, x.w0));
    penacho.setMatrixAt(i, _m);
    // La ramita hereda el tinte de SU HILO, no el de la sección: si tomara el de la sección, el
    // penacho volvería a ser una mancha de un solo color colgando de un haz multicolor.
    tintar(x.seccion, x.fib, 5);
    PC[i * 3] = _c.r; PC[i * 3 + 1] = _c.g; PC[i * 3 + 2] = _c.b;
    PCUR[i * 3] = x.curva[0]; PCUR[i * 3 + 1] = x.curva[1]; PCUR[i * 3 + 2] = x.curva[2];
    PSDF[i * 3 + 1] = x.dist;
    PEN_DE[i] = x.seccion;
  });

  vainas.instanceMatrix.needsUpdate = true;
  somas.instanceMatrix.needsUpdate = true;
  botones.instanceMatrix.needsUpdate = true;
  penacho.instanceMatrix.needsUpdate = true;
  halos.instanceMatrix.needsUpdate = true;

  /* ── 7 · LAS SINAPSIS: relación real entre dos memorias ─────────────────────────────────── */
  // EL COLOR DE UNA RELACIÓN SE GANA. La paleta del panel pinta `related` de turquesa — que es
  // exactamente el color de gio en esta escena, así que sus 500 relaciones se leían como ramas
  // suyas cruzando el árbol. Acá el grueso va en un blanco frío neutro y sólo llevan color las
  // dos que piden una decisión: `conflicts_with` (algo se contradice) y `supersedes` (algo quedó
  // reemplazado). Las demás son tejido conectivo: importan por dónde pasan, no por su tono.
  const NEUTRO = '#9fb6d8';
  const RELCOL = { conflicts_with: '#ff5d5d', supersedes: '#c08bff' };
  const SIN = [];
  for (const y of (cfg.sinapsis || [])) {
    const A = POSMEM.get(y.source), B = POSMEM.get(y.target);
    // Una relación cuyo extremo no está dibujado NO se inventa a mitad de camino. Se cuenta y se
    // declara aparte: recortar sin decir cuánto es como un dibujo empieza a mentir.
    if (!A || !B) continue;
    SIN.push({ A, B, rel: y.relation, conf: Math.max(0, Math.min(1, Number(y.confidence) || 0)),
               ma: y.source, mb: y.target });
  }
  const sinRecortadas = (cfg.sinapsis || []).length - SIN.length;

  let sinInst = null, YSF = null, SIN_SEC = null;
  if (SIN.length) {
    const gSin = new THREE.CylinderGeometry(1, 1, 1, 4, 1, true);
    const YC = new Float32Array(SIN.length * 3), YCUR = new Float32Array(SIN.length * 3),
          YCONF = new Float32Array(SIN.length);
    YSF = new Float32Array(SIN.length * 2);
    for (let k = 0; k < SIN.length; k++) YSF[k * 2 + 1] = 1;
    SIN_SEC = new Array(SIN.length);
    gSin.setAttribute('aColor', new THREE.InstancedBufferAttribute(YC, 3));
    gSin.setAttribute('aCurva', new THREE.InstancedBufferAttribute(YCUR, 3));
    gSin.setAttribute('aConf', new THREE.InstancedBufferAttribute(YCONF, 1));
    gSin.setAttribute('aSF', new THREE.InstancedBufferAttribute(YSF, 2));
    sinInst = new THREE.InstancedMesh(gSin,
      new THREE.ShaderMaterial({ vertexShader: SIN_V, fragmentShader: SIN_F, transparent: true,
        blending: THREE.AdditiveBlending, depthWrite: false }), SIN.length);
    sinInst.frustumCulled = false; sinInst.renderOrder = 1; mundo.add(sinInst);
    const _u = new THREE.Vector3(), _w = new THREE.Vector3();
    SIN.forEach((y, i) => {
      _p.set(y.A[0], y.A[1], y.A[2]);
      _d.set(y.B[0] - y.A[0], y.B[1] - y.A[1], y.B[2] - y.A[2]);
      const len = _d.length() || 0.001;
      _q.setFromUnitVectors(_UP, _u.copy(_d).normalize());
      // EL RADIO SE CALCULA ACÁ, en el bucle, y no se guarda en el objeto de la relación. Es la
      // lección de las 586 sinapsis que desaparecieron: un NaN en la matriz de una instancia la
      // borra sin error ni warning, y se ve idéntico a «decidimos no dibujar relaciones».
      const r = 0.10 + y.conf * 0.14;
      _m.compose(_p.clone().addScaledVector(_d, 0.5), _q, _s.set(r, len, r));
      sinInst.setMatrixAt(i, _m);
      _c.set(RELCOL[y.rel] || NEUTRO);
      YC[i * 3] = _c.r; YC[i * 3 + 1] = _c.g; YC[i * 3 + 2] = _c.b;
      YCONF[i] = y.conf;
      // Arqueada, y hacia un lado que depende de la dirección: dos relaciones entre el mismo par
      // quedarían superpuestas si fueran rectas, y se leerían como una sola.
      _w.set(_u.z, 0.35, -_u.x).normalize().multiplyScalar(len * 0.17);
      YCUR[i * 3] = _w.x; YCUR[i * 3 + 1] = _w.y; YCUR[i * 3 + 2] = _w.z;
      SIN_SEC[i] = [MEM_SEC.get(y.ma), MEM_SEC.get(y.mb)];
    });
    sinInst.instanceMatrix.needsUpdate = true;
  }

  /* ── LOS IDS ────────────────────────────────────────────────────────────────────────────────
     Un espacio de ids CONTIGUO sobre las tres familias direccionables, con el 0 reservado para
     «el fondo»: si el 0 fuera una instancia valida, un clic al vacio se leeria como un acierto.
       1                        .. nF                 -> neurona (eslabon)
       nF+1                     .. nF+totBot          -> memoria (boton)
       nF+totBot+1              .. nF+totBot+RAM      -> terminal (ramita)
     Los HALOS quedan afuera a proposito: son discos de 2,4·Rhaz encarando a la camara y taparian
     todo lo que hay detras. Las SINAPSIS tambien: en reposo son casi invisibles y no pueden robar
     un clic que el ojo cree estar dando sobre una rama. Las dos exclusiones se DECLARAN, no se
     esconden — la ficha dice cuantos elementos son elegibles de cuantos dibujados. */
  const BASE_NEURONA = 1, BASE_MEM = 1 + nF, BASE_RAM = 1 + nF + totBot;
  const ID_TOTAL = BASE_RAM + RAM.length - 1;
  const idRGB = (cuantos, base) => {
    const a = new Uint8Array(Math.max(1, cuantos) * 3);
    for (let i = 0; i < cuantos; i++) {
      const v = base + i;
      a[i * 3] = v & 255; a[i * 3 + 1] = (v >> 8) & 255; a[i * 3 + 2] = (v >> 16) & 255;
    }
    return new THREE.InstancedBufferAttribute(a, 3, true);   // normalizado: byte exacto, no float
  };
  const AID_FIB = idRGB(nF, BASE_NEURONA);
  gVaina.setAttribute('aIdRGB', AID_FIB);
  gSoma.setAttribute('aIdRGB', AID_FIB);         // el soma ES su eslabon: mismo id, mas area de clic
  gBot.setAttribute('aIdRGB', idRGB(totBot, BASE_MEM));
  gPen.setAttribute('aIdRGB', idRGB(RAM.length, BASE_RAM));

  const mIdVaina = new THREE.ShaderMaterial({ vertexShader: ID_VAINA_V, fragmentShader: ID_F,
                                              side: THREE.DoubleSide });
  const mIdPlano = new THREE.ShaderMaterial({ vertexShader: ID_PLANO_V, fragmentShader: ID_F });
  const mIdPen = new THREE.ShaderMaterial({ vertexShader: ID_PEN_V, fragmentShader: ID_F,
                                            uniforms: uPulso, side: THREE.DoubleSide });
  // Una escena aparte con mallas que COMPARTEN geometria e instanceMatrix con las visuales: un
  // Object3D tiene un solo padre, asi que no se pueden reusar los objetos, pero si sus datos. Nada
  // se duplica en memoria y no hay una segunda fuente de verdad que se pueda desincronizar.
  const escenaId = new THREE.Scene();
  const gemela = (geo, mat, mesh, cuenta) => {
    const m = new THREE.InstancedMesh(geo, mat, cuenta);
    m.instanceMatrix = mesh.instanceMatrix;
    m.frustumCulled = false; escenaId.add(m); return m;
  };
  gemela(gVaina, mIdVaina, vainas, nF);
  gemela(gSoma, mIdPlano, somas, nF);
  if (totBot) gemela(gBot, mIdPlano, botones, totBot);
  if (RAM.length) gemela(gPen, mIdPen, penacho, RAM.length);

  const LADO = 21;                                  // la mordida, en pixeles CSS
  const rtId = new THREE.WebGLRenderTarget(LADO, LADO, {
    minFilter: THREE.NearestFilter, magFilter: THREE.NearestFilter,
    format: THREE.RGBAFormat, type: THREE.UnsignedByteType, depthBuffer: true, samples: 0 });
  const bufId = new Uint8Array(LADO * LADO * 4);

  /**
   * sondear: QUE HAY EXACTAMENTE bajo (x,y). Devuelve {tipo, id, sec, ...} o null.
   *
   * Se recorta la proyeccion a una ventana de 21x21 con setViewOffset —la camara es la MISMA, solo
   * se rasteriza ese pedazo—, asi que el costo de fragmento es despreciable. Gana el acierto mas
   * cercano al cursor EN PANTALLA, no en el mundo: el usuario apunta con la pantalla, y el mas
   * cercano en el mundo puede estar detras de otra cosa. Agarrar algo que no se ve es peor que no
   * agarrar nada.
   */
  function sondear(x, y) {
    const W = innerWidth, H = innerHeight, m = (LADO - 1) / 2;
    camera.setViewOffset(W, H, Math.round(x) - m, Math.round(y) - m, LADO, LADO);
    const antesRT = renderer.getRenderTarget();
    renderer.setRenderTarget(rtId);
    renderer.setClearColor(0x000000, 1);
    renderer.clear(true, true, false);
    renderer.render(escenaId, camera);
    renderer.readRenderTargetPixels(rtId, 0, 0, LADO, LADO, bufId);
    renderer.setRenderTarget(antesRT);
    camera.clearViewOffset();

    let mejor = 0, mejorD = Infinity;
    for (let fy = 0; fy < LADO; fy++) for (let fx = 0; fx < LADO; fx++) {
      const o = (fy * LADO + fx) * 4;
      const id = bufId[o] | (bufId[o + 1] << 8) | (bufId[o + 2] << 16);
      if (!id) continue;                              // 0 = fondo, y por eso los ids arrancan en 1
      // La lectura viene de abajo hacia arriba; para la DISTANCIA AL CENTRO da igual, porque la
      // ventana esta centrada en el cursor y la medida es simetrica.
      const d = (fx - m) * (fx - m) + (fy - m) * (fy - m);
      if (d < mejorD) { mejorD = d; mejor = id; }
    }
    if (!mejor) return null;
    if (mejor < BASE_MEM) {
      const i = mejor - BASE_NEURONA;
      return { tipo: 'neurona', id: mejor, i, sec: FIB[i].sec, fib: FIB[i].fib };
    }
    if (mejor < BASE_RAM) {
      const i = mejor - BASE_MEM;
      return { tipo: 'memoria', id: mejor, i, sec: BOT_DE[i], mem: BOT_MEM[i] };
    }
    const i = mejor - BASE_RAM;
    return { tipo: 'terminal', id: mejor, i, sec: PEN_DE[i], fib: RAM[i].fib };
  }

  if (cfg.ornamento) cfg.ornamento(mundo, S, THREE);

  /* ── ENCUADRE INICIAL SACADO DE LA ESCENA, no de un numero elegido a ojo ──────────────────
     Un `dist` fijo en la configuracion funciona hasta que la colocacion cambia de forma, y ahi
     se sale del cuadro sin que nada avise. Peor: el foco en el origen deja la mitad de arriba
     afuera cuando la escena no esta centrada en cero — que es exactamente lo que pasa con las
     laminas, que crecen hacia arriba. Se mide la caja y se despeja la distancia del FOV. */
  // El encuadre completo se GUARDA, no se recalcula: «ver todo» tiene que devolverte exactamente
  // a donde arrancaste, y un recalculo con otra ventana te deja en otro lado.
  let ENCUADRE0 = null;
  // EL RADIO DE LA ESCENA SE MIDE SIEMPRE, aunque la camara venga con distancia fija: no lo usa
  // solo el encuadre, lo usa la rampa de profundidad de cada cuadro. Calcularlo adentro del `if`
  // dejaba a la atmosfera sin escala cuando alguien pasaba `dist` a mano — y eso no se ve como
  // error, se ve como que la profundidad «a veces no anda».
  let RESC = 1, CX = 0, CY = 0, CZ = 0;
  {
    let mnx = 1e9, mny = 1e9, mnz = 1e9, mxx = -1e9, mxy = -1e9, mxz = -1e9;
    for (const s of S) for (let k = 0; k <= 4; k++) {
      const q = enCurva(s, k / 4);
      if (q[0] < mnx) mnx = q[0]; if (q[0] > mxx) mxx = q[0];
      if (q[1] < mny) mny = q[1]; if (q[1] > mxy) mxy = q[1];
      if (q[2] < mnz) mnz = q[2]; if (q[2] > mxz) mxz = q[2];
    }
    CX = (mnx + mxx) / 2; CY = (mny + mxy) / 2; CZ = (mnz + mxz) / 2;
    for (const s2 of S) for (let k = 0; k <= 4; k++) {
      const q = enCurva(s2, k / 4);
      const d2 = Math.hypot(q[0] - CX, q[1] - CY, q[2] - CZ);
      if (d2 > RESC) RESC = d2;
    }
  }
  if (!cfg.camara || cfg.camara.dist == null) {
    const cx = CX, cy = CY, cz = CZ;
    // RADIO REAL al punto mas lejano, no media diagonal de la caja. La diagonal sobreestima
    // muchisimo en una forma achatada y el encuadre sale con la escena ocupando un tercio del
    // cuadro y el resto negro. Medido: 1,6x de sobra en el boceto B.
    const R = RESC;
    // Radio de la esfera que envuelve la caja, despejado contra el FOV VERTICAL y corregido por
    // el aspecto: en una ventana apaisada lo que se sale primero es el alto, no el ancho.
    // El FOV de three es VERTICAL. En una ventana apaisada el limite es el alto, asi que se
    // despeja contra el vertical y listo; en una ventana mas alta que ancha manda el horizontal y
    // hay que dividir por el aspecto, o la escena se sale por los costados.
    const fov = (camera.fov * Math.PI) / 180;
    const efec = camera.aspect >= 1 ? fov : 2 * Math.atan(Math.tan(fov / 2) * camera.aspect);
    // 0,92 y no 1,06: la caja se mide sobre los ejes de los haces, pero lo que se ve incluye los
    // penachos terminales, que sobresalen. Con holgura mayor a 1 la escena quedaba ocupando media
    // pantalla y el resto negro — medido sobre el render, no calculado.
    const d = (R / Math.sin(efec / 2)) * 0.82;
    cam.meta.foco.set(cx, cy, cz); cam.est.foco.set(cx, cy, cz);
    cam.meta.dist = d; cam.est.dist = d;
    ENCUADRE0 = { p: [cx, cy, cz], d };
  }

  /* ── rótulos: el nombre REAL del tema sobre las ramas gruesas ───────────────────────────── */
  const rot = crearRotulos(host);
  function reRotular() {
    const items = [];
    for (const s of S) {
      // El nucleo NO se rotula en la escena: su nombre esta en la miga y su rotulo caia justo
      // encima de los tres actores, que salen todos de el. Un rotulo que tapa a otros tres cuesta
      // mas de lo que aporta.
      if (!s.etiqueta || s.nivel === 0 || s.nivel > 3) continue;
      // 0,90 y no 0,78: con el nucleo, TODOS los actores arrancan del mismo punto central, asi que
      // a mitad de camino sus rotulos caen practicamente encima unos de otros y encima del titulo.
      // Cuanto mas cerca de la punta, mas separados estan entre si.
      const q = enCurva(s, s.nivel <= 1 ? 0.90 : 0.78);
      items.push({
        p: q, texto: s.etiqueta,
        color: cfg.colorDe(s),
        prio: s.carga,                       // gana el que mas memoria representa, no el primero
        // Los actores son imprescindibles: si el rotulo de uno se esconde, la escena afirma que
        // ese actor no esta ahi. Los temas si se pueden esconder — son decenas y se recuperan
        // acercandose.
        imprescindible: s.nivel <= 1,
        tam: s.nivel <= 1 ? 15 : s.nivel === 2 ? 12 : 10.5,
        // Cada nivel aparece a su distancia: todos juntos vuelven a ser la maraña, pero de texto.
        rango: s.nivel <= 1 ? 1e9 : s.nivel === 2 ? 620 : 300,
      });
    }
    rot.set(items);
  }
  reRotular();

  /* ═══ NAVEGACIÓN ════════════════════════════════════════════════════════════════════════
     Es la mitad del pedido: «es muy difícil moverse entre ramas». La respuesta no es afinar la
     sensibilidad del mouse — es que moverse entre ramas deje de ser puntería y pase a ser recorrer
     la estructura, que ya existe y estaba desaprovechada.                                      */

  let sel = 0, hov = -1;

  /** camino a la raíz: encender la rama de la que colgás es lo que hace legible la jerarquía. */
  function camino(i) { const c = []; let k = i; while (k >= 0) { c.push(k); k = S[k].padre; } return c; }

  // LA SELECCIÓN VIVE POR SECCIÓN Y SE REPARTE A CADA HILO. Antes `aSel` era por sección porque
  // una sección era una instancia; ahora una sección son decenas de instancias, así que hay un
  // paso intermedio. Escribir directo sobre ASEL con índices de sección —que es lo que había—
  // encendería el hilo número 3, no la sección 3: se ve como que la selección agarra cualquier cosa.
  const SELSEC = new Float32Array(n);
  // EL FOCO EXACTO. `sel` sigue siendo la seccion —de ahi salen las migas, las flechas y el
  // impulso—, pero lo que el usuario señalo es UNA instancia concreta. Se guardan las dos cosas
  // porque contestan preguntas distintas: la seccion dice DONDE estas parado en el arbol, el foco
  // dice QUE agarraste. Colapsarlas seria volver a elegir un monton.
  let foco = null;
  // FAMSEC: cuanto se ve cada seccion. 1 = lo que estas mirando, 0,45 = el camino del que cuelga,
  // 0,08 = el resto. Sin aislar, todo en 1.
  const FAMSEC = new Float32Array(n).fill(1);
  let aislado = -1;

  /** aislar: dejar UNA rama sola. Es lo que el usuario pidio con «que sean mas solas o separadas
   *  para ver cada uno»: la separacion en el espacio tiene un techo medido (25,5 % de solape en
   *  pantalla es lo mejor que da el reparto), asi que lo que falta no se consigue moviendo ramas
   *  sino APAGANDO las otras. Y se apagan, no se esconden: esconderlas contesta «como es esta
   *  rama» pero borra «donde esta», que es la otra mitad de para que sirve un mapa. */
  function aislar(i) {
    aislado = i;
    if (i < 0) { FAMSEC.fill(1); }
    else {
      FAMSEC.fill(0.08);
      // el camino hasta la raiz, para que la rama no quede flotando sin de donde cuelga
      let k = i; while (k >= 0) { FAMSEC[k] = Math.max(FAMSEC[k], 0.45); k = S[k].padre; }
      // y todo su subarbol al maximo
      (function bajar(j) { FAMSEC[j] = 1; for (const h of S[j].hijos) bajar(h); })(i);
    }
    repartirFamilia();
    hud();
  }

  function repartirFamilia() {
    for (let k = 0; k < nF; k++) { const f = FAMSEC[FIB_SEC[k]]; DLVF[k * 4 + 3] = f; SSF[k * 2 + 1] = f; }
    for (let k = 0; k < totBot; k++) BSF[k * 2 + 1] = FAMSEC[BOT_DE[k]];
    for (let k = 0; k < RAM.length; k++) PSDF[k * 3 + 2] = FAMSEC[PEN_DE[k]];
    for (let k = 0; k < n; k++) HSF[k * 2 + 1] = FAMSEC[k];
    gVaina.attributes.aDLVF.needsUpdate = true; gSoma.attributes.aSF.needsUpdate = true;
    if (totBot) gBot.attributes.aSF.needsUpdate = true;
    if (RAM.length) gPen.attributes.aSDF.needsUpdate = true;
    gHalo.attributes.aSF.needsUpdate = true;
    if (sinInst && YSF) {
      for (let k = 0; k < YSF.length / 2; k++) {
        const par = SIN_SEC[k];
        // una sinapsis se ve si CUALQUIERA de sus dos puntas esta en lo que estas mirando: si
        // pidiera las dos, aislar una rama borraria justo las relaciones que salen de ella, que es
        // lo mas interesante que tiene para contar.
        YSF[k * 2 + 1] = Math.max(par[0] != null ? FAMSEC[par[0]] : 0, par[1] != null ? FAMSEC[par[1]] : 0);
      }
      sinInst.geometry.attributes.aSF.needsUpdate = true;
    }
  }

  function pintarSeleccion() {
    SELSEC.fill(0);
    const c = camino(sel);
    // El camino a la raíz se enciende con degradado: la sección elegida al máximo y los ancestros
    // cada vez más tenues. Un camino plano dice «esto está resaltado»; el degradado dice «venís
    // de allá», que es la información que hace falta para no perderse.
    c.forEach((k, d) => { SELSEC[k] = Math.max(0.10, 1 - d * 0.17); });
    for (const h of S[sel].hijos) SELSEC[h] = Math.max(SELSEC[h], 0.34);
    if (hov >= 0 && hov !== sel) SELSEC[hov] = Math.max(SELSEC[hov], 0.55);
    for (let k = 0; k < nF; k++) { const v = SELSEC[FIB_SEC[k]]; TNS[k * 4 + 2] = v; SSF[k * 2] = v; }
    for (let k = 0; k < totBot; k++) BSF[k * 2] = SELSEC[BOT_DE[k]];
    for (let k = 0; k < RAM.length; k++) PSDF[k * 3] = SELSEC[PEN_DE[k]];
    for (let k = 0; k < n; k++) HSF[k * 2] = SELSEC[k];
    // Y encima de todo eso, la instancia exacta al maximo. Es la diferencia visible entre «elegi
    // este haz» y «elegi ESTE hilo».
    if (foco) {
      if (foco.tipo === 'neurona') { TNS[foco.i * 4 + 2] = 1.6; SSF[foco.i * 2] = 1.6; }
      else if (foco.tipo === 'memoria' && totBot) BSF[foco.i * 2] = 1.8;
      else if (foco.tipo === 'terminal' && RAM.length) PSDF[foco.i * 3] = 1.8;
    }
    if (sinInst && YSF) {
      for (let k = 0; k < YSF.length / 2; k++) {
        const p = SIN_SEC[k];
        // Se enciende si CUALQUIERA de sus dos memorias está en lo seleccionado: una relación es
        // simétrica en lo que muestra, aunque el dato tenga dirección.
        YSF[k * 2] = Math.max(p[0] != null ? SELSEC[p[0]] : 0, p[1] != null ? SELSEC[p[1]] : 0);
      }
      sinInst.geometry.attributes.aSF.needsUpdate = true;
    }
    gVaina.attributes.aTNS.needsUpdate = true; gSoma.attributes.aSF.needsUpdate = true;
    if (totBot) gBot.attributes.aSF.needsUpdate = true;
    if (RAM.length) gPen.attributes.aSDF.needsUpdate = true;
    gHalo.attributes.aSF.needsUpdate = true;
    hud();
  }

  /** encuadrar: la distancia sale del TAMAÑO de lo que se va a mirar, no de un número fijo. */
  function encuadrar(i, brusco) {
    const s = S[i];
    const q = enCurva(s, 0.5);
    // El alcance de una sección incluye a sus hijas: mirar una rama gruesa y ver sólo su tramo
    // deja fuera todo lo que cuelga, que es justamente lo que fuiste a ver.
    let ext = s.largo * 1.1;
    for (const h of s.hijos) ext = Math.max(ext, Math.hypot(
      S[h].b[0] - q[0], S[h].b[1] - q[1], S[h].b[2] - q[2]) * 1.5);
    cam.volarA(q, Math.max(26, ext * 1.55), brusco ? null : undefined);
  }

  // EL IMPULSO SALE DE UN ACTO, no de un temporizador. Es la misma regla que el panel: sin evento
  // no hay luz. Acá el evento es elegir una sección — el frente sale de la raíz y llega hasta ella,
  // que es la respuesta visual a «cómo se llega hasta acá».
  let pulT0 = -1, pulMax = 0, pulVel = 1;
  // avanzarPulso: dónde está el frente `ms` después de haber salido. Vive aparte del bucle para
  // que la prueba lo pueda correr sin depender de que el navegador dé cuadros — bajo
  // --virtual-time-budget no da ninguno, y medir contra rAF ahí devuelve «no se movió» para algo
  // que funciona perfecto.
  function avanzarPulso(ms) {
    const v = frenteEn(ms, pulVel, pulMax, uPulso.uAncho.value);
    if (v < 0) pulT0 = -1;
    uPulso.uFrente.value = v;
    return v;
  }

  function lanzarPulso(i) {
    pulMax = (S[i].dist || 0) + (S[i].largo || 0) * 0.6 + 40;
    pulVel = pulMax / 1.25;                       // 1,25 s de la raíz al destino, mida lo que mida
    pulT0 = performance.now();
  }

  /** verTodo: saca el aislamiento y vuelve al encuadre completo. Es el boton que pidio el usuario
   *  y tambien la salida de emergencia: en cualquier navegador 3D, lo primero que se necesita
   *  cuando te perdiste es UN gesto que te devuelva a la vista general. */
  function verTodo() {
    if (aislado >= 0) aislar(-1);
    sel = 0; pintarSeleccion();
    if (ENCUADRE0) cam.volarA(ENCUADRE0.p, ENCUADRE0.d);
    else encuadrar(0);
  }

  function elegir(i, sinVolar) {
    if (i < 0 || i >= S.length) return;
    sel = i; pintarSeleccion(); lanzarPulso(i);
    if (!sinVolar) encuadrar(i);
  }

  /* ── el picking va contra la CURVA, no contra el cilindro ────────────────────────────────
     Un raycast contra la malla instanciada mide el tubo RECTO: el shader lo arquea después, así
     que con curvatura 0,20 el punto donde hay que hacer clic queda hasta un 20 % del largo lejos
     de donde se ve el tubo. Se ve como «el clic no agarra». Muestreando la misma curva que dibuja
     el shader, lo que se clickea es lo que se ve. Son ~250 secciones × 7 muestras: nada. */
  const ray = new THREE.Raycaster(); const ptr = new THREE.Vector2();
  const _t = new THREE.Vector3();
  function bajoElCursor(x, y) {
    ptr.set((x / innerWidth) * 2 - 1, -(y / innerHeight) * 2 + 1);
    ray.setFromCamera(ptr, camera);
    let mejor = -1, mejorD = Infinity;
    for (let i = 0; i < S.length; i++) {
      const s = S[i];
      for (let k = 0; k <= 6; k++) {
        const q = enCurva(s, k / 6);
        _t.set(q[0], q[1], q[2]);
        const d = ray.ray.distanceToPoint(_t);
        // Tolerancia proporcional al grosor: una rama fina pide más puntería que el tronco, que es
        // lo natural, pero con un piso para que las hojas sigan siendo clickeables.
        // La tolerancia es el RADIO REAL DEL HAZ, que ahora existe como número: lo que se clickea
        // es exactamente el cilindro que ocupan los hilos, ni más ni menos. Con un piso, para que
        // una hoja de un solo hilo siga siendo clickeable.
        const tol = Math.max(2.2, (s.Rhaz || 1) * 1.25);
        if (d < tol) {
          // se desempata por CERCANÍA A LA CÁMARA, no por distancia al rayo: si no, una rama lejana
          // y gorda le gana a la fina que tenés adelante.
          const dc = camera.position.distanceTo(_t);
          if (dc < mejorD) { mejorD = dc; mejor = i; }
        }
      }
    }
    return mejor;
  }

  let mx = 0, my = 0, movido = false, abajo = null;
  renderer.domElement.addEventListener('pointerdown', (ev) => { abajo = { x: ev.clientX, y: ev.clientY }; movido = false; });
  renderer.domElement.addEventListener('pointermove', (ev) => {
    mx = ev.clientX; my = ev.clientY;
    if (abajo && (Math.abs(ev.clientX - abajo.x) + Math.abs(ev.clientY - abajo.y)) > 4) movido = true;
  });
  renderer.domElement.addEventListener('pointerup', (ev) => {
    // Un arrastre NO es un clic. Sin esto, cada vez que girás la cámara terminás seleccionando algo
    // y la vista se va sola: es el modo más rápido de que la navegación se sienta fuera de control.
    if (!movido && abajo) {
      const h = sondear(ev.clientX, ev.clientY);
      if (h) { foco = h; elegir(h.sec); }
      else {
        // PLAN B declarado: si el pase de identidad no devuelve nada —un pixel de aire entre dos
        // hilos— se cae al raycast analitico, que agarra el haz. Es peor puntería, pero un clic que
        // no hace NADA se siente roto; y se nota en la ficha, que dice «haz» en vez del elemento.
        const i = bajoElCursor(ev.clientX, ev.clientY);
        if (i >= 0) { foco = null; elegir(i); }
      }
    }
    abajo = null;
  });

  addEventListener('keydown', (ev) => {
    const s = S[sel];
    if (ev.key === 'ArrowUp') { if (s.padre >= 0) elegir(s.padre); }
    else if (ev.key === 'ArrowDown') { if (s.hijos.length) elegir(s.hijos[0]); }
    else if (ev.key === 'ArrowLeft' || ev.key === 'ArrowRight') {
      // HERMANAS: el salto lateral. Es literalmente «moverse entre ramas», y con el mouse era
      // imposible porque dos hermanas pueden estar en lados opuestos de la escena.
      if (s.padre < 0) return;
      const hs = S[s.padre].hijos, k = hs.indexOf(sel);
      elegir(hs[(k + (ev.key === 'ArrowRight' ? 1 : hs.length - 1)) % hs.length]);
    } else if (ev.key === 'Home' || ev.key === '0') { verTodo(); }
    else if (ev.key === 'a' || ev.key === 'A') { aislar(aislado === sel ? -1 : sel); }
    else if (ev.key === 'Escape') { if (aislado >= 0) aislar(-1); else verTodo(); }
    else return;
    ev.preventDefault();
  });

  /* ── HUD: migas + ficha ─────────────────────────────────────────────────────────────────── */
  const panel = document.createElement('div');
  panel.className = 'hud';
  host.appendChild(panel);
  const tip = document.createElement('div');
  tip.className = 'tip'; host.appendChild(tip);

  const N_FMT = (x) => x.toLocaleString('es');
  const esc = (t) => String(t == null ? '' : t)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

  /** describir: que ES lo que agarraste. Cada clase contesta una pregunta distinta, asi que no
   *  puede haber un texto unico — decir «elemento 12.043» seria poder señalarlo y no saber que es. */
  function describir(h) {
    if (!h) return '<b>el haz entero</b>';
    if (h.tipo === 'neurona') {
      const e = FIB[h.i];
      return `<b>una neurona</b> · hilo ${N_FMT(e.fib)}<br><span class="d">eslabón ${e.orden + 1}${
        e.ultimo ? ' (terminal)' : ''} · ${e.nodos} nodos de Ranvier</span>`;
    }
    if (h.tipo === 'memoria') {
      const m = h.mem || {};
      const edad = Number(m.age_days);
      return `<b>${esc(m.topic || '(sin tema)')}</b><br><span class="d">una memoria${
        Number.isFinite(edad) ? ` · hace ${edad < 1 ? 'menos de un día' : Math.round(edad) + ' días'}` : ''
        }${m.importance != null ? ` · importancia ${m.importance}` : ''}</span>`;
    }
    return `<b>una terminal</b> · hilo ${N_FMT(RAM[h.i].fib)}<br><span class="d">rama fina, sin mielina</span>`;
  }
  function hud() {
    const s = S[sel];
    const c = camino(sel).reverse();
    const migas = c.map((k, i) => {
      const e = S[k].etiqueta || (k === 0 ? (cfg.titulo || 'memoria') : '·');
      return `<span class="miga${i === c.length - 1 ? ' hoy' : ''}" data-i="${k}">${e}</span>`;
    }).join('<span class="sep">›</span>');
    const porQue = s.criterio === 'tema' ? 'parte por TEMA'
      : s.criterio === 'tiempo' ? 'parte por TIEMPO'
      : s.criterio === 'reparto' ? `reparto de un corte por ${s.de || '—'}`
      : s.criterio === 'orden' ? 'mismo tema y misma edad: parte por orden'
      : 'no se parte más';
    panel.innerHTML = `
      <div class="migas">${migas}</div>
      <div class="ficha">
        <div class="fila elegido">${describir(foco)}</div>
        <div class="fila"><b>${N_FMT(s.carga)}</b> memorias por este haz</div>
        <div class="fila"><b>${N_FMT(s.fibras || 0)}</b> hilos · nivel <b>${s.nivel}</b></div>
        <div class="fila dim">${porQue}</div>
        ${s.hoja ? `<div class="fila dim">${s.memorias.length ? `${N_FMT(s.memorias.length)} botones dibujados` : 'sin botones'}${
          s.recortado || s.absorbidas > s.memorias.length
            ? ` · absorbe ${N_FMT(s.absorbidas)}` : ''}</div>` : ''}
        <div class="fila dim">${s.hijos.length} ${s.hijos.length === 1 ? 'sección hija' : 'secciones hijas'}</div>
      </div>
      <div class="acciones">
        <button class="bt" data-ac="todo">ver todo</button>
        <button class="bt${aislado === sel ? ' on' : ''}" data-ac="aislar">${
          aislado === sel ? 'mostrar el resto' : 'ver esta sola'}</button>
      </div>
      <div class="teclas">↑ padre · ↓ hija · ←→ hermanas · clic: volar · A: ver sola · 0: ver todo<br>
        arrastrar: girar · botón del medio o shift: mover · rueda: acercar al cursor</div>`;
    panel.querySelectorAll('.miga').forEach((el) =>
      el.addEventListener('click', () => elegir(Number(el.dataset.i))));
    panel.querySelectorAll('.bt').forEach((el) => el.addEventListener('click', () => {
      if (el.dataset.ac === 'todo') verTodo();
      else aislar(aislado === sel ? -1 : sel);
    }));
  }

  /* ── bucle ──────────────────────────────────────────────────────────────────────────────── */
  let ultimoHov = 0, cuadros = 0, tPrev = 0;
  function frame() {
    requestAnimationFrame(frame);
    cuadros++;
    // EL dt REAL, y se lo pasamos nosotros. La cámara suaviza por tiempo, no por cuadro: si el
    // cuadro se estira —y con miles de instancias se estira—, el movimiento tiene que seguir
    // durando lo mismo. Se topa en 50 ms para que volver de una pestaña en segundo plano no
    // teletransporte la vista.
    const t = performance.now();
    const dt = tPrev ? Math.min(0.05, (t - tPrev) / 1000) : 1 / 60;
    tPrev = t;
    cam.tick(dt);
    // El hover se resuelve a 20 Hz y no por cuadro: es un barrido sobre todas las secciones, y a
    // 60 Hz se lleva tiempo de cuadro para contestar lo mismo tres veces seguidas.
    const ahora = performance.now();
    // 90 ms y no 50: el hover ahora corre un PASE DE IDENTIDAD entero —cuatro mallas, 31.000
    // instancias— y aunque el costo de fragmento sea nulo (21x21 pixeles), el de vertice es el
    // mismo que el del pase visual. A 11 Hz el señalado se sigue sintiendo instantaneo y se libera
    // la mitad del presupuesto. El CLIC no pasa por este freno: ese va siempre.
    if (ahora - ultimoHov > 90 && !abajo) {
      ultimoHov = ahora;
      const h = sondear(mx, my);
      const i = h ? h.sec : -1;
      if (i !== hov) {
        hov = i; pintarSeleccion();
        renderer.domElement.style.cursor = i >= 0 ? 'pointer' : '';
      }
      if (h) {
        const s = S[i];
        tip.style.display = 'block';
        tip.style.left = (mx + 16) + 'px'; tip.style.top = (my + 14) + 'px';
        tip.innerHTML = describir(h) + `<br><span class="d">${s.etiqueta || '(sin nombre)'} · ${
          N_FMT(s.carga)} memorias</span>`;
      } else tip.style.display = 'none';
    }
    // LA RAMPA SIGUE A LA ORBITA. Los dos extremos salen de donde esta la camara y de cuanto mide
    // la escena, no de una densidad fija: asi hay contraste de profundidad mirando el cuerpo entero
    // Y metido adentro de una hoja, que es donde una densidad fija se apaga del todo.
    const dOrb = cam.est.dist;
    uAtm.uAtm.value.set(Math.max(1, dOrb - RESC * 0.55), dOrb + RESC * 1.45);
    uPulso.uT.value = ahora * 0.001;
    if (pulT0 > 0) avanzarPulso(ahora - pulT0);
    rot.tick(camera, cam.est.dist);
    composer.render();
  }
  addEventListener('resize', () => {
    camera.aspect = innerWidth / innerHeight; camera.updateProjectionMatrix();
    renderer.setSize(innerWidth, innerHeight);
    renderer.getDrawingBufferSize(_dbs); composer.setSize(_dbs.x, _dbs.y);
  });

  pintarSeleccion();
  frame();

  // UN PULSO AL ABRIR, UNA SOLA VEZ. No es un latido de fondo —eso sería inventar actividad, que
  // es justo lo que este panel no hace—: es la presentación del recorrido. Sale de un evento real
  // (abrir la vista), llega hasta la sección más lejana y no vuelve a salir hasta que elijas algo.
  const masLejos = S.reduce((m, x) => ((x.dist || 0) > (m.dist || 0) ? x : m), S[0]);
  setTimeout(() => { if (pulT0 < 0) lanzarPulso(masLejos.idx); }, 550);

  const vista = { scene, camera, cam, renderer, elegir, aislar, verTodo, S, camino, bajoElCursor,
                  sondear, get foco() { return foco; }, BOT_DE, BOT_MEM, ID_TOTAL,
                  BASE_NEURONA, BASE_MEM, BASE_RAM,
                  FAMSEC, get aislado() { return aislado; }, get encuadre0() { return ENCUADRE0; },
                  get cuadros() { return cuadros; },
                  RAM, PEN_DE, FIB, FIB_SEC, uPulso, uCont, contorno, lanzarPulso, avanzarPulso,
                  get sel() { return sel; },
                  POSMEM, MEM_SEC, SIN, sinInst,
                  conteos: { secciones: n, neuronas: nF, hilos: S[0].fibras, nodos: totNodos,
                             señalables: ID_TOTAL,
                             botones: totBot, ramitas: RAM.length,
                             sinapsis: SIN.length, sinapsisRecortadas: sinRecortadas } };

  /* ═══ PRUEBA EN LA PAGINA (#prueba) ═════════════════════════════════════════════════════
     Una captura demuestra que el dibujo se dibuja; no demuestra NADA sobre lo que el usuario
     reclamo, que es que moverse funcione. Esto ejercita la navegacion de verdad —bajar, saltar
     entre hermanas, subir, clic— y deja los numeros MEDIDOS en pantalla, para que la
     verificacion sea mirable y no una afirmacion mia.

     Cada linea es un invariante con su valor de fallo distinto del valor bueno: si la camara no
     se mueve, el desplazamiento da 0 y se ve 0. */
  if (location.hash === '#prueba') setTimeout(() => probar(vista), 900);
  // #seccion=N — aterriza en una seccion al cargar. Sirve para capturar SIEMPRE el mismo primer
  // plano entre una version y la siguiente: comparar dos renders con encuadres distintos no dice
  // nada sobre si el cambio mejoro algo.
  // #solo=N — aterriza en una seccion Y la aisla. Sirve para capturar el mismo primer plano entre
  // una version y la siguiente: comparar dos renders con encuadres distintos no dice nada.
  const ms = /^#solo=(\d+)$/.exec(location.hash || '');
  if (ms) setTimeout(() => {
    const i = Math.min(S.length - 1, Number(ms[1]));
    elegir(i); aislar(i);
    for (let k = 0; k < 140; k++) cam.tick(1 / 60);
  }, 400);
  const mf = /^#seccion=(\d+)$/.exec(location.hash || '');
  if (mf) setTimeout(() => {
    elegir(Math.min(S.length - 1, Number(mf[1])));
    // Y SE ASIENTA LA CAMARA A MANO. Bajo --virtual-time-budget el navegador casi no corre
    // cuadros, asi que el vuelo —que ahora dura por tiempo— no avanza y la captura sale SIEMPRE
    // con el encuadre ancho. Se veria como «el clic no encuadra» cuando en el navegador de verdad
    // encuadra perfecto. Se empuja el rig los pasos que daria en ~2 s a 60 Hz.
    for (let k = 0; k < 140; k++) cam.tick(1 / 60);
  }, 400);
  return vista;
}

function probar(v) {
  const S = v.S, out = [];
  const FIB = v.FIB;
  const pos = () => v.camera.position.clone();
  // ASENTAR SIN DEPENDER DE rAF. La primera version esperaba con setTimeout y leia la posicion,
  // y dio 0,0 en las tres pruebas de teclado — pero NO porque la navegacion estuviera rota: bajo
  // --virtual-time-budget el navegador dispara los timers de golpe y casi no corre cuadros, asi
  // que la camara no llegaba a interpolar. El valor de fallo del arnes era identico al valor de
  // fallo de la funcion, que es como un test se vuelve una afirmacion sobre nada.
  //
  // Se resuelve moviendo el rig A MANO la misma cantidad de pasos que daria en ~1 s a 60 Hz: se
  // prueba la logica —tecla -> objetivo -> convergencia— sin preguntarle la hora a nadie. Que rAF
  // ande de verdad se mide aparte, con el contador de cuadros.
  //
  // Y el dt VA EXPLÍCITO. La cámara nueva suaviza por tiempo: si el dt saliera del reloj, 120
  // tick() en un bucle cerrado darían dt≈0 y la cámara no se movería nada — o sea que el arnés
  // roto se leería idéntico a la navegación rota. Es el mismo agujero que ya nos comió una vez con
  // rAF, tapado antes de que muerda.
  const asentar = () => { for (let i = 0; i < 120; i++) v.cam.tick(1 / 60); return Promise.resolve(); };
  const tecla = (k) => dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true }));

  (async () => {
    // 1 · la raiz tiene hijas y el arbol es un ARBOL, no una lista
    const hojas = S.filter((s) => s.hoja).length;
    const maxNiv = S.reduce((m, s) => Math.max(m, s.nivel), 0);
    out.push(['el arbol tiene profundidad', maxNiv + ' niveles', maxNiv >= 4]);
    out.push(['la raiz se bifurca', S[0].hijos.length + ' hijas', S[0].hijos.length >= 2 && S[0].hijos.length <= 8]);
    out.push(['secciones hoja', hojas + ' de ' + S.length, hojas > 0 && hojas < S.length]);

    // 2 · NINGUNA MEMORIA PERDIDA: la carga de la raiz tiene que ser la suma de sus hijas
    const suma = S[0].hijos.reduce((a, i) => a + S[i].carga, 0);
    out.push(['ninguna memoria se pierde', suma + ' = ' + S[0].carga, suma === S[0].carga]);

    // 3 · el grosor es DATO: la seccion mas cargada tiene que llevar mas hilos y ser mas gruesa
    let gordo = S[0], cargado = S[0];
    for (const s of S) { if (s.Rhaz > gordo.Rhaz) gordo = s; if (s.carga > cargado.carga) cargado = s; }
    out.push(['el haz mas grueso es el mas cargado', gordo.idx === cargado.idx ? 'si' : 'no', gordo.idx === cargado.idx]);

    // 3b · LOS HILOS SE CONSERVAN. Es EL invariante del cambio pedido: si el numero de hilos del
    //      padre no fuera la suma de sus hijas, el grosor volveria a ser una estimacion y la rama
    //      volveria a ser inventada. Un axon no aparece ni desaparece en una bifurcacion.
    let fuga = 0, peor = null;
    for (const s of S) {
      if (!s.hijos.length) continue;
      const suma = s.hijos.reduce((a, i) => a + S[i].fibras, 0);
      if (suma !== s.fibras) { fuga++; peor = s; }
    }
    out.push(['ningun hilo aparece ni desaparece',
              fuga ? fuga + ' bifurcaciones con fuga (' + (peor.etiqueta || peor.idx) + ')' : 'las ' +
                S.filter((s) => s.hijos.length).length + ' bifurcaciones cierran', fuga === 0]);
    out.push(['el tronco lleva los hilos de todas las hojas',
              S[0].fibras + ' = ' + S.filter((s) => !s.hijos.length).reduce((a, s) => a + s.fibras, 0),
              S[0].fibras === S.filter((s) => !s.hijos.length).reduce((a, s) => a + s.fibras, 0)]);

    // 3c · YA NO ES UN ARBOL. Un arbol tiene suelo y copa: todas sus ramas de primer nivel salen
    //      hacia arriba, asi que la suma de sus direcciones apunta fuerte a +Y. En un ganglio las
    //      direcciones se reparten por la esfera y SE CANCELAN. Se mide el largo de la suma
    //      normalizada: 1 es «todas para el mismo lado» (un arbol), 0 es «reparto isotropo».
    const d1 = S.filter((s) => s.nivel === 1);
    const sx = d1.reduce((a, s) => a + s.dir[0], 0) / Math.max(1, d1.length);
    const sy = d1.reduce((a, s) => a + s.dir[1], 0) / Math.max(1, d1.length);
    const sz = d1.reduce((a, s) => a + s.dir[2], 0) / Math.max(1, d1.length);
    const sesgo = Math.hypot(sx, sy, sz);
    out.push(['el nucleo no tiene arriba', 'sesgo ' + sesgo.toFixed(2) + ' (un arbol da ~1)',
              sesgo < 0.35]);

    // 4 · NAVEGACION. Se mide el DESPLAZAMIENTO REAL de la camara, no que la funcion no tire error.
    const c0 = v.cuadros;
    v.elegir(0); await asentar();
    const p0 = pos();
    tecla('ArrowDown'); await asentar();
    const p1 = pos();
    const dBajar = p0.distanceTo(p1);
    out.push(['bajar a la hija mueve la camara', dBajar.toFixed(1) + ' u', dBajar > 5]);

    tecla('ArrowRight'); await asentar();
    const p2 = pos();
    const dHermana = p1.distanceTo(p2);
    out.push(['saltar a la hermana mueve la camara', dHermana.toFixed(1) + ' u', dHermana > 3]);

    tecla('ArrowUp'); await asentar();
    const p3 = pos();
    out.push(['subir al padre vuelve al punto de partida', p0.distanceTo(p3).toFixed(1) + ' u', p0.distanceTo(p3) < dBajar * 0.35]);

    // Un click de verdad, con el raycast contra la curva: es el camino que mas se usa.
    const antes = pos();
    const objetivo = v.S.reduce((m, x) => (x.nivel === 2 && x.carga > (m ? m.carga : 0) ? x : m), null);
    if (objetivo) { v.elegir(objetivo.idx); await asentar(); }
    out.push(['volar a una seccion de nivel 2', antes.distanceTo(pos()).toFixed(1) + ' u', antes.distanceTo(pos()) > 5]);

    // Y aparte: rAF corre de verdad en este navegador, o no. Se dice el numero, no se supone.
    const cuadros = v.cuadros - c0;
    out.push(['cuadros de rAF en este navegador', cuadros + (cuadros ? '' : '  (headless: no corre)'),
              'dato']);

    // 5 · la camara NUNCA se da vuelta: es la queja de fondo sobre trackball
    out.push(['la vertical se mantiene', v.camera.up.y.toFixed(2), v.camera.up.y > 0.99]);

    // 6 · EL PENACHO cuelga SOLO de secciones hoja. Si brotara de una seccion con hijas, la ramita
    //     fina se cruzaria con la rama gruesa que sale del mismo punto y se veria como basura.
    const malPuestas = v.RAM.reduce((a, x, i) => a + (S[v.PEN_DE[i]].hijos.length ? 1 : 0), 0);
    out.push(['el penacho sale solo de las hojas', malPuestas + ' mal puestas', malPuestas === 0]);
    out.push(['hay penacho', v.RAM.length + ' ramitas', v.RAM.length > S.length * 4]);

    // 7 · EL IMPULSO AVANZA. Se mide el uniform real del shader, no que la funcion no tire error:
    //     un frente que no se mueve es exactamente lo que se ve cuando la animacion esta rota.
    const dest = S.reduce((m, x) => (x.dist > (m ? m.dist : 0) ? x : m), null);
    out.push(['el impulso tiene a donde llegar', Math.round(dest.dist) + ' u de recorrido', dest.dist > 50]);

    // EL FRENTE SE APAGA SOLO. Éste es EL invariante del impulso, y la primera versión que escribí
    // era una TAUTOLOGÍA —`f0 < 0 || f0 >= 0`, verdadera siempre— que además mostraba «no» como
    // valor y pasaba igual. Es exactamente el patrón que este proyecto ya arrastra: el valor de
    // fallo idéntico al tranquilizador.
    //
    // Lo que hay que probar es que la luz NO se queda prendida para siempre: sin eso, el panel
    // dejaría de poder decir «acá no está pasando nada», que es la mitad de lo que dice. Se lanza
    // el pulso, se lo empuja más allá de su destino y se exige ver el apagado.
    v.lanzarPulso(dest.idx);
    v.uPulso.uFrente.value = 0;
    const antesDelFinal = v.uPulso.uFrente.value;
    // se corre el reloj del pulso hasta pasarse: el bucle no avanza bajo virtual-time
    v.avanzarPulso(dest.dist * 3 + 500);
    const despues = v.uPulso.uFrente.value;
    out.push(['el frente viaja', antesDelFinal >= 0 ? 'arranco en 0' : 'NO ARRANCO', antesDelFinal >= 0]);
    out.push(['y se APAGA solo al llegar', despues < 0 ? 'apagado (-1)' : 'SIGUE EN ' + despues.toFixed(0),
              despues < 0]);

    // 8 · LA CLASIFICACION, que es la correccion pedida: los actores y sus cuentas
    const porRacimo = new Map();
    for (const x of S) if (x.nivel === 1) porRacimo.set(x.etiqueta, x.carga);
    const suma1 = [...porRacimo.values()].reduce((a, b) => a + b, 0);
    out.push(['actores de primer nivel', [...porRacimo.keys()].join(', '), porRacimo.size >= 2]);
    out.push(['los actores suman el total', suma1 + ' = ' + S[0].carga, suma1 === S[0].carga]);
    const mus = [...porRacimo.entries()].find(([k]) => /musubi/i.test(k));
    out.push(['Musubi es un actor, no un balde gris', mus ? mus[1] + ' notas' : 'NO ESTA', !!mus]);

    // 9 · CADA MEMORIA TIENE UN PUNTO. Sin esto una relacion no se puede dibujar sin inventarle
    //     una posicion a alguno de sus dos extremos, que es la manera silenciosa de mentir.
    out.push(['toda memoria tiene su boton', v.POSMEM.size + ' de ' + S[0].carga,
              v.POSMEM.size === S[0].carga]);

    // 10 · LAS SINAPSIS UNEN PUNTOS REALES, no posiciones inventadas, y ninguna quedo con un
    //      extremo en el origen — que es donde termina un [0,0,0] por defecto y se ve como un
    //      arco saliendo del centro de la nada.
    const enOrigen = v.SIN.reduce((a, y) =>
      a + ((Math.hypot(...y.A) < 0.001 || Math.hypot(...y.B) < 0.001) ? 1 : 0), 0);
    out.push(['sinapsis dibujadas', v.SIN.length + '', v.SIN.length > 0]);
    out.push(['ninguna sinapsis nace en el origen', enOrigen + '', enOrigen === 0]);
    // Y NINGUNA CON NaN: una instancia con NaN en su matriz desaparece sin error ni warning, y se
    // ve exactamente igual que si hubieramos decidido no dibujar relaciones. Ya paso.
    let nan = 0;
    if (v.sinInst) { const m = v.sinInst.instanceMatrix.array;
      for (let k = 0; k < m.length; k++) if (!Number.isFinite(m[k])) nan++; }
    out.push(['ninguna matriz con NaN', nan + '', nan === 0]);

    // 12 · SELECCIONAR CUALQUIERA, NO UN MONTON. Es el reclamo principal, asi que se mide y no se
    //      afirma. Cuatro cosas, y ninguna de ellas puede pasar por accidente.
    const dibujadas = S.length + v.FIB.length + v.RAM.length + v.POSMEM.size;
    const elegibles = v.ID_TOTAL;
    out.push(['elementos que se pueden señalar', elegibles + ' de ' + dibujadas + ' dibujados',
              elegibles > S.length * 20]);

    // (a) TODO id decodifica a algo real. Un hueco en el espacio de ids se ve como un clic que
    //     agarra la cosa equivocada, no como un error.
    let rotos = 0;
    for (let id = 1; id <= v.ID_TOTAL; id++) {
      let ok = false;
      if (id < v.BASE_MEM) ok = (id - v.BASE_NEURONA) < v.FIB.length;
      else if (id < v.BASE_RAM) ok = v.BOT_MEM[id - v.BASE_MEM] != null;
      else ok = (id - v.BASE_RAM) < v.RAM.length;
      if (!ok) rotos++;
    }
    out.push(['ningun id apunta a la nada', rotos + ' rotos de ' + v.ID_TOTAL, rotos === 0]);

    // (b) SEÑALAR EL CIELO NO DEVUELVE NADA. El valor de fallo tiene que ser distinto del bueno:
    //     si el fondo tuviera id, un clic al vacio se leeria como un acierto.
    const cielo = v.sondear(4, 4);
    out.push(['señalar el vacio no inventa nada', cielo ? 'DEVOLVIO ' + cielo.tipo : 'nada', !cielo]);

    // (c) LA PUNTERIA: se proyectan botones REALES a pantalla y se pregunta que hay ahi. Lo que
    //     vuelve tiene que ser ESE boton. No da 100 % y no tiene por que: un boton tapado por un
    //     hilo de adelante devuelve el hilo, que es la respuesta correcta. Lo que el numero mide es
    //     que la sonda acierte a la cosa y no al haz.
    const _p3 = new THREE.Vector3();
    let probados = 0, exactos = 0, mismaSeccion = 0;
    const objetivo2 = v.S.reduce((m, x) => (x.hoja && x.memorias.length > 6 ? x : m), null);
    if (objetivo2) {
      v.elegir(objetivo2.idx); await asentar();
      v.camera.updateMatrixWorld(true);
      for (let k = 0; k < objetivo2.memorias.length && probados < 60; k++) {
        const pm = v.POSMEM.get(objetivo2.memorias[k].id);
        if (!pm) continue;
        _p3.set(pm[0], pm[1], pm[2]).project(v.camera);
        if (_p3.z >= 1) continue;
        const sx = Math.round((_p3.x * 0.5 + 0.5) * innerWidth);
        const sy = Math.round((-_p3.y * 0.5 + 0.5) * innerHeight);
        if (sx < 2 || sy < 2 || sx > innerWidth - 2 || sy > innerHeight - 2) continue;
        probados++;
        const h = v.sondear(sx, sy);
        if (!h) continue;
        if (h.tipo === 'memoria' && h.mem && h.mem.id === objetivo2.memorias[k].id) exactos++;
        if (h.sec === objetivo2.idx) mismaSeccion++;
      }
    }
    out.push(['señalar un boton devuelve ESE boton',
              probados ? exactos + ' de ' + probados : 'no se pudo probar', probados > 0 && exactos > probados * 0.25]);
    out.push(['y nunca cae en otra rama',
              probados ? mismaSeccion + ' de ' + probados : '—', probados > 0 && mismaSeccion > probados * 0.7]);

    // 13 · AISLAR apaga el resto y deja el subarbol entero encendido
    v.aislar(objetivo2 ? objetivo2.idx : 1);
    const enc = Array.from(v.FAMSEC).filter((x) => x >= 0.99).length;
    const apag = Array.from(v.FAMSEC).filter((x) => x < 0.2).length;
    out.push(['aislar apaga el resto', enc + ' encendidas · ' + apag + ' apagadas',
              enc >= 1 && apag > S.length * 0.5]);
    v.aislar(-1);
    out.push(['y ver todo las vuelve a prender',
              Array.from(v.FAMSEC).filter((x) => x >= 0.99).length + ' de ' + S.length,
              Array.from(v.FAMSEC).every((x) => x >= 0.99)]);

    // 11 · EL TRONCO no vuelve a ser un tubo largo y vacio: mas corto que las ramas de nivel 1
    const l1 = S.filter((x) => x.nivel === 1).reduce((a, x) => a + x.largo, 0)
             / Math.max(1, S.filter((x) => x.nivel === 1).length);
    out.push(['el tronco es mas corto que un actor',
              S[0].largo.toFixed(0) + ' vs ' + l1.toFixed(0), S[0].largo < l1]);

    const caja = document.createElement('div');
    caja.className = 'prueba';
    caja.innerHTML = '<div class="t">prueba de navegacion — medido en esta pagina</div>'
      + out.map(([q, val, ok]) => '<div class="l ' + (ok === 'dato' ? 'dato' : ok ? 'ok' : 'mal')
        + '"><i>' + (ok === 'dato' ? '·' : ok ? '✓' : '✗') + '</i><span>' + q
        + '</span><b>' + val + '</b></div>').join('')
      + '<div class="r">' + out.filter((x) => x[2] === true).length + ' / '
      + out.filter((x) => x[2] !== 'dato').length + ' invariantes</div>';
    document.body.appendChild(caja);
  })();
}
