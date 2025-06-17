package setting

import (
	"path/filepath"
	"strings"

	"github.com/trknhr/ghosttype/internal/model/entity"
	"github.com/trknhr/ghosttype/internal/utils"
)

type ContextModel struct {
	Commands []string
}

func NewContextModelFromDir(projectRoot string) entity.SuggestModel {
	cmds := []string{}

	if npmCmds, err := utils.ExtractNpmScripts(filepath.Join(projectRoot, "package.json")); err == nil {
		cmds = append(cmds, npmCmds...)
	}
	if makeCmds, err := utils.ExtractMakeTargets(filepath.Join(projectRoot, "Makefile")); err == nil {
		cmds = append(cmds, makeCmds...)
	}
	if mvnCmds, err := utils.ExtractMavenTargets(filepath.Join(projectRoot, "pom.xml")); err == nil {
		cmds = append(cmds, mvnCmds...)
	}

	return &ContextModel{Commands: cmds}
}

func (m *ContextModel) Learn(_ []string) error {
	// no-op: context is static
	return nil
}

func (m *ContextModel) Predict(input string) ([]entity.Suggestion, error) {
	var out []entity.Suggestion
	for _, cmd := range m.Commands {
		if strings.HasPrefix(cmd, input) {
			out = append(out, entity.Suggestion{
				Text:   cmd,
				Score:  1,
				Source: "context",
			})
		}
	}
	return out, nil
}

func (m *ContextModel) Weight() float64 {
	return 1
}
