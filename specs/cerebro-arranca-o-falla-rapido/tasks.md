# Tareas — El canal arranca, o falla rápido

Estado: **completo**. Build, `go vet ./...`, la suite entera y `golangci-lint` (0 issues) en verde.
5 invariantes (C1–C5), 5 sabotajes, los 5 en rojo.

## El incidente, y cómo se llegó a la causa

- [x] **T0 — El mensaje del error apuntaba al lado equivocado.** «check authentication and network
      connectivity» con «OAuth tokens found in secure storage». Medido en la máquina afectada:
      `api.anthropic.com:443` **alcanzable**, tokens presentes, **19,76 GB de RAM libres**, **cero
      procesos claude** colgados. Ni auth ni red ni recursos.

- [x] **T0b — Dos hipótesis mías, descartadas por medir.** Primero pensé en contención (había cinco
      terminales abiertas): la RAM la desmintió. Después en que la red estaba caída: el
      `[skills] idle` de los logs probaba que otros canales seguían hablando.

- [x] **T0c — ★ Y una tercera, corregida por el cronómetro.** Vi que el timeout del cliente era 60 s
      y el del host 60 s, y dije «el que debería proteger nunca puede ganar». **Falso.** Medido
      contra una IP que no rutea: un request falla en **21 s**, no en 60 — gana el timeout de
      conexión del SO. El problema no era una request, era **la suma de tres**:

      ```
      initialize                 21 s
      notifications/initialized  21 s   <- nadie espera su respuesta, y cuesta igual
      tools/list                 21 s
                               ------
                                 63 s   contra los 60 s del host   → margen: 3 s
      ```

## Lo construido

- [x] **T1 — `clienteCerebro(timeoutSeg, dialSeg)`** con los dos timeouts **separados**: uno acota
      cuánto se espera a que el otro extremo CONTESTE, el otro cuánto para saber si siquiera ESTÁ.
- [x] **T2 — `defaultDialTimeoutSeg = 5`**, con el porqué medido escrito al lado de la constante.
- [x] **T3 — Flag `--dial-timeout`**, y un valor ≤ 0 cae al default (nunca a «sin límite»).
- [x] **T4 — 5 invariantes y 5 sabotajes**, los 5 en rojo.

      | Sabotaje | Inv. | Resultado |
      |---|---|---|
      | El dial vuelve a manos del SO | C1 | rojo |
      | El default se agranda a 25 s | C2 | rojo |
      | El dial pisa el timeout de request | C3 | rojo |
      | Un dial de 0 vuelve a ser «sin límite» | C4 | rojo |
      | Desaparece el timeout de request | C5 | rojo |

## El número

Arranque completo con el central caído, medido por C2:

| | antes | ahora |
|---|---|---|
| `initialize` + `initialized` + `tools/list` | **63 s** | **15,001 s** |
| presupuesto del host | 60 s | 60 s |
| resultado | la sesión **no levanta** | levanta, sin el canal federado |

## Lo que enseñó

**Un mensaje de error puede mandarte a buscar donde no es.** «check authentication and network
connectivity» con los tokens en su lugar y la red sana: quien lo lea sin medir se va a pasar la
tarde revisando credenciales. Lo que lo resolvió fue cronometrar, no leer.

**Dos timeouts que se parecen no son el mismo.** «Cuánto espero a que conteste» y «cuánto espero
para saber si está» son preguntas distintas, y dejar la segunda en manos del sistema operativo es
delegar una decisión de producto a un default que nadie eligió.

## Fuera de alcance, dicho de frente

- **Sin reintentos ni circuit breaker.** Con el central caído cada llamada sigue costando 5 s. Este
  spec compra que la sesión **arranque**, no que ande bien sin central.
- **La notificación `initialized` sigue costando su dial completo** aunque nadie espere su
  respuesta: es un tercio del arranque y se podría mandar sin bloquear. Se deja anotado.
- **No se tocó la máquina afectada.** Ahí no había nada roto: el central ya volvió y le responde.
