// tiny element builder. strings become text nodes, so nothing from the snapshot is ever parsed as markup.
export function el(tag, cls, ...kids) {
  const node = document.createElement(tag);
  if (cls) node.className = cls;
  for (const k of kids) if (k != null) node.append(k);
  return node;
}

export function clear(node) { node.replaceChildren(); }

// 3m, 12m, 2h 5m, 3d
export function age(ms) {
  const m = Math.max(0, Math.floor(ms / 60000));
  if (m < 1) return 'just now';
  if (m < 60) return m + 'm';
  if (m < 1440) return Math.floor(m / 60) + 'h ' + (m % 60) + 'm';
  return Math.floor(m / 1440) + 'd';
}
