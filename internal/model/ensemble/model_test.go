package ensemble_test

import (
	"testing"
	"time"

	"github.com/trknhr/ghosttype/internal/model/ensemble"
	"github.com/trknhr/ghosttype/internal/model/entity"
)

type mockModel struct {
	id     string
	result []entity.Suggestion
	weight float64
	delay  time.Duration
}

func (m *mockModel) Predict(input string) ([]entity.Suggestion, error) {
	time.Sleep(m.delay)
	return m.result, nil
}

func (m *mockModel) Learn(entries []string) error {
	return nil
}

func (m *mockModel) Weight() float64 {
	return m.weight
}

func TestProgressivePredict(t *testing.T) {
	light := &mockModel{
		id: "light",
		result: []entity.Suggestion{
			{Text: "light suggestion", Score: 0.5},
		},
		weight: 1.0,
		delay:  10 * time.Millisecond,
	}

	heavy := &mockModel{
		id: "heavy",
		result: []entity.Suggestion{
			{Text: "heavy suggestion", Score: 1.0},
		},
		weight: 2.0,
		delay:  100 * time.Millisecond,
	}

	e := ensemble.NewEnsemble([]entity.SuggestModel{light})
	e.AddHeavyModel(heavy)

	ch, err := e.ProgressivePredict("gi")
	if err != nil {
		t.Fatalf("ProgressivePredict failed: %v", err)
	}

	var lightSuggestions, heavySuggestions []entity.Suggestion

	for i := 0; i < 2; i++ {
		select {
		case r, ok := <-ch:
			if !ok {
				t.Fatalf("result channel closed unexpectedly")
			}
			if i == 0 {
				lightSuggestions = r
			} else {
				heavySuggestions = r
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timeout waiting for result %d", i)
		}
	}

	if len(lightSuggestions) != 1 || lightSuggestions[0].Text != "light suggestion" {
		t.Errorf("unexpected light result: %+v", light)
	}

	if len(heavySuggestions) != 1 || heavySuggestions[0].Text != "heavy suggestion" {
		t.Errorf("unexpected heavy result: %+v", heavy)
	}
}

type syncMockModel struct {
	output []entity.Suggestion
	weight float64
}

func (m *syncMockModel) Predict(input string) ([]entity.Suggestion, error) {
	return m.output, nil
}

func (m *syncMockModel) Learn([]string) error {
	return nil
}

func (m *syncMockModel) Weight() float64 {
	return m.weight
}

func TestEnsemblePredict(t *testing.T) {
	model1 := &syncMockModel{
		output: []entity.Suggestion{
			{Text: "git commit", Score: 1.0},
			{Text: "git add", Score: 0.5},
		},
		weight: 1.0,
	}
	model2 := &syncMockModel{
		output: []entity.Suggestion{
			{Text: "git commit", Score: 0.8}, // 0.8 * 2.0 = 1.6
			{Text: "git push", Score: 0.9},   // 0.9 * 2.0 = 1.8
		},
		weight: 2.0,
	}

	e := ensemble.NewEnsemble([]entity.SuggestModel{model1})
	e.AddHeavyModel(model2)

	suggestions, err := e.Predict("git")
	if err != nil {
		t.Fatalf("Predict failed: %v", err)
	}

	if len(suggestions) != 3 {
		t.Fatalf("expected 3 unique suggestions, got %d", len(suggestions))
	}

	// ordered by score
	expected := []struct {
		Text  string
		Score float64
	}{
		{"git commit", 1.0 + 1.6},
		{"git push", 1.8},
		{"git add", 0.5},
	}

	for i, exp := range expected {
		if suggestions[i].Text != exp.Text {
			t.Errorf("expected suggestion[%d] to be %q, got %q", i, exp.Text, suggestions[i].Text)
		}
		if suggestions[i].Score != exp.Score {
			t.Errorf("expected suggestion[%d] score to be %f, got %f", i, exp.Score, suggestions[i].Score)
		}
	}
}
