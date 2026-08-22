// layout.mjs — LA FÍSICA DEL GRAFO. Sin DOM, sin three.js, sin red.
//
// POR QUÉ ESTÁ SEPARADO. dashboard.mjs arranca WebGL, cuelga listeners de `document` y abre un
// EventSource en su nivel superior: importarlo fuera de un navegador es imposible. Mientras la
// física vivía adentro, la única forma de probarla era recortar funciones del fuente con un script
// —frágil y, sobre todo, imposible de correr en CI—. Acá se importa y se prueba con `node --test`.
//
// El módulo tiene UNA instancia de estado, igual que cuando vivía en dashboard.mjs: hay un solo
// grafo asentándose a la vez.

// iterSettle: cuántas iteraciones de física corre el layout según el tamaño. Es una sola
// heurística, sin capas: más nodos, menos iteraciones, porque cada una cuesta O(n log n) con
// Barnes-Hut. Los grafos chicos pueden pagar muchas y quedan más prolijos.
//
// El corte de arriba (>2000 → 55) es lo que hace que el grafo de código del central, con más de
// 8.000 nodos, no se pase toda la sesión asentándose. No es un número mágico: cada tope de
// iteraciones es un equilibrio ligeramente distinto, no uno correcto contra uno incorrecto.
export function iterSettle(n){ return n>2000?55:(n>500?90:(n>180?150:230)); }

// iterParaCambio: cuantas iteraciones merece ESTE cambio, que no es lo mismo que cuantas merece
// el grafo entero.
//
// POR QUE EXISTE. `changed` da true con UN solo nodo nuevo, y eso disparaba iterSettle(n)
// completo: 55 iteraciones sobre 3.678 nodos = 1,45 s de CPU, CADA VEZ que se guardaba una
// observacion. Con los hooks de captura escribiendo seguido, eso es el panel trabandose cada
// pocos segundos — y encima reacomodando todo el grafo, asi que lo que estabas mirando se movia
// de lugar sin que nada importante hubiera cambiado.
//
// Un grafo YA ASENTADO que recibe unas pocas memorias nuevas no necesita re-asentarse entero: el
// resto esta en equilibrio y el damping de 0,86 lo deja donde esta. Alcanza con acomodar lo nuevo.
export function iterParaCambio(n, nuevos, yaAsentado){
  if(!yaAsentado) return iterSettle(n);                 // primera vez: el layout todavia no existe
  if(nuevos<=0) return 0;                               // nada nuevo que ubicar
  return Math.min(iterSettle(n), 4+nuevos*2);           // acomodar lo nuevo, no rehacer el mundo
}

// ---------- Barnes-Hut: la repulsión lejana se aproxima por el centro de masa de un cubo ----------
//
// POR QUÉ NO ALCANZABA UNA GRILLA. El corte de repulsión es rx*0.85, o sea el 85% del radio del
// cerebro: casi TODOS los pares caen dentro del corte, así que una grilla espacial terminaría
// mirando las mismas 27 celdas que son el volumen entero. La única forma de bajar de O(n²) sin
// tocar la física es agregar los lejanos: un octree con criterio de apertura theta.
//
// A 3.667 neuronas el bucle exacto son 6,7 M de pares POR ITERACIÓN (×90 iteraciones ≈ 600 M):
// congela la pestaña. Con el octree quedan ~n·log n.
export const BH_MIN=700;    // debajo de esto se usa el bucle EXACTO: los grafos chicos conservan su look bit a bit
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
      const jx=NS[j].x, jy=NS[j].y, jz=NS[j].z;
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
   Ahora settleStart() arma el estado, settlePasada() avanza una iteracion, y settleTick() consume
   un presupuesto de milisegundos por frame. El grafo se ve organizarse en vez de congelarse. */
let NS=[], SY=[], RADIOS=()=>({rx:118,ry:94,rz:87});
let _setQ=0, _setCut2=0, _setCharge=0, _setRest=0, _setBH=false;
const _setKS=0.09, _setKC=0.0042, _setDamp=0.86;   // resortes cortos+fuertes: conectadas mas juntas

