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
	LightModels []model.SuggestModel
	HeavyModels []model.SuggestModel
	Models      []model.SuggestModel
}

func New(models ...model.SuggestModel) *Ensemble {
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

// type Ensemble struct {
// 	LightModels []model.SuggestModel // 軽量で高速なモデル
// 	HeavyModels []model.SuggestModel // 重い・遅いが高品質なモデル
// 	Models      []model.SuggestModel // 既存のフィールド（後方互換性のため）
// }

func NewWithClassification(lightModels, heavyModels []model.SuggestModel) *Ensemble {
	allModels := make([]model.SuggestModel, 0, len(lightModels)+len(heavyModels))
	allModels = append(allModels, lightModels...)
	allModels = append(allModels, heavyModels...)

	return &Ensemble{
		LightModels: lightModels,
		HeavyModels: heavyModels,
		Models:      allModels,
	}
}

// Progressive enhancement prediction
func (e *Ensemble) NextPredict(input string) (<-chan []model.Suggestion, error) {
	resultChan := make(chan []model.Suggestion, 2)
	ctx, cancel := context.WithTimeout(context.Background(), SuggestionTimeout)

	go func() {
		defer cancel()
		defer close(resultChan)

		if len(e.LightModels) > 0 {
			lightResults := e.executeLightModels(ctx, input)
			resultChan <- lightResults
		}

		if len(e.HeavyModels) > 0 {
			heavyResults := e.executeHeavyModels(ctx, input)
			resultChan <- heavyResults
		}
	}()

	return resultChan, nil
}

func (e *Ensemble) executeLightModels(ctx context.Context, input string) []model.Suggestion {
	scoreMap := make(map[string]float64)
	var mu sync.Mutex

	lightCtx, lightCancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer lightCancel()

	g, lightCtx := errgroup.WithContext(lightCtx)

	for _, m := range e.LightModels {
		model := m
		g.Go(func() error {
			select {
			case <-lightCtx.Done():
				return lightCtx.Err()
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

	g.Wait()

	return e.rankSuggestions(scoreMap)
}

func (e *Ensemble) executeHeavyModels(ctx context.Context, input string) []model.Suggestion {
	scoreMap := make(map[string]float64)
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)

	for _, m := range e.HeavyModels {
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

	g.Wait()

	return e.rankSuggestions(scoreMap)
}

func (e *Ensemble) rankSuggestions(scoreMap map[string]float64) []model.Suggestion {
	type ranked struct {
		Text  string
		Score float64
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

	return results
}
