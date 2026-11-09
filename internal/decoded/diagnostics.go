package decoded

import (
	"encoding/json"
	"errors"
	"os"
)

// ImportDiagnostic contains fixed phase codes and aggregate counts only.
// Imported values, paths, identifiers and raw errors must never be included.
type ImportDiagnostic struct {
	Stage  string
	Status string
	Code   string
	Counts map[string]int
}

type importDiagnostics struct {
	events []ImportDiagnostic
	stage  string
	counts map[string]int
	code   string
}

func (d *importDiagnostics) step(stage string) {
	if d.stage != "" {
		d.finish(nil)
	}
	d.stage, d.counts, d.code = stage, nil, ""
	d.events = append(d.events, ImportDiagnostic{Stage: stage, Status: "started"})
}

func (d *importDiagnostics) finish(err error) {
	if d.stage == "" {
		return
	}
	status, code := "complete", ""
	if err != nil {
		status, code = "failed", "import_structure_rejected"
		var syntax *json.SyntaxError
		var mismatch *json.UnmarshalTypeError
		var fileError *os.PathError
		switch {
		case errors.As(err, &syntax):
			code = "import_json_invalid"
			d.counts = map[string]int{"json_error_byte": int(syntax.Offset)}
		case errors.As(err, &mismatch):
			code = "import_json_type_invalid"
			d.counts = map[string]int{"json_error_byte": int(mismatch.Offset)}
		case errors.As(err, &fileError):
			code = "import_file_unreadable"
		}
		if d.code != "" {
			code = d.code
		}
	} else if d.counts["unresolved_links"] > 0 {
		status, code = "partial", "equipment_links"
	}
	d.events = append(d.events, ImportDiagnostic{Stage: d.stage, Status: status, Code: code, Counts: d.counts})
	d.stage = ""
}
