package ensemble

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/trknhr/ghosttype/internal/model/entity"
	"golang.org/x/sync/errgroup"
)

const SuggestionTimeout = 2 * time.Second

type Ensemble struct {
	LightModels atomic.Value // []model.SuggestModel
	HeavyModels atomic.Value // []model.SuggestModel
	Models      atomic.Value // []model.SuggestModel
}

func NewEnsemble(lightModels []entity.SuggestModel) *Ensemble {
	e := &Ensemble{}
	e.LightModels.Store(lightModels)
	e.HeavyModels.Store([]entity.SuggestModel{})
	e.Models.Store(lightModels)
	return e
}

func (e *Ensemble) AddHeavyModel(m entity.SuggestModel) {
	// append heavy
	heavy := e.HeavyModels.Load().([]entity.SuggestModel)
	newHeavy := make([]entity.SuggestModel, len(heavy)+1)
	copy(newHeavy, heavy)
	newHeavy[len(heavy)] = m
	e.HeavyModels.Store(newHeavy)

	// append all
	all := e.Models.Load().([]entity.SuggestModel)
	newAll := make([]entity.SuggestModel, len(all)+1)
	copy(newAll, all)
	newAll[len(all)] = m
	e.Models.Store(newAll)
}

func (e *Ensemble) Learn(entries []string) error {
	var allErr error
	for _, m := range e.Models.Load().([]entity.SuggestModel) {
		err := m.Learn(entries)
		if err != nil {
			allErr = errors.Join(allErr, err)
		}
	}

	return allErr
}

func (e *Ensemble) Predict(input string) ([]entity.Suggestion, error) {
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

	for _, m := range e.Models.Load().([]entity.SuggestModel) {
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

	results := make([]entity.Suggestion, len(rankedList))
	for i := range rankedList {
		results[i] = entity.Suggestion{
			Text:  rankedList[i].Text,
			Score: rankedList[i].Score,
		}
	}

	return results, nil
}

func (m *Ensemble) Weight() float64 {
	return 0
}

// Progressive enhancement prediction
func (e *Ensemble) NextPredict(input string) (<-chan []entity.Suggestion, error) {
	resultChan := make(chan []entity.Suggestion, 2)
	ctx, cancel := context.WithTimeout(context.Background(), SuggestionTimeout)

	go func() {
		defer cancel()
		defer close(resultChan)

		if len(e.LightModels.Load().([]entity.SuggestModel)) > 0 {
			lightResults := e.executeLightModels(ctx, input)
			resultChan <- lightResults
		}

		if len(e.HeavyModels.Load().([]entity.SuggestModel)) > 0 {
			heavyResults := e.executeHeavyModels(ctx, input)
			resultChan <- heavyResults
		}
	}()

	return resultChan, nil
}

func (e *Ensemble) executeLightModels(ctx context.Context, input string) []entity.Suggestion {
	scoreMap := make(map[string]float64)
	var mu sync.Mutex

	lightCtx, lightCancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer lightCancel()

	g, lightCtx := errgroup.WithContext(lightCtx)

	for _, m := range e.LightModels.Load().([]entity.SuggestModel) {
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

func (e *Ensemble) executeHeavyModels(ctx context.Context, input string) []entity.Suggestion {
	scoreMap := make(map[string]float64)
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)

	for _, m := range e.HeavyModels.Load().([]entity.SuggestModel) {
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

func (e *Ensemble) rankSuggestions(scoreMap map[string]float64) []entity.Suggestion {
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

	results := make([]entity.Suggestion, len(rankedList))
	for i := range rankedList {
		results[i] = entity.Suggestion{
			Text:  rankedList[i].Text,
			Score: rankedList[i].Score,
		}
	}

	return results
}
