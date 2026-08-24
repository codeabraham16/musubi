// Brain-dashboard WebGL (three.js). Se bundlea con esbuild → ../dashboard.bundle.js (commiteado),
// que el binario Go sirve embebido (go:embed) sobre loopback. Reemplaza el render Canvas 2D anterior
// por una escena three (neuronas icosaedro+fresnel, aristas-tubo con PULSO CONTINUO), REUSANDO la
// lógica de datos y de ACTIVIDAD REAL del dashboard previo (diff de snapshots): reposo azul tenue +
// escribir(verde)/recordar(cyan)/relacionar(ámbar) derivados de id-nuevo / heat↑,recency↓ / sinapsis-nueva.
import * as THREE from 'three';
import { TrackballControls } from 'three/examples/jsm/controls/TrackballControls.js';
import { EffectComposer } from 'three/examples/jsm/postprocessing/EffectComposer.js';
import { RenderPass } from 'three/examples/jsm/postprocessing/RenderPass.js';
import { UnrealBloomPass } from 'three/examples/jsm/postprocessing/UnrealBloomPass.js';
import { SMAAPass } from 'three/examples/jsm/postprocessing/SMAAPass.js';
import { extraerPersonas, neuronaDeEvento, clasificarEvento,
         fusionarActores, mapaDeEncendido,
         grupoDeNeurona, ordenarRacimos, GRUPO_LIBRO, GRUPO_SIN_ATRIBUIR } from './personas.mjs';
import { construirRacimo } from './arbol-memoria.mjs';
import { crearImpulsos, AMBAR_FRENTE } from './impulsos.mjs';
import { iterParaCambio, settleStart, settleTick, settlePendiente } from './layout.mjs';

/* ---------- paletas ---------- */
const DOMPAL=['#2dd4bf','#a78bfa','#fbbf24','#4ade80','#38bdf8','#f472b6','#fb923c','#f87171','#a3e635','#22d3ee','#e879f9','#facc15'];
const RELCOL={ conflicts_with:'#f87171', supersedes:'#a78bfa', scoped:'#38bdf8', related:'#2dd4bf', compatible:'#4ade80', not_conflict:'#64748b' };
// color por TIPO DE ACTIVIDAD (ancla a señales reales): 0 reposo · 1 escribir · 2 recordar · 3 relacionar
const AK=['#7f9cc9','#43e08b','#31c9ff','#f5c451'];
const REPOSO=AK[0];
// LENTE CÓDIGO: color de arista por TIPO (llama / importa / contiene). En reposo cada tubo toma su color.
const EDGEKIND={ CALLS:'#38bdf8', IMPORTS:'#a78bfa', CONTAINS:'#5b6b86' };
// DESPACHOS: una terminal le escribe a otra («PRINCIPAL → PLANIFICADOR»). Azul cuando el par
// pertenece a la misma persona, cian cuando CRUZA personas — que es el caso interesante, porque
// es trabajo que salió de una cabeza y entró en otra.
const COLOR_DESPACHO='#8a99ff', COLOR_CRUCE='#2dd4bf';
const edgeBase=s=>(s&&s.kind&&EDGEKIND[s.kind])||REPOSO;
let DOMCOL=new Map(), DOMAINS=[];
const domColor=d=>DOMCOL.get(d)||'#64748b';
// Los racimos que NO son personas van en gris, fuera de la paleta. La distinción tiene que
// entrar por el ojo antes que por la leyenda: si el libro mayor —el 46,3 % de la memoria— tuviera
// un color de la paleta, sería el racimo más vistoso de la pantalla y se leería como una persona.
// Grises CLAROS, no oscuros. El primer intento usó #5b6472/#3f4652 y el resultado medido en
// pantalla fue que los dos racimos —el 64 % de la memoria— quedaban casi invisibles sobre el
// fondo negro, sobre todo el libro mayor, que es viejo y por eso el factor de recencia lo apaga
// todavía más. «No compite por atención» no puede significar «no se ve»: la mitad del acervo
// desapareciendo es una afirmación falsa sobre cuánto hay.
const GRIS_LIBRO='#98a2b3', GRIS_SIN='#78839a';
// quienEscribio: lo que el tooltip dice del AUTOR. Los tres casos se nombran distinto porque son
// distintos: una persona, el registro del propio motor, y una nota que no se pudo atribuir. Decir
// «sin atribuir» a un `git-commit` sería tratar como hueco lo que es una categoría.
function quienEscribio(n){
  if(n._grupoTipo==='libro') return 'libro mayor · lo escribe el motor';
  if(n._grupoTipo==='sin') return 'sin autor · anterior a la columna';
  return 'de '+(n._grupo||'?');
}
function colorDeRacimo(clave,i){
  if(clave===GRUPO_LIBRO) return GRIS_LIBRO;
  if(clave===GRUPO_SIN_ATRIBUIR) return GRIS_SIN;
  return DOMPAL[i%DOMPAL.length];
}
function hash01(str){ let h=2166136261; for(let i=0;i<str.length;i++){ h^=str.charCodeAt(i); h=Math.imul(h,16777619); } return (h>>>0)%1000/1000; }

/* ---------- escala del volumen (proporcional a la población) ---------- */
let baseR=118, growth=1, rx=118, ry=94, rz=87;
function applyScale(){ rx=baseR*growth; ry=rx; rz=rx; }   // esférico (redondo, NO ovalado)

// ---------- puente con la fisica (layout.mjs) ----------
// La lente que se esta asentando vive ACA y no en layout.mjs: la fisica no sabe de lentes, y no
// tiene por que. Lo unico que devuelve es 'termine'; guardar donde quedo es decision del panel.
let asentandoLente='memory';
const radios=()=>({rx,ry,rz});

function arrancarAsentado(its, cual){
  settleStart(NEURONS, SYN, radios, its);
  // settleStart sale sin hacer nada si el grafo esta vacio; en ese caso no hay asentado que
  // marcar y ASENTADO tiene que quedar como estaba.
  if(settlePendiente()>0){ asentandoLente=cual; ASENTADO[cual]=false; }
}

// asentar corre un tramo y, si termino, fotografia las posiciones buenas.
function asentar(ms){
  const paso=settleTick(ms);
  if(!paso.trabajo) return false;
  if(paso.termino){   // ya asentado: esta es la posicion que vale la pena recordar
    POS[asentandoLente]=new Map(NEURONS.map(n=>[n.id,{x:n.x,y:n.y,z:n.z,ph:n.ph}]));
    ASENTADO[asentandoLente]=true;
  }
  return true;
}
const inEllipsoid=(x,y,z)=>(x*x)/(rx*rx)+(y*y)/(ry*ry)+(z*z)/(rz*rz)<=1;
function randInBrain(){ for(let i=0;i<80;i++){ const x=(Math.random()*2-1)*rx,y=(Math.random()*2-1)*ry,z=(Math.random()*2-1)*rz; if(inEllipsoid(x,y,z)) return {x,y,z}; } return {x:0,y:0,z:0}; }

/* ---------- estado del grafo (fuente de verdad = snapshot) ---------- */
let NEURONS=[], SYN=[];
// POS guarda las posiciones YA ASENTADAS para sembrar el proximo build y no re-asentar.
// Va SEPARADO POR LENTE: con un solo mapa, al pasar a codigo las claves guardadas eran las de
// memoria, TODOS los nodos salian `_new`, y el layout se recalculaba entero en cada toggle.
// Medido: settle(55) sobre 6113 nodos = 3.037 ms de hilo principal bloqueado, cada vez.
const POS={memory:new Map(), code:new Map()};
// ASENTADO: si cambias de lente A MITAD del asentado, POS ya quedo sembrado con posiciones a
// medio ordenar y la proxima vez `changed` daria false -> el grafo se quedaria desordenado
// para siempre. Esta bandera es la que dice si esa lente termino de asentarse de verdad.
const ASENTADO={memory:false, code:false};
let prevStats=new Map(), prevSyn=new Set(), thinking=0, actInc=new Float32Array(0), bestInc=new Float32Array(0), akSrc=new Int8Array(0);
let motion=true, needsRebuild=false;
let lens='memory';   // lente activa (memory|code|personas); los datos viven en GRAPH/PULSE
// La vista de PERSONAS dibuja en su propio canvas 2D. Se crea una sola vez; se apaga sola
// cuando la lente no es la suya, asi que no gasta un frame mientras nadie la mira.
// Lo último que se extrajo, para que el HUD pinte la MISMA verdad que el lienzo. Vive acá y no
// dentro de renderLens porque renderHUD corre en cada poll (cada 5 s) y necesita leerlo.
let PERSONAS = null;
// CENSO es la respuesta de /api/actores tal cual: trae `estado` ademas del censo. ENCENDIDO es
// el mapa principal→neurona que sale de fundirlo con las terminales. Los dos arrancan en null y
// eso NO es "cero actores": es "todavia no se pregunto", que la vista distingue.
let CENSO = null, ENCENDIDO = null, _bajandoCenso = null;
// Eventos del spool local, que llegan SIN credencial. Se cuentan aparte de los que sí traen un
// principal y aun así no encuentran neurona: son dos problemas y tienen dos arreglos distintos.
let SIN_CREDENCIAL = 0;

// buildGraph: PORTADO del dashboard anterior. Detecta ACTIVIDAD REAL diffeando el snapshot y la tipa
// (escribir/recordar/relacionar); corre el force-sim si cambió la topología. Marca needsRebuild para
// que la escena three recree las mallas cuando cambian nodos/aristas.
function buildGraph(brain){
  const prev=POS.memory, ns0=brain.neurons||[];
  const N0=240; growth=Math.max(0.85, Math.min(2.2, Math.cbrt((ns0.length||1)/N0))); applyScale();
  // EL RACIMO ES LA PERSONA, no el dominio. Es el cambio de sujeto de la escena principal:
  // antes agrupaba por QUÉ trata una memoria, ahora por QUIÉN la escribió. La maquinaria es la
  // misma —anclas en una esfera de Fibonacci y el layout tira de cada nodo hacia la suya—, y por
  // eso hereda los 60 FPS que el canvas 2D de la lente aparte no podía dar (medido: 60,3 contra
  // 13,8, con 11.012 strokes por cuadro).
  //
  // Se cachea el grupo EN la neurona (`_grupo`) porque se consulta dos veces por nodo acá y una
  // más en el tooltip; recalcularlo es barato pero repetirlo escondería que es el mismo criterio.
  ns0.forEach(n=>{ const g=grupoDeNeurona(n); n._grupo=g.clave; n._grupoTipo=g.tipo; });
  const counts={}; ns0.forEach(n=>counts[n._grupo]=(counts[n._grupo]||0)+1);
  // ordenarRacimos deja al LIBRO MAYOR y a lo SIN ATRIBUIR al final aunque pesen más: no son
  // personas, y ordenarlos por tamaño entre ellas afirmaría que lo son. Medido, el libro mayor
  // es el 46,3 % — el racimo más grande de todos.
  const doms=ordenarRacimos(Object.keys(counts), counts);
  DOMCOL=new Map(); DOMAINS=[]; const dIdx=new Map();
  // ANCLA a 0,62 del radio y VOLUMEN PROPIO de 0,32, escalado por la raíz cúbica del tamaño para
  // que todos los racimos queden con la misma DENSIDAD (si no, el libro mayor —1.028 nodos— sería
  // una masa apretada al lado de davantis, y el tamaño se leería como intensidad).
  // Los dos números salen de un banco con los parámetros reales, no de probar a ojo: con ancla
  // 0,52 y sin volumen propio la holgura entre racimos daba 0,38 (se pisaban); así da 1,26. Y
  // 0,62+0,39 = 1,01 del radio, o sea el conjunto entra justo en el cuadro que encuadra la cámara.
  const FRAC_ANCLA=0.62, FRAC_RADIO=0.32, medioRacimo=Math.max(1,ns0.length/Math.max(doms.length,1));
  doms.forEach((d,i)=>{ const col=colorDeRacimo(d,i); DOMCOL.set(d,col); dIdx.set(d,i);
    const k=i+0.5, phi=Math.acos(1-2*k/Math.max(doms.length,1)), th=Math.PI*(1+Math.sqrt(5))*k;
    DOMAINS.push({name:d,color:col,count:counts[d],
      ax:Math.cos(th)*Math.sin(phi)*rx*FRAC_ANCLA, ay:Math.cos(phi)*ry*FRAC_ANCLA, az:Math.sin(th)*Math.sin(phi)*rz*FRAC_ANCLA,
      ar:rx*FRAC_RADIO*Math.cbrt(counts[d]/medioRacimo)}); });

  // EL ARBOL SE ARMA ACA, antes de que las neuronas tengan posicion, porque la posicion SALE de el.
  // Detras de la firma: armarlo cuesta 85 ms sobre el central y no cambia mientras no cambie la
  // memoria. Y va DESPUES de DOMAINS porque cada arbol crece dentro del racimo de su persona: sin
  // el ancla y el radio del racimo no hay donde plantarlo.
  const _fp=firmaGrafo(ns0);
  if(_fp!==_firmaPersonas){
    _firmaPersonas=_fp;
    refrescarPersonas(brain);
    const b=construirBosqueMemoria(ns0);
    BOSQUE=b.neuronas; POSMEM=b.posiciones;
  }

  // los ids previos DE ESTA LENTE, no los de NEURONS (que todavia tiene el grafo de la otra).
  const prevIds=new Set(prev.keys());
  NEURONS=ns0.map(n=>{ const p=prev.get(n.id);
    const anc=dIdx.has(n._grupo)?DOMAINS[dIdx.get(n._grupo)]:null;
    // LA POSICIÓN SALE DEL ÁRBOL, no de un sorteo ni de la física. `randInBrain()` y el asentado
    // con fuerzas quedaron para la lente CÓDIGO, donde el sujeto son símbolos y no hay jerarquía
    // que ramificar. El respaldo —el centro del racimo— es para una memoria que llegue por un
    // delta antes de que el árbol se rehaga: cae en el centro y el próximo armado la ubica.
    const t=POSMEM.get(n.id);
    const base=t||(anc?{x:anc.ax,y:anc.ay,z:anc.az}:{x:0,y:0,z:0});
    // El punto se achica: dejó de flotar solo y pasó a ser el BOTÓN TERMINAL de una rama de 0,17
    // de grosor. Con el tamaño de antes (hasta 4,4) el botón se comía el árbol entero.
    const r=Math.max(0.40, Math.min(1.9, 0.40+Math.sqrt(Math.max(n.importance,0))*0.34+Math.log(1+(n.heat||0))*0.20));
    const rec=Math.max(0.10, Math.min(1, 1-(n.recency_days||0)/45));
    return {...n, x:base.x,y:base.y,z:base.z, vx:0,vy:0,vz:0, r, rec, col:domColor(n._grupo),
      gx:anc?anc.ax:0, gy:anc?anc.ay:0, gz:anc?anc.az:0, gr:anc?anc.ar:0,
      ph:(p&&p.ph!=null)?p.ph:Math.random()*6.283, phx:Math.random()*6.283, phz:Math.random()*6.283,
      di:dIdx.has(n._grupo)?dIdx.get(n._grupo):-1, act:0, ak:0, adj:[], _new:!p}; });
  const idx=new Map(NEURONS.map((n,i)=>[n.id,i]));
  SYN=(brain.synapses||[]).filter(s=>idx.has(s.source)&&idx.has(s.target))
    .map(s=>{ const hs=hash01(s.source+'>'+s.target); return {...s, a:idx.get(s.source), b:idx.get(s.target), off:hs}; });
  NEURONS.forEach(n=>{n.adj=[]; n.deg=0;});
  for(const s of SYN){ const w=0.35+(s.confidence||0)*0.55; NEURONS[s.a].adj.push({j:s.b,w}); NEURONS[s.b].adj.push({j:s.a,w}); NEURONS[s.a].deg++; NEURONS[s.b].deg++; }
  actInc=new Float32Array(NEURONS.length); bestInc=new Float32Array(NEURONS.length); akSrc=new Int8Array(NEURONS.length);

  // ACTIVIDAD REAL entre polls (idéntico al dashboard previo, ahora TIPADA por color):
  const firstLoad=prevStats.size===0;
  if(firstLoad){ thinking=0; }   // primera carga = reposo puro: NO fabricar pulso (solo se enciende lo que cambia de verdad entre polls)
  else { let hits=0;
    for(const n of NEURONS){ const ps=prevStats.get(n.id);
      if(!ps){ if(n.age_days!=null && n.age_days<0.02){ n.act=1; n.ak=1; hits++; } }   // id nuevo Y joven (<~30min) → escribir; si solo ENTRÓ al top-300 (memoria vieja) no es actividad
      else if(n.heat>ps.heat || (ps.rec!=null && n.recency_days!=null && n.recency_days<ps.rec-0.0004)){ n.act=1; n.ak=2; hits++; } } // accedida → recordar
    for(const s of SYN){ if(!prevSyn.has(s.source+'|'+s.target) && prevStats.has(s.source) && prevStats.has(s.target)){ NEURONS[s.a].act=1; NEURONS[s.a].ak=3; NEURONS[s.b].act=1; NEURONS[s.b].ak=3; hits++; } } // sinapsis nueva ENTRE neuronas ya visibles → relacionar (no si una recién entró al top-300)
    if(hits>0){ thinking=Math.min(1, thinking+0.5+hits*0.2); actViva=true; }   // despierta el bucle aunque este en pausa
  }
  prevStats=new Map(NEURONS.map(n=>[n.id,{heat:n.heat, rec:n.recency_days}]));
  prevSyn=new Set(SYN.map(s=>s.source+'|'+s.target));

  const nuevos=NEURONS.reduce((k,n)=>k+(n._new?1:0),0);
  const changed=nuevos>0||NEURONS.length!==prevIds.size;
  // El asentado ya NO congela: se reparte en trozos de pocos ms por frame (ver settleTick).
  // POS se siembra igual ahora —asi la proxima vez arranca de donde quedo— y se re-siembra al
  // terminar de asentar, que es cuando las posiciones son las buenas.
  // NO HAY ASENTADO. La posición es determinista y ya está: no hay nada que relajar. Se va con eso
  // el tirón del layout y también la deriva —dos cargas del mismo grafo dan el mismo dibujo—.
  //
  // Lo que se PIERDE y hay que decirlo: la física tiraba de lo relacionado y lo dejaba cerca. Ahora
  // la posición la manda el tema o el tiempo, así que las sinapsis cruzan más espacio. Es el precio
  // de que la rama signifique algo.
  POS.memory=new Map(NEURONS.map(n=>[n.id,{x:n.x,y:n.y,z:n.z,ph:n.ph}]));
  ASENTADO.memory=true;
  if(changed) needsRebuild=true;
}

