// press twice to confirm: the first press arms, a second one within a few seconds confirms.
// the timeout and reset (click elsewhere, Esc) put it back to idle. no dom.
const WAIT = 4000;

export function createConfirmPress({ wait = WAIT, setTimer = setTimeout, clearTimer = clearTimeout, onChange = () => {} } = {}) {
  let armed = false;
  let timer = null;
  const set = (on) => {
    if (on === armed) return;
    armed = on;
    onChange(on);
  };
  const stop = () => { if (timer !== null) clearTimer(timer); timer = null; };
  return {
    get armed() { return armed; },
    // 'arm' on the first press, 'confirm' on the second
    press() {
      if (armed) { stop(); set(false); return 'confirm'; }
      set(true);
      timer = setTimer(() => { timer = null; set(false); }, wait);
      return 'arm';
    },
    reset() { stop(); set(false); },
  };
}
