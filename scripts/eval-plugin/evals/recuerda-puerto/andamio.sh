# shellcheck shell=bash
# Cuerpo del andamio de recuerda-puerto. NO corre solo: correr.sh le antepone la cabecera y
# lib/andamio-comun.sh, que definen andamio_* (ver el README de la suite).
declare -F andamio_falla >/dev/null || {
	echo "andamio.sh no corre solo: lo arma correr.sh con lib/andamio-comun.sh" >&2
	exit 1
}

# El repo: la configuración de desarrollo. Deja TRES puertos a la vista (8080 del proxy, 3000 del
# frontend, 5432 de la base) y ninguno es el de staging: el que conteste uno de esos, adivinó.
mkdir -p frontend
cat >README.md <<'FIN'
# Tamarindo

Sistema de turnos de la cooperativa de panaderos.

## Entornos

- Desarrollo: `docker compose up` levanta todo (ver `docker-compose.yml`).
- Staging: lo administra el equipo. Sus datos no se versionan en este repositorio.
FIN
cat >docker-compose.yml <<'FIN'
services:
  proxy:
    image: nginx:1.27
    ports:
      - "8080:80"
  frontend:
    build: ./frontend
    ports:
      - "3000:3000"
  db:
    image: postgres:16
    environment:
      POSTGRES_DB: tamarindo
    ports:
      - "5432:5432"
FIN
cat >frontend/README.md <<'FIN'
# Frontend de Tamarindo

La pantalla de turnos que ven las panaderías.
FIN
andamio_git init -q .
andamio_commit "Tamarindo: configuración de desarrollo"

# La memoria: lo que el equipo ya sabía. El puerto de staging está SÓLO acá.
andamio_activar_memoria
andamio_rpc siembra <<'FIN'
{"name":"musubi_save_observation","arguments":{"topic_key":"project/profile","mem_type":"semantic","content":"Tamarindo es el sistema de turnos de una cooperativa de panaderos. Este repositorio tiene la configuración de desarrollo (docker-compose) y la documentación del equipo."}}
{"name":"musubi_save_observation","arguments":{"topic_key":"tamarindo/staging","mem_type":"semantic","content":"El entorno de staging de Tamarindo escucha en el puerto 48213, no en el 8080: el 8080 lo ocupa el proxy de la cooperativa. Lo decidió el equipo el 2026-03-02."}}
{"name":"musubi_save_observation","arguments":{"topic_key":"tamarindo/base-de-datos","mem_type":"semantic","content":"La base de datos de staging de Tamarindo es PostgreSQL 16 y escucha en el puerto 5432."}}
{"name":"musubi_save_observation","arguments":{"topic_key":"tamarindo/frontend","mem_type":"semantic","content":"El servidor de desarrollo del frontend de Tamarindo corre en el puerto 3000."}}
FIN

andamio_estado_proyecto_conocido

# El plugin, arrancado como en la corrida, tiene que encontrar el dato.
andamio_rpc plugin <<'FIN'
{"name":"musubi_recall","arguments":{"query":"puerto del entorno de staging de Tamarindo"}}
FIN
grep -q '48213' "${ANDAMIO_TMP}/salida.jsonl" ||
	andamio_falla "el recall del plugin no trae el puerto sembrado: el brazo con plugin no lo vería"

andamio_cerrar 48213