// buildCodeGraph: la LENTE CÓDIGO (Track 20). Mapea el grafo de código a los MISMOS campos que
// buildGraph, así el resto del pipeline (settle/spread/rebuildMeshes/animate) lo dibuja sin tocar
// nada: COLOR por módulo/paquete (reusa DOMPAL/DOMCOL), TAMAÑO por centralidad (grado), aristas
// tipadas por kind (CALLS/IMPORTS/CONTAINS). Reposo puro — sin sistema de actividad de memoria; el
// "porqué" (EXPLICADO_POR) se resuelve on-hover contra /api/explained.
function buildCodeGraph(code){
  const prev=POS.code, ns0=code.nodes||[];
  const N0=240; growth=Math.max(0.85, Math.min(2.2, Math.cbrt((ns0.length||1)/N0))); applyScale();
  const counts={}; ns0.forEach(n=>counts[n.module]=(counts[n.module]||0)+1);
  const mods=Object.keys(counts).sort((a,b)=>counts[b]-counts[a]||a.localeCompare(b));
  DOMCOL=new Map(); DOMAINS=[]; const dIdx=new Map();
  mods.forEach((d,i)=>{ const col=DOMPAL[i%DOMPAL.length]; DOMCOL.set(d,col); dIdx.set(d,i);
    DOMAINS.push({name:d,color:col,count:counts[d]}); });

  // los ids previos DE ESTA LENTE, no los de NEURONS (que todavia tiene el grafo de la otra).
  const prevIds=new Set(prev.keys());
  NEURONS=ns0.map(n=>{ const p=prev.get(n.key); const base=p?{x:p.x,y:p.y,z:p.z}:randInBrain();
    const r=Math.max(0.9, Math.min(6.0, 0.9+Math.sqrt(Math.max(n.degree||0,0))*0.62)); // tamaño = centralidad
    return { id:n.key, topic:n.name||n.key, domain:n.module, mem_type:n.kind, heat:n.degree||0,
      path:n.path, line:n.line, gist:(n.path?n.path+(n.line?':'+n.line:''):(n.kind||'')), _code:true, _exp:undefined,
      x:base.x,y:base.y,z:base.z, vx:0,vy:0,vz:0, r, rec:1, col:domColor(n.module),
      ph:(p&&p.ph!=null)?p.ph:Math.random()*6.283, phx:Math.random()*6.283, phz:Math.random()*6.283,
      di:dIdx.has(n.module)?dIdx.get(n.module):-1, act:0, ak:0, adj:[], _new:!p }; });
  const idx=new Map(NEURONS.map((n,i)=>[n.id,i]));
  SYN=(code.edges||[]).filter(s=>idx.has(s.source)&&idx.has(s.target))
    .map(s=>{ const hs=hash01(s.source+'>'+s.target); return {...s, a:idx.get(s.source), b:idx.get(s.target), off:hs}; });
  NEURONS.forEach(n=>{n.adj=[]; n.deg=0;});
  for(const s of SYN){ const w=0.35+(s.confidence||0)*0.55; NEURONS[s.a].adj.push({j:s.b,w}); NEURONS[s.b].adj.push({j:s.a,w}); NEURONS[s.a].deg++; NEURONS[s.b].deg++; }
  actInc=new Float32Array(NEURONS.length); bestInc=new Float32Array(NEURONS.length); akSrc=new Int8Array(NEURONS.length);
  prevStats=new Map(); prevSyn=new Set(); thinking=0;   // código = reposo puro (sin diff de actividad)

  const nuevos=NEURONS.reduce((k,n)=>k+(n._new?1:0),0);
  const changed=nuevos>0||NEURONS.length!==prevIds.size;
  POS.code=new Map(NEURONS.map(n=>[n.id,{x:n.x,y:n.y,z:n.z,ph:n.ph}]));
  const its=iterParaCambio(NEURONS.length, nuevos, ASENTADO.code);
  if(its>0){ arrancarAsentado(its,'code'); needsRebuild=true; }
  else if(changed) needsRebuild=true;
}

// refrescarBase: rebuildMeshes fotografia n.x/y/z en BX/BY/BZ. Mientras el layout se asienta esas
// posiciones cambian, asi que hay que re-fotografiarlas o el dibujo se queda en el primer frame.
//
// Y con `k<1` lo DIBUJADO persigue a la fisica en vez de saltar con ella. Importa porque el layout
// es violento de verdad al principio: arranca con los nodos al azar en todo el volumen y el clamp
// permite 40 unidades por iteracion sobre un radio de 118 — un tercio del cerebro de un salto.
// Antes eso no se veia porque pasaba entero adentro del congelamiento; ahora que se reparte en
// frames, sin suavizar se ve como si el grafo se volviera loco. La FISICA no cambia: sigue
// corriendo exacta, sobre n.x/n.y/n.z. Lo unico que se amortigua es el punto donde se dibuja.
//
// Al terminar de asentar se copia exacto (k=1). Ese salto final es invisible: con damping 0.86 por
// iteracion las velocidades ya son ~0, asi que lo dibujado y la fisica estan practicamente encima.
function refrescarBase(k){ if(!BX) return; const m=Math.min(N, NEURONS.length);
  if(k>=1){ for(let i=0;i<m;i++){ const n=NEURONS[i]; BX[i]=n.x; BY[i]=n.y; BZ[i]=n.z; } return; }
  for(let i=0;i<m;i++){ const n=NEURONS[i];
    BX[i]+=(n.x-BX[i])*k; BY[i]+=(n.y-BY[i])*k; BZ[i]+=(n.z-BZ[i])*k; } }

// spread: propaga `act` por la adyacencia REAL de sinapsis (el vecino que se despierta adopta el ak
// del que lo encendió). Topología real; decay estético.
function spread(){ const n=NEURONS.length; if(!n) return; actInc.fill(0); bestInc.fill(0);
  for(let i=0;i<n;i++){ const ai=NEURONS[i].act; if(ai<0.012) continue; const ak=NEURONS[i].ak;
    const adj=NEURONS[i].adj; for(let k=0;k<adj.length;k++){ const j=adj[k].j, c=ai*adj[k].w; actInc[j]+=c; if(c>bestInc[j]){ bestInc[j]=c; akSrc[j]=ak; } } }
  for(let i=0;i<n;i++){ const m=NEURONS[i]; if(m.act<0.15 && bestInc[i]>0) m.ak=akSrc[i]; m.act=Math.min(1, m.act*0.93 + actInc[i]*0.11); }
}

/* ================= ESCENA THREE ================= */
const cv=document.getElementById('brain');
const renderer=new THREE.WebGLRenderer({canvas:cv, antialias:true});
renderer.setPixelRatio(Math.min(devicePixelRatio,2)); renderer.info.autoReset=false;
const scene=new THREE.Scene(); scene.fog=new THREE.FogExp2(0x05070d,0.0016);
const camera=new THREE.PerspectiveCamera(58, innerWidth/innerHeight, 1, 6000); camera.position.set(0,20,340);
const world=new THREE.Group(); scene.add(world);
scene.add(new THREE.AmbientLight(0x28405f,0.8));
const dl=new THREE.DirectionalLight(0xfff0dc,1.15); dl.position.set(-0.5,0.9,0.7); scene.add(dl);
const dl2=new THREE.DirectionalLight(0x6f9bff,0.4); dl2.position.set(0.6,-0.4,-0.55); scene.add(dl2);

// neuronas: icosaedro facetado + rim fresnel (onBeforeCompile)
const NGEO=new THREE.IcosahedronGeometry(1,1);
const nodeMat=new THREE.MeshStandardMaterial({ color:0xffffff, roughness:0.4, metalness:0.0, flatShading:true });
nodeMat.onBeforeCompile=(sh)=>{ sh.fragmentShader=sh.fragmentShader.replace('#include <opaque_fragment>',
  'float _fres=pow(1.0-max(dot(normalize(normal),normalize(vViewPosition)),0.0),2.4);\n  outgoingLight+=diffuseColor.rgb*_fres*1.5;\n#include <opaque_fragment>'); };

// aristas: tubo con shader de pulso continuo que barre el axón.
//
// UNA SOLA InstancedMesh PARA TODAS. Antes cada sinapsis era su propio Mesh con su propio
// ShaderMaterial: a 486 aristas se notaba poco, pero el grafo completo del cerebro central
// tiene 3.411 y eso son 3.411 draw calls con 3.411 materiales — el motivo real por el que el
// grafo estaba capado a 300 nodos, más que las esferas (que ya iban instanciadas).
//
// Lo que antes era un uniform por material ahora es un ATRIBUTO POR INSTANCIA: color, veloci-
// dad, brillo y base viajan en buffers y se actualizan in situ. `uTime` es lo único que queda
// compartido, porque es el mismo reloj para todas.
const EGEO=new THREE.CylinderGeometry(1,1,1,6,1,true);
const VERT=[
  'attribute vec3 aColor; attribute float aSpeed; attribute float aGlow; attribute float aBase;',
  'varying vec2 vUv; varying vec3 vColor; varying float vSpeed; varying float vGlow; varying float vBase;',
  'void main(){ vUv=uv; vColor=aColor; vSpeed=aSpeed; vGlow=aGlow; vBase=aBase;',
  // instanceMatrix lo declara three cuando el material se usa con InstancedMesh (USE_INSTANCING).
  '  gl_Position=projectionMatrix*modelViewMatrix*instanceMatrix*vec4(position,1.0); }'].join('\n');
const FRAG=['precision highp float;','uniform float uTime;',
  'varying vec2 vUv; varying vec3 vColor; varying float vSpeed; varying float vGlow; varying float vBase;',
  'float band(float y){ float p=fract(y); return smoothstep(0.0,0.06,p)*(1.0-smoothstep(0.06,0.36,p)); }',
  'void main(){ float y=vUv.y-uTime*vSpeed; float pulse=band(y)+band(y+0.5); float i=vBase+pulse*vGlow; gl_FragColor=vec4(vColor*i,i); }'].join('\n');

