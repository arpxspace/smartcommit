package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"smartcommit/internal/ai"
	"smartcommit/internal/config"
	"smartcommit/internal/git"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SessionState int

const (
	StateLoading SessionState = iota
	StateAnalysis
	StateHistoryAnalysis
	StateQuestioning
	StateReview
	StateCommit
	StateError
	StateSuccess
	StateSetup
	StateNoRepo
	StateWelcome
	StateDiffTooLarge
)

type SetupStep int

const (
	SetupStepProvider SetupStep = iota
	SetupStepOpenAIKey
	SetupStepConfirmOpenAIKey
	SetupStepOllamaURL
	SetupStepOllamaModel
)

type Model struct {
	State            SessionState
	Spinner          spinner.Model
	TextArea         textarea.Model
	Viewport         viewport.Model
	Err              error
	Config           *config.Config
	AIClient         ai.Provider
	Diff             string
	History          string
	HistoryCtx       []string
	Questions        []string
	Answers          map[string]string
	CurrentQIdx      int
	CommitMsg        string
	SetupStep        SetupStep
	SelectedProvider config.ProviderType
	Width            int
	Height           int
}

func NewModel() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	ta := textarea.New()
	ta.Placeholder = "Type your answer here..."
	ta.Focus()

	vp := viewport.New(80, 20)

	return Model{
		State:    StateLoading,
		Spinner:  s,
		TextArea: ta,
		Viewport: vp,
		Answers:  make(map[string]string),
		Width:    80, // Default width
		Height:   24, // Default height
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.Spinner.Tick,
		checkPrerequisitesCmd,
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Viewport.Width = msg.Width
		m.Viewport.Height = msg.Height
		m.TextArea.SetWidth(msg.Width - 4) // Adjust textarea width too
	case tea.KeyMsg:

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.State != StateQuestioning && m.State != StateReview && m.State != StateSetup && m.State != StateWelcome && m.State != StateDiffTooLarge {
				return m, tea.Quit
			}
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd
	case errMsg:
		m.Err = msg
		m.State = StateError
		return m, nil
	case diffTooLargeMsg:
		m.State = StateDiffTooLarge
		return m, nil
	case prerequisitesCheckedMsg:
		m.Config = msg.Config
		client, err := ai.NewClient(m.Config)
		if err != nil {
			return m, func() tea.Msg { return errMsg(err) }
		}
		m.AIClient = client
		m.Diff = msg.Diff
		m.History = msg.History
		// Transition to Welcome screen instead of History Analysis
		m.State = StateWelcome
		return m, nil
	case historyAnalysisResultMsg:
		m.HistoryCtx = msg.KeyContext
		m.State = StateAnalysis
		return m, analyzeChangesCmd(m.AIClient, m.Diff, m.History)
	case analysisResultMsg:
		m.Questions = msg.Questions
		if len(m.Questions) == 0 {
			return m, generateCommitMsgCmd(m.AIClient, m.Diff, m.History, m.HistoryCtx, m.Answers)
		}
		m.State = StateQuestioning
		m.TextArea.Focus()
		return m, nil
	case commitMsgGeneratedMsg:
		m.CommitMsg = msg.Message
		m.State = StateCommit
		return m, commitCmd(m.CommitMsg)
	case commitSuccessMsg:
		m.State = StateSuccess
		return m, tea.Quit
	case setupRequiredMsg:
		m.Config = msg.Config
		m.State = StateSetup
		return m, nil
	case noRepoMsg:
		m.State = StateNoRepo
		return m, nil
	}

	// Handle state-specific updates
	switch m.State {
	case StateDiffTooLarge:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "m", "enter":
				// Manual Mode
				m.CommitMsg = ""
				m.State = StateCommit
				return m, commitCmd(m.CommitMsg)
			case "q", "ctrl+c":
				return m, tea.Quit
			}
		}
	case StateWelcome:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "1", "enter":
				// AI Mode
				m.State = StateHistoryAnalysis
				return m, analyzeHistoryCmd(m.AIClient, m.Diff, m.History)
			case "2":
				// Manual Mode
				m.CommitMsg = "" // Empty message triggers manual editor
				m.State = StateCommit
				return m, commitCmd(m.CommitMsg)
			case "c", "C":
				// Reconfigure provider
				m.State = StateSetup
				m.SetupStep = SetupStepProvider
				return m, nil
			}
		}
	case StateSetup:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch m.SetupStep {
			case SetupStepProvider:
				// Provider selection
				switch msg.String() {
				case "1":
					m.SelectedProvider = config.ProviderOpenAI
					// Check for env var
					if os.Getenv("OPENAI_API_KEY") != "" {
						m.SetupStep = SetupStepConfirmOpenAIKey
					} else {
						m.SetupStep = SetupStepOpenAIKey
					}
					m.TextArea.Reset()
					return m, nil
				case "2":
					m.SelectedProvider = config.ProviderOllama
					m.SetupStep = SetupStepOllamaURL
					m.TextArea.Reset()
					m.TextArea.SetValue("http://localhost:11434") // Default
					return m, nil
				}
			case SetupStepConfirmOpenAIKey:
				switch strings.ToLower(msg.String()) {
				case "y", "enter":
					m.Config.Provider = config.ProviderOpenAI
					m.Config.OpenAIAPIKey = os.Getenv("OPENAI_API_KEY")
					if err := m.Config.Save(); err != nil {
						m.Err = err
						m.State = StateError
						return m, nil
					}
					return m, checkPrerequisitesCmd
				case "n":
					m.SetupStep = SetupStepOpenAIKey
					m.TextArea.Reset()
					return m, nil
				}
			case SetupStepOpenAIKey:
				if msg.Type == tea.KeyEnter {
					input := strings.TrimSpace(m.TextArea.Value())
					if input != "" {
						m.Config.Provider = config.ProviderOpenAI
						m.Config.OpenAIAPIKey = input
						if err := m.Config.Save(); err != nil {
							m.Err = err
							m.State = StateError
							return m, nil
						}
						m.TextArea.Reset()
						return m, checkPrerequisitesCmd
					}
				}
			case SetupStepOllamaURL:
				if msg.Type == tea.KeyEnter {
					input := strings.TrimSpace(m.TextArea.Value())
					if input != "" {
						m.Config.OllamaURL = input
						m.SetupStep = SetupStepOllamaModel
						m.TextArea.Reset()
						m.TextArea.SetValue("llama3.1") // Default
						return m, nil
					}
				}
			case SetupStepOllamaModel:
				if msg.Type == tea.KeyEnter {
					input := strings.TrimSpace(m.TextArea.Value())
					if input != "" {
						m.Config.Provider = config.ProviderOllama
						m.Config.OllamaModel = input
						if err := m.Config.Save(); err != nil {
							m.Err = err
							m.State = StateError
							return m, nil
						}
						m.TextArea.Reset()
						return m, checkPrerequisitesCmd
					}
				}
			}
		}
		m.TextArea, cmd = m.TextArea.Update(msg)
		return m, cmd
	case StateQuestioning:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.Type == tea.KeyEnter {
				// Submit the current answer
				answer := strings.TrimSpace(m.TextArea.Value())
				if answer != "" {
					m.Answers[m.Questions[m.CurrentQIdx]] = answer
					m.CurrentQIdx++
					m.TextArea.Reset()
					m.TextArea.Focus()

					// Check if we've answered all questions
					if m.CurrentQIdx >= len(m.Questions) {
						m.State = StateLoading
						return m, generateCommitMsgCmd(m.AIClient, m.Diff, m.History, m.HistoryCtx, m.Answers)
					}
					return m, nil
				}
			}
		}
		m.TextArea, cmd = m.TextArea.Update(msg)
		return m, cmd
	case StateNoRepo:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}
	}

	return m, cmd
}

