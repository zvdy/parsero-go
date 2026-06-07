package diff

import (
	"reflect"
	"testing"
)

func TestComputeNewlyReachable(t *testing.T) {
	prev := []Probe{
		{URL: "http://x/admin", StatusCode: 403},
		{URL: "http://x/open", StatusCode: 200},
	}
	cur := []Probe{
		{URL: "http://x/admin", StatusCode: 200}, // became reachable
		{URL: "http://x/open", StatusCode: 200},
		{URL: "http://x/new", StatusCode: 200}, // newly present and reachable
	}
	got := Compute(prev, cur)
	want := []string{"http://x/admin", "http://x/new"}
	if !reflect.DeepEqual(got.NewlyReachable, want) {
		t.Errorf("NewlyReachable = %v, want %v", got.NewlyReachable, want)
	}
	if len(got.NoLongerReachable) != 0 {
		t.Errorf("NoLongerReachable = %v, want empty", got.NoLongerReachable)
	}
	if !got.HasChanges() {
		t.Error("HasChanges() = false, want true")
	}
}

func TestComputeNoLongerReachable(t *testing.T) {
	prev := []Probe{{URL: "http://x/secret", StatusCode: 200}}
	cur := []Probe{{URL: "http://x/secret", StatusCode: 404}}
	got := Compute(prev, cur)
	if len(got.NewlyReachable) != 0 {
		t.Errorf("NewlyReachable = %v, want empty", got.NewlyReachable)
	}
	want := []string{"http://x/secret"}
	if !reflect.DeepEqual(got.NoLongerReachable, want) {
		t.Errorf("NoLongerReachable = %v, want %v", got.NoLongerReachable, want)
	}
}

func TestComputeNoChange(t *testing.T) {
	probes := []Probe{
		{URL: "http://x/a", StatusCode: 200},
		{URL: "http://x/b", StatusCode: 403},
	}
	got := Compute(probes, probes)
	if got.HasChanges() {
		t.Errorf("expected no changes, got %+v", got)
	}
}
