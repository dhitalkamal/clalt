// an invented summary reply: two accounts, one project each.
const stats = (o) => ({ sessions: 0, tokens: 0, cost: 0, activeMinutes: 0, toolCalls: 0, subagents: [], tools: [], files: [], failureCount: 0, failures: [], ...o });

export function sample() {
  const alpha = stats({
    sessions: 1, tokens: 540, cost: 0, activeMinutes: 5, toolCalls: 4,
    subagents: [{ name: 'reviewer', count: 2 }, { name: 'explorer', count: 1 }],
    tools: [{ name: 'Edit', count: 2 }, { name: 'Bash', count: 1 }, { name: 'Read', count: 1 }],
    files: [{ name: 'src/a.go', count: 2 }, { name: 'README.md', count: 1 }],
    failureCount: 1, failures: [{ tool: 'Bash', text: 'boom: build failed', count: 1 }],
  });
  const beta = stats({ sessions: 1, tokens: 1500, cost: 0.0105, activeMinutes: 0, toolCalls: 1, tools: [{ name: 'Write', count: 1 }], files: [{ name: 'notes.txt', count: 1 }] });
  return {
    v: 1, day: '2026-10-27', from: '2026-10-27T00:00:00-07:00', to: '2026-10-28T00:00:00-07:00', truncated: false,
    totals: stats({ sessions: 2, tokens: 1700, cost: 0.0148, activeMinutes: 65, toolCalls: 5 }),
    accounts: [
      { id: 'personal', label: 'PERSONAL', projects: [{ name: 'beta', ...beta }], ...beta },
      { id: 'work', label: 'WORK', projects: [{ name: 'alpha', ...alpha }], ...alpha },
    ],
  };
}

export function empty() {
  return { v: 1, day: '2026-10-27', truncated: false, totals: stats({}), accounts: [] };
}

// a history reply: two accounts over 4 days.
export function history() {
  const d = (day, tokens, p5, p7, cost = 0) => ({ day, tokens, cost, messages: tokens ? 3 : 0, peak5hTokens: p5, peak7dTokens: p7, peak5hPct: null, peak7dPct: null });
  return {
    v: 1, days: 4, from: '2026-10-24', to: '2026-10-27', truncated: false,
    accounts: [
      { id: 'personal', label: 'PERSONAL', days: [d('2026-10-24', 0, 0, 0), d('2026-10-25', 7, 7, 7), d('2026-10-26', 0, 0, 7), d('2026-10-27', 0, 0, 7)] },
      { id: 'work', label: 'WORK', days: [d('2026-10-24', 1200000, 800000, 1200000, 3.5), d('2026-10-25', 0, 0, 1200000), d('2026-10-26', 300, 300, 1200300), d('2026-10-27', 90000, 50000, 1290300, 0.4)] },
    ],
  };
}
