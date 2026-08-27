# Spec — S4b · El export a Prometheus, y los cabos de la Fase 1

Cierra los cabos que S2 y S4 dejaron declarados. Contrato observable; cada invariante tiene una
prueba que **sabe fallar**.

```
GET /metrics   ->  además de las métricas del servidor, musubi_fleet_device_* de las máquinas
                   que ESA credencial puede ver.
```

---

## La tensión que este slice resuelve

S4 dejó a Musubi guardando el **presente** de la flota y la historia explícitamente afuera:
graficar series es para lo que existe Prometheus, que este repo ya despliega. Pero exportar
chocaba con S3: **un scrape presenta UNA credencial, y la compuerta de la flota es por máquina y
por capacidad.**

La salida fácil habría sido *«/metrics es infraestructura, que vea todo»*. Eso convierte al
scraper en una puerta trasera que sortea el eje entero: bastaría con darle su token a alguien
para leer la telemetría de todos los tenants.

**La resolución es que el scraper no es un caso especial: es un principal más.** Se exporta
exactamente lo que esa credencial puede ver, con la misma `PuedeSobreDevice` que usa la tool.

---

## H1 · Lo desconocido no se exporta

### E1 — Un valor que no se midió NO se exporta como 0: no se exporta

En Prometheus importa más que en el JSON. Una serie ausente se dibuja como un hueco y `absent()`
la puede alertar; un 0 entra al gráfico como una medición real. Un `cpu_percent 0` en el primer
latido de cada agente pintaría **una caída a cero en cada reinicio**, y esas caídas fantasma son
exactamente lo que hace que alguien deje de mirar un dashboard.

Si ninguna máquina tiene el valor, tampoco se emiten `HELP` y `TYPE`: cabeceras sin series es ruido.

### E2 — Una máquina que nunca reportó aporta `up` y nada más

No se inventan series de una muestra que no existe.

---

## H2 · El scrape pasa por la misma compuerta que todo lo demás

### E3 — Se exporta sólo lo que ESA credencial puede ver

Mismo `PuedeSobreDevice` que la tool: tenencia ∧ concesión ∧ aparato.

### E4 — Un admin sin concesiones no exporta nada, **y la salida lo dice**

C1 llega hasta el scrape. Un bloque vacío y mudo manda a alguien a depurar Prometheus cuando el
problema está en `principals.yaml`; se emite un comentario (que el parser ignora) explicando qué
falta.

### E5 — El scrape no cruza tenants

Un principal acotado exporta lo suyo aunque tenga el comodín de capacidades.

---

## H3 · El formato no se puede corromper

### E6 — Un nombre con comillas no parte la línea

Los nombres de máquina los escribe un administrador. Un device llamado `a"b` corrompería **todo
el scrape**, no sólo esa serie. Se escapa según el exposition format.

### E7 — Los bytes grandes salen enteros, no en notación científica

Prometheus acepta `5.024e+11`, pero el humano que depura un dump, no.

### E8 — La cardinalidad está acotada

Labels: `device`, `project`, `tier`, `os`. Las **tags** quedan afuera: son texto libre del
administrador y meterlas es la forma clásica de voltear un Prometheus.

---

## H4 · Cabos cerrados de S2/S4

### E9 — El agente reporta lo que sabe de sí mismo (versión y dirección)

El inventario mostraba `address` y `agent_version` vacíos desde S1. **No afloja B4/D5:** el
invariante es que el device no puede decir *quién es* —eso sale del token—, no que no pueda decir
*cómo está*. La fila que toca es la del token presentado y ninguna otra.

Va **antes** del corte por «no vino muestra»: un agente en un OS sin colector manda sólo su
versión, y era justo la máquina de la que menos se sabe la que se quedaba anónima.

La dirección prefiere el **tailnet** (100.64.0.0/10): la IP de la LAN de una oficina no le sirve
al cerebro para alcanzar nada. Es informativa — si se usara para autenticar, un device podría
mentir.

### E10 — La guarda del conteo de tools cubre los DOS idiomas

`README.en.md` llegó a decir **27 tools cuando había 66**, con una tabla parada en 8 dominios de
16. La deriva pasó porque la guarda miraba un solo archivo. Se regeneró la tabla y la guarda ahora
cubre los dos: **una guarda que cubre un solo idioma enseña que el otro no importa.**

---

## H5 · Lo que este slice NO hace

- **No mete Alertmanager.** Las reglas se evalúan y se ven en la UI de Prometheus; notificar
  necesita Alertmanager, y eso ya estaba anotado como pendiente en `prometheus.yml`.
- **No exporta métricas por proceso ni por interfaz.** Sigue siendo el agregado del host.
- **Cero dependencias nuevas.** El exposition format se escribe a mano, como ya hacía
  `observability.go`.
