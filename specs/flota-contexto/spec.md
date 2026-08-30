# Spec — S14 · El cruce: la flota le pregunta a la memoria

Fase 5, segundo slice y el **diferenciador**. S13 contesta qué **HIZO** Musubi en una máquina;
esto contesta qué **SABÍA**. Cruzarlas es lo único de todo el track que ningún panel del mercado
puede dar, porque ningún panel tiene al lado la memoria del equipo y su código.

```
musubi_fleet_contexto {"device":"altura-db","horas":24}
  ->  { terminos:[…], servicios_ocultos:…, actividad:{…},
        memoria:[{…, enlace:"termino"|"ventana"}], codigo:[…], no_visto:[…] }
```

---

## X1 · Correlación, no causa

### K1 — la tool NO adivina la causa, y eso es el diseño

Lo tentador es que conteste «anda lenta PORQUE el martes se desplegó X». Lo haría **adivinando**,
y una causa adivinada con aire de certeza es peor que no decir nada: manda a alguien a arreglar lo
que no está roto y, la segunda vez que acierta, se le empieza a creer.

Junta los hechos y **declara cómo los enlazó**. Quien concluye es quien lee.

### K2 — los dos enlaces no se mezclan, y el fuerte gana

`termino` = el texto **NOMBRA** la máquina o uno de sus servicios. Es evidencia (puede ser
homónima). `ventana` = **sólo** coincide en el tiempo y en el proyecto; no es evidencia de nada por
sí solo.

Presentados iguales, cualquier coincidencia temporal se lee como una pista. Cuando una nota entra
por las dos vías se conserva el enlace **fuerte**: al revés, un acierto de término quedaría
rotulado «coincidió en el tiempo» y perdería justo el peso que lo hace útil.

### K3 — `no_visto` viaja siempre

Cinco límites declarados, incluido que un `cuando` de código es cuándo Musubi **re-resumió** el
archivo, no la fecha de un commit. Musubi no lee el historial de git y decirlo es más barato que
que alguien lo suponga.

---

## X2 · Los términos

### K4 — salen del INVENTARIO, no del texto de los comandos

Sacar palabras del argv parece obvio: de `systemctl restart nginx` cae `nginx`. Sobre datos reales
produce basura —`cmd`, `type`, `/c`, una ruta de Windows— y una lista de términos con ruido hace
que la mitad de los enlaces no valgan, que es la forma más rápida de que alguien deje de mirar la
herramienta.

Los términos son el **nombre de la máquina** y sus **servicios**: pocos, exactos, explicables. Un
término que no sirve es un problema de inventario, no de heurística — y ése sí se arregla.

### K5 — el nombre de la máquina va primero y siempre

Es el término más preciso que existe: lo eligió una persona, es único en el proyecto y no depende
de que nadie haya declarado servicios. Si el tope recorta, recorta lo otro.

### K6 — se saca el sufijo de unidad, y sólo ése

Nadie escribe `nginx.service` en una nota. Se recortan `.service`, `.timer`, `.socket` y
`.target`; un servicio llamado `api.altura` conserva su nombre entero, porque ahí el punto es
parte del nombre. La limpieza es de cuatro sufijos conocidos, **no genérica**.

### K13 — el enlace por término busca la FRASE, no el OR de sus tokens

`buildFTSQuery` une los tokens con OR. Es correcto para el **recall** —lenguaje natural, cada
palabra suma señal— y es falso para un **enlace**: `avahi-daemon` se busca como `"avahi" OR
"daemon"`, y cualquier nota que diga «daemon» queda enlazada a un servicio que no menciona.

Medido en la primera corrida contra la flota real: una nota sobre decisiones de roadmap enlazada a
`avahi-daemon`. Eso no es un enlace flojo, es **evidencia inventada** — y no hay respaldo a OR si
la frase no encuentra nada, porque ese respaldo devolvería exactamente lo que esto elimina.

### K14 — los servicios DECLARADOS por una persona van primero

Un host enumera decenas de units y el tope se llena con las primeras. Medido: las doce ranuras se
gastaron en units del sistema y quedó afuera `alturito20`, el único servicio del que alguien
escribió algo alguna vez.

El criterio no es adivinar cuál importa: `Declarado` **ya significa** que una persona lo puso a
mano. Viene del inventario, no de una heurística sobre el nombre.

### K7 — LOS TÉRMINOS SON INFORMACIÓN Y SE COMPUERTAN

Decirle a alguien «busqué `postgres` en esta máquina» le está diciendo que ahí corre un postgres —
que es exactamente lo que `musubi_fleet_services` protege con `metrics`.

Sin esa capacidad no se arman términos de servicio, y la respuesta marca `servicios_ocultos: true`
en vez de devolver una lista corta que se leería como «esta máquina no corre nada».

---

## X3 · El aislamiento

### K8 — la memoria se lee en el proyecto de LA MÁQUINA

Un principal `read: all` alcanza la memoria de todos los tenants. Si la búsqueda usara **su**
alcance, el contexto de una máquina de `altura` traería notas de `crm`: no sería una fuga (puede
verlas por otra puerta) pero sí una **respuesta falsa**, y encima con el sello de una herramienta
que dice haber correlacionado.

El scope se fija al proyecto del device, que además siempre es igual o más angosto que lo que la
credencial ya podía ver.

### K9 — la Muralla 2 vale también por esta puerta

Una observación archivada, superseded o en cuarentena no puede aparecer. El predicado canónico
(`visibleObsPredicate`) lo cumplen nueve consultas de recall; ésta es la décima y **la primera que
llega desde fuera del recall**, que es por donde una muralla se rodea sin querer.

### K10 — la actividad se compuerta hecho por hecho, con la MISMA función que S13

`hechosVisiblesPara` está extraída y no copiada. Una compuerta duplicada es la peor duplicación
posible de este repo: la copia que se queda vieja es la del camino que se usa menos, y acá
«quedarse vieja» significa mostrarle a alguien un plano que no le corresponde.

---

## X4 · Las fechas, que es donde esto se rompe en silencio

### K11 — la memoria y la flota NO guardan el mismo formato

En la misma base conviven dos, y no es descuido de nadie: la flota escribe desde Go con RFC3339 y
la memoria deja que SQLite ponga `CURRENT_TIMESTAMP`.

```
observations.created_at  ->  2026-08-30 13:50:03
code_memory.updated_at   ->  2026-08-29 18:56:39
device_commands.creado   ->  2026-08-29T19:06:17Z
```

Comparar con el formato equivocado **no da error: da vacío**, y ese vacío se lee como «no había
nada escrito ese día».

### K12 — el driver convierte al LEER y no al COMPARAR

`modernc.org/sqlite` mira el tipo declarado de la columna: sobre un `DATETIME` devuelve RFC3339
aunque los bytes no lo estén. O sea que **mirar lo que vuelve en Go lleva a la conclusión
equivocada sobre cómo comparar**, y el error del otro lado —«corrijo el WHERE para que coincida»—
es justo el que no da error.

Se comparan los bytes con `formatoDeMemoria` y se aceptan **los dos** al parsear.

---

## Lo que este slice NO hace

- **No consulta Prometheus.** La serie sigue sin estar (**B5**) y se declara.
- **No lee git.** El `cuando` de un archivo es cuándo se re-resumió su gist.
- **No usa embeddings.** La búsqueda por término es FTS: determinista, explicable y sin costo de
  motor. Un contexto que dependiera de un embedder no se podría pedir desde un panel.
