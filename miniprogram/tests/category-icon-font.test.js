const test=require('node:test');
const assert=require('node:assert/strict');
function fresh(wx) {global.wx=wx;delete require.cache[require.resolve('../utils/category-icons')];return require('../utils/category-icons');}
test('bundled font loads once for all components and uses a local base64 source',async()=>{
 let reads=0,loads=0;
 const icons=fresh({getFileSystemManager:()=>({readFile:o=>{reads++;assert.match(o.filePath,/assets\/category-icons/);o.success({data:'mock-base64'});}}),loadFontFace:o=>{loads++;assert.equal(o.global,true);assert.match(o.source,/data:font\/woff2;base64/);o.success();}});
 const states=[];const unsubscribe=icons.subscribe(v=>states.push(v));
 assert.equal(await icons.loadFont(),true);assert.equal(await icons.loadFont(),true);unsubscribe();assert.equal(reads,1);assert.equal(loads,1);assert.deepEqual(states,[false,true]);
 assert.equal(icons.catalog.length,48);assert.equal(new Set(icons.catalog.map(i=>i.key)).size,48);assert.ok(icons.glyph('dining'));assert.equal(icons.glyph('unknown'),'');
});
test('font loader tries ttf after woff2 failure and keeps emoji if both fail',async()=>{
 const formats=[];
 const icons=fresh({getFileSystemManager:()=>({readFile:o=>{formats.push(o.filePath);o.success({data:'mock'});}}),loadFontFace:o=>o.fail()});
 assert.equal(await icons.loadFont(),false);assert.equal(formats.length,2);assert.match(formats[1],/ttf$/);assert.equal(icons.fallback('🐱'),'🐱');assert.equal(icons.fallback(''),'📦');
});
test('unsupported font APIs retain the emoji path without rejecting',async()=>{
 const icons=fresh({});assert.equal(await icons.loadFont(),false);assert.equal(icons.emojiCount([{system_icon_key:'dining',icon_key:'unknown'},{system_icon_key:'',icon_key:''}]),1);
});
