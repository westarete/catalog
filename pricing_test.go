package main

import (
	"bytes"
	"math"
	"testing"
	"time"
)

func TestPricingCostMatchesARealRun(t *testing.T) {
	// The Haiku 5.5 run that prompted this feature: 24 calls, 388,131 tokens
	// in, 12,560 out, about 4.5 cents at $0.10 / $0.50 per million.
	p := pricing{InputPerMTok: 0.10, OutputPerMTok: 0.50}
	got := p.cost(tokens{input: 388131, output: 12560})
	if math.Abs(got-0.04509) > 0.00001 {
		t.Errorf("cost = %f, want about 0.04509", got)
	}
}

func TestPricingCostChargesCacheReadsSeparately(t *testing.T) {
	p := pricing{InputPerMTok: 1, OutputPerMTok: 5, CacheReadPerMTok: 0.1}
	got := p.cost(tokens{input: 1_000_000, cached: 1_000_000, output: 1_000_000})
	if want := 1 + 0.1 + 5; math.Abs(got-want) > 1e-9 {
		t.Errorf("cost = %f, want %f", got, want)
	}
}

func TestPricingCostDefaultsCacheReadsToTheInputPrice(t *testing.T) {
	p := pricing{InputPerMTok: 1, OutputPerMTok: 5}
	if got, want := p.cost(tokens{cached: 1_000_000}), 1.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("cost = %f, want %f", got, want)
	}
}

func TestFormatDollars(t *testing.T) {
	for d, want := range map[float64]string{
		0:       "<$0.001",
		0.0004:  "<$0.001",
		0.0063:  "~$0.006",
		0.04509: "~$0.045",
		0.999:   "~$0.999",
		12.4:    "~$12.40",
	} {
		if got := formatDollars(d); got != want {
			t.Errorf("formatDollars(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestCostLineEndsWithEstimateWhenPriced(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config{InputPricePerMTok: 0.10, OutputPricePerMTok: 0.50}
	newReporter(&buf).cost(24, tokens{input: 388131, output: 12560}, 68*time.Second, cfg)
	want := "24 model calls, 388,131 tokens in, 12,560 out, 1m8s, ~$0.045\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCostLineNotesRequestsOverTheWarningSize(t *testing.T) {
	used := tokens{input: 388131, output: 12560, long: 3}
	for name, tc := range map[string]struct {
		cfg  *config
		want string
	}{
		"priced": {
			&config{WarnRequestTokens: 100_000, InputPricePerMTok: 0.10, OutputPricePerMTok: 0.50},
			"24 model calls, 388,131 tokens in, 12,560 out, 1m8s, ~$0.045\n" +
				"note: 3 of 24 requests went over 100,000 tokens; the estimate uses base prices, so the real cost may be higher.\n",
		},
		"unpriced": {
			&config{WarnRequestTokens: 100_000},
			"24 model calls, 388,131 tokens in, 12,560 out, 1m8s\n" +
				"note: 3 of 24 requests went over 100,000 tokens; some models charge more for long requests.\n",
		},
	} {
		var buf bytes.Buffer
		newReporter(&buf).cost(24, used, 68*time.Second, tc.cfg)
		if got := buf.String(); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}
