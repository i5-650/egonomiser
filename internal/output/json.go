package output

import (
	"encoding/json"
	"io"

	"e-go-nomiser/internal/types"
)

// JSON writes the full report to w as indented JSON, so it can be consumed
// by scripts today and reused as an HTTP API response body later.
func JSON(w io.Writer, report types.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
