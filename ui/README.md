# CaimanDB Studio (ui/)

Consola web de administración de CaimanDB, servida directamente por el
propio nodo (`internal/caimandb/http_query.go` sirve este directorio en
`/` cuando `config.UIDir` apunta aquí — por defecto ya lo hace).

Es HTML/CSS/JS puro: sin build step, sin npm, sin framework. Las
únicas dependencias externas son CDNs (Tabler Icons, Google Fonts,
CodeMirror y Chart.js), cargadas desde `index.html`.

## Qué hace cada archivo

- `index.html` — estructura de la app: pantalla de conexión, rail de
  iconos (Query / Dashboard), barra lateral de entidades, editor de
  consultas con pestañas, panel de resultados y panel de dashboard.
- `style.css` — tema oscuro. Toda variable visual vive en `:root`.
- `app.js` — toda la lógica. **No hay datos de ejemplo ni
  `Math.random` en ningún sitio**: cada número, fila o gráfico sale de
  una petición HTTP real contra el nodo:
  - `GET /entities?db=` — bases de datos y bloques reales, con conteo
    de documentos, para la barra lateral y el gráfico de "Documentos
    por bloque".
  - `POST /query` — ejecuta NQL de verdad. Además del texto de
    resultado (que produce cualquier comando), para `FIND`/`GET`
    también devuelve `rows`/`columns` estructurados — una grilla real,
    no una tabla ASCII parseada.
  - `GET /status` — métricas reales del proceso (bases de datos,
    shards, operaciones, caché L1, métricas de consultas). El panel
    "Dashboard" sondea este endpoint cada 5s y construye su propio
    historial de ops/s y latencia a partir de los deltas reales entre
    muestreos — no simula series de tiempo.
  - `GET /watch` — Server-Sent Events con los cambios reales
    (insert/update/delete) del nodo, mostrados en "Eventos en vivo".

## Autenticación

HTTP Basic Auth con las credenciales que se introducen en el
formulario de conexión — el mismo usuario/rol creado con
`CREATE USER` dentro de CaimanDB. La contraseña solo vive en memoria
del script (una variable JS); nunca se escribe en localStorage,
sessionStorage ni cookies, y desaparece al recargar o desconectar.

## Desarrollo local

No requiere build. Basta con levantar el nodo (`make run` o
`./bin/caimandb`) y abrir `http://localhost:<query_port>/` — el propio
servidor de consultas sirve este directorio en la raíz, mismo origen
que la API, sin problemas de CORS.
