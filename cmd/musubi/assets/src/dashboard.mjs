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
const inEllipsoid=(x,y,z)=>(x*x)/(rx*rx)+(y*y)/(ry*ry)+(z*z)/(rz*rz)<=1;
function randInBrain(){ for(let i=0;i<80;i++){ const x=(Math.random()*2-1)*rx,y=(Math.random()*2-1)*ry,z=(Math.random()*2-1)*rz; if(inEllipsoid(x,y,z)) return {x,y,z}; } return {x:0,y:0,z:0}; }
function clampBrain(n){ const q=(n.x*n.x)/(rx*rx)+(n.y*n.y)/(ry*ry)+(n.z*n.z)/(rz*rz); if(q>1){ const s=1/Math.sqrt(q); n.x*=s; n.y*=s; n.z*=s; n.vx*=0.4; n.vy*=0.4; n.vz*=0.4; } }

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
let lens='memory';   // lente activa (memory|code); los datos viven en GRAPH/PULSE (ver más abajo)

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

  const changed=NEURONS.some(n=>n._new)||NEURONS.length!==prevIds.size;
  // El asentado ya NO congela: se reparte en tramos de pocos ms por frame (ver settleTick).
  // POS se siembra igual ahora —asi la proxima vez arranca de donde quedo— y se re-siembra al
  // terminar de asentar, que es cuando las posiciones son las buenas.
  POS.memory=new Map(NEURONS.map(n=>[n.id,{x:n.x,y:n.y,z:n.z,ph:n.ph}]));
  if(changed || !ASENTADO.memory){ settleStart(iterSettle(NEURONS.length),'memory'); needsRebuild=true; }
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

  const changed=NEURONS.some(n=>n._new)||NEURONS.length!==prevIds.size;
  POS.code=new Map(NEURONS.map(n=>[n.id,{x:n.x,y:n.y,z:n.z,ph:n.ph}]));
  if(changed || !ASENTADO.code){ settleStart(iterSettle(NEURONS.length),'code'); needsRebuild=true; }
}

// iterSettle: cuántas iteraciones de física corre el layout según el tamaño. Es una sola
// cosa y no dos copias, porque estaba duplicado literal en buildGraph y buildCodeGraph y
// cualquier ajuste tenía que acordarse de los dos.
//
// El tramo >2000 es nuevo: con Barnes-Hut el grafo completo (3.667) tarda ~2 s a 90
// iteraciones, y eso es un congelamiento visible del hilo principal en la primera carga.
// A 55 baja a ~1,2 s. Es un layout, no una simulación: la diferencia entre 55 y 90
// iteraciones es un equilibrio ligeramente distinto, no uno correcto contra uno incorrecto.
function iterSettle(n){ return n>2000?55:(n>500?90:(n>180?150:230)); }

// ---------- Barnes-Hut: la repulsión lejana se aproxima por el centro de masa de un cubo ----------
//
// POR QUÉ NO ALCANZABA UNA GRILLA. El corte de repulsión es rx*0.85, o sea el 85% del radio del
// cerebro: casi TODOS los pares caen dentro del corte, así que una grilla espacial terminaría
// mirando las mismas 27 celdas que son el volumen entero. La única forma de bajar de O(n²) sin
// tocar la física es agregar los lejanos: un octree con criterio de apertura theta.
//
// A 3.667 neuronas el bucle exacto son 6,7 M de pares POR ITERACIÓN (×90 iteraciones ≈ 600 M):
// congela la pestaña. Con el octree quedan ~n·log n.
const BH_MIN=700;    // debajo de esto se usa el bucle EXACTO: los grafos chicos conservan su look bit a bit
const BH_THETA2=0.7*0.7;
let bhCap=0, bhChi, bhCnt, bhCX, bhCY, bhCZ, bhBod, bhHalf, bhOX, bhOY, bhOZ, bhUsed=0;
function bhGrow(need){ if(need<=bhCap) return; bhCap=Math.max(need, bhCap*2, 1024);
  bhChi=new Int32Array(bhCap*8); bhCnt=new Int32Array(bhCap);
  bhCX=new Float64Array(bhCap); bhCY=new Float64Array(bhCap); bhCZ=new Float64Array(bhCap);
  bhBod=new Int32Array(bhCap); bhHalf=new Float64Array(bhCap);
  bhOX=new Float64Array(bhCap); bhOY=new Float64Array(bhCap); bhOZ=new Float64Array(bhCap); }
