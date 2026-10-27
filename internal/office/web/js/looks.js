// which character a record gets: a hash of its key, so it never changes while the key lives
import { hash } from './model.js';

export function pickLook(key, list) {
  return list[hash(String(key)) % list.length];
}
