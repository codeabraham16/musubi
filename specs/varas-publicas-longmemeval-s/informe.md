# Varas públicas: LongMemEval (recuperación), ARB y ForgetEval-Adv

> Unidad `aa1cab60` del lote `965b399b`, rama `medir/varas-publicas` (sale de `origin/main`
> `704c5013`). Medido el 2026-09-30 en `davantis` (Linux, 7,6 GB de RAM compartidos con otras
> sesiones). Los datos y las salidas por pregunta quedaron **fuera del repo**: son sesiones de chat
> de un dataset ajeno.

## Lo que hay que llevarse

1. **El BM25 que publica el paper es de LongMemEval-M, no de S.** Es la Tabla 9: 0,634 · 0,516 ·
   0,710 · 0,540 (recall_all@5 · ndcg_any@5 · recall_all@10 · ndcg_any@10). Una réplica propia de
   BM25 queda a menos de 0,02 en las cuatro columnas sobre M y lejos sobre S (§2.3). Así que contra
   ese número se compara en M; en S, contra la réplica sobre el mismo corpus.
2. **En S, la config híbrida le gana a BM25 en las cuatro columnas**, con p < 0,01 en todas:
   0,8520 · 0,8299 · 0,9475 · 0,8515 contra 0,7351 · 0,7671 · 0,8234 · 0,7922 (promedio estricto,
   n = 419; §2.2).
3. **En S, `musubi_recall` tal como está configurado pierde contra BM25 en las cuatro, y la causa
   es MMR (λ 0,75), no el corrector de tipeo.** `produccion` da 0,3914 de recall_all@5. La caída
   está entera en las preguntas con dos o más sesiones de oro (0,8445 → 0,1767), y la primera
   sesión de oro sigue arriba. La ablación del 2026-10-01 lo separa (§2.2.1, una de cada tres
   preguntas, n = 140): apagar MMR lleva recall_all@10 de 0,4786 a 0,9500 (66 preguntas ganan y
   ninguna pierde), y apagar el corrector no mueve ninguna de las cuatro columnas. Subir λ
   recupera de forma monótona: 0,95 ya iguala el recall_all@10 de MMR apagado, pero no su
   recall_all@5 (0,7357 contra 0,8571). **No se tocó el default:** el otro lado de la balanza, la
   redundancia que MMR evita, no lo mide esta vara. El hook por turno no corre MMR.
