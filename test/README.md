# Tests

El proyecto original no incluía pruebas automatizadas, así que aquí
no se han inventado tests que aparenten una cobertura inexistente.
Esta carpeta deja la estructura lista para empezar:

- `test/integration/` — pruebas de extremo a extremo contra un
  binario de CaimanDB real (levantar el motor, hablarle por NQL o HTTP,
  comprobar resultados). Candidatas naturales: ciclo de vida de una
  base de datos/bloque, INSERT+FIND, una transacción BEGIN/COMMIT y
  un ROLLBACK.
- `test/fixtures/` — datos de ejemplo (JSON/CSV) para los comandos
  `INSERT ... FROM "file.json"` e `IMPORT`.

Para pruebas unitarias de paquete, la convención de Go es colocar
`_test.go` junto al código que prueban dentro de
`internal/caimandb/`, no aquí.

Ejecuta `scripts/test.sh` una vez tengas Go instalado — corre
`go build`, `go vet` y `go test ./...`.
