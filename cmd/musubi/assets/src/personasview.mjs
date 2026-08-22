// personasview.mjs — dibuja la lente PERSONAS: una neurona con dendritas por terminal, los
// despachos como axones dirigidos, y un racimo por persona.
//
// POR QUÉ CANVAS 2D Y NO three.js, teniendo three.js ya en el bundle: lo que hace que esto se
// lea como una dendrita y no como un alambre es el trazo que ADELGAZA hacia la punta. En canvas
// 2D es una propiedad del stroke; en three.js hay que construir geometría por segmento. Para
// ~4 000 segmentos, canvas 2D es menos código, menos memoria y se ve mejor.
//
// El render vive aparte de personas.mjs a propósito: allá es lógica pura que corre en
// `node --test`, acá hay DOM. Mezclarlos deja la lógica sin poder testear.

const IND = [79, 107, 255], JADE = [63, 208, 163], CYAN = [53, 208, 224];
// Paleta de PERSONAS. El color acá no es decorativo: es la única forma de saber de quién es
// cada racimo. Arranca por el acento de marca y sigue por la familia Aurora; ámbar queda
// último porque en el resto del sistema significa «aviso» y conviene no gastarlo.
const PALETA = [IND, JADE, [160, 107, 255], CYAN, [232, 178, 74]];

// PRNG con semilla: el árbol de cada neurona tiene que ser EL MISMO en cada recarga. Con
// Math.random(), dos personas mirando la misma pantalla ven dibujos distintos y no pueden
// hablar de lo que ven — y una captura de ayer deja de comparar con la de hoy.
const rng = (s) => () => ((s = (s * 1664525 + 1013904223) >>> 0) / 4294967296);

const add = (a, b) => [a[0] + b[0], a[1] + b[1], a[2] + b[2]];
const mul = (a, k) => [a[0] * k, a[1] * k, a[2] * k];
const norm = (a) => { const l = Math.hypot(a[0], a[1], a[2]) || 1; return [a[0] / l, a[1] / l, a[2] / l]; };
const cross = (a, b) => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]];

// desviar: gira una dirección `ang` radianes hacia un lado cualquiera del plano perpendicular.
function desviar(d, ang, r) {
  const up = Math.abs(d[1]) > 0.9 ? [1, 0, 0] : [0, 1, 0];
  const e1 = norm(cross(d, up)), e2 = norm(cross(d, e1));
  const t = r() * Math.PI * 2;
  const lat = add(mul(e1, Math.cos(t)), mul(e2, Math.sin(t)));
  return norm(add(mul(d, Math.cos(ang)), mul(lat, Math.sin(ang))));
}

