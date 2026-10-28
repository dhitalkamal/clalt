// number and date formatting of the summary panel. no dom here, so node can test it.

const UNITS = ['', 'k', 'M', 'B'];

// 950, 1.2k, 123k, 2.5M. under 100 of a unit keeps one decimal, no trailing .0
export function fmtTokens(n) {
  let v = Number.isFinite(n) && n > 0 ? n : 0;
  let u = 0;
  while (v >= 1000 && u < UNITS.length - 1) { v /= 1000; u++; }
  let s = v < 100 && u > 0 ? v.toFixed(1) : String(Math.round(v));
  if (s.endsWith('.0')) s = s.slice(0, -2);
  // 999.9k rounds up to 1000k: say 1M
  if (Number(s) >= 1000 && u < UNITS.length - 1) return fmtTokens(Math.round(Number(s)) * Math.pow(1000, u));
  return s + UNITS[u];
}

export function fmtCost(c) {
  const v = Number.isFinite(c) && c > 0 ? c : 0;
  if (v > 0 && v < 0.005) return '<$0.01';
  return '$' + v.toFixed(2);
}

export function fmtMinutes(m) {
  const v = Number.isFinite(m) && m > 0 ? Math.round(m) : 0;
  if (v < 60) return v + 'm';
  return Math.floor(v / 60) + 'h ' + String(v % 60).padStart(2, '0') + 'm';
}

const DAY = /^(\d{4})-(\d{2})-(\d{2})$/;

function parts(day) {
  const m = typeof day === 'string' && DAY.exec(day);
  if (!m) return null;
  const [y, mo, d] = [Number(m[1]), Number(m[2]), Number(m[3])];
  const t = new Date(Date.UTC(y, mo - 1, d));
  return t.getUTCFullYear() === y && t.getUTCMonth() === mo - 1 && t.getUTCDate() === d ? t : null;
}

export function validDay(day) { return parts(day) !== null; }

const iso = (t) => t.toISOString().slice(0, 10);

// the calendar day delta days from day, null for something that is not a day
export function shiftDay(day, delta) {
  const t = parts(day);
  if (!t) return null;
  t.setUTCDate(t.getUTCDate() + delta);
  return iso(t);
}

// the local calendar day of a date, as the server cuts days in local time
export function localDay(d = new Date()) {
  const p = (n) => String(n).padStart(2, '0');
  return d.getFullYear() + '-' + p(d.getMonth() + 1) + '-' + p(d.getDate());
}

const WEEK = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
const MONTH = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

// Today, Tue 27 Oct / Yesterday, Mon 26 Oct / Tue 20 Oct / Wed 31 Dec 2025
export function dayLabel(day, today) {
  const t = parts(day);
  if (!t) return String(day);
  let s = WEEK[t.getUTCDay()] + ' ' + t.getUTCDate() + ' ' + MONTH[t.getUTCMonth()];
  const now = parts(today);
  if (now && now.getUTCFullYear() !== t.getUTCFullYear()) s += ' ' + t.getUTCFullYear();
  if (day === today) return 'Today, ' + s;
  if (day === shiftDay(today, -1)) return 'Yesterday, ' + s;
  return s;
}

// 10-07, for an axis
export function shortDay(day) { return typeof day === 'string' ? day.slice(5) : ''; }

// colour step of the limit meter: amber above 60, red above 90
export function limitTone(v) { return v > 90 ? 'hot' : v > 60 ? 'warn' : 'ok'; }

// a percent as a whole number inside 0..100
export function meterPct(v) { return Number.isFinite(v) ? Math.min(100, Math.max(0, Math.round(v))) : 0; }
