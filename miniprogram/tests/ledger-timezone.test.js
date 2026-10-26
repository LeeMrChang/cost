const assert=require('node:assert/strict');
const {spawnSync}=require('node:child_process');
const path=require('node:path');
const test=require('node:test');
for(const timezone of ['UTC','America/Los_Angeles','Asia/Shanghai']) {
 test('ledger civil dates and weekdays stay in Shanghai when device timezone is '+timezone,()=>{
  const script=`const {groups}=require('./utils/ledger');
   const g=groups([{id:1,kind:'expense',amount_cent:1000,occurred_at:'2025-12-31T16:30:00Z'}],[],[])[0];
   console.log(JSON.stringify({date:g.date,time:g.items[0].time,weekday:g.weekday}));`;
  const result=spawnSync(process.execPath,['-e',script],{cwd:path.resolve(__dirname,'..'),env:{...process.env,TZ:timezone},encoding:'utf8'});
  assert.equal(result.status,0,result.stderr);
  assert.deepEqual(JSON.parse(result.stdout),{date:'2026-01-01',time:'00:30',weekday:'\u661f\u671f\u56db'});
 });
}
