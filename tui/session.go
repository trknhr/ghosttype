package tui

import (
	"database/sql"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/trknhr/ghosttype/internal"
	"github.com/trknhr/ghosttype/model"
	"github.com/trknhr/ghosttype/model/ensemble"
)

type tuiModel struct {
	input          textinput.Model
	list           list.Model
	db             *sql.DB
	engine         *ensemble.Ensemble
	lastInput      string
	width          int
	height         int
	selected       string
	isLoadingHeavy bool
	lightResults   []model.Suggestion
	heavyResults   []model.Suggestion
}

// compactDelegate renders items in a single-line compact form.
type compactDelegate struct{}

func (d compactDelegate) Height() int                               { return 1 }
func (d compactDelegate) Spacing() int                              { return 0 }
func (d compactDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d compactDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(suggestionItem)
	if !ok {
		return
	}
	str := i.text
	if index == m.Index() {
		str = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("> " + str)
	} else {
		str = "  " + str
	}
	fmt.Fprint(w, str)
}

type suggestionItem struct{ text string }

func (i suggestionItem) Title() string       { return i.text }
func (i suggestionItem) Description() string { return "" }
func (i suggestionItem) FilterValue() string { return i.text }

func NewTuiModel(db *sql.DB, initialInput string, filterModels string) (*tuiModel, error) {
	input := textinput.New()
	input.Placeholder = "Type command prefix..."
	input.SetValue(initialInput)
	input.Focus()

	l := list.New([]list.Item{}, &compactDelegate{}, 40, 10)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)

	engine := internal.GenerateModel(db, filterModels)

	return &tuiModel{
		input:  input,
		list:   l,
		db:     db,
		engine: engine,
	}, nil
}

func (m *tuiModel) Init() tea.Cmd {
	return textinput.Blink
}

type suggestionsMsg struct {
	prefix      string
	suggestions []model.Suggestion
	isHeavy     bool
	err         error
}

// Progressive enhancementを使用した新しい関数
func fetchProgressiveSuggestionsCmd(engine *ensemble.Ensemble, prefix string) tea.Cmd {
	return func() tea.Msg {
		// NextPredictがensembleに実装されているかチェック
		// if ensemble, ok := engine.(interface {
		// 	NextPredict(string) (<-chan []model.Suggestion, error)
		// }); ok {
		resultChan, err := engine.NextPredict(prefix)
		if err != nil {
			return suggestionsMsg{prefix, nil, false, err}
		}

		// 最初の結果（軽量モデル）を返す
		// select {
		// case suggestions, ok := <-resultChan:
		// 	if !ok {
		// 		return suggestionsMsg{prefix, nil, false, fmt.Errorf("no suggestions received")}
		// 	}

		// 	logger.Debug("light model suggestions received for prefix '%s': %v", prefix, suggestions)
		// 	// 軽量モデルの結果を即座に返す
		// 	lightMsg := suggestionsMsg{prefix, suggestions, false, nil}

		// 	// 重いモデルの結果を待つgoroutineを開始
		// 	go func() {
		// 		if heavySuggestions, ok := <-resultChan; ok {
		// 			// 重いモデルの結果が来たら別のメッセージとして送信
		// 			// 注意: これはTea.Programに直接送信する方法が必要
		// 			// 実際の実装では、別のアプローチが必要かもしれません
		// 			_ = heavySuggestions
		// 		}
		// 	}()

		// 	return lightMsg
		// 	// default:
		// 	// 	logger.Debug("No suggestions yet for prefix '%s', waiting...", prefix)
		// 	// 	return suggestionsMsg{prefix, nil, true, nil}
		// 	// return suggestionsMsg{prefix, nil, false, fmt.Errorf("timeout waiting for suggestions")}
		// }
		// // }

		// // // フォールバック: 従来のPredict
		// suggestions, err := engine.Predict(prefix)
		// return suggestionsMsg{prefix, suggestions, false, err}
		suggestions, ok := <-resultChan
		if !ok {
			return suggestionsMsg{prefix, nil, false, fmt.Errorf("no suggestions received")}
		}

		// 軽量モデルの結果を返す
		// return suggestionsMsg{prefix, suggestions, false, nil}
		return tea.Batch(
			// 軽量モデルの結果を即座に返す
			func() tea.Msg {
				return suggestionsMsg{prefix, suggestions, false, nil}
			},
			// 重いモデルの結果を待つ
			func() tea.Msg {
				if heavySuggestions, ok := <-resultChan; ok && len(heavySuggestions) > 0 {
					return suggestionsMsg{prefix, heavySuggestions, true, nil}
				}
				return nil
			},
		)()

	}
}

