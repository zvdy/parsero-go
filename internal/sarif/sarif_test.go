package sarif

import (
	"encoding/json"
	"testing"

	"github.com/zvdy/parsero-go/internal/store"
)

func TestBuildOnlyIncludesReachable(t *testing.T) {
	rows := []store.ResultRow{
		{URL: "http://x/admin", StatusCode: 200},
		{URL: "http://x/private", StatusCode: 403},
		{URL: "http://x/open", StatusCode: 200},
	}
	rep := Build(store.Scan{Target: "x"}, rows)
	if len(rep.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(rep.Runs))
	}
	res := rep.Runs[0].Results
	if len(res) != 2 {
		t.Fatalf("expected 2 reachable results, got %d", len(res))
	}
}

func TestBuildSeverity(t *testing.T) {
	rows := []store.ResultRow{
		{URL: "http://x/admin", StatusCode: 200}, // sensitive -> error
		{URL: "http://x/stuff", StatusCode: 200}, // normal -> warning
	}
	rep := Build(store.Scan{Target: "x"}, rows)
	levels := map[string]string{}
	for _, r := range rep.Runs[0].Results {
		levels[r.Locations[0].PhysicalLocation.ArtifactLocation.URI] = r.Level
	}
	if levels["http://x/admin"] != "error" {
		t.Errorf("admin path level = %q, want error", levels["http://x/admin"])
	}
	if levels["http://x/stuff"] != "warning" {
		t.Errorf("normal path level = %q, want warning", levels["http://x/stuff"])
	}
}

func TestBuildMarshals(t *testing.T) {
	rep := Build(store.Scan{Target: "x"}, []store.ResultRow{{URL: "http://x/a", StatusCode: 200}})
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var generic map[string]any
	if err := json.Unmarshal(b, &generic); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if generic["version"] != "2.1.0" {
		t.Errorf("version = %v, want 2.1.0", generic["version"])
	}
	if _, ok := generic["$schema"]; !ok {
		t.Error("missing $schema")
	}
}
