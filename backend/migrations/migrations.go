// Package migrations embeds the SQL migration files so the API binary can apply them
// at startup without shipping the .sql files alongside it.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
