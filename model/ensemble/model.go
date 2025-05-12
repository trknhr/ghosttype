package ensemble

import (
	"sort"

	"github.com/trknhr/ghosttype/model"
)

type Ensemble struct {
	Models []model.SuggestModel
}

func New(models ...model.SuggestModel) *Ensemble {
	return &Ensemble{Models: models}
}

func (e *Ensemble) Learn(entries []string) {
	for _, m := range e.Models {
		m.Learn(entries)
	}
}

func (e *Ensemble) Predict(input string) []string {
	type ranked struct {
		Text  string
		Score float64
	}
	scoreMap := make(map[string]float64)

	for _, model := range e.Models {
		suggestions := model.Predict(input)
		weight := model.Weight()
		for _, s := range suggestions {
			scoreMap[s.Text] += s.Score * weight
		}
	}

	var rankedList []ranked
	for text, score := range scoreMap {
		rankedList = append(rankedList, ranked{text, score})
	}
	sort.Slice(rankedList, func(i, j int) bool {
		return rankedList[i].Score > rankedList[j].Score
	})

	results := make([]string, len(rankedList))
	for i := range rankedList {
		results[i] = rankedList[i].Text
	}
	return results
}
