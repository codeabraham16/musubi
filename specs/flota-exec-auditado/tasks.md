# Tasks — S5 · Ejecución remota auditada

El plano de terminal. Suite entera verde, vet limpio, cross-compila en las tres plataformas.

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | Dominio: `Comando`, estados, límites, vencimiento, truncado | `internal/fleet/comando.go` |
| T2 | Migración **31**: `device_commands` — la tabla más sensible del esquema | `internal/memory/migrations.go` |
| T3 | Store: encolar, tomar (transaccional), guardar resultado, bitácora, poda | `internal/memory/comandos.go` |
| T4 | `musubi_fleet_exec` y `musubi_fleet_log` | `internal/mcp/methods_exec.go` |
| T5 | Los comandos vuelven en el latido; `POST /fleet/result` | `internal/mcp/fleet_http.go`, `http.go` |
| T6 | El ejecutor del agente: argv sin shell, timeout que mata, buffers acotados | `cmd/musubi/ejecutor.go` |
| T7 | Las 5 guardas del proyecto, declaradas | varios `_test.go`, ambos READMEs |

## Invariantes

| Inv | Test | Sabotaje — **verificado corriéndolo** |
|---|---|---|
| **F1** | `TestElPedidoQuedaAuditadoAunqueNadieLoEjecute` | encolar recién al terminar → ✅ falla |
| **F2** | `TestLaPodaBorraLaSalidaYConservaLaBitacora` | podar con DELETE → ✅ falla |
| **F3** | `TestUnaMaquinaNoPuedeEscribirLaBitacoraDeOtra` | quitar la comparación de `device_id` → ✅ falla |
| **F4** | `TestSinCapacidadExecNoSeEncolaNada`, `TestLaCapacidadExecEsPorMaquina` | quitar `PuedeSobreDevice` → ✅ fallan |
| **F5** | `TestUnComandoSoloLlegaASuMaquina` | cola sin filtrar por máquina → ✅ falla |
| **F6** | `TestUnDeviceRevocadoNoRecibeComandos` | entregar antes de resolver el token |
| **F7** | `TestNoHayShellImplicito` | pasar el argv por `sh -c` → ✅ falla |
| **F8** | `TestElTimeoutMataElComando` | `exec.Command` en vez de `CommandContext` |
| **F9** | `TestLaSalidaSeAcotaEnElAgente...`, `...TambienEnElCerebro` | buffer sin tope → ✅ falla |
| **F10** | `TestUnComandoViejoVenceYNoSeEntrega` | no vencer los viejos → ✅ falla |
| — | `TestDosLatidosNoSeLlevanElMismoComando` | leer y marcar en dos pasos |
| — | `TestElRechazoDeExecNoRevelaSiLaMaquinaExiste` | distinguir «no existe» de «no podés» |
| — | `TestUnExitDistintoDeCeroEsResultadoNoError` | tratar exit≠0 como fallo del canal |
| — | `TestElBufferAcotadoNoRompeAlProcesoHijo` | reportar escrituras cortas → EPIPE |
| — | `TestElComandoNoHeredaStdin` | heredar stdin → un `cat` bloquea hasta el timeout |
| — | `TestLasDosRutasSeDerivanDeLaBase`, `TestElAgenteUsaLaRutaCorrecta...` | concatenar sobre una base que ya tiene ruta → ✅ fallan |
| — | `TestLaEsperaDeExecNoSuperaAlTransporte`, `...MaquinaCaidaVuelveEnseguida` | subir el tope de espera |

## 🔴 Dos defectos que sólo aparecieron corriendo procesos de verdad

**1 · El agente construía `/fleet/heartbeat/fleet/result` → 404 en cada resultado.** Le pasaba la
URL del latido como «base». **Los unitarios no podían agarrarlo**: apuntaban a un `httptest` que
responde a *cualquier* ruta. Contra un cerebro real, cada comando se ejecutaba y su resultado se
perdía — la bitácora quedaba llena de `entregado` sin terminar.

Arreglado derivando las dos rutas de la base (y tolerando que alguien setee `MUSUBI_BRAIN_URL` con
la ruta ya pegada). La prueba nueva usa un servidor que **exige** las rutas correctas, que es lo
que el permisivo no verificaba.

**2 · La espera de `exec` superaba el deadline del transporte.** Lo destapó el *cronómetro* de un
sabotaje: sin la compuerta, un exec sobre una máquina inexistente tardó **90 s** — timeout del
comando (30) más dos márgenes (60) — cuando el transporte corta a los **60**. El caller no habría
recibido la nota honesta «sigue corriendo»: habría recibido un timeout de HTTP, que se lee como
«el cerebro no anda» cuando todo funciona.

Ahora la espera se topa en 45 s, una máquina que no late **vuelve enseguida** diciéndolo, y un
comando con timeout largo se encola en vez de bloquear.

## Verificado end to end, con dos procesos y comandos de verdad

```
1. uname -sr            -> exit 0 · "Linux 7.0.0-30-generic\n"
2. sh -c 'exit 47'      -> exit 47 · stderr capturado · error de canal VACÍO
                           (un comando que falla es un RESULTADO, no una máquina rota)
3. echo '$HOME y *'     -> "$HOME y *"   literal, sin expandir: no hay shell implícito
4. sleep 30 (timeout 2) -> exit null · "excedió su timeout de 2s y fue terminado"

5. `miron` (ADMIN, con metrics, SIN exec) -> RECHAZADO
6. su bitácora           -> 0 entradas visibles · sin_permiso: 6
7. la bitácora de gio    -> los 5 comandos con quién, qué, exit y estado
8. KILL-SWITCH: comando encolado + revocar + el agente vuelve
     -> "credencial inválida o revocada"; el archivo NO se creó
```

El punto 5-6 es lo que separa este track de un RAT: **un admin de la memoria no ejecuta nada, y
ni siquiera ve qué se ejecutó.** El 8 es el kill-switch cortando la cola.

## Lo que queda fuera

- **Shell interactiva (PTY)** — es bidireccional y con streaming, otro problema. **S5b**, anotado
  en `ABIERTO.md`. El one-shot cubre lo que se hace el 80 % de las veces.
- **Allowlist de comandos por máquina** — es política, va con **S10**.
- ~~**Exec en Tier B/C**~~ — **HECHO** en **S7** (SSH) y **S8** (ADB).
- ~~**La poda automática de salidas**~~ **HECHA en S10 (A11)**: `podarSalidasSiToca` cuelga del
  latido propio de la flota, como mucho una vez por hora.
- **Cero dependencias nuevas.**
