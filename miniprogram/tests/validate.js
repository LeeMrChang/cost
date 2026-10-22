const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const root = path.resolve(__dirname, '..');
function walk(dir) {
 for (const file of fs.readdirSync(dir)) {
  const full = path.join(dir, file);
  if (fs.statSync(full).isDirectory()) walk(full);
  else if (file.endsWith('.json')) JSON.parse(fs.readFileSync(full, 'utf8'));
  else if (file.endsWith('.js')) new vm.Script(fs.readFileSync(full, 'utf8'), { filename: full });
 }
}
walk(root);
const app = JSON.parse(fs.readFileSync(path.join(root, 'app.json'), 'utf8'));
for (const page of app.pages) for (const extension of ['js', 'json', 'wxml', 'wxss']) {
 if (!fs.existsSync(path.join(root, page + '.' + extension))) throw new Error('Missing page file: ' + page + '.' + extension);
}
console.log('JavaScript syntax, JSON and page files validated');