// El chequeo va ANTES de tomar el índice: si se toma primero y se crece después, la celda
// recién reservada cae fuera de los buffers viejos y se escribe en la nada.
function bhCell(ox,oy,oz,half){ if(bhUsed>=bhCap) bhGrowKeep(); const c=bhUsed++;
  bhChi.fill(-1,c*8,c*8+8); bhCnt[c]=0; bhCX[c]=bhCY[c]=bhCZ[c]=0; bhBod[c]=-1;
  bhHalf[c]=half; bhOX[c]=ox; bhOY[c]=oy; bhOZ[c]=oz; return c; }
function bhGrowKeep(){ const old={chi:bhChi,cnt:bhCnt,cx:bhCX,cy:bhCY,cz:bhCZ,bod:bhBod,h:bhHalf,ox:bhOX,oy:bhOY,oz:bhOZ,n:bhCap};
  bhCap=old.n; bhGrow(old.n*2 || 1024);   // bhGrow sale temprano si need<=bhCap: hay que pedirle MÁS del actual
  bhChi.set(old.chi); bhCnt.set(old.cnt); bhCX.set(old.cx); bhCY.set(old.cy); bhCZ.set(old.cz);
  bhBod.set(old.bod); bhHalf.set(old.h); bhOX.set(old.ox); bhOY.set(old.oy); bhOZ.set(old.oz); }
function bhOct(c,x,y,z){ return (x>bhOX[c]?1:0)|(y>bhOY[c]?2:0)|(z>bhOZ[c]?4:0); }
function bhInsert(root,i,px,py,pz){
  let c=root;
  for(let guard=0; guard<64; guard++){
    bhCnt[c]++; bhCX[c]+=px; bhCY[c]+=py; bhCZ[c]+=pz;
    if(bhCnt[c]===1){ bhBod[c]=i; return; }                 // celda vacía → hoja con este cuerpo
    if(bhBod[c]>=0){                                        // hoja ocupada → subdividir y re-alojar
      const j=bhBod[c]; bhBod[c]=-1;
      const jx=NEURONS[j].x, jy=NEURONS[j].y, jz=NEURONS[j].z;
      const oj=bhOct(c,jx,jy,jz), h=bhHalf[c]*0.5;
      let k=bhChi[c*8+oj];
      if(k<0){ k=bhCell(bhOX[c]+(oj&1?h:-h), bhOY[c]+(oj&2?h:-h), bhOZ[c]+(oj&4?h:-h), h); bhChi[c*8+oj]=k; }
      bhCnt[k]++; bhCX[k]+=jx; bhCY[k]+=jy; bhCZ[k]+=jz; bhBod[k]=j;
    }
    const o=bhOct(c,px,py,pz), h=bhHalf[c]*0.5;
    let k=bhChi[c*8+o];
    if(k<0){ k=bhCell(bhOX[c]+(o&1?h:-h), bhOY[c]+(o&2?h:-h), bhOZ[c]+(o&4?h:-h), h); bhChi[c*8+o]=k; }
    c=k;
  }
}
const bhStack=new Int32Array(16384);
function bhForce(root,i,px,py,pz,cut2,charge,out){
  let sp=0; bhStack[sp++]=root; let fx=0,fy=0,fz=0;
  while(sp>0){ const c=bhStack[--sp]; const m=bhCnt[c]; if(m===0) continue;
    const cx=bhCX[c]/m, cy=bhCY[c]/m, cz=bhCZ[c]/m;
    const dx=px-cx, dy=py-cy, dz=pz-cz; const d2=dx*dx+dy*dy+dz*dz;
    const leaf=bhBod[c]>=0;
    if(leaf && bhBod[c]===i) continue;                       // no se repele a sí mismo
    // criterio de apertura: si el cubo se ve chico desde acá, vale su centro de masa
    const w=bhHalf[c]*2;
    if(leaf || (w*w < BH_THETA2*d2)){
      if(d2>cut2 || d2<1e-3) continue;
      const f=charge*m/d2, d=Math.sqrt(d2);
      fx+=dx/d*f; fy+=dy/d*f; fz+=dz/d*f;
    } else { for(let o=0;o<8;o++){ const k=bhChi[c*8+o]; if(k>=0 && sp<bhStack.length) bhStack[sp++]=k; } }
  }
  out[0]=fx; out[1]=fy; out[2]=fz;
}
const _bhOut=new Float64Array(3);

/* ---------- ASENTADO POR TRAMOS ----------
   Antes era un `for` sincronico de N iteraciones. Medido con las funciones reales: sobre el grafo
   de codigo del central (8362 nodos, 18073 aristas) son 3.952 ms con el hilo principal BLOQUEADO.
   Durante ese rato requestAnimationFrame no corre, asi que no es que baje el framerate: NO HAY
   FRAMES. Ese era el "se va toda la animacion al tocar codigo".
   Ahora settleStart() arma el estado, settleStep() corre UNA iteracion, y settleTick() consume
   un presupuesto de milisegundos por frame. El grafo se ve organizarse en vez de congelarse. */
