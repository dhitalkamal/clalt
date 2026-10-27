// the day as plain text for the clipboard: ascii only, lines a person can paste anywhere.
import { summaryModel } from './model.js';

// anything outside printable ascii becomes ?, so a paste never carries surprises
export function ascii(s) {
  let out = '';
  for (const ch of String(s)) {
    const c = ch.codePointAt(0);
    out += c === 9 || c === 10 || (c >= 0x20 && c <= 0x7e) ? ch : '?';
  }
  return out;
}

export function summaryText(sum) {
  const m = summaryModel(sum);
  const out = ['clalt summary for ' + m.day];
  if (m.empty) {
    out.push('nothing happened on this day.');
  } else {
    out.push('total: ' + m.line);
    if (m.truncated) out.push('note: some transcripts were not read (size limit), numbers may be low.');
    for (const a of m.accounts) {
      out.push('', a.title + ': ' + a.line);
      if (a.limit) out.push('  ' + a.limit);
      for (const p of a.projects) {
        out.push('  ' + p.title + ': ' + p.line);
        if (p.subagents) out.push('    subagents: ' + p.subagents);
        if (p.tools) out.push('    tools: ' + p.tools);
        if (p.files) out.push('    files: ' + p.files);
        if (p.failureCount) {
          out.push('    failures: ' + p.failureCount);
          for (const f of p.failures) out.push('      ' + f);
        }
      }
    }
  }
  return ascii(out.join('\n') + '\n');
}
