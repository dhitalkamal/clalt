// tiny element builder. no innerHTML, no inline style attributes (csp).
export function h(tag, props = {}, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(props)) {
    if (v == null || v === false) continue;
    if (k === 'class') el.className = v;
    else if (k === 'text') el.textContent = v;
    else if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
    else if (k === 'value' || k === 'checked' || k === 'disabled' || k === 'selected') el[k] = v;
    else el.setAttribute(k, v === true ? '' : v);
  }
  for (const kid of kids.flat()) if (kid != null) el.append(kid);
  return el;
}

// labelled field row: <label>text <control></label>
export function field(text, control) {
  return h('label', { class: 'ed-field' }, h('span', { text }), control);
}

export function numberInput(value, min, max, onChange) {
  return h('input', {
    type: 'number', value: String(value), min, max, step: 1,
    onchange: (e) => {
      const n = Math.round(Number(e.target.value));
      if (Number.isFinite(n)) onChange(n);
    },
  });
}

export function selectInput(options, value, onChange) {
  const sel = h('select', { onchange: (e) => onChange(e.target.value) },
    options.map(([v, t]) => h('option', { value: v, text: t, selected: v === value })));
  return sel;
}

// true while the user types in a form control, so shortcuts stay out of the way
export function isTyping(target) {
  const t = target && target.tagName;
  return t === 'INPUT' || t === 'SELECT' || t === 'TEXTAREA';
}
