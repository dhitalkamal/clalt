// shared test data: a small catalog and a valid layout
export const catalog = [
  { id: 'desk', footprint: { w: 3, d: 2 }, levels: [0, 1], tags: ['desk'], label: 'Desk', category: 'work' },
  { id: 'chair', footprint: { w: 1, d: 1 }, levels: [0, 1], tags: ['seat'], label: 'Chair', category: 'work' },
  { id: 'sofa', footprint: { w: 4, d: 2 }, levels: [0], tags: ['seat'], label: 'Sofa', category: 'lounge' },
];

export function baseLayout() {
  return {
    v: 1, name: 'test', title: 'Test', grid: 0.5,
    size: { w: 40, d: 20 },
    walls: { color: '#e9e4dc' }, floorColor: '#d9b88f',
    door: { side: 'front', at: 3 },
    deck: { x: 20, z: 0, w: 10, d: 8 },
    stairs: { x: 30, z: 4, dir: 'w' },
    rooms: [
      { id: 'r1', type: 'lobby', level: 0, x: 0, z: 0, w: 8, d: 8, label: 'Lobby' },
      { id: 'r2', type: 'cabins', level: 0, x: 10, z: 0, w: 8, d: 8, label: 'Cabins' },
      { id: 'r3', type: 'workspace', level: 1, x: 20, z: 0, w: 6, d: 6, label: 'Loft' },
    ],
    furniture: [
      { id: 'f1', item: 'desk', level: 0, x: 11, z: 1, rot: 0, room: 'r2' },
      { id: 'f2', item: 'chair', level: 0, x: 12, z: 3, rot: 0, room: 'r2' },
    ],
  };
}
