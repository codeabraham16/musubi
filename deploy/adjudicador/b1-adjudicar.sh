#!/usr/bin/env bash
#
# b1-adjudicar.sh — el ADJUDICADOR de la cola de conflictos de la memoria de Musubi (B1).
#
# Claude Code sin interfaz sobre la SUSCRIPCIÓN (sin ANTHROPIC_API_KEY ⇒ no factura por token)
# habla con una memoria de Musubi por MCP y resuelve un LOTE de relaciones pendientes con Sonnet 5.
# Filosofía Musubi: el libro mayor no se tacha; ante la duda, se deja pendiente.
#
# ─────────────────────────────────────────────────────────────────────────────────────────────
# POR QUÉ VIVE EN EL REPO
#
# Hasta el 2026-09-25 existía sólo como /home/musubi/b1-adjudicar.sh en el server (sha256
# ef3e477e…, 3362 bytes), fuera de git: sin historia, sin guarda y sin copia en ninguna otra
# máquina. Corría bien —tres veces por día, la cola del cerebro en 0 en cada corrida—, pero la
# memoria LOCAL de cada laptop tiene su PROPIA cola y nadie la juzgaba: la noche del 2026-09-24,
# con muchos merges de varias sesiones, la de davantis llegó a 94 relaciones pendientes, todas
# commit→nota. Y que el veredicto del cerebro baje no alcanza: medido esa noche, sólo 29 de esos
# 94 pares existían también allá, porque cada lado compara contra las notas que tiene.
#
# Así que el mismo guion sirve para las dos puntas, y lo único que cambia es a qué memoria habla:
#
#	server  (default)  B1_MCP_CONFIG=~/b1-workspace/mcp.json   B1_SERVIDOR=cerebro   (HTTP)
#	laptop             B1_MCP_CONFIG=<el que escribe instalar-adjudicador-local.sh>
#	                   B1_SERVIDOR=musubi   (stdio: `musubi daemon` con MUSUBI_HOME fijado)
#
# Uso: b1-adjudicar.sh [lote]     (lote = cuántas relaciones juzga por corrida; default 30)
set -u

MCP_CONFIG="${B1_MCP_CONFIG:-$HOME/b1-workspace/mcp.json}"
SERVIDOR="${B1_SERVIDOR:-cerebro}"
BATCH="${1:-30}"
cd "$(dirname "$MCP_CONFIG")" || exit 1

unset ANTHROPIC_API_KEY
# En el server la sesión de Claude viene de un token guardado; en una laptop con la sesión
# iniciada no hace falta, y exportar uno vacío la rompería.
if [[ -r "$HOME/.claude-b1-token" ]]; then
	CLAUDE_CODE_OAUTH_TOKEN="$(tr -d '\r\n ' <"$HOME/.claude-b1-token")"
	export CLAUDE_CODE_OAUTH_TOKEN
fi
CLAUDE="${B1_CLAUDE:-$(command -v claude || echo "$HOME/.npm-global/bin/claude")}"

# LAS ÚNICAS TRES HERRAMIENTAS QUE PUEDE USAR: leer la cola, leer las observaciones y juzgar.
# Nada que guarde, borre o archive memoria. Lo custodia TestElAdjudicadorSoloPuedeLeerYJuzgar.
HERRAMIENTAS="mcp__${SERVIDOR}__musubi_conflicts,mcp__${SERVIDOR}__musubi_memory_expand,mcp__${SERVIDOR}__musubi_judge"

read -r -d '' PROMPT <<PROMPT_EOF
Sos el ADJUDICADOR NOCTURNO de la memoria de Musubi. Tu trabajo: resolver relaciones de conflicto pendientes entre observaciones, con criterio y con MUCHO cuidado.

PASOS:
1. Llama musubi_conflicts con order="oldest" y limit=${BATCH}: drena la cola en FIFO, empezando por la mas vieja. SIN order="oldest" el orden es ESTABLE y las viejas no se alcanzan nunca (pedir siempre las primeras N devuelve siempre las mismas N); y sin limit te bajas la cola entera, ~77 KB, para usar ${BATCH}.
2. Para cada una, lee el contenido REAL de ambas observaciones con musubi_memory_expand (pasando source_id y target_id). No adivines por el titulo: lee el cuerpo.
3. Decidí el veredicto segun estas reglas (en orden de prioridad):
   - EL LIBRO MAYOR NO SE TACHA: NUNCA uses 'supersedes' si el target es un commit (topic_key = "git-commit") o un artefacto SDD (topic_key empieza con "sdd/"). Esos se citan, no se reemplazan. Ahi el veredicto correcto casi siempre es 'related' o 'not_conflict'.
   - CUIDADO CON LA DIRECCION: 'supersedes' oculta el TARGET. Si el target es la observacion mas nueva/valiosa/correcta y el source es la vieja, NO uses supersedes (ocultarias lo bueno). En ese caso 'related' o 'conflicts_with'.
   - 'supersedes' SOLO si una observacion CLARAMENTE deja obsoleta a la otra (la mas nueva corrige/reemplaza/actualiza EL MISMO hecho, ej: dice "CORRECCION", "AHORA", "RESUELTO", "reemplaza lo anterior"). Es el UNICO veredicto que OCULTA memoria, asi que exigi CERTEZA ALTA.
   - UNA FRASE VIEJA NO ES UNA NOTA VIEJA: si un commit vuelve FALSA una afirmacion concreta de la nota (un numero, un "falta X", un "esto esta roto") pero el resto de la nota sigue valiendo, el veredicto es 'related' y NO 'supersedes'. En la razon copia esa frase, asi: LA NOTA QUEDO VIEJA EN ESTO: «<la frase exacta>». Es lo que le avisa a quien la lea que parte ya no vale.
   - 'not_conflict' / 'compatible' / 'related' / 'scoped' si NO se contradicen (mismo tema, conviven o se complementan).
   - 'conflicts_with' si se contradicen de verdad pero NINGUNA reemplaza claramente a la otra.
   - ANTE LA DUDA: NO resuelvas, dejala pendiente. Es preferible dejar una pendiente que ocultar algo por error.
4. Aplica cada veredicto con musubi_judge (relation_id, relation, y una razon corta y clara).

Al final devolve un RESUMEN en texto plano, una linea por conflicto procesado:
[relation_id-corto] VEREDICTO — razon en 1 linea (y el topic_key de cada lado)
Y al final: cuantas resolviste por tipo y cuantas dejaste pendientes y por que.
PROMPT_EOF

echo "=== B1 adjudicador — lote de ${BATCH} contra «${SERVIDOR}» (Sonnet 5, suscripción) $(date -Is) ==="
timeout 1500 "$CLAUDE" -p "$PROMPT" \
	--model claude-sonnet-5 \
	--mcp-config "$MCP_CONFIG" \
	--allowedTools "$HERRAMIENTAS" \
	--permission-mode dontAsk \
	--output-format json </dev/null 2>&1 | python3 -c "import sys,json
raw=sys.stdin.read()
try: d=json.loads(raw)
except Exception: print('--- SALIDA NO-JSON DE CLAUDE ---'); print(raw[-2000:]); sys.exit(1)
print('--- RESULTADO ---')
print(d.get('result','(sin result)'))
print('--- costo equiv:', d.get('total_cost_usd'), '| error:', d.get('is_error'), '| turns:', d.get('num_turns'))
sys.exit(1 if d.get('is_error') else 0)"
