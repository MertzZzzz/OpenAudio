// Package migrations embeds the SQL migrations (golang-migrate format) into the binary.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
