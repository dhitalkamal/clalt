import test from 'node:test';
import assert from 'node:assert/strict';
import { fmtTokens, fmtCost, fmtMinutes, shiftDay, dayLabel, localDay, shortDay, validDay } from '../summary/format.js';

test('fmtTokens uses k, M and B with one decimal under 100', () => {
  assert.equal(fmtTokens(0), '0');
  assert.equal(fmtTokens(950), '950');
  assert.equal(fmtTokens(1000), '1k');
  assert.equal(fmtTokens(1234), '1.2k');
  assert.equal(fmtTokens(99999), '100k');
  assert.equal(fmtTokens(123456), '123k');
  assert.equal(fmtTokens(2500000), '2.5M');
  assert.equal(fmtTokens(3100000000), '3.1B');
  assert.equal(fmtTokens(-5), '0');
  assert.equal(fmtTokens(NaN), '0');
  assert.equal(fmtTokens(undefined), '0');
});

test('fmtCost', () => {
  assert.equal(fmtCost(0), '$0.00');
  assert.equal(fmtCost(0.004), '<$0.01');
  assert.equal(fmtCost(0.126), '$0.13');
  assert.equal(fmtCost(12), '$12.00');
  assert.equal(fmtCost(1234.5), '$1234.50');
  assert.equal(fmtCost(null), '$0.00');
});

test('fmtMinutes', () => {
  assert.equal(fmtMinutes(0), '0m');
  assert.equal(fmtMinutes(45), '45m');
  assert.equal(fmtMinutes(60), '1h 00m');
  assert.equal(fmtMinutes(65), '1h 05m');
  assert.equal(fmtMinutes(600), '10h 00m');
  assert.equal(fmtMinutes(undefined), '0m');
});

test('shiftDay moves by calendar days across month and year ends', () => {
  assert.equal(shiftDay('2026-10-27', -1), '2026-10-26');
  assert.equal(shiftDay('2026-10-31', 1), '2026-11-01');
  assert.equal(shiftDay('2026-01-01', -1), '2025-12-31');
  assert.equal(shiftDay('2028-02-28', 1), '2028-02-29');
  assert.equal(shiftDay('2026-03-01', -1), '2026-02-28');
  assert.equal(shiftDay('bad', 1), null);
});

test('validDay accepts real dates only', () => {
  assert.equal(validDay('2026-10-27'), true);
  assert.equal(validDay('2026-02-30'), false);
  assert.equal(validDay('2026-1-27'), false);
  assert.equal(validDay(''), false);
  assert.equal(validDay(null), false);
});

test('localDay reads the local calendar of a date', () => {
  assert.equal(localDay(new Date(2026, 9, 27, 23, 59)), '2026-10-27');
  assert.equal(localDay(new Date(2026, 0, 5, 0, 1)), '2026-01-05');
});

test('dayLabel says Today and Yesterday, else weekday and date', () => {
  assert.equal(dayLabel('2026-10-27', '2026-10-27'), 'Today, Tue 27 Oct');
  assert.equal(dayLabel('2026-10-26', '2026-10-27'), 'Yesterday, Mon 26 Oct');
  assert.equal(dayLabel('2026-10-20', '2026-10-27'), 'Tue 20 Oct');
  assert.equal(dayLabel('2025-12-31', '2026-10-27'), 'Wed 31 Dec 2025');
});

test('shortDay is month and day for axes', () => {
  assert.equal(shortDay('2026-10-07'), '10-07');
});
