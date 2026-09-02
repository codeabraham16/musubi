-- alertas.sql — recibe las alertas de Alertmanager en el CRM (Supabase/PostgREST).
--
-- ─────────────────────────────────────────────────────────────────────────────────────────────
-- POR QUÉ UNA FUNCIÓN Y NO UNA TABLA DIRECTA
--
-- Alertmanager postea un JSON con claves de PRIMER NIVEL: receiver, status, alerts, groupLabels…
-- PostgREST mapea esas claves a los ARGUMENTOS de una función, así que un `POST /rpc/recibir_alerta`
-- entra tal cual, sin adaptador en el medio. Contra una tabla no funcionaría: el cuerpo tendría que
-- tener la forma de una fila, y no la tiene.
--
-- Hay que declarar TODOS los campos que manda Alertmanager aunque no se usen. PostgREST rechaza el
-- cuerpo entero si trae una clave que no corresponde a ningún argumento — y el rechazo se vería
-- como "el CRM no recibe alertas" sin decir por qué.
--
-- ─────────────────────────────────────────────────────────────────────────────────────────────
-- SEGURIDAD
--
-- `security definer` para poder insertar sin darle permisos de tabla a `anon`. Se concede EXECUTE
-- sobre ESTA función y nada más: la clave anon que va a vivir en la config de Alertmanager sólo
-- puede llamar a esto. Es el mismo criterio que el principal `prometheus` de Musubi, que tiene
-- `metrics` y nada de `exec`.
--
-- `set search_path = public` no es decoración: sin eso, una función `security definer` es
-- vulnerable a que alguien anteponga un esquema propio y secuestre los nombres.
-- ─────────────────────────────────────────────────────────────────────────────────────────────

create table if not exists public.alertas (
  id           bigserial primary key,
  recibida_en  timestamptz not null default now(),
  estado       text not null,          -- firing | resolved
  nombre       text not null,          -- alertname
  severidad    text,
  device       text,                   -- la máquina, cuando la alerta es de flota
  proyecto     text,
  resumen      text,
  runbook      text,
  -- `nota` es la ACLARACIÓN en prosa; `runbook` es la referencia a una sección de
  -- deploy/RUNBOOK.md. Se separaron el 2026-09-02: hasta entonces las dos vivían en `runbook` y
  -- el aviso de Telegram las dibujaba igual, con una flecha que prometía un enlace y a veces era
  -- una nota. Acá la separación importa por lo mismo: quien consulta la vista tiene que poder
  -- distinguir «andá a leer esto» de «ojo con esto».
  nota         text,
  inicio       timestamptz,
  fin          timestamptz,
  -- La huella que calcula Alertmanager. Es lo que permite unir el `firing` con su `resolved`
  -- despues, en vez de adivinar por nombre + device.
  huella       text,
  crudo        jsonb not null          -- la alerta entera, por si algún día hace falta un campo
);

-- PARA UNA BASE QUE YA EXISTE: `create table if not exists` NO agrega columnas nuevas, así que
-- una instalación anterior se quedaría sin `nota` y el insert de abajo fallaría en cada alerta —
-- silenciosamente, del lado del webhook. El alter es aditivo y se puede correr las veces que sea.
alter table public.alertas add column if not exists nota text;

create index if not exists idx_alertas_recientes on public.alertas (recibida_en desc);
create index if not exists idx_alertas_device    on public.alertas (device, recibida_en desc)
  where device is not null;
-- Lo que se consulta de verdad: "qué está firing ahora".
create index if not exists idx_alertas_firing    on public.alertas (nombre, recibida_en desc)
  where estado = 'firing';

create or replace function public.recibir_alerta(
  receiver            text  default null,
  status              text  default null,
  alerts              jsonb default '[]'::jsonb,
  "groupLabels"       jsonb default null,
  "commonLabels"      jsonb default null,
  "commonAnnotations" jsonb default null,
  "externalURL"       text  default null,
  version             text  default null,
  "groupKey"          text  default null,
  "truncatedAlerts"   int   default null
) returns int
language plpgsql
security definer
set search_path = public
as $$
declare
  n int := 0;
  a jsonb;
begin
  for a in select * from jsonb_array_elements(coalesce(alerts, '[]'::jsonb)) loop
    insert into public.alertas
      (estado, nombre, severidad, device, proyecto, resumen, runbook, nota, inicio, fin, huella, crudo)
    values (
      coalesce(a->>'status', status, 'firing'),
      coalesce(a->'labels'->>'alertname', '(sin nombre)'),
      a->'labels'->>'severity',
      a->'labels'->>'device',
      a->'labels'->>'project',
      a->'annotations'->>'summary',
      a->'annotations'->>'runbook',
      a->'annotations'->>'nota',
      nullif(a->>'startsAt','')::timestamptz,
      -- Alertmanager manda un endsAt del año 0001 cuando la alerta sigue activa. Guardarlo como
      -- fecha real haría que cualquier consulta por rango la trajera de la nada.
      nullif(nullif(a->>'endsAt',''), '0001-01-01T00:00:00Z')::timestamptz,
      a->>'fingerprint',
      a
    );
    n := n + 1;
  end loop;
  return n;
end $$;

revoke all on function public.recibir_alerta from public;
grant execute on function public.recibir_alerta(
  text, text, jsonb, jsonb, jsonb, jsonb, text, text, text, int
) to anon, authenticated, service_role;

-- Lo que vas a mirar: qué está activo ahora, sin el ruido de los resueltos.
create or replace view public.alertas_activas as
select distinct on (huella)
       -- `nota` va AL FINAL y no junto a `resumen`, aunque ahí se leería mejor: `create or
       -- replace view` de Postgres sólo admite AGREGAR columnas al final. Meterla en el medio
       -- falla con «cannot change name of view column» en cualquier base que ya exista, que son
       -- todas las que importan.
       nombre, severidad, device, resumen, runbook, inicio, recibida_en, estado, nota
  from public.alertas
 where huella is not null
 order by huella, recibida_en desc;
