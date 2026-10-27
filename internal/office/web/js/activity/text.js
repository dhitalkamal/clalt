// small text helpers for the activity panel. pure, no dom.
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const two = (n) => String(n).padStart(2, '0');

// cuts to maxLines first, then maxChars. newlines inside the kept part are untouched.
// hidden is how many characters were left out.
export function truncateText(text, { maxChars = 1200, maxLines = 20 } = {}) {
  const s = typeof text === 'string' ? text : '';
  let out = s;
  const lines = out.split('\n');
  if (lines.length > maxLines) out = lines.slice(0, maxLines).join('\n');
  if (out.length > maxChars) out = out.slice(0, maxChars);
  return { text: out, cut: out.length < s.length, hidden: s.length - out.length };
}

// first non-blank line, trimmed, at most max characters including the ellipsis
export function snippet(text, max) {
  const line = String(text || '').split('\n').map((l) => l.trim()).find((l) => l) || '';
  return line.length > max ? line.slice(0, Math.max(0, max - 3)) + '...' : line;
}

export function formatDuration(ms) {
  if (!(ms > 0)) return '';
  if (ms < 1000) return Math.round(ms) + 'ms';
  if (ms < 10000) return (ms / 1000).toFixed(1) + 's';
  if (ms < 60000) return Math.round(ms / 1000) + 's';
  const mins = Math.floor(ms / 60000);
  if (mins < 60) return mins + 'm ' + two(Math.floor((ms % 60000) / 1000)) + 's';
  return Math.floor(mins / 60) + 'h ' + two(mins % 60) + 'm';
}

// 14:03:09 for today, "Oct 26 09:05" for any other day. local time. empty when the stamp is not a date.
export function clock(iso, now = new Date()) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const hm = two(d.getHours()) + ':' + two(d.getMinutes());
  if (d.toDateString() === now.toDateString()) return hm + ':' + two(d.getSeconds());
  return MONTHS[d.getMonth()] + ' ' + d.getDate() + ' ' + hm;
}

const tail = (p) => {
  const segs = String(p).split(/[\\/]/).filter(Boolean);
  return segs.length > 3 ? '.../' + segs.slice(-3).join('/') : String(p);
};

// text is the first path shortened plus a count of the rest, title lists every path
export function fileSummary(files) {
  const list = Array.isArray(files) ? files.filter((f) => typeof f === 'string' && f) : [];
  if (!list.length) return { text: '', title: '' };
  return { text: tail(list[0]) + (list.length > 1 ? ' +' + (list.length - 1) : ''), title: list.join('\n') };
}
