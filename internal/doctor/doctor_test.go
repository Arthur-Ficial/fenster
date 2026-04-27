package doctor

import (
	"strings"
	"testing"
)

func TestReport_HasOverallStatus(t *testing.T) {
	r := Run()
	if r.Status != StatusOK && r.Status != StatusDegraded && r.Status != StatusFailing {
		t.Fatalf("unknown status %q", r.Status)
	}
}

func TestReport_HasChromeCheck(t *testing.T) {
	r := Run()
	got := false
	for _, c := range r.Checks {
		if c.ID == CheckChrome {
			got = true
		}
	}
	if !got {
		t.Fatal("expected Chrome check")
	}
}

func TestReport_AlwaysCoversCoreCategories(t *testing.T) {
	r := Run()
	wanted := map[string]bool{
		CheckChrome: false, CheckGPU: false, CheckDisk: false,
		CheckOS: false, CheckProfile: false, CheckPromptAPI: false,
	}
	for _, c := range r.Checks {
		wanted[c.ID] = true
	}
	for k, ok := range wanted {
		if !ok {
			t.Errorf("missing check %s", k)
		}
	}
}

func TestReport_FailingCheckHasFix(t *testing.T) {
	r := Run()
	for _, c := range r.Checks {
		if c.Status == StatusFailing && c.Fix == "" {
			t.Errorf("failing check %s missing Fix", c.ID)
		}
	}
}

func TestRender_Plain(t *testing.T) {
	r := Run()
	out := r.RenderPlain()
	if !strings.Contains(out, "fenster doctor") {
		t.Fatalf("expected header in plain render, got %q", out[:min(80, len(out))])
	}
}

func TestRender_JSON(t *testing.T) {
	r := Run()
	out := r.RenderJSON()
	if !strings.Contains(out, `"status"`) || !strings.Contains(out, `"checks"`) {
		t.Fatalf("expected JSON status+checks, got %q", out[:min(120, len(out))])
	}
}
