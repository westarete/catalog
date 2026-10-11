package main

import (
	"errors"
	"fmt"
)

// pricing is what the configured model charges, in dollars per million tokens.
// The Models API reports no prices, and they change, so they live in
// .catalog/config.toml where a person (or an agent reading the pricing page)
// keeps them current. Without a [pricing] table a run reports tokens only.
type pricing struct {
	InputPerMTok  float64 `toml:"input_per_mtok"`
	OutputPerMTok float64 `toml:"output_per_mtok"`
	// CacheReadPerMTok is optional. Unset, cached tokens are charged at the
	// input price, which can only overstate the cost.
	CacheReadPerMTok float64 `toml:"cache_read_per_mtok"`
}

func (p pricing) validate() error {
	if p.InputPerMTok <= 0 || p.OutputPerMTok <= 0 {
		return errors.New("[pricing] needs input_per_mtok and output_per_mtok, both above zero")
	}
	if p.CacheReadPerMTok < 0 {
		return errors.New("[pricing] cache_read_per_mtok cannot be negative")
	}
	return nil
}

// cost is the dollar cost of the given token counts. The API's input count
// excludes cache reads, so the two are charged separately.
func (p pricing) cost(t tokens) float64 {
	cacheRead := p.CacheReadPerMTok
	if cacheRead == 0 {
		cacheRead = p.InputPerMTok
	}
	return (float64(t.input)*p.InputPerMTok +
		float64(t.cached)*cacheRead +
		float64(t.output)*p.OutputPerMTok) / 1_000_000
}

// formatDollars renders an estimate for the cost line: "~$0.045", "~$12.40".
// Costs under a tenth of a cent read "<$0.001" rather than a misleading zero.
func formatDollars(d float64) string {
	switch {
	case d < 0.0005:
		return "<$0.001"
	case d < 1:
		return fmt.Sprintf("~$%.3f", d)
	default:
		return fmt.Sprintf("~$%.2f", d)
	}
}
