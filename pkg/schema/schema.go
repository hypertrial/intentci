package schema

import _ "embed"

//go:embed requirement.schema.json
var RequirementJSON []byte

//go:embed evidence.schema.json
var EvidenceJSON []byte

//go:embed verdict.schema.json
var VerdictJSON []byte

//go:embed repair.schema.json
var RepairJSON []byte

//go:embed ir.schema.json
var IRJSON []byte
