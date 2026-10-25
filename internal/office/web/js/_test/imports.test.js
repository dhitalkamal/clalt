// every named import between the page's own modules must exist in the module it
// comes from. a missing one stops the browser from loading the whole module
// graph, and unit tests that never load that file cannot see it.
// run: node --test internal/office/web/js/_test/imports.test.js
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, dirname, resolve, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');

function sources(dir) {
  const out = [];
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) {
      if (name !== 'vendor' && name !== 'node_modules') out.push(...sources(p));
    } else if (name.endsWith('.js') || name.endsWith('.mjs')) {
      out.push(p);
    }
  }
  return out;
}

const stripComments = (s) => s.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:'"`\\])\/\/.*$/gm, '$1');

// names a module exports, or null when it re-exports everything (cannot be checked)
function exportsOf(src) {
  const s = stripComments(src);
  if (/export\s*\*\s*from/.test(s)) return null;
  const names = new Set();
  for (const m of s.matchAll(/export\s+(?:async\s+)?(?:function\*?|const|let|var|class)\s+([A-Za-z_$][\w$]*)/g)) names.add(m[1]);
  for (const m of s.matchAll(/export\s*\{([^}]*)\}/g)) {
    for (const part of m[1].split(',')) {
      const bits = part.trim().split(/\s+as\s+/);
      if (bits[0]) names.add((bits[1] || bits[0]).trim());
    }
  }
  if (/export\s+default\b/.test(s)) names.add('default');
  return names;
}

// named imports from relative paths: [{names, from}]
function importsOf(src) {
  const s = stripComments(src);
  const out = [];
  for (const m of s.matchAll(/import\s*(?:[A-Za-z_$][\w$]*\s*,\s*)?\{([^}]*)\}\s*from\s*['"](\.[^'"]+)['"]/g)) {
    const names = m[1].split(',').map((p) => p.trim().split(/\s+as\s+/)[0].trim()).filter(Boolean);
    out.push({ names, from: m[2] });
  }
  return out;
}

test('every named import between page modules is exported by its target', () => {
  const missing = [];
  for (const file of sources(root)) {
    for (const imp of importsOf(readFileSync(file, 'utf8'))) {
      const target = resolve(dirname(file), imp.from);
      let src;
      try { src = readFileSync(target, 'utf8'); } catch { missing.push(`${relative(root, file)}: ${imp.from} not found`); continue; }
      const names = exportsOf(src);
      if (!names) continue;
      for (const n of imp.names) if (!names.has(n)) missing.push(`${relative(root, file)}: ${n} not exported by ${imp.from}`);
    }
  }
  assert.deepEqual(missing, []);
});
