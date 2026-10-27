// the hall sits on a flat calm colour: no lawn, trees, sky dome or clouds. the garden clears this and draws its own sky.
import * as THREE from 'three';
import { backdropColor } from '../nav/cutmath.js';

// layout is the v1 layout json, null puts the stage back to the clear colour
export function setBackdrop(stage, layout) {
  const { scene } = stage;
  if (!layout) { scene.background = null; return; }
  const hex = backdropColor(layout.floorColor, layout.walls && layout.walls.color);
  scene.background = new THREE.Color(hex);
  if (scene.fog) scene.fog.color.set(hex);
}
