package cli

// hookEvents are the claude hook events that clalt run installs for a session.
// timeout is in seconds. SessionEnd gets 1 so that it stays inside claude's
// default 1.5 second budget for those hooks: a longer timeout would raise the
// budget for the user's own SessionEnd hooks too. an async hook cannot block
// claude and has no timeout of its own, so the tool events, which fire on every
// tool call, are async.
var hookEvents = []struct {
	name    string
	matcher string
	timeout int
	async   bool
}{
	{name: "SessionStart", timeout: 5},
	{name: "UserPromptSubmit", timeout: 5},
	{name: "Stop", timeout: 5},
	{name: "StopFailure", timeout: 5},
	// only the notifications that mean claude is blocked on the user, so a
	// reply does not start a process for every other kind
	{name: "Notification", matcher: "permission_prompt|elicitation_dialog|elicitation_url_dialog|agent_needs_input", timeout: 5},
	{name: "SessionEnd", timeout: 1},
	// these two only end needs-you, see feed.ApplyHook
	{name: "PostToolUse", async: true},
	{name: "PostToolUseFailure", async: true},
	// subagent starts and stops, and the Agent tool call (Task is its old
	// name) that only supplies the short description. the matcher keeps this
	// from running for any other tool. sync, so that the description is
	// stored before the SubagentStart that claims it.
	{name: "SubagentStart", timeout: 5},
	{name: "SubagentStop", timeout: 5},
	{name: "PreToolUse", matcher: "Agent|Task", timeout: 5},
}