// ── DENDRITAS ────────────────────────────────────────────────────────────────────────────────
// Mismo truco que las aristas —UN cilindro instanciado, atributos por instancia— con dos cosas
// que una arista no necesita:
//
//   1. ADELGAZAMIENTO. Lo que hace que una rama se lea como dendrita y no como un palito es que
//      la punta sea más fina que la base. En canvas 2D eso era una propiedad del trazo; acá es
//      geometría, y se resuelve en el vertex shader escalando `position.xz` según la altura. Sin
//      esto habría que emitir un cilindro cónico por segmento, o sea 22.000 geometrías.
//   2. EL IMPULSO viaja por `aGlow`, que JS escribe por frame igual que en las aristas. No hay un
//      bucle que lo fabrique: se enciende cuando llega un evento del riel.
const DGEO=new THREE.CylinderGeometry(1,1,1,5,1,true);
// El shader va como template literal y no como array + join: GLSL acepta los saltos reales, y
// asi el fuente se lee igual que el shader que corre.
const DVERT=`
attribute vec3 aColor; attribute float aTaper; attribute float aGlow; attribute float aBase; attribute float aWarn;
varying vec3 vColor; varying float vGlow; varying float vBase; varying float vY; varying float vWarn;
void main(){ vColor=aColor; vGlow=aGlow; vBase=aBase; vWarn=aWarn; vY=position.y+0.5;
  vec3 p=position; p.xz*=mix(1.0,aTaper,vY);
  gl_Position=projectionMatrix*modelViewMatrix*instanceMatrix*vec4(p,1.0); }
`;
// El brillo cae hacia la punta: una dendrita real se apaga en los extremos, y sin eso las
// puntas finas quedan como un halo de polvo blanco alrededor del arbol.
const DFRAG=`
precision highp float;
varying vec3 vColor; varying float vGlow; varying float vBase; varying float vY; varying float vWarn;
void main(){ float i=vBase+vGlow*(1.0-0.35*vY);
  // EL FRENTE SATURA: un impulso fuerte quema hacia el blanco, uno debil apenas tine la rama.
  vec3 c=mix(vColor, vec3(1.0), clamp(vGlow*0.5, 0.0, 0.8));
  // Y si fallo, el nucleo se va a AMBAR ENTERO, sin dejar nada del color de la rama. Las dos
  // formas intermedias que probe no se leian, y las dos por la misma razon —quedaba cian adentro
  // del nucleo—: tenir el DESTINO de la saturacion deja el 20 % que el clamp no cubre, y tenir
  // con el ambar palido del HUD sobre un medio aditivo es casi blanco (ver impulsos.mjs).
  c=mix(c, vec3(${AMBAR_FRENTE.join(", ")}), vWarn);
  gl_FragColor=vec4(c*i,i); }
`;

// post: MSAA + bloom + SMAA
const _dbs=new THREE.Vector2(); renderer.getDrawingBufferSize(_dbs);
const composer=new EffectComposer(renderer, new THREE.WebGLRenderTarget(_dbs.x,_dbs.y,{samples:4}));
composer.setSize(_dbs.x,_dbs.y);
composer.addPass(new RenderPass(scene,camera));
const bloom=new UnrealBloomPass(new THREE.Vector2(_dbs.x,_dbs.y),0.95,0.7,0.28); composer.addPass(bloom);
composer.addPass(new SMAAPass(_dbs.x,_dbs.y));

const controls=new TrackballControls(camera, renderer.domElement);
controls.rotateSpeed=2.4; controls.zoomSpeed=1.3; controls.panSpeed=0.6; controls.dynamicDampingFactor=0.12; controls.staticMoving=false;

// mallas + buffers reconstruibles al cambiar el grafo. Las aristas son UNA InstancedMesh
// (edgeInst) con sus atributos por instancia; ya no hay un array de mallas ni de materiales.
let inst=null, N=0, BX,BY,BZ,RAD,PHX,PHY,PHZ,GX,GY,GZ,PULL,QROT,ADJ;
let edgeInst=null, edgeMat=null, ECOL=null, ESPD=null, EGLW=null, EBAS=null;
// DENDRITAS: una instancia por segmento de todos los árboles, y los somas aparte.
//
// Los buffers están APLANADOS a propósito. `DTRONCO[i]` y `DDIST[i]` dicen de qué árbol es la
// instancia i y a qué distancia del soma está, medida A LO LARGO de la rama. Con eso, escribir el
// brillo de las 12.010 instancias es un solo barrido lineal sin bajar por ninguna estructura
// anidada — que es lo que permite hacerlo por frame sin salirse del presupuesto.
let denInst=null, denMat=null, somaInst=null, DCOL=null, DGLW=null, DBAS=null, DTAP=null, DWARN=null;
let BOSQUE=[], DDIST=null, DTRONCO=null, DALC=null, DRAD=null;
// POSMEM: la posición de cada memoria, que ahora SALE DEL ÁRBOL. Es el reemplazo de `randInBrain()`.
let POSMEM=new Map();
// NEURONAS_DE traduce un RACIMO a los índices de sus neuronas. Es la última milla del camino
// `principal` -> persona -> racimo -> árboles.
//
// Antes el destino era una terminal y ahora es el racimo entero, y el motivo es medido: sólo el
// 7,5 % de las notas del central están firmadas por una terminal, así que las neuronas ya no son
// terminales sino tramos del árbol de temas. Lo que una llamada de `gio` dice con certeza es «esto
// lo hizo gio»; repartirlo a UNA de sus neuronas sería elegir a dedo. Enciende su racimo.
let NEURONAS_DE=new Map();
// El registro de pulsos vivos. NO hay ningún bucle que los fabrique: `impulsar()` es la única
// puerta, y la llama el riel cuando llega una invocación de verdad (ver impulsos.mjs).
const IMPULSOS=crearImpulsos();
let denSucio=false;
// Estado para no rehacer trabajo que no cambio. NHOT/EHOT recuerdan si el color escrito para esa
// instancia es el TENIDO POR ACTIVIDAD, para poder devolverlo al base UNA vez al apagarse en vez
// de reescribirlo todos los frames. `resto` es el residuo del arrastre y `actViva` si hay pulso.
let NHOT=null, EHOT=null, resto=0, actViva=false, lastThink=-1;
// _v/_up/_q se fueron con el quaternion de las aristas: ya nadie los usa.
const _m=new THREE.Matrix4(), _c=new THREE.Color(), _c2=new THREE.Color(), _pos=new THREE.Vector3(), _scl=new THREE.Vector3(), _eu=new THREE.Euler();
let framed=false;

// disposeMeshes: saca las mallas de la escena y libera SOLO lo que es de ellas.
//
// LA REGLA: se libera lo que se CLONO, nunca lo compartido. `EGEO`/`DGEO` se clonan por
// reconstruccion (cada clon lleva sus propios atributos por instancia) y hay que liberarlos o se
// acumulan. `NGEO` NO: es una sola icosaedro de modulo que usan las 2.219 memorias Y los 11 somas,
// y se vuelve a usar en la reconstruccion siguiente. Liberarla tira sus buffers de GPU para
// recrearlos acto seguido, y ademas se estaba haciendo DOS VECES por reconstruccion —una por
// `inst` y otra por `somaInst`— sobre el mismo objeto.
function disposeMeshes(){ if(inst){ world.remove(inst); inst=null; }
  if(edgeInst){ world.remove(edgeInst); edgeInst.geometry.dispose(); edgeInst=null; }
  if(edgeMat){ edgeMat.dispose(); edgeMat=null; }
  if(denInst){ world.remove(denInst); denInst.geometry.dispose(); denInst=null; }
  if(somaInst){ world.remove(somaInst); somaInst=null; }
  if(denMat){ denMat.dispose(); denMat=null; }
  ECOL=ESPD=EGLW=EBAS=null; DCOL=DGLW=DBAS=DTAP=DWARN=DDIST=DTRONCO=DALC=DRAD=null;
  NEURONAS_DE=new Map(); }
// OJO: `BOSQUE` NO se limpia acá. Es DATO —la geometría de los árboles, que arma buildGraph—, no
// una malla. Limpiarlo desde disposeMeshes lo borraba justo antes de usarlo, porque
// rebuildMeshes() empieza llamando a disposeMeshes(): el bosque se armaba con 12.010 segmentos y
// llegaba vacío al constructor de la malla. Sin excepción y sin dendritas.

const _qd=new THREE.Quaternion(), _va=new THREE.Vector3(), _vb=new THREE.Vector3(), _vd=new THREE.Vector3();
const _UP=new THREE.Vector3(0,1,0);
// construirBosqueMemoria: EL ÁRBOL DE CADA RACIMO, y acá está el cambio de fondo — las memorias
// dejan de ser puntos sorteados dentro de una esfera y pasan a ser las PUNTAS de la dendrita.
//
// La forma ya no sale de un PRNG: sale del dato (ver `arbol-memoria.mjs`). Lo único que se elige acá
// es la ESCALA — cuánto mide el primer tramo— y se elige contra el radio del racimo: el camino
// total de una rama es del orden de seis veces el primer tramo, así que con `ar/6` el árbol llega
// justo al borde de su racimo y no se mete en el de al lado.
function construirBosqueMemoria(ns0){
  const porRacimo=new Map();
  for(const n of ns0){ const k=n._grupo; if(!porRacimo.has(k)) porRacimo.set(k,[]); porRacimo.get(k).push(n); }
  const neuronas=[], posiciones=new Map();
  let semilla=13;
  for(const d of DOMAINS){
    const ms=porRacimo.get(d.name)||[]; if(!ms.length) continue;
    const r=construirRacimo(ms, { centro:[d.ax,d.ay,d.az], radio:d.ar,
      // La ESCALA sale medida, no a ojo: con `ar/54` el alcance del árbol da 73 en un racimo de
      // radio 77, o sea llena su racimo y no se mete en el de al lado.
      // El GROSOR también: con `radioHoja:0.55` el tronco salía 5,8 —el 8 % del radio del racimo—
      // y el árbol se leía como coral, no como dendrita. A 0,17 el tronco queda en ~1,8 sobre un
      // alcance de 73: relación 1:40, que es la de una dendrita de verdad.
      escala:Math.max(0.25, d.ar/54), radioHoja:0.17, semilla:(semilla+=577), min:30, max:150 });
    for(const [id,p] of r.posiciones) posiciones.set(id,p);
    r.neuronas.forEach((nu,k)=>neuronas.push({ ...nu, id:d.name+'#'+k, color:d.color, racimo:d.name }));
  }
  return { neuronas, posiciones };
}


// refrescarEncendido: el mapa `principal` -> terminal. Se rehace aparte del grafo porque su otra
// mitad —el censo de actores— llega por su propio camino y varios segundos después: sin esto, los
// eventos que caen en una persona por su credencial de servicio (`davantis-crm`, `crm-cabina`) se
// contarían como sin neurona hasta el siguiente rearmado del grafo.
// RACIMO_DE: de una TERMINAL al racimo donde vive. `personaDe()` no alcanza: el racimo puede ser el
// libro mayor, y una terminal cuya persona no tiene racimo propio no enciende nada.
let RACIMO_DE=new Map();

function refrescarEncendido(){
  if(!PERSONAS) return;
  const fus=fusionarActores(PERSONAS.terminales, CENSO && CENSO.censo);
  PERSONAS.actores=fus.actores; PERSONAS.sinDeclarar=fus.sinDeclarar; PERSONAS.censo=CENSO;
  ENCENDIDO=mapaDeEncendido(PERSONAS.terminales, fus.actores);
  RACIMO_DE=new Map(PERSONAS.terminales.map(t=>[t.id, t.persona||'']));
}

// refrescarPersonas: las cuatro cosas que salen de quién firma la memoria — las terminales, sus
// racimos, el mapa de encendido y los árboles. Van juntas porque son la misma lectura del grafo,
// y separarlas ya dejó una vez el mapa apuntando a un bosque que ya no existía.
// LA FIRMA de lo que personas+bosque miran. Barata (una pasada sobre 2.221 enteros) contra los
// 36,8 ms que cuesta rehacerlos: `extraerPersonas` corre regexes sobre todos los gists (22,7 ms) y
// `bosque` genera 12.010 segmentos (14,1 ms). Eso pasaba en CADA poll —cada 5 s, en el hilo
// principal— y el presupuesto de un cuadro a 60 fps son 16,6 ms: se saltaban dos o tres cuadros
// cada cinco segundos para recalcular casi siempre lo mismo.
//
// Va el CALOR y no solo la cantidad: `extraerPersonas` deriva el calor de cada terminal de la suma
// del de sus notas, y con una firma que solo mirara cuantas hay, ese numero se congelaba hasta que
// entrara una memoria nueva. El tooltip diria un calor viejo sin que nada avisara.
let _firmaPersonas='';
// Mira `ns0` —el grafo crudo— y no NEURONS, porque la firma se consulta ANTES de que NEURONS
// exista: la posición de cada neurona sale del árbol, así que el árbol tiene que estar primero.
function firmaGrafo(ns0){ let h=0; for(const n of ns0) h+=n.heat||0; return ns0.length+':'+h; }

function refrescarPersonas(brain){
  PERSONAS=extraerPersonas(brain);
  refrescarEncendido();
  return PERSONAS;
}

