import os
#!/usr/bin/env python3
"""Serie temporal del payload de recall: busca el ESCALON del cambio, no el promedio.

Si el adelgazamiento rige desde una fecha, el tok/item cae en escalon. Un promedio
tibio sobre una ventana contaminada (sesiones con binario viejo) se ve igual que
'el cambio casi no sirvio', y son cosas distintas.
"""
import json, os, glob
from collections import defaultdict

DIV = 2.6
# Raíz de los transcripts de Claude Code. NO va clavada: este script corre en los dos lados de la
# malla y la ruta difiere (davantis-1 es Windows). Overridealo con MUSUBI_TRANSCRIPTS=<ruta>.
_DEF = os.path.join(os.path.expanduser('~'), '.claude', 'projects')
RAIZ = os.environ.get('MUSUBI_TRANSCRIPTS') or _DEF
ES_RECALL = lambda n: isinstance(n, str) and n.endswith('musubi_recall')

por_dia = defaultdict(lambda: {'res':0,'items':0,'chars':0,'idlen':0,'idn':0,
                               'con_score':0,'sin_score':0,'ca':[], 'used':0,'usedn':0})

for fp in glob.glob(os.path.join(RAIZ, '*', '*.jsonl')):
    try:
        tu = {}
        with open(fp, encoding='utf-8', errors='replace') as fh:
            for linea in fh:
                try: m = json.loads(linea)
                except Exception: continue
                ts = m.get('timestamp','')
                msg = m.get('message') or {}
                cont = msg.get('content')
                if not isinstance(cont, list): continue
                for bl in cont:
                    if not isinstance(bl, dict): continue
                    if bl.get('type') == 'tool_use' and ES_RECALL(bl.get('name')):
                        tu[bl.get('id')] = ts
                    elif bl.get('type') == 'tool_result':
                        ts_uso = tu.get(bl.get('tool_use_id'))
                        if ts_uso is None: continue
                        dia = (ts or ts_uso)[:10]
                        if dia < '2026-09-01': continue
                        c = bl.get('content')
                        txt = (''.join(x.get('text','') for x in c if isinstance(x,dict))
                               if isinstance(c, list) else (c if isinstance(c,str) else ''))
                        if not txt: continue
                        try: d = json.loads(txt)
                        except Exception: continue
                        items = d.get('items') or []
                        if not isinstance(items, list) or not items: continue
                        a = por_dia[dia]
                        a['res'] += 1; a['items'] += len(items); a['chars'] += len(txt)
                        ut = d.get('used_tokens')
                        if isinstance(ut,(int,float)): a['used'] += ut; a['usedn'] += 1
                        for it in items:
                            if not isinstance(it, dict): continue
                            i = it.get('id')
                            if isinstance(i, str): a['idlen'] += len(i); a['idn'] += 1
                            if 'score' in it: a['con_score'] += 1
                            else: a['sin_score'] += 1
                            ca = it.get('created_at')
                            if isinstance(ca, str) and len(a['ca']) < 2: a['ca'].append(ca)
    except Exception: pass

print('%-11s %4s %6s %8s %7s %8s %9s  %s' % ('dia','res','items','tok/item','len(id)','%score','used/real','ejemplo created_at'))
for dia in sorted(por_dia):
    a = por_dia[dia]
    if not a['items']: continue
    tpi = a['chars']/DIV/a['items']
    idl = a['idlen']/a['idn'] if a['idn'] else 0
    tot = a['con_score'] + a['sin_score']
    pct = 100*a['con_score']/tot if tot else 0
    ur = (a['used']/a['usedn'])/(a['chars']/DIV/a['res']) if a['usedn'] else 0
    print('%-11s %4d %6d %8.1f %7.1f %7.0f%% %9.2f  %s' % (
        dia, a['res'], a['items'], tpi, idl, pct, ur, (a['ca'] or ['—'])[0]))
