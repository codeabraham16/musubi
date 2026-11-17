# Scripts de medición del envoltorio del `recall`

Están **en el repo a propósito**. La §1 de `../proposal.md` citaba
`scratchpad/{recall_args,recall_ratio,field_breakdown}.py` y esos archivos ya **no existen**: el scratchpad
de una sesión se borra al reiniciarla, así que la receta de cómo se midió el track se perdió y hubo que
reescribirla. Una receta fuera del árbol envejece callada.

Todos miden sobre los transcripts `.jsonl` de Claude Code. La raíz sale del entorno:

```bash
MUSUBI_TRANSCRIPTS=~/.claude/projects python3 -I specs/recall-envoltorio/scripts/<script>.py
```

Sin la variable usan `~/.claude/projects`. Tardan minutos: barren todos los `.jsonl` del disco.

| script | qué contesta |
|---|---|
| `medir_envoltorio.py` | payload por ventana: tok/ítem, ratio contra el budget, desglose por campo |
| `serie_envoltorio.py` | **la serie POR DÍA** — `len(id)`, `%score`, `used/real`. La que encuentra los escalones |
| `cierre_envoltorio.py` | el número final con ventanas limpias + simulación de lo que falta recortar |
| `mezcla_budget.py` | separa el efecto del cambio de la mezcla de llamadas: ítems/llamada y budget pedido |

## El gotcha que te va a morder

**No midas desde el día del merge.** El id corto se mergeó el 2026-11-02 y tardó hasta el **11-09** en
imponerse, porque cada sesión ya abierta siguió con el binario viejo. La ventana que arranca en el merge da
**−5,1 %**; la que arranca en el escalón real da **−8,1 %**. Una ventana contaminada se ve igual que «el
cambio no sirvió».

Por eso el orden es: `serie_envoltorio.py` primero, para **encontrar** el escalón; recién después
`cierre_envoltorio.py` con la ventana que la serie te mostró. El promedio esconde el escalón que la serie
encuentra.

Y antes de concluir que el ahorro no se tradujo en nada, corré `mezcla_budget.py`: en este track el tok/ítem
bajó 8,1 % mientras el tok/resultado **subió** 15 %, y la causa no era el cambio sino que los callers
pasaron a pedir budgets más grandes. La métrica invariante a eso es el **ratio payload/budget**.
