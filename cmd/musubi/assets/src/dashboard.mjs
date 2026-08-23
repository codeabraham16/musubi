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
import { extraerPersonas, agruparPorPersona } from './personas.mjs';
import { crearVista, colorPersona, COLOR_DESPACHO, COLOR_CRUCE } from './personasview.mjs';
import { iterParaCambio, settleStart, settleTick, settlePendiente } from './layout.mjs';

/* ---------- paletas ---------- */
const DOMPAL=['#2dd4bf','#a78bfa','#fbbf24','#4ade80','#38bdf8','#f472b6','#fb923c','#f87171','#a3e635','#22d3ee','#e879f9','#facc15'];
const RELCOL={ conflicts_with:'#f87171', supersedes:'#a78bfa', scoped:'#38bdf8', related:'#2dd4bf', compatible:'#4ade80', not_conflict:'#64748b' };
// color por TIPO DE ACTIVIDAD (ancla a señales reales): 0 reposo · 1 escribir · 2 recordar · 3 relacionar
const AK=['#7f9cc9','#43e08b','#31c9ff','#f5c451'];
const REPOSO=AK[0];
// LENTE CÓDIGO: color de arista por TIPO (llama / importa / contiene). En reposo cada tubo toma su color.
const EDGEKIND={ CALLS:'#38bdf8', IMPORTS:'#a78bfa', CONTAINS:'#5b6b86' };
const edgeBase=s=>(s&&s.kind&&EDGEKIND[s.kind])||REPOSO;
let DOMCOL=new Map(), DOMAINS=[];
const domColor=d=>DOMCOL.get(d)||'#64748b';
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
const VISTA_PERSONAS = crearVista(document.getElementById('personas'));
// Lo último que se extrajo, para que el HUD pinte la MISMA verdad que el lienzo. Vive acá y no
// dentro de renderLens porque renderHUD corre en cada poll (cada 5 s) y necesita leerlo.
let RACIMOS = [], PERSONAS = null;

