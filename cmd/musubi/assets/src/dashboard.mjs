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
let NEURONS=[], SYN=[], POS=new Map();
let prevStats=new Map(), prevSyn=new Set(), thinking=0, actInc=new Float32Array(0), bestInc=new Float32Array(0), akSrc=new Int8Array(0);
let motion=true, needsRebuild=false;
let lens='memory';   // lente activa (memory|code); los datos viven en GRAPH/PULSE (ver más abajo)

// buildGraph: PORTADO del dashboard anterior. Detecta ACTIVIDAD REAL diffeando el snapshot y la tipa
// (escribir/recordar/relacionar); corre el force-sim si cambió la topología. Marca needsRebuild para
// que la escena three recree las mallas cuando cambian nodos/aristas.
function buildGraph(brain){
  const prev=POS, ns0=brain.neurons||[];
  const N0=240; growth=Math.max(0.85, Math.min(2.2, Math.cbrt((ns0.length||1)/N0))); applyScale();
  const counts={}; ns0.forEach(n=>counts[n.domain]=(counts[n.domain]||0)+1);
  const doms=Object.keys(counts).sort((a,b)=>counts[b]-counts[a]||a.localeCompare(b));
  DOMCOL=new Map(); DOMAINS=[]; const dIdx=new Map();
  doms.forEach((d,i)=>{ const col=DOMPAL[i%DOMPAL.length]; DOMCOL.set(d,col); dIdx.set(d,i);
    const k=i+0.5, phi=Math.acos(1-2*k/Math.max(doms.length,1)), th=Math.PI*(1+Math.sqrt(5))*k;
    DOMAINS.push({name:d,color:col,count:counts[d], ax:Math.cos(th)*Math.sin(phi)*rx*0.52, ay:Math.cos(phi)*ry*0.52, az:Math.sin(th)*Math.sin(phi)*rz*0.52}); });

  const prevIds=new Set(NEURONS.map(n=>n.id));
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
    if(hits>0) thinking=Math.min(1, thinking+0.5+hits*0.2);
  }
  prevStats=new Map(NEURONS.map(n=>[n.id,{heat:n.heat, rec:n.recency_days}]));
  prevSyn=new Set(SYN.map(s=>s.source+'|'+s.target));

  const changed=NEURONS.some(n=>n._new)||NEURONS.length!==prevIds.size;
  if(changed) settle(iterSettle(NEURONS.length));
  POS=new Map(NEURONS.map(n=>[n.id,{x:n.x,y:n.y,z:n.z,ph:n.ph}]));
  if(changed) needsRebuild=true;   // solo recrear mallas si CAMBIÓ la topología (evita el parpadeo cada poll)
}

