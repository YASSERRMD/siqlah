// Package energy provides energy-per-token estimation for AI inference workloads.
package energy

import "fmt"

// EnergyEstimate holds the result of an energy estimation.
type EnergyEstimate struct {
	Joules float64
	Source string // "model-benchmark" or "none"
}

// EnergyEstimator estimates energy consumption for an AI inference call.
type EnergyEstimator interface {
	Estimate(model string, inputTokens, outputTokens int64) (*EnergyEstimate, error)
}

// modelBenchmark holds per-token energy figures in joules.
type modelBenchmark struct {
	InputJoulesPerToken  float64
	OutputJoulesPerToken float64
}

// BenchmarkEstimator uses the AI Energy Score benchmark table.
// Values are derived from published hardware benchmarks (H100, A100) and
// typical batch sizes. All figures are conservative upper-bound estimates.
type BenchmarkEstimator struct {
	table map[string]modelBenchmark
}

// NewBenchmarkEstimator creates an estimator pre-loaded with popular models.
func NewBenchmarkEstimator() *BenchmarkEstimator {
	// Energy per token in joules.
	// Source: AI Energy Score methodology, MLCOMMONS MLPerf Inference benchmarks.
	// Conversion: (power_W / throughput_tok_s) = J/tok
	table := map[string]modelBenchmark{
		// OpenAI
		"gpt-4o":           {InputJoulesPerToken: 0.0030, OutputJoulesPerToken: 0.0090},
		"gpt-4o-mini":      {InputJoulesPerToken: 0.0004, OutputJoulesPerToken: 0.0012},
		"gpt-4-turbo":      {InputJoulesPerToken: 0.0035, OutputJoulesPerToken: 0.0105},
		"gpt-3.5-turbo":    {InputJoulesPerToken: 0.0002, OutputJoulesPerToken: 0.0006},
		// Anthropic
		"claude-opus-4":        {InputJoulesPerToken: 0.0040, OutputJoulesPerToken: 0.0120},
		"claude-sonnet-4-5":    {InputJoulesPerToken: 0.0012, OutputJoulesPerToken: 0.0036},
		"claude-haiku-4-5":     {InputJoulesPerToken: 0.0003, OutputJoulesPerToken: 0.0009},
		"claude-3-5-sonnet":    {InputJoulesPerToken: 0.0010, OutputJoulesPerToken: 0.0030},
		"claude-3-haiku":       {InputJoulesPerToken: 0.0002, OutputJoulesPerToken: 0.0006},
		// Meta Llama (estimated for 8B on H100)
		"meta-llama/Llama-3.1-8B":   {InputJoulesPerToken: 0.0001, OutputJoulesPerToken: 0.0003},
		"meta-llama/Llama-3.1-70B":  {InputJoulesPerToken: 0.0008, OutputJoulesPerToken: 0.0024},
		"meta-llama/Llama-3.1-405B": {InputJoulesPerToken: 0.0045, OutputJoulesPerToken: 0.0135},
		// Google
		"google/gemini-1.5-pro":   {InputJoulesPerToken: 0.0025, OutputJoulesPerToken: 0.0075},
		"google/gemini-1.5-flash": {InputJoulesPerToken: 0.0003, OutputJoulesPerToken: 0.0009},
		// Mistral
		"mistralai/Mistral-7B-Instruct-v0.3":   {InputJoulesPerToken: 0.0001, OutputJoulesPerToken: 0.0003},
		"mistralai/Mixtral-8x7B-Instruct-v0.1": {InputJoulesPerToken: 0.0004, OutputJoulesPerToken: 0.0012},
	}
	return &BenchmarkEstimator{table: table}
}

// Estimate computes total energy in joules for the given model and token counts.
// Returns source="none" if the model is not in the benchmark table.
func (e *BenchmarkEstimator) Estimate(model string, inputTokens, outputTokens int64) (*EnergyEstimate, error) {
	b, ok := e.table[model]
	if !ok {
		return &EnergyEstimate{Joules: 0, Source: "none"}, nil
	}
	joules := b.InputJoulesPerToken*float64(inputTokens) +
		b.OutputJoulesPerToken*float64(outputTokens)
	return &EnergyEstimate{Joules: joules, Source: "model-benchmark"}, nil
}

// RegisterModel adds or replaces a benchmark entry.
func (e *BenchmarkEstimator) RegisterModel(model string, inputJoulesPerToken, outputJoulesPerToken float64) error {
	if model == "" {
		return fmt.Errorf("model name must not be empty")
	}
	e.table[model] = modelBenchmark{
		InputJoulesPerToken:  inputJoulesPerToken,
		OutputJoulesPerToken: outputJoulesPerToken,
	}
	return nil
}

// CarbonEstimateKgCO2e converts joules and carbon intensity (gCO2e/kWh) to kg CO2e.
func CarbonEstimateKgCO2e(joules, gCO2ePerKWh float64) float64 {
	// kWh = joules / 3_600_000; kgCO2e = kWh * gCO2e/kWh / 1000
	return (joules / 3_600_000) * gCO2ePerKWh / 1000
}
