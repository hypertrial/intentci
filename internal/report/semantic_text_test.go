package report_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hypertrial/intentci/internal/report"
	"github.com/hypertrial/intentci/pkg/protocol"
)

func TestWriteText_SemanticSection(t *testing.T) {
	res := sampleResult(protocol.StatusPass)
	res.Semantic = &protocol.SemanticRun{Enabled: true, Provider: "local", Enforcement: "advisory", FindingCount: 2}
	var buf bytes.Buffer
	if err := report.WriteText(&buf, res); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "findings: 2") {
		t.Fatalf("%s", buf.String())
	}
	res.Semantic = &protocol.SemanticRun{Enabled: false, Skipped: "disabled"}
	buf.Reset()
	if err := report.WriteText(&buf, res); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "disabled") {
		t.Fatalf("%s", buf.String())
	}
	res.Semantic = &protocol.SemanticRun{Enabled: true, Skipped: "provider down"}
	buf.Reset()
	if err := report.WriteText(&buf, res); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "skipped: provider down") {
		t.Fatalf("%s", buf.String())
	}
	if err := report.ValidateResultSchema(res); err != nil {
		t.Fatal(err)
	}
}
