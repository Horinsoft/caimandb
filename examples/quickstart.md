# Guía rápida

## 1. Compilar y arrancar

```bash
go build ./cmd/caimandb
./caimandb
```

En el primer arranque, si no existe `configs/caimandb.conf`, CaimanDB
crea uno con valores por defecto (creando también la carpeta
`configs/` si hace falta). Puedes partir de
[`configs/caimandb.conf.example`](../configs/caimandb.conf.example) en su
lugar, copiándolo como `configs/caimandb.conf`.

## 2. Crear una base de datos y un bloque

```sql
CREATE DB shop
USE shop
CREATE BLOCK products
```

## 3. Insertar documentos

```sql
INSERT products {"name": "Teclado mecánico", "price": 45.99, "stock": 120}
INSERT products {"name": "Mouse inalámbrico", "price": 19.99, "stock": 300}
```

## 4. Consultar

```sql
FIND products WHERE price < 30
FIND products SELECT name, price ORDER price:DESC
COUNT products WHERE stock > 0
```

## 5. Transacción

```sql
BEGIN shop products
UPDATE products WHERE name = "Mouse inalámbrico" INC stock = -1
COMMIT
```

## 6. Vía HTTP

```bash
curl -u admin:change-me -X POST http://localhost:1555/query \
  -d 'FIND products WHERE price < 30'
```

Más comandos: [`docs/nql-reference.md`](../docs/nql-reference.md).
Más endpoints: [`docs/api/http-api.md`](../docs/api/http-api.md).
