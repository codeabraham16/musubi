import os
#!/usr/bin/env python3
"""Numero final del track: ventanas LIMPIAS (no la contaminada) + simulacion de lo que falta."""
import json, os, glob
from collections import defaultdict
DIV = 2.6
# Raíz de los transcripts de Claude Code. NO va clavada: este script corre en los dos lados de la
# malla y la ruta difiere (davantis-1 es Windows). Overridealo con MUSUBI_TRANSCRIPTS=<ruta>.
_DEF = os.path.join(os.path.expanduser('~'), '.claude', 'projects')
RAIZ = os.environ.get('MUSUBI_TRANSCRIPTS') or _DEF
ES = lambda n: isinstance(n, str) and n.endswith('musubi_recall')
# ventana vieja: hasta el escalon del 10-27. limpia: desde que len(id) quedo parejo (11-09).
V = [('ANTES   (09-12 → 10-26)', '2026-09-01', '2026-10-27'),
     ('DESPUES (11-09 → 11-13, binario nuevo parejo)', '2026-11-09', '2099-01-01')]
def vent(d):
    for i,(_,a,b) in enumerate(V):
        if a <= d < b: return i
    return None
A = [defaultdict(float) for _ in V]
C = [defaultdict(int) for _ in V]
for fp in glob.glob(os.path.join(RAIZ,'*','*.jsonl')):
    try:
        tu={}
        for linea in open(fp, encoding='utf-8', errors='replace'):
            try: m=json.loads(linea)
            except Exception: continue
            ts=m.get('timestamp',''); msg=m.get('message') or {}
            cont=msg.get('content')
            if not isinstance(cont,list): continue
            for bl in cont:
                if not isinstance(bl,dict): continue
                if bl.get('type')=='tool_use' and ES(bl.get('name')): tu[bl.get('id')]=ts
                elif bl.get('type')=='tool_result':
                    tu_ts=tu.get(bl.get('tool_use_id'))
                    if tu_ts is None: continue
                    v=vent((ts or tu_ts)[:10])
                    if v is None: continue
                    c=bl.get('content')
                    txt=(''.join(x.get('text','') for x in c if isinstance(x,dict))
                         if isinstance(c,list) else (c if isinstance(c,str) else ''))
                    if not txt: continue
                    try: d=json.loads(txt)
                    except Exception: continue
                    it_=d.get('items') or []
                    if not isinstance(it_,list) or not it_: continue
                    A[v]['res']+=1; A[v]['items']+=len(it_); A[v]['chars']+=len(txt)
                    for it in it_:
                        if not isinstance(it,dict): continue
                        for k,val in it.items():
                            C[v][k]+=len(k)+3+len(val if isinstance(val,str) else json.dumps(val,ensure_ascii=False))
    except Exception: pass

tpi=[]
for v,(nom,_,_) in enumerate(V):
    a=A[v]
    if not a['items']: print('%s: sin datos' % nom); tpi.append(0); continue
    t=a['chars']/DIV/a['items']; tpi.append(t)
    print('── %s' % nom)
    print('   %d resultados · %d items · %.1f tok/item · %.0f tok por resultado'
          % (a['res'], a['items'], t, a['chars']/DIV/a['res']))
if tpi[0] and tpi[1]:
    print('\n🔑 AHORRO MEDIDO: %.1f → %.1f tok/item  =  −%.1f %%\n' % (tpi[0], tpi[1], 100*(1-tpi[1]/tpi[0])))

print('── LO QUE QUEDA POR GANAR (sobre la ventana nueva) ──')
tot=sum(C[1].values()) or 1
a=A[1]; base=a['chars']/DIV/a['items']
for cand,nom in [(['score'],'1.1  dropear score'),
                 (['full_tokens'],'2.1  dropear full_tokens'),
                 (['score','full_tokens'],'1.1 + 2.1 juntos'),
                 (['author','project_id'],'extra: author + project_id'),
                 (['score','full_tokens','author','project_id'],'todo lo de arriba')]:
    gan=sum(C[1].get(k,0) for k in cand)
    print('   %-28s %5.1f %% del payload  →  %.1f tok/item  (%.0f tok/resultado)'
          % (nom, 100*gan/tot, base*(1-gan/tot), (a['chars']-gan)/DIV/a['res']))
print('\n   campos del item hoy, por peso:')
for k,n in sorted(C[1].items(), key=lambda x:-x[1]):
    print('      %-14s %5.1f %%' % (k, 100*n/tot))
