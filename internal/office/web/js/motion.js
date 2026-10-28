// does anything about the agents need the 15 fps rate? seated idle ones only breathe, so they do not.
export function agentsMoving(agents) {
  for (const a of agents) {
    if (a.mode === 'away') {
      // walking in starts from away, a leaver is still shrinking
      if (a.scale > 0.01 && a.scale < 0.99) return true;
      continue;
    }
    if (a.mode !== 'seated' || a.s.state === 'busy' || a.s.state === 'needs-you' || a.scale < 0.99) return true;
  }
  return false;
}

// the hall cutaway is easing a wall: it fades without the camera moving, so it needs frames of its own
export const fading = (app) => !!app.cutBusy;
