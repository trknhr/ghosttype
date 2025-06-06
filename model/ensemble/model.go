package ensemble

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/trknhr/ghosttype/model"
	"golang.org/x/sync/errgroup"
)

const SuggestionTimeout = 2 * time.Second

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
	var mu sync.Mutex

	// time out context
	ctx, cancel := context.WithTimeout(context.Background(), SuggestionTimeout)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	for _, m := range e.Models {
		model := m

		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				suggestions, err := model.Predict(input)
				if err != nil {
					return err
				}

				weight := model.Weight()

				mu.Lock()
				for _, s := range suggestions {
					scoreMap[s.Text] += s.Score * weight
				}
				mu.Unlock()

				return nil
			}
		})
	}

	err := g.Wait()

	if err != nil {
		return nil, err
	}

	rankedList := make([]ranked, 0, len(scoreMap))
	for text, score := range scoreMap {
		rankedList = append(rankedList, ranked{text, score})
	}
	sort.Slice(rankedList, func(i, j int) bool {
		return rankedList[i].Score > rankedList[j].Score
	})

	results := make([]model.Suggestion, len(rankedList))
	for i := range rankedList {
		results[i] = model.Suggestion{
			Text:  rankedList[i].Text,
			Score: rankedList[i].Score,
		}
	}

	return results, nil
}

func (m *Ensemble) Weight() float64 {
	return 0
}
