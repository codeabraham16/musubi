# Tasks — S7 · Tier B por protocolo

Abre la Fase 2. Suite entera verde (2 pasadas), vet limpio, cross-compila.

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | Ejecución por SSH invocando al cliente del sistema | `internal/fleet/remoto.go` |
| T2 | Citado POSIX de cada argumento para la shell remota | `internal/fleet/remoto.go` |
| T3 | Traducción de los fallos de `ssh` a algo accionable | `internal/fleet/remoto.go` |
| T4 | Ruteo por tier en `musubi_fleet_exec` — misma tool, misma compuerta, misma bitácora | `internal/mcp/methods_exec.go` |
| T5 | `last_seen` de un Tier B lo estampa el cerebro, **sólo si llegó** | `internal/mcp/methods_exec.go` |

## La decisión: Musubi NO guarda credenciales de Tier B

La respuesta fácil era una tabla de llaves SSH — y es exactamente lo que S6 se negó a construir
para las contraseñas de pantalla: un volcado de esa tabla es la flota entera, con permiso de
escritura.

**Se invoca al `ssh` del sistema.** La credencial nunca entra a Musubi, ni el secreto ni una
referencia a él: las llaves las administra quien opera con `ssh-agent` y `~/.ssh/config`, y se
hereda gratis todo lo que ya tiene (jump hosts, certificados, claves por host, `known_hosts`).

El costo se declara: **el cerebro necesita `ssh` instalado** y hay un `fork+exec` por comando.
Para decenas de máquinas no es nada; si algún día son miles, se revisa.

## Invariantes

| Test | Sabotaje — **verificado corriéndolo** |
|---|---|
| `TestNuncaSeDesactivaLaVerificacionDeHostKey` | `StrictHostKeyChecking=no` → ✅ falla |
| `TestElArgvSeCitaParaLaShellRemota` | pasar el argv sin citar → ✅ falla |
| `TestElCitadoNeutralizaLaShell` (9 casos) | — |
| `TestEl255DeSSHEsFalloDeCanalNoResultado` | tratar el 255 como exit code → ✅ falla |
| `TestElErrorDeHostKeyMandaALaSolucionBuenaYNoALaMala` | devolver el stderr crudo → ✅ falla |
| `TestUnTierBSeEjecutaPorSSHYNoSeEncola` | no rutear por tier → ✅ falla |
| `TestUnTierBInalcanzableNoFiguraVivo` | estampar el latido sin mirar el error → ✅ falla |
| `TestElTimeoutCortaLaConexionRemota`, `TestLaSalidaRemotaSeAcota` | — |

## 🔴 Un bug REAL de S5 que este slice destapó

El test del timeout remoto falló midiendo **30 s con un timeout de 1 s**. No era el test:
`CommandContext` mata al proceso al vencer, pero **`Run` vuelve cuando se cierran las TUBERÍAS**,
y un comando que dejó un hijo en background se las lleva abiertas.

**El ejecutor del AGENTE tenía el mismo patrón, y ahí es peor**: atiende los comandos de forma
secuencial, así que un `sh -c "algo &"` dejaba a la máquina sin atender nada más — latiendo pero
muda. Medido con la guarda quitada: **30 s con un timeout de 1 s**.

Arreglado con `cmd.WaitDelay` en los dos. La prueba de S5 no lo agarraba porque usaba `sleep 30`
como argv directo: ahí el proceso matado ES el que tiene las tuberías, y no hay huérfano.

## 🔴 Una guarda mía que confundió un cero real con un desconocido

`TestLaReglaDeLosParesSeRespetaEnEstaPlataforma` empezó a fallar por el **swap**: la regla decía
que un `total > 0` con `usado == 0` significa «me olvidé de medir». En memoria y disco es cierto
—en una máquina encendida ese cero es imposible—; en **swap es común y legítimo**: un swap vacío
es un swap sano.

La guarda que escribí para cazar *«un cero que significa no sé»* cayó ella misma en esa confusión.
Y lo destapó la máquina: la prueba pasó durante horas y empezó a fallar cuando el swap se liberó.
**Una prueba que depende del estado del entorno para tener razón, no la tenía.**

## Verificado con `ssh` de verdad

```
1. Tier B enrolado con address 192.0.2.77 (TEST-NET, inalcanzable)
2. exec  → transporte: ssh · "el comando excedió su timeout de 8s y se cortó la conexión"
3. la bitácora lo tiene igual, con quién lo pidió    ← F1 vale también en Tier B
4. fleet_list → online=False, nunca_latio=True       ← no figura vivo lo que no se alcanzó

5. contra 127.0.0.1 sin sshd, el ssh REAL:
     traducido: no se pudo alcanzar "127.0.0.1": ssh: connect to host 127.0.0.1 port 22: Connection refused
     crudo:     ssh: connect to host 127.0.0.1 port 22: Connection refused
6. `musubi:pantalla` por exec en Tier B → RECHAZADO   ← el escalamiento de S6 sigue cerrado
```

El mensaje que más importa es el de **host key desconocida**: es el fallo más común al enrolar un
Tier B, y el stderr crudo de `ssh` manda a la gente a buscar `StrictHostKeyChecking=no` en
internet. La traducción dice `ssh-keyscan` **y advierte explícitamente contra la solución mala**.

## Lo que queda fuera

- **SNMP, MQTT, Redfish** (**A17 → S7c**) — los tres piden una librería (dependencia) o un protocolo binario a
  mano. SSH cubre routers, NAS, Raspberry Pis y servers sin agente. → `ABIERTO.md`
- ~~**Métricas de Tier B** (`/proc` sobre SSH)~~ **HECHAS en S8**: el colector se refactorizó para
  tomar CONTENIDO, y ese parseo lo comparten las tres fuentes (local, SSH y ADB).
- ~~**Sondeo periódico** — hoy `last_seen` se estampa cuando alguien ejecuta.~~ **HECHO en S10
  (A19)**, junto con el umbral de «en línea» POR TIER, sin el cual no arreglaba nada. (Ojo: la sigla
  **S7b** quedó reasignada a «la shell contra un `sshd` real».)
- **Cero dependencias nuevas.**
