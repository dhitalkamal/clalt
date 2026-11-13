# Copilot CLI in the office

Copilot CLI sessions show up by themselves while the office runs, in their own
Copilot team. There is no setup command. The office reads the files Copilot
already writes: `session-state/<id>/events.jsonl` and `workspace.yaml` under
`$COPILOT_HOME`, or `~/.copilot` when that is not set. Only sessions whose
events file changed in the last 6 hours are looked at.

- busy: a prompt was sent, a turn started or a tool is running, and the turn
  has not ended yet.
- needs you: a permission request is open. Several can be open at once, the
  session keeps waving until every one is answered, or the turn ends or is
  cancelled. clalt keeps only the request ids, never the command text.
- idle: the last turn ended, or the session just opened.
- gone: the session shut down, or has been quiet for 6 hours.

clalt only reads those two files, and only the tail of a big events file. It
never opens Copilot's login or settings (`config.json`, `providers.json`,
`mcp-config.json`, `permissions-config.json`, `hooks/`), writes nothing in
Copilot's folder, and keeps no prompt or reply text: only the id, folder,
model, state and times. The model shown is the one Copilot's auto mode
resolved to, or "auto" until it has picked one; Copilot's internal helper
models are ignored. It shows no Copilot usage or limits, and cannot move,
focus or stop a Copilot session.

