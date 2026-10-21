const fs=require('fs'),path=require('path'),vm=require('vm');
const root=path.resolve(__dirname,'..');
const p='D:/网站备份/寿县/原型文件/1011更新版/安徽寿县古城景区原型/智慧驾驶舱/原型/智慧管理端/03寿县古城景区营收统计系统/寿县古城景区营收统计系统.html';
const html=fs.readFileSync(p,'utf8');
const modules=vm.runInNewContext(html.match(/const MODULES = ([\s\S]*?\n\]);/)[1]);
if(process.argv.includes('--bootstrap')){
fs.writeFileSync(path.join(root,'docs/prototype-modules.json'),JSON.stringify(modules,null,2));
fs.writeFileSync(path.join(root,'frontend/apps/web-antd/public/revenue-logo.png'),Buffer.from(html.match(/src="data:image\/png;base64,([^"]+)"/)[1],'base64'));
let css=html.match(/<style>([\s\S]*?)<\/style>/)[1];
fs.writeFileSync(path.join(root,'frontend/apps/web-antd/src/views/revenue/prototype.css'),css);
}
const q=s=>"'"+String(s).replaceAll("'","''")+"'";
const sql=[];
modules.forEach((m,i)=>{sql.push(`INSERT INTO REVENUESTATISTICSSYSTEM.MODULES VALUES(${q(m.key)},${q(m.name)},${q(m.icon)},${q(m.desc)},${q(JSON.stringify(m.cols))},1,${i});`);
for(let month=1;month<=12;month++){const sum=m.rows.reduce((s,r)=>s+Number(r[2].replaceAll(',','')),0);let remaining=Math.round(m.trend[month-1]*1e6);
m.rows.forEach((r,j)=>{const amount=j===m.rows.length-1?remaining:Math.round(m.trend[month-1]*1e6*Number(r[2].replaceAll(',',''))/sum);remaining-=amount;
for(let day=1;day<=21;day++){const cents=Math.floor(amount/21)+(day<=amount%21?1:0);const qty=Math.max(1,Math.round(Number(r[1])*m.trend[month-1]/m.value/21));const visitors=Math.max(1,Math.round(parseFloat(m.visitors)*10000*m.trend[month-1]/m.value/m.rows.length/21));const date=`${month>10?2025:2026}-${String(month).padStart(2,'0')}-${String(day).padStart(2,'0')}`;
sql.push(`INSERT INTO REVENUESTATISTICSSYSTEM.REVENUE_RECORDS(ID,MODULE_KEY,BUSINESS_DATE,CATEGORY,QUANTITY,AMOUNT_CENTS,VISITORS,SOURCE,NOTE,CREATED_BY) VALUES(${q(`seed-${m.key}-${month}-${j}-${day}`)},${q(m.key)},${q(date)},${q(r[0])},${qty},${cents},${visitors},'原型测试数据','原型趋势按品类及日期分配，仅供验收','seed');`);
}})}});
fs.writeFileSync(path.join(root,'deploy/seed.sql'),sql.join('\n'));
if(!process.argv.includes('--bootstrap')){console.log('Seed updated');process.exit(0);}
const routes=modules.map((m,i)=>({path:'/module/'+m.key,name:m.key,title:m.name,order:i+3}));routes.unshift({path:'/dashboard',name:'Dashboard',title:'综合数据看板',order:1},{path:'/ranking',name:'Ranking',title:'营收排行榜',order:2});routes.push({path:'/settings',name:'Settings',title:'系统设置',order:12},{path:'/integrations',name:'Integrations',title:'接口对接',order:13});
fs.writeFileSync(path.join(root,'frontend/apps/web-antd/src/router/routes/modules/team.ts'),`import type { RouteRecordRaw } from 'vue-router';\nconst routes: RouteRecordRaw[] = ${JSON.stringify(routes)}.map(r=>({path:r.path,name:r.name,component:()=>import('#/views/revenue/index.vue'),meta:{title:r.title,order:r.order,icon:'lucide:chart-column'}}));\nexport default routes;`);
console.log(`Extracted ${modules.length} modules; ${sql.length} seed statements`);