function rebuildDendritas(){
  // LOS ÁRBOLES SON DE LA MEMORIA. En la lente código el sujeto son símbolos, no personas, y
  // BOSQUE sigue cargado con los troncos que armó la lente anterior: sin este corte se dibujaban
  // encima del grafo de código, en las posiciones viejas —los anclajes de racimo ya no existen—,
  // y quedaba una mancha blanca flotando que no era nada. Se ve sólo entrando por memoria y
  // cambiando después; el atajo `?lens=code` no lo mostraba porque ahí BOSQUE nunca se llena.
  if(lens==='code') return;
  const total=BOSQUE.reduce((k,t)=>k+t.segs.length,0);
  if(!total) return;
  DCOL=new Float32Array(total*3); DGLW=new Float32Array(total); DBAS=new Float32Array(total);
  DTAP=new Float32Array(total); DDIST=new Float32Array(total); DTRONCO=new Int32Array(total);
  DWARN=new Float32Array(total);
  // El alcance de cada árbol se copia a un array plano: el frente del impulso se calcula contra
  // él una vez por tronco y por frame, y buscarlo en BOSQUE adentro del bucle grande sería
  // bajar por un objeto doce mil veces para leer siempre el mismo número.
  DALC=new Float32Array(BOSQUE.length); DRAD=new Float32Array(BOSQUE.length);
  NEURONAS_DE=new Map();
  const geo=DGEO.clone();
  geo.setAttribute('aColor', new THREE.InstancedBufferAttribute(DCOL,3));
  geo.setAttribute('aGlow',  new THREE.InstancedBufferAttribute(DGLW,1));
  geo.setAttribute('aBase',  new THREE.InstancedBufferAttribute(DBAS,1));
  geo.setAttribute('aTaper', new THREE.InstancedBufferAttribute(DTAP,1));
  geo.setAttribute('aWarn',  new THREE.InstancedBufferAttribute(DWARN,1));
  denMat=new THREE.ShaderMaterial({ uniforms:{}, vertexShader:DVERT, fragmentShader:DFRAG,
    transparent:true, blending:THREE.AdditiveBlending, depthWrite:false });
  denInst=new THREE.InstancedMesh(geo, denMat, total);
  denInst.frustumCulled=false;   // la malla envuelve toda la escena, igual que las aristas

  let i=0;
  // Los pulsos vivos guardan el INDICE de su tronco, y los indices acaban de cambiar. Uno que
  // sobreviva enciende, por lo que le quede de vida, la neurona equivocada — sin error de ninguna
  // clase, y atribuyendole la llamada a quien no fue.
  IMPULSOS.limpiar();
  BOSQUE.forEach((tr,ti)=>{ _c.set(tr.color||'#7f9cc9');
    if(!NEURONAS_DE.has(tr.racimo)) NEURONAS_DE.set(tr.racimo, []);
    NEURONAS_DE.get(tr.racimo).push(ti);
    DALC[ti]=tr.alcanceRama||1; DRAD[ti]=tr.rSoma||1;
    for(const sg of tr.segs){
      _va.set(sg.a[0],sg.a[1],sg.a[2]); _vb.set(sg.b[0],sg.b[1],sg.b[2]);
      _vd.subVectors(_vb,_va); const len=_vd.length()||0.001;
      _qd.setFromUnitVectors(_UP,_vd.normalize());
      _m.compose(_pos.copy(_va).addScaledVector(_vd,len*0.5), _qd, _scl.set(sg.w0,len,sg.w0));
      denInst.setMatrixAt(i,_m);
      DCOL[i*3]=_c.r; DCOL[i*3+1]=_c.g; DCOL[i*3+2]=_c.b;
      DTAP[i]=Math.max(0.05, sg.w1/sg.w0);
      // El brillo en reposo cae con el nivel: el tronco se ve y las puntas se insinúan. Plano,
      // el árbol se lee como una maraña de alambre del mismo peso.
      //
      // Y ARRANCA BAJO. Con 0,62 en el nivel 0 el árbol entero saturaba a blanco: son ramas que se
      // superponen sobre blending ADITIVO y con bloom encima, así que el brillo se suma tres veces
      // antes de llegar al ojo. El color del racimo desaparecía — cuatro racimos, todos blancos.
      // El decaimiento tiene que ser SUAVE, no el 0,74 que servia para un arbol decorativo de 4-5
      // niveles: este tiene 10, y con esa caida el 80 % de las ramas —que son las puntas, donde
      // estan las memorias— quedaba por debajo del piso y el arbol se veia como cuatro palitos.
      DBAS[i]=Math.max(0.17, 0.52*Math.pow(0.90,sg.nivel));
      DDIST[i]=sg.dist; DTRONCO[i]=ti; DGLW[i]=0; i++;
    } });
  denInst.instanceMatrix.needsUpdate=true;
  world.add(denInst);

  // Los SOMAS, en su propia malla: son once esferas y no justifican un shader, pero sí que se
  // vean como cuerpos y no como el nacimiento de las ramas.
  somaInst=new THREE.InstancedMesh(NGEO, nodeMat, BOSQUE.length);
  BOSQUE.forEach((tr,k)=>{ _m.compose(_pos.set(tr.centro[0],tr.centro[1],tr.centro[2]),
    new THREE.Quaternion(), _scl.setScalar(tr.rSoma));
    somaInst.setMatrixAt(k,_m); somaInst.setColorAt(k,_c.set(tr.color||'#7f9cc9')); });
  denSucio=true;   // buffers recién creados: el primer frame tiene que subirlos
  somaInst.instanceMatrix.needsUpdate=true; if(somaInst.instanceColor) somaInst.instanceColor.needsUpdate=true;
  world.add(somaInst);
}

// LOS DESPACHOS QUEDARON SIN CUERPO DEL QUE COLGAR, y por eso no se dibujan en esta fase.
//
// Un despacho va de una TERMINAL a otra, y las terminales ya no son neuronas: con sólo el 7,5 % de
// las notas firmadas, las neuronas pasaron a ser tramos del árbol de temas. Colgar el axón de una
// neurona cualquiera del racimo sería elegir a dedo de dónde sale, que es exactamente el invento
// que este dibujo no hace.
//
// Vuelven como líneas de campo entre RACIMOS —que sí tienen cuerpo— con el resto del campo. La
// leyenda los sigue contando («27 pares se escriben · 152 despachos») para que su ausencia del
// dibujo no se lea como que no existen.

function rebuildMeshes(){
  disposeMeshes(); N=NEURONS.length; if(!N) return;
  BX=new Float32Array(N); BY=new Float32Array(N); BZ=new Float32Array(N); RAD=new Float32Array(N);
  PHX=new Float32Array(N); PHY=new Float32Array(N); PHZ=new Float32Array(N);
  GX=new Float32Array(N); GY=new Float32Array(N); GZ=new Float32Array(N); PULL=new Float32Array(N); QROT=[];
  ADJ=Array.from({length:N},()=>[]);
  inst=new THREE.InstancedMesh(NGEO, nodeMat, N); NHOT=new Uint8Array(N); resto=1; lastThink=-1;
  for(let i=0;i<N;i++){ const n=NEURONS[i]; BX[i]=n.x; BY[i]=n.y; BZ[i]=n.z; RAD[i]=n.r; PHX[i]=n.phx; PHY[i]=n.ph; PHZ[i]=n.phz;
    QROT.push(new THREE.Quaternion().setFromEuler(_eu.set(Math.random()*6.283,Math.random()*6.283,Math.random()*6.283)));
    _m.compose(_pos.set(n.x,n.y,n.z), QROT[i], _scl.set(n.r,n.r,n.r)); inst.setMatrixAt(i,_m); inst.setColorAt(i,_c.set(n.col)); }
  inst.instanceMatrix.needsUpdate=true; if(inst.instanceColor) inst.instanceColor.needsUpdate=true;
  world.add(inst);
  // Aristas: UNA instancia por sinapsis dentro de UNA malla. El índice en los buffers es el
  // índice en SYN, así que animate() escribe por posición sin buscar nada.
  const E=SYN.length;
  if(E){
    ECOL=new Float32Array(E*3); ESPD=new Float32Array(E); EGLW=new Float32Array(E); EBAS=new Float32Array(E); EHOT=new Uint8Array(E);
    const geo=EGEO.clone();
    geo.setAttribute('aColor', new THREE.InstancedBufferAttribute(ECOL,3));
    geo.setAttribute('aSpeed', new THREE.InstancedBufferAttribute(ESPD,1));
    geo.setAttribute('aGlow',  new THREE.InstancedBufferAttribute(EGLW,1));
    geo.setAttribute('aBase',  new THREE.InstancedBufferAttribute(EBAS,1));
    edgeMat=new THREE.ShaderMaterial({ uniforms:{ uTime:{value:0} }, vertexShader:VERT, fragmentShader:FRAG,
      transparent:true, blending:THREE.AdditiveBlending, depthWrite:false });
    edgeInst=new THREE.InstancedMesh(geo, edgeMat, E);
    edgeInst.frustumCulled=false;   // la malla envuelve todo el cerebro: cullearla por su bbox la haría desaparecer entera
    for(let i=0;i<E;i++){ const s=SYN[i]; ADJ[s.a].push(s.b); ADJ[s.b].push(s.a);
      s.__i=i; s.__r=0.28+(s.confidence||0)*0.5;
      _c.set(edgeBase(s)); ECOL[i*3]=_c.r; ECOL[i*3+1]=_c.g; ECOL[i*3+2]=_c.b;
      ESPD[i]=0.42+(s.confidence||0)*0.5; EGLW[i]=0.55; EBAS[i]=0.06; }
    world.add(edgeInst);
  } else {
    for(const s of SYN){ ADJ[s.a].push(s.b); ADJ[s.b].push(s.a); }
  }
  rebuildDendritas();
  // ENCUADRE, y es POR LENTE. La esfera de codigo se mira desde lejos porque su borde es liso y
  // llenar el cuadro con ella la desborda; el arbol tiene el detalle en las puntas y pide estar
  // cerca. Un solo numero servia cuando las dos lentes dibujaban una esfera.
  if(!framed && N){ let mr=0; for(const n of NEURONS){ const d=Math.hypot(n.x,n.y,n.z); if(d>mr)mr=d; }
    camera.position.set(0,20,Math.max(200, mr*(lens==='code'?2.7:1.85))); framed=true; }
  needsRebuild=false;
}

/* ---------- interacción: arrastre de neurona + vecinos + resorte de retorno ---------- */
const ray=new THREE.Raycaster(); const ptr=new THREE.Vector2(-9,-9); let mx=0,my=0;
let drag=-1, dragPid=-1; const _plane=new THREE.Plane(), _dt=new THREE.Vector3(), _camDir=new THREE.Vector3();
const tip=document.getElementById('tip');
addEventListener('pointermove',ev=>{ ptr.x=(ev.clientX/innerWidth)*2-1; ptr.y=-(ev.clientY/innerHeight)*2+1; mx=ev.clientX; my=ev.clientY; });
renderer.domElement.addEventListener('pointerdown',ev=>{ if(!inst) return; ptr.x=(ev.clientX/innerWidth)*2-1; ptr.y=-(ev.clientY/innerHeight)*2+1;
  ray.setFromCamera(ptr,camera); const hit=ray.intersectObject(inst);
  if(hit.length){ drag=hit[0].instanceId; dragPid=ev.pointerId; try{ renderer.domElement.setPointerCapture(ev.pointerId); }catch(_){}
    renderer.domElement.style.cursor='grabbing'; camera.getWorldDirection(_camDir); _plane.setFromNormalAndCoplanarPoint(_camDir, hit[0].point);
    PULL.fill(0); for(const j of ADJ[drag]) PULL[j]=0.55; ev.stopImmediatePropagation(); ev.preventDefault(); } }, true);
function endDrag(){ if(drag>=0){ drag=-1; renderer.domElement.style.cursor=''; if(PULL) PULL.fill(0); try{ if(dragPid>=0) renderer.domElement.releasePointerCapture(dragPid); }catch(_){} dragPid=-1; } }
renderer.domElement.addEventListener('pointerup',endDrag,true); renderer.domElement.addEventListener('pointercancel',endDrag,true);
addEventListener('pointerup',endDrag); addEventListener('blur',endDrag);
function hover(){ if(drag>=0 || !inst){ tip.classList.remove('on'); return; } ray.setFromCamera(ptr,camera);
  // LOS SOMAS PRIMERO. Son once contra 2.219 memorias, y están adentro de la nube de puntos de su
  // racimo: si gana el rayo más cercano, la neurona queda tapada por cualquier memoria que le pase
  // por delante y nunca se la puede mirar. Preguntarle primero cuesta un raycast sobre once
  // instancias.
  if(somaInst){ const hs=ray.intersectObject(somaInst);
    if(hs.length){ const tr=BOSQUE[hs[0].instanceId];
      if(tr){ tipNeurona(tr, mx, my); return; } } }
  const hit=ray.intersectObject(inst);
  if(hit.length){ const n=NEURONS[hit[0].instanceId]; tip.querySelector('.tt').textContent=n.topic||n.domain;
    if(n._code){
      fetchExplain(n);   // trae (una vez, on-demand) las memorias que EXPLICAN este símbolo
      let tg=esc(n.gist||'(sin ruta)');
      if(Array.isArray(n._exp) && n._exp.length) tg+='<div class="why">'+n._exp.slice(0,3).map(e=>`<span>${esc(e.topic_key)}</span>`).join('')+'</div>';
      tip.querySelector('.tg').innerHTML=tg;
      let meta=`<i>${esc(quienEscribio(n))}</i><i>${esc(n.domain)}</i><i>centralidad ${n.heat}</i>`;
      if(Array.isArray(n._exp)) meta+= n._exp.length?`<i style="color:var(--purple)">explicado ×${n._exp.length}</i>`:`<i style="opacity:.55">sin memorias</i>`;
      tip.querySelector('.tm').innerHTML=meta;
    } else {
      tip.querySelector('.tg').textContent=n.gist||'(sin resumen)';
      tip.querySelector('.tm').innerHTML=`<i>${esc(quienEscribio(n))}</i><i>${esc(n.domain)}</i><i>calor ${n.heat}</i>`;
    }
    const tw=tip.offsetWidth||220, th=tip.offsetHeight||70; let x=mx+16,y=my+16; if(x+tw>innerWidth-8)x=mx-tw-16; if(y+th>innerHeight-8)y=my-th-16;
    tip.style.left=x+'px'; tip.style.top=y+'px'; tip.classList.add('on'); } else tip.classList.remove('on'); }

// fetchExplain: trae UNA vez las memorias que explican el símbolo (weld F3), cacheado en el nodo
// (n._exp). Lazy + debounce por el flag: solo pega a /api/explained la primera vez que se hoverea.
async function fetchExplain(n){ if(n._exp!==undefined || n._expLoading) return; n._expLoading=true;
  try{ const r=await fetch('/api/explained?symbol='+encodeURIComponent(n.id),{cache:'no-store'}); n._exp=r.ok?((await r.json())||[]):[]; }
  catch(_){ n._exp=[]; } finally{ n._expLoading=false; } }

