package markov

import (
	"fmt"
	"sort"
	"strings"

	"github.com/trknhr/ghosttype/model"
)

type MarkovModel struct {
	Transitions map[string]map[string]int
}

func NewMarkovModel() model.SuggestModel {
	return &MarkovModel{
		Transitions: make(map[string]map[string]int),
	}
}

func (m *MarkovModel) Learn(entries []string) error {
	for _, entry := range entries {
		tokens := strings.Fields(entry)
		for i := 0; i < len(tokens)-1; i++ {
			from := tokens[i]
			to := tokens[i+1]

			if _, ok := m.Transitions[from]; !ok {
				m.Transitions[from] = make(map[string]int)
			}
			m.Transitions[from][to]++
		}
	}
	return nil
}

func (m *MarkovModel) Predict(input string) ([]model.Suggestion, error) {
	tokens := strings.Fields(input)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("input is empty or contains only whitespace")
	}
	last := tokens[len(tokens)-1]

	nextMap, ok := m.Transitions[last]
	if !ok || len(nextMap) == 0 {
		return nil, nil
	}

	// Order by score
	type pair struct {
		token string
		count int
	}
	var pairs []pair
	for token, count := range nextMap {
		pairs = append(pairs, pair{token, count})
	}

	// DESC
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].count > pairs[j].count
	})

	results := make([]model.Suggestion, len(pairs))

	for i := 0; i < len(pairs); i++ {
		results[i] = model.Suggestion{
			Text:   fmt.Sprintf("%s %s", strings.TrimSpace(input), pairs[i].token),
			Score:  float64(pairs[i].count),
			Source: "markov",
		}
	}
	return results, nil
}

func (m *MarkovModel) Weight() float64 {
	return 0.4
}
