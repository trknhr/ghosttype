package ensemble

import (
	"errors"
	"sort"

	"github.com/trknhr/ghosttype/model"
)

type Ensemble struct {
	Models []model.SuggestModel
}

func New(models ...model.SuggestModel) model.SuggestModel {
	return &Ensemble{Models: models}
}

func (e *Ensemble) Learn(entries []string) error {
	var allErr error
	for _, m := range e.Models {
		err := m.Learn(entries)
		if err != nil {
			allErr = errors.Join(allErr, err)
		}
	}

	return allErr
}

func (e *Ensemble) Predict(input string) ([]model.Suggestion, error) {
	type ranked struct {
		Text  string
		Score float64
	}
	scoreMap := make(map[string]float64)

	var allErr error
	for _, model := range e.Models {
		suggestions, err := model.Predict(input)
		if err != nil {
			allErr = errors.Join(allErr, err)
		}
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

	results := make([]model.Suggestion, len(rankedList))
	for i := range rankedList {
		results[i].Text = rankedList[i].Text
	}
	return results, allErr
}

func (m *Ensemble) Weight() float64 {
	return 0
}
