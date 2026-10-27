// merging reply pages into the list the panel holds. lists are newest first, like the reply.
// the reply has no id per entry, so an entry is told apart by its time, kind, agent, tool and text.
function sum(s) {
  let h = 5381;
  for (let i = 0; i < s.length; i++) h = ((h * 33) ^ s.charCodeAt(i)) >>> 0;
  return h.toString(36);
}

export function entryKey(e) {
  const text = typeof e.text === 'string' ? e.text : '';
  return [e.t, e.kind, e.agent || '', e.tool || '', e.durationMs || 0, e.ok === false ? 0 : 1, text.length, sum(text)].join('|');
}

// entries newer than the newest known one go in front. the newest known entry is looked for in the
// batch together with what follows it, so two identical entries of one second are not folded into one.
// without any overlap (a gap) the entries not already known are taken.
export function mergeNewer(known, incoming) {
  if (!known.length) return { entries: incoming, added: incoming.length };
  const kk = known.map(entryKey);
  const ik = incoming.map(entryKey);
  for (let i = 0; i < ik.length; i++) {
    if (ik[i] !== kk[0]) continue;
    const n = Math.min(kk.length, ik.length - i);
    let same = true;
    for (let j = 1; j < n && same; j++) same = ik[i + j] === kk[j];
    if (!same) continue;
    return i ? { entries: [...incoming.slice(0, i), ...known], added: i } : { entries: known, added: 0 };
  }
  const have = new Set(kk);
  const fresh = incoming.filter((e, i) => !have.has(ik[i]));
  return { entries: [...fresh, ...known], added: fresh.length };
}

// older entries go to the end. pages should not overlap, but a repeat at the boundary is dropped.
export function mergeOlder(known, incoming) {
  const kk = known.map(entryKey);
  const ik = incoming.map(entryKey);
  let k = Math.min(kk.length, ik.length);
  while (k > 0) {
    let same = true;
    for (let j = 0; j < k && same; j++) same = kk[kk.length - k + j] === ik[j];
    if (same) break;
    k--;
  }
  const rest = incoming.slice(k);
  return { entries: [...known, ...rest], added: rest.length };
}
