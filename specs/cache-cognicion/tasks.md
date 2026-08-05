# Tareas — Caché de cognición (F3)

Estado: **completo**. Build, vet y los paquetes tocados en verde.

- [x] **T1 — Decorador** (`internal/cognition/cache.go`): LRU con `map` + `container/list`, TTL con
      reloj inyectable, clave con prefijo de largo, y `Stats()` para la telemetría de F5.

- [x] **T2 — Config** (`CacheConfig`): `enabled` (**`*bool`**, para distinguir "no lo escribieron"
      de "lo apagaron"), `max_entries` y `ttl_seconds`. `max_entries <= 0` con el caché encendido es
      error explícito, no un default silencioso.

- [x] **T3 — Cableado** en `NewProvider`, por FUERA del portero y también por fuera del router:
      una pregunta repetida no debería ni elegir motor.

- [x] **T4 — 13 tests de invariantes** (`cache_test.go`), con nombres `TestK*`.

- [x] **T5 — Sabotaje: 4 mutaciones, cada una en rojo.**

      | Sabotaje | Invariante | Resultado |
      |---|---|---|
      | Clave sin prefijo de largo (concatenar a secas) | K8 | rojo — 3 pares colisionaron + el test punta a punta |
      | Cachear la respuesta vacía ante un error | K2 | rojo |
      | Vaciar el caché entero al llenarse (como el `rerankCache`) | K4 | rojo los 2 tests |
      | Ignorar el TTL | K6 | rojo |

- [x] **T6 — Tres tests de F1/F2 arreglados sin aflojarlos.** El caché cambia lo que devuelve
      `NewProvider`, así que `TestFactoryBuildsOpenAICompat`, `TestC6SinFlotaNoHayRouter` y
      —la importante— `TestInspectGatewayNoLeMienteAlConstructor` empezaron a fallar.

      Se resolvió con un helper `unwrapCache`, **no** relajando las aserciones. Lo que esos tests
      protegen —que el motor real nunca salga de fábrica sin portero, y que el doctor cuente la
      misma historia que el constructor— se sigue verificando igual de fuerte, y una capa nueva
      mañana no vuelve a romperlos.

- [x] **T7 — Renombre de los invariantes a `K*`.** El paquete ya tenía `C0..C7` (los del router,
      F2). Dos juegos de invariantes con los mismos nombres en el mismo paquete Go es una trampa
      para el que lea después.

      *Gotcha del renombre automático*: el `-replace` pisó una referencia legítima al **C6 del
      router** en `factory.go`. Buscar y reemplazar sobre identificadores cortos toca cosas que no
      son suyas — se revirtió a mano.

## Correcciones al plan original, dichas de frente

- **No es un caché semántico.** El roadmap lo llamaba así; se entregó matcheo **exacto**. La carpeta
  se renombró de `cache-semantico-cognicion` a `cache-cognicion` para que el nombre no afirme lo que
  el código no hace. El porqué del diferimiento está en K5, y el argumento más fuerte no es el
  costo: un hit por similitud devuelve la respuesta de **otra pregunta**, lo que está en tensión
  directa con K0 y merece su propia fase con su propio umbral medido.

- **K7 no se verificó en esta máquina.** `-race` exige cgo y acá no hay compilador de C. El test de
  concurrencia comprueba la cota y el conteo; la ausencia de carreras la certifica la CI.

## Fuera de alcance

- **Borrar el `rerankCache`.** Cachea otra cosa (el orden de ids ya parseado). Es una limpieza
  aparte; dejar los dos conviviendo **sin decirlo** sería deuda escondida.
- **Persistencia en disco.** Necesita invalidación, versionado de formato y una decisión sobre
  escribir secretos en disco — que es un no.
- **Publicar la métrica.** `Stats()` ya cuenta; exponerlo es F5.
