// input state for the navigation modes: keys down, mouse, wheel and double click, collected between frames.
const TEXT = new Set(['INPUT', 'TEXTAREA', 'SELECT']);

// true when a key event must not drive the camera: typing in a field, or a ctrl or meta shortcut
export function ignoreKey(e) {
  if (e.ctrlKey || e.metaKey) return true;
  const t = e.target;
  return !!(t && (TEXT.has(t.tagName) || t.isContentEditable));
}

// x and y are client pixels. keys holds KeyboardEvent.code values. consume() clears the per-frame deltas.
export function createInput(dom) {
  const input = {
    keys: new Set(),
    mouse: { x: 0, y: 0, dx: 0, dy: 0, buttons: 0, over: false },
    wheel: 0,
    ctrlWheel: false,
    dbl: null,
    consume() { input.mouse.dx = 0; input.mouse.dy = 0; input.wheel = 0; input.ctrlWheel = false; input.dbl = null; },
    dispose() { for (const [t, ev, fn, o] of subs) t.removeEventListener(ev, fn, o); input.keys.clear(); },
  };
  const m = input.mouse;
  const subs = [];
  const on = (t, ev, fn, o) => { t.addEventListener(ev, fn, o); subs.push([t, ev, fn, o]); };
  on(window, 'keydown', (e) => { if (!ignoreKey(e)) input.keys.add(e.code); });
  on(window, 'keyup', (e) => input.keys.delete(e.code));
  on(window, 'blur', () => input.keys.clear());
  on(dom, 'pointerdown', (e) => { m.buttons = e.buttons; m.x = e.clientX; m.y = e.clientY; try { dom.setPointerCapture(e.pointerId); } catch (err) { /* not capturable */ } });
  on(dom, 'pointermove', (e) => { m.dx += e.clientX - m.x; m.dy += e.clientY - m.y; m.x = e.clientX; m.y = e.clientY; m.buttons = e.buttons; });
  on(dom, 'pointerup', (e) => { m.buttons = e.buttons; });
  on(dom, 'pointercancel', () => { m.buttons = 0; });
  on(dom, 'pointerenter', () => { m.over = true; });
  on(dom, 'pointerleave', () => { m.over = false; });
  on(dom, 'wheel', (e) => { e.preventDefault(); input.wheel += e.deltaY; if (e.ctrlKey) input.ctrlWheel = true; m.x = e.clientX; m.y = e.clientY; }, { passive: false });
  on(dom, 'dblclick', (e) => { input.dbl = { x: e.clientX, y: e.clientY }; });
  on(dom, 'contextmenu', (e) => e.preventDefault());
  return input;
}
