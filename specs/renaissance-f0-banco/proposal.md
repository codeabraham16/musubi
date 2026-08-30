# F0 · El banco del motor de diseño

Primera fase de [renaissance-rey-del-diseno](../renaissance-rey-del-diseno/proposal.md).

## El problema, con su fecha

El 2026-08-21 el motor de diseño se degradó de golpe: el bloque de método pasó de 8 principios
constantes (~700 chars) a 30 tarjetas del acervo (16.728 chars, **24×**), y el acervo saltó de ~525 a
1.736 entradas el mismo día. Los dos commits eran razonables por separado. Juntos convirtieron el
brief en un sermón donde el 68 % del texto es idéntico para cualquier pedido.

**Nadie lo notó durante ocho días**, hasta que el usuario lo sintió usándolo en Altura. No existe —ni
existió nunca— ninguna medida de calidad de recuperación del motor. Las suites verdes de
`methods_design_test.go` siguieron pasando todo el tiempo, porque miden que el brief *se arme*, no que
sirva.

Ésa es la falla que arregla esta fase: **el motor no tiene marcador.**

## Qué entrega

Un banco que convierte «el diseño salió feo» en un número que se puede ver empeorar en un PR.

1. **Set dorado** de pedidos de diseño reales de los proyectos vivos, cada uno con paráfrasis, más
   consultas fuera de dominio y payloads de inyección. Un solo archivo, consumido por los dos bancos.
2. **Banco estructural** (`go test`, offline, en CI): mide tamaño del brief, fracción variable,
   abstención y superficie de inyección. Con **umbrales versionados** fijados hoy en el valor
   medido, de modo que cualquier regresión sale en rojo.
3. **Sonda de producción** (bajo demanda, tras `-tags sonda`): mide contra el central real lo que
   depende del embebedor y del acervo vivo — estabilidad de paráfrasis, precisión y cobertura.

## Una corrección al plan del track

La propuesta del track decía que el banco entero corre «sin red y sin LLM, si no, no entra a CI».
**Eso no se puede sostener para todas las métricas y conviene decirlo ahora.**

La estabilidad de paráfrasis (M1), la precisión@6 (M3) y la cobertura (M8) dependen del embebedor real
(bge-m3) y del acervo real de 1.736 entradas. Medirlas offline exigiría un embebedor falso — y un
embebedor falso mide al embebedor falso, no al motor. Es exactamente el modo de falla que este
proyecto ya documentó cuatro veces: *el test espera el proxy, no la cosa*.

Por eso el banco se parte en dos, y cada mitad mide lo que honestamente puede medir:

| Métrica | Dónde vive | Por qué |
|---|---|---|
| M2 abstención · M4 tamaño · M5 fracción variable · M6 inyección | banco estructural, en CI | son propiedades del **ensamblado**, ciertas con cualquier recuperador |
| M1 paráfrasis · M3 precisión@6 · M7 latencia · M8 cobertura | sonda de producción | dependen del **embebedor y del acervo reales** |

## Alcance

Esta fase **no arregla nada del motor**. Sólo lo mide. Los umbrales quedan clavados en el valor de
hoy —incluidos los malos— y cada fase siguiente los aprieta al aterrizar. Un banco que ya naciera
exigiendo el objetivo estaría rojo desde el commit uno y se apagaría a la semana.

---

## Línea base medida (2026-08-29, contra el central `0.107.0-main.05c016a`)

### Banco estructural — fixture, offline
| Métrica | Valor | Umbral fijado |
|---|---|---|
| M2 abstención fuera de dominio | 0,25 | ≥ 0,25 |
| M4 tokens del brief · p50 | 6.419 | ≤ 6.600 |
| M4 tokens del brief · máximo (`limit=100`) | 7.268 | ≤ 7.500 |
| M5 fracción variable por pedido | 0,047 | ≥ 0,04 |
| M6 payload del prompt fuera de instrucción | 1,00 | ≥ 1,00 |
| M6 payload del prompt fuera del eco | 0,00 | ≥ 0,00 |
| M6 payload del acervo fuera de instrucción | 0,00 | ≥ 0,00 |

### Sonda — central real, acervo de 1.736 entradas
| Métrica | Valor | Objetivo del track |
|---|---|---|
| M1 estabilidad de paráfrasis | **0,09** | ≥ 0,80 |
| M3 precisión temática @6 | **0,22** | ≥ 0,80 |
| M2 abstención fuera de dominio | **0,00** | 1,00 |
| M7 latencia p50 / p95 | 571 / 628 ms | ≤ 1.200 ms |
| M8 ids distintos servidos | 190 | — |

**Lo que la línea base agregó a lo que ya sabíamos.** La auditoría había medido la estabilidad de
paráfrasis en 0,21 sobre un solo pedido. Sobre los 16 pedidos reales da **0,09**, y tres de ellos
—`altura-orden-produccion`, `crm-ficha-cliente`, `panel-grafo`— dan **0,00**: dos maneras de pedir
lo mismo no comparten ni un patrón. Y la precisión temática, que no se había medido nunca, da
**0,22**: apenas uno de cada cinco items del corpus servido toca el tema que se pidió.

El único caso que se comporta es `generico-tabla-accesible` (precisión 0,78) — y es justamente el
pedido cuyo vocabulario coincide literalmente con los `topic_key` del acervo. Es una pista sobre qué
está recuperando de verdad el motor.

## Gotcha de la sonda en kernelos-pc

NordVPN excluye por **ruta exacta del binario, nombre de archivo incluido**. Medido: recompilar el
binario de test dentro de la carpeta de `musubi.exe` **tampoco** alcanza. El binario efímero de
`go test` recibe `socket ... forbidden by its access permissions` en el primer dial, que se lee
igual que un central caído y no lo es. Por eso la sonda acepta un transporte alternativo:

```bash
MUSUBI_SONDA_CURL='C:\Windows\System32\curl.exe'   go test -tags sonda ./internal/mcp -run TestSondaDiseno -v
```

Sin esa variable usa `net/http` directo, que es lo correcto en la laptop, en el server y en CI.
