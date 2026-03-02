package harness

// parse.go — helpers that extract token counts and cost from agent stdout/stderr.
//
// Ported from thegent src/thegent/agents/crew/harness.py:
//   _parse_tokens_from_output / _parse_cost_from_output

import (
	"regexp"
	"strconv"
	"strings"
)

// Token-count patterns (most-specific first).
var (
	// "Tokens: 1234 input, 567 output"  or  "Usage: 1234 prompt, 567 completion"
	reTokensInOut = regexp.MustCompile(`(?i)(?:tokens?|usage)[\s:]*?(\d+)[\s,]*(?:input|prompt)[^\d]*(\d+)[\s,]*(?:output|completion)`)
	// "input_tokens=1234 ... output_tokens=567"
	reTokensEquals = regexp.MustCompile(`(?i)(?:input|prompt)_tokens?=(\d+).*?(?:output|completion)_tokens?=(\d+)`)
	// "Prompt tokens: 1234, Completion tokens: 567"
	reTokensLabel = regexp.MustCompile(`(?i)prompt[\s_-]?tokens?[\s:]*(\d+).*?completion[\s_-]?tokens?[\s:]*(\d+)`)
	// "1234 + 567 = 1801 total"
	reTokensSum = regexp.MustCompile(`(?i)(\d+)\s*\+\s*(\d+)\s*=\s*\d+\s*total`)
)

// Cost patterns.
var (
	// "cost_usd=0.0123"
	reCostEquals = regexp.MustCompile(`(?i)cost_usd=(\d+\.?\d*)`)
	// "Cost: $0.05"  or  "Total cost: $0.05"
	reCostLabel = regexp.MustCompile(`(?i)(?:total\s+)?cost[\s:]*(?:\$)?(\d+\.?\d*)`)
)

// parseTokens extracts (promptTokens, completionTokens) from combined agent output.
func parseTokens(output string) (int, int) {
	for _, re := range []*regexp.Regexp{reTokensInOut, reTokensEquals, reTokensLabel, reTokensSum} {
		m := re.FindStringSubmatch(output)
		if m == nil {
			continue
		}
		p, errP := strconv.Atoi(m[1])
		c, errC := strconv.Atoi(m[2])
		if errP == nil && errC == nil {
			return p, c
		}
	}
	return 0, 0
}

// parseCost extracts a cost-in-USD value from combined agent output.
func parseCost(output string) float64 {
	for _, re := range []*regexp.Regexp{reCostEquals, reCostLabel} {
		m := re.FindStringSubmatch(output)
		if m == nil {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(m[1]), 64)
		if err == nil {
			return v
		}
	}
	return 0.0
}
