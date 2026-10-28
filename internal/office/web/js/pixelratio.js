// device pixel ratio the renderer uses: capped so retina and phones do not pay for pixels nobody can tell apart
export const DPR_CAP = 1.5;

export function pickPixelRatio(dpr, cap = DPR_CAP) {
  if (!Number.isFinite(dpr) || dpr < 1) return 1;
  return Math.min(dpr, cap);
}
