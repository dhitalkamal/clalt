package office

// the tools a session can belong to. an empty Tool reads as claude.
const (
	ToolClaude  = "claude"
	ToolCopilot = "copilot"
)

// CopilotAccount is the reserved id of the Copilot team. "~" can never be in a
// clalt account name, so it cannot clash with a real account.
const CopilotAccount = "~copilot"

// copilotLabel is what the Copilot team is called on the page.
const copilotLabel = "Copilot"

// copilotWhy is why a Copilot session has no actions.
const copilotWhy = "a Copilot session: clalt only shows it"

func toolOf(t string) string {
	if t == ToolCopilot {
		return ToolCopilot
	}
	return ToolClaude
}