export function crearVista(canvas) {
  const cx = canvas.getContext('2d');
  const cam = { yaw: 0.55, pitch: -0.22, dist: 1180, f: 1120, zoom: 1.24 };
  let W = 0, H = 0, DATOS = null, SEG = [], AX = [], NODOS = [], IDX = new Map();
  let foco = null, arrastrando = false, lx = 0, ly = 0, quieto = 0, vivo = false;
  const buf = [];
  const suave = !matchMedia('(prefers-reduced-motion: reduce)').matches;

  function medir() {
    const dpr = Math.min(devicePixelRatio || 1, 2);
    W = canvas.clientWidth; H = canvas.clientHeight;
    canvas.width = Math.round(W * dpr); canvas.height = Math.round(H * dpr);
    cx.setTransform(dpr, 0, 0, dpr, 0, 0);
  }

  // construir: de {terminales, despachos} a geometría 3D. Se llama sólo cuando cambian los
  // datos, no por frame: son ~4 000 segmentos y rehacerlos a 60 fps sería absurdo.
  function construir(datos, racimos) {
    DATOS = datos; SEG = []; AX = []; NODOS = []; IDX = new Map();
    if (!racimos.length) return;

    // un racimo por persona, repartidos en una fila con separación proporcional a su tamaño
    const anchoTotal = racimos.reduce((s, r) => s + Math.sqrt(r.notas), 0) || 1;
    let x = 0;
    const centros = racimos.map((r) => {
      const w = Math.sqrt(r.notas) / anchoTotal;
      const c = [(x + w / 2 - 0.5) * 1180, 0, 0];
      x += w;
      return c;
    });

    racimos.forEach((rac, ri) => {
      const col = PALETA[ri % PALETA.length];
      const c = centros[ri];
      const R = 105 + 62 * Math.sqrt(rac.terminales.length);
      rac.terminales.forEach((t, i) => {
        // esfera de Fibonacci: reparte sin apelmazar y es determinista
        const k = i + 0.5, phi = Math.acos(1 - 2 * k / rac.terminales.length);
        const th = Math.PI * (1 + Math.sqrt(5)) * k;
        const nd = {
          id: t.id, notas: t.notas, persona: rac.persona, col,
          r: Math.max(4.2, Math.sqrt(t.notas) * 1.15),
          pos: [c[0] + Math.cos(th) * Math.sin(phi) * R,
                c[1] + Math.cos(phi) * R * 0.78,
                c[2] + Math.sin(th) * Math.sin(phi) * R],
        };
        NODOS.push(nd); IDX.set(t.id, nd);
      });
    });

    // las dendritas
    let semilla = 7;
    for (const nd of NODOS) {
      const r = rng(semilla += 977);
      // La densidad está ACOTADA: cada segmento es un stroke por frame. Con profundidad 5 en
      // todas las neuronas pasaba de 7 000 trazos y el panel se arrastraba.
      const raices = Math.max(4, Math.min(7, Math.round(Math.log(nd.notas + 1) * 1.6)));
      const prof = nd.notas > 150 ? 5 : (nd.notas > 55 ? 4 : 3);
      const L0 = nd.r * (nd.notas > 120 ? 2.5 : 2.9);
      for (let i = 0; i < raices; i++) {
        const k = i + 0.5, phi = Math.acos(1 - 2 * k / raices);
        const th = Math.PI * (1 + Math.sqrt(5)) * k;
        const d0 = norm([Math.cos(th) * Math.sin(phi), Math.cos(phi), Math.sin(th) * Math.sin(phi)]);
        crecer(add(nd.pos, mul(d0, nd.r * 0.85)), d0, L0, Math.max(1.5, nd.r * 0.3), prof, nd, r);
      }
    }

    // los axones
    for (const d of DATOS.despachos) {
      const A = IDX.get(d.de), B = IDX.get(d.a);
      if (!A || !B) continue;
      const rr = rng(d.de.length * 131 + d.a.length * 17 + d.veces);
      const m = mul(add(A.pos, B.pos), 0.5);
      // el control se separa del eje para que la ida y la vuelta entre dos terminales no se
      // dibujen una encima de la otra y parezcan un solo despacho
      const ctrl = add(m, [(rr() - 0.5) * 150, -70 - d.veces * 4.5 - rr() * 60, (rr() - 0.5) * 150]);
      const P = [];
      for (let i = 0; i <= 26; i++) {
        const t = i / 26, u = 1 - t;
        P.push([u * u * A.pos[0] + 2 * u * t * ctrl[0] + t * t * B.pos[0],
                u * u * A.pos[1] + 2 * u * t * ctrl[1] + t * t * B.pos[1],
                u * u * A.pos[2] + 2 * u * t * ctrl[2] + t * t * B.pos[2]]);
      }
      AX.push({ de: d.de, a: d.a, veces: d.veces, P, cruza: A.persona !== B.persona });
    }
  }

  function crecer(p, d, largo, w, prof, nodo, r) {
    if (prof <= 0 || w < 0.28 || SEG.length > 26000) return;
    const hijos = r() < 0.28 ? 3 : 2;
    for (let i = 0; i < hijos; i++) {
      const d2 = desviar(d, 0.38 + r() * 0.42, r);
      const l2 = largo * (0.66 + r() * 0.16);
      const q = add(p, mul(d2, l2));
      SEG.push({ a: p, b: q, w, col: nodo.col, nodo: nodo.id });
      crecer(q, d2, l2, w * 0.6, prof - 1, nodo, r);
    }
  }

  function proy(p) {
    const cy = Math.cos(cam.yaw), sy = Math.sin(cam.yaw);
    const x = p[0] * cy - p[2] * sy, z = p[0] * sy + p[2] * cy, y = p[1];
    const cp = Math.cos(cam.pitch), sp = Math.sin(cam.pitch);
    const y2 = y * cp - z * sp, z2 = y * sp + z * cp;
    // OJO: el zoom mueve la CÁMARA, no la distancia focal. Dividir las dos por `zoom` se
    // cancela en el centro de la escena y el zoom no hace nada justo donde estás mirando.
    const zc = z2 + cam.dist / cam.zoom;
    const s = cam.f / Math.max(zc, 40);
    return { x: W / 2 + x * s, y: H / 2 + y2 * s, s, z: zc };
  }

  function pintar() {
    cx.clearRect(0, 0, W, H);
    if (!NODOS.length) {
      cx.fillStyle = 'rgba(154,157,171,.7)';
      cx.font = "13px 'JetBrains Mono', ui-monospace, monospace";
      cx.textAlign = 'center';
      cx.fillText('todavía no hay despachos entre terminales en esta memoria', W / 2, H / 2);
      return;
    }
    buf.length = 0;
    const lejos = cam.dist / cam.zoom * 1.9;

    for (const s of SEG) {
      const A = proy(s.a), B = proy(s.b);
      // una rama de menos de medio píxel no se ve pero cuesta un stroke igual
      if (Math.abs(A.x - B.x) + Math.abs(A.y - B.y) < 0.8) continue;
      buf.push({ z: (A.z + B.z) / 2, t: 0, A, B, s });
    }
    for (const ax of AX) {
      for (let i = 0; i < ax.P.length - 1; i++) {
        const A = proy(ax.P[i]), B = proy(ax.P[i + 1]);
        buf.push({ z: (A.z + B.z) / 2, t: 1, A, B, ax, ult: i === ax.P.length - 2 });
      }
    }
    for (const nd of NODOS) buf.push({ z: proy(nd.pos).z, t: 2, nd, P: proy(nd.pos) });
    buf.sort((a, b) => b.z - a.z);   // pintor: lo lejano primero

    for (const it of buf) {
      if (it.t === 0) {
        const off = foco && foco !== it.s.nodo;
        const p = Math.max(0.12, Math.min(1, 1.35 - it.A.z / lejos));
        const c = it.s.col;
        cx.strokeStyle = `rgba(${c[0]},${c[1]},${c[2]},${((off ? 0.055 : 0.3) * p).toFixed(3)})`;
        cx.lineWidth = Math.max(0.35, it.s.w * it.A.s * 1.15);
        cx.lineCap = 'round';
        cx.beginPath(); cx.moveTo(it.A.x, it.A.y); cx.lineTo(it.B.x, it.B.y); cx.stroke();
      } else if (it.t === 1) {
        const toca = foco && (foco === it.ax.de || foco === it.ax.a);
        const off = foco && !toca;
        const p = Math.max(0.2, Math.min(1, 1.35 - it.A.z / lejos));
        const c = it.ax.cruza ? CYAN : [138, 153, 255];
        const base = toca ? 0.95 : (off ? 0.05 : 0.34 + it.ax.veces * 0.022);
        cx.strokeStyle = `rgba(${c[0]},${c[1]},${c[2]},${(base * p).toFixed(3)})`;
        cx.lineWidth = Math.max(0.5, (0.7 + it.ax.veces * 0.16) * it.A.s * (toca ? 1.5 : 1));
        cx.beginPath(); cx.moveTo(it.A.x, it.A.y); cx.lineTo(it.B.x, it.B.y); cx.stroke();
        if (it.ult) {   // el despacho tiene DIRECCIÓN y se tiene que ver
          const an = Math.atan2(it.B.y - it.A.y, it.B.x - it.A.x);
          const L = Math.max(4, 7 * it.B.s * (toca ? 1.5 : 1));
          cx.fillStyle = `rgba(${c[0]},${c[1]},${c[2]},${(base * p).toFixed(3)})`;
          cx.beginPath();
          cx.moveTo(it.B.x, it.B.y);
          cx.lineTo(it.B.x - Math.cos(an - 0.42) * L, it.B.y - Math.sin(an - 0.42) * L);
          cx.lineTo(it.B.x - Math.cos(an + 0.42) * L, it.B.y - Math.sin(an + 0.42) * L);
          cx.closePath(); cx.fill();
        }
      } else {
        const nd = it.nd, P = it.P, off = foco && foco !== nd.id;
        const R = Math.max(2.2, nd.r * P.s * 1.5), c = nd.col;
        const g = cx.createRadialGradient(P.x, P.y, 0, P.x, P.y, R * 2.6);
        g.addColorStop(0, `rgba(${c[0]},${c[1]},${c[2]},${off ? 0.1 : 0.3})`);
        g.addColorStop(1, `rgba(${c[0]},${c[1]},${c[2]},0)`);
        cx.fillStyle = g; cx.beginPath(); cx.arc(P.x, P.y, R * 2.6, 0, 6.2832); cx.fill();
        cx.fillStyle = `rgba(${c[0]},${c[1]},${c[2]},${off ? 0.28 : 0.95})`;
        cx.beginPath(); cx.arc(P.x, P.y, R, 0, 6.2832); cx.fill();
        cx.fillStyle = `rgba(255,255,255,${off ? 0.06 : 0.22})`;
        cx.beginPath(); cx.arc(P.x, P.y, R * 0.42, 0, 6.2832); cx.fill();
        nd._sx = P.x; nd._sy = P.y; nd._sr = R;
        if (!off) {   // se etiquetan TODAS: una terminal chica sin nombre es un punto anónimo
          cx.fillStyle = foco === nd.id ? '#ECEAE3'
            : (nd.notas >= 60 ? 'rgba(236,234,227,.62)' : 'rgba(236,234,227,.34)');
          cx.font = `${Math.max(9, Math.min(12, 11 * P.s * 1.3))}px 'JetBrains Mono', ui-monospace, monospace`;
          cx.textAlign = 'center';
          cx.fillText(nd.id.toLowerCase(), P.x, P.y + R + 14);
        }
      }
    }
  }

  // ── interacción ──
  canvas.addEventListener('pointerdown', (e) => {
    arrastrando = true; lx = e.clientX; ly = e.clientY;
    try { canvas.setPointerCapture(e.pointerId); } catch (_) { /* sin captura se sigue pudiendo */ }
  });
  canvas.addEventListener('pointerup', () => { arrastrando = false; });
  canvas.addEventListener('pointermove', (e) => {
    if (arrastrando) {
      cam.yaw += (e.clientX - lx) * 0.006;
      cam.pitch = Math.max(-1.2, Math.min(1.2, cam.pitch + (e.clientY - ly) * 0.005));
      lx = e.clientX; ly = e.clientY; quieto = 0;
      return;
    }
    let mejor = null, dm = 1e9;
    const rc = canvas.getBoundingClientRect();
    for (const nd of NODOS) {
      if (nd._sx == null) continue;
      const d = Math.hypot(e.clientX - rc.left - nd._sx, e.clientY - rc.top - nd._sy);
      if (d < Math.max(16, nd._sr * 1.9) && d < dm) { dm = d; mejor = nd; }
    }
    const id = mejor ? mejor.id : null;
    if (id !== foco) { foco = id; if (alFocar) alFocar(mejor, DATOS); }
  });
  canvas.addEventListener('pointerleave', () => {
    if (foco !== null) { foco = null; if (alFocar) alFocar(null, DATOS); }
  });
  canvas.addEventListener('wheel', (e) => {
    e.preventDefault();
    cam.zoom = Math.max(0.42, Math.min(3.4, cam.zoom * (e.deltaY > 0 ? 1.09 : 0.917)));
  }, { passive: false });

  let alFocar = null;
  addEventListener('resize', () => { if (vivo) medir(); });

  return {
    // datos: {terminales, despachos} de extraerPersonas; racimos: agruparPorPersona(...)
    cargar(datos, racimos) { construir(datos, racimos); },
    activar(v) { vivo = v; if (v) medir(); },
    activa() { return vivo; },
    // frame: lo llama el bucle del dashboard. No abre su propio requestAnimationFrame para no
    // tener DOS bucles peleándose por el mismo frame cuando la lente está apagada.
    frame(motion) {
      if (!vivo) return;
      if (motion && suave && !arrastrando && !foco) { quieto++; if (quieto > 40) cam.yaw += 0.0016; }
      pintar();
    },
    onFoco(fn) { alFocar = fn; },
  };
}
