# S6c — la pantalla que no tenía motor (A18, parte 1)

## El hueco, que estaba ACTIVO

`capsPorTier` le concede `screen` a Tier C, y está bien concedido: un Android **tiene**
framebuffer. Pero `methods_pantalla.go` sólo habla RustDesk. El camino entero de un móvil pasaba:

1. la autorización — la concesión es real;
2. «¿está en línea?» — la sonda le escribe `last_seen` igual que un latido propio;
3. la colisión de `rustdesk_id` — sin id no hay colisión posible;
4. y entonces **abría la sesión, acuñaba la contraseña, la mostraba la única vez que se muestra**,
   y encolaba `musubi:pantalla` en una cola que en Tier C **no drena nadie**: el agente es de Tier A.

El comando vencía a los 15 minutos y la bitácora quedaba diciendo que se abrió una pantalla.

## La distinción que faltaba

Son **dos preguntas distintas** y no había función que hiciera la segunda:

| pregunta | quién la responde |
|---|---|
| ¿este tier **sabe honrar** `screen`? | `capsPorTier` |
| ¿Musubi **tiene con qué** abrirla? | `fleet.MotorDePantalla` (nuevo) |

Tier A → `rustdesk`. Tier C → todavía nada: su motor es **scrcpy sobre ADB**, que es otro (A18 → S8b).
Un tier nuevo cae en el `default` y se niega, en vez de heredar un sí por descuido.

## Decisiones

- **Se niega, no avisa.** Abrir igual entregaría una contraseña de un solo uso para una sesión que
  nadie levanta.
- **La negativa va ANTES de «¿está latiendo?»**: que no haya motor es permanente, y culpar al
  silencio mandaría a alguien a depurar una sonda que anda bien.
- **Va ANTES de acuñar nada.** El daño de este hueco no era fallar: era *entregar*.
- **NO se toca la matriz.** Sacarle `screen` a Tier C haría fallar el alta de móviles ya enrolados
  y borraría la intención que S8b viene a cumplir. La concesión es correcta; el motor es lo que falta.
- **Se ve en el inventario y en el panel** (`pantalla_sin_motor`), no recién al fallar una apertura.
  Misma lección que `puede_actuar` (A23): una capacidad inerte y una viva no pueden compartir dibujo.
  En el panel la rama va **antes** del `—` de «sin id», porque un Tier C no tiene `rustdesk_id` y si
  no, se leería como «ya va a llegar» — y no va a llegar.

## Pruebas (5 sabotajes verificados)

| # | prueba | sabotaje que la tumba |
|---|---|---|
| AH | `TestUnAndroidNoAcunaContrasenaDePantalla` — y verifica que **no quedó sesión ni comando** | quitar la guarda |
| AI | `TestLaFaltaDeMotorSeDiceAntesQueElSilencio` | mover la guarda debajo de `EnLinea` |
| AJ | `TestElInventarioMarcaLaPantallaSinMotor` — marca el Tier C y **no** el Tier A | quitar el bloque del inventario |
| AK | `TestLaPaginaDeFlotaDistingueUnaPantallaSinMotor` (orden) | poner la rama después del `—` |
| AL | la misma, presencia | quitar la rama |

## Lo que queda fuera

El motor en sí (scrcpy sobre ADB) sigue siendo **A18 → S8b**. Esto no lo adelanta: lo vuelve honesto.