// buildGraph: PORTADO del dashboard anterior. Detecta ACTIVIDAD REAL diffeando el snapshot y la tipa
// (escribir/recordar/relacionar); corre el force-sim si cambió la topología. Marca needsRebuild para
// que la escena three recree las mallas cuando cambian nodos/aristas.
function buildGraph(brain){
  const prev=POS.memory, ns0=brain.neurons||[];
  const N0=240; growth=Math.max(0.85, Math.min(2.2, Math.cbrt((ns0.length||1)/N0))); applyScale();
  const counts={}; ns0.forEach(n=>counts[n.domain]=(counts[n.domain]||0)+1);
  const doms=Object.keys(counts).sort((a,b)=>counts[b]-counts[a]||a.localeCompare(b));
  DOMCOL=new Map(); DOMAINS=[]; const dIdx=new Map();
  doms.forEach((d,i)=>{ const col=DOMPAL[i%DOMPAL.length]; DOMCOL.set(d,col); dIdx.set(d,i);
    const k=i+0.5, phi=Math.acos(1-2*k/Math.max(doms.length,1)), th=Math.PI*(1+Math.sqrt(5))*k;
    DOMAINS.push({name:d,color:col,count:counts[d], ax:Math.cos(th)*Math.sin(phi)*rx*0.52, ay:Math.cos(phi)*ry*0.52, az:Math.sin(th)*Math.sin(phi)*rz*0.52}); });

  // los ids previos DE ESTA LENTE, no los de NEURONS (que todavia tiene el grafo de la otra).
  const prevIds=new Set(prev.keys());
  NEURONS=ns0.map(n=>{ const p=prev.get(n.id); const base=p?{x:p.x,y:p.y,z:p.z}:randInBrain();
    const r=Math.max(0.9, Math.min(6.0, 0.9+Math.sqrt(Math.max(n.importance,0))*0.72+Math.log(1+(n.heat||0))*0.38)); // tamaño del prototipo (más chico)
    const rec=Math.max(0.10, Math.min(1, 1-(n.recency_days||0)/45));
    return {...n, x:base.x,y:base.y,z:base.z, vx:0,vy:0,vz:0, r, rec, col:domColor(n.domain),
      ph:(p&&p.ph!=null)?p.ph:Math.random()*6.283, phx:Math.random()*6.283, phz:Math.random()*6.283,
      di:dIdx.has(n.domain)?dIdx.get(n.domain):-1, act:0, ak:0, adj:[], _new:!p}; });
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
  POS.memory=new Map(NEURONS.map(n=>[n.id,{x:n.x,y:n.y,z:n.z,ph:n.ph}]));
  const its=iterParaCambio(NEURONS.length, nuevos, ASENTADO.memory);
  if(its>0){ arrancarAsentado(its,'memory'); needsRebuild=true; }
  else if(changed) needsRebuild=true;   // cambio la topologia sin nodos nuevos: recrear las mallas
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
// Estado para no rehacer trabajo que no cambio. NHOT/EHOT recuerdan si el color escrito para esa
// instancia es el TENIDO POR ACTIVIDAD, para poder devolverlo al base UNA vez al apagarse en vez
// de reescribirlo todos los frames. `resto` es el residuo del arrastre y `actViva` si hay pulso.
let NHOT=null, EHOT=null, resto=0, actViva=false, lastThink=-1;
// _v/_up/_q se fueron con el quaternion de las aristas: ya nadie los usa.
const _m=new THREE.Matrix4(), _c=new THREE.Color(), _c2=new THREE.Color(), _pos=new THREE.Vector3(), _scl=new THREE.Vector3(), _eu=new THREE.Euler();
let framed=false;

function disposeMeshes(){ if(inst){ world.remove(inst); inst.geometry.dispose(); inst=null; }
  if(edgeInst){ world.remove(edgeInst); edgeInst.geometry.dispose(); edgeInst=null; }
  if(edgeMat){ edgeMat.dispose(); edgeMat=null; }
  ECOL=ESPD=EGLW=EBAS=null; }

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
  if(!framed && N){ let mr=0; for(const n of NEURONS){ const d=Math.hypot(n.x,n.y,n.z); if(d>mr)mr=d; } camera.position.set(0,20,Math.max(240,mr*2.7)); framed=true; }
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
function hover(){ if(drag>=0 || !inst){ tip.classList.remove('on'); return; } ray.setFromCamera(ptr,camera); const hit=ray.intersectObject(inst);
  if(hit.length){ const n=NEURONS[hit[0].instanceId]; tip.querySelector('.tt').textContent=n.topic||n.domain;
    if(n._code){
      fetchExplain(n);   // trae (una vez, on-demand) las memorias que EXPLICAN este símbolo
      let tg=esc(n.gist||'(sin ruta)');
      if(Array.isArray(n._exp) && n._exp.length) tg+='<div class="why">'+n._exp.slice(0,3).map(e=>`<span>${esc(e.topic_key)}</span>`).join('')+'</div>';
      tip.querySelector('.tg').innerHTML=tg;
      let meta=`<i>${esc(n.mem_type||'')}</i><i>${esc(n.domain)}</i><i>centralidad ${n.heat}</i>`;
      if(Array.isArray(n._exp)) meta+= n._exp.length?`<i style="color:var(--purple)">explicado ×${n._exp.length}</i>`:`<i style="opacity:.55">sin memorias</i>`;
      tip.querySelector('.tm').innerHTML=meta;
    } else {
      tip.querySelector('.tg').textContent=n.gist||'(sin resumen)';
      tip.querySelector('.tm').innerHTML=`<i>${esc(n.domain)}</i><i>${esc(n.mem_type||'sin tipo')}</i><i>calor ${n.heat}</i>`;
    }
    const tw=tip.offsetWidth||220, th=tip.offsetHeight||70; let x=mx+16,y=my+16; if(x+tw>innerWidth-8)x=mx-tw-16; if(y+th>innerHeight-8)y=my-th-16;
    tip.style.left=x+'px'; tip.style.top=y+'px'; tip.classList.add('on'); } else tip.classList.remove('on'); }

// fetchExplain: trae UNA vez las memorias que explican el símbolo (weld F3), cacheado en el nodo
// (n._exp). Lazy + debounce por el flag: solo pega a /api/explained la primera vez que se hoverea.
async function fetchExplain(n){ if(n._exp!==undefined || n._expLoading) return; n._expLoading=true;
  try{ const r=await fetch('/api/explained?symbol='+encodeURIComponent(n.id),{cache:'no-store'}); n._exp=r.ok?((await r.json())||[]):[]; }
  catch(_){ n._exp=[]; } finally{ n._expLoading=false; } }

/* ---------- loop ---------- */
const AMP=2.4;
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
  // La lente personas se dibuja ACA y no en su propio requestAnimationFrame: dos bucles
  // peleandose por el mismo frame es como se pierde el control del costo por cuadro.
  if(VISTA_PERSONAS.activa()){ VISTA_PERSONAS.frame(motion); return; }
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
  $('kDomains').textContent=code
    ?(cg.total_modules?(cg.truncated&&DOMAINS.length<cg.total_modules?`${DOMAINS.length}/${cg.total_modules}`:cg.total_modules):DOMAINS.length)
    :(gdoms.length||DOMAINS.length);
  // La leyenda también: su ranking y sus conteos salían del top-N por saliencia, que está
  // sesgado por recencia y calor — un dominio grande y frío no aparecía. El color se sigue
  // tomando de DOMCOL (la muestra); los que no entraron se pintan en reposo.
  const legend=(!code&&gdoms.length)
    ?gdoms.slice().sort((x,y)=>(y.count-x.count)||String(x.domain).localeCompare(String(y.domain))).slice(0,10)
      .map(dd=>({name:dd.domain,count:dd.count,color:(DOMCOL&&DOMCOL.get(dd.domain))||'#7f9cc9'}))
    :DOMAINS.slice(0,10);
  // En la lente personas esta tarjeta muestra a las PERSONAS, no a los dominios. El guardia va
  // acá y no en renderLens porque renderHUD corre en CADA poll: sin él, la leyenda de personas
  // vivía cinco segundos y volvía a ser la de dominios sola.
  if(lens==='personas') pintarLeyendaPersonas();
  else $('domlegend').innerHTML=legend.length?legend.map(dd=>`<div class="lg"><span class="sw" style="background:${dd.color};color:${dd.color}"></span>${esc(dd.name)} <b>${dd.count}</b></div>`).join(''):'<div class="empty">sin dominios</div>';
  // En la lente personas los tres primeros KPI cambian de SUJETO, y por eso cambian el valor
  // JUNTO con la etiqueta: dejar el rótulo «Terminales» sobre el conteo de memorias es la
  // forma más barata de que un panel mienta con datos ciertos.
  if(lens==='personas'&&PERSONAS){
    $('kActive').textContent=PERSONAS.terminales.length;
    // DESPACHOS son los mensajes, no los PARES. `despachos.length` cuenta pares distintos
    // (A→B una vez, valga 1 o valga 40) y ponerlo bajo el rótulo «despachos» dividía la cifra
    // real por cinco. El total es la suma de `veces`; los pares van a la leyenda, aparte.
    $('kSyn').textContent=PERSONAS.despachos.reduce((s,d)=>s+d.veces,0);
    $('kDomains').textContent=RACIMOS.length;
  }
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

function conectarVivo(){
  // EventSource y no fetch: el stream local es same-origin sobre loopback y no lleva credencial
  // (el bearer se queda en el relay), asi que se puede usar el que reconecta solo. El del CEREBRO
  // sí necesita header, y por eso ese lo abre el relay con fetch — ver livestream.go.
  let es;
  try{ es=new EventSource('/api/stream'); }catch(_){ return; }
  es.addEventListener('backlog', m=>{ try{ (JSON.parse(m.data)||[]).forEach(anotarEvento); }catch(_){} pintarVivo(); });
  es.addEventListener('uso', m=>{ try{ anotarEvento(JSON.parse(m.data)); }catch(_){} pintarVivo(); });
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

async function fetchGraph(which){
  const r=await fetch('/api/graph?lens='+which,{cache:'no-store'});
  if(!r.ok) throw 0;
  GRAPH[which]=await r.json();
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

async function poll(){
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
  if(lens==='personas'){
    // Se alimenta del MISMO grafo de memoria que ya esta en cache: no pide nada nuevo al
    // server. La persona sale de `author`, que viaja en el grafo desde 0.107.0.
    if(!GRAPH.memory) return;
    const datos = extraerPersonas(GRAPH.memory);
    RACIMOS = agruparPorPersona(datos.terminales);
    PERSONAS = datos;
    VISTA_PERSONAS.cargar(datos, RACIMOS);
    pintarLeyendaPersonas();
    return;
  }
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

// setLens: conmuta memoria↔código. Reconstruye el grafo, repinta leyendas/etiquetas y el HUD.
function setLens(v){ lens=v; const b=$('lensBtn');
  const cvB=document.getElementById('brain'), cvP=document.getElementById('personas');
  const esPer = lens==='personas';
  // Se OCULTA el WebGL en vez de dejarlo debajo: sigue costando GPU aunque no se vea.
  if(cvB) cvB.hidden = esPer;
  if(cvP) cvP.hidden = !esPer;
  VISTA_PERSONAS.activar(esPer);
  if(b){ b.textContent = esPer?'◉ personas':(lens==='code'?'◉ código':'◉ memoria');
    b.classList.toggle('code',lens==='code'); b.classList.toggle('personas',esPer);
    b.setAttribute('aria-pressed',String(lens!=='memory')); }
  applyLensLabels();
  // La lente de código se baja on-demand: no viaja en el pulso ni tiene por qué estar en
  // memoria si nadie la miró. Si falta, se pide y recién ahí se dibuja.
  if(lens==='code' && !GRAPH.code){ fetchGraph('code').then(()=>{ renderLens(); if(PULSE) renderHUD(hudShape()); }).catch(()=>{}); return; }
  renderLens(); if(PULSE) renderHUD(hudShape()); }
// applyLensLabels: intercambia los textos estáticos del HUD según la lente (leyenda de aristas,
// títulos, guía). Se llama al togglear, no en cada poll.
function applyLensLabels(){ const code=lens==='code', per=lens==='personas';
  const set=(id,t)=>{ const e=$(id); if(e) e.textContent=t; };
  // Los contadores de la cabecera siguen describiendo la MEMORIA aun en la lente personas: son
  // el universo del que sale este grafo, y renombrarlos ahí sí sería mentir.
  set('lblNodes', code?'nodos':'neuronas'); set('lblEdges', code?'aristas':'sinapsis');
  set('domTitle', per?'Personas':(code?'Módulos':'Dominios'));
  set('lblActive', per?'Terminales':(code?'Nodos':'Memorias activas'));
  set('lblSyn', per?'Despachos':(code?'Aristas':'Sinapsis'));
  set('lblDomains', per?'Personas':(code?'Módulos':'Dominios'));
  if(per){
    const al0=$('actlegend'); if(al0) al0.innerHTML =
      `<div class="lg"><span class="sw" style="background:${COLOR_DESPACHO};color:${COLOR_DESPACHO}"></span>despacho</div>`+
      `<div class="lg"><span class="sw" style="background:${COLOR_CRUCE};color:${COLOR_CRUCE}"></span>entre personas</div>`;
    const ht0=$('howto'); if(ht0) ht0.innerHTML =
      `<span><b>·</b> cada neurona es una <b>terminal</b>; sus <b>dendritas</b>, cuánto escribió</span>`+
      `<span><b>·</b> las flechas son <b>despachos</b> y tienen <b>dirección</b>: quién le escribió a quién</span>`+
      `<span><b>·</b> el <b>color</b> agrupa por <b>persona</b> · la persona sale de quién <b>firma</b>, no de quién menciona</span>`;
    pintarLeyendaPersonas();
    return;
  }
  const al=$('actlegend'); if(al) al.innerHTML = code
    ? `<div class="lg"><span class="sw" style="background:${EDGEKIND.CALLS};color:${EDGEKIND.CALLS}"></span>llama</div><div class="lg"><span class="sw" style="background:${EDGEKIND.IMPORTS};color:${EDGEKIND.IMPORTS}"></span>importa</div><div class="lg"><span class="sw" style="background:${EDGEKIND.CONTAINS};color:${EDGEKIND.CONTAINS}"></span>contiene</div>`
    : `<div class="lg"><span class="sw" style="background:#7f9cc9;color:#7f9cc9"></span>reposo</div><div class="lg"><span class="sw" style="background:#43e08b;color:#43e08b"></span>escribir</div><div class="lg"><span class="sw" style="background:#31c9ff;color:#31c9ff"></span>recordar</div><div class="lg"><span class="sw" style="background:#f5c451;color:#f5c451"></span>relacionar</div>`;
  const ht=$('howto'); if(ht) ht.innerHTML = code
    ? `<span><b>·</b> cada punto es un <b>símbolo</b> (función, tipo, archivo)</span><span><b>·</b> las líneas son <b>llamadas / imports</b>; el color agrupa por <b>módulo</b></span><span><b>·</b> el <b>tamaño</b> = centralidad · <b>hover</b> muestra qué memorias lo explican</span>`
    : `<span><b>·</b> cada punto es una <b>memoria</b></span><span><b>·</b> las líneas, <b>relaciones</b>; la luz que viaja = <b>recuerdo activándose</b></span><span><b>·</b> el <b>color</b> agrupa por dominio · el <b>brillo</b>, recencia · el <b>tamaño</b>, importancia</span>`;
}
// pintarLeyendaPersonas: quién es quién, con EL MISMO color con que se dibujó su racimo.
// Y declara las notas sin `author`: el reparto por persona se hace sobre las que lo tienen, y
// callar cuántas quedaron afuera convierte una muestra parcial en un total aparente.
function pintarLeyendaPersonas(){
  const dl=$('domlegend'); if(!dl) return;
  if(!RACIMOS.length){ dl.innerHTML='<div class="empty">sin terminales firmadas</div>'; return; }
  const filas=RACIMOS.map((r,i)=>{ const c=colorPersona(i);
    return `<div class="lg"><span class="sw" style="background:${c};color:${c}"></span>${esc(r.persona)} <b>${r.notas}</b></div>`; });
  const pares=PERSONAS?PERSONAS.despachos.length:0;
  if(pares) filas.push(`<div class="lg" style="opacity:.55">${pares} pares se escriben</div>`);
  const sa=PERSONAS?PERSONAS.sinAutor:0;
  if(sa) filas.push(`<div class="lg" style="opacity:.55">${sa} notas sin autor · no se reparten</div>`);
  dl.innerHTML=filas.join('');
}
const lensBtn=$('lensBtn'); if(lensBtn) lensBtn.addEventListener('click',()=>
  setLens(lens==='memory' ? 'code' : (lens==='code' ? 'personas' : 'memory')));

// La lente se puede fijar por URL (?lens=personas). Sirve para dos cosas concretas: que el
// CRM pueda enlazar directo a una vista, y que una captura automatica pueda verificar una
// lente sin simular un click. Un valor desconocido se ignora y queda `memory`.
const _lensURL=new URLSearchParams(location.search).get('lens');
if(_lensURL==='code'||_lensURL==='personas') setLens(_lensURL);

poll(); setInterval(poll,5000);
conectarVivo();
requestAnimationFrame(animate);