func fetchSuggestionsCmd(engine model.SuggestModel, prefix string) tea.Cmd {
	return func() tea.Msg {
		suggestions, err := engine.Predict(prefix)
		return suggestionsMsg{prefix, suggestions, false, err}
	}
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEnter:
			if item, ok := m.list.SelectedItem().(suggestionItem); ok {
				m.selected = item.text
				m.input.SetValue(item.text)
				return m, tea.Quit
			}

		case tea.KeyUp, tea.KeyDown:
			m.input.Blur()

		default:
			if !m.input.Focused() {
				m.input.Focus()
			}
			m.input, _ = m.input.Update(msg)
		}
	// case suggestionsMsg:
	// 	if msg.prefix != m.lastInput {
	// 		// discard outdated suggestions
	// 		return m, nil
	// 	}
	// 	if msg.err != nil {
	// 		m.list.SetItems([]list.Item{suggestionItem{text: "Error: " + msg.err.Error()}})
	// 	} else {
	// 		items := make([]list.Item, 0, len(msg.suggestions))
	// 		for _, s := range msg.suggestions {
	// 			items = append(items, suggestionItem{text: s.Text})
	// 		}
	// 		m.list.SetItems(items)
	// 		m.list.ResetSelected()
	// 	}
	case suggestionsMsg:
		if msg.prefix != m.lastInput {
			// discard outdated suggestions
			return m, nil
		}

		// if msg.err != nil {
		// 	m.list.SetItems([]list.Item{suggestionItem{text: "Error: " + msg.err.Error()}})
		// 	m.isLoadingHeavy = false
		// } else {
		// 	items := make([]list.Item, 0, len(msg.suggestions))
		// 	for _, s := range msg.suggestions {
		// 		items = append(items, suggestionItem{text: s.Text})
		// 	}
		// 	m.list.SetItems(items)
		// 	m.list.ResetSelected()

		// 	if msg.isHeavy {
		// 		m.isLoadingHeavy = false
		// 	} else {
		// 		m.isLoadingHeavy = true
		// 	}
		// }
		// ... エラー処理 ...
		if msg.isHeavy {
			// 重いモデルの結果を保存
			m.heavyResults = msg.suggestions
			m.isLoadingHeavy = false
		} else {
			// 軽量モデルの結果を保存
			m.lightResults = msg.suggestions
			m.isLoadingHeavy = true
		}

		// ここで mergeSuggestions が呼ばれています
		mergedSuggestions := m.mergeSuggestions()
		items := make([]list.Item, 0, len(mergedSuggestions))
		for _, s := range mergedSuggestions {
			items = append(items, suggestionItem{text: s.Text})
		}
		m.list.SetItems(items)
		m.list.ResetSelected()

	}

	m.list, _ = m.list.Update(msg)

	prefix := strings.TrimSpace(m.input.Value())
	if prefix != m.lastInput {
		m.lastInput = prefix

		m.isLoadingHeavy = false
		m.lightResults = nil
		m.heavyResults = nil

		if prefix != "" {
			cmds = append(cmds, fetchProgressiveSuggestionsCmd(m.engine, prefix))
			// cmds = append(cmds, fetchSuggestionsCmd(m.engine, prefix))
		} else {
			m.list.SetItems([]list.Item{})
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *tuiModel) View() string {
	s := "Ghosttype\n\n"
	if m.isLoadingHeavy {
		s += " (enhancing...)"
	}
	s += m.input.View() + "\n\n"
	s += m.list.View() + "\n"
	s += "(q = Ctrl+C)"
	return s
}

func (m *tuiModel) SelectedText() string {
	return m.selected
}

func (m *tuiModel) mergeSuggestions() []model.Suggestion {
	// 軽量と重量の結果をスコアベースでマージ
	scoreMap := make(map[string]float64)

	// 軽量モデルの結果を追加（重み: 1.0）
	for _, s := range m.lightResults {
		scoreMap[s.Text] += s.Score * 1.0
	}

	// 重いモデルの結果を追加（重み: 1.5で高品質を重視）
	for _, s := range m.heavyResults {
		scoreMap[s.Text] += s.Score * 1.5
	}

	// スコア順にソート
	type ranked struct {
		Text  string
		Score float64
	}

	rankedList := make([]ranked, 0, len(scoreMap))
	for text, score := range scoreMap {
		rankedList = append(rankedList, ranked{text, score})
	}

	// スコアの降順でソート
	for i := 0; i < len(rankedList); i++ {
		for j := i + 1; j < len(rankedList); j++ {
			if rankedList[i].Score < rankedList[j].Score {
				rankedList[i], rankedList[j] = rankedList[j], rankedList[i]
			}
		}
	}

	// model.Suggestionの形式で返す
	results := make([]model.Suggestion, len(rankedList))
	for i, r := range rankedList {
		results[i] = model.Suggestion{
			Text:  r.Text,
			Score: r.Score,
		}
	}

	return results
}
