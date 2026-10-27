// the embedded default hall layout compiled in node, shared by the hall nav tests
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { compile } from '../scene/plan.js';

const dir = fileURLToPath(new URL('../../layout/', import.meta.url));
const read = (n) => JSON.parse(fs.readFileSync(dir + n, 'utf8'));

export const hallLayout = () => read('default.json');
export const hallCatalog = () => read('catalog.json').map((c) => ({ ...c, tags: c.tags || [] }));
export const hallCompiled = () => compile(hallLayout(), hallCatalog());
