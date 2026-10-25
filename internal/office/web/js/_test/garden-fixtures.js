// loads the v2 layout fixtures next to the tests. the _test folder starts with an underscore so go:embed skips it.
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';

const dir = fileURLToPath(new URL('./fixtures/', import.meta.url));

export const NAMES = ['rows', 'village', 'river', 'street'];
export const fixture = (name) => JSON.parse(fs.readFileSync(dir + name + '.json', 'utf8'));

// layouts written by the go generators (3 and 8 accounts), copied here so the tests do not depend on the generator
export const GEN = ['rows-3', 'rows-8', 'village-3', 'village-8', 'river-3', 'river-8', 'street-3', 'street-8', 'village-10-16-40'].map((n) => 'gen-' + n);
// layouts with team pavilions (5 and 14 members, 14 gives two pavilions); the hand-made team-* fixtures stay out of ALL (their pavilion is not on a road)
export const TEAM = ['rows', 'village', 'river', 'street'].flatMap((t) => [5, 14].map((n) => 'gen-team-' + t + '-' + n));
export const ALL = [...NAMES, ...GEN, ...TEAM];
