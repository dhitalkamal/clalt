// which tool an account or session belongs to, and the fixed look of the Copilot team. pure, no dom.
// the id has a "~" because clalt account names can never contain one, so it cannot clash with a real account.
export const COPILOT_ID = '~copilot';
export const COPILOT_LABEL = 'Copilot';
// fuchsia: outside the claude palette (model.js) and at least 25 apart from each entry in CIELAB
export const COPILOT_TINT = 0xd04fb0;

// a record is an account {id, tool}, a session {account, tool} or a bare tool string holder
export const isCopilot = (x) => !!x && (x.tool === 'copilot' || x.id === COPILOT_ID || x.account === COPILOT_ID);
export const toolName = (x) => (isCopilot(x) ? 'copilot' : 'claude');

// the text for an account id: the snapshot label, Copilot for the team even if its record is missing
export function accountLabel(accounts, id) {
  const info = accounts && accounts.get ? accounts.get(id) : null;
  if (info && info.label) return info.label;
  if (id === COPILOT_ID) return COPILOT_LABEL;
  return id || '-';
}

// the accounts that have claude usage: everything but the Copilot team
export const claudeOnly = (list) => (Array.isArray(list) ? list : []).filter((a) => !isCopilot(a));

// the cabin owners the layout editor offers: every account, the Copilot team by its label
export const editorAccounts = (accounts) => (accounts && accounts.entries ? [...accounts.entries()] : [])
  .map(([id, a]) => ({ id, name: accountLabel(accounts, id) || (a && a.label) }));
