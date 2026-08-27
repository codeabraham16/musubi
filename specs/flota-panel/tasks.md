# Tasks — S9 · El panel de flota

Suite entera verde, vet limpio. **El bundle WebGL no se tocó** (su job de CI compara bytes).

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | El proxy al cerebro, con estado explícito | `cmd/musubi/flota.go` |
| T2 | La página: HTML+CSS+JS plano, sin paso de build | `cmd/musubi/assets/flota.html` |
| T3 | Rutas `/flota` y `/api/flota` en el panel local | `cmd/musubi/dashboard.go` |

## Las tres decisiones de forma

**1 · No va dentro del bundle WebGL.** Ese bundle dibuja neuronas de memoria en 3D; esto es una
tabla de máquinas con números. Dos problemas de UI distintos. Y el bundle se commitea con sus
bytes comparados por la CI: tocarlo para agregar una tabla sería un riesgo gratuito.

Verificado: el único `<script>` de la página es **inline**, sin `src=`. Las tres menciones a
`three`/`bundle` en el archivo son los comentarios que explican por qué no se usan.

**2 · Lo sirve el panel LOCAL y proxea al cerebro** — el mismo patrón que el censo de actores y el
riel, por las mismas tres razones: el bearer no puede vivir en el DOM, el cerebro no publica CORS
para `127.0.0.1`, y el panel ya tiene la credencial.

**3 · El ESTADO viaja siempre.** Una flota vacía puede significar **cinco cosas distintas** y las
cinco se dibujan igual si lo único que viaja es la lista.

## Invariantes

| Test | Sabotaje — **verificado corriéndolo** |
|---|---|
| `TestUnaFlotaVaciaDistingueSusCincoCausas` | colapsar los estados en «lista vacía» → ✅ falla |
| `TestElTokenNoViajaAlNavegador` | mandarlo «para que refresque solo» → ✅ falla |
| `TestSiFallanLasMetricasIgualSeVeLaFlota` | propagar el error de métricas → ✅ falla |
| `TestUnaMaquinaSinMetricasSeDistingueDeUnaEnCero` | inventar ceros → ✅ falla |
| `TestElPanelPreguntaPorLasMismasToolsYNoInventaUnaRutaAparte` | un endpoint «para el panel» |
| `TestLaPaginaDeFlotaNoDependeDelBundleWebGL` | meter three.js |

## El invariante del track llega hasta el píxel

`null` se dibuja **`—`**, nunca `0 %`. Un agente recién arrancado, un Windows sin sensor, un macOS
sin CPU: los tres dibujan un guion.

Acá engaña más que en el JSON: **un gráfico no se lee, se mira de reojo.** Una barra en cero se
interpreta antes de pensar.

Tres cosas más que la tabla muestra a propósito:

- **`admite` y `puedo` por separado.** Lo primero es de la máquina, lo segundo de tu credencial.
- **La edad del dato al lado del dato.** Una máquina que late pero dejó de medir muestra su última
  muestra buena para siempre y **parece sana**; la edad delata ese caso (⚠ pasados 10 min).
- **Solo lectura.** No hay botón de ejecutar ni de abrir pantalla. Un panel que ejecuta es un panel
  que ejecuta **con un click de más**, y esos planos ya tienen su tool, su compuerta y su bitácora.

## 🔴 Un susto durante el e2e

La primera corrida apuntó el panel al **cerebro real** del usuario (`100.79.126.62:7717`) con un
token equivocado, porque asumí variables de entorno que no existen: el destino se pasa con
`--central` y `--token-env`, no con `MUSUBI_BRAIN_URL`.

Se cortó apenas apareció en el log (cinco 401 con backoff, sin efecto). Queda anotado porque la
lección es de método: **antes de correr algo que sale a la red, verificar a dónde apunta de
verdad** — el default de una herramienta bien configurada es la infraestructura de producción.

## Verificado end to end

```
/flota          → 7.258 bytes, un solo <script> inline, cero dependencias
/api/flota      → estado: vivo · cerebro: 127.0.0.1:7904
  nas-casa  B linux    online=True   cpu=11.9%  mem=49.9%  métricas=True
            admite=[metrics exec]  puedo=[metrics exec]
  router    B openwrt  online=False  cpu=—      mem=—      métricas=False
            admite=[metrics]       puedo=[metrics]

el token del cerebro en el JSON → ✓ no aparece
con el cerebro caído            → estado: "caido" + el motivo, NO una tabla vacía
```

`router` dibuja **`—`** y no `0 %`: nunca reportó. Ésa es la diferencia que este slice existe para
sostener.

## Lo que queda fuera

- **Acciones desde el panel** (fuera de alcance por diseño) — a propósito, ver arriba.
- **Gráficos históricos** (fuera de alcance por diseño) — eso es Grafana sobre el export de S4b, que ya existe.
- ~~**Un enlace desde el panel del cerebro** — habría que tocar el bundle.~~ **HECHO en S9b (A21), y
  el motivo de acá era FALSO**: la CI compara los bytes de `dashboard.bundle.js`, no los de la
  cáscara `dashboard.html`. El enlace se agregó sin tocar el bundle.
- **Cero dependencias nuevas**, ni de Go ni de JS.
