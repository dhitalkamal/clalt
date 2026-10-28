// small dev overlay: fps, draw calls, triangles and frame time. off by default, Shift+F toggles it.
export function createHud(renderer, doc = document) {
  let el = null;
  let frames = 0;
  let ms = 0;
  let since = performance.now();

  const toggle = () => {
    if (el) { el.remove(); el = null; return false; }
    el = doc.createElement('div');
    el.style.cssText = 'position:fixed;left:8px;bottom:8px;z-index:50;padding:4px 8px;font:11px/1.4 ui-monospace,monospace;color:#cfe;background:rgba(0,0,0,.65);border-radius:4px;pointer-events:none;white-space:pre';
    doc.body.append(el);
    return true;
  };

  // called after each drawn frame with how long the step and render took
  const frame = (now, frameMs, mode) => {
    frames++;
    ms += frameMs;
    if (!el || now - since < 500) return;
    const i = renderer.info.render;
    const fps = (frames * 1000) / (now - since);
    el.textContent = `${fps.toFixed(1)} fps  ${mode}\n${(ms / frames).toFixed(1)} ms/frame\n${i.calls} draws  ${i.triangles} tris`;
    frames = 0;
    ms = 0;
    since = now;
  };

  return { toggle, frame, shown: () => !!el };
}