4. **El léxico pierde contra BM25, y más cuanto más grande es el pajar.** En M da 0,3957 · 0,3230 ·
   0,6383 · 0,3959 contra el número publicado (§2.4). **Toda esa distancia es la expansión por
   co-ocurrencia (PRF):** con la PRF apagada, el léxico le gana al BM25 publicado en las cuatro
   columnas (0,7404 · 0,6597 · 0,8085 · 0,6797), y en S queda a la par de la híbrida (§2.5).
   **Eso NO autoriza a apagarla:** el banco propio mide lo contrario (PR #454), y la hipótesis es
   que la PRF se descarrila en docs de miles de palabras. Lo que sí autoriza es medir la PRF contra
   el largo de los docs.
5. **Costos (§2.6):**
   - Sin embebedor, ninguna corrida pasó de 112 MiB de RSS, M (2,7 GB) incluido, porque el lector va
     de a una pregunta.
   - La híbrida tardó 73 minutos en S, con un núcleo entero en promedio, y llegó a 906 MiB de RSS +
     swap en lo que se muestreó. Por eso no se corrió en M.
   - Hay una hipótesis de código sobre el tokenizador de POTION que conviene medir antes de
     intentarlo (§2.7).
6. **PASO 3, en este orden (§3):**
   1. ForgetEval-Adv, de 3 a 5 días. Antes hay que decidir dos cosas: qué es `release` en Musubi, y
      cómo se crea un `supersedes` sin un conflicto detectado.
   2. ARB `edit2ripple`, sólo en Go, de 2 a 3 días. Son 29 de sus 58 casos, y se reporta por
      lenguaje.

---

## 1. Las tres varas (PASO 1)

| Vara | Paper | Datos | Licencia | Tamaño y formato |
|---|---|---|---|---|
| **LongMemEval** (Wu et al., ICLR 2025) | arXiv 2410.10813 (v1 14-oct-2024, v2), CC BY 4.0 | HF `xiaowu0162/longmemeval` (el original, hoy marcado como deprecado) · código en github.com/xiaowu0162/LongMemEval | código y datos MIT | `longmemeval_s` 278.025.796 B · `longmemeval_m` 2.745.274.681 B · `oracle` 15.388.478 B. Un arreglo JSON de 500 preguntas; cada una trae su propio pajar de sesiones con turnos `{role, content, has_answer}` |
| LongMemEval, versión limpia | — | HF `xiaowu0162/longmemeval-cleaned` (2025-09) | MIT | `s_cleaned` 277.383.467 B · `m_cleaned` 2.737.100.077 B. «removes noisy history sessions that interfere with the answer correctness». **No se midió** (ver §2.7) |
| **Agent Retrieval Bench** (Qin y Xie) | arXiv 2607.24882 (27-jul-2026), CC BY 4.0 | HF `eyuansu71/agent_retrieval_bench` · código en github.com/eyuansu62/agent-retrieval-bench | código y metadata MIT; **cada archivo del corpus conserva la licencia de su repo de origen** (DATA_LICENSE.md) | 5 releases `tar.zst` que suman 1.061.786.103 B y 10,5 M de chunks; JSONL + `corpus_manifest.jsonl`. `edit2ripple` sola pesa 128,7 MB |
| **ForgetEval-Adv** (Dongxu Yang) | arXiv 2606.15903 v2 (16-jun-2026), licencia arXiv no exclusiva | github.com/deeplethe/lethe, `bench/forgeteval/` | MIT | **no es un JSON de datos**: código Python con instancias de la dataclass `GeneratedCase` (`adversarial.py` 89 KB, `adversarial_generated.py` 127 KB) más `adversarial_generated_labels.json` (10 KB) |

Descargado y verificado para esta unidad: `longmemeval_s` (sha256 `08d8dad4…894`) y
`longmemeval_m` (sha256 `fb5413e3b077c62927daab794836991a2fcfa61ceacab57dc679fb02daaff2d9`, el
tamaño exacto que publica Hugging Face; la descarga se cortó dos veces y se retomó con `curl -C -`).
De ARB y ForgetEval-Adv se leyeron README, licencias, manifiesto y documentación; no se bajó el
corpus.

---

## 2. LongMemEval, sólo recuperación (PASO 2)

### 2.1 Qué se midió y cómo

**Adaptador propio en Go**, en `internal/recalleval/publico/`. No corre código de terceros: el
protocolo se portó leyendo `run_retrieval.py`, `eval_utils.py` y `print_retrieval_metrics.py` del
repo del paper, y cada regla lleva su prueba.

- **Corpus por pregunta:** una entrada por sesión del pajar. El texto son los turnos del **usuario**
  unidos con un espacio, que es lo único que implementa el paper a nivel sesión
  (`process_item_flat_index`). El modo `full` (usuario y asistente) es una extensión nuestra.
- **Oro:** la sesión cuyo id contiene `answer` **y** tiene algún turno de usuario con
  `has_answer`. Si ninguno lo tiene, el paper renombra `answer`→`noans` y la sesión deja de ser oro.
- **Ids opacos** (`lme-` + sha256 truncado del id y la posición): el id de sesión trae la palabra
  `answer`, y pasárselo a Musubi sería una fuga de la etiqueta.
- **Una base temporal por pregunta:** `os.MkdirTemp` + la plantilla ya migrada de las pruebas +
  `RemoveAll` al salir. La función no recibe directorio, así que no hay camino que termine en
  `.musubi/memory.db`.
- **Brazos:**
  - `bm25-paper`: réplica de `BM25Okapi` de `rank_bm25` (k1 1,5 · b 0,75 · ε 0,25 con el piso de
    idf negativo), tokenización `split(" ")` y la pregunta a secas como consulta, como el paper.
  - `lexical` = `ConfigLexica` · `turno` = `ConfigTurno` (el ranker del hook: pool de 50, sin
    embebedor, con corrector de tipeo).
  - `hybrid` = `ConfigHibrida` · `produccion` = `ConfigProduccion` (MMR y corrector), las dos con
    POTION multilingual 128M **local**, sin red.
- **Métricas del paper:** `recall_all@k` (todas las sesiones de oro en el top-k; es la columna
  «Recall» de sus tablas), `recall_any@k` (alguna) y `ndcg_any@k`. Además, las de recalleval.
- **Dos promedios, porque el paper tiene dos:**
  - **estricto** (`run_retrieval.py`, n = 419): fuera las 30 abstenciones y las 51 preguntas sin
    evidencia del lado del usuario;
  - **sin abstenciones** (`print_retrieval_metrics.py`, n = 470): es el script que imprime
    exactamente las cuatro columnas de la tabla. En las 51 preguntas sin oro, `recall_all` vale 1
    para cualquier recuperador (`all([])`) y el nDCG vale 0.
- **Lectura en streaming** (`json.Decoder`, una pregunta por vez).
- **El adaptador está verificado:** 17 pruebas unitarias, y 10 sabotajes que dan ROJO por su motivo
  en el arnés (`deploy/cmd/arnes`). Cubren el lector que no hace streaming, la regla del oro, las
  abstenciones, la fuga del id, un `recall_all` que se conforma con una sola sesión de oro, el
  descuento del nDCG, el piso del idf, los docs que la base no acepta, la base que queda en el disco
  y un brazo de la ablación que no sale del léxico.

### 2.2 Resultados en LongMemEval-S

**Promedio estricto (n = 419).** Las cuatro primeras columnas son las de la Tabla 9 del paper.

<!-- hibrido-user.json · dataset sin registrar · modo user · 4356 s -->
| brazo | n | recall_all@5 | ndcg_any@5 | recall_all@10 | ndcg_any@10 | recall_any@5 | recall_any@10 | rv_mrr | devueltos |
|---|---|---|---|---|---|---|---|---|---|
| bm25-paper | 419 | 0,7351 | 0,7671 | 0,8234 | 0,7922 | 0,8831 | 0,9308 | 0,7876 | 50,0 |
| lexical | 419 | 0,6468 | 0,6409 | 0,8687 | 0,6930 | 0,8902 | 0,9570 | 0,6452 | 46,7 |
| turno | 419 | 0,6372 | 0,6315 | 0,8687 | 0,6872 | 0,8807 | 0,9547 | 0,6396 | 44,2 |
| hybrid | 419 | 0,8520 | 0,8299 | 0,9475 | 0,8515 | 0,9499 | 0,9857 | 0,8318 | 46,7 |
| produccion | 419 | 0,3914 | 0,5992 | 0,5203 | 0,6337 | 0,9165 | 0,9547 | 0,8194 | 46,7 |

**Promedio sin abstenciones (n = 470),** el del script que imprime la tabla del paper:

<!-- hibrido-user.json · dataset sin registrar · modo user · 4356 s -->
| brazo | n | recall_all@5 | ndcg_any@5 | recall_all@10 | ndcg_any@10 | recall_any@5 | recall_any@10 | rv_mrr | devueltos |
|---|---|---|---|---|---|---|---|---|---|
| bm25-paper | 470 | 0,7638 | 0,6839 | 0,8426 | 0,7062 | 0,7872 | 0,8298 | 0,7021 | 50,2 |
| lexical | 470 | 0,6851 | 0,5713 | 0,8830 | 0,6178 | 0,7936 | 0,8532 | 0,5752 | 46,9 |
| turno | 470 | 0,6766 | 0,5629 | 0,8830 | 0,6127 | 0,7851 | 0,8511 | 0,5702 | 44,4 |
| hybrid | 470 | 0,8681 | 0,7399 | 0,9532 | 0,7591 | 0,8468 | 0,8787 | 0,7416 | 46,9 |
| produccion | 470 | 0,4574 | 0,5342 | 0,5723 | 0,5649 | 0,8170 | 0,8511 | 0,7304 | 46,9 |

1. **La híbrida le gana a la réplica de BM25 en las cuatro columnas y con los dos promedios:**
   0,8520 · 0,8299 · 0,9475 · 0,8515 contra 0,7351 · 0,7671 · 0,8234 · 0,7922 (estricto). Lleva la
   co-ocurrencia encendida, igual que el léxico (§2.5): el vector compensa lo que la PRF desordena.
2. **El léxico pierde contra BM25 en el tope y gana más abajo.** En recall_all@5 queda en 0,6468
   contra 0,7351 y pierde en los dos nDCG, pero en recall_all@10 gana (0,8687 contra 0,8234): trae
   las sesiones de oro, sólo que no arriba. §2.5 muestra que eso es la PRF. El hook (`turno`) queda
   pegado al léxico.
3. **`produccion` es la peor en las cuatro columnas del paper**, con 0,3914 de recall_all@5, menos
   de la mitad que la híbrida. Es la config de `musubi_recall`: la híbrida más MMR (λ 0,75) y el
   corrector de tipeo. El hook por turno no corre MMR (`ConfigTurno`), así que esto pega en
   `musubi_recall` y no en el recall de cada turno. Devuelve la misma cantidad de docs que la
   híbrida (46,7 por pregunta), así que lo que cambia es el orden. Y en recall_any@5 le gana a BM25
   (0,9165 contra 0,8831): encuentra la primera sesión de oro, pero no el resto. Partido por cuántas
   sesiones de oro tiene la pregunta:

   <!-- hibrido-user.jsonl · estricto · n=419 -->
   | sesiones de oro | n | brazo | recall_all@5 | recall_any@5 | recall_all@10 | ndcg_any@10 | rv_mrr |
   |---|---|---|---|---|---|---|---|
   | una | 136 | bm25-paper | 0,7721 | 0,7721 | 0,8456 | 0,7518 | 0,6533 |
   | una | 136 | hybrid | 0,8676 | 0,8676 | 0,9559 | 0,8118 | 0,6825 |
   | una | 136 | produccion | 0,8382 | 0,8382 | 0,9044 | 0,7868 | 0,6722 |
   | dos o más | 283 | bm25-paper | 0,7173 | 0,9364 | 0,8127 | 0,8116 | 0,8521 |
   | dos o más | 283 | hybrid | 0,8445 | 0,9894 | 0,9435 | 0,8706 | 0,9036 |
   | dos o más | 283 | produccion | 0,1767 | 0,9541 | 0,3357 | 0,5601 | 0,8900 |

   Con una sola sesión de oro, `produccion` queda a 0,03 de la híbrida. Con dos o más, recall_any@5
   y el MRR casi no se mueven, pero recall_all@5 cae de 0,8445 a 0,1767: la primera sesión de oro
   sigue arriba y las otras salen del top-5. **Es el patrón de MMR**, que baja lo que se parece a lo
   ya elegido, y las sesiones de oro de una misma pregunta tratan el mismo tema: así se arma una
   pregunta de varias sesiones. Cuánto se parecen entre sí no se midió. El corrector de tipeo no
   tiene por qué distinguir entre una sesión de oro y varias, y en el léxico mueve poco (`turno`
   contra `lexical`, que además difieren en el pool). La ablación que los separa se corrió después
   y confirma el patrón: §2.2.1.

   Tampoco es el sesgo que documenta `RedundanciaAtK` en `recalleval/metrics.go`. En el banco propio
   la relevancia se etiqueta por `topic_key`, y MMR sale castigado por construcción. Acá el oro lo
   puso el dataset. Lo que sí se ve es el costo real de diversificar cuando la evidencia
   complementaria se parece entre sí. Cuánto de eso es aceptable queda como decisión de producto,
   la misma que ese comentario deja abierta, y acá hay número para un solo lado: la redundancia del
   tope no se midió.

**Pregunta por pregunta** (estricto, n = 419; prueba de signos exacta de dos colas sobre los pares
que difieren):

<!-- hibrido-user.jsonl · lexical contra bm25-paper · estricto · n=419 -->
| métrica | gana lexical | gana bm25-paper | iguales | p |
|---|---|---|---|---|
| recall_all@5 | 43 | 80 | 296 | 0,0011 |
| recall_all@10 | 37 | 18 | 364 | 0,014 |
| ndcg_any@5 | 89 | 211 | 119 | 1,4e-12 |
| ndcg_any@10 | 100 | 213 | 106 | 1,6e-10 |

<!-- hibrido-user.jsonl · hybrid contra bm25-paper · estricto · n=419 -->
| métrica | gana hybrid | gana bm25-paper | iguales | p |
|---|---|---|---|---|
| recall_all@5 | 63 | 14 | 342 | 1,4e-08 |
| recall_all@10 | 57 | 5 | 357 | 3,1e-12 |
| ndcg_any@5 | 124 | 85 | 210 | 0,0084 |
| ndcg_any@10 | 136 | 90 | 193 | 0,0027 |

<!-- hibrido-user.jsonl · hybrid contra lexical · estricto · n=419 -->
| métrica | gana hybrid | gana lexical | iguales | p |
|---|---|---|---|---|
| recall_all@5 | 90 | 4 | 325 | 3,2e-22 |
| recall_all@10 | 36 | 3 | 380 | 3,6e-08 |
| ndcg_any@5 | 245 | 17 | 157 | 6,2e-53 |
| ndcg_any@10 | 261 | 19 | 139 | 1,5e-55 |

<!-- hibrido-user.jsonl · produccion contra hybrid · estricto · n=419 -->
| métrica | gana produccion | gana hybrid | iguales | p |
|---|---|---|---|---|
| recall_all@5 | 6 | 199 | 214 | 3,8e-51 |
| recall_all@10 | 3 | 182 | 234 | 4,3e-50 |
| ndcg_any@5 | 24 | 259 | 136 | 5,8e-51 |
| ndcg_any@10 | 27 | 271 | 121 | 7,6e-52 |

**Por tipo de pregunta** (estricto; cada celda es recall_all@5 · recall_all@10 · ndcg_any@10):

<!-- hibrido-user.json · dataset sin registrar · modo user · 4356 s -->
| tipo | n | bm25-paper | lexical | hybrid | produccion |
|---|---|---|---|---|---|
| knowledge-update | 72 | 0,958 · 0,972 · 0,928 | 0,792 · 1,000 · 0,759 | 0,972 · 1,000 · 0,922 | 0,208 · 0,417 · 0,594 |
| multi-session | 121 | 0,554 · 0,702 · 0,731 | 0,529 · 0,777 · 0,695 | 0,769 · 0,901 · 0,846 | 0,157 · 0,314 · 0,534 |
| single-session-assistant | 5 | 0,800 · 0,800 · 0,800 | 1,000 · 1,000 · 0,572 | 1,000 · 1,000 · 0,826 | 1,000 · 1,000 · 0,852 |
| single-session-preference | 30 | 0,667 · 0,733 · 0,567 | 0,433 · 0,700 · 0,500 | 0,700 · 0,967 · 0,629 | 0,733 · 0,900 · 0,651 |
| single-session-user | 64 | 0,922 · 0,938 · 0,905 | 0,875 · 0,984 · 0,786 | 0,984 · 1,000 · 0,961 | 0,922 · 0,953 · 0,918 |
| temporal-reasoning | 127 | 0,701 · 0,819 · 0,770 | 0,598 · 0,858 · 0,657 | 0,827 · 0,929 · 0,814 | 0,346 · 0,449 · 0,595 |

Por tipo se ve lo mismo:
- La híbrida queda arriba de la réplica en todas las celdas menos una: ndcg_any@10 de
  knowledge-update, 0,922 contra 0,928.
- La caída de `produccion` está entera en los tres tipos donde casi todas las preguntas tienen dos o
  más sesiones de oro: knowledge-update (70 de 72), multi-session (121 de 121) y temporal-reasoning
  (92 de 127).
- En los tres tipos de una sola sesión, `produccion` queda pegada a la híbrida, y en
  single-session-preference hasta la supera en recall_all@5 (0,733 contra 0,700).

#### 2.2.1 Ablación: es MMR, no el corrector (2026-10-01)

Corrida `abl-mmr-s-cada3`, con el build del commit `069eb413`. Usa `MUSUBI_LONGMEMEVAL_ABLACION_MMR=1`
y `MUSUBI_LONGMEMEVAL_CADA=3`, así que **mide una de cada tres preguntas del archivo**: la 1.ª, la 4.ª,
la 7.ª… Es una muestra sistemática fijada de antemano. Las otras 333 no cuentan en nada, y el informe
lo dice en la línea `MUESTRA`. Quedan **140 preguntas en el promedio estricto**, de 419. Cada brazo es
`ConfigProduccion()` con **una sola** cosa cambiada; una prueba lo exige y su sabotaje da ROJO en el
arnés. La corrida tardó 20:13 y llegó a 1,1 GB de RSS máx. El corrector venció su plazo 0 veces.

<!-- abl-mmr-s-cada3.jsonl · longmemeval_s.json · modo user · cada=3 · estricto · n=140 -->
| brazo | λ MMR | corrector | n | recall_all@5 | ndcg_any@5 | recall_all@10 | ndcg_any@10 | recall_any@5 | rv_mrr |
|---|---|---|---|---|---|---|---|---|---|
| produccion | 0,75 | sí | 140 | 0,3429 | 0,5738 | 0,4786 | 0,6101 | 0,9000 | 0,8058 |
| prod-mmr-0.85 | 0,85 | sí | 140 | 0,4643 | 0,6066 | 0,6429 | 0,6561 | 0,9000 | 0,8056 |
| prod-mmr-0.90 | 0,90 | sí | 140 | 0,5429 | 0,6358 | 0,8429 | 0,7101 | 0,8929 | 0,8094 |
| prod-mmr-0.95 | 0,95 | sí | 140 | 0,7357 | 0,7253 | 0,9500 | 0,7795 | 0,9214 | 0,8159 |
| prod-sin-mmr | apagado | sí | 140 | 0,8571 | 0,8199 | 0,9500 | 0,8425 | 0,9500 | 0,8213 |
| prod-sin-tipeo | 0,75 | no | 140 | 0,3429 | 0,5738 | 0,4786 | 0,6101 | 0,9000 | 0,8058 |
| hybrid | apagado | no | 140 | 0,8643 | 0,8214 | 0,9500 | 0,8431 | 0,9500 | 0,8213 |
| bm25-paper | — | — | 140 | 0,7571 | 0,7743 | 0,8000 | 0,7922 | 0,8929 | 0,7841 |

La muestra se parece a la corrida entera de §2.2: `produccion` 0,3429 contra 0,3914 y `hybrid`
0,8643 contra 0,8520 en recall_all@5.

**Contra `produccion`, pregunta por pregunta** (prueba de signos exacta de dos colas):

<!-- abl-mmr-s-cada3.jsonl · contra produccion · estricto · n=140 -->
| brazo | recall_all@5 gana/pierde · p | recall_all@10 gana/pierde · p | ndcg_any@10 gana/pierde · p |
|---|---|---|---|
| prod-mmr-0.85 | 19/2 · 2,2e-04 | 24/1 · 1,5e-06 | 54/6 · 9,7e-11 |
| prod-mmr-0.90 | 32/4 · 1,9e-06 | 52/1 · 1,2e-14 | 81/6 · 7,0e-18 |
| prod-mmr-0.95 | 57/2 · 6,1e-15 | 66/0 · 2,7e-20 | 96/8 · 2,8e-20 |
| prod-sin-mmr | 74/2 · 7,7e-20 | 66/0 · 2,7e-20 | 96/10 · 8,7e-19 |
| prod-sin-tipeo | 0/0 · — | 0/0 · — | 0/0 · — |

**Con dos o más sesiones de oro** (n = 95), que es donde estaba la caída:

<!-- abl-mmr-s-cada3.jsonl · oro ≥ 2 · estricto · n=95 -->
| brazo | recall_all@5 | recall_all@10 | ndcg_any@10 | recall_any@5 | rv_mrr |
|---|---|---|---|---|---|
| produccion | 0,1263 | 0,2947 | 0,5511 | 0,9474 | 0,8847 |
| prod-mmr-0.85 | 0,3158 | 0,5158 | 0,6214 | 0,9579 | 0,8878 |
| prod-mmr-0.90 | 0,4526 | 0,7895 | 0,6915 | 0,9684 | 0,8902 |
| prod-mmr-0.95 | 0,7053 | 0,9263 | 0,7762 | 0,9789 | 0,8930 |
| prod-sin-mmr | 0,8316 | 0,9263 | 0,8517 | 0,9684 | 0,8899 |
| hybrid | 0,8421 | 0,9263 | 0,8526 | 0,9684 | 0,8899 |

Lo que dice:
1. **El corrector de tipeo no es.** `prod-sin-tipeo` es idéntico a `produccion` en las cuatro
   columnas del paper en las 140 preguntas. Cambia algo en 2, y sólo en el nDCG a 30 y a 50. El
   corrector sí corre: `hybrid` y `prod-sin-mmr` difieren sólo en él, y se separan en 1 pregunta
   (`gpt4_a56e767c`, multi-session). Que no haga nada es lo esperable, porque las preguntas de
   LongMemEval no traen tipeos.
2. **Es MMR, y en proporción a λ.** La recuperación es monótona en las cuatro columnas: 0,75 →
   0,85 → 0,90 → 0,95 → apagado. Con una sola sesión de oro (n = 45) pesa mucho menos, pero
   pesa: recall_all@10 da 0,8667 con MMR y 1,0000 sin él (6 preguntas ganan y ninguna pierde).
3. **Ningún λ lo arregla gratis.** λ 0,95 iguala el recall_all@10 de MMR apagado (0,9500), pero en
   recall_all@5 queda 0,12 abajo y en ndcg_any@10, 0,06 abajo. Por tipo, la distancia está en
   temporal-reasoning (recall_all@5 0,667 contra 0,881) y en knowledge-update (0,792 contra 0,917).

**Por qué esto no basta para cambiar el default, y qué lo haría.** Esta vara mide un solo lado: el
costo de diversificar cuando la evidencia complementaria se parece entre sí. El oro lo puso el
dataset, así que acá no corre el sesgo por construcción del banco propio. En el pajar de
LongMemEval casi no hay redundancia que evitar: son sesiones de relleno distintas entre sí. MMR no
tiene nada que ganar ahí, y lo que se ve es su precio entero. En la memoria real sí hay redundancia;
el caso que lo originó fueron las 7 fases SDD de un mismo cambio contadas una por una. El barrido
del banco propio (2026-09-11, corpus real) midió los dos ejes:

| λ | redundancia@10 | R@10 |
|---|---|---|
| apagado | 0,7453 | 0,4063 |
| 0,90 | 0,7202 (−3 %) | 0,3727 |
| 0,75 | 0,6313 (−15 %) | 0,2679 |

Ahí la relevancia sale del `topic_key`, así que la caída de R@10 es una cota de arriba. Las dos
varas juntas dicen esto:
- λ 0,75 compra 15 % menos de redundancia y paga, con oro exógeno, 47 puntos de recall_all@10 en
  LongMemEval-S;
- λ 0,90 a 0,95 paga mucho menos y compra casi nada de redundancia (−3 % a 0,90; a 0,95 no se
  midió).

**La decisión es de producto, y le toca al usuario.** Las opciones razonables son tres:
- apagar MMR en `musubi_recall`;
- subir λ a 0,95;
- dejarlo y medir la redundancia a 0,95 sobre el corpus real antes de elegir.

### 2.3 La Tabla 9 del paper es M, no S

El pedido era comparar contra «el BM25 del paper». Su único número de BM25 a nivel sesión está en
la Tabla 9 (apéndice E.2, K = V): **0,634 · 0,516 · 0,710 · 0,540**. El paper no dice sobre qué
variante está. Se resolvió midiendo, con la réplica, y cruzando tablas:

| Réplica de BM25 | promedio | recall_all@5 | ndcg_any@5 | recall_all@10 | ndcg_any@10 |
|---|---|---|---|---|---|
| S, usuario | sin abstenciones | 0,7638 | 0,6839 | 0,8426 | 0,7062 |
| S, usuario | estricto | 0,7351 | 0,7671 | 0,8234 | 0,7922 |
| S, completo | sin abstenciones | 0,7638 | 0,6948 | 0,8511 | 0,7168 |
| **M, usuario** | **sin abstenciones** | **0,6149** | **0,5344** | **0,6936** | **0,5583** |
| M, usuario | estricto | 0,5680 | 0,5995 | 0,6563 | 0,6263 |
| M, completo | sin abstenciones | 0,6085 | 0,5158 | 0,6723 | 0,5400 |
| **Tabla 9 publicada** | — | **0,634** | **0,516** | **0,710** | **0,540** |

1. En S la réplica queda lejos en las cuatro columnas y con los dos promedios (+0,13 de recall y
   +0,17 de nDCG). En M, con el protocolo del paper (usuario, sin abstenciones), queda a menos de
   0,02 en las cuatro.
2. **La Tabla 9 repite filas de la Tabla 3, que el paper declara sobre M** y corre con Stella V5,
   el recuperador del texto principal. Stella V5, sesión, K = V + summary: 0,689 · 0,608 · 0,749 ·
   0,624 en las dos. Stella V5, ronda, K = V + fact: 0,644 · 0,498 · 0,784 · 0,536 en las dos.
3. **El paper no es consistente consigo mismo, y eso explica el resto de la distancia.** Para la
   misma fila (Stella, sesión, K = V) la Tabla 3 dice 0,706 · 0,617 · 0,783 · 0,638 y la Tabla 9
   0,720 · 0,594 · 0,794 · 0,615. En K = V + fact el recall coincide y el nDCG difiere en
   exactamente 0,100 (0,620 contra 0,520, y 0,652 contra 0,552). Una réplica a ±0,02 de una tabla
   que difiere ±0,02 de sí misma está dentro del ruido del paper.

**Consecuencia:** comparar a Musubi en S contra 0,634/0,516/0,710/0,540 sería comparar dos pajares
distintos (50 sesiones contra 500). La comparación justa en S es contra la réplica sobre **el mismo
corpus**, que es la de §2.2. Contra el número publicado se compara en M (§2.4).

> Nota vieja que esto corrige: al leer el paper (PASO 1) se había anotado «es probable que sea S»,
> por la fila de Stella K = V ronda. Medido, es M.

### 2.4 LongMemEval-M: el mismo pajar que la Tabla 9

M trae ~500 sesiones por pregunta, diez veces S, y es el pajar de la Tabla 9 (§2.3). Se corrieron
los brazos léxicos, con la ablación de §2.5 en la misma corrida; los híbridos no (§2.7).

**Promedio sin abstenciones (n = 470),** el de la Tabla 9, con la fila publicada al pie:

<!-- ablacion4-m.json · longmemeval_m.json · modo user · 840 s -->
| brazo | n | recall_all@5 | ndcg_any@5 | recall_all@10 | ndcg_any@10 |
|---|---|---|---|---|---|
| bm25-paper | 470 | 0,6149 | 0,5344 | 0,6936 | 0,5583 |
| lexical | 470 | 0,3957 | 0,3230 | 0,6383 | 0,3959 |
| turno | 470 | 0,3553 | 0,2976 | 0,6447 | 0,3839 |
| abl-sin-cooc | 470 | 0,7404 | 0,6597 | 0,8085 | 0,6797 |
| **Tabla 9 publicada (BM25)** | — | 0,634 | 0,516 | 0,710 | 0,540 |

**Promedio estricto (n = 419):**

<!-- ablacion4-m.json · longmemeval_m.json · modo user · 840 s -->
| brazo | n | recall_all@5 | ndcg_any@5 | recall_all@10 | ndcg_any@10 | recall_any@5 | recall_any@10 | rv_mrr | devueltos |
|---|---|---|---|---|---|---|---|---|---|
| bm25-paper | 419 | 0,5680 | 0,5995 | 0,6563 | 0,6263 | 0,7446 | 0,8043 | 0,6323 | 502,0 |
| lexical | 419 | 0,3222 | 0,3623 | 0,5943 | 0,4441 | 0,6348 | 0,8186 | 0,4064 | 465,9 |
| turno | 419 | 0,2768 | 0,3338 | 0,6014 | 0,4306 | 0,5895 | 0,8138 | 0,3934 | 91,8 |

1. **Contra el número publicado, el léxico de Musubi tal como está configurado pierde en las cuatro
   columnas:** 0,3957 · 0,3230 · 0,6383 · 0,3959 contra 0,634 · 0,516 · 0,710 · 0,540. La réplica
   de BM25 sobre el mismo archivo queda a menos de 0,02 del publicado (§2.3), así que la distancia
   es del ranker y no del adaptador.
2. **La distancia crece con el pajar.** En S, sin abstenciones, el léxico perdía por 0,08 de
   recall_all@5 contra la réplica; en M pierde por más de 0,2. El hook (`turno`) queda con el léxico.
3. **Con la co-ocurrencia apagada (`abl-sin-cooc`, §2.5), el mismo léxico le gana al BM25 publicado
   en las cuatro columnas:** 0,7404 · 0,6597 · 0,8085 · 0,6797.

Pregunta por pregunta, `lexical` contra la réplica (estricto, n = 419):

<!-- ablacion4-m.jsonl · lexical contra bm25-paper · estricto · n=419 -->
| métrica | gana lexical | gana bm25-paper | iguales | p |
|---|---|---|---|---|
| recall_all@5 | 29 | 132 | 258 | 6,7e-17 |
| recall_all@10 | 45 | 71 | 303 | 0,02 |
| ndcg_any@5 | 81 | 237 | 101 | 6,8e-19 |
| ndcg_any@10 | 98 | 246 | 75 | 8,1e-16 |

### 2.5 De dónde sale la distancia con BM25: ablación de la fusión (exploratoria)

El léxico de Musubi no es un BM25 solo: es una fusión RRF de siete señales, todas con peso 1
(`memory.PesosUniformes`; producción no fija otros pesos). Los brazos de la ablación son
`ConfigLexica` con algunas señales en peso 0 y **nada más distinto** (`brazosDeAblacion`, fijado
por `TestBrazosDeAblacion`). Son exploratorios, no configs de producción.

**LongMemEval-S, promedio estricto (n = 419):**

<!-- ablacion4-s.json · longmemeval_s.json · modo user · 173 s -->
| brazo | n | recall_all@5 | ndcg_any@5 | recall_all@10 | ndcg_any@10 | recall_any@5 | recall_any@10 | rv_mrr | devueltos |
|---|---|---|---|---|---|---|---|---|---|
| bm25-paper | 419 | 0,7351 | 0,7671 | 0,8234 | 0,7922 | 0,8831 | 0,9308 | 0,7876 | 50,0 |
| lexical | 419 | 0,6468 | 0,6409 | 0,8687 | 0,6930 | 0,8902 | 0,9570 | 0,6452 | 46,7 |
| abl-sin-planas | 419 | 0,6468 | 0,6410 | 0,8687 | 0,6931 | 0,8902 | 0,9570 | 0,6452 | 46,7 |
| abl-sin-grafo | 419 | 0,6468 | 0,6409 | 0,8687 | 0,6930 | 0,8902 | 0,9570 | 0,6452 | 46,7 |
| abl-sin-cooc | 419 | 0,8473 | 0,8734 | 0,9093 | 0,8886 | 0,9451 | 0,9666 | 0,8931 | 46,7 |
| abl-solo-lexico | 419 | 0,8473 | 0,8734 | 0,9093 | 0,8886 | 0,9451 | 0,9666 | 0,8931 | 46,7 |
| turno | 419 | 0,6372 | 0,6315 | 0,8687 | 0,6872 | 0,8807 | 0,9547 | 0,6396 | 44,2 |

1. **La distancia con BM25 es entera de la expansión por co-ocurrencia (PRF).** Apagar sólo esa
   señal (`abl-sin-cooc`) da exactamente lo mismo que dejar el léxico solo (`abl-solo-lexico`): las
   470 preguntas, métrica por métrica. Con eso Musubi pasa de perder a **ganarle a la réplica de BM25
   en las cuatro columnas**: 0,8473 · 0,8734 · 0,9093 · 0,8886 contra 0,7351 · 0,7671 · 0,8234 ·
   0,7922. Y queda a la par de la híbrida de §2.2, que lleva la PRF encendida (0,8520 · 0,8299 ·
   0,9475 · 0,8515): mejor en los dos nDCG, peor en recall_all@10. La híbrida sin PRF no se midió.
2. **El grafo no mueve nada acá, y eso no es un veredicto sobre el grafo.** `abl-sin-grafo` es
   idéntico al léxico en las 470. La centralidad se calcula sobre `observation_relations`, y la
   siembra del banco (`SeedEngine` → `SaveObservation`) no pasa por la detección de conflictos ni por
   el dedup, que son los que crean relaciones: con el grafo vacío, la señal no existe.
3. **Las tres señales planas son neutras.** `abl-sin-planas` difiere del léxico en 1 de 470
   preguntas, por redondeo. Con `created_at` constante, `NoBump` e importancia pareja suman la misma
   constante a todos los candidatos, y una constante no cambia el orden.
4. **El hook paga lo mismo.** `turno` (el ranker del hook por turno) lleva la co-ocurrencia
   encendida y queda con el léxico: 0,6372 de recall_all@5.

Pregunta por pregunta (estricto, n = 419; prueba de signos exacta de dos colas):

<!-- ablacion4-s.jsonl · abl-sin-cooc contra bm25-paper · estricto · n=419 -->
| métrica | gana abl-sin-cooc | gana bm25-paper | iguales | p |
|---|---|---|---|---|
| recall_all@5 | 55 | 8 | 356 | 9,8e-10 |
| recall_all@10 | 45 | 9 | 365 | 7,3e-07 |
| ndcg_any@5 | 128 | 20 | 271 | 1,8e-20 |
| ndcg_any@10 | 137 | 26 | 256 | 2,1e-19 |

<!-- ablacion4-s.jsonl · abl-sin-cooc contra lexical · estricto · n=419 -->
| métrica | gana abl-sin-cooc | gana lexical | iguales | p |
|---|---|---|---|---|
| recall_all@5 | 87 | 3 | 329 | 2e-22 |
| recall_all@10 | 24 | 7 | 388 | 0,0033 |
| ndcg_any@5 | 249 | 26 | 144 | 7,1e-47 |
| ndcg_any@10 | 259 | 32 | 128 | 2,6e-45 |

Por tipo de pregunta (recall_all@5 · recall_all@10 · ndcg_any@10, estricto), en recall_all@5 y en
ndcg_any@10 no pierde en ningún tipo contra ninguno de los dos; empata con el léxico en recall_all@5
de single-session-assistant, que tiene n = 5. La única celda donde queda abajo es recall_all@10 de
knowledge-update contra el léxico de producción: 0,986 contra 1,000, una pregunta de 72.

<!-- ablacion4-s.json · longmemeval_s.json · modo user · 173 s -->
| tipo | n | bm25-paper | lexical | abl-sin-cooc |
|---|---|---|---|---|
| knowledge-update | 72 | 0,958 · 0,972 · 0,928 | 0,792 · 1,000 · 0,759 | 0,986 · 0,986 · 0,977 |
| multi-session | 121 | 0,554 · 0,702 · 0,731 | 0,529 · 0,777 · 0,695 | 0,736 · 0,826 · 0,859 |
| single-session-assistant | 5 | 0,800 · 0,800 · 0,800 | 1,000 · 1,000 · 0,572 | 1,000 · 1,000 · 1,000 |
| single-session-preference | 30 | 0,667 · 0,733 · 0,567 | 0,433 · 0,700 · 0,500 | 0,733 · 0,800 · 0,675 |
| single-session-user | 64 | 0,922 · 0,938 · 0,905 | 0,875 · 0,984 · 0,786 | 1,000 · 1,000 · 0,992 |
| temporal-reasoning | 127 | 0,701 · 0,819 · 0,770 | 0,598 · 0,858 · 0,657 | 0,819 · 0,921 · 0,861 |

**LongMemEval-M, promedio estricto (n = 419):**

<!-- ablacion4-m.json · longmemeval_m.json · modo user · 840 s -->
| brazo | n | recall_all@5 | ndcg_any@5 | recall_all@10 | ndcg_any@10 | recall_any@5 | recall_any@10 | rv_mrr | devueltos |
|---|---|---|---|---|---|---|---|---|---|
| abl-sin-cooc | 419 | 0,7088 | 0,7400 | 0,7852 | 0,7624 | 0,8902 | 0,9236 | 0,7680 | 465,9 |
| abl-sin-grafo | 419 | 0,3222 | 0,3623 | 0,5943 | 0,4441 | 0,6348 | 0,8186 | 0,4064 | 465,9 |
| abl-sin-planas | 419 | 0,3222 | 0,3623 | 0,5943 | 0,4441 | 0,6348 | 0,8186 | 0,4064 | 465,9 |
| abl-solo-lexico | 419 | 0,7088 | 0,7400 | 0,7852 | 0,7624 | 0,8902 | 0,9236 | 0,7680 | 465,9 |
| bm25-paper | 419 | 0,5680 | 0,5995 | 0,6563 | 0,6263 | 0,7446 | 0,8043 | 0,6323 | 502,0 |
| lexical | 419 | 0,3222 | 0,3623 | 0,5943 | 0,4441 | 0,6348 | 0,8186 | 0,4064 | 465,9 |
| turno | 419 | 0,2768 | 0,3338 | 0,6014 | 0,4306 | 0,5895 | 0,8138 | 0,3934 | 91,8 |

**En M se repite lo mismo, y más marcado:**
- `abl-sin-cooc` y `abl-solo-lexico` vuelven a ser idénticos en las 470 preguntas: la PRF explica
  toda la distancia.
- `abl-sin-grafo` y `abl-sin-planas` son idénticos al léxico en las 470. En M no aparece ni la
  pregunta de redondeo de S.
- La PRF cuesta más con el pajar más grande: en recall_all@5 estricto, apagarla lleva al léxico de
  0,3222 a 0,7088 en M, y de 0,6468 a 0,8473 en S.
- Sin la PRF, el léxico le gana a la réplica en las cuatro columnas, con p < 10⁻⁹ en las cuatro:

<!-- ablacion4-m.jsonl · abl-sin-cooc contra bm25-paper · estricto · n=419 -->
| métrica | gana abl-sin-cooc | gana bm25-paper | iguales | p |
|---|---|---|---|---|
| recall_all@5 | 67 | 8 | 344 | 1e-12 |
| recall_all@10 | 64 | 10 | 345 | 9e-11 |
| ndcg_any@5 | 150 | 47 | 222 | 9,7e-14 |
| ndcg_any@10 | 167 | 50 | 202 | 6,4e-16 |

<!-- ablacion4-m.jsonl · abl-sin-cooc contra lexical · estricto · n=419 -->
| métrica | gana abl-sin-cooc | gana lexical | iguales | p |
|---|---|---|---|---|
| recall_all@5 | 167 | 5 | 247 | 4,1e-43 |
| recall_all@10 | 88 | 8 | 323 | 3,7e-18 |
| ndcg_any@5 | 293 | 28 | 98 | 7,7e-57 |
| ndcg_any@10 | 308 | 32 | 79 | 8,5e-58 |

Por tipo, en M `abl-sin-cooc` queda arriba de los otros dos en los seis tipos y en las tres
métricas:

<!-- ablacion4-m.json · longmemeval_m.json · modo user · 840 s -->
| tipo | n | bm25-paper | lexical | abl-sin-cooc |
|---|---|---|---|---|
| knowledge-update | 72 | 0,819 · 0,889 · 0,835 | 0,319 · 0,708 · 0,525 | 0,931 · 0,958 · 0,914 |
| multi-session | 121 | 0,397 · 0,488 · 0,552 | 0,306 · 0,455 · 0,459 | 0,537 · 0,620 · 0,704 |
| single-session-assistant | 5 | 0,800 · 0,800 · 0,800 | 0,400 · 0,800 · 0,342 | 1,000 · 1,000 · 1,000 |
| single-session-preference | 30 | 0,367 · 0,433 · 0,315 | 0,300 · 0,433 · 0,257 | 0,500 · 0,633 · 0,501 |
| single-session-user | 64 | 0,750 · 0,875 · 0,717 | 0,500 · 0,781 · 0,446 | 0,969 · 1,000 · 0,922 |
| temporal-reasoning | 127 | 0,535 · 0,622 · 0,599 | 0,252 · 0,598 · 0,431 | 0,654 · 0,764 · 0,705 |

**Hipótesis del mecanismo, sin medir.** La PRF cosecha los términos que aparecen en al menos 2 de
los 5 primeros docs del léxico, los busca con un segundo FTS y suma ese ranking a la fusión **con el
mismo peso que la consulta**. En sesiones de chat enteras —miles de palabras cada una— los términos
compartidos por 2 de 5 sesiones son vocabulario genérico de conversación, y el segundo ranking arrastra
el tope hacia las sesiones más largas o más charlatanas: la deriva de consulta, el riesgo clásico de
PRF. Las observaciones de Musubi son mucho más cortas que una sesión, y ahí la PRF existe para otra
cosa (el puente de vocabulario «deploy» → «despliegue», Track 14 #2). Para confirmarlo, habría que
medir la PRF contra el largo de los docs.

**Lo que esto NO autoriza: apagar la co-ocurrencia en producción. Las dos varas que la midieron se
contradicen.** En el banco propio, el barrido de pesos de PR #454 (commit `12453b71`, 1.975 docs y
43 consultas, MMR apagado) reporta que llevar la co-ocurrencia a peso 0 **cuesta** −0,0998 de nDCG:
ahí la PRF ayuda. Acá, la misma operación **suma** +0,1956 de ndcg_any@10. Son métricas con otro
descuento, pero el signo es opuesto. Y cada vara tiene su sesgo:
- **El banco etiqueta por `topic_key`.** Una PRF que trae vocabulario del mismo topic cae justo
  sobre esa etiqueta, así que el banco puede estar sobrevaluándola.
- **LongMemEval tiene docs de miles de palabras**, todos del mismo topic y sin historia de uso.
  Ahí se castiga a la PRF por un largo que la memoria real no tiene.

El comentario de `memory.PesosRRF` ya dice que el default «no cambia hasta que una medición lo
mande», y ninguna de estas dos lo manda sola. Lo que sí autoriza es medir la PRF contra el largo de
los docs: una vara de docs cortos, o esta misma cortada en turnos. Las alternativas a probar son un
peso menor que 1, o una PRF que no se encienda sobre docs largos.

### 2.6 Costos, memoria, determinismo y anomalías

**Costo de cada corrida** (`/usr/bin/time -v`). La máquina estaba compartida con otras sesiones y
varias corridas se pisaron entre sí, así que el reloj es una cota de arriba, no una medición de
rendimiento. El caso más claro: `lexico-m-user` y `ablacion4-m` recorren el mismo M, y la segunda,
con más brazos, tardó 14 minutos contra 25 y gastó menos CPU (426 s de usuario contra 772). Las dos
corrieron enteras al lado de la híbrida, pero la primera además se pisó con las dos ablaciones de S.
Ni siquiera el tiempo de CPU es limpio: con dos hilos compartiendo un núcleo, el mismo trabajo cuesta
más CPU.

| corrida | dataset · modo | brazos | preguntas | reloj | s por pregunta | RSS máx | plazo del corrector vencido |
|---|---|---|---|---|---|---|---|
| bm25-s-full | longmemeval_s.json · full | bm25-paper | 500 | 0:16.26 | 0,03 | 27 MiB | 0 |
| bm25-m-user | longmemeval_m.json · user | bm25-paper | 500 | 0:56.91 | 0,11 | 55 MiB | 0 |
| bm25-m-full | longmemeval_m.json · full | bm25-paper | 500 | 2:16.02 | 0,27 | 112 MiB | 0 |
| lexico-user | dataset sin registrar · user | bm25-paper, lexical, turno | 500 | 2:31.23 | 0,30 | 30 MiB | 6 |
| ablacion-s | longmemeval_s.json · user | abl-sin-planas, abl-solo-lexico, bm25-paper, lexical, turno | 500 | 4:01.43 | 0,48 | 26 MiB | 21 |
| ablacion4-s | longmemeval_s.json · user | abl-sin-cooc, abl-sin-grafo, abl-sin-planas, abl-solo-lexico, bm25-paper, lexical, turno | 500 | 2:52.57 | 0,35 | 28 MiB | 2 |
| hibrido-user | dataset sin registrar · user | bm25-paper, hybrid, lexical, produccion, turno | 500 | 1:12:44 | 8,73 | 876 MiB | 7 |
| lexico-m-user | longmemeval_m.json · user | bm25-paper, lexical, turno | 500 | 24:48.03 | 2,98 | 66 MiB | 4 |
| ablacion4-m | longmemeval_m.json · user | abl-sin-cooc, abl-sin-grafo, abl-sin-planas, abl-solo-lexico, bm25-paper, lexical, turno | 500 | 14:00.14 | 1,68 | 65 MiB | 0 |
| abl-mmr-s-cada3 | longmemeval_s.json · user · una de cada 3 | bm25-paper, hybrid, lexical, prod-mmr-0.85, prod-mmr-0.90, prod-mmr-0.95, prod-sin-mmr, prod-sin-tipeo, produccion, turno | 167 | 20:13.21 | 7,27 | 1.107 MiB | 0 |

`lexico-user` y `hibrido-user` salieron del primer build del adaptador, que todavía no guardaba el
nombre del archivo en el JSON: de ahí el «dataset sin registrar». Lo que cambió después no toca la
siembra ni el ranking: el nombre de la variable y de la prueba, el modo `SOLO_BM25`, el campo
`dataset`, la ablación (opt-in) y comentarios. Que las dos corrieron sobre S, y que el build viejo
rankea igual que el nuevo, lo muestra la tabla de determinismo de abajo: `bm25-paper` y `lexical`
coinciden pregunta por pregunta con `ablacion4-s`, que sí registra el archivo.

- **Memoria, contra el techo de 1 GB de la spec.** Sin embebedor, ninguna corrida pasó de 112 MiB
  de RSS, M incluido: el lector decodifica una pregunta por vez, así que el archivo de 2,7 GB nunca
  está entero en memoria. La híbrida carga la tabla de POTION (~512 MB en float32) y corrió con
  `GOMEMLIMIT=900MiB`. Su RSS máximo está en la tabla, pero el RSS no cuenta lo que el kernel mandó
  a swap, y la máquina estaba swapeando. Por eso se muestreó `VmRSS` + `VmSwap` de
  `/proc/<pid>/status` cada 20 s. El muestreo arrancó tarde y cubre sólo los últimos 22 minutos:

  hibrido-user: 68 muestras entre las 15:14:43 y las 15:37:04; máximo de RSS + swap 906 MiB a las 15:24:43 (RSS 371 MiB + swap 535 MiB)

  En lo muestreado, el máximo quedó abajo de 1 GB (1.024 MiB). De los primeros 50 minutos sólo está
  el RSS máximo de toda la corrida, 876 MiB, que no cuenta el swap: para ese tramo no se puede
  afirmar que no pasó de 1 GB. `GOMEMLIMIT` lo acota, pero es un límite blando del runtime de Go, no
  una garantía.

- **Determinismo entre corridas.** Los brazos sin corrector de tipeo dan exactamente las mismas
  métricas en todas las corridas, pregunta por pregunta. `turno` y `produccion` pasan la consulta
  por el corrector, que tiene un plazo de 60 ms: bajo carga el plazo vence, la consulta va sin
  corregir y el ranking puede cambiar (la última columna de la tabla de costos cuenta esos
  vencimientos). Es la única fuente de diferencia que apareció. `hybrid` y `produccion` corrieron
  una sola vez, así que su determinismo no está medido.

| brazo | contra lexico-user | preguntas en común | con alguna métrica distinta | cuáles |
|---|---|---|---|---|
| bm25-paper | ablacion-s | 470 | 0 | — |
| bm25-paper | ablacion4-s | 470 | 0 | — |
| bm25-paper | hibrido-user | 470 | 0 | — |
| lexical | ablacion-s | 470 | 0 | — |
| lexical | ablacion4-s | 470 | 0 | — |
| lexical | hibrido-user | 470 | 0 | — |
| turno | ablacion-s | 470 | 1 | 32260d93 |
| turno | ablacion4-s | 470 | 0 | — |
| turno | hibrido-user | 470 | 1 | 32260d93 |

| brazo | contra bm25-m-user | preguntas en común | con alguna métrica distinta | cuáles |
|---|---|---|---|---|
| bm25-paper | lexico-m-user | 470 | 0 | — |
| bm25-paper | ablacion4-m | 470 | 0 | — |

| brazo | contra lexico-m-user | preguntas en común | con alguna métrica distinta | cuáles |
|---|---|---|---|---|
| lexical | ablacion4-m | 470 | 0 | — |
| turno | ablacion4-m | 470 | 0 | — |

- **Anomalías del dataset.** Las 500 preguntas se reparten igual en todas las corridas, y en las
  mismas proporciones que en los scripts del paper. Ninguna pregunta entró al promedio estricto sin
  oro. No hubo listas del pajar con largos distintos (ids, sesiones y fechas). La base guardó una
  observación por sesión en todas las preguntas: no dejó afuera ninguna. En M hay
  empates de BM25 justo en el corte: el orden entre empatados sigue al `argsort` de numpy, como en
  el paper, así que en esas preguntas la métrica depende del orden del corpus también allá.

| corrida | estrictas | abstenciones | sin oro de usuario | evaluadas sin oro | listas desparejas | docs que la base no aceptó | empates de BM25 en el corte (@5 · @10) |
|---|---|---|---|---|---|---|---|
| bm25-s-full | 419 | 30 | 51 | 0 | 0 | 0 | 0 · 0 |
| lexico-user | 419 | 30 | 51 | 0 | 0 | 0 | 0 · 0 |
| ablacion4-s | 419 | 30 | 51 | 0 | 0 | 0 | 0 · 0 |
| hibrido-user | 419 | 30 | 51 | 0 | 0 | 0 | 0 · 0 |
| bm25-m-user | 419 | 30 | 51 | 0 | 0 | 0 | 1 · 5 |
| bm25-m-full | 419 | 30 | 51 | 0 | 0 | 0 | 3 · 0 |
| lexico-m-user | 419 | 30 | 51 | 0 | 0 | 0 | 1 · 5 |
| ablacion4-m | 419 | 30 | 51 | 0 | 0 | 0 | 1 · 5 |

### 2.7 Qué no se midió, y por qué

- **QA de punta a punta:** la unidad pedía sólo recuperación, y un lector LLM gasta cuota.
- **Granularidad por ronda** (la columna «Value = round» de la Tabla 9): el paper la calcula
  convirtiendo turnos a sesiones con un `k` efectivo que crece (`evaluate_retrieval_turn2session`).
  Es otra adaptación, y Musubi guarda observaciones, no turnos.
- **La versión limpia del dataset** (`longmemeval-cleaned`): los números del paper son del
  original, y la vara era replicarlos. Correrla es cambiar la ruta del archivo.
- **Los brazos híbridos en M.** No se corrieron. En S, la híbrida es por lejos la corrida más cara
  (§2.6), y M multiplica por diez el pajar de cada pregunta. El caché de embeddings de la prueba
  rinde más en M que en S, porque ahí las sesiones se repiten más entre preguntas. Contando
  `haystack_session_ids`: en M hay 52.476 distintas en 250.948 lugares, y en S 19.829 en 25.112.
  Pero la siembra y la búsqueda son por pregunta, sobre ~500 docs cada vez, **y no se perfiló dónde
  se va el tiempo de la híbrida**, así que no hay una proyección honesta para M. Lo medido con
  `/usr/bin/time -v`:
  - 4.314 s de CPU de usuario en 4.364 s de reloj: en promedio, un núcleo entero durante toda la
    corrida. También es una cota de arriba, porque corrió al lado de las otras.
  - `lexico-user`, que recorre el mismo archivo con los mismos brazos léxicos, gastó 81 s. La
    diferencia, unos 4.230 s, es todo lo que agrega el vector: embeber ~20 mil textos distintos
    (19.829 sesiones más las consultas), guardar los vectores y buscar con ellos, MMR incluido.
  - 1,1 millones de fallos de página mayores (la tabla de POTION iba y venía del swap), y aun así la
    CPU promedió 100 %: la corrida fue cálculo, no espera al disco.

  Repartidos entre los textos, son ~0,2 s por sesión: muchísimo para un embebedor estático, que es
  tokenizar, buscar filas y promediar. **Una hipótesis de código, sin medir:**
  - `NewStaticProvider` arma el tokenizer desde `tokenizer.json`, y ese camino busca las piezas del
    vocabulario con un mapa (`unigram.coincidencias`): en cada runa del texto prueba todos los largos
    hasta `maxRunes` y arma un string para cada uno, sin cortar antes.
  - En POTION multilingual, `maxRunes` vale 186 (leído del encabezado de `tokenizer.idx`).
  - El índice ordenado que usa la consulta liviana (`piezasOrdenadas`) contesta lo mismo y corta
    apenas ninguna pieza sigue coincidiendo.
  - Si es eso, el costo por runa es fijo y alto: no se nota en observaciones cortas y pesa en
    sesiones enteras. Aparte, el Viterbi arma los ids anteponiendo (`append([]int{id}, ids...)`),
    que es cuadrático en la cantidad de tokens.

  Se confirma con un benchmark de `EncodeIDs` sobre un texto largo por los dos caminos, y es lo
  primero que hay que hacer antes de correr la híbrida en M. **Confirmada el 2026-10-01:** el camino
  del mapa tarda 77–88 ms por cada 1.000 caracteres, y el del índice, 0,65 ms. Es ~100× más rápido
  y da los mismos ids. Queda como propuesta: que `NewStaticProvider` use `tokenizer.idx` cuando
  existe. No se hizo acá por la RAM: la máquina
  la compartían otras sesiones y llegó a tener menos de 100 MB disponibles, con el swap casi lleno.
- **Un solo embebedor:** POTION es el que Musubi usa sin red. Contriever y Stella, los densos del
  paper, necesitan GPU o un servicio aparte.

---

## 3. Factibilidad de ARB y ForgetEval-Adv (PASO 3, sin implementar)

### 3.1 Agent Retrieval Bench (ARB)

**Qué mide.** Dada una señal real de trabajo (un test, un comentario de review, una traza, una
edición) y un repo congelado en un commit, ¿el recuperador trae los archivos que el agente
necesita? ¿Se abstiene cuando el repo no tiene la respuesta? 427 muestras en 25 repos:

| Subconjunto | n | Señal de consulta | Oro |
|---|---|---|---|
| code2test | 106 | intención de un PR o señal de cambio de implementación | los tests relacionados |
| comment2context | 80 | comentario de review + un archivo ya dado | los archivos de contexto que faltan |
| trace2code | 101 | salida de una falla reproducida | los archivos fuente de la causa raíz |
| **edit2ripple** | 58 | intención + un cambio anclado en un archivo | los otros archivos afectados |
| no-gold natural | 50 | issue resuelto fuera del repo | abstenerse |
| no-gold contrafactual | 32 | consulta plausible contra el repo equivocado | abstenerse |

Métricas: MRR, Recall@k, P@k/F0.5 y BCY@4k–32k (*Budgeted Context Yield*: se empaquetan los
archivos rankeados hasta un presupuesto de tokens y se mide cuánto oro entra).
Publicado, sobre positivos (R@20 · MRR · BCY@8k): BM25 0,4452 · 0,1520 · 0,2051; léxico
0,4940 · 0,1574 · 0,2650; RepoMap 0,6333 · 0,2158 · 0,3788; Qwen3-Emb-8B 0,7029 · 0,2336 · 0,3732.

**Mapeo a Musubi.**
- `edit2ripple` ↔ `musubi_impact` (cierre transitivo de callers de un símbolo) sobre el grafo de
  `musubi_codegraph_index`: del hunk anclado salen los símbolos tocados, de `musubi_impact` sus
  callers, y el ranking de archivos sale de la profundidad del BFS. Se mide R@k y MRR por archivo.
- `code2test`, `comment2context` y `trace2code` ↔ `musubi_recall_code`, `musubi_search_keyword` y
  `musubi_code_context`. Miden la recuperación de código de Musubi, no su memoria.
- Los no-gold ↔ la abstención del recall (track `renaissance-f3-abstencion`).

**Lo que limita el mapeo, medido en el README de ARB contra la cobertura del grafo de Musubi.** El
grafo extrae consts, funcs, métodos, tipos y vars en **Go**; en TS/JS/Python, sólo clases y
funciones de nivel superior (ni métodos, ni `async def`); en cualquier otro lenguaje, **nada**. Los
58 casos de `edit2ripple` son: **29 en Go** (gin 17, caddy 6, etcd 6), 10 en Python (transformers 8,
diffusers 1, pytest 1), 5 en TypeScript (vite), 11 en Rust (tokio 6, clap 5) y 3 en Java
(spring-boot). O sea: medible de verdad en 29, a medias en 15, **imposible en 14**. Y un ripple no es
sólo una arista CALLS: tests, docs y configuración cambian juntos sin llamarse. Un número global
castigaría la cobertura de lenguajes, no el ranking; hay que reportar por lenguaje.

**A verificar antes de construir:** si el corpus trae archivos enteros o sólo chunks. El grafo de
código necesita archivos enteros; si vienen chunks, hay que reconstruir el snapshot desde git (el
commit base está en la metadata) y eso es descargar los repos.

**Esfuerzo:** `edit2ripple` sólo en Go, **2 a 3 días**: lector del JSONL, snapshot por commit,
indexado del grafo, ranking por impacto y métricas, con sus sabotajes. Los otros tres subconjuntos
positivos, **1 a 2 días más**, porque reusan el lector y las métricas. La descarga mínima son
128,7 MB (`edit2ripple`). **Licencias:** el corpus es de 25 repos con licencias propias. Se puede
usar localmente para medir, pero **no se commitea ni se redistribuye**.

### 3.2 ForgetEval-Adv

**Qué mide.** Si una memoria **olvida bien**. Cada caso inscribe hechos (`setup_facts`), aplica
mutaciones y hace una consulta final. Pasa si el top-10 **contiene** lo que tiene que estar
(`must_contain`) y **no contiene** lo que se tuvo que ir (`must_not_contain`). La comparación es
por substring sobre el texto del top-10, 1/0 por caso; un método que el adaptador no implementa
cuenta como falla.

- **Mutaciones:** `("supersede", consulta_vieja, texto_nuevo)`, `("release", consulta)` y
  `("purge", consulta)`.
- **Familias:** supersession, decay, amnesia, purge y drift.
- **10 categorías de ataque:** `substring_trap`, `prefix_collision`, `paraphrase_supersession`,
  `negation_trap`, `temporal_qualifier`, `shared_attribute`, `compound_fact`,
  `identifier_obfuscation`, `cross_lingual_identifier` y `recursive_supersession`.
- **Casos:** 385 = 132 escritos a mano + 253 redactados por LLM y validados por oráculo. Las
  etiquetas de los generados: easy 174, llm_lift 55, unsolvable 24. Aparte está la suite ForgetEval
  de 1000 casos por plantilla, que los sistemas publicados ya saturan (Lethe 99,3 %).
- **Resultados publicados:**
  - sobre los 385 casos (paper v2): Lethe 244 (63,4 %), Mem0 263 (68,3 %), LangGraph 242
    (62,9 %) y LangGraph con LLM 359 (93,2 %). El paper llama a 63–68 % «la banda» de los
    sistemas sin LLM en la mutación;
  - en el documento v0.4 del repo, que todavía tenía 112 casos: Lethe 62,5 %, Mem0 67,9 %,
    LangMem 61,6 % y MemPalace 0 %.

**Mapeo a Musubi.**

| ForgetEval | Musubi | Estado |
|---|---|---|
| `reset` | base temporal nueva, como en §2.1 | listo (ya existe en este adaptador) |
| `inscribe(texto)` | `SaveObservation` | listo |
| `recall_texts(consulta)` | `Recall` + el contenido completo (el gist puede recortar, y el puntaje es por substring) | listo, con cuidado |
| `supersede(vieja, nuevo)` | guardar el nuevo + `musubi_judge relation=supersedes` sobre la vieja (el target queda oculto del recall) | **a verificar:** `musubi_judge` juzga un `relation_id` que ya existe. Si el detector de conflictos no liga la paráfrasis con la vieja (justo lo que ataca `paraphrase_supersession`), no hay relación que juzgar, y hace falta crearla desde el motor |
| `release(consulta)` | olvido sin borrado: ¿archivar?, ¿decaimiento? | **a decidir:** no encontré una primitiva de Musubi que suelte un recuerdo a pedido |
| `purge(consulta)` | borrado duro | **a verificar:** no hay tool de borrado en el MCP. Y una nota del acervo avisa que en SQLite un `UPDATE`/`DELETE` deja la página vieja libre: para que sea borrado de verdad hacen falta `secure_delete` o `VACUUM` |
| `conflicts_with` | ambas quedan visibles | es lo contrario de lo que pide el caso: no sirve para `supersede` |

**Riesgos.**
1. Las categorías de identificadores (`cross_lingual_identifier`, `identifier_obfuscation`) piden
   reconocer la misma entidad bajo otra escritura. Un Musubi model-free va a fallar ahí como falla
   Lethe (0 % en `cross_lingual_identifier`).
2. Los casos son código Python. **Se parsean sin ejecutarlos**: un parser en Go para el subconjunto
   literal (`GeneratedCase(...)` con strings, listas y tuplas), o el módulo `ast`, que parsea sin
   correr nada. Preferible Go, para no sumar otra cadena de herramientas.
3. Los 253 casos generados tienen `unsolvable` = 24: se reportan aparte, no se mezclan con el
   puntaje.

**Esfuerzo: 3 a 5 días.**
- Parser de los casos, con sus sabotajes: 1 día.
- Adaptador: 1 a 2 días. Depende de dos decisiones previas, qué primitiva es `release` y cómo se
  crea una relación `supersedes` sin conflicto detectado.
- Corrida y análisis por categoría: 1 día. Corre en segundos: son cientos de hechos, no millones de
  chunks.

### 3.3 Orden recomendado

1. **ForgetEval-Adv primero.** Mide lo que distingue a Musubi como producto: supersede, conflictos y
   olvido. Es barata de correr y no trae problemas de licencia. Antes de escribir código hay que
   cerrar las dos decisiones de la tabla (`release` y la relación sin conflicto previo): sin ellas,
   el adaptador mediría un contrato inventado.
2. **ARB `edit2ripple`, sólo Go.** Mide `musubi_impact` contra un oro externo, con el techo de
   cobertura dicho por lenguaje.
3. Los otros subconjuntos de ARB, si el grafo de código pasa a ser la vara principal.
