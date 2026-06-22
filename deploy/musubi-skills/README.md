# Cosechador central del catálogo de marketplace (Track 8 / T8.4)

Este directorio contiene el **GitHub Action** que mantiene fresco el catálogo estático de
Agent Skills que consume `musubi_discover_skills`. **No corre desde el repo de Musubi**: está
acá como artefacto listo para desplegar en el repo que hostea el catálogo.

## Cómo desplegarlo

1. Copiá [`harvest.yml`](harvest.yml) al repo **`musubi-skills`** en
   `.github/workflows/harvest.yml`.
2. En ese repo, agregá el secret **`SKILLSMP_API_KEY`** (Settings → Secrets and variables →
   Actions → New repository secret). Sacás la key gratis en https://skillsmp.com (sube el
   límite a 500/día; sin key corre en el tier anónimo de 50/día).
3. Listo. Corre solo cada lunes, o a mano desde la pestaña Actions ("Run workflow"). Genera
   `marketplace-index.json` en la raíz del repo y lo commitea si cambió.

## Cómo lo consumen los usuarios

Musubi lee ese JSON por `raw.githubusercontent` — el default de la config:

```yaml
sourcing:
  marketplace_enabled: true
  marketplace_catalog_url: https://raw.githubusercontent.com/codeabraham16/musubi-skills/main/marketplace-index.json
```

`musubi_discover_skills` sirve desde ese catálogo estático (**cero rate limit**) y solo cae
a la API en vivo si el catálogo todavía no existe o no está disponible. Mientras el archivo
no exista en `musubi-skills`, el comportamiento es el modo live (transición sin fricción).

## Ajustar la cosecha

Editá los flags en `harvest.yml` (`--seeds`, `--top`, `--min-stars`) o pasá `seeds` al
correr el workflow a mano. Recordá: el catálogo es un **subconjunto curado** por relevancia
(seeds = stacks) y popularidad (estrellas), no un mirror del 1.7M.
