// numbers the agent modules share
export const WALK_SPEED = 0.85;
export const STAND_YAW = 0; // waving agents face the fixed camera
export const SEAT_Y = 0.4;
export const CHAR_SCALE = 1.0;
export const K = CHAR_SCALE / 0.85; // flair offsets were tuned at 0.85
export const LIFT = 1.0;
export const TAG_Y = 0.62 * K; // one tag height for every agent
export const BUBBLE_Y = 0.9;
export const TAU = Math.PI * 2;

export const angleDiff = (a, b) => Math.atan2(Math.sin(a - b), Math.cos(a - b));

// arm angles about the shoulder for the seated and break poses. brow and cheer are the
// work reactions: hand to the forehead, arms up
export const ARM = { type: 0.5, lap: 1.15, mouth: -0.35, chin: -0.2, hold: 0.8, game: 0.6, swing: 0.45, stretch: 1.65, brow: -0.9, cheer: 2.5 };
