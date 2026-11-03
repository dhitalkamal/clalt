// the raised hand of an agent that needs you. pure, no three.js.
export const canStand = (slot) => !!slot && Array.isArray(slot.stand) && slot.stand.length > 0;

// a seat with nowhere to stand asks for help from the chair
export const seatedAsk = (mode, state, slot) => mode === 'seated' && state === 'needs-you' && !canStand(slot);

// right arm angle, body lift and lean; reduced motion holds the hand up without swinging
export function askPose(t, calm) {
  if (calm) return { arm: -1.35, lift: 0, lean: 0 };
  return { arm: -1.35 + Math.sin(Math.PI * 3 * t) * 0.4, lift: Math.abs(Math.sin(Math.PI * 2.8 * t)) * 0.03, lean: Math.sin(Math.PI * 2.8 * t) * 0.05 };
}
