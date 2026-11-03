// true when the viewer asked for reduced motion: poses and bubbles then change in place
export const reduced = () => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
