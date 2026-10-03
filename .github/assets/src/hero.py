# Banner del README de Musubi: hero.svg (español) y hero.en.svg (inglés). CSS puro, sin <script>,
# sin filtros y sin recursos externos, porque GitHub sirve el SVG como <img>.
#
# Uso, desde cualquier carpeta:   python .github/assets/src/hero.py
# Escribe .github/assets/hero.svg y hero.en.svg. La salida es determinista: dos corridas dan los
# mismos bytes. Entradas, en esta carpeta: knot-path.txt (la curva del nudo, la misma del glifo del
# cuerpo) y seal-glyphs.json (contornos del sello 結び, de Noto Sans JP, SIL OFL 1.1).
#
# El estado BASE de cada elemento es el estado final y estático; las animaciones parten de
# «escondido» (from) y terminan ahí. Con prefers-reduced-motion se apagan todas y queda el final.
import re, math, bisect, random, pathlib, sys, json, os, tempfile

D = pathlib.Path(__file__).parent
OUT = D.parent  # .github/assets

# ── tokens del cuerpo ────────────────────────────────────────────────────────────────────────
GROUND, SURFACE, RAISED, LINE = "#0C1020", "#121734", "#182042", "#2A335C"
INK, MUTED, FAINT = "#E9ECF7", "#98A0C0", "#5A6390"
CORD, CORD_HI = "#6366F1", "#818CF8"
TEAL, VIOLET, GREEN = "#2dd4bf", "#a78bfa", "#4ade80"  # SÓLO dentro del nudo
SANS = "-apple-system,'Segoe UI',system-ui,'Helvetica Neue',Arial,sans-serif"
MONO = "ui-monospace,'SF Mono','Cascadia Code',Menlo,Consolas,monospace"
CJK = "'Hiragino Sans','Yu Gothic','Noto Sans CJK JP','Noto Sans JP',Meiryo,sans-serif"
if os.environ.get("HERO_FONT"):  # prueba de robustez: fuente de reemplazo ancha, p. ej. Verdana
    SANS = os.environ["HERO_FONT"]
    MONO = os.environ.get("HERO_MONO", "'Courier New',monospace")
W, H = 1200, 460
SEAL = json.loads((D / "seal-glyphs.json").read_text(encoding="utf-8"))  # contornos de Noto Sans JP (SIL OFL 1.1)

TXT = {
    "es": dict(
        lang="es",
        tag="Memoria persistente para agentes de IA",
        agent="Tu agente", agent_sub="Claude Code · Cursor", agent_mono="hooks + MCP",
        disk="Tu disco", disk_sub="SQLite local, en .musubi/", disk_mono="nada externo obligatorio",
        save="guarda lo que aprende", persist="persiste",
        back="solo lo relevante, solo lo nuevo",
        foot="servidor MCP en Go · local-first · model-free · MIT",
        alt="Musubi, 結び: memoria persistente para agentes de IA. Tu agente guarda lo que aprende, "
            "Musubi lo persiste en tu disco local y le devuelve solo lo relevante y solo lo nuevo.",
    ),
    "en": dict(
        lang="en",
        tag="Persistent memory for AI agents",
        agent="Your agent", agent_sub="Claude Code · Cursor", agent_mono="hooks + MCP",
        disk="Your disk", disk_sub="Local SQLite, in .musubi/", disk_mono="nothing external required",
        save="saves what it learns", persist="persists",
        back="only what's relevant, only what's new",
        foot="MCP server in Go · local-first · model-free · MIT",
        alt="Musubi, 結び: persistent memory for AI agents. Your agent saves what it learns, "
            "Musubi persists it on your local disk and gives back only what's relevant and only what's new.",
    ),
}

# ── el nudo: la polilínea del glifo, llevada a coordenadas finales ──────────────────────────
KD = (D / "knot-path.txt").read_text(encoding="utf-8").strip()  # la curva cerrada del glifo
NAT = [tuple(map(float, m.split(","))) for m in re.findall(r"[ML] ([-\d.]+,[-\d.]+)", KD)]
KS, KCX, KCY = 2.4, 600, 250
P = [(KCX + (x - 240) * KS, KCY + (y - 151.5) * KS) for x, y in NAT]  # cerrado: P[0] == P[-1]
CL = [0.0]
for a, b in zip(P, P[1:]):
    CL.append(CL[-1] + math.dist(a, b))
TOTAL = CL[-1]


def num(x):
    s = f"{x:.1f}"
    return s[:-2] if s.endswith(".0") else s


def poly(pts):
    return "M" + " L".join(f"{num(x)},{num(y)}" for x, y in pts)


def at(s):
    s %= TOTAL
    i = min(bisect.bisect_right(CL, s) - 1, len(P) - 2)
    t = (s - CL[i]) / (CL[i + 1] - CL[i])
    return (P[i][0] + t * (P[i + 1][0] - P[i][0]), P[i][1] + t * (P[i + 1][1] - P[i][1]))


