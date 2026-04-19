package energy_test

import (
	"math"
	"testing"

	"github.com/yasserrmd/siqlah/internal/energy"
)

func TestBenchmarkEstimator_KnownModel(t *testing.T) {
	est := energy.NewBenchmarkEstimator()
	got, err := est.Estimate("gpt-4o", 1000, 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Source != energy.SourceModelBenchmark {
		t.Errorf("source = %q, want %q", got.Source, energy.SourceModelBenchmark)
	}
	// 1000*0.003 + 500*0.009 = 3.0 + 4.5 = 7.5 J
	want := 7.5
	if math.Abs(got.Joules-want) > 1e-9 {
		t.Errorf("joules = %v, want %v", got.Joules, want)
	}
}

func TestBenchmarkEstimator_UnknownModel(t *testing.T) {
	est := energy.NewBenchmarkEstimator()
	got, err := est.Estimate("unknown-model-xyz", 1000, 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Source != energy.SourceNone {
		t.Errorf("source = %q, want %q", got.Source, energy.SourceNone)
	}
	if got.Joules != 0 {
		t.Errorf("joules = %v, want 0", got.Joules)
	}
}

func TestBenchmarkEstimator_RegisterModel(t *testing.T) {
	est := energy.NewBenchmarkEstimator()
	if err := est.RegisterModel("custom/model", 0.001, 0.003); err != nil {
		t.Fatalf("register: %v", err)
	}
	got, err := est.Estimate("custom/model", 100, 100)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	want := 0.001*100 + 0.003*100
	if math.Abs(got.Joules-want) > 1e-9 {
		t.Errorf("joules = %v, want %v", got.Joules, want)
	}
}

func TestBenchmarkEstimator_RegisterModel_EmptyName(t *testing.T) {
	est := energy.NewBenchmarkEstimator()
	if err := est.RegisterModel("", 0.001, 0.003); err == nil {
		t.Error("expected error for empty model name, got nil")
	}
}

func TestCarbonEstimateKgCO2e(t *testing.T) {
	// 3_600_000 J = 1 kWh; 1 kWh * 1000 gCO2e/kWh / 1000 = 1 kgCO2e
	got := energy.CarbonEstimateKgCO2e(3_600_000, 1000)
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("CarbonEstimateKgCO2e(3600000, 1000) = %v, want 1.0", got)
	}
}

func TestStaticCarbonLookup_KnownRegion(t *testing.T) {
	cl := energy.NewStaticCarbonLookup()
	got, err := cl.Intensity("us-west-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Oregon — heavy hydro/wind
	if got != 130 {
		t.Errorf("us-west-2 intensity = %v, want 130", got)
	}
}

func TestStaticCarbonLookup_UnknownFallsBackToGlobal(t *testing.T) {
	cl := energy.NewStaticCarbonLookup()
	got, err := cl.Intensity("xx-nowhere-99")
	if err != nil {
		t.Fatalf("unexpected error for unknown region: %v", err)
	}
	// Must fall back to global average (436)
	if got != 436 {
		t.Errorf("unknown region fallback = %v, want 436", got)
	}
}

func TestStaticCarbonLookup_RegisterRegion(t *testing.T) {
	cl := energy.NewStaticCarbonLookup()
	cl.RegisterRegion("test-region", 99.9)
	got, err := cl.Intensity("test-region")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 99.9 {
		t.Errorf("got %v, want 99.9", got)
	}
}
