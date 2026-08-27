# Spec — S3 · El tercer eje: qué puede pedirle una persona a qué máquina

Tercer slice del track **Control de flota**. Cierra la Fase 0. Depende de S1 (el registro) y S2
(el agente y las dos puertas).

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

```yaml
# principals.yaml — el eje NUEVO, opcional y fail-closed.
principals:
  - name: gio
    token_sha256: ...
    project_id: casa
    role: admin
    fleet:
      metrics: ["*"]              # toda máquina de su alcance
      exec:    ["pc-gio", "nas"]  # sólo esas dos
      # `screen` ausente ⇒ NINGUNA máquina. Ausencia = nada, nunca = todo.
```

```go
// internal/mcp/fleet_authz.go
func PuedeSobreDevice(p *Principal, d fleet.Device, c fleet.Cap) bool
func capsQuePuede(p *Principal, d fleet.Device) []fleet.Cap
func puedeOtorgar(p *Principal, c fleet.Cap) bool
```

---

## Por qué existe este slice, y por qué ANTES de exec

Musubi ya tiene dos ejes para las personas: **alcance** (`read: own|all`) y **autoridad**
(`write: none|own|any`). Los dos hablan de la MEMORIA.

Ninguno sabe decir *«esta persona puede mirar las métricas de las 40 máquinas, ejecutar en
tres, y en ninguna abrir la pantalla»*. Y colapsarlo en el rol es exactamente el puente de
privilegio que este track evita desde el proposal: **administrar la memoria del equipo no puede
convertirse, de rebote, en root sobre toda la flota.**

Va antes de S5 (exec) a propósito. Construir la ejecución remota primero y la autorización
después es cómo se despacha un exec sin guarda: entre una cosa y la otra siempre hay una
release. La compuerta se construye antes que aquello que compuerta.

---

## H1 · El eje es nuevo, y su ausencia significa NADA

### C1 — El rol de memoria NO otorga capacidades de flota

Un principal `admin`, `write=any`, `read=all` y **sin sección `fleet:`** no puede metrics, ni
exec, ni screen sobre ninguna máquina. Es el invariante que sostiene el track entero.

### C2 — La concesión es POR CAPACIDAD, no un booleano

Tener `metrics` sobre una máquina no da `exec` sobre ella. Mirar cómo está un servidor y poder
escribir en él son permisos de peso muy distinto, y el modelo no los deja colapsar.

### C3 — La concesión es POR MÁQUINA

`exec: ["pc-gio"]` no da exec sobre `nas`. El comodín `["*"]` existe y es explícito: hay que
escribirlo.

---

## H2 · El eje nuevo se INTERSECA con lo que ya había, nunca lo reemplaza

### C4 — La tenencia sigue mandando

Una concesión que nombra una máquina de otro proyecto no la alcanza. El grant no es una puerta
lateral a la tenencia: se aplica **después** de ella, nunca en su lugar.

### C5 — El dispositivo también tiene que poder

`PuedeSobreDevice` es la conjunción de dos lados: la persona tiene el grant **y** la máquina
sabe honrar la capacidad (su tier la admite, la tiene concedida, y no está revocada). Que
alguien tenga `screen: ["*"]` no hace que un router de Tier B tenga pantalla.

### C6 — Revocar la máquina gana sobre cualquier concesión

Un device revocado no admite nada de nadie, por más `["*"]` que tenga el principal. El
kill-switch de S2 no se puede sortear desde el otro eje.

---

## H3 · No se puede otorgar lo que no se tiene

### C7 — Enrolar no es una vía de auto-concesión

`musubi_fleet_enroll` **rechaza** conceder a la máquina nueva una capacidad que el principal no
tenga con `["*"]`.

Sin esto hay un escalamiento real y corto: alguien con `exec` sobre dos máquinas nombradas da de
alta una tercera con `exec`, y acaba de ampliarse su propio alcance sin que nadie lo autorice.
Se exige el comodín —y no «tenerla en cualquier máquina»— porque una máquina recién nacida no
está en ninguna lista: sólo quien ya la tendría igual puede otorgarla.

### C8 — `fleet_list` dice qué podés ejercer VOS

Cada fila lleva `puedo`: la intersección real entre lo que la máquina admite y lo que esta
credencial tiene concedido. Un inventario que muestra capacidades que quien mira no puede usar
enseña a ignorar el campo.

---

## H4 · La confianza local se conserva, pero declarada

### C9 — stdio local sigue teniendo acceso pleno

`principal == nil` (el daemon local, sin auth) conserva todas las capacidades sobre todas las
máquinas, igual que ya hace con la memoria en `canCall`, `isAdmin` y `writeOriginFor`.

Se prueba **explícitamente** para que sea una decisión visible y no un descuido heredado: es la
vía de arranque —alguien tiene que poder otorgar la primera capacidad— y es coherente con el
resto del código.

---

## H5 · Lo que este slice NO hace

- **No ejecuta ni captura nada.** Sigue sin haber exec (S5) ni pantalla (S6). Este slice
  construye la compuerta; lo que pase por ella llega después.
- **No inventa selectores por tag.** `["*"]` o nombres. Agrupar por tag es tentador y no hay
  todavía un caso real que lo pida; entra cuando lo haya, con su prueba.
- **No agrega tools de administración del eje.** Las concesiones se editan en
  `principals.yaml`, que ya recarga en caliente (≤10 s). Una tool para otorgarse capacidades a
  uno mismo por la red merece pensarse con más cuidado que un slice de fundación.
- **Cero dependencias nuevas.**