let _setQ=0, _setLens='memory', _setCut2=0, _setCharge=0, _setRest=0, _setBH=false;
const _setKS=0.09, _setKC=0.0042, _setDamp=0.86;   // resortes cortos+fuertes: conectadas mas juntas

function settleStart(iters, cual){
  const n=NEURONS.length; if(!n){ _setQ=0; return; }
  // SIN atractores de dominio (esparce como el prototipo) · más repulsión + resortes cortos
  const cut=rx*0.85; _setCut2=cut*cut; _setCharge=rx*rx*0.06; _setRest=rx*0.044;
  _setBH = n>=BH_MIN; if(_setBH) bhGrow(Math.max(1024, n*4));
  _setLens = cual || lens; _setQ = iters; ASENTADO[_setLens]=false;
}

// settleTick: corre iteraciones hasta agotar el presupuesto. Devuelve true si movio algo, para que
// animate() sepa que tiene que refrescar las posiciones base de las instancias.
function settleTick(ms){
  if(_setQ<=0) return false;
  const t0=performance.now();
  do { settleStep(); _setQ--; } while(_setQ>0 && performance.now()-t0<ms);
  if(_setQ===0){   // ya asentado: esta es la posicion que vale la pena recordar
    POS[_setLens]=new Map(NEURONS.map(n=>[n.id,{x:n.x,y:n.y,z:n.z,ph:n.ph}]));
    ASENTADO[_setLens]=true;
  }
  return true;
}

