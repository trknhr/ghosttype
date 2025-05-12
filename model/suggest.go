package model

type Suggestion struct {
	Text   string
	Source string
	Score  float64
}

type SuggestModel interface {
	Learn(entries []string)
	Predict(input string) []Suggestion
	Weight() float64
}
