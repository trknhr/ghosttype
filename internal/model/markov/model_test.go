package markov

import (
	"reflect"
	"testing"

	"github.com/trknhr/ghosttype/internal/model/entity"
)

func TestMarkovModel_LearnAndPredict(t *testing.T) {
	modelMarkov := NewMarkovModel()

	// Training data simulating common shell commands
	entries := []string{
		"git commit",
		"git commit",
		"git push",
		"git push origin main",
		"git push",
		"git pull",
		"npm install",
		"npm run build",
		"npm run build",
	}

	modelMarkov.Learn(entries)

	tests := []struct {
		input    string
		expected []entity.Suggestion
	}{
		{
			input: "git",
			expected: []entity.Suggestion{
				{Text: "git push", Score: 3, Source: "markov"},
				{Text: "git commit", Score: 2, Source: "markov"},
				{Text: "git pull", Score: 1, Source: "markov"},
			},
		},
		{
			input: "npm",
			expected: []entity.Suggestion{
				{Text: "npm run", Score: 2, Source: "markov"},
				{Text: "npm install", Score: 1, Source: "markov"},
			},
		},
		{
			input: "npm run",
			expected: []entity.Suggestion{
				{Text: "npm run build", Score: 2, Source: "markov"},
			},
		},
		{
			input:    "echo", // Not in training data
			expected: nil,
		},
	}

	for _, tt := range tests {
		got, err := modelMarkov.Predict(tt.input)

		if err != nil {
			t.Errorf("Predict(%q) returned unexpected error: %v", tt.input, err)
		}
		// Compare the predicted results with expected output
		if !reflect.DeepEqual(got, tt.expected) {
			t.Errorf("Predict(%q) mismatch.\nExpected: %#v\nGot:      %#v", tt.input, tt.expected, got)
		}
	}
}