function settleStep(){ const n=NEURONS.length; if(!n) return;
  const cut2=_setCut2, charge=_setCharge, rest=_setRest, kS=_setKS, kC=_setKC, damp=_setDamp, bh=_setBH;
  {
    if(bh){
      let half=1; for(const a of NEURONS){ const m=Math.max(Math.abs(a.x),Math.abs(a.y),Math.abs(a.z)); if(m>half) half=m; }
      bhUsed=0; const root=bhCell(0,0,0,half*1.05);
      for(let i=0;i<n;i++){ const a=NEURONS[i]; bhInsert(root,i,a.x,a.y,a.z); }
      for(let i=0;i<n;i++){ const a=NEURONS[i];
        bhForce(root,i,a.x,a.y,a.z,cut2,charge,_bhOut);
        a.vx+=_bhOut[0]*.5-a.x*kC; a.vy+=_bhOut[1]*.5-a.y*kC; a.vz+=_bhOut[2]*.5-a.z*kC; }
    } else {
    for(let i=0;i<n;i++){ const a=NEURONS[i]; let fx=0,fy=0,fz=0;
      for(let j=i+1;j<n;j++){ const b=NEURONS[j]; let dx=a.x-b.x,dy=a.y-b.y,dz=a.z-b.z, d2=dx*dx+dy*dy+dz*dz;
        if(d2>cut2||d2<1e-3) continue; const f=charge/d2, d=Math.sqrt(d2), ux=dx/d,uy=dy/d,uz=dz/d;
        fx+=ux*f; fy+=uy*f; fz+=uz*f; b.vx-=ux*f*.5; b.vy-=uy*f*.5; b.vz-=uz*f*.5; }
      a.vx+=fx*.5-a.x*kC; a.vy+=fy*.5-a.y*kC; a.vz+=fz*.5-a.z*kC; }
    }
    for(const s of SYN){ const a=NEURONS[s.a], b=NEURONS[s.b];
      let dx=b.x-a.x,dy=b.y-a.y,dz=b.z-a.z, d=Math.hypot(dx,dy,dz)||1, f=(d-rest)*kS, ux=dx/d,uy=dy/d,uz=dz/d;
      a.vx+=ux*f; a.vy+=uy*f; a.vz+=uz*f; b.vx-=ux*f; b.vy-=uy*f; b.vz-=uz*f; }
    for(const a of NEURONS){ a.vx*=damp; a.vy*=damp; a.vz*=damp;
      const s2=a.vx*a.vx+a.vy*a.vy+a.vz*a.vz; if(s2>1600){ const s=40/Math.sqrt(s2); a.vx*=s;a.vy*=s;a.vz*=s; }
      a.x+=a.vx; a.y+=a.vy; a.z+=a.vz; clampBrain(a); } }
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
  const _t0 = STATS ? performance.now() : 0;
  if(needsRebuild || (inst && (N!==NEURONS.length || (edgeInst?edgeInst.count:0)!==SYN.length))) rebuildMeshes();
  // El layout se asienta de a tramos: 6 ms por frame deja ~10 ms para dibujar y mantiene 60 fps
  // mientras el grafo se organiza a la vista.
  const asentando = settleTick(6); if(asentando) refrescarBase(_setQ>0 ? 0.16 : 1);
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
  $('domlegend').innerHTML=legend.length?legend.map(dd=>`<div class="lg"><span class="sw" style="background:${dd.color};color:${dd.color}"></span>${esc(dd.name)} <b>${dd.count}</b></div>`).join(''):'<div class="empty">sin dominios</div>';
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

// renderLens: reconstruye el grafo con la lente activa desde lo que hay en cache (sin re-pollear).
function renderLens(){
  if(lens==='code'){ if(GRAPH.code) buildCodeGraph(GRAPH.code); }
  else if(GRAPH.memory) buildGraph(GRAPH.memory);
}

function setMotion(v){ motion=v; const b=$('motionBtn'); if(b){ b.textContent=motion?'❚❚ pausar':'▶ reanudar'; b.classList.toggle('paused',!motion); b.setAttribute('aria-pressed',String(!motion)); } }
$('motionBtn').addEventListener('click',()=>setMotion(!motion)); setMotion(motion);

// setLens: conmuta memoria↔código. Reconstruye el grafo, repinta leyendas/etiquetas y el HUD.
function setLens(v){ lens=v; const b=$('lensBtn');
  if(b){ b.textContent=lens==='code'?'◉ código':'◉ memoria'; b.classList.toggle('code',lens==='code'); b.setAttribute('aria-pressed',String(lens==='code')); }
  applyLensLabels();
  // La lente de código se baja on-demand: no viaja en el pulso ni tiene por qué estar en
  // memoria si nadie la miró. Si falta, se pide y recién ahí se dibuja.
  if(lens==='code' && !GRAPH.code){ fetchGraph('code').then(()=>{ renderLens(); if(PULSE) renderHUD(hudShape()); }).catch(()=>{}); return; }
  renderLens(); if(PULSE) renderHUD(hudShape()); }
// applyLensLabels: intercambia los textos estáticos del HUD según la lente (leyenda de aristas,
// títulos, guía). Se llama al togglear, no en cada poll.
function applyLensLabels(){ const code=lens==='code';
  const set=(id,t)=>{ const e=$(id); if(e) e.textContent=t; };
  set('lblNodes', code?'nodos':'neuronas'); set('lblEdges', code?'aristas':'sinapsis');
  set('domTitle', code?'Módulos':'Dominios'); set('lblActive', code?'Nodos':'Memorias activas');
  set('lblSyn', code?'Aristas':'Sinapsis'); set('lblDomains', code?'Módulos':'Dominios');
  const al=$('actlegend'); if(al) al.innerHTML = code
    ? `<div class="lg"><span class="sw" style="background:${EDGEKIND.CALLS};color:${EDGEKIND.CALLS}"></span>llama</div><div class="lg"><span class="sw" style="background:${EDGEKIND.IMPORTS};color:${EDGEKIND.IMPORTS}"></span>importa</div><div class="lg"><span class="sw" style="background:${EDGEKIND.CONTAINS};color:${EDGEKIND.CONTAINS}"></span>contiene</div>`
    : `<div class="lg"><span class="sw" style="background:#7f9cc9;color:#7f9cc9"></span>reposo</div><div class="lg"><span class="sw" style="background:#43e08b;color:#43e08b"></span>escribir</div><div class="lg"><span class="sw" style="background:#31c9ff;color:#31c9ff"></span>recordar</div><div class="lg"><span class="sw" style="background:#f5c451;color:#f5c451"></span>relacionar</div>`;
  const ht=$('howto'); if(ht) ht.innerHTML = code
    ? `<span><b>·</b> cada punto es un <b>símbolo</b> (función, tipo, archivo)</span><span><b>·</b> las líneas son <b>llamadas / imports</b>; el color agrupa por <b>módulo</b></span><span><b>·</b> el <b>tamaño</b> = centralidad · <b>hover</b> muestra qué memorias lo explican</span>`
    : `<span><b>·</b> cada punto es una <b>memoria</b></span><span><b>·</b> las líneas, <b>relaciones</b>; la luz que viaja = <b>recuerdo activándose</b></span><span><b>·</b> el <b>color</b> agrupa por dominio · el <b>brillo</b>, recencia · el <b>tamaño</b>, importancia</span>`;
}
const lensBtn=$('lensBtn'); if(lensBtn) lensBtn.addEventListener('click',()=>setLens(lens==='code'?'memory':'code'));

poll(); setInterval(poll,5000);
requestAnimationFrame(animate);
