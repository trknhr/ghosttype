package freq

import (
	"sort"
	"strings"

	"github.com/trknhr/ghosttype/model"
)

type FreqModel struct {
	Counts map[string]int
}

func NewFreqModel() model.SuggestModel {
	return &FreqModel{Counts: make(map[string]int)}
}

func (m *FreqModel) Learn(entries []string) error {
	for _, entry := range entries {
		cmd := strings.TrimSpace(entry)
		if cmd != "" {
			m.Counts[cmd]++
		}
	}
	return nil
}

func (m *FreqModel) Predict(input string) ([]model.Suggestion, error) {
	type pair struct {
		cmd   string
		count int
	}

	var matches []pair
	for cmd, count := range m.Counts {
		if strings.HasPrefix(cmd, input) {
			matches = append(matches, pair{cmd, count})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].count > matches[j].count
	})

	results := make([]model.Suggestion, len(matches))
	for i := range matches {
		results[i] = model.Suggestion{
			Text:   matches[i].cmd,
			Score:  float64(matches[i].count),
			Source: "freq",
		}
	}
	return results, nil
}

func (m *FreqModel) Weight() float64 {
	return 0.5
}
