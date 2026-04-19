package api

import "net/http"

// EnergyStatsResponse is returned by GET /v1/stats/energy.
type EnergyStatsResponse struct {
	InferenceRegion            string  `json:"inference_region,omitempty"`
	CarbonIntensityGCO2ePerKWh float64 `json:"carbon_intensity_gco2e_per_kwh,omitempty"`
	EnergyEstimationEnabled    bool    `json:"energy_estimation_enabled"`
}

func (s *Server) handleEnergyStats(w http.ResponseWriter, r *http.Request) {
	resp := EnergyStatsResponse{
		EnergyEstimationEnabled: true,
	}
	if s.inferenceRegion != "" {
		resp.InferenceRegion = s.inferenceRegion
		if intensity, err := s.carbonLookup.Intensity(s.inferenceRegion); err == nil {
			resp.CarbonIntensityGCO2ePerKWh = intensity
		}
	}
	writeJSON(w, http.StatusOK, resp)
}