/* ---------- loop ---------- */
// AMP: la amplitud del vaiven de cada nodo. En la lente MEMORIA es CERO y no es una decision de
// gusto: la memoria ahora es la PUNTA de una rama, y la rama es geometria fija. Con el vaiven
// puesto, el punto se despega de su rama y el dibujo deja de decir lo que dice.
//
// Efecto lateral bueno: sin movimiento, el bucle por nodo y por arista no reescribe nada en reposo
// (ya habia un guardia para eso) — o sea que el 98 % de los frames dejan de tocar 2.219 matrices.
const AMP_CODIGO=2.4;
let AMP=2.4;
/* ---------- MEDIDOR (opt-in con ?stats=1) ----------
   Existe porque "se siente pesado" no es una medicion. `renderer.info.reset()` ya se llamaba
   todos los frames y NADIE leia renderer.info: la mitad del plumbing estaba puesta.
   Separa lo que puedo optimizar en JS (los dos bucles) de lo que cuesta la GPU (draw + bloom +
   SMAA), que es la parte que no se puede medir desde headless porque SwiftShader se cuelga. */
const STATS = new URLSearchParams(location.search).has('stats');
let _statBox=null, _acumFrame=0, _acumJS=0, _frames=0, _ultStat=0;
if(STATS){
  _statBox=document.createElement('div');
  _statBox.style.cssText='position:fixed;left:50%;top:8px;transform:translateX(-50%);z-index:99;'+
    'font:11px/1.5 ui-monospace,Menlo,monospace;color:#9fe8ff;background:rgba(6,10,18,.86);'+
    'border:1px solid #23405c;border-radius:7px;padding:7px 12px;white-space:pre;pointer-events:none;'+
    'letter-spacing:.02em;text-align:center';
  _statBox.textContent='midiendo…';
  document.body.appendChild(_statBox);
}

function animate(){ requestAnimationFrame(animate); renderer.info.reset();
  const _t0 = STATS ? performance.now() : 0;
  if(needsRebuild || (inst && (N!==NEURONS.length || (edgeInst?edgeInst.count:0)!==SYN.length))) rebuildMeshes();
  // El layout se asienta de a tramos: 6 ms por frame deja ~10 ms para dibujar y mantiene 60 fps
  // mientras el grafo se organiza a la vista.
  const asentando = asentar(6); if(asentando) refrescarBase(settlePendiente()>0 ? 0.16 : 1);
  const t=performance.now()/1000;
  if(motion){ thinking*=0.985; spread(); }
  if(inst && N){
    // objetivo de arrastre (coords locales)
    if(drag>=0){ ray.setFromCamera(ptr,camera); if(ray.ray.intersectPlane(_plane,_dt)) world.worldToLocal(_dt); else drag=-1; }
    let Dkx=0,Dky=0,Dkz=0;
    if(drag>=0){ const fx=BX[drag]+Math.sin(t*0.5+PHX[drag])*AMP, fy=BY[drag]+Math.sin(t*0.44+PHY[drag])*AMP, fz=BZ[drag]+Math.sin(t*0.57+PHZ[drag])*AMP;
      Dkx=_dt.x-fx; Dky=_dt.y-fy; Dkz=_dt.z-fz; GX[drag]=Dkx; GY[drag]=Dky; GZ[drag]=Dkz; }
    // ¿SE MOVIÓ ALGO DE VERDAD? Con la animación en pausa, sin arrastre y con el residuo del
    // último ya decaido, las posiciones son idénticas a las del frame anterior — y los dos bucles
    // de abajo escribirían exactamente lo mismo que ya está. Medido en la lente código del central
    // (8193 nodos, 17661 aristas): eran 16,5 ms de JS y 2 MB de subida a la GPU POR FRAME, cuando
    // el presupuesto entero de un frame a 60 fps son 16,6 ms.
    if(motion || drag>=0 || resto>0.002 || actViva || asentando){
    let rMax=0, hayAct=false, colorSucio=false;
    // La 3x3 (rotación x escala) es CONSTANTE por nodo y ya quedó sembrada en rebuildMeshes: por
    // frame sólo cambian los 3 floats de traslación, escritos DIRECTO en el buffer de instancias.
    // compose+setMatrixAt costaba 6,20 ms a 8193 nodos; esto, 1,57 ms.
    const NM=inst.instanceMatrix.array;
    for(let i=0;i<N;i++){ const n=NEURONS[i];
      const dr=motion?AMP:0;
      const fx=BX[i]+Math.sin(t*0.5+PHX[i])*dr, fy=BY[i]+Math.sin(t*0.44+PHY[i])*dr, fz=BZ[i]+Math.sin(t*0.57+PHZ[i])*dr;
      if(i===drag){} else if(PULL[i]>0){ const p=PULL[i]; GX[i]+=(Dkx*p-GX[i])*0.14; GY[i]+=(Dky*p-GY[i])*0.14; GZ[i]+=(Dkz*p-GZ[i])*0.14; }
      else { GX[i]*=0.945; GY[i]*=0.945; GZ[i]*=0.945; }
      const gx=GX[i], gy=GY[i], gz=GZ[i];
      const g=Math.abs(gx)+Math.abs(gy)+Math.abs(gz); if(g>rMax) rMax=g;
      const x=fx+gx, y=fy+gy, z=fz+gz; n.x=x; n.y=y; n.z=z;
      const o=i*16; NM[o+12]=x; NM[o+13]=y; NM[o+14]=z;
      // color: dominio + tinte de actividad si está encendida (cubre neuronas aisladas sin aristas).
      // Se escribe SÓLO mientras hay actividad, y una última vez al apagarse. En la lente código no
      // hay actividad NUNCA (es reposo puro), así que estos writes eran íntegramente basura.
      if(n.act>0.06){ hayAct=true; _c.set(n.col); _c2.set(AK[n.ak]||REPOSO); _c.lerp(_c2, Math.min(0.85,n.act*0.8)); inst.setColorAt(i,_c); NHOT[i]=1; colorSucio=true; }
      else if(NHOT[i]){ inst.setColorAt(i,_c.set(n.col)); NHOT[i]=0; colorSucio=true; } }
    inst.instanceMatrix.needsUpdate=true; if(colorSucio && inst.instanceColor) inst.instanceColor.needsUpdate=true;
    // aristas: siguen a las neuronas + pulso (reposo tenue vs actividad brillante).
    // Todo se escribe en los buffers de la ÚNICA malla instanciada: una draw call para las
    // 3.411 del cerebro completo, en vez de una por arista.
    if(edgeInst){
      edgeMat.uniforms.uTime.value=t;
      const EM=edgeInst.instanceMatrix.array;
      const pensando=Math.abs(thinking-lastThink)>0.002; if(pensando) lastThink=thinking;
      let attrSucio=false;
      for(let si=0; si<SYN.length; si++){ const s=SYN[si]; const a=NEURONS[s.a], b=NEURONS[s.b];
        // Base ortonormal escrita a mano en el buffer. El cilindro es simétrico radialmente, así
        // que CUALQUIER par de ejes perpendiculares al eje A->B sirve: no hace falta el quaternion
        // de setFromUnitVectors, que era el 62% del costo del frame. 10,33 ms -> 3,04 ms.
        let dx=b.x-a.x, dy=b.y-a.y, dz=b.z-a.z;
        const len=Math.sqrt(dx*dx+dy*dy+dz*dz)||1, il=1/len; dx*=il; dy*=il; dz*=il;
        let px,py,pz;
        if(Math.abs(dy)<0.9){ px=dz; py=0; pz=-dx; } else { px=0; py=-dz; pz=dy; }   // el eje menos alineado: evita el caso degenerado
        const pl=Math.sqrt(px*px+py*py+pz*pz)||1, ipl=1/pl; px*=ipl; py*=ipl; pz*=ipl;
        // q = p x d, NO d x p: con (X,Y,Z)=(p,d,q) la terna tiene que ser DERECHA. Al reves el
        // determinante sale negativo, se invierte el winding y el back-face culling se come las
        // aristas. Verificado sobre 200.004 casos: det>0 en todos.
        const qx=py*dz-pz*dy, qy=pz*dx-px*dz, qz=px*dy-py*dx;
        const r=s.__r, o=si*16;
        EM[o   ]=px*r;    EM[o+1 ]=py*r;    EM[o+2 ]=pz*r;    EM[o+3 ]=0;
        EM[o+4 ]=dx*len;  EM[o+5 ]=dy*len;  EM[o+6 ]=dz*len;  EM[o+7 ]=0;
        EM[o+8 ]=qx*r;    EM[o+9 ]=qy*r;    EM[o+10]=qz*r;    EM[o+11]=0;
        EM[o+12]=(a.x+b.x)*0.5; EM[o+13]=(a.y+b.y)*0.5; EM[o+14]=(a.z+b.z)*0.5; EM[o+15]=1;
        const act=Math.max(a.act,b.act), ak=(a.act>=b.act?a.ak:b.ak);
        if(act>0.06 && ak>0){ _c.set(AK[ak]); EGLW[si]=0.55+act*3.6; ESPD[si]=1.0+act*1.6;   // ACTIVIDAD: brillante
          ECOL[si*3]=_c.r; ECOL[si*3+1]=_c.g; ECOL[si*3+2]=_c.b; EHOT[si]=1; attrSucio=true; }
        else if(EHOT[si] || pensando){ _c.set(edgeBase(s)); EGLW[si]=0.5+thinking*0.35; ESPD[si]=0.42+(s.confidence||0)*0.5;   // REPOSO: color por tipo (código) o azul tenue (memoria)
          ECOL[si*3]=_c.r; ECOL[si*3+1]=_c.g; ECOL[si*3+2]=_c.b; EHOT[si]=0; attrSucio=true; } }
      edgeInst.instanceMatrix.needsUpdate=true;
      if(attrSucio){ const at=edgeInst.geometry.attributes;
        at.aColor.needsUpdate=true; at.aSpeed.needsUpdate=true; at.aGlow.needsUpdate=true; }
    }
    resto=rMax; actViva=hayAct;
    }
  }
  // ── EL IMPULSO ───────────────────────────────────────────────────────────────────────────
  // Va FUERA del bloque de arriba y con su propia condición a propósito: un pulso tiene que
  // recorrer su árbol aunque la animación esté en pausa y aunque no se haya movido un solo nodo.
  // Metido adentro, pausar el panel congelaba el impulso a mitad de camino.
  //
  // `denSucio` es lo que apaga el árbol UNA vez cuando muere el último pulso, en vez de reescribir
  // doce mil ceros en cada frame de reposo — que es el 98 % de los frames.
  if(denInst && DALC){
    const vivos=IMPULSOS.vivos(t);
    if(vivos>0 || denSucio){
      const r=IMPULSOS.escribir(t,{glow:DGLW, warn:DWARN, dist:DDIST, tronco:DTRONCO, alcances:DALC});
      const at=denInst.geometry.attributes;
      at.aGlow.needsUpdate=true; at.aWarn.needsUpdate=true;
      // El SOMA fogonea con el arranque. La rotación de estas instancias es la identidad, así que
      // la escala son los tres elementos de la diagonal y se escriben directo: recomponer once
      // matrices por frame para cambiar un número sería el mismo gasto que costaba el grafo entero.
      if(somaInst){ const SM=somaInst.instanceMatrix.array;
        for(let k=0;k<DRAD.length;k++){ const e=DRAD[k]*(1+0.9*r.flash[k]), o=k*16;
          SM[o]=e; SM[o+5]=e; SM[o+10]=e; }
        somaInst.instanceMatrix.needsUpdate=true; }
      denSucio = vivos>0;
    }
  }
  const _tJS = STATS ? performance.now()-_t0 : 0;
  controls.update(); hover(); composer.render();
  if(STATS){
    const ahora=performance.now();
    _acumFrame += ahora-_t0; _acumJS += _tJS; _frames++;
    if(ahora-_ultStat > 500){
      const r=renderer.info.render, m=renderer.info.memory;
      const msF=_acumFrame/_frames, msJS=_acumJS/_frames;
      _statBox.textContent =
        Math.round(1000/msF)+' fps   frame '+msF.toFixed(1)+' ms'+
        '   ·   mis bucles '+msJS.toFixed(1)+' ms   ·   render+bloom '+(msF-msJS).toFixed(1)+' ms\n'+
        r.calls+' draw · '+(r.triangles/1000).toFixed(0)+'k tris · dpr '+renderer.getPixelRatio()+
        ' · '+m.geometries+' geo · '+m.textures+' tex';
      _acumFrame=_acumJS=_frames=0; _ultStat=ahora;
    }
  }
}

