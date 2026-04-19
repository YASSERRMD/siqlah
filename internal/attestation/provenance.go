package attestation

import "github.com/yasserrmd/siqlah/pkg/vur"

// SiqlahPredicate is the "https://siqlah.dev/receipt/v1" predicate body.
// It carries the full receipt plus optional model provenance and energy footprint.
type SiqlahPredicate struct {
	Receipt         vur.Receipt      `json:"receipt"`
	ModelProvenance *ModelProvenance `json:"model_provenance,omitempty"`
	EnergyFootprint *EnergyFootprint `json:"energy_footprint,omitempty"`
}

// ModelProvenance captures verifiable identity information about the AI model
// that produced the output covered by this receipt.
type ModelProvenance struct {
	ModelName      string `json:"model_name"`
	ModelDigest    string `json:"model_digest,omitempty"`
	SignerIdentity string `json:"signer_identity,omitempty"`
	Verified       bool   `json:"verified"`
}

// EnergyFootprint captures the estimated energy and carbon cost of the inference.
type EnergyFootprint struct {
	Joules      float64 `json:"joules"`
	CarbonGCO2e float64 `json:"carbon_gco2e,omitempty"`
	Region      string  `json:"region,omitempty"`
}

// buildPredicate constructs a SiqlahPredicate from a receipt, populating
// optional sub-objects only when the relevant fields are present.
func buildPredicate(r *vur.Receipt) SiqlahPredicate {
	pred := SiqlahPredicate{Receipt: *r}

	if r.ModelSignerIdentity != "" || r.ModelSignatureVerified {
		pred.ModelProvenance = &ModelProvenance{
			ModelName:      r.Model,
			ModelDigest:    r.ModelDigest,
			SignerIdentity: r.ModelSignerIdentity,
			Verified:       r.ModelSignatureVerified,
		}
	}

	if r.EnergyEstimateJoules > 0 {
		carbon := r.CarbonIntensityGCO2ePerKWh * r.EnergyEstimateJoules / 3_600_000
		pred.EnergyFootprint = &EnergyFootprint{
			Joules:      r.EnergyEstimateJoules,
			CarbonGCO2e: carbon,
			Region:      r.InferenceRegion,
		}
	}

	return pred
}
