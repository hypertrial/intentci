// Package schema embeds JSON Schema definitions distributed with IntentCI.
package schema

import (
	_ "embed"
)

//go:embed contract.schema.json
var ContractJSON []byte

//go:embed result.schema.json
var ResultJSON []byte