def sub(s0, s1):
    m = max(4, int((s1 - s0) / 2.5))
    return [at(s0 + (s1 - s0) * k / m) for k in range(m + 1)]


def inter(p, q, r, s):
    d1 = (q[0] - p[0], q[1] - p[1])
    d2 = (s[0] - r[0], s[1] - r[1])
    den = d1[0] * d2[1] - d1[1] * d2[0]
    if abs(den) < 1e-9:
        return None
    t = ((r[0] - p[0]) * d2[1] - (r[1] - p[1]) * d2[0]) / den
    u = ((r[0] - p[0]) * d1[1] - (r[1] - p[1]) * d1[0]) / den
    return (t, u) if 0 <= t < 1 and 0 <= u < 1 else None


visits = []
N = len(P) - 1
for i in range(N):
    for j in range(i + 2, N):
        if i == 0 and j == N - 1:
            continue
        r = inter(P[i], P[i + 1], P[j], P[j + 1])
        if r:
            t, u = r
            pt = (P[i][0] + t * (P[i + 1][0] - P[i][0]), P[i][1] + t * (P[i + 1][1] - P[i][1]))
            visits.append((CL[i] + t * (CL[i + 1] - CL[i]), pt))
            visits.append((CL[j] + u * (CL[j + 1] - CL[j]), pt))
visits.sort()
assert len(visits) == 6, f"el nudo debería cruzarse 3 veces, hay {len(visits)//2}"
OVER = visits[0::2]  # alternado: cruza por encima en las visitas pares
for k in range(3):  # cada cruce tiene una visita por encima y otra por debajo
    pts = {(round(v[1][0], 3), round(v[1][1], 3)) for v in OVER}
assert len({(round(v[1][0], 2), round(v[1][1], 2)) for v in OVER}) == 3, "dos «por encima» en el mismo cruce"

CLIP_R = 21  # radio del círculo que recorta cada parche; cubre el solape de dos tubos que cruzan a >25°
# ángulo de cada cruce, para comprobar que CLIP_R alcanza
for _s, _pt in OVER:
    _a, _b = at(_s - 1), at(_s + 1)
    _o = [v for v in visits if abs(v[1][0] - _pt[0]) < 1e-6 and abs(v[1][1] - _pt[1]) < 1e-6 and abs(v[0] - _s) > 1]
    _c, _d = at(_o[0][0] - 1), at(_o[0][0] + 1)
    _t1 = math.atan2(_b[1] - _a[1], _b[0] - _a[0])
    _t2 = math.atan2(_d[1] - _c[1], _d[0] - _c[0])
    _ang = abs(math.degrees(_t1 - _t2)) % 180
    _ang = min(_ang, 180 - _ang)
    assert _ang > 25, f"cruce demasiado rasante ({_ang:.0f}°): CLIP_R no cubre el solape"
    print(f"cruce en ({_pt[0]:.0f},{_pt[1]:.0f}) a {_ang:.0f}°")

# ── el cordón de la escena y la vuelta ──────────────────────────────────────────────────────
CY_ = 250  # eje de la escena
CARD_W, CARD_H = 300, 128
AX, DX_ = 48, 852  # x de las tarjetas
CARD_Y = CY_ - CARD_H // 2 - 2
LEFT_CORD = (AX + CARD_W, CY_ + 4, 524, CY_ + 4)  # del borde de la tarjeta al nudo
RIGHT_CORD = (676, CY_ + 4, DX_, CY_ + 4)
BACK_Y = 396
BACK_D = (f"M{DX_ + CARD_W // 2},{CARD_Y + CARD_H + 4} V{BACK_Y - 36} Q{DX_ + CARD_W // 2},{BACK_Y} "
          f"{DX_ + CARD_W // 2 - 36},{BACK_Y} H{AX + CARD_W // 2 + 36} Q{AX + CARD_W // 2},{BACK_Y} "
          f"{AX + CARD_W // 2},{BACK_Y - 36} V{CARD_Y + CARD_H + 12}")

# ── el nombre: trazos propios, monolínea, con el mismo carácter que el nudo ─────────────────
WM_H, WM_W, WM_PITCH, WM_SW = 50, 36, 58, 6.5
WM_Y = 40


def letters():
    return {
        "M": "M0,50 L0,0 L18,29 L36,0 L36,50",
        "U": "M0,0 L0,32 A18,18 0 0 0 36,32 L36,0",
        "S": "M32.1,8.2 A15,12.5 0 1 0 18,25 A15,12.5 0 1 1 3.9,41.8",
        "B": "M0,0 L0,50 M0,0 L20,0 A12.5,12.5 0 0 1 20,25 L0,25 M0,25 L22,25 A12.5,12.5 0 0 1 22,50 L0,50",
        "I": "M5,0 L31,0 M18,0 L18,50 M5,50 L31,50",
    }


