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
  if(changed) settle(NEURONS.length>500?90:(NEURONS.length>180?150:230));
  POS=new Map(NEURONS.map(n=>[n.id,{x:n.x,y:n.y,z:n.z,ph:n.ph}]));
  if(changed) needsRebuild=true;   // solo recrear mallas si CAMBIÓ la topología (evita el parpadeo cada poll)
}

function settle(iters){ const n=NEURONS.length; if(!n) return;
  // SIN atractores de dominio (esparce como el prototipo) · más repulsión + resortes cortos = clusters conectados juntos, resto separado
  const cut=rx*0.85, cut2=cut*cut, charge=rx*rx*0.06, rest=rx*0.044, kS=0.09, kC=0.0042, damp=0.86; // resortes más cortos+fuertes: conectadas más juntas
  for(let it=0; it<iters; it++){
    for(let i=0;i<n;i++){ const a=NEURONS[i]; let fx=0,fy=0,fz=0;
      for(let j=i+1;j<n;j++){ const b=NEURONS[j]; let dx=a.x-b.x,dy=a.y-b.y,dz=a.z-b.z, d2=dx*dx+dy*dy+dz*dz;
        if(d2>cut2||d2<1e-3) continue; const f=charge/d2, d=Math.sqrt(d2), ux=dx/d,uy=dy/d,uz=dz/d;
        fx+=ux*f; fy+=uy*f; fz+=uz*f; b.vx-=ux*f*.5; b.vy-=uy*f*.5; b.vz-=uz*f*.5; }
      a.vx+=fx*.5-a.x*kC; a.vy+=fy*.5-a.y*kC; a.vz+=fz*.5-a.z*kC; }
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

// aristas: tubo con shader de pulso continuo que barre el axón
const EGEO=new THREE.CylinderGeometry(1,1,1,6,1,true);
const VERT=['varying vec2 vUv;','void main(){ vUv=uv; gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0); }'].join('\n');
const FRAG=['precision highp float;','uniform float uTime; uniform vec3 uColor; uniform float uSpeed; uniform float uBase; uniform float uGlow;','varying vec2 vUv;',
  'float band(float y){ float p=fract(y); return smoothstep(0.0,0.06,p)*(1.0-smoothstep(0.06,0.36,p)); }',
  'void main(){ float y=vUv.y-uTime*uSpeed; float pulse=band(y)+band(y+0.5); float i=uBase+pulse*uGlow; gl_FragColor=vec4(uColor*i,i); }'].join('\n');

// post: MSAA + bloom + SMAA
const _dbs=new THREE.Vector2(); renderer.getDrawingBufferSize(_dbs);
const composer=new EffectComposer(renderer, new THREE.WebGLRenderTarget(_dbs.x,_dbs.y,{samples:4}));
composer.setSize(_dbs.x,_dbs.y);
composer.addPass(new RenderPass(scene,camera));
const bloom=new UnrealBloomPass(new THREE.Vector2(_dbs.x,_dbs.y),0.95,0.7,0.28); composer.addPass(bloom);
composer.addPass(new SMAAPass(_dbs.x,_dbs.y));

const controls=new TrackballControls(camera, renderer.domElement);
controls.rotateSpeed=2.4; controls.zoomSpeed=1.3; controls.panSpeed=0.6; controls.dynamicDampingFactor=0.12; controls.staticMoving=false;

// mallas + buffers reconstruibles al cambiar el grafo
let inst=null, N=0, BX,BY,BZ,RAD,PHX,PHY,PHZ,GX,GY,GZ,PULL,QROT,ADJ, tubeMeshes=[], tubeMats=[];
const _m=new THREE.Matrix4(), _c=new THREE.Color(), _c2=new THREE.Color(), _v=new THREE.Vector3(), _up=new THREE.Vector3(0,1,0), _q=new THREE.Quaternion(), _pos=new THREE.Vector3(), _scl=new THREE.Vector3(), _eu=new THREE.Euler();
let framed=false;

function disposeMeshes(){ if(inst){ world.remove(inst); inst.geometry.dispose(); inst=null; }
  for(const t of tubeMeshes){ world.remove(t); } tubeMats.forEach(m=>m.dispose()); tubeMeshes=[]; tubeMats=[]; }

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
  for(const s of SYN){ ADJ[s.a].push(s.b); ADJ[s.b].push(s.a);
    const mat=new THREE.ShaderMaterial({ uniforms:{ uTime:{value:0}, uColor:{value:new THREE.Color(REPOSO)}, uSpeed:{value:0.42+(s.confidence||0)*0.5}, uBase:{value:0.06}, uGlow:{value:0.55} },
      vertexShader:VERT, fragmentShader:FRAG, transparent:true, blending:THREE.AdditiveBlending, depthWrite:false });
    const mesh=new THREE.Mesh(EGEO,mat); mesh.__r=0.28+(s.confidence||0)*0.5; s.__mat=mat; s.__mesh=mesh; world.add(mesh); tubeMeshes.push(mesh); tubeMats.push(mat); }
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
    tip.querySelector('.tg').textContent=n.gist||'(sin resumen)';
    tip.querySelector('.tm').innerHTML=`<i>${n.domain}</i><i>${n.mem_type||'sin tipo'}</i><i>calor ${n.heat}</i>`;
    const tw=tip.offsetWidth||220, th=tip.offsetHeight||70; let x=mx+16,y=my+16; if(x+tw>innerWidth-8)x=mx-tw-16; if(y+th>innerHeight-8)y=my-th-16;
    tip.style.left=x+'px'; tip.style.top=y+'px'; tip.classList.add('on'); } else tip.classList.remove('on'); }

