const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');
const root = path.resolve(__dirname, '..');

// WeChat's CommonJS loader resolves JavaScript modules, unlike Node's JSON require extension.
function loadApp() {
 const cache = new Map();let app;
 const context = {wx: {}, App: value => {app = value;}, setTimeout, clearTimeout};
 function load(file) {
  const resolved = file.endsWith('.js') ? file : file + '.js';
  if (!resolved.startsWith(root + path.sep) || !fs.existsSync(resolved)) throw new Error('WeChat module is not defined: ' + path.relative(root, resolved));
  if (cache.has(resolved)) return cache.get(resolved).exports;
  const module = {exports: {}};cache.set(resolved, module);
  vm.runInNewContext('(function(require,module,exports){\n' + fs.readFileSync(resolved,'utf8') + '\n})',context,{filename: resolved})(
   id => load(path.resolve(path.dirname(resolved), id)),module,module.exports);
  return module.exports;
 }
 load(path.join(root,'app.js'));
 return {app, load};
}

test('app and icon catalog load with WeChat JavaScript-only module resolution', async () => {
 const {app,load}=loadApp();assert.equal(typeof app.onLaunch,'function');app.onLaunch();
 const icons=load(path.join(root,'utils/category-icons.js'));
 assert.equal(icons.catalog.length,48);assert.equal(await icons.loadFont(),false);
 const backend = JSON.parse(fs.readFileSync(path.resolve(root,'../backend/internal/domain/category_icon_catalog.json'),'utf8'));
 assert.equal(JSON.stringify(icons.catalog),JSON.stringify(backend));
});
