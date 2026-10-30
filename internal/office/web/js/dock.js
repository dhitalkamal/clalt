// keeps the Today and Reviews pages off the bottom-left controls row.
// the row can wrap, so the offsets are measured and handed to css as variables. a camera bar, if a page has one, counts too.
const NARROW = 700;
const GAP = 12;

// bottom offsets in px: side for the left pages on wide screens, sheet for the bottom sheet under 700px (clear of the controls row and the bar)
export function dockInsets({ vw, vh, controlsTop, barTop, barShown, gap = GAP }) {
  const above = (top) => Math.max(0, Math.round(vh - top + gap));
  return {
    side: vw > NARROW ? above(controlsTop) : gap,
    sheet: vw <= NARROW ? above(barShown ? Math.min(barTop, controlsTop) : controlsTop) : gap,
  };
}

export function bindDock() {
  const controls = document.getElementById('controls');
  const bar = document.getElementById('bar');
  if (!controls) return;
  const root = document.documentElement.style;
  const apply = () => {
    const barShown = !!bar && !bar.hidden;
    const d = dockInsets({
      vw: innerWidth, vh: innerHeight, controlsTop: controls.getBoundingClientRect().top,
      barTop: barShown ? bar.getBoundingClientRect().top : 0, barShown,
    });
    root.setProperty('--dock-side', d.side + 'px');
    root.setProperty('--dock-sheet', d.sheet + 'px');
  };
  apply();
  addEventListener('resize', apply);
  if (typeof ResizeObserver === 'function') {
    const ro = new ResizeObserver(apply);
    ro.observe(controls);
    if (bar) ro.observe(bar);
  }
  // the bar is shown or hidden by its hidden attribute
  if (bar) new MutationObserver(apply).observe(bar, { attributes: true, attributeFilter: ['hidden'] });
}
