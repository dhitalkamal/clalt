// 2d helpers for the garden: polygons on the ground plane (x east, z south), curves and distances.
// pure, no three.js, so node tests can load it. angles here are radians; layout files use degrees.
export const rad = (d) => (d * Math.PI) / 180;

export function inPoly(x, z, poly) {
  let c = false;
  for (let i = 0, j = poly.length - 1; i < poly.length; j = i++) {
    const [xi, zi] = poly[i];
    const [xj, zj] = poly[j];
    if (zi > z !== zj > z && x < ((xj - xi) * (z - zi)) / (zj - zi) + xi) c = !c;
  }
  return c;
}

export function distSeg(x, z, a, b) {
  const dx = b[0] - a[0];
  const dz = b[1] - a[1];
  const l2 = dx * dx + dz * dz || 1;
  const t = Math.max(0, Math.min(1, ((x - a[0]) * dx + (z - a[1]) * dz) / l2));
  return Math.hypot(x - (a[0] + dx * t), z - (a[1] + dz * t));
}

// distance to the outline of a closed polygon
export function distPoly(x, z, poly) {
  let m = 1e9;
  for (let i = 0; i < poly.length; i++) m = Math.min(m, distSeg(x, z, poly[i], poly[(i + 1) % poly.length]));
  return m;
}

// distance to an open polyline
export function distLine(x, z, pts) {
  let m = 1e9;
  for (let i = 0; i + 1 < pts.length; i++) m = Math.min(m, distSeg(x, z, pts[i], pts[i + 1]));
  return m;
}

export function centroid(poly) {
  let x = 0;
  let z = 0;
  for (const p of poly) { x += p[0]; z += p[1]; }
  return [x / poly.length, z / poly.length];
}

export function scalePoly(poly, k, about) {
  const c = about || centroid(poly);
  return poly.map((p) => [c[0] + (p[0] - c[0]) * k, c[1] + (p[1] - c[1]) * k]);
}

export function boundsOf(pts) {
  let x0 = Infinity; let x1 = -Infinity; let z0 = Infinity; let z1 = -Infinity;
  for (const [x, z] of pts) { x0 = Math.min(x0, x); x1 = Math.max(x1, x); z0 = Math.min(z0, z); z1 = Math.max(z1, z); }
  return { x0, x1, z0, z1 };
}

// catmull-rom through the points, n samples per segment
export function curve(pts, n = 8) {
  const out = [];
  for (let i = 0; i + 1 < pts.length; i++) {
    const p0 = pts[Math.max(0, i - 1)];
    const p1 = pts[i];
    const p2 = pts[i + 1];
    const p3 = pts[Math.min(pts.length - 1, i + 2)];
    for (let s = 0; s < n; s++) {
      const t = s / n;
      const t2 = t * t;
      const t3 = t2 * t;
      const f = (a, b, c, d) => 0.5 * (2 * b + (-a + c) * t + (2 * a - 5 * b + 4 * c - d) * t2 + (-a + 3 * b - 3 * c + d) * t3);
      out.push([f(p0[0], p1[0], p2[0], p3[0]), f(p0[1], p1[1], p2[1], p3[1])]);
    }
  }
  out.push(pts[pts.length - 1]);
  return out;
}

// point on a polyline at fraction t (0..1) with the unit tangent
export function along(pts, t) {
  let total = 0;
  const lens = [];
  for (let i = 0; i + 1 < pts.length; i++) { const l = Math.hypot(pts[i + 1][0] - pts[i][0], pts[i + 1][1] - pts[i][1]); lens.push(l); total += l; }
  let want = t * total;
  for (let i = 0; i < lens.length; i++) {
    if (want <= lens[i] || i === lens.length - 1) {
      const u = lens[i] ? Math.min(1, want / lens[i]) : 0;
      const a = pts[i];
      const b = pts[i + 1];
      return { x: a[0] + (b[0] - a[0]) * u, z: a[1] + (b[1] - a[1]) * u, tx: (b[0] - a[0]) / (lens[i] || 1), tz: (b[1] - a[1]) / (lens[i] || 1) };
    }
    want -= lens[i];
  }
  return null;
}

// rotated rectangle {x,z,w,d,rot in radians} -> 4 corner points
export function footprint(x, z, w, d, rot) {
  const c = Math.cos(rot);
  const s = Math.sin(rot);
  return [[-w / 2, -d / 2], [w / 2, -d / 2], [w / 2, d / 2], [-w / 2, d / 2]].map(([lx, lz]) => [x + lx * c + lz * s, z - lx * s + lz * c]);
}

// separating axis test for two convex polygons, gap > 0 means they stay apart by at least gap
export function polysOverlap(a, b, gap = 0) {
  for (const poly of [a, b]) {
    for (let i = 0; i < poly.length; i++) {
      const p = poly[i];
      const q = poly[(i + 1) % poly.length];
      const nx = -(q[1] - p[1]);
      const nz = q[0] - p[0];
      const l = Math.hypot(nx, nz) || 1;
      let a0 = 1e9; let a1 = -1e9; let b0 = 1e9; let b1 = -1e9;
      for (const v of a) { const d = (v[0] * nx + v[1] * nz) / l; a0 = Math.min(a0, d); a1 = Math.max(a1, d); }
      for (const v of b) { const d = (v[0] * nx + v[1] * nz) / l; b0 = Math.min(b0, d); b1 = Math.max(b1, d); }
      if (a1 + gap < b0 || b1 + gap < a0) return false;
    }
  }
  return true;
}

// small seeded random so the garden looks the same on every load
export function rng(seed) {
  let s = seed >>> 0 || 1;
  return () => { s = (s * 1664525 + 1013904223) >>> 0; return s / 4294967296; };
}