/* ---------- loop ---------- */
const AMP=2.4;
function animate(){ requestAnimationFrame(animate); renderer.info.reset();
  if(needsRebuild || (inst && (N!==NEURONS.length || tubeMeshes.length!==SYN.length))) rebuildMeshes();
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
    // aristas: siguen a las neuronas + pulso (reposo tenue vs actividad brillante)
    for(let si=0; si<SYN.length; si++){ const s=SYN[si], m=tubeMeshes[si]; if(!m) continue; const a=NEURONS[s.a], b=NEURONS[s.b];
      _v.set(b.x-a.x,b.y-a.y,b.z-a.z); const len=_v.length()||1;
      m.position.set((a.x+b.x)/2,(a.y+b.y)/2,(a.z+b.z)/2); m.scale.set(m.__r,len,m.__r); _q.setFromUnitVectors(_up,_v.normalize()); m.quaternion.copy(_q);
      const u=tubeMats[si].uniforms; u.uTime.value=t;
      const act=Math.max(a.act,b.act), ak=(a.act>=b.act?a.ak:b.ak);
      if(act>0.06 && ak>0){ u.uColor.value.set(AK[ak]); u.uGlow.value=0.55+act*3.6; u.uSpeed.value=1.0+act*1.6; }   // ACTIVIDAD: brillante
      else { u.uColor.value.set(REPOSO); u.uGlow.value=0.5+thinking*0.35; u.uSpeed.value=0.42+(s.confidence||0)*0.5; } } // REPOSO: tenue (sube leve con thinking)
  }
  controls.update(); hover(); composer.render();
}

/* ---------- HUD (PORTADO, paridad con el dashboard previo) ---------- */
const $=id=>document.getElementById(id);
const esc=s=>(s==null?'':String(s)).replace(/[&<>"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]));
function renderHUD(d){
  const b=d.brain||{neurons:[],synapses:[]}, ins=d.insights||{}, ob=ins.observations||{};
  const total=b.total_neurons||0, shown=(b.neurons||[]).length;
  $('neuronCount').textContent=b.truncated?`${shown}/${total}`:shown;
  $('synCount').textContent=(b.synapses||[]).length;
  $('proj').textContent=d.project||'—'; $('ver').textContent=d.version||'';
  $('kActive').textContent=ob.active!=null?ob.active:'—';
  $('kSyn').textContent=(b.synapses||[]).length;
  $('kDomains').textContent=DOMAINS.length||((d.graph||{}).domains||[]).length;
  $('domlegend').innerHTML=DOMAINS.length?DOMAINS.slice(0,10).map(dd=>`<div class="lg"><span class="sw" style="background:${dd.color};color:${dd.color}"></span>${esc(dd.name)} <b>${dd.count}</b></div>`).join(''):'<div class="empty">sin dominios</div>';
  const runs=((d.orchestration||{}).runs)||[];
  $('kRuns').textContent=runs.filter(r=>r.status==='running'||r.done<r.total).length;
  const h=d.health||{}, bad=(h.checks||[]).filter(c=>c.status&&c.status!=='ok').length;
  $('health').className='pill '+(h.status==='ok'?'ok':'warn');
  $('healthTxt').textContent=h.status==='ok'?'sano':(bad?`${bad} avisos`:'revisar');
  $('checks').innerHTML=(h.checks||[]).length?(h.checks||[]).map(c=>{ const ok=!c.status||c.status==='ok';
    return `<div class="chk"><span class="s" style="background:${ok?'var(--green)':'var(--amber)'};box-shadow:0 0 7px ${ok?'var(--green)':'var(--amber)'}"></span>${esc(c.code||c.name||'check')}</div>`;}).join(''):'<div class="empty">sin checks</div>';
  const tk=d.tokens||{}, budgeted=tk.status&&tk.status!=='unbudgeted'&&tk.budget;
  $('tokLabel').textContent=budgeted?'Tokens de sesión':'Tokens (sin techo)';
  $('tokVal').textContent=budgeted?`${tk.total||0} / ${tk.budget}`:(tk.total||0);
  const bar=$('tokBar'); bar.className='bar'+(tk.status==='watch'||tk.status==='over'?' watch':''); bar.querySelector('i').style.width=Math.min(100, budgeted?(tk.pct_used||0):(tk.total?8:0))+'%';
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

async function poll(){ try{ const r=await fetch('/api/snapshot',{cache:'no-store'}); if(!r.ok) throw 0;
  const d=await r.json(); buildGraph(d.brain||{neurons:[],synapses:[]}); renderHUD(d); $('liveTxt').textContent='en vivo';
  }catch(e){ $('liveTxt').textContent='reconectando'; } }

function setMotion(v){ motion=v; const b=$('motionBtn'); if(b){ b.textContent=motion?'❚❚ pausar':'▶ reanudar'; b.classList.toggle('paused',!motion); b.setAttribute('aria-pressed',String(!motion)); } }
$('motionBtn').addEventListener('click',()=>setMotion(!motion)); setMotion(motion);

poll(); setInterval(poll,5000);
requestAnimationFrame(animate);