/* ---------- UNA ITERACION TAMBIEN SE CORTA A LA MITAD ----------
   Repartir el asentado en iteraciones NO alcanzo, y el numero lo dice: UNA iteracion cuesta
   26,4 ms sobre los 3.678 nodos de memoria y 73,0 ms sobre los 8.362 de codigo, contra un
   presupuesto de 6 ms por frame. El grano era 12x el tramo, asi que el `do{}while` de settleTick
   corria igual una iteracion entera y el frame se iba a 73 ms. No es un ajuste de constante: hay
   que poder cortar ADENTRO de la iteracion.

   DONDE CORTAR, medido y no supuesto: con 8.362 nodos y 18.073 aristas la iteracion cuesta
   68,5 ms; con los MISMOS nodos y CERO aristas, 68,4 ms. Las aristas no cuestan nada. Todo el
   costo es la repulsion por nodo, y escala con la cantidad de nodos (4x nodos -> 3,7x tiempo).
   Se rebana por nodo, entonces, y nada mas.

   Y SALE IDENTICO BIT A BIT. La repulsion de cada nodo lee (a) el arbol Barnes-Hut, que se arma
   una vez al principio de la pasada y no se toca, y (b) su propia posicion, que no cambia hasta
   la integracion del final. Cada nodo escribe SOLO su velocidad. Por eso el orden no importa y
   cortar a la mitad da el mismo resultado que correrla de un tiron. Lo custodia layout.test.mjs,
   que corre en CI: sin esa prueba, rebanar distinto se veria como "el grafo quedo un poco
   distinto", que es justo lo que nadie mira. */
const _setTROZO=256;   // nodos por trozo: ~2 ms a la escala medida, grano fino sin overhead
let _setFase=0;        // 0 = falta armar el arbol · 1 = repulsion en curso · 2 = resortes+integracion
let _setI=0;           // por que nodo va la repulsion de ESTA pasada
let _setRoot=0;        // raiz del arbol de ESTA pasada

// clampBrain mantiene el nodo dentro del elipsoide.
function clampBrain(n, rx, ry, rz){
  const q=(n.x*n.x)/(rx*rx)+(n.y*n.y)/(ry*ry)+(n.z*n.z)/(rz*rz);
  if(q>1){ const s=1/Math.sqrt(q); n.x*=s; n.y*=s; n.z*=s; n.vx*=0.4; n.vy*=0.4; n.vz*=0.4; }
}

// settleStart fija el mundo a asentar.
//
// `leerRadios` es una FUNCION y no tres numeros a proposito: reconstruir el grafo los cambia
// (applyScale escala con la cantidad de nodos), y clampBrain los quiere frescos. Se leen una vez
// por llamada a settlePasada, que es exactamente cuando los leia el codigo original — dentro de
// una pasada nada puede cambiarlos, porque no hay await de por medio.
export function settleStart(neuronas, sinapsis, leerRadios, iters){
  NS=neuronas; SY=sinapsis; RADIOS=leerRadios;
  const n=NS.length; if(!n){ _setQ=0; return; }
  // SIN atractores de dominio (esparce como el prototipo) · más repulsión + resortes cortos
  const rx=RADIOS().rx;
  const cut=rx*0.85; _setCut2=cut*cut; _setCharge=rx*rx*0.06; _setRest=rx*0.044;
  _setBH = n>=BH_MIN; if(_setBH) bhGrow(Math.max(1024, n*4));
  _setQ = iters;
  // Reiniciar la pasada en curso es OBLIGATORIO: bhGrow acaba de reasignar los arrays del arbol,
  // asi que un _setRoot a medio recorrer apuntaria a memoria de otro grafo.
  _setFase=0; _setI=0;
}

// settlePendiente: cuantas iteraciones faltan. El panel lo usa para decidir cuanto suavizar.
export function settlePendiente(){ return _setQ; }

// settleTick gasta hasta `ms` en asentar. Devuelve si hizo algo y si termino de asentar. Guardar
// las posiciones finales es cosa de quien llama: la fisica no sabe de lentes ni de cache.
export function settleTick(ms){
  if(_setQ<=0) return {trabajo:false, termino:false};
  const t0=performance.now();
  do {
    if(settlePasada(t0, ms)) _setQ--;     // la pasada TERMINO; si no, sigue el proximo frame
    else break;                            // se acabo el presupuesto a mitad de la pasada
  } while(_setQ>0 && performance.now()-t0<ms);
  return {trabajo:true, termino:_setQ===0};
}