# ── tiempos (s) ─────────────────────────────────────────────────────────────────────────────
T_BG, T_CORD, D_CORD = 0.0, 0.2, 1.6
LIFE = D_CORD + 0.9  # el hilo vive lo que tarda en dibujarse más un respiro, y se va bajo el tubo
T_TUBE, T_WORD, T_TAG, T_SEAL = 1.7, 1.9, 2.8, 3.1
T_CARD1, T_CARD2, T_CONN, T_LABEL, T_BACK, T_FOOT = 2.9, 3.1, 3.5, 3.9, 4.0, 4.6
T_LOOP, LOOP = 5.2, 7.0
# Un trazo con dasharray «n 200» y pathLength=100 se esconde con un offset que lo deje fuera de [0,100]
# MÁS un margen: el cap redondo asoma hacia adentro del camino y dejaba un puntito varado en el extremo.
HID = -102  # reposo y final: el trazo ya pasó el final del camino


def pc(t):  # segundos dentro del ciclo -> porcentaje
    return f"{t / LOOP * 100:.2f}%"


def START(n):  # offset de salida para un trazo de largo n: queda justo antes del inicio
    return n + 2


def kf_run(name, t0, t1, n):
    """trazo de largo n que sale al segundo t0 del ciclo y llega al final del camino en t1"""
    head = "0%" if t0 == 0 else f"0%,{pc(t0)}"
    return (f"@keyframes {name}{{{head}{{stroke-dashoffset:{START(n)}}}"
            f"{pc(t1)}{{stroke-dashoffset:{HID}}}100%{{stroke-dashoffset:{HID}}}}}")


