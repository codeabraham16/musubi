import os
#!/usr/bin/env python3
"""Mezcla: items-por-llamada y budget pedido, para separar efecto de mezcla."""
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
PORLL = [[] for _ in V]
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
                if bl.get('type')=='tool_use' and ES(bl.get('name')):
                        inp=bl.get('input') or {}
                        tu[bl.get('id')]=(ts, inp.get('token_budget'))
                elif bl.get('type')=='tool_result':
                    par=tu.get(bl.get('tool_use_id'))
                    if par is None: continue
                    tu_ts, bud = par
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
                    PORLL[v].append((len(it_), bud, len(txt)/DIV/len(it_)))
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
    import statistics as st
print('%-26s %8s %9s %9s %9s %9s' % ('ventana','llamadas','items/ll','budget','tok/item','tok/res'))
for i,(nom,_,_) in enumerate(V):
    d=PORLL[i]
    if not d: print('%-26s (sin datos)'%nom[:26]); continue
    its=[x[0] for x in d]; buds=[x[1] for x in d if isinstance(x[1],(int,float))]
    tki=[x[2] for x in d]
    print('%-26s %8d %9.1f %9s %9.1f %9.0f' % (nom[:26], len(d), st.mean(its),
          ('%.0f'%st.median(buds)) if buds else 'n/d', st.mean(tki), st.mean(tki)*st.mean(its)))
    print('%-26s    mediana items=%.0f  p90 items=%.0f  budget p90=%s  llamadas con budget=%d/%d' % (
        '', st.median(its), sorted(its)[int(.9*(len(its)-1))],
        ('%.0f'%sorted(buds)[int(.9*(len(buds)-1))]) if buds else 'n/d', len(buds), len(d)))