// settlePasada avanza UNA iteracion todo lo que entre en el presupuesto. Devuelve true si la
// completo. Retomar es seguro: el estado de donde iba vive en _setFase/_setI/_setRoot.
export function settlePasada(t0, ms){
  const n=NS.length; if(!n) return true;
  const cut2=_setCut2, charge=_setCharge, rest=_setRest, kS=_setKS, kC=_setKC, damp=_setDamp, bh=_setBH;

  if(_setFase===0){
    if(bh){
      let half=1; for(const a of NS){ const m=Math.max(Math.abs(a.x),Math.abs(a.y),Math.abs(a.z)); if(m>half) half=m; }
      bhUsed=0; _setRoot=bhCell(0,0,0,half*1.05);
      for(let i=0;i<n;i++){ const a=NS[i]; bhInsert(_setRoot,i,a.x,a.y,a.z); }
    }
    _setFase=1; _setI=0;
  }

  if(_setFase===1){
    if(bh){
      while(_setI<n){
        const hasta=Math.min(n, _setI+_setTROZO);
        for(let i=_setI;i<hasta;i++){ const a=NS[i];
          bhForce(_setRoot,i,a.x,a.y,a.z,cut2,charge,_bhOut);
          a.vx+=_bhOut[0]*.5-a.x*kC; a.vy+=_bhOut[1]*.5-a.y*kC; a.vz+=_bhOut[2]*.5-a.z*kC; }
        _setI=hasta;
        // El chequeo va DESPUES del trozo: siempre se avanza algo, nunca se gira en falso.
        if(_setI<n && performance.now()-t0>=ms) return false;
      }
    } else {
      // Bucle EXACTO O(n2), solo por debajo de BH_MIN (700 nodos). No se rebana porque cada nodo
      // escribe la velocidad de los OTROS —cortarlo cambiaria el resultado— y porque a esa escala
      // una iteracion entera es barata.
      for(let i=0;i<n;i++){ const a=NS[i]; let fx=0,fy=0,fz=0;
        for(let j=i+1;j<n;j++){ const b=NS[j]; let dx=a.x-b.x,dy=a.y-b.y,dz=a.z-b.z, d2=dx*dx+dy*dy+dz*dz;
          if(d2>cut2||d2<1e-3) continue; const f=charge/d2, d=Math.sqrt(d2), ux=dx/d,uy=dy/d,uz=dz/d;
          fx+=ux*f; fy+=uy*f; fz+=uz*f; b.vx-=ux*f*.5; b.vy-=uy*f*.5; b.vz-=uz*f*.5; }
        a.vx+=fx*.5-a.x*kC; a.vy+=fy*.5-a.y*kC; a.vz+=fz*.5-a.z*kC; }
      _setI=n;
    }
    _setFase=2;
  }

  // Resortes e integracion. NO se rebanan: medido, las 18.073 aristas cuestan 0,1 ms sobre 68,5.
  for(const s of SY){ const a=NS[s.a], b=NS[s.b];
    let dx=b.x-a.x,dy=b.y-a.y,dz=b.z-a.z, d=Math.hypot(dx,dy,dz)||1, f=(d-rest)*kS, ux=dx/d,uy=dy/d,uz=dz/d;
    a.vx+=ux*f; a.vy+=uy*f; a.vz+=uz*f; b.vx-=ux*f; b.vy-=uy*f; b.vz-=uz*f; }
  const r=RADIOS();
  for(const a of NS){ a.vx*=damp; a.vy*=damp; a.vz*=damp;
    const s2=a.vx*a.vx+a.vy*a.vy+a.vz*a.vz; if(s2>1600){ const s=40/Math.sqrt(s2); a.vx*=s;a.vy*=s;a.vz*=s; }
    a.x+=a.vx; a.y+=a.vy; a.z+=a.vz; clampBrain(a, r.rx, r.ry, r.rz); }

  _setFase=0; _setI=0;
  return true;
}