func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)

	if m.Err != nil {
		errStr := m.Err.Error()
		// Check for Ollama model not found error
		if m.Config != nil && m.Config.Provider == config.ProviderOllama &&
			strings.Contains(errStr, "404") && strings.Contains(errStr, "model") && strings.Contains(errStr, "not found") {

			modelName := m.Config.OllamaModel
			cmd := fmt.Sprintf("ollama pull %s", modelName)
			cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)

			return fmt.Sprintf(
				"\n %s Model '%s' not found.\n\n You don't have this model installed through Ollama.\n To install it, run the following command:\n\n   %s\n\n Press ctrl+c to quit.\n",
				errorStyle.Render("Error:"),
				modelName,
				cmdStyle.Render(cmd),
			)
		}
		return fmt.Sprintf("%s %v\nPress ctrl+c to quit.", errorStyle.Render("Error:"), m.Err)
	}

	switch m.State {
	case StateLoading:
		return fmt.Sprintf("\n %s Checking prerequisites...\n\n", m.Spinner.View())
	case StateDiffTooLarge:
		return fmt.Sprintf(`
 %s

 The staged changes are too large for AI analysis.
 (> 40k characters)

 You can:
 1. Press 'm' or Enter to write the commit message manually.
 2. Press 'q' to quit and stage fewer changes.

`, errorStyle.Render("Warning: Large Diff Detected"))
	case StateWelcome:
		providerInfo := ""
		if m.Config != nil {
			if m.Config.Provider == config.ProviderOpenAI {
				providerInfo = infoStyle.Render(" (using OpenAI)")
			} else if m.Config.Provider == config.ProviderOllama {
				providerInfo = infoStyle.Render(fmt.Sprintf(" (using Ollama: %s)", m.Config.OllamaModel))
			}
		}
		return fmt.Sprintf(`
 %s%s

 How would you like to proceed?

 1. I need help writing a commit message (Recommended)
 2. I already know what to write

 %s
 (Press 1 or 2)
`, titleStyle.Render("SmartCommit"), providerInfo, infoStyle.Render("Press 'c' to reconfigure provider"))
	case StateSetup:
		switch m.SetupStep {
		case SetupStepProvider:
			return fmt.Sprintf(`

 Choose your AI provider:

 1. OpenAI (GPT-4o)
    %s

 2. Ollama (llama3.1)
    %s

 (Press 1 or 2)
`,
				infoStyle.Faint(true).Render("Not private, costs money, great accuracy/performance"),
				infoStyle.Faint(true).Render("Private, free, low accuracy/performance"),
			)
		case SetupStepConfirmOpenAIKey:
			return fmt.Sprintf(
				"\n %s\n\n Found OPENAI_API_KEY in your environment.\n Would you like to use it?\n\n %s\n",
				titleStyle.Render("OpenAI API Key Detected"),
				infoStyle.Render("(y/n)"),
			)
		case SetupStepOpenAIKey:
			return fmt.Sprintf(
				"\n %s\n\n%s\n\n%s\n",
				titleStyle.Render("Please enter your OpenAI API Key:"),
				m.TextArea.View(),
				infoStyle.Render("(Press Enter to save)"),
			)
		case SetupStepOllamaURL:
			return fmt.Sprintf(
				"\n %s\n\n%s\n\n%s\n",
				titleStyle.Render("Please enter your Ollama URL:"),
				m.TextArea.View(),
				infoStyle.Render("(Press Enter to continue)"),
			)
		case SetupStepOllamaModel:
			return fmt.Sprintf(
				"\n %s\n\n%s\n\n%s\n",
				titleStyle.Render("Please enter the Ollama model name:"),
				m.TextArea.View(),
				infoStyle.Render("(Press Enter to save)"),
			)
		}
		return "\n Setup...\n\n"
	case StateNoRepo:
		return fmt.Sprintf("\n %s Not a git repository.\n\n Please run smartcommit inside a git repository.\n Press q to quit.\n\n", errorStyle.Render("Error:"))
	case StateHistoryAnalysis:
		return fmt.Sprintf("\n %s Analyzing history context...\n\n", m.Spinner.View())
	case StateAnalysis:
		return fmt.Sprintf("\n %s Analyzing changes and generating questions...\n\n", m.Spinner.View())
	case StateQuestioning:
		if m.CurrentQIdx < len(m.Questions) {
			// Use dynamic width, defaulting to 70 if width is small or not set
			wrapWidth := m.Width - 10
			if wrapWidth < 40 {
				wrapWidth = 40
			}
			questionStyle := lipgloss.NewStyle().Width(wrapWidth)
			return fmt.Sprintf(
				"\n%s %s\n\n%s\n\n%s\n",
				titleStyle.Render(fmt.Sprintf("Question %d/%d:", m.CurrentQIdx+1, len(m.Questions))),
				questionStyle.Render(m.Questions[m.CurrentQIdx]),
				m.TextArea.View(),
				infoStyle.Render("(Press Enter to submit)"),
			)
		}
	case StateReview:
		// Deprecated state, should not be reached
		return ""
	case StateCommit:
		return "\n Opening editor...\n\n"
	case StateSuccess:
		successMsg := "Successfully committed!\n\n"
		cta := infoStyle.Render("If you're enjoying smartcommit, give us a star on GitHub: https://github.com/arpxspace/smartcommit")
		return successMsg + cta + "\n\n"
	}

	return "\n Unknown state\n\n"
}

