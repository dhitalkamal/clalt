import test from 'node:test';
import assert from 'node:assert/strict';
import { pctModel, pctReadout, pctSeries, pctNote, PCT_GUIDE } from '../summary/pctchart.js';
import { limitLine } from '../summary/model.js';
import { summaryText } from '../summary/copytext.js';
import { sample } from './summary-fixture.js';

const days = [
  { day: '2026-10-24', peak5hPct: null, peak7dPct: null },
  { day: '2026-10-25', peak5hPct: 40, peak7dPct: 10 },
  { day: '2026-10-26', peak5hPct: 95.5, peak7dPct: 12 },
  { day: '2026-10-27', peak5hPct: 130, peak7dPct: null },
];

test('pctModel scales 0 to 100 and puts the guide at 90 percent', () => {
  const m = pctModel({ days, key: 'peak5hPct', width: 400, height: 200 });
  assert.equal(m.top, 100);
  assert.deepEqual(m.yTicks.map((t) => t.value), [0, 25, 50, 75, 100]);
  assert.equal(m.yTicks[0].label, '0%');
  assert.equal(PCT_GUIDE, 90);
  const p = m.plot;
  assert.ok(Math.abs(m.guide.y - (p.y + p.h * 0.1)) < 0.001);
  assert.ok(Math.abs(m.bars[1].h - p.h * 0.4) < 0.001);
  // above 100 is clamped to the top
  assert.ok(Math.abs(m.bars[3].h - p.h) < 0.001);
});

test('pctModel marks days with no reading instead of drawing zero', () => {
  const m = pctModel({ days, key: 'peak7dPct', width: 400, height: 200 });
  assert.equal(m.bars[0].missing, true);
  assert.equal(m.bars[0].h, 0);
  assert.equal(m.bars[3].missing, true);
  assert.equal(m.bars[1].missing, false);
});

test('pctModel marks the bars over the guide', () => {
  const m = pctModel({ days, key: 'peak5hPct', width: 400, height: 200 });
  assert.deepEqual(m.bars.map((b) => b.hot), [false, false, true, true]);
});

test('pctModel keeps bars inside the plot at 90 days', () => {
  const many = Array.from({ length: 90 }, (_, i) => ({ day: new Date(Date.UTC(2026, 6, 1 + i)).toISOString().slice(0, 10), peak5hPct: i }));
  const m = pctModel({ days: many, key: 'peak5hPct', width: 340, height: 180 });
  for (const b of m.bars) assert.ok(b.x >= m.plot.x - 0.001 && b.x + b.w <= m.plot.x + m.plot.w + 0.001);
});

test('pctReadout reads a day or says nothing was read', () => {
  assert.equal(pctReadout(days[1], 'peak5hPct', '5h'), '10-25: 5h peak 40%');
  assert.equal(pctReadout(days[0], 'peak5hPct', '5h'), '10-24: no 5h reading');
  assert.equal(pctReadout(days[2], 'peak5hPct', '5h'), '10-26: 5h peak 96%, over the 90% line');
});

test('pctSeries tells whether readings exist and where they began', () => {
  assert.deepEqual(pctSeries(days, 'peak5hPct'), { any: true, firstDay: '2026-10-25', gaps: true });
  assert.deepEqual(pctSeries(days.slice(1, 3), 'peak5hPct'), { any: true, firstDay: '2026-10-25', gaps: false });
  assert.deepEqual(pctSeries(days.slice(0, 1), 'peak5hPct'), { any: false, firstDay: null, gaps: true });
  assert.deepEqual(pctSeries([], 'peak5hPct'), { any: false, firstDay: null, gaps: false });
});

test('pctNote only talks about days before readings began', () => {
  assert.match(pctNote(days, ['peak5hPct', 'peak7dPct']), /began on 10-25/);
  assert.equal(pctNote(days.slice(1, 3), ['peak5hPct', 'peak7dPct']), '');
  assert.match(pctNote(days.slice(0, 1), ['peak5hPct', 'peak7dPct']), /No percent readings yet/);
  assert.equal(pctNote([], ['peak5hPct']), '');
});

test('limitLine gives peak and now, or nothing', () => {
  assert.equal(limitLine({ peak5hPct: 70, last5hPct: 35.4 }), '5h limit: peak 70%, now 35%');
  assert.equal(limitLine({ peak5hPct: 70, last5hPct: null }), '5h limit: peak 70%');
  assert.equal(limitLine({ peak5hPct: null, last5hPct: null }), '');
  assert.equal(limitLine({}), '');
});

test('the copied text carries the limit line of an account', () => {
  const s = sample();
  s.accounts[1].peak5hPct = 88;
  s.accounts[1].last5hPct = 20;
  const t = summaryText(s);
  assert.match(t, /\n {2}5h limit: peak 88%, now 20%\n/);
  assert.doesNotMatch(summaryText(sample()), /5h limit/);
});