// buildCodeGraph: la LENTE CÓDIGO (Track 20). Mapea el grafo de código a los MISMOS campos que
// buildGraph, así el resto del pipeline (settle/spread/rebuildMeshes/animate) lo dibuja sin tocar
// nada: COLOR por módulo/paquete (reusa DOMPAL/DOMCOL), TAMAÑO por centralidad (grado), aristas
// tipadas por kind (CALLS/IMPORTS/CONTAINS). Reposo puro — sin sistema de actividad de memoria; el
// "porqué" (EXPLICADO_POR) se resuelve on-hover contra /api/explained.
function buildCodeGraph(code){
  const prev=POS, ns0=code.nodes||[];
  const N0=240; growth=Math.max(0.85, Math.min(2.2, Math.cbrt((ns0.length||1)/N0))); applyScale();
  const counts={}; ns0.forEach(n=>counts[n.module]=(counts[n.module]||0)+1);
  const mods=Object.keys(counts).sort((a,b)=>counts[b]-counts[a]||a.localeCompare(b));
  DOMCOL=new Map(); DOMAINS=[]; const dIdx=new Map();
  mods.forEach((d,i)=>{ const col=DOMPAL[i%DOMPAL.length]; DOMCOL.set(d,col); dIdx.set(d,i);
    DOMAINS.push({name:d,color:col,count:counts[d]}); });

  const prevIds=new Set(NEURONS.map(n=>n.id));
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
  if(changed) settle(iterSettle(NEURONS.length));
  POS=new Map(NEURONS.map(n=>[n.id,{x:n.x,y:n.y,z:n.z,ph:n.ph}]));
  if(changed) needsRebuild=true;
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

function settle(iters){ const n=NEURONS.length; if(!n) return;
  // SIN atractores de dominio (esparce como el prototipo) · más repulsión + resortes cortos = clusters conectados juntos, resto separado
  const cut=rx*0.85, cut2=cut*cut, charge=rx*rx*0.06, rest=rx*0.044, kS=0.09, kC=0.0042, damp=0.86; // resortes más cortos+fuertes: conectadas más juntas
  const bh=n>=BH_MIN;
  if(bh) bhGrow(Math.max(1024, n*4));
  for(let it=0; it<iters; it++){
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
const _m=new THREE.Matrix4(), _c=new THREE.Color(), _c2=new THREE.Color(), _v=new THREE.Vector3(), _up=new THREE.Vector3(0,1,0), _q=new THREE.Quaternion(), _pos=new THREE.Vector3(), _scl=new THREE.Vector3(), _eu=new THREE.Euler();
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
  inst=new THREE.InstancedMesh(NGEO, nodeMat, N);
  for(let i=0;i<N;i++){ const n=NEURONS[i]; BX[i]=n.x; BY[i]=n.y; BZ[i]=n.z; RAD[i]=n.r; PHX[i]=n.phx; PHY[i]=n.ph; PHZ[i]=n.phz;
    QROT.push(new THREE.Quaternion().setFromEuler(_eu.set(Math.random()*6.283,Math.random()*6.283,Math.random()*6.283)));
    _m.compose(_pos.set(n.x,n.y,n.z), QROT[i], _scl.set(n.r,n.r,n.r)); inst.setMatrixAt(i,_m); inst.setColorAt(i,_c.set(n.col)); }
  inst.instanceMatrix.needsUpdate=true; if(inst.instanceColor) inst.instanceColor.needsUpdate=true;
  world.add(inst);
  // Aristas: UNA instancia por sinapsis dentro de UNA malla. El índice en los buffers es el
  // índice en SYN, así que animate() escribe por posición sin buscar nada.
  const E=SYN.length;
  if(E){
    ECOL=new Float32Array(E*3); ESPD=new Float32Array(E); EGLW=new Float32Array(E); EBAS=new Float32Array(E);
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
function animate(){ requestAnimationFrame(animate); renderer.info.reset();
  if(needsRebuild || (inst && (N!==NEURONS.length || (edgeInst?edgeInst.count:0)!==SYN.length))) rebuildMeshes();
  const t=performance.now()/1000;
  if(motion){ thinking*=0.985; spread(); }
  if(inst && N){
    // objetivo de arrastre (coords locales)
    if(drag>=0){ ray.setFromCamera(ptr,camera); if(ray.ray.intersectPlane(_plane,_dt)) world.worldToLocal(_dt); else drag=-1; }
    let Dkx=0,Dky=0,Dkz=0;
    if(drag>=0){ const fx=BX[drag]+Math.sin(t*0.5+PHX[drag])*AMP, fy=BY[drag]+Math.sin(t*0.44+PHY[drag])*AMP, fz=BZ[drag]+Math.sin(t*0.57+PHZ[drag])*AMP;
      Dkx=_dt.x-fx; Dky=_dt.y-fy; Dkz=_dt.z-fz; GX[drag]=Dkx; GY[drag]=Dky; GZ[drag]=Dkz; }
    for(let i=0;i<N;i++){ const n=NEURONS[i];
      const dr=motion?AMP:0;
      const fx=BX[i]+Math.sin(t*0.5+PHX[i])*dr, fy=BY[i]+Math.sin(t*0.44+PHY[i])*dr, fz=BZ[i]+Math.sin(t*0.57+PHZ[i])*dr;
      if(i===drag){} else if(PULL[i]>0){ const p=PULL[i]; GX[i]+=(Dkx*p-GX[i])*0.14; GY[i]+=(Dky*p-GY[i])*0.14; GZ[i]+=(Dkz*p-GZ[i])*0.14; }
      else { GX[i]*=0.945; GY[i]*=0.945; GZ[i]*=0.945; }
      const x=fx+GX[i], y=fy+GY[i], z=fz+GZ[i]; n.x=x; n.y=y; n.z=z;
      _m.compose(_pos.set(x,y,z), QROT[i], _scl.set(RAD[i],RAD[i],RAD[i])); inst.setMatrixAt(i,_m);
      // color: dominio + tinte de actividad si está encendida (cubre neuronas aisladas sin aristas)
      _c.set(n.col); if(n.act>0.06){ _c2.set(AK[n.ak]||REPOSO); _c.lerp(_c2, Math.min(0.85,n.act*0.8)); } inst.setColorAt(i,_c); }
    inst.instanceMatrix.needsUpdate=true; if(inst.instanceColor) inst.instanceColor.needsUpdate=true;
    // aristas: siguen a las neuronas + pulso (reposo tenue vs actividad brillante).
    // Todo se escribe en los buffers de la ÚNICA malla instanciada: una draw call para las
    // 3.411 del cerebro completo, en vez de una por arista.
    if(edgeInst){
      edgeMat.uniforms.uTime.value=t;
      for(let si=0; si<SYN.length; si++){ const s=SYN[si]; const a=NEURONS[s.a], b=NEURONS[s.b];
        _v.set(b.x-a.x,b.y-a.y,b.z-a.z); const len=_v.length()||1;
        _q.setFromUnitVectors(_up,_v.normalize());
        _m.compose(_pos.set((a.x+b.x)/2,(a.y+b.y)/2,(a.z+b.z)/2), _q, _scl.set(s.__r,len,s.__r));
        edgeInst.setMatrixAt(si,_m);
        const act=Math.max(a.act,b.act), ak=(a.act>=b.act?a.ak:b.ak);
        if(act>0.06 && ak>0){ _c.set(AK[ak]); EGLW[si]=0.55+act*3.6; ESPD[si]=1.0+act*1.6; }   // ACTIVIDAD: brillante
        else { _c.set(edgeBase(s)); EGLW[si]=0.5+thinking*0.35; ESPD[si]=0.42+(s.confidence||0)*0.5; } // REPOSO: color por tipo (código) o azul tenue (memoria)
        ECOL[si*3]=_c.r; ECOL[si*3+1]=_c.g; ECOL[si*3+2]=_c.b; }
      edgeInst.instanceMatrix.needsUpdate=true;
      const at=edgeInst.geometry.attributes;
      at.aColor.needsUpdate=true; at.aSpeed.needsUpdate=true; at.aGlow.needsUpdate=true;
    }
  }
  controls.update(); hover(); composer.render();
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
