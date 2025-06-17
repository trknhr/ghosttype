package entity

type Suggestion struct {
	Text   string
	Source string
	Score  float64
}

type SuggestModel interface {
	Learn(entries []string) error
	Predict(input string) ([]Suggestion, error)
	Weight() float64
}
