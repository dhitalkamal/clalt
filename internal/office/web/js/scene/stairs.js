// stairs of a layout in cells and world units. the flight is 2 cells wide and 6 long, (x, z) is its top cell,
// dir is the way up. a 2x2 landing continues past the top at deck height.
const DIRS = { n: [0, -1], s: [0, 1], e: [1, 0], w: [-1, 0] };

export function stairCells(st) {
  const { x, z, dir } = st;
  if (dir === 'n') return { flight: { x, z, w: 2, d: 6 }, landing: { x, z: z - 2, w: 2, d: 2 } };
  if (dir === 's') return { flight: { x, z: z - 5, w: 2, d: 6 }, landing: { x, z: z + 1, w: 2, d: 2 } };
  if (dir === 'e') return { flight: { x: x - 5, z, w: 6, d: 2 }, landing: { x: x + 1, z, w: 2, d: 2 } };
  return { flight: { x, z, w: 6, d: 2 }, landing: { x: x - 2, z, w: 2, d: 2 } };
}

// world frame of the flight: top edge centre T, bottom edge centre B, yaw that turns the local flight (up is -z)
export function stairWorld(st, f) {
  const { flight, landing } = stairCells(st);
  const a = DIRS[st.dir];
  const mid = { x: f.X(flight.x + flight.w / 2), z: f.Z(flight.z + flight.d / 2) };
  const half = (st.dir === 'n' || st.dir === 's' ? flight.d : flight.w) * 0.25;
  const T = { x: mid.x + a[0] * half, z: mid.z + a[1] * half };
  const B = { x: mid.x - a[0] * half, z: mid.z - a[1] * half };
  const yaw = { n: 0, s: Math.PI, e: -Math.PI / 2, w: Math.PI / 2 }[st.dir];
  const landingMid = { x: f.X(landing.x + landing.w / 2), z: f.Z(landing.z + landing.d / 2) };
  return { flight, landing, T, B, yaw, landingMid, dir: a };
}