/* ---------- HUD (PORTADO, paridad con el dashboard previo) ---------- */
const $=id=>document.getElementById(id);
const esc=s=>(s==null?'':String(s)).replace(/[&<>"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]));
function renderHUD(d){
  const code=lens==='code';
  const b=d.brain||{neurons:[],synapses:[]}, ins=d.insights||{}, ob=ins.observations||{};
  const cg=d.code||{nodes:[],edges:[]};
  const shown=code?(cg.nodes||[]).length:(b.neurons||[]).length;
  const total=code?(cg.total_nodes||0):(b.total_neurons||0);
  const trunc=code?cg.truncated:b.truncated;
  // Las aristas se recortan MUCHO más que los nodos —una se pierde en cuanto uno de sus dos
  // extremos queda fuera del top-N—, así que necesitan su propio total y su propia bandera.
  // Imprimir su largo pelado al lado de un "300/3660" le enseña al ojo que ahí el número es
  // el total, y no lo es: en el cerebro central eran 486 dibujadas sobre 3620 reales.
  const edgeN=code?(cg.edges||[]).length:(b.synapses||[]).length;
  const edgeTotal=code?(cg.total_edges||0):(b.total_synapses||0);
  const edgeTrunc=code?!!cg.edges_truncated:!!b.synapses_truncated;
  const edgeTxt=edgeTrunc?`${edgeN}/${edgeTotal}`:edgeN;
  $('neuronCount').textContent=trunc?`${shown}/${total}`:shown;
  $('synCount').textContent=edgeTxt;
  $('proj').textContent=d.project||'—'; $('ver').textContent=d.version||'';
  // En lente MEMORIA el KPI es el universo recuperable: utilization.active y observations.visible
  // salen los dos del predicado canónico; observations.active queda de último recurso porque
  // cuenta como vivas las notas superadas y las que están en cuarentena.
  // En lente CÓDIGO el KPI es el total de nodos, no los dibujados: con `shown` contradecía al
  // contador de arriba, que en la misma pantalla ya decía "400/8193".
  const visibles=(ins.utilization||{}).active;
  $('kActive').textContent=code?(total||shown):(visibles!=null?visibles:(ob.visible!=null?ob.visible:(ob.active!=null?ob.active:'—')));
  $('kSyn').textContent=edgeTxt;
  // Los dominios REALES vienen en graph.domains, calculados por SQL sobre toda la memoria.
  // DOMAINS se arma de la muestra dibujada y sirve para COLOREAR: usarlo para contar decía 46
  // donde había 90. En lente código el equivalente del universo es total_modules.
  const gdoms=((d.graph||{}).domains)||[];
  // PERSONAS cuenta personas. El libro mayor y lo sin atribuir son racimos y no personas: sumarlos
  // pondría un número cierto bajo un rótulo falso. Y ya NO se usa `graph.domains` del servidor
  // acá: ese conteo es por DOMINIO, que dejó de ser el sujeto de esta escena. Mezclarlos daría
  // «90» bajo el rótulo «Personas».
  const personasVisibles=DOMAINS.filter(dd=>dd.name!==GRUPO_LIBRO&&dd.name!==GRUPO_SIN_ATRIBUIR).length;
  $('kDomains').textContent=code
    ?(cg.total_modules?(cg.truncated&&DOMAINS.length<cg.total_modules?`${DOMAINS.length}/${cg.total_modules}`:cg.total_modules):DOMAINS.length)
    :personasVisibles;
  // La leyenda también: su ranking y sus conteos salían del top-N por saliencia, que está
  // sesgado por recencia y calor — un dominio grande y frío no aparecía. El color se sigue
  // tomando de DOMCOL (la muestra); los que no entraron se pintan en reposo.
  // La leyenda sale de DOMAINS, que ahora es la lista de RACIMOS armada sobre el grafo entero
  // (el tope de 300 se fue: el payload trae las 2.217). Antes salía de `graph.domains`, que el
  // servidor calcula por dominio en SQL — otro sujeto, y ya no aplica.
  const legend=DOMAINS.slice(0,10);
  // En la lente personas esta tarjeta muestra a las PERSONAS, no a los dominios. El guardia va
  // acá y no en renderLens porque renderHUD corre en CADA poll: sin él, la leyenda de personas
  // vivía cinco segundos y volvía a ser la de dominios sola.
  if(code) $('domlegend').innerHTML=legend.length?legend.map(dd=>`<div class="lg"><span class="sw" style="background:${dd.color};color:${dd.color}"></span>${esc(dd.name)} <b>${dd.count}</b></div>`).join(''):'<div class="empty">sin módulos</div>';
  else pintarLeyendaRacimos(legend);
  const runs=((d.orchestration||{}).runs)||[];
  $('kRuns').textContent=runs.filter(r=>r.status==='running'||r.done<r.total).length;
  const h=d.health||{}, bad=(h.checks||[]).filter(c=>c.status&&c.status!=='ok').length;
  $('health').className='pill '+(h.status==='ok'?'ok':'warn');
  $('healthTxt').textContent=h.status==='ok'?'sano':(bad?`${bad} avisos`:'revisar');
  $('checks').innerHTML=(h.checks||[]).length?(h.checks||[]).map(c=>{ const ok=!c.status||c.status==='ok';
    return `<div class="chk"><span class="s" style="background:${ok?'var(--green)':'var(--amber)'};box-shadow:0 0 7px ${ok?'var(--green)':'var(--amber)'}"></span>${esc(c.code||c.name||'check')}</div>`;}).join(''):'<div class="empty">sin checks</div>';
  // TRES estados, no dos. Sin `session_id` nadie pasó un id de hook, y sin hooks el ledger NUNCA
  // rota: es un proceso always-on (el cerebro central) donde el total es un ACUMULADO DE POR VIDA.
  // Compararlo contra un techo pensado para UNA sesión daba una barra clavada al 100% y un número
  // que no significaba nada (se vio en vivo: 2.153.453 / 8000, sin moverse desde el deploy).
  // El techo es blando y no recorta, así que acá lo único que hay que arreglar es lo que se dice.
  // Ver internal/memory/ledger.go (LedgerAdd) para el porqué del sessionID vacío.
  const tk=d.tokens||{}, sessioned=!!tk.session_id;
  const budgeted=sessioned&&tk.status&&tk.status!=='unbudgeted'&&tk.budget;
  $('tokLabel').textContent=budgeted?'Tokens de sesión':(sessioned?'Tokens (sin techo)':'Tokens acumulados');
  $('tokVal').textContent=budgeted?`${tk.total||0} / ${tk.budget}`:(tk.total||0);
  // El estado del gobernador sólo se pinta si hay sesión con techo: si no, `over` es permanente por
  // construcción y la alerta visual pierde todo su valor de aviso.
  const bar=$('tokBar'); bar.className='bar'+(budgeted&&(tk.status==='watch'||tk.status==='over')?' watch':''); bar.querySelector('i').style.width=Math.min(100, budgeted?(tk.pct_used||0):(tk.total?8:0))+'%';
  $('runs').innerHTML=runs.length?runs.slice(0,6).map(r=>{ const done=r.done||0,tot=r.total||0,pct=tot?Math.round(done*100/tot):0;
    const col=r.status==='done'?'var(--green)':r.status==='failed'?'var(--red)':'var(--amber)';
    return `<div class="run"><span class="s" style="width:6px;height:6px;border-radius:50%;background:${col};box-shadow:0 0 7px ${col}"></span><span class="rl">${esc(r.workflow_id||r.run_id)}</span><span class="rp">${done}/${tot}·${pct}%</span></div>`;}).join(''):'<div class="empty">sin runs activos</div>';
  const rec=d.recent||[];
  $('recent').innerHTML=rec.length?rec.slice(0,6).map(o=>`<div class="item"><span class="tk">${esc(o.topic_key)}</span><span class="gg">${esc(o.gist)}</span></div>`).join(''):'<div class="empty">memoria en formación</div>';
}

/* ---------- RIEL EN VIVO ----------
   Lo que se ve aca NO sale de la base local: llega por SSE desde el cerebro central, a traves del
   relay del panel (cmd/musubi/livestream.go). El motivo esta medido: en 24 h, la base local tuvo
   97.889 invocaciones de las cuales 97.815 (99,92%) fueron tres tools de sondeo, y el trabajo real
   fueron 4 save_observation y 1 recall. El central, en las mismas 24 h, tuvo 29 tools distintas y
   ~550 eventos de trabajo de seis terminales, con `principal` para decir de quien fue cada uno.

   EL GRAFO NO SE PINTA CON ESTOS EVENTOS, y es deliberado. El grafo dibuja la memoria LOCAL; un
   evento del central encendiendo una neurona local seria inventar. Lo que si hace un evento es
   PEDIR UN PULSO YA: si de verdad cambio algo local, los deltas reales lo encienden; si no cambio
   nada, no se enciende nada. La animacion sigue saliendo de datos medidos, nunca del feed. */
const VIVO={ nuevos:[], vistos:new Set(), sondeos:[], enlace:{estado:'conectando'} };
const VIVO_MAX=40;          // filas en el riel
const VIVO_DEDUPE=600;      // claves recordadas para no repetir al reconectar

// El backlog se re-manda en cada reconexion, asi que hay que descartar lo ya visto. La clave
// incluye la hora porque `seq` reinicia si el cerebro se reinicia, y dos eventos distintos con el
// mismo seq quedarian pisados.
function vistoYa(e){
  // EL ORIGEN VA EN LA CLAVE. Local y central numeran su `seq` por separado, asi que sin
  // esto dos eventos distintos con la misma marca y el mismo numero se pisan — y el que
  // llega segundo se descarta EN SILENCIO, que es la peor forma de perder un evento.
  const k=(e.origen||'central')+'|'+(e.seq||0)+'|'+(e.at||'');
  if(VIVO.vistos.has(k)) return true;
  VIVO.vistos.add(k);
  if(VIVO.vistos.size>VIVO_DEDUPE){ VIVO.vistos=new Set([...VIVO.vistos].slice(-VIVO_MAX*2)); }
  return false;
}

let _pollPedido=0;
// refrescarYa: adelanta el pulso cuando el cerebro dice que paso algo, en vez de esperar los 5 s
// del sondeo. Con techo: una rafaga de eventos pide UN pulso, no uno por evento.
function refrescarYa(){
  const ahora=performance.now();
  if(ahora-_pollPedido<2000) return;
  _pollPedido=ahora;
  setTimeout(()=>{ poll().catch(()=>{}); }, 350);   // margen para que la escritura ya este visible
}

function anotarEvento(e){
  if(!e || vistoYa(e)) return;
  if(e.kind==='sondeo'){ VIVO.sondeos.push(Date.now()); return; }
  if(e.perdidos>0) VIVO.nuevos.push({hueco:e.perdidos});
  VIVO.nuevos.push(e);
  // El grafo de codigo SI cambia de verdad con esto: se invalida para que se vuelva a bajar.
  if(e.tool==='musubi_codegraph_push') GRAPH.code=null;
  refrescarYa();
}

// hace: la antiguedad en la forma mas corta que sigue siendo exacta.
function hace(iso){
  const t=Date.parse(iso); if(!isFinite(t)) return '';
  const s=Math.max(0,Math.round((Date.now()-t)/1000));
  if(s<60) return s+'s';
  const m=Math.round(s/60); if(m<60) return m+'m';
  return Math.round(m/60)+'h';
}
// El prefijo `musubi_` esta en las 53 tools: repetirlo 40 veces gasta ancho de columna sin decir nada.
const cortaTool=t=>String(t||'').replace(/^musubi_/,'');

// nodoEvento arma UNA fila. El nodo se guarda la hora en un data- para poder refrescar despues
// solo el texto de la antiguedad, sin volver a armar nada.
function nodoEvento(e){
  const d=document.createElement('div');
  if(e.hueco){ d.className='ev hueco';
    d.innerHTML=`<span class="s"></span><span class="t">se perdieron ${e.hueco} eventos</span>`;
    return d; }
  // El origen se MUESTRA, no se deduce. Un riel que mezcla lo de esta maquina con lo de toda
  // la empresa sin decir cual es cual afirma algo falso. Los eventos locales no traen
  // `principal` —no hay token en stdio, no hay a quien distinguir— asi que ocupan ese lugar.
  const local = e.origen==='local';
  d.className='ev'+(local?' loc':'')+(e.outcome&&e.outcome!=='ok'?' err':'');
  d.dataset.at=e.at||'';
  d.innerHTML=`<span class="s"></span><span class="t">${esc(cortaTool(e.tool))}</span>`+
    (local?'<span class="q aqui">esta maquina</span>'
          :(e.principal?`<span class="q">${esc(e.principal)}</span>`:''))+
    `<span class="d">${hace(e.at)}</span>`;
  return d;
}

// pintarVivo INSERTA las filas nuevas; no reconstruye el riel.
//
// Antes reasignaba innerHTML entero, y eso se pagaba dos veces por segundo de dos formas visibles:
// el contenedor quedaba un instante sin hijos, su alto colapsaba, y la linea del separador de
// abajo saltaba hacia arriba y volvia — un parpadeo gris, una vez por segundo; y la animacion de
// entrada se re-disparaba en TODAS las filas a la vez, no solo en la que acababa de llegar.
// Insertando, la animacion queda donde tiene sentido: en lo que de verdad es nuevo.
function pintarVivo(){
  const cont=$('vivo'); if(!cont) return;
  if(VIVO.nuevos.length){
    const vacio=cont.querySelector('.empty'); if(vacio) vacio.remove();
    // Llegan de mas viejo a mas nuevo; cada uno va al frente, asi el ultimo queda arriba.
    for(const e of VIVO.nuevos) cont.insertBefore(nodoEvento(e), cont.firstChild);
    VIVO.nuevos.length=0;
    while(cont.children.length>VIVO_MAX) cont.removeChild(cont.lastChild);
  }
  if(!cont.children.length) cont.innerHTML='<div class="empty">sin trabajo todavia</div>';
  pintarEnlace();
}

function pintarEnlace(){
  const en=$('enlace'); if(!en) return; const s=VIVO.enlace;
  const cls='enl'+(s.estado==='conectado'?' ok':(s.estado==='conectando'?'':' mal'));
  const txt = s.estado==='conectado' ? (s.destino||'enlazado')
    : s.estado==='conectando' ? 'conectando…'
    : s.estado==='apagado' ? 'apagado' : 'sin enlace';
  // Escribir solo lo que cambio: asignar el mismo textContent igual invalida el layout del nodo.
  if(en.className!==cls) en.className=cls;
  if(en.textContent!==txt) en.textContent=txt;
  const tt=s.detalle||''; if(en.title!==tt) en.title=tt;
}

// tictac corre a 1 Hz y NO toca la estructura: reescribe el texto de la antiguedad solo en las
// filas donde de verdad cambio, y el contador de sondeo. Con la antiguedad en unidades gruesas
// (s -> m -> h), la enorme mayoria de los segundos no cambia ni una fila.
function tictac(){
  const cont=$('vivo');
  if(cont) for(const n of cont.children){
    const at=n.dataset&&n.dataset.at; if(!at) continue;
    const d=n.lastElementChild; if(!d||!d.classList.contains('d')) continue;
    const t=hace(at); if(d.textContent!==t) d.textContent=t;
  }
  // El sondeo, agregado: cuantos por minuto. Es la unica forma honesta de mostrar el 99% del
  // trafico sin que tape el 1% que importa. Ventana rodante, por eso se recalcula cada segundo.
  const corte=Date.now()-60000;
  VIVO.sondeos=VIVO.sondeos.filter(t=>t>corte);
  const lat=$('latido'), txt=$('latidoTxt');
  if(lat&&txt){ const n=VIVO.sondeos.length;
    const s=n>0?`sondeo · ${n}/min`:'sin sondeo';
    lat.classList.toggle('vive', n>0);
    if(txt.textContent!==s) txt.textContent=s;
  }
}
setInterval(tictac, 1000);

// impulsar: de UNA invocacion real del riel a UN impulso en la lente personas. Es el unico
// camino por el que nace un pulso; no hay bucle que los fabrique.
//
// NO se llama desde el `backlog`, y eso es deliberado: el backlog son eventos que YA pasaron
// (la corrida medida traia 230 de golpe al conectar). Dispararlos como impulsos seria mostrar
// como presente algo pasado, que es justo lo que este rediseño vino a sacar.
//
// Tampoco se acumulan con la lente apagada: `reloj` no avanza mientras la vista no esta viva, y
// los pulsos encolados ahi saldrian todos juntos al volver, mintiendo sobre cuando ocurrieron.
function impulsar(ev){
  if(!ev || !ev.tool) return;
  // El MAPA manda. Se arma al construir el grafo y sale del censo: sin el, un servicio no
  // tiene donde caer y el evento se cuenta como "sin neurona" en vez de repartirse a dedo.
  // DOS AUSENCIAS DISTINTAS, y confundirlas manda a una acción imposible. Un evento sin
  // `principal` viene del spool de ESTA máquina, donde el stdio no tiene credencial: no hay dueño
  // que declarar. Uno CON principal y sin neurona sí es un dueño sin declarar. Medido en una
  // ventana de 40 s: 8 eventos locales sin credencial y 0 principales sin neurona — o sea el
  // contador decía «falta declarar su dueño» para los 8 casos donde eso no aplica.
  if(!String(ev.principal||'').trim()) SIN_CREDENCIAL++;
  const n=neuronaDeEvento(ev, ENCENDIDO), c=clasificarEvento(ev);
  const pu = n ? {terminal:n.terminal, exacta:n.exacta, capa:c.capa, falla:c.falla, ms:c.ms}
               : {terminal:'', capa:c.capa, falla:c.falla, ms:c.ms};
  // La escena principal. El reloj es el MISMO que usa animate(): si el impulso naciera con otro,
  // el frente arrancaría corrido y en el peor caso ya vencido.
  //
  // Enciende TODAS las neuronas del racimo de esa persona, no una. Con las neuronas siendo tramos
  // del árbol de temas, no hay forma de saber en cuál cae una llamada a una tool: lo que el evento
  // dice con certeza es de quién es. Elegir una sería inventarlo.
  const ahora = performance.now()/1000;
  const ns = n ? (NEURONAS_DE.get(RACIMO_DE.get(n.terminal)) || null) : null;
  if(ns && ns.length) for(const ti of ns) IMPULSOS.nacer(ti, pu, ahora);
  else IMPULSOS.nacer(-1, pu, ahora);
}

function conectarVivo(){
  // EventSource y no fetch: el stream local es same-origin sobre loopback y no lleva credencial
  // (el bearer se queda en el relay), asi que se puede usar el que reconecta solo. El del CEREBRO
  // sí necesita header, y por eso ese lo abre el relay con fetch — ver livestream.go.
  let es;
  try{ es=new EventSource('/api/stream'); }catch(_){ return; }
  es.addEventListener('backlog', m=>{ try{ (JSON.parse(m.data)||[]).forEach(anotarEvento); }catch(_){} pintarVivo(); });
  es.addEventListener('uso', m=>{ try{ const ev=JSON.parse(m.data); anotarEvento(ev); impulsar(ev); }catch(_){} pintarVivo(); });
  es.addEventListener('enlace', m=>{ try{ VIVO.enlace=JSON.parse(m.data); pintarVivo(); }catch(_){} });
  es.onerror=()=>{ // EventSource reconecta solo; lo unico que hace falta es no mentir mientras tanto
    if(VIVO.enlace.estado!=='apagado'){ VIVO.enlace={estado:'caido',detalle:'se corto el stream del panel'}; pintarVivo(); }
  };
}

/* ---------- resize / poll / init ---------- */
function resize(){ const w=innerWidth,h=innerHeight; renderer.setSize(w,h); camera.aspect=w/h; camera.updateProjectionMatrix();
  renderer.getDrawingBufferSize(_dbs); composer.setSize(_dbs.x,_dbs.y); controls.handleResize(); }
addEventListener('resize',resize); resize();

/* ---------- datos: el GRAFO se baja aparte del PULSO ----------
 *
 * Antes esto pedía /api/snapshot entero cada 5 s. Con el tope en 300 neuronas eran 481 KB por
 * tick; sin tope habrían sido 2,3 MB, o sea 117 s por pedido a través de un túnel — nunca
 * terminaba uno y el grafo se veía apagado porque la actividad sólo se enciende con datos
 * frescos. Ese acoplamiento era el motivo real del tope.
 *
 * Ahora: el GRAFO se baja una vez (/api/graph, sin tope) y el PULSO (18 KB) trae cada 5 s los
 * contadores y los DELTAS. Las memorias nuevas llegan enteras en el pulso y se AGREGAN al grafo
 * cacheado, así que guardar algo no fuerza re-bajar nada. Sólo se vuelve a bajar el grafo si el
 * conteo del server no coincide con lo que tenemos — es decir, cuando algo DESAPARECIÓ
 * (archivado, superseded, cuarentena), que es raro y no se puede reconstruir con un delta.
 */
const GRAPH={memory:null, code:null};
let PULSE=null, since=null;

// Un pedido de grafo EN VUELO por lente, compartido. Dos caminos pueden pedir el mismo grafo a
// la vez —el arranque en paralelo y el primer poll— y bajarlo dos veces son 1,6 MB y dos ranuras
// de conexión al pedo, justo cuando las ranuras son el recurso escaso.
const _bajando={};
async function fetchGraph(which){
  if(_bajando[which]) return _bajando[which];
  _bajando[which]=(async()=>{
    try{
      const r=await fetch('/api/graph?lens='+which,{cache:'no-store'});
      if(!r.ok) throw 0;
      GRAPH[which]=await r.json();
    } finally { _bajando[which]=null; }
  })();
  return _bajando[which];
}

// fetchCenso: baja el CENSO DE ACTORES (quien llama al cerebro y cuanto).
//
// Va por su propio camino y no por el pulso a proposito: el volumen historico de cada actor
// cambia en horas, y meterlo en un sondeo de 5 s seria pagarlo 720 veces por hora para ver el
// mismo numero. El servidor ademas lo cachea 60 s (ver cmd/musubi/actores.go).
//
// NUNCA TIRA. Un censo que no llega deja `estado` diciendo por que, y la lente dibuja las
// terminales sola — que es exactamente como se veia antes de que el censo existiera.
async function fetchCenso(){
  if(_bajandoCenso) return _bajandoCenso;
  _bajandoCenso=(async()=>{
    try{
      const r=await fetch('/api/actores',{cache:'no-store'});
      if(!r.ok) throw 0;
      const j=await r.json();
      // El cuerpo del central viaja anidado en `censo` como texto JSON crudo. Se desanida aca
      // para que el resto del panel vea una sola forma.
      CENSO={ estado:j.estado, detalle:j.detalle||'', destino:j.destino||'',
              censo:j.censo||null };
    }catch(_){
      CENSO={ estado:'caido', detalle:'el panel no pudo pedir el censo', censo:null };
    } finally { _bajandoCenso=null; }
  })();
  return _bajandoCenso;
}

// aplicaDeltas: mete lo que trajo el pulso en el grafo cacheado. No toca el render ni la
// actividad — de eso se sigue encargando buildGraph, que diffea contra su propio estado
// anterior. Devuelve si el cache quedó consistente con lo que declara el server.
function aplicaDeltas(g,p){
  if(!g) return false;
  if(p.touched && p.touched.length){
    const byId=new Map(g.neurons.map((n,i)=>[n.id,i]));
    for(const t of p.touched){ const i=byId.get(t.id); if(i==null) continue;
      g.neurons[i].heat=t.heat; g.neurons[i].recency_days=t.recency_days; }
  }
  if(p.new_neurons && p.new_neurons.length){
    const vistos=new Set(g.neurons.map(n=>n.id));
    for(const n of p.new_neurons) if(!vistos.has(n.id)){ g.neurons.push(n); vistos.add(n.id); }
  }
  if(p.new_synapses && p.new_synapses.length){
    const vistas=new Set(g.synapses.map(s=>s.source+'|'+s.target));
    for(const s of p.new_synapses){ const k=s.source+'|'+s.target; if(!vistas.has(k)){ g.synapses.push(s); vistas.add(k); } }
  }
  g.total_neurons=p.counts.neurons; g.total_synapses=p.counts.synapses;
  g.truncated=g.neurons.length<g.total_neurons;
  g.synapses_truncated=g.synapses.length<g.total_synapses;
  return g.neurons.length===p.counts.neurons;
}

// hudShape: compone para renderHUD la MISMA forma que tenía /api/snapshot, así el HUD no se
// entera de que los datos ahora llegan por dos caminos.
function hudShape(){
  const p=PULSE||{};
  return { brain:GRAPH.memory||{neurons:[],synapses:[]}, code:GRAPH.code||{nodes:[],edges:[]},
    insights:p.insights||{}, graph:{ domains:p.domains||[], total_observations:(p.counts||{}).neurons||0 },
    health:p.health||{}, tokens:p.tokens||{}, recent:p.recent||[], orchestration:p.orchestration||{},
    project:p.project, version:p.version };
}

// EL PANEL SE AHOGABA SOLO, y este guardia es el arreglo. `/api/pulse` corre el diagnóstico
// completo del cerebro; sobre una base de 54 MB con 56 MB de WAL eso MIDE 45-51 s. Con un
// setInterval de 5 s se lanzaba un pedido nuevo antes de que volviera el anterior: a los 30 s ya
// había seis en vuelo, que es el tope de conexiones por origen de un navegador, y desde ahí TODO
// fetch quedaba encolado para siempre. El síntoma no era "lento": era el panel entero en «—» sin
// un solo error en consola, porque las promesas no se resolvían ni fallaban. Medido: 8 conexiones
// ESTABLISHED contra el panel y ningún poll completado.
let sondeando=false;
async function poll(){
  if(sondeando) return;
  sondeando=true;
  try{
    const r=await fetch('/api/pulse'+(since?('?since='+encodeURIComponent(since)):''),{cache:'no-store'});
    if(!r.ok) throw 0;
    PULSE=await r.json();
    if(lens==='code'){ if(!GRAPH.code) await fetchGraph('code'); }
    else if(!GRAPH.memory || !aplicaDeltas(GRAPH.memory,PULSE)) await fetchGraph('memory');
    since=PULSE.now;
    renderLens(); renderHUD(hudShape());
    $('liveTxt').textContent='en vivo';
  }catch(e){ $('liveTxt').textContent='reconectando'; }
  finally{ sondeando=false; }
}

// CONSTRUIDO recuerda DE QUE OBJETO se armo el grafo que se esta dibujando.
//
// La lente CODIGO se baja una sola vez y no cambia entre pulsos, pero renderLens() la reconstruia
// en CADA poll: 8362 nodos, 17661 aristas, sus listas de adyacencia y dos Map grandes, todo desde
// datos identicos. Medido con el cuerpo real de buildCodeGraph a esa escala: 28,8 ms cada 5
// segundos —casi dos frames— mas la basura de ~26.000 objetos que el GC recoge cuando le toca.
// Ese era el tiron periodico en la lente codigo. En memoria son 6,7 ms, y por eso alli no se nota.
//
// SOLO SE PUEDE SALTEAR CODIGO, y la asimetria no es un detalle:
//   - fetchGraph() ASIGNA un objeto nuevo a GRAPH[lente] ⇒ para codigo, identidad distinta
//     significa exactamente "hay grafo nuevo".
//   - aplicaDeltas() MUTA g.neurons EN EL LUGAR ⇒ para memoria el objeto es el MISMO aunque el
//     contenido cambio. Comparar por identidad alli saltearia una reconstruccion necesaria, y
//     ademas buildGraph diffea heat/recencia adentro para encender la actividad: saltearla
//     apagaria el latido del cerebro, que es la razon de ser del panel.
let CONSTRUIDO=null;

// renderLens: reconstruye el grafo con la lente activa desde lo que hay en cache (sin re-pollear).
function renderLens(){
  if(lens==='code'){
    if(!GRAPH.code) return;
    if(CONSTRUIDO===GRAPH.code) return;   // mismo objeto ⇒ mismo grafo ⇒ no hay nada que rehacer
    buildCodeGraph(GRAPH.code); CONSTRUIDO=GRAPH.code; return;
  }
  // Al volver a memoria, CONSTRUIDO pasa a apuntar al grafo de memoria: si despues se vuelve a
  // codigo, la comparacion falla y se reconstruye — que es lo correcto, porque NEURONS quedo con
  // el grafo de la otra lente.
  if(GRAPH.memory){ buildGraph(GRAPH.memory); CONSTRUIDO=GRAPH.memory; }
}

function setMotion(v){ motion=v; const b=$('motionBtn'); if(b){ b.textContent=motion?'❚❚ pausar':'▶ reanudar'; b.classList.toggle('paused',!motion); b.setAttribute('aria-pressed',String(!motion)); } }
$('motionBtn').addEventListener('click',()=>setMotion(!motion)); setMotion(motion);

// setLens: conmuta memoria↔código. Son DOS y no tres: la lente de personas aparte se retiró
// cuando sus dos piezas —los árboles y el impulso— pasaron a la escena principal. Tenerla al
// lado habría sido una tercera vista contando lo mismo, y dos lugares donde arreglar cada cosa.
function setLens(v){ lens=v; AMP = lens==='code' ? AMP_CODIGO : 0;
  // Cada lente se encuadra sola. Sin esto, la primera que se abre fija la camara y la otra hereda
  // una distancia pensada para un dibujo con otra forma.
  framed=false;
  const b=$('lensBtn');
  // Cambiar de lente SIEMPRE rehace las mallas. El disparador de abajo compara cantidades, y dos
  // grafos distintos con la misma cantidad de nodos no se distinguen ahí: es poco probable, pero
  // el modo de falla es dibujar un grafo con las mallas del otro.
  needsRebuild=true;
  if(b){ b.textContent = lens==='code'?'◉ código':'◉ memoria';
    b.classList.toggle('code',lens==='code');
    b.setAttribute('aria-pressed',String(lens!=='memory')); }
  applyLensLabels();
  // La lente de código se baja on-demand: no viaja en el pulso ni tiene por qué estar en
  // memoria si nadie la miró. Si falta, se pide y recién ahí se dibuja.
  if(lens==='code' && !GRAPH.code){ fetchGraph('code').then(()=>{ renderLens(); if(PULSE) renderHUD(hudShape()); }).catch(()=>{}); return; }
  renderLens(); if(PULSE) renderHUD(hudShape()); }
// applyLensLabels: intercambia los textos estáticos del HUD según la lente (leyenda de aristas,
// títulos, guía). Se llama al togglear, no en cada poll.
function applyLensLabels(){ const code=lens==='code';
  const set=(id,t)=>{ const e=$(id); if(e) e.textContent=t; };
  // Los contadores de la cabecera describen la MEMORIA: es el universo del que sale este grafo,
  // y renombrarlos por lo que se dibuja encima sería mentir con datos ciertos.
  set('lblNodes', code?'nodos':'neuronas'); set('lblEdges', code?'aristas':'sinapsis');
  set('domTitle', code?'Módulos':'Personas');
  set('lblActive', code?'Nodos':'Memorias activas');
  set('lblSyn', code?'Aristas':'Sinapsis');
  set('lblDomains', code?'Módulos':'Personas');
  pistas(`<span><b>arrastrá</b> para rotar</span><span class="sep">·</span>`+
    `<span><b>rueda</b> para acercar</span><span class="sep">·</span>`+
    `<span><b>hover</b> revela el detalle</span>`);
  const al=$('actlegend'); if(al) al.innerHTML = code
    ? `<div class="lg"><span class="sw" style="background:${EDGEKIND.CALLS};color:${EDGEKIND.CALLS}"></span>llama</div><div class="lg"><span class="sw" style="background:${EDGEKIND.IMPORTS};color:${EDGEKIND.IMPORTS}"></span>importa</div><div class="lg"><span class="sw" style="background:${EDGEKIND.CONTAINS};color:${EDGEKIND.CONTAINS}"></span>contiene</div>`
    : `<div class="lg"><span class="sw" style="background:#7f9cc9;color:#7f9cc9"></span>reposo</div><div class="lg"><span class="sw" style="background:#43e08b;color:#43e08b"></span>escribir</div><div class="lg"><span class="sw" style="background:#31c9ff;color:#31c9ff"></span>recordar</div><div class="lg"><span class="sw" style="background:#f5c451;color:#f5c451"></span>relacionar</div>`+
      // El IMPULSO va rotulado APARTE de la actividad de la memoria. Son dos lenguajes distintos
      // sobre la misma escena —lo que le pasa a una NOTA contra quién LLAMÓ al cerebro— y sin el
      // rótulo el ámbar aparece dos veces queriendo decir dos cosas.
      `<div class="lg" style="opacity:.5;margin-left:6px">impulso:</div>`+
      `<div class="lg"><span class="sw" style="background:rgba(255,255,255,.32);color:rgba(255,255,255,.32)"></span>sondeo</div>`+
      `<div class="lg"><span class="sw" style="background:#fff;color:#fff"></span>trabajo real</div>`+
      `<div class="lg"><span class="sw" style="background:#ff9905;color:#ff9905"></span>falló</div>`;
  const ht=$('howto'); if(ht) ht.innerHTML = code
    ? `<span><b>·</b> cada punto es un <b>símbolo</b> (función, tipo, archivo)</span><span><b>·</b> las líneas son <b>llamadas / imports</b>; el color agrupa por <b>módulo</b></span><span><b>·</b> el <b>tamaño</b> = centralidad · <b>hover</b> muestra qué memorias lo explican</span>`
    : `<span><b>·</b> cada punto es una <b>memoria</b></span>`+
      `<span><b>·</b> las líneas, <b>relaciones</b>; la luz que viaja = <b>recuerdo activándose</b></span>`+
      `<span><b>·</b> el <b>racimo</b> y el <b>color</b> agrupan por <b>quién la escribió</b> · gris = no es de una persona</span>`+
      `<span><b>·</b> la <b>neurona ramificada</b> de cada racimo es una <b>terminal</b>; sus dendritas, cuánto escribió</span>`+
      `<span><b>·</b> el <b>impulso</b> que la recorre es <b>UNA llamada real a una tool</b>, en el momento en que ocurre. <b>Sin evento no hay luz</b>: si el cerebro está quieto, esto está quieto</span>`;
}
// tipNeurona: qué es este árbol. Dice CUÁNTAS memorias carga y POR QUÉ están juntas — que es la
// pregunta que el dibujo nuevo abre: si la rama significa algo, hay que poder leer qué.
const COMO={ tema:'mismo tema', tiempo:'misma época', reparto:'reparto', fundido:'fundido con su vecina', orden:'sin criterio (mismo tema y misma fecha)' };
function tipNeurona(nu, px, py){
  const tip=document.getElementById('tip'); if(!tip) return;
  tip.querySelector('.tt').textContent = nu.etiqueta || nu.racimo || 'neurona';
  tip.querySelector('.tg').innerHTML =
    `<b>${nu.memorias}</b> memorias · racimo <b>${esc(nu.racimo)}</b>`;
  tip.querySelector('.tm').innerHTML =
    `<i>agrupadas por ${esc(COMO[nu.criterio]||nu.criterio)}</i>`+
    `<i>${nu.segs.length} ramas</i><i>alcance ${Math.round(nu.alcance)}</i>`;
  const tw=tip.offsetWidth||240, th=tip.offsetHeight||80;
  const ax=(typeof px==='number'?px:mx), ay=(typeof py==='number'?py:my);
  let x=ax+16, y=ay+16;
  if(x+tw>innerWidth-8) x=ax-tw-16;
  if(y+th>innerHeight-8) y=ay-th-16;
  tip.style.left=x+'px'; tip.style.top=y+'px'; tip.classList.add('on');
}

// tipTerminal: el detalle de una terminal al pasarle el mouse por su SOMA.
//
// Vivía como callback de la vista 2D. Ahora lo llama `hover()` con el mismo `#tip` que usan las
// memorias: son la misma pieza de UI y duplicarla es garantizar que se despeguen.
function tipTerminal(nd, px, py) {
  const tip = document.getElementById('tip');
  if (!tip) return;
  if (!nd) { tip.classList.remove('on'); return; }
  const D = (PERSONAS && PERSONAS.despachos) || [];
  const lista = (arr, dir) => arr.length
    ? arr.sort((a,b)=>b.veces-a.veces).slice(0,4).map(d=>`${esc((dir==='sale'?d.a:d.de).toLowerCase())} <b>${d.veces}</b>`).join(' · ')
    : '—';
  const salen = D.filter(d => d.de === nd.id), entran = D.filter(d => d.a === nd.id);
  const num = (n) => (n||0).toLocaleString('es');
  tip.querySelector('.tt').textContent = nd.id.toLowerCase();
  if (nd.tipo === 'actor') {
    // Un ACTOR no tiene notas ni firmas: no escribe. Mostrarle esos campos en cero seria
    // afirmar que escribio poco, cuando lo cierto es que no es de los que escriben.
    const L = nd.llamadas || {};
    tip.querySelector('.tg').innerHTML = nd.exacta === false
      ? `credencial de <b>${esc(nd.persona)}</b> · por convención del nombre, sin declarar`
      : `servicio · <b>dueño no declarado</b>${L.proyecto ? ` · proyecto ${esc(L.proyecto)}` : ''}`;
    tip.querySelector('.tm').innerHTML =
      `<i>${num(L.calls)} llamadas</i><i>${num(L.trabajo)} trabajo · ${num(L.sondeo)} sondeo</i><i>${L.tools||0} tools distintas</i>`;
  } else {
    // Se dice `notas` y `firma` por separado a propósito: son cosas distintas y confundirlas fue
    // el error que hubo que corregir para saber de quién es cada terminal.
    tip.querySelector('.tg').innerHTML =
      `de <b>${esc(nd.persona || 'sin autor')}</b> · ${nd.notas} notas la nombran · ${nd.firmas} las firma`;
    // Las llamadas van al final y sólo si el censo llegó: son la OTRA naturaleza de la misma
    // identidad —escribe y ademas llama— y es el numero que explica por que la neurona que mas
    // late puede ser la que menos escribio.
    const L = nd.llamadas;
    tip.querySelector('.tm').innerHTML =
      `<i>calor ${nd.calor}</i>${L?`<i>${num(L.calls)} llamadas · ${num(L.trabajo)} trabajo</i>`:''}` +
      `<i>escribe a: ${lista(salen,'sale')}</i><i>recibe de: ${lista(entran,'entra')}</i>`;
  }
  // La posicion la manda la vista con el evento que la origino. El `mx/my` global lo
  // actualiza otro listener y no siempre corrio antes: medido, el tooltip aparecia en
  // (16,16) tapando la cabecera.
  const tw = tip.offsetWidth || 240, th = tip.offsetHeight || 80;
  const ax = (typeof px === 'number' ? px : mx), ay = (typeof py === 'number' ? py : my);
  let x = ax + 16, y = ay + 16;
  if (x + tw > innerWidth - 8) x = ax - tw - 16;
  if (y + th > innerHeight - 8) y = ay - th - 16;
  tip.style.left = x + 'px'; tip.style.top = y + 'px'; tip.classList.add('on');
}

// pistas: la barra de abajo dice qué se puede HACER, y eso cambia con la lente. La de personas
// tiene desplazamiento y doble click, que no existen en las otras dos: dejar el texto viejo es
// esconder los dos gestos que hacen falta para llegar a una neurona chica.
function pistas(html){ const e=$('pistas'); if(e) e.innerHTML=html; }

// pintarLeyendaRacimos: quién es quién, con EL MISMO color con que se dibujó su racimo — sale
// de DOMAINS, que es lo que la escena usó para pintar, y no de una paleta paralela que podía
// desincronizarse y poner un color en la leyenda y otro en el dibujo.
//
// Debajo van las declaraciones. TODAS estas líneas vivían en la lente aparte, o sea que hasta hoy
// sólo las veía quien se acordaba de cambiar de vista: cuántas notas no tienen autor, cuántas
// credenciales no tienen dueño declarado, y si el censo llegó. Sin ellas una muestra parcial se
// lee como un total, y un cerebro apagado se dibuja igual que uno sin actores.
function pintarLeyendaRacimos(legend){
  const dl=$('domlegend'); if(!dl) return;
  const filas=(legend||[]).map(dd=>
    `<div class="lg"><span class="sw" style="background:${dd.color};color:${dd.color}"></span>${esc(dd.name)} <b>${dd.count}</b></div>`);
  if(!filas.length) filas.push('<div class="empty">sin racimos</div>');
  const linea=(t)=>filas.push(`<div class="lg" style="opacity:.55">${t}</div>`);
  // EL ESTADO DEL CENSO, cuando no está vivo. Sin esta línea, un cerebro apagado y un cerebro sin
  // actores se dibujan igual — y la única diferencia entre los dos es si lo que ves es la verdad
  // o una pantalla que nunca preguntó.
  if(CENSO && CENSO.estado && CENSO.estado!=='vivo')
    filas.push(`<div class="lg" style="opacity:.7">censo de actores ${esc(CENSO.estado)} · ${esc(CENSO.detalle||'')}</div>`);
  const acts=(PERSONAS&&PERSONAS.actores)||[];
  if(acts.length) linea(`${acts.length} actores ◯ · ${acts.reduce((t,a)=>t+a.calls,0).toLocaleString('es')} llamadas`);
  const sd=PERSONAS?PERSONAS.sinDeclarar||0:0;
  if(sd) linea(`${sd} sin dueño declarado · no encienden neurona`);
  const ds=PERSONAS?PERSONAS.despachos:null;
  // Los DESPACHOS son los mensajes y los PARES son las flechas. Decir "27 despachos" cuando hay
  // 27 pares que suman 140 mensajes divide la cifra real por cinco.
  if(ds&&ds.length) linea(`${ds.length} pares se escriben · ${ds.reduce((t,d)=>t+d.veces,0)} despachos`);
  const sa=PERSONAS?PERSONAS.sinAutor:0;
  if(sa) linea(`${sa} notas sin autor · no se reparten`);
  // Los eventos que no encontraron neurona SE DECLARAN. Repartirlos a la neurona de otro para que
  // la pantalla no quede quieta sería exactamente el tipo de invento que este rediseño vino a sacar.
  // Y son DOS ausencias distintas: un evento sin `principal` viene del spool de esta máquina, donde
  // el stdio no lleva credencial — no hay dueño que declarar. Uno CON principal y sin neurona sí.
  if(SIN_CREDENCIAL) linea(`${SIN_CREDENCIAL} eventos de esta máquina · sin credencial, no se atribuyen`);
  const sinDueno=Math.max(0, IMPULSOS.cuenta().sinTronco - SIN_CREDENCIAL);
  if(sinDueno) linea(`${sinDueno} eventos sin neurona · falta declarar su dueño`);
  dl.innerHTML=filas.join('');
}
const lensBtn=$('lensBtn'); if(lensBtn) lensBtn.addEventListener('click',()=>
  setLens(lens==='memory' ? 'code' : 'memory'));

// La lente se puede fijar por URL (?lens=code). Sirve para dos cosas concretas: que el CRM pueda
// enlazar directo a una vista, y que una captura automatica pueda verificar una lente sin simular
// un click. Un valor desconocido se ignora y queda `memory` — incluido el viejo `?lens=personas`,
// que ahora ES la lente de memoria y por eso no necesita redirigir a ningun lado.
applyLensLabels();   // la primera carga también: el HTML trae los rótulos de la lente memoria
const _lensURL=new URLSearchParams(location.search).get('lens');
if(_lensURL==='code') setLens('code');

// El GRAFO no depende del PULSO, así que se pide EN PARALELO en vez de esperar a que el pulso
// vuelva. Con el pulso en ~50 s y el grafo en ~9 s, la diferencia es ver el dibujo a los nueve
// segundos en vez de tener la pantalla vacía casi un minuto. El HUD (contadores, salud, riel) sí
// necesita el pulso y llena cuando llega.
fetchGraph(lens==='code'?'code':'memory').then(()=>{ renderLens(); }).catch(()=>{});
// EL CENSO SE PIDE AL ARRANCAR, y ya no sólo al entrar a una lente. Dejó de ser un adorno de una
// vista: es lo que traduce el `principal` de un evento en la terminal que tiene que encenderse.
// Sin él, todo lo que llame con credencial de servicio cae como «sin neurona».
fetchCenso().then(()=>{ refrescarEncendido(); }).catch(()=>{});
poll(); setInterval(poll,5000);
conectarVivo();
requestAnimationFrame(animate);
