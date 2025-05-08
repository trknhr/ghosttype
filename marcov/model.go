package markov

type Model struct {
	Transitions map[string]map[string]int
}

func NewModel() *Model {
	return &Model{
		Transitions: make(map[string]map[string]int),
	}
}

func (m *Model) Learn(entries []string) {
	for i := 0; i < len(entries)-1; i++ {
		from := entries[i]
		to := entries[i+1]

		if _, ok := m.Transitions[from]; !ok {
			m.Transitions[from] = make(map[string]int)
		}
		m.Transitions[from][to]++
	}
}

func (m *Model) PredictNext(current string) string {
	nextMap, ok := m.Transitions[current]
	if !ok || len(nextMap) == 0 {
		return ""
	}

	var bestCmd string
	maxCount := 0
	for cmd, count := range nextMap {
		if count > maxCount {
			bestCmd = cmd
			maxCount = count
		}
	}
	return bestCmd
}
