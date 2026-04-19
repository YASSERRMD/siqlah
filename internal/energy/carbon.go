package energy

import "fmt"

// CarbonLookup returns the average grid carbon intensity in gCO2e/kWh for a region.
type CarbonLookup interface {
	Intensity(region string) (float64, error)
}

// StaticCarbonLookup uses hardcoded averages from Electricity Maps (2024 annual avg).
type StaticCarbonLookup struct {
	table map[string]float64
}

// NewStaticCarbonLookup returns a CarbonLookup with averages for major cloud regions.
func NewStaticCarbonLookup() *StaticCarbonLookup {
	// All values in gCO2e/kWh. Source: Electricity Maps 2024 annual averages.
	table := map[string]float64{
		// AWS regions
		"us-east-1":      360, // N. Virginia — PJM grid
		"us-east-2":      560, // Ohio — midwest coal/gas mix
		"us-west-1":      210, // N. California — cleaner grid
		"us-west-2":      130, // Oregon — heavy hydro/wind
		"eu-west-1":      280, // Ireland
		"eu-west-2":      225, // London
		"eu-west-3":      60,  // Paris — heavy nuclear
		"eu-central-1":   320, // Frankfurt
		"eu-north-1":     20,  // Stockholm — almost entirely hydro
		"ap-southeast-1": 430, // Singapore
		"ap-northeast-1": 490, // Tokyo
		"ap-northeast-2": 420, // Seoul
		"ap-south-1":     700, // Mumbai — coal heavy
		"ca-central-1":   40,  // Canada — heavy hydro
		"sa-east-1":      74,  // São Paulo — hydro dominated
		// GCP regions (use same grid approximations)
		"us-central1":  490, // Iowa
		"us-east4":     360, // N. Virginia
		"us-west1":     130, // Oregon
		"europe-west1": 170, // Belgium
		"europe-west4": 310, // Netherlands
		// Azure regions
		"eastus":         360,
		"westus":         200,
		"westeurope":     310, // Netherlands
		"northeurope":    280, // Ireland
		"swedencentral":  20,  // Sweden — almost all hydro+nuclear
		// Generic fallback keys
		"global": 436, // IEA 2023 world average
	}
	return &StaticCarbonLookup{table: table}
}

// Intensity returns gCO2e/kWh for the given region.
// Falls back to "global" average if the region is unknown.
func (l *StaticCarbonLookup) Intensity(region string) (float64, error) {
	if v, ok := l.table[region]; ok {
		return v, nil
	}
	if v, ok := l.table["global"]; ok {
		return v, nil
	}
	return 0, fmt.Errorf("no carbon intensity data for region %q", region)
}

// RegisterRegion adds or replaces a region's carbon intensity.
func (l *StaticCarbonLookup) RegisterRegion(region string, gCO2ePerKWh float64) {
	l.table[region] = gCO2ePerKWh
}
