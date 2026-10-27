// bounded undo and redo over json snapshots of the layout.
// snapshots are stored as strings so nothing outside can mutate them.

export const HISTORY_LIMIT = 100;

export function createHistory(initial, limit = HISTORY_LIMIT) {
  let stack = [JSON.stringify(initial)];
  let at = 0;

  const copy = () => JSON.parse(stack[at]);

  return {
    push(state) {
      const s = JSON.stringify(state);
      if (s === stack[at]) return false;
      stack = stack.slice(0, at + 1);
      stack.push(s);
      // limit counts undo steps, so keep limit + 1 snapshots
      if (stack.length > limit + 1) stack = stack.slice(stack.length - limit - 1);
      at = stack.length - 1;
      return true;
    },
    undo() {
      if (at === 0) return null;
      at--;
      return copy();
    },
    redo() {
      if (at >= stack.length - 1) return null;
      at++;
      return copy();
    },
    current: copy,
    canUndo: () => at > 0,
    canRedo: () => at < stack.length - 1,
    // start over from a new baseline (loading another layout)
    reset(state) {
      stack = [JSON.stringify(state)];
      at = 0;
    },
  };
}
