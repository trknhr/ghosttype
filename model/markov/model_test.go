package markov

import (
	"reflect"
	"testing"

	"github.com/trknhr/ghosttype/model"
)

func TestMarkovModel_LearnAndPredict(t *testing.T) {
	modelMarkov := NewModel()

	// Training data simulating common shell commands
	entries := []string{
		"git commit",
		"git push",
		"git push origin main",
		"git pull",
		"npm install",
		"npm run build",
	}

	modelMarkov.Learn(entries)

	tests := []struct {
		input    string
		expected []model.Suggestion
	}{
		{
			input: "git",
			expected: []model.Suggestion{
				{Text: "git push", Score: 2, Source: "markov"},
				{Text: "git commit", Score: 1, Source: "markov"},
				{Text: "git pull", Score: 1, Source: "markov"},
			},
		},
		{
			input: "npm",
			expected: []model.Suggestion{
				{Text: "npm install", Score: 1, Source: "markov"},
				{Text: "npm run", Score: 1, Source: "markov"},
			},
		},
		{
			input: "npm run",
			expected: []model.Suggestion{
				{Text: "npm run build", Score: 1, Source: "markov"},
			},
		},
		{
			input:    "echo", // Not in training data
			expected: nil,
		},
	}

	for _, tt := range tests {
		got := modelMarkov.Predict(tt.input)

		// Compare the predicted results with expected output
		if !reflect.DeepEqual(got, tt.expected) {
			t.Errorf("Predict(%q) mismatch.\nExpected: %#v\nGot:      %#v", tt.input, tt.expected, got)
		}
	}
}
