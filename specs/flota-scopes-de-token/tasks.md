# Tasks — S3 · El tercer eje

**Fase 0 del track «Control de flota»: CERRADA.** Suite entera verde (19 paquetes), vet limpio.

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | `Principal.Fleet` — el tercer eje, y su parseo fail-closed | `internal/mcp/principals.go` |
| T2 | La compuerta: `PuedeSobreDevice`, `capsQuePuede`, `puedeOtorgar` | `internal/mcp/fleet_authz.go` |
| T3 | C7 cableado: enrolar no es vía de auto-concesión | `internal/mcp/methods_fleet.go` |
| T4 | C8 cableado: el inventario dice qué podés ejercer **vos** | `internal/mcp/methods_fleet.go` |

## Invariantes, y qué los custodia

| Inv | Test | Sabotaje — **verificado corriéndolo** |
|---|---|---|
| **C1** | `TestElRolDeMemoriaNoOtorgaCapacidadesDeFlota` | `if p.isAdmin() { return true }` → ✅ falla (4 aserciones) |
| **C1** | `TestUnAdminSinGrantsEnrolaPeroNoConcede` | ídem → ✅ falla |
| C2 | `TestLaConcesionEsPorCapacidadYNoUnBooleano` | que `tieneGrant` ignore la capacidad |
| C3 | `TestLaConcesionEsPorMaquina` | no mirar el selector → ✅ falla |
| **C4** | `TestElGrantNoEsUnaPuertaLateralALaTenencia` | quitar `alcanzaElProyecto` → ✅ falla |
| C5 | `TestUnGrantNoLeDaPantallaAUnRouter` | quitar `d.Permite` → ✅ falla |
| C6 | `TestRevocarLaMaquinaGanaSobreElComodin` | ídem → ✅ falla |
| **C7** | `TestNoSePuedeOtorgarLoQueNoSeTiene` | aceptar cualquier selector, no sólo `*` → ✅ falla |
| C8 | `TestElInventarioDiceQuePuedeQuienMira` | devolver `caps` también en `puedo` |
| C9 | `TestStdioLocalConservaAccesoPleno` | — (decisión declarada, no accidente) |
| — | `TestElYamlDeGrantsEsFailClosed` (4 subtests) | ignorar claves desconocidas en `parsearFleet` |

## La asimetría que es todo el diseño

**La ausencia significa NADA; nunca «todas».** Un principal `admin` + `read=all` + `write=any` y
sin sección `fleet:` no puede metrics, ni exec, ni screen sobre ninguna máquina. El rol de la
memoria y el poder sobre las máquinas son ejes distintos, y colapsarlos es el puente de
privilegio que el track evita desde el proposal.

El sabotaje que lo prueba es literalmente la simplificación que alguien va a proponer —
`if p.isAdmin() { return true }` — y rompe cuatro aserciones de golpe.

**La compuerta se construyó antes que lo que compuerta.** Exec es S5 y pantalla S6. Al revés,
entre la ejecución remota y su autorización siempre hay una release de por medio.

## C7 · El escalamiento corto que se cierra

Sin esta guarda: alguien con `exec: ["pc-gio", "nas"]` da de alta una **tercera** máquina con
`exec` y acaba de ampliarse el alcance sin que nadie lo autorice. Se exige el **comodín** —y no
«tenerla en alguna máquina»— porque una máquina recién nacida no figura en ninguna lista de
nombres: el único criterio honesto es *«¿la tendrías igual?»*, y sólo `["*"]` responde que sí.

## Cambio de comportamiento respecto de S2 (verificado, y correcto)

El token **legacy** resuelve a `admin` federado y **no tiene grants de flota**. Desde S3, por la
red ya no puede conceder capacidades. Medido contra procesos reales:

```
admin legacy → enroll con caps:["exec"]  -> RECHAZADO ✓
   no podés conceder "exec": tu credencial no tiene esa capacidad con el comodín ["*"]
admin legacy → enroll SIN caps           -> ALTA OK ✓  caps: []
```

Es fail-closed y es lo que se quiere: **administrar el inventario** y **tener poder sobre las
máquinas** son cosas distintas, y el legacy sólo tiene lo primero. La vía de arranque sigue
siendo el stdio local (C9), que es quien otorga la primera capacidad.

## Verificado end to end, contra procesos reales

`principals.yaml` con dos identidades y el eje declarado — un jefe (`metrics`+`exec` con
comodín) y un observador (`metrics`):

```
1. jefe → enroll pc-gio caps:[metrics,exec]   -> caps concedidas: [metrics exec]
2. jefe → enroll con caps:[screen]            -> RECHAZADO (no tiene screen)
3. agent --once                                -> ✓ latido registrado
4. EL MISMO INVENTARIO, DOS CREDENCIALES:
     JEFE        pc-gio  admite=[metrics exec]  puedo=[metrics exec]  online=True
     OBSERVADOR  pc-gio  admite=[metrics exec]  puedo=[metrics]       online=True
```

El punto 4 es C8 funcionando: `admite` es propiedad de la **máquina**, `puedo` es propiedad de
**quien mira**. Un inventario que sólo mostrara lo primero enseña a ignorar el campo.

## Lo que queda fuera, a propósito

- **Selectores por tag** (**B2**). Sólo `["*"]` o nombres. Agrupar por tag es tentador y todavía no hay
  un caso real que lo pida.
- **Tools para administrar el eje por la red** (**B3**). Las concesiones se editan en `principals.yaml`
  (que ya recarga en caliente, ≤10 s). Una tool para otorgarse capacidades a uno mismo por la
  red merece más cuidado que un slice de fundación.
- **Cero dependencias nuevas.**

## Estado del track

**Fase 0 COMPLETA** — S1 (registro) · S2 (agente y las dos puertas) · S3 (el tercer eje).

Lo que hay ahora: máquinas registradas con identidad no falsificable, un agente que late y se
apaga solo al ser revocado, dos puertas que no se cruzan, y una compuerta de tres lados
(tenencia ∧ concesión ∧ aparato) lista para que exec y pantalla pasen por ella.

**Siguiente: Fase 1**, los tres planos en paralelo — S4 telemetría del host · S5 exec/PTY ·
S6 RustDesk self-hosted. S4 es el más directo: el latido pasa de decir «estoy viva» a «cómo
estoy», y ya tiene la capacidad `metrics` esperándolo.
