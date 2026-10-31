// input state for the navigation modes: keys down, mouse, wheel and double click, collected between frames.
const TEXT = new Set(['INPUT', 'TEXTAREA', 'SELECT']);
const LINE_PX = 16; // wheel deltaMode 1 (firefox lines)
const PAGE_PX = 800; // wheel deltaMode 2 (pages)
const PINCH_LOG = 0.01; // sky zooms by wheel * 0.01 on a ctrl wheel, so this keeps pinch and zoom in step

// true when a key event must not drive the camera: typing in a field, a focused control in an open dialog,
// or a ctrl or meta shortcut
export function ignoreKey(e) {
  if (e.ctrlKey || e.metaKey) return true;
  const t = e.target;
  return !!(t && (TEXT.has(t.tagName) || t.isContentEditable || (t.closest && t.closest('dialog[open]'))));
}

// what a held mouse button does in the sky: left orbits, shift+left (or ctrl+left, the mac right click) and right pan
export function dragMode(buttons, shift, ctrl = false) {
  if (buttons & 2) return 'pan';
  if (buttons & 1) return shift || ctrl ? 'pan' : 'orbit';
  return null;
}

// x and y are client pixels. keys holds KeyboardEvent.code values. consume() clears the per-frame deltas.
export function createInput(dom) {
  const input = {
    keys: new Set(),
    mouse: { x: 0, y: 0, dx: 0, dy: 0, buttons: 0, shift: false, ctrl: false, over: false },
    wheel: 0,
    ctrlWheel: false,
    dbl: null,
    consume() { input.mouse.dx = 0; input.mouse.dy = 0; input.wheel = 0; input.ctrlWheel = false; input.dbl = null; },
    dispose() { for (const [t, ev, fn, o] of subs) t.removeEventListener(ev, fn, o); input.keys.clear(); },
  };
  const m = input.mouse;
  const subs = [];
  const ptrs = new Map(); // active pointers by id, for the two finger pinch
  let pinch = null; // {d, x, y} spread and midpoint of the first two pointers last frame
  const on = (t, ev, fn, o) => { t.addEventListener(ev, fn, o); subs.push([t, ev, fn, o]); };
  const isMeta = (e) => e.key === 'Meta' || e.code === 'MetaLeft' || e.code === 'MetaRight';
  on(window, 'keydown', (e) => {
    if (isMeta(e)) { input.keys.clear(); return; }
    if (ignoreKey(e)) input.keys.delete(e.code); else input.keys.add(e.code);
  });
  // macOS drops the keyup of keys released while cmd is held, so cmd itself resets the held set
  on(window, 'keyup', (e) => { if (isMeta(e)) input.keys.clear(); else input.keys.delete(e.code); });
  on(window, 'blur', () => input.keys.clear());
  if (typeof document !== 'undefined') on(document, 'visibilitychange', () => { if (document.visibilityState === 'hidden') input.keys.clear(); });
  const pair = () => { const [a, b] = [...ptrs.values()]; return { d: Math.hypot(a.x - b.x, a.y - b.y) || 1, x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 }; };
  on(dom, 'pointerdown', (e) => {
    ptrs.set(e.pointerId, { x: e.clientX, y: e.clientY });
    if (ptrs.size === 2) { pinch = pair(); m.buttons = 2; m.x = pinch.x; m.y = pinch.y; return; }
    m.buttons = e.buttons; m.shift = e.shiftKey; m.ctrl = e.ctrlKey; m.x = e.clientX; m.y = e.clientY; try { dom.setPointerCapture(e.pointerId); } catch (err) { /* not capturable */ } });
  on(dom, 'pointermove', (e) => {
    const p = ptrs.get(e.pointerId);
    if (p) { p.x = e.clientX; p.y = e.clientY; }
    if (ptrs.size >= 2 && p) {
      // pinch: the spread change is a wheel zoom, the midpoint drag a pan (the right button path), no orbit
      const now = pair();
      input.wheel -= Math.log(now.d / pinch.d) / PINCH_LOG;
      input.ctrlWheel = true;
      m.dx += now.x - pinch.x; m.dy += now.y - pinch.y;
      m.x = now.x; m.y = now.y; m.buttons = 2;
      pinch = now;
      return;
    }
    m.dx += e.clientX - m.x; m.dy += e.clientY - m.y; m.x = e.clientX; m.y = e.clientY; m.buttons = e.buttons;
  });
  const lift = (e) => {
    ptrs.delete(e.pointerId);
    pinch = null;
    const rest = [...ptrs.values()][0];
    if (rest) { m.x = rest.x; m.y = rest.y; m.buttons = 1; } else m.buttons = e.type === 'pointerup' ? e.buttons : 0;
  };
  on(dom, 'pointerup', lift);
  on(dom, 'pointercancel', lift);
  on(dom, 'pointerenter', () => { m.over = true; });
  on(dom, 'pointerleave', () => { m.over = false; });
  on(dom, 'wheel', (e) => { e.preventDefault(); input.wheel += e.deltaY * (e.deltaMode === 1 ? LINE_PX : e.deltaMode === 2 ? PAGE_PX : 1); if (e.ctrlKey) input.ctrlWheel = true; m.x = e.clientX; m.y = e.clientY; }, { passive: false });
  on(dom, 'dblclick', (e) => { input.dbl = { x: e.clientX, y: e.clientY }; });
  on(dom, 'contextmenu', (e) => e.preventDefault());
  return input;
}
