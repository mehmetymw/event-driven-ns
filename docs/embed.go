package docs

import "embed"

//go:embed openapi.yaml swagger.html
var Static embed.FS
