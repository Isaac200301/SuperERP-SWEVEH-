// Package migrations embebe las migraciones SQL para que el binario de migración
// y las pruebas de integración apliquen exactamente los mismos archivos.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
