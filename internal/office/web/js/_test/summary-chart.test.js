import test from 'node:test';
import assert from 'node:assert/strict';
import { niceTicks, dayTicks, chartModel, readout } from '../summary/chart.js';
import { history } from './summary-fixture.js';

test('niceTicks picks 1, 2 or 5 steps and covers the max', () => {
  assert.deepEqual(niceTicks(0), { top: 10, ticks: [0, 5, 10] });
  assert.deepEqual(niceTicks(7), { top: 8, ticks: [0, 2, 4, 6, 8] });
  assert.deepEqual(niceTicks(95), { top: 100, ticks: [0, 20, 40, 60, 80, 100] });
  assert.deepEqual(niceTicks(1200000), { top: 1500000, ticks: [0, 500000, 1000000, 1500000] });
  const t = niceTicks(123456);
  assert.ok(t.top >= 123456 && t.ticks.length >= 3 && t.ticks.length <= 7);
  assert.equal(t.ticks[0], 0);
  assert.equal(t.ticks.at(-1), t.top);
});

test('niceTicks ignores bad input', () => {
  assert.deepEqual(niceTicks(NaN), niceTicks(0));
  assert.deepEqual(niceTicks(-4), niceTicks(0));
});

test('dayTicks labels every day when there is room and always the last', () => {
  const days = ['2026-10-24', '2026-10-25', '2026-10-26', '2026-10-27'];
  assert.deepEqual(dayTicks(days, 10).map((t) => t.i), [0, 1, 2, 3]);
  const many = Array.from({ length: 30 }, (_, i) => `2026-10-${String(i + 1).padStart(2, '0')}`);
  const t = dayTicks(many, 6);
  assert.ok(t.length <= 6 && t.length >= 3);
  assert.equal(t.at(-1).i, 29);
  assert.equal(t.at(-1).text, '10-30');
  // no two labels closer than the step
  for (let k = 1; k < t.length; k++) assert.ok(t[k].i - t[k - 1].i >= Math.floor(30 / 6));
});

test('dayTicks of nothing is nothing', () => {
  assert.deepEqual(dayTicks([], 5), []);
});

test('chartModel scales bars into the plot and lays out ticks', () => {
  const w = history().accounts[1].days;
  const m = chartModel({ days: w, key: 'tokens', width: 400, height: 200 });
  assert.equal(m.bars.length, 4);
  const plot = m.plot;
  assert.ok(plot.x > 0 && plot.y > 0 && plot.x + plot.w <= 400 && plot.y + plot.h <= 200);
  // the tallest bar fills 1.2M of a 1.5M axis
  const b0 = m.bars[0];
  assert.ok(Math.abs(b0.h - plot.h * (1200000 / m.top)) < 0.01);
  assert.ok(Math.abs(b0.y + b0.h - (plot.y + plot.h)) < 0.01);
  // an empty day is a zero height bar kept for its slot
  assert.equal(m.bars[1].h, 0);
  assert.ok(m.bars[0].x >= plot.x && m.bars[3].x + m.bars[3].w <= plot.x + plot.w + 0.001);
  assert.equal(m.yTicks[0].label, '0');
  assert.equal(m.yTicks.at(-1).label, '1.5M');
  assert.ok(m.xTicks.length >= 2);
  for (const b of m.bars) assert.ok(b.w > 0);
});

test('chartModel keeps every bar a visible sliver when the value is tiny next to the max', () => {
  const days = [{ day: '2026-10-01', tokens: 1 }, { day: '2026-10-02', tokens: 1000000 }];
  const m = chartModel({ days, key: 'tokens', width: 300, height: 160 });
  assert.ok(m.bars[0].h >= 1);
});

test('chartModel on a narrow width still fits and thins the x labels', () => {
  const days = Array.from({ length: 90 }, (_, i) => ({ day: new Date(Date.UTC(2026, 6, 1 + i)).toISOString().slice(0, 10), tokens: i * 1000 }));
  const m = chartModel({ days, key: 'tokens', width: 340, height: 180 });
  assert.equal(m.bars.length, 90);
  assert.ok(m.xTicks.length <= Math.floor(m.plot.w / 40));
  for (const b of m.bars) assert.ok(b.x >= m.plot.x - 0.001 && b.x + b.w <= m.plot.x + m.plot.w + 0.001);
});

test('readout reads one day aloud', () => {
  const d = history().accounts[1].days[0];
  assert.equal(readout(d), '10-24: 1.2M tokens, $3.50, busiest 5h 800k, busiest 7d 1.2M');
  assert.equal(readout({ day: '2026-10-25', tokens: 0, cost: 0 }), '10-25: no usage');
});

test('readout adds the live percent when there is one', () => {
  const d = { ...history().accounts[1].days[3], peak5hPct: 41.5, peak7dPct: 12 };
  assert.match(readout(d), /now 5h 42%, 7d 12%$/);
});
