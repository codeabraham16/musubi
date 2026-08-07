# Tareas — El arranque con arsenal (Fase B · track «Conocimiento unificado»)

Estado: **completo**. Build, vet, `golangci-lint` (0 issues) y la suite entera en verde.

- [x] **T1 — Medir antes de proponer, y corregirme.** La Fase A cerró declarando que «un proyecto
      nuevo necesita `sync.central_url` configurado». **Es falso**: `musubi provision` ya lo escribe
      (`ensureSyncConfig`, paso 5 de `Run`). El hueco real era otro y peor porque no se ve: **no
      había forma de VER el arsenal**, así que `musubi_install_skill` exigía saber el nombre exacto
      de memoria. Una tool de instalación sin descubrimiento es una adivinanza.

- [x] **T2 — Dejar de tirar lo que ya llegaba.** `FetchSkill` pedía el catálogo entero al central y
      descartaba todo menos el match exacto. Se extrajo `ListArsenal(query)` y `FetchSkill` pasó a
      usarlo: cero plomería nueva, cero transporte nuevo, y una duplicación menos —el parseo del
      sobre y la distinción `[]` vs `null` viven ahora en un solo lugar.

- [x] **T3 — B1: `musubi_list_skills` gana `source`** (`local` · `central` · `all`), más los campos
      `origin` e `installed`. `installed` es puntero para poder **omitirse** en las entradas
      locales: un `false` ahí sería falso y un `true` trivial sería ruido.

- [x] **T4 — B2: `musubi provision --skills`.** Instala el arsenal al unir el proyecto. Sin el flag
      no se hace **ninguna llamada al central**: sólo queda un paso `todo` que dice cómo pedirlo.

- [x] **T5 — El orden importa, y es al revés de lo obvio.** El paso del arsenal corre **después** de
      `injectLocalSetup`, no antes. Las skills cognitivas que escribe `setup` son las locales del
      proyecto; con el orden invertido, `setup` sobrescribiría lo recién adoptado. Puesto último, el
      arsenal ve lo local y lo saltea (G10).

- [x] **T6 — 12 invariantes con test propio**, cada uno con su control donde hacía falta: G5 verifica
      que `local` SIGUE andando sin central, G6 que la tolerancia de forma sí existe, G10 que la
      skill faltante sí se instala, G12 que la sana se instala igual que la maliciosa se rechaza.

- [x] **T7 — Sabotaje: 13 mutaciones, cada una en rojo.**

      | Sabotaje | Invariante | Resultado |
      |---|---|---|
      | El default sale a la red | G1 | rojo |
      | `central` lee el disco local | G2 | rojo |
      | Nunca marca `installed` | G3 | rojo |
      | `all` duplica lo ya instalado | G4 | rojo |
      | Sin central, degrada en silencio a local | G5 | rojo |
      | Un `source` inválido cae al default | G6 | rojo |
      | `list_skills` deja de ser `readOnly` | G7 | rojo |
      | `"null"` del central se toma por arsenal vacío | extra | rojo |
      | El paso informativo llama al central | G8 | rojo |
      | Reporta instaladas sin escribir | G9 | rojo |
      | Pisa lo local | G10 | rojo |
      | `--dry-run` igual escribe | G11 | rojo |
      | **Abre una SEGUNDA puerta de escritura** | G12 | rojo |

- [x] **T8 — Extremo a extremo contra el binario y un central REAL** (`musubi serve` en loopback),
      con **tres proyectos**: el arsenal arranca vacío → B no puede instalar lo que no existe → A
      promueve → **B ve la skill completa marcada `installed:false`** → B la instala con sus
      triggers y reglas → el arsenal pasa a `installed:true` → `all` no la duplica → re-instalar sin
      `overwrite` se rechaza y el archivo queda byte por byte igual → `provision --skills --dry-run`
      reporta el faltante sin escribir → sin el flag deja la guía. **19 de 19 chequeos.**

## Dos cosas que el arnés enseñó

**El binario recién compilado no alcanza el tailnet.** El primer intento de e2e apuntaba al central
de producción y murió con *«socket in a way forbidden by its access permissions»*: NordVPN excluye
por **ruta exacta**, y `musubi-e2e.exe` en el scratchpad no está en la lista. Por eso el e2e final
levanta su propio central en loopback. La compatibilidad de formato con el central real se verificó
aparte, con el binario global —que sí está excluido— contra `100.79.126.62:7717`.

**Una aserción de ruta fija sólo caza a un atacante.** La primera versión de G12 verificaba la fuga
mirando `filepath.Dir(root)/evil.yaml`. Pero `../evil` aterriza en `.musubi/`, no ahí: el payload de
un nivel se le escapaba. Ahora barre el árbol entero buscando cualquier archivo `evil*`. El test
igual habría fallado por `Fallidas`, pero por el motivo equivocado.

## Fuera de alcance, dicho de frente

- **No se re-traen actualizaciones.** Una skill del arsenal que cambió después de instalada no se
  refresca. La marca de procedencia deja la puerta abierta; la comparación no está construida.
- **`--skills` trae el arsenal entero.** Elegir un subconjunto se hace con `musubi_install_skill` de
  a una. Es deliberado: la curaduría se cobra al subir.
- **Sigue sin poder distinguirse un `*` intencional de un `*` por pereza.** Con skills de varias
  personas adentro va a ser el default, y el contexto se va a llenar de reglas que no aplican. Es el
  próximo problema real de este track.
- **El arsenal sigue teniendo 1 skill.** Nada se promovió todavía: eso es curaduría del dueño, no
  código.
