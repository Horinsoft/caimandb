# Drivers de CaimanDB

Clientes ligeros para conectarse a CaimanDB desde distintos lenguajes,
todos sin dependencias externas (solo librería estándar de cada
lenguaje, salvo PHP que requiere la extensión `curl`, muy común en
cualquier instalación).

| Carpeta | Lenguaje | Requiere |
|---|---|---|
| `js/` | JavaScript / Node.js | Node 18+ (usa `fetch` nativo) |
| `go/` | Go | Go 1.21+ |
| `python/` | Python | Python 3.8+ |
| `php/` | PHP | PHP 7.4+, ext-curl |
| `java/` | Java | Java 11+ |

## Qué hacen

Los cinco hablan el mismo protocolo — el de CaimanDB documentado en
`docs/api/http-api.md` del repo principal — y exponen la misma forma
de API en cada lenguaje:

- `login(usuario, contraseña)` — autentica contra el servidor admin y guarda el JWT
- `query(nql, db)` — ejecuta cualquier comando NQL crudo (`FIND`, `INSERT`, `UPDATE`, `JOIN`, transacciones, lo que sea)
- Wrappers de conveniencia: `insert`, `get`, `find`, `search`, `update`, `delete`, `count`
- `health()`, `status()`
- `watch(...)` — se suscribe al change stream en tiempo real (Server-Sent Events)

Todos soportan tanto Basic Auth (usuario/contraseña) como JWT Bearer
token (vía `login()` o pasando el token directamente).

## Qué NO hacen (todavía)

- No incluyen un query builder tipado por lenguaje — las cláusulas
  `WHERE`/`SET`/`ORDER` se pasan como texto NQL crudo. Es la opción
  más simple y la que menos supuestos hace sobre la sintaxis exacta
  que soporta tu versión de CaimanDB; armar un DSL por lenguaje encima
  de esto es un buen próximo paso si lo necesitás.
- No manejan pooling de conexiones ni retries — son clientes finos,
  pensados como punto de partida.

## Verificación

Este sandbox no tenía compiladores de todos los lenguajes disponibles.
Estado real de cada uno:

- **JavaScript**: sintaxis verificada con `node --check`.
- **Python**: sintaxis y un smoke test (import + instanciación) verificados con `python3`.
- **Java**: compilado y corrido de verdad (incluyendo el armado de
  comandos NQL de `find`/`insert`/`update` contra un puerto muerto,
  para confirmar que la lógica llega intacta hasta la capa de red).
- **Go**: sin toolchain de Go disponible para compilar — escrito con
  cuidado y revisado a mano, pero corré `go build ./...` antes de
  confiar en él y avisame si sale algún error.
- **PHP**: sin intérprete de PHP disponible — mismo caso que Go, corré
  `php -l` antes de confiar en él.

Si algo no compila tal cual está, pegame el error y lo corrijo.
