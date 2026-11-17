import os
#!/usr/bin/env python3
"""Mide el payload real de musubi_recall en los transcripts, por ventana temporal.

Reconstruye la medicion del proposal specs/recall-envoltorio/proposal.md:
  - tokens por resultado y por item (divJSON = 2.6 chars/token, el estimador de Musubi)
  - ratio payload / token_budget pedido
  - desglose de chars por campo
Es SOLO LECTURA. No escribe nada fuera de su propia salida.
"""
import json, os, sys, glob
from collections import defaultdict

DIV = 2.6  # chars/token, estimador de Musubi para JSON
# Raíz de los transcripts de Claude Code. NO va clavada: este script corre en los dos lados de la
# malla y la ruta difiere (davantis-1 es Windows). Overridealo con MUSUBI_TRANSCRIPTS=<ruta>.
_DEF = os.path.join(os.path.expanduser('~'), '.claude', 'projects')
RAIZ = os.environ.get('MUSUBI_TRANSCRIPTS') or _DEF
# ventanas: (nombre, desde, hasta) en ISO
VENTANAS = [
    ('ANTES  (plan, 09-10→10-24)', '2026-09-10', '2026-10-25'),
    ('DESPUES (post Tier2a, 11-02→)', '2026-11-02', '2099-01-01'),
]

def ventana_de(ts):
    if not ts: return None
    d = ts[:10]
    for i, (_, a, b) in enumerate(VENTANAS):
        if a <= d < b: return i
    return None

ES_RECALL = lambda n: isinstance(n, str) and n.endswith('musubi_recall')

acc = [defaultdict(float) for _ in VENTANAS]
campos = [defaultdict(int) for _ in VENTANAS]
archivos = leidos = 0

for fp in glob.glob(os.path.join(RAIZ, '*', '*.jsonl')):
    archivos += 1
    try:
        tool_use = {}   # id -> (nombre, token_budget, ts)
        with open(fp, encoding='utf-8', errors='replace') as fh:
            for linea in fh:
                try: m = json.loads(linea)
                except Exception: continue
                ts = m.get('timestamp', '')
                msg = m.get('message') or {}
                cont = msg.get('content')
                if not isinstance(cont, list): continue
                for bl in cont:
                    if not isinstance(bl, dict): continue
                    t = bl.get('type')
                    if t == 'tool_use' and ES_RECALL(bl.get('name')):
                        inp = bl.get('input') or {}
                        tool_use[bl.get('id')] = (bl['name'], inp.get('token_budget'), ts)
                    elif t == 'tool_result':
                        ref = tool_use.get(bl.get('tool_use_id'))
                        if not ref: continue
                        _, budget, ts_uso = ref
                        v = ventana_de(ts or ts_uso)
                        if v is None: continue
                        c = bl.get('content')
                        if isinstance(c, list):
                            txt = ''.join(x.get('text','') for x in c if isinstance(x, dict))
                        else:
                            txt = c if isinstance(c, str) else json.dumps(c)
                        if not txt: continue
                        a = acc[v]
                        a['resultados'] += 1
                        a['chars'] += len(txt)
                        try:
                            d = json.loads(txt)
                        except Exception:
                            a['no_json'] += 1; continue
                        items = d.get('items') or d.get('results') or []
                        if not isinstance(items, list): items = []
                        a['items'] += len(items)
                        if isinstance(budget, (int, float)) and budget > 0:
                            a['con_budget'] += 1
                            a['budget_sum'] += budget
                            a['ratio_sum'] += (len(txt)/DIV) / budget
                        ut = d.get('used_tokens')
                        if isinstance(ut, (int, float)):
                            a['used_tok_sum'] += ut; a['con_used'] += 1
                        for it in items:
                            if not isinstance(it, dict): continue
                            for k, val in it.items():
                                campos[v][k] += len(k) + 3 + len(val if isinstance(val, str) else json.dumps(val, ensure_ascii=False))
        leidos += 1
    except Exception:
        pass

print('archivos .jsonl recorridos: %d (leidos %d)\n' % (archivos, leidos))
for v, (nom, a, b) in enumerate(VENTANAS):
    A = acc[v]
    if not A['resultados']:
        print('── %s ──\n  sin datos\n' % nom); continue
    res, its, ch = A['resultados'], A['items'], A['chars']
    print('── %s ──' % nom)
    print('  resultados de recall  : %d' % res)
    print('  items                 : %d  (%.1f por resultado)' % (its, its/res))
    print('  payload               : %.0f chars = %.0f tok  (divJSON %.1f)' % (ch, ch/DIV, DIV))
    print('  por resultado         : %.0f tok' % (ch/DIV/res))
    if its: print('  por item              : %.1f tok' % (ch/DIV/its))
    if A['con_budget']:
        print('  ratio payload/budget  : %.2f×   (sobre %d llamadas con budget, mediana pedida %.0f)'
              % (A['ratio_sum']/A['con_budget'], A['con_budget'], A['budget_sum']/A['con_budget']))
    if A['con_used']:
        print('  used_tokens informado : %.0f prom  vs  %.0f real  -> %s'
              % (A['used_tok_sum']/A['con_used'], ch/DIV/res,
                 'HONESTO' if abs(A['used_tok_sum']/A['con_used'] - ch/DIV/res) < 0.35*(ch/DIV/res) else 'MIENTE'))
    tot = sum(campos[v].values()) or 1
    print('  desglose por campo (%% de los chars de items):')
    for k, n in sorted(campos[v].items(), key=lambda x: -x[1])[:10]:
        print('      %-16s %5.1f%%' % (k, 100*n/tot))
    print()
