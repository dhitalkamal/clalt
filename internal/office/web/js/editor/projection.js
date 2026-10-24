// camera math without three.js: world point to screen and screen to a
// horizontal plane. matrices are three.js style (column-major, .elements).

function mulPoint(e, x, y, z, w) {
  return [
    e[0] * x + e[4] * y + e[8] * z + e[12] * w,
    e[1] * x + e[5] * y + e[9] * z + e[13] * w,
    e[2] * x + e[6] * y + e[10] * z + e[14] * w,
    e[3] * x + e[7] * y + e[11] * z + e[15] * w,
  ];
}

// screen pixels of a world point; visible is false behind the camera
export function projectPoint(camera, p, width, height) {
  const v = mulPoint(camera.matrixWorldInverse.elements, p.x, p.y, p.z, 1);
  const c = mulPoint(camera.projectionMatrix.elements, v[0], v[1], v[2], v[3]);
  const w = c[3] || 1e-9;
  return { x: ((c[0] / w + 1) / 2) * width, y: ((1 - c[1] / w) / 2) * height, visible: c[3] > 0 };
}

function unproject(camera, nx, ny, nz) {
  const v = mulPoint(camera.projectionMatrixInverse.elements, nx, ny, nz, 1);
  const w = v[3] || 1e-9;
  const m = mulPoint(camera.matrixWorld.elements, v[0] / w, v[1] / w, v[2] / w, 1);
  return { x: m[0], y: m[1], z: m[2] };
}

// where the pixel ray meets the plane y = planeY, or null when it never does
export function groundPoint(camera, sx, sy, width, height, planeY) {
  const nx = (sx / width) * 2 - 1;
  const ny = 1 - (sy / height) * 2;
  const a = unproject(camera, nx, ny, -1);
  const b = unproject(camera, nx, ny, 1);
  const dy = b.y - a.y;
  if (Math.abs(dy) < 1e-9) return null;
  const t = (planeY - a.y) / dy;
  if (t < 0) return null;
  return { x: a.x + (b.x - a.x) * t, y: planeY, z: a.z + (b.z - a.z) * t };
}
