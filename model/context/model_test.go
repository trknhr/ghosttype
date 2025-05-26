package context_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/trknhr/ghosttype/model"
	"github.com/trknhr/ghosttype/model/context"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
}

func TestContextModel_Predict(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile(t, tmpDir, "package.json", `{
		"scripts": {
			"dev": "vite dev",
			"build": "vite build"
		}
	}`)
	writeFile(t, tmpDir, "Makefile", `
build:
	go build -o bin/app
`)
	writeFile(t, tmpDir, "pom.xml", `
<project>
  <build>
    <plugins>
      <plugin>
        <executions>
          <execution>
            <phase>compile</phase>
          </execution>
        </executions>
      </plugin>
    </plugins>
  </build>
</project>
`)

	model := context.NewContextModelFromDir(tmpDir)

	tests := []struct {
		input    string
		expected []string
	}{
		{"npm", []string{"npm run dev", "npm run build"}},
		{"make", []string{"make build"}},
		{"mvn", []string{"mvn compile"}},
	}

	for _, tt := range tests {
		results, err := model.Predict(tt.input)
		if err != nil {
			t.Errorf("Predict(%q) returned error: %v", tt.input, err)
			continue
		}
		actual := extractTexts(results)
		for _, want := range tt.expected {
			if !contains(actual, want) {
				t.Errorf("Predict(%q) missing expected: %q (got %v)", tt.input, want, actual)
			}
		}
	}
}

func extractTexts(suggestions []model.Suggestion) []string {
	var texts []string
	for _, s := range suggestions {
		texts = append(texts, s.Text)
	}
	return texts
}

func contains(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}
