package markov

import (
	"sort"
	"strings"
)

type Model struct {
	Transitions map[string]map[string]int
}

func NewModel() *Model {
	return &Model{
		Transitions: make(map[string]map[string]int),
	}
}

func (m *Model) Learn(entries []string) {
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
}

func (m *Model) PredictNext(input string) []string {
	tokens := strings.Fields(input)
	if len(tokens) == 0 {
		return nil
	}
	last := tokens[len(tokens)-1]

	nextMap, ok := m.Transitions[last]
	if !ok || len(nextMap) == 0 {
		return nil
	}

	// スコア順に並べる
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

	results := make([]string, len(pairs))
	for i := 0; i < len(pairs); i++ {
		results[i] = pairs[i].token
	}
	return results
}
