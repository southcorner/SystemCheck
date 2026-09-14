// Package migrations embeds the SQL migration files so they ship inside the
// server binary.
package migrations

import "embed"

// FS holds all *.sql migration files, applied in lexical order.
//
//go:embed *.sql
var FS embed.FS