// Messages and Commands

type errMsg error

type diffTooLargeMsg struct{}

type prerequisitesCheckedMsg struct {
	Config  *config.Config
	Diff    string
	History string
}

type setupRequiredMsg struct {
	Config *config.Config
}

type noRepoMsg struct{}

type historyAnalysisResultMsg struct {
	KeyContext []string
}

type analysisResultMsg struct {
	Questions []string
}

type commitMsgGeneratedMsg struct {
	Message string
}

type commitSuccessMsg struct{}

func checkPrerequisitesCmd() tea.Msg {
	cfg, err := config.Load()
	if err != nil {
		return errMsg(err)
	}

	// Check if setup is needed - validate provider-specific requirements
	needsSetup := false
	if cfg.Provider == "" {
		needsSetup = true
	} else if cfg.Provider == config.ProviderOpenAI {
		// For OpenAI, check config first, then fall back to env var
		if cfg.OpenAIAPIKey == "" {
			envKey := os.Getenv("OPENAI_API_KEY")
			if envKey != "" {
				// Use env var and save it to config for consistency
				cfg.OpenAIAPIKey = envKey
				cfg.Save() // Ignore error, not critical
			} else {
				needsSetup = true
			}
		}
	} else if cfg.Provider == config.ProviderOllama && (cfg.OllamaURL == "" || cfg.OllamaModel == "") {
		needsSetup = true
	}

	if needsSetup {
		return setupRequiredMsg{Config: cfg}
	}

	if !git.IsRepo() {
		return noRepoMsg{}
	}

	diff, err := git.GetStagedDiff()
	if err != nil {
		return errMsg(err)
	}
	if strings.TrimSpace(diff) == "" {
		return errMsg(fmt.Errorf("no staged changes found"))
	}

	// Warn if diff is too large (approx 12k chars ~ 3-4k tokens)
	if len(diff) > 40000 { // ~10k tokens, safety limit
		return diffTooLargeMsg{}
	}

	history, err := git.GetRecentHistory(10) // Get last 10 commits
	if err != nil {
		return errMsg(err)
	}

	return prerequisitesCheckedMsg{
		Config:  cfg,
		Diff:    diff,
		History: history,
	}
}

