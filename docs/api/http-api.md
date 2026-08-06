# API HTTP

CaimanDB expone dos servidores HTTP independientes, cada uno en su
propio puerto configurable.

## Servidor de administración (`internal/caimandb/http_admin.go`)

Puerto por defecto: `admin_port` (`CAIMANDB_ADMIN_PORT`, 1556 si no se
configura).

| Ruta | Descripción |
|---|---|
| `GET /api/v1/health` | Health check |
| `GET /api/v1/status` | Estado del motor |
| `GET /api/v1/dbs` | Listar bases de datos |
| `/api/v1/db/...` | Operaciones sobre una base de datos concreta |
| `POST /api/v1/query` | Ejecutar una consulta NQL |
| `POST /api/v1/backup` | Backup de una base de datos |
| `POST /api/v1/restore` | Restore de una base de datos |
| `POST /api/v1/compact` | Compactación |
| `/api/v1/users` | Gestión de usuarios |
| `POST /api/v1/auth/login` | Login (devuelve JWT) |
| `POST /api/v1/auth/logout` | Logout |
| `/api/v1/token` | Gestión de tokens |
| `GET /api/v1/metrics` | Métricas Prometheus (`promhttp`) |

## Servidor de consultas (`internal/caimandb/http_query.go`)

Puerto por defecto: `query_port` (`CAIMANDB_QUERY_PORT`, 1555 si no se
configura).

| Ruta | Descripción |
|---|---|
| `POST /query` | Ejecutar una consulta NQL. Para `FIND`/`GET`, la respuesta JSON incluye además `rows`/`columns` con los documentos ya estructurados (no solo el texto de resultado) |
| `GET /status` | Estado del motor |
| `GET /health` | Health check |
| `GET /entities?db=` | Bases de datos y bloques reales (con conteo de documentos y tamaño) de la base indicada — usado por la barra lateral de `ui/` |
| `GET /watch?db=&block=` | Change stream en tiempo real (Server-Sent Events). `db` y `block` son filtros opcionales; sin ninguno, transmite todos los cambios del nodo. Ejemplo: `curl -N "http://host:puerto/watch?db=mydb&block=users"`. La conexión permanece abierta hasta que el cliente la cierra; se envía un evento `change` (JSON con `op`/`db`/`block`/`id`/`data`/`timestamp`) por cada insert/update/delete, y un comentario `: heartbeat` cada 15s. Un suscriptor lento nunca bloquea las escrituras: si su buffer se llena, se descartan sus eventos (contador `caimandb_change_events_dropped_total`). |

## Autenticación

Ambos servidores soportan JWT (`auth_jwt.go`) y Basic Auth como
alternativa. Las credenciales por defecto se configuran con
`query_user` / `query_password` (o `CAIMANDB_QUERY_USER`) y el secreto
JWT con `jwt_secret` (o `CAIMANDB_JWT_SECRET`).
