# La cadena de alertas, en Docker

## Por qué esto existía a medias

El cerebro expone `/metrics` desde S4b. Hasta hoy **nadie lo scrapeaba**: las 9 reglas de S4b, las
de políticas de S10 y el dead-man's switch entero estaban escritos, probados y **sin evaluar**.

No era «media alarma»: era ninguna. Y no lo vio el registro de abiertos porque ese archivo cubre
**código**, y esto era **despliegue** — la misma clase de hueco que persigue todo el track, sólo
que un piso más abajo.

## La cadena tiene cuatro eslabones

```
cerebro /metrics  →  Prometheus  →  Alertmanager  →  vos (y el watchdog externo)
      ✅ ya estaba      ① acá         ① acá            ② después
```

## ① Levantar Prometheus + Alertmanager

En **`musubi-server`**, con el repo clonado:

```sh
sudo deploy/docker/preparar.sh          # copia config, genera el token del scraper
# ↑ imprime un bloque `principals:` — pegalo en el principals.yaml del cerebro ANTES de seguir
cd deploy/docker && docker compose up -d
```

**El paso del `principals.yaml` no es opcional.** Las capacidades de flota **no se derivan del
rol**: un token admin sin concesiones explícitas no ve ni una máquina. Sin ese bloque verías las
métricas del servidor y **ninguna de la flota**, con las alertas de flota inertes y sin avisar.

### Verificarlo — no alcanza con que los contenedores estén `Up`

```sh
curl -s http://127.0.0.1:9099/api/v1/targets | grep -o '"health":"[a-z]*"'   # los dos `up`
curl -s http://127.0.0.1:9099/api/v1/rules | grep -c '"name"'                # las reglas cargadas
curl -s http://127.0.0.1:9099/api/v1/alertmanagers                           # Prometheus lo VE
curl -s 'http://127.0.0.1:9099/api/v1/query?query=musubi_fleet_device_up'    # ¿hay flota?
```

Esa última es la que importa: si vuelve vacía, el principal `prometheus` no tiene `fleet.metrics`.

### Puerto 9099, no 9090

**9090 es de Cockpit**, que viene instalado en muchos servidores Linux —`musubi-server` incluido—
y lo ocupa. El fallo peligroso no es que Prometheus no arranque: es que alguien abra el 9090, vea
una UI y **crea que Prometheus está andando**. Tres archivos tienen que coincidir en este puerto y
lo custodia `TestElPuertoDePrometheusEsElMismoEnTodosLados`.

### Loopback a propósito

Ninguno de los dos tiene autenticación, y la API de Alertmanager **puede silenciar alertas sin
credencial**: exponerla al tailnet sería darle a cualquiera en la red la capacidad de apagar la
vigilancia. Para las UIs desde otra máquina:

```sh
ssh -N -L 9099:127.0.0.1:9099 -L 9093:127.0.0.1:9093 usuario@musubi-server
```

## ② El watchdog externo (A22)

Recién **después** de que ① esté verificado. `MusubiSiempreViva` está siempre en firing a propósito;
su valor está en **dejar de llegar**. Necesita del otro lado un servicio que espere el ping y grite
si falta:

1. Creá un check en Healthchecks.io (o Dead Man's Snitch, o un cron en otra máquina).
2. **Period 5 min, grace 10 min** — tiene que coincidir con el `repeat_interval: 5m` de la ruta
   `watchdog` en `alertmanager.yml`. Un period más corto da falsas alarmas; uno más largo tarda
   más en avisar que la vigilancia murió.
3. `echo -n 'https://hc-ping.com/<uuid>' | sudo tee /etc/musubi/watchdog_url`
4. `sudo chmod 0400 /etc/musubi/watchdog_url`

Con `url_file`, Alertmanager relee el archivo en cada envío: **rotar la URL no pide reinicio.**

**Probalo apagando la vigilancia**: `docker compose stop alertmanager`, esperá el grace, y el
servicio externo tiene que gritar. Si no grita, el watchdog no está cubriendo nada — y ese es
exactamente el fallo que viene a detectar.

## Revertir

```sh
cd deploy/docker && docker compose down -v      # -v borra también la TSDB
sudo rm -rf /etc/musubi-prometheus
```

Y sacá el principal `prometheus` del `principals.yaml` del cerebro.