func analyzeHistoryCmd(client ai.Provider, diff, history string) tea.Cmd {
	return func() tea.Msg {
		analysis, err := client.AnalyzeHistory(context.Background(), diff, history)
		if err != nil {
			return errMsg(err)
		}
		return historyAnalysisResultMsg{KeyContext: analysis.KeyContext}
	}
}

func analyzeChangesCmd(client ai.Provider, diff, history string) tea.Cmd {
	return func() tea.Msg {
		questions, err := client.GenerateQuestions(context.Background(), diff, history)
		if err != nil {
			return errMsg(err)
		}
		return analysisResultMsg{Questions: questions}
	}
}

func generateCommitMsgCmd(client ai.Provider, diff, history string, historyCtx []string, answers map[string]string) tea.Cmd {
	return func() tea.Msg {
		fullHistoryContext := history
		if len(historyCtx) > 0 {
			fullHistoryContext += "\n\nKey Context from History:\n- " + strings.Join(historyCtx, "\n- ")
		}

		msg, err := client.GenerateCommitMessage(context.Background(), diff, fullHistoryContext, answers)
		if err != nil {
			return errMsg(err)
		}
		return commitMsgGeneratedMsg{Message: msg}
	}
}

func commitCmd(msg string) tea.Cmd {
	c := git.CommitCmd(msg)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return errMsg(err)
		}
		return commitSuccessMsg{}
	})
}