def build(lang):
    L = TXT[lang]
    Wt = 11.0  # grosor del tubo
    css = []
    add = css.append

    # ── keyframes ───────────────────────────────────────────────────────────────────────────
    add("@keyframes fadeIn{from{opacity:0}}")
    add("@keyframes rise{from{opacity:0;transform:translateY(14px)}}")
    add("@keyframes wdraw{from{stroke-dashoffset:110}}")
    add("@keyframes ddraw{from{stroke-dashoffset:110}}")
    add("@keyframes settle{0%{transform:scale(.9)}55%{transform:scale(1.035)}100%{transform:scale(1)}}")
    # cordón inicial: se dibuja, se sostiene y se va cuando el tubo ya está
    d = D_CORD / LIFE * 100
    add(f"@keyframes cordlife{{0%{{opacity:0;stroke-dashoffset:100}}1%{{opacity:1}}"
        f"{d:.1f}%{{opacity:1;stroke-dashoffset:0}}{d + 9:.1f}%{{opacity:1}}100%{{opacity:0;stroke-dashoffset:0}}}}")
    add(f"@keyframes spark{{0%{{opacity:0;stroke-dashoffset:0}}1%{{opacity:1}}"
        f"{d:.1f}%{{opacity:1;stroke-dashoffset:-100}}{d + 3:.1f}%{{opacity:0;stroke-dashoffset:-100}}100%{{opacity:0}}}}")
    add("@keyframes burst{0%{opacity:0;transform:scale(.5)}22%{opacity:1}100%{opacity:0;transform:scale(1.55)}}")
    add("@keyframes bob{0%,100%{transform:translateY(0)}50%{transform:translateY(-3px)}}")
    add("@keyframes breathe{0%,100%{opacity:.82}50%{opacity:1}}")
    add("@keyframes drift{0%{opacity:0;transform:translateY(0)}20%{opacity:.8}80%{opacity:.8}"
        "100%{opacity:0;transform:translateY(-38px)}}")
    add("@keyframes blink{0%,49%{opacity:1}50%,100%{opacity:.15}}")
    # ciclo de circulación (LOOP s): cada evento con su ventana
    add(kf_run("pktL", 0, 1.2, 12))
    add(kf_run("pktR", 1.5, 2.7, 12))
    add(kf_run("pktB", 3.3, 5.7, 12))
    add(f"@keyframes pulse{{0%,{pc(1.05)}{{transform:scale(1)}}{pc(1.35)}{{transform:scale(1.045)}}{pc(1.9)}{{transform:scale(1)}}100%{{transform:scale(1)}}}}")
    add(kf_run("glint", 1.15, 2.55, 11))
    add(f"@keyframes ring{{0%,{pc(1.2)}{{opacity:0;transform:scale(.55)}}{pc(1.3)}{{opacity:.5}}{pc(2.3)}{{opacity:0;transform:scale(1.9)}}100%{{opacity:0}}}}")
    for k, t0 in enumerate((2.7, 2.9, 3.1)):
        add(f"@keyframes slab{k}{{0%,{pc(t0)}{{opacity:0}}{pc(t0 + .25)}{{opacity:.75}}{pc(t0 + .9)}{{opacity:0}}100%{{opacity:0}}}}")
    add(f"@keyframes backglow{{0%,{pc(3.7)}{{opacity:0}}{pc(4.4)}{{opacity:1}}{pc(5.4)}{{opacity:0}}100%{{opacity:0}}}}")
    add(f"@keyframes recv{{0%,{pc(5.4)}{{opacity:0}}{pc(5.75)}{{opacity:.9}}{pc(6.4)}{{opacity:0}}100%{{opacity:0}}}}")

    # ── clases ──────────────────────────────────────────────────────────────────────────────
    ease = "cubic-bezier(.65,0,.35,1)"
    add(f".bgfx{{animation:fadeIn 1.4s ease-out {T_BG}s backwards}}")
    add(f".cord{{opacity:0;stroke-dasharray:100 200;animation:cordlife {LIFE}s linear {T_CORD}s both}}")
    add(f".spark{{opacity:0;stroke-dasharray:.01 200;animation:spark {LIFE}s linear {T_CORD}s both}}")
    add(f".tube{{animation:fadeIn .7s ease-out {T_TUBE}s backwards}}")
    add(f".settle{{transform-origin:{KCX}px {KCY}px;animation:settle 1s cubic-bezier(.3,.7,.3,1) {T_TUBE}s backwards}}")
    add(f".glowk{{animation:fadeIn .9s ease-out {T_TUBE}s backwards,breathe 7s ease-in-out {T_TUBE + 1}s infinite}}")
    add(f".burst{{opacity:0;transform-origin:{KCX}px {KCY}px;animation:burst 1.4s ease-out {T_TUBE}s both}}")
    add(f".bob{{animation:bob 7s ease-in-out {T_TUBE + 1}s infinite}}")
    add(f".pulse{{transform-origin:{KCX}px {KCY}px;animation:pulse {LOOP}s ease-in-out {T_LOOP}s infinite}}")
    add(f".ring{{opacity:0;transform-origin:{KCX}px {KCY}px;animation:ring {LOOP}s ease-out {T_LOOP}s infinite}}")
    # el destello y los paquetes nacen escondidos por el dash offset (más allá del final del trazo):
    # sin animación —reduced-motion, o antes de que arranque el ciclo— no se ve nada varado
    add(f".glint{{stroke-dasharray:7 200;stroke-dashoffset:{HID};animation:glint {LOOP}s linear {T_LOOP}s infinite}}")
    add(f".wl{{stroke-dasharray:100 200}}")
    for i in range(6):
        add(f".wl{i}{{animation:wdraw .7s {ease} {T_WORD + .12 * i:.2f}s backwards}}")
    add(f".tag{{animation:rise .9s cubic-bezier(.2,.7,.2,1) {T_TAG}s backwards}}")
    add(f".seal{{animation:fadeIn 1s ease-out {T_SEAL}s backwards}}")
    add(f".card1{{animation:rise .9s cubic-bezier(.2,.7,.2,1) {T_CARD1}s backwards}}")
    add(f".card2{{animation:rise .9s cubic-bezier(.2,.7,.2,1) {T_CARD2}s backwards}}")
    add(f".conn{{animation:fadeIn .8s ease-out {T_CONN}s backwards}}")
    add(f".lab{{animation:fadeIn .8s ease-out {T_LABEL}s backwards}}")
    add(f".back{{stroke-dasharray:100 200;animation:ddraw 1.3s {ease} {T_BACK}s backwards}}")
    add(f".backfx{{animation:fadeIn .8s ease-out {T_BACK + 1.1}s backwards}}")
    add(f".foot{{animation:fadeIn 1s ease-out {T_FOOT}s backwards}}")
    add(f".pkt{{stroke-dasharray:12 200;stroke-dashoffset:{HID}}}")
    for nm in ("L", "R", "B"):
        add(f".pkt{nm}{{animation:pkt{nm} {LOOP}s linear {T_LOOP}s infinite}}")
    for k in range(3):
        add(f".slab{k}{{opacity:0;animation:slab{k} {LOOP}s ease-in-out {T_LOOP}s infinite}}")
    add(f".backglow{{opacity:0;animation:backglow {LOOP}s ease-in-out {T_LOOP}s infinite}}")
    add(f".recv{{opacity:0;animation:recv {LOOP}s ease-in-out {T_LOOP}s infinite}}")
    add(f".cur{{animation:blink 1.1s steps(1) {T_CARD1 + 1}s infinite}}")
    add("@media (prefers-reduced-motion:reduce){*{animation:none!important}}")

    rnd = random.Random(7)
    parts = []
    A = parts.append

    A(f'<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 {W} {H}" '
      f'width="{W}" height="{H}" role="img" aria-label="{L["alt"]}">')
    A("<!-- 結び: contornos de Noto Sans JP (SIL Open Font License 1.1), convertidos a trazos -->")
    A(f"<title>Musubi</title><desc>{L['alt']}</desc>")
    A(f"<style>text{{font-family:{SANS}}}.mono{{font-family:{MONO}}}" + "".join(css) + "</style>")

    # defs
    A("<defs>")
    A(f'<linearGradient id="kg" gradientUnits="userSpaceOnUse" x1="{num(KCX - 40 * KS)}" y1="{num(KCY - 35.5 * KS)}" '
      f'x2="{num(KCX + 40 * KS)}" y2="{num(KCY + 35.5 * KS)}">'
      f'<stop offset="0" stop-color="{TEAL}"/><stop offset=".5" stop-color="{VIOLET}"/><stop offset="1" stop-color="{GREEN}"/>'
      "</linearGradient>")
    A(f'<radialGradient id="bloomA"><stop offset="0" stop-color="{CORD}" stop-opacity=".36"/>'
      f'<stop offset=".38" stop-color="{CORD}" stop-opacity=".14"/><stop offset="1" stop-color="{CORD}" stop-opacity="0"/></radialGradient>')
    A(f'<radialGradient id="flash"><stop offset="0" stop-color="{CORD_HI}" stop-opacity=".62"/>'
      f'<stop offset=".45" stop-color="{CORD}" stop-opacity=".2"/><stop offset="1" stop-color="{CORD}" stop-opacity="0"/></radialGradient>')
    A(f'<radialGradient id="bloomB"><stop offset="0" stop-color="{VIOLET}" stop-opacity=".26"/>'
      f'<stop offset="1" stop-color="{VIOLET}" stop-opacity="0"/></radialGradient>')
    A(f'<radialGradient id="topl" cx=".5" cy="0" r=".9"><stop offset="0" stop-color="{CORD_HI}" stop-opacity=".16"/>'
      f'<stop offset="1" stop-color="{CORD_HI}" stop-opacity="0"/></radialGradient>')
    A(f'<radialGradient id="vig" cx=".5" cy=".46" r=".78"><stop offset=".55" stop-color="#03050C" stop-opacity="0"/>'
      f'<stop offset="1" stop-color="#03050C" stop-opacity=".62"/></radialGradient>')
    A(f'<radialGradient id="gridfade" gradientUnits="userSpaceOnUse" cx="{KCX}" cy="{KCY}" r="600">'
      f'<stop offset="0" stop-color="{LINE}" stop-opacity=".62"/><stop offset=".55" stop-color="{LINE}" stop-opacity=".26"/>'
      f'<stop offset="1" stop-color="{LINE}" stop-opacity="0"/></radialGradient>')
    A(f'<radialGradient id="floor"><stop offset="0" stop-color="{CORD}" stop-opacity=".5"/>'
      f'<stop offset="1" stop-color="{CORD}" stop-opacity="0"/></radialGradient>')
    A(f'<linearGradient id="card" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="{RAISED}"/>'
      f'<stop offset="1" stop-color="{SURFACE}"/></linearGradient>')
    A('<linearGradient id="sheen" x1="0" y1="0" x2="1" y2="0"><stop offset="0" stop-color="#fff" stop-opacity="0"/>'
      '<stop offset=".5" stop-color="#fff" stop-opacity=".22"/><stop offset="1" stop-color="#fff" stop-opacity="0"/></linearGradient>')
    A(f'<linearGradient id="cordg" gradientUnits="userSpaceOnUse" x1="{AX + CARD_W}" y1="0" x2="{DX_}" y2="0">'
      f'<stop offset="0" stop-color="{CORD}" stop-opacity=".55"/><stop offset=".5" stop-color="{CORD}" stop-opacity=".18"/>'
      f'<stop offset="1" stop-color="{CORD}" stop-opacity=".55"/></linearGradient>')
    for k, (_, pt) in enumerate(OVER):
        A(f'<clipPath id="cl{k}"><circle cx="{num(pt[0])}" cy="{num(pt[1])}" r="{CLIP_R}"/></clipPath>')
    A(f'<path id="kn" pathLength="100" d="{poly(P[:-1])} Z"/>')
    A(f'<path id="pl" pathLength="100" d="M{LEFT_CORD[0]},{LEFT_CORD[1]} H{LEFT_CORD[2]}"/>')
    A(f'<path id="pr" pathLength="100" d="M{RIGHT_CORD[0]},{RIGHT_CORD[1]} H{RIGHT_CORD[2]}"/>')
    A(f'<path id="pb" pathLength="100" d="{BACK_D}"/>')
    A("</defs>")

    # fondo
    A(f'<rect width="{W}" height="{H}" rx="8" fill="{GROUND}"/>')
    A('<g class="bgfx">')
    A(f'<rect width="{W}" height="{H}" rx="8" fill="url(#topl)"/>')
    vx = "".join(f"M{x},0V{H}" for x in range(0, W + 1, 40))
    hy = "".join(f"M0,{y}H{W}" for y in range(10, H + 1, 40))
    A(f'<path d="{vx}{hy}" stroke="url(#gridfade)" stroke-width="1" fill="none"/>')
    A(f'<circle cx="{KCX}" cy="{KCY}" r="440" fill="url(#bloomA)"/>')
    A(f'<circle cx="{KCX}" cy="{KCY}" r="190" fill="url(#bloomB)"/>')
    A("</g>")
    # polvo en el aire, tres planos de tamaño
    for _ in range(10):
        x, y = rnd.uniform(70, 1130), rnd.uniform(60, 430)
        r = rnd.choice((.9, 1.2, 1.6))
        dur, dl = rnd.uniform(9, 17), rnd.uniform(4, 9)
        A(f'<circle cx="{x:.0f}" cy="{y:.0f}" r="{r}" fill="{INK}" fill-opacity=".38" opacity="0" '
          f'style="animation:drift {dur:.1f}s ease-in-out {dl:.1f}s infinite"/>')
    # suelo bajo el nudo
    A(f'<ellipse cx="{KCX}" cy="{KCY + 100}" rx="118" ry="11" fill="url(#floor)"/>')

    # brillo difuso y sombra del nudo (estáticos; el nudo en sí flota)
    A('<g class="glowk" fill="none" stroke-linecap="round" stroke-linejoin="round">')
    for w_, a in ((Wt + 38, .05), (Wt + 26, .07), (Wt + 15, .12)):
        A(f'<use href="#kn" stroke="url(#kg)" stroke-opacity="{a}" stroke-width="{w_}"/>')
    A(f'<use href="#kn" y="12" stroke="#03050C" stroke-opacity=".30" stroke-width="{Wt + 12}"/>')
    A("</g>")

    # cordón de la escena (pasa por detrás de las tarjetas y del nudo)
    A('<g class="conn" fill="none" stroke-linecap="round">')
    A(f'<use href="#pl" stroke="url(#cordg)" stroke-width="2"/><use href="#pr" stroke="url(#cordg)" stroke-width="2"/>')
    A(f'<circle cx="{LEFT_CORD[2]}" cy="{LEFT_CORD[3]}" r="3.4" fill="{CORD_HI}" fill-opacity=".8"/>'
      f'<circle cx="{RIGHT_CORD[0]}" cy="{RIGHT_CORD[1]}" r="3.4" fill="{CORD_HI}" fill-opacity=".8"/>')
    A("</g>")
    A('<g fill="none" stroke-linecap="round">')
    for nm in ("pl", "pr"):
        k = "L" if nm == "pl" else "R"
        A(f'<use href="#{nm}" class="pkt pkt{k}" stroke="{CORD_HI}" stroke-width="5"/>')
    A("</g>")

    # la vuelta: la promesa del producto, el único trazo vivo en índigo
    A('<g fill="none" stroke-linecap="round" stroke-linejoin="round">')
    A(f'<use href="#pb" class="back" stroke="{CORD}" stroke-width="2.2"/>')
    A(f'<use href="#pb" class="pkt pktB" stroke="#fff" stroke-opacity=".9" stroke-width="5"/>')
    A("</g>")
    ax, ay = AX + CARD_W // 2, CARD_Y + CARD_H + 12
    A(f'<path class="backfx" d="M{ax - 7},{ay + 9} L{ax},{ay - 1} L{ax + 7},{ay + 9}" fill="none" stroke="{CORD}" '
      f'stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>')

    # tarjetas con elevación: sombra en capas, cuerpo, filo superior
    def card(x, cls):
        s = f'<g class="{cls}">'
        for dy, g, al in ((18, 8, .06), (11, 5, .09), (6, 2.5, .13), (2, 1, .2)):
            s += (f'<rect x="{x - g}" y="{CARD_Y + dy - g}" width="{CARD_W + 2 * g}" height="{CARD_H + 2 * g}" '
                  f'rx="{8 + g}" fill="#03050C" fill-opacity="{al}"/>')
        s += (f'<rect x="{x}" y="{CARD_Y}" width="{CARD_W}" height="{CARD_H}" rx="8" fill="url(#card)" stroke="{LINE}"/>'
              f'<rect x="{x + 10}" y="{CARD_Y + .5}" width="{CARD_W - 20}" height="1" fill="url(#sheen)"/>')
        return s

    # — agente
    A(card(AX, "card1"))
    ix, iy = AX + 24, CARD_Y + 30
    A(f'<rect x="{ix}" y="{iy}" width="46" height="38" rx="6" fill="{GROUND}" fill-opacity=".7" stroke="{MUTED}" stroke-width="1.8"/>')
    A(f'<path d="M{ix + 1},{iy + 10} H{ix + 45}" stroke="{MUTED}" stroke-opacity=".5" stroke-width="1.4"/>')
    A(f'<circle cx="{ix + 8}" cy="{iy + 5}" r="1.5" fill="{MUTED}" fill-opacity=".7"/><circle cx="{ix + 14}" cy="{iy + 5}" r="1.5" fill="{MUTED}" fill-opacity=".7"/>')
    A(f'<path d="M{ix + 9},{iy + 19} L{ix + 16},{iy + 24} L{ix + 9},{iy + 29}" fill="none" stroke="{INK}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>')
    A(f'<path class="cur" d="M{ix + 21},{iy + 30} H{ix + 31}" stroke="{CORD_HI}" stroke-width="2.4" stroke-linecap="round"/>')
    tx = AX + 90
    A(f'<text x="{tx}" y="{CARD_Y + 46}" font-size="21" font-weight="600" fill="{INK}">{L["agent"]}</text>')
    A(f'<text x="{tx}" y="{CARD_Y + 72}" font-size="16" fill="{MUTED}">{L["agent_sub"]}</text>')
    A(f'<text class="mono" x="{tx}" y="{CARD_Y + 98}" font-size="13" fill="{MUTED}" fill-opacity=".8">{L["agent_mono"]}</text>')
    A(f'<rect class="recv" x="{AX}" y="{CARD_Y}" width="{CARD_W}" height="{CARD_H}" rx="8" fill="{CORD}" fill-opacity=".10" stroke="{CORD_HI}" stroke-width="1.5"/>')
    A("</g>")

    # — disco (cilindro con tres losas que se encienden al persistir)
    A(card(DX_, "card2"))
    cx_, top_ = DX_ + 46, CARD_Y + 28
    rx_, ry_, hs = 22, 8, 13  # radio, semieje vertical, alto de losa
    ys = [top_ + ry_ + k * hs for k in range(4)]
    for k in range(3):
        y0, y1 = ys[k], ys[k + 1]
        A(f'<path class="slab{k}" d="M{cx_ - rx_},{y0} A{rx_},{ry_} 0 0 0 {cx_ + rx_},{y0} L{cx_ + rx_},{y1} '
          f'A{rx_},{ry_} 0 0 1 {cx_ - rx_},{y1} Z" fill="{CORD}" fill-opacity=".75"/>')
    A(f'<g fill="none" stroke="{MUTED}" stroke-width="1.8" stroke-linecap="round">')
    A(f'<ellipse cx="{cx_}" cy="{top_ + ry_}" rx="{rx_}" ry="{ry_}" fill="{GROUND}" fill-opacity=".7"/>')
    A(f'<path d="M{cx_ - rx_},{top_ + ry_} V{ys[3]} A{rx_},{ry_} 0 0 0 {cx_ + rx_},{ys[3]} V{top_ + ry_}"/>')
    for k in (1, 2):
        A(f'<path d="M{cx_ - rx_},{ys[k]} A{rx_},{ry_} 0 0 0 {cx_ + rx_},{ys[k]}" stroke-opacity=".6"/>')
    A("</g>")
    tx = DX_ + 90
    A(f'<text x="{tx}" y="{CARD_Y + 46}" font-size="21" font-weight="600" fill="{INK}">{L["disk"]}</text>')
    A(f'<text x="{tx}" y="{CARD_Y + 72}" font-size="16" fill="{MUTED}">{L["disk_sub"]}</text>')
    A(f'<text class="mono" x="{tx}" y="{CARD_Y + 98}" font-size="13" fill="{MUTED}" fill-opacity=".8">{L["disk_mono"]}</text>')
    A("</g>")

    # etiquetas del cordón
    A(f'<g class="lab" font-size="15.5" fill="{MUTED}" text-anchor="middle">')
    A(f'<text x="{(LEFT_CORD[0] + LEFT_CORD[2]) // 2}" y="{CY_ - 12}">{L["save"]}</text>')
    A(f'<text x="{(RIGHT_CORD[0] + RIGHT_CORD[2]) // 2}" y="{CY_ - 12}">{L["persist"]}</text>')
    A("</g>")

    # etiqueta de la vuelta, en una pastilla que corta la línea
    pw = 372 if lang == "es" else 396
    A('<g class="backfx">')
    A(f'<rect x="{KCX - pw // 2}" y="{BACK_Y - 17}" width="{pw}" height="34" rx="17" fill="{SURFACE}" stroke="{CORD}" stroke-opacity=".7"/>')
    A(f'<rect class="backglow" x="{KCX - pw // 2}" y="{BACK_Y - 17}" width="{pw}" height="34" rx="17" fill="{CORD}" fill-opacity=".22" stroke="{CORD_HI}"/>')
    A(f'<text x="{KCX}" y="{BACK_Y + 5.5}" font-size="16" font-weight="600" fill="{CORD_HI}" text-anchor="middle">{L["back"]}</text>')
    A("</g>")

    # ── el nudo: cordón que se anuda, tubo con luz, anillo, destello ────────────────────────
    A(f'<use href="#kn" class="cord" fill="none" stroke="url(#kg)" stroke-width="3.4" stroke-linecap="round" stroke-linejoin="round"/>')
    A(f'<use href="#kn" class="spark" fill="none" stroke="#fff" stroke-width="9" stroke-linecap="round"/>')
    A(f'<circle class="burst" cx="{KCX}" cy="{KCY}" r="175" fill="url(#flash)"/>')
    A(f'<g class="ring" fill="none" stroke="{CORD_HI}"><circle cx="{KCX}" cy="{KCY}" r="100" stroke-opacity=".10" stroke-width="14"/>'
      f'<circle cx="{KCX}" cy="{KCY}" r="100" stroke-opacity=".5" stroke-width="1.3"/></g>')

    def main_el(attrs, dx, dy):
        return f'<use href="#kn" {attrs} x="{num(dx)}" y="{num(dy)}"/>'

    def patch_el(d):
        def f(attrs, dx, dy):
            return f'<path d="{d}" {attrs} transform="translate({num(dx)},{num(dy)})"/>'
        return f

    def tube(el):
        o = el(f'stroke="#070A18" stroke-opacity=".95" stroke-width="{Wt + 3.6}"', 0, 0)
        o += el(f'stroke="url(#kg)" stroke-width="{Wt}"', 0, 0)
        o += el(f'stroke="#060818" stroke-opacity=".30" stroke-width="{Wt * .5:.1f}"', .2 * Wt, .26 * Wt)
        o += el(f'stroke="#fff" stroke-opacity=".18" stroke-width="{Wt * .46:.1f}"', -.1 * Wt, -.15 * Wt)
        o += el(f'stroke="#fff" stroke-opacity=".8" stroke-width="{Wt * .15:.1f}"', -.18 * Wt, -.24 * Wt)
        return o

    A('<g class="tube"><g class="settle"><g class="bob"><g class="pulse">')
    A('<g fill="none" stroke-linecap="round" stroke-linejoin="round">' + tube(main_el) + "</g>")
    # el cruce de verdad: en cada «por encima» se vuelve a pasar el tramo, y su borde oscuro abre el hueco
    A('<g fill="none" stroke-linecap="butt" stroke-linejoin="round">')
    for k, (s, _) in enumerate(OVER):
        # el parche se recorta con un círculo: así todas sus capas terminan en el mismo borde,
        # que coincide con el tubo de abajo (sin el escalón del doble alfa en las puntas)
        A(f'<g clip-path="url(#cl{k})">' + tube(patch_el(poly(sub(s - CLIP_R - 14, s + CLIP_R + 14)))) + "</g>")
    A("</g>")
    A('<g fill="none" stroke-linecap="round" stroke-linejoin="round">')
    A(f'<use href="#kn" class="glint" stroke="#fff" stroke-opacity=".28" stroke-width="{Wt * 1.9:.1f}" style="stroke-dasharray:11 200"/>')
    A(f'<use href="#kn" class="glint" stroke="#fff" stroke-opacity=".9" stroke-width="{Wt * .42:.1f}"/>')
    A("</g>")
    A("</g></g></g></g>")

    # ── el nombre ───────────────────────────────────────────────────────────────────────────
    total_w = 5 * WM_PITCH + WM_W
    x0 = KCX - total_w / 2
    A('<g fill="none" stroke-linecap="round" stroke-linejoin="round">')
    lt = letters()
    for i, ch in enumerate("MUSUBI"):
        gx = x0 + i * WM_PITCH
        for w_, col, al in ((WM_SW + 15, CORD, .07), (WM_SW + 7, CORD_HI, .11), (WM_SW, INK, 1)):
            A(f'<path class="wl wl{i}" pathLength="100" transform="translate({num(gx)},{WM_Y})" d="{lt[ch]}" '
              f'stroke="{col}" stroke-opacity="{al}" stroke-width="{num(w_)}"/>')
    A("</g>")
    A(f'<text class="tag" x="{KCX}" y="{WM_Y + WM_H + 40}" font-size="22" fill="{MUTED}" text-anchor="middle">{L["tag"]}</text>')
    # sello 結び
    sx, sy = W - 48 - 40, 36
    A(f'<g class="seal"><rect x="{sx}" y="{sy}" width="40" height="64" rx="6" fill="none" stroke="{LINE}" stroke-width="1.4"/>')
    for gname, gy in (("uni7D50", sy + 20), ("uni3073", sy + 44)):  # 結 y び, en trazos: no dependen de ninguna fuente
        g = SEAL[gname]
        x0_, y0_, x1_, y1_ = g["bounds"]
        k = 22 / max(x1_ - x0_, y1_ - y0_)
        A(f'<path transform="translate({num(sx + 20 - (x0_ + x1_) / 2 * k)},{num(gy + (y0_ + y1_) / 2 * k)}) '
          f'scale({k:.5f},{-k:.5f})" fill="{MUTED}" fill-opacity=".9" d="{g["d"]}"/>')
    A("</g>")

    # pie
    A(f'<text class="foot mono" x="{KCX}" y="{H - 24}" font-size="14" fill="{MUTED}" fill-opacity=".85" text-anchor="middle">{L["foot"]}</text>')

    # viñeta y marco, por encima de todo lo de fondo
    A(f'<rect width="{W}" height="{H}" rx="8" fill="url(#vig)" pointer-events="none"/>')
    A(f'<rect x=".5" y=".5" width="{W - 1}" height="{H - 1}" rx="8" fill="none" stroke="{LINE}"/>')
    A("</svg>")
    return "\n".join(parts) + "\n"


if __name__ == "__main__":
    prueba = bool(os.environ.get("HERO_FONT"))  # con otra fuente se escribe al temporal, no al repo
    for lang in ("es", "en"):
        svg = build(lang)
        nombre = "hero.svg" if lang == "es" else "hero.en.svg"
        destino = (pathlib.Path(tempfile.gettempdir()) if prueba else OUT) / nombre
        destino.write_text(svg, encoding="utf-8", newline="\n")
        print(destino, len(svg.encode("utf-8")), "bytes")
