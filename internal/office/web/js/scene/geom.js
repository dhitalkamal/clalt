// pure grid math shared by the renderer and the tests. cells are 0.5 world units, the layout origin is the
// top-left of the room, world x and z are centred on the middle of the layout.
export const G = 0.5;
export const MEZ = 1.29;
export const levels = [0, MEZ];

const RAD = Math.PI / 180;

// facing of a furniture rotation as a yaw for three.js: rot 0 faces +z (the camera), a quarter turn faces +x
export const yawOf = (rot) => (rot || 0) * RAD;

// local item offset (world units, x east and z south at rot 0) turned by the item's rotation
export function rotate(lx, lz, rot) {
  const t = yawOf(rot);
  const c = Math.round(Math.cos(t) * 1e6) / 1e6;
  const s = Math.round(Math.sin(t) * 1e6) / 1e6;
  return { x: lx * c + lz * s, z: -lx * s + lz * c };
}

// footprint after rotation: 90 and 270 swap width and depth
export function turned(fp, rot) {
  return rot === 90 || rot === 270 ? { w: fp.d, d: fp.w } : { w: fp.w, d: fp.d };
}

// converter between layout cells and world units for one layout size
export function frame(size) {
  const xmin = -(size.w * G) / 2;
  const zmin = -(size.d * G) / 2;
  return {
    W: size.w * G, D: size.d * G, xmin, zmin,
    X: (cx) => xmin + cx * G,
    Z: (cz) => zmin + cz * G,
    cx: (x) => (x - xmin) / G,
    cz: (z) => (z - zmin) / G,
  };
}

export function cellToWorld(size, level, x, z) {
  const f = frame(size);
  return { x: f.X(x), y: levels[level] || 0, z: f.Z(z) };
}

export function worldToCell(size, level, p) {
  const f = frame(size);
  return { x: Math.floor(f.cx(p.x) + 1e-9), z: Math.floor(f.cz(p.z) + 1e-9) };
}

// local solid rectangle [x, z, w, d] in footprint cells (rot 0) as an axis rectangle of rotated cells
export function solidCells(rect, fp, rot) {
  const out = [];
  const t = turned(fp, rot);
  for (let i = 0; i < rect[2]; i++) {
    for (let j = 0; j < rect[3]; j++) {
      // centre of this local cell relative to the footprint centre, in cells
      const lx = rect[0] + i + 0.5 - fp.w / 2;
      const lz = rect[1] + j + 0.5 - fp.d / 2;
      const r = rotate(lx, lz, rot);
      out.push({ i: Math.floor(r.x + t.w / 2), j: Math.floor(r.z + t.d / 2) });
    }
  }
  return out;
}
