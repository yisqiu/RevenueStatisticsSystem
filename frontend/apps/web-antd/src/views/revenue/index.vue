<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useUserStore } from '@vben/stores';
import { message, Modal } from 'ant-design-vue';
import RevenueChart from './chart.vue';
import { date, range, search, today } from './filters';
import { requestBlob, requestJson } from './request';

type RevenueModule={key:string;name:string;icon:string;desc:string;cols:string[];enabled:number;value:number;mom:number|null;visitors:number;trend:number[];rows:(string|number)[][];structLabels:string[];struct:Record<string,number>};
type RevenueRecord={id?:string;module:string;date:string;category:string;quantity:number;amount:string;visitors:number;source:string;note:string;version?:number};
type Settings={name:string;level:string;department:string;version:number};
type Integration={id?:string;name:string;module:string;enabled:number};
const route=useRoute(),router=useRouter(),userStore=useUserStore();
const pagePath=route.path;
const busy=ref(false),saving=ref(false),exporting=ref(false),error=ref('');
const data=ref<{modules:RevenueModule[];total:number;visitors:number;week:number[];weekVisitors:number[];year:string;months?:string[];trendFrom?:string;trendTo?:string;mom?:Record<string,number|null>}>({modules:[],total:0,visitors:0,week:[],weekVisitors:[],year:''});
const settings=ref<Settings>({name:'',level:'',department:'',version:1});
const settingsDraft=ref<Settings>({...settings.value});
const recordSearch=ref('');
const visibleRecords=computed(()=>records.value.filter(r=>!recordSearch.value||[r.date,r.category,r.source,r.note].some(v=>v.includes(recordSearch.value))));
const settingsEdit=ref(false),records=ref<RevenueRecord[]>([]),integrations=ref<Integration[]>([]),testResult=ref(''),apiToken=ref('');
const formOpen=ref(false),recordListOpen=ref(false),integrationOpen=ref(false),fileInput=ref<HTMLInputElement>();
const recordFormElement=ref<HTMLFormElement>(),settingsFormElement=ref<HTMLFormElement>(),integrationFormElement=ref<HTMLFormElement>();
const form=ref<RevenueRecord>({module:'ticket',date:today(),category:'',quantity:1,amount:'0.00',visitors:0,source:'手工录入',note:''});
const integration=ref<Integration>({name:'',module:'ticket',enabled:1});
const sampleRequest=JSON.stringify({sourceId:'source-20261021-001',record:{date:today(),category:'门票',quantity:3,amount:'90.00',visitors:3,note:'来源系统流水'}},null,2);
const isSuper=computed(()=>Boolean(userStore.userInfo?.super));
const canWrite=computed(()=>isSuper.value || (userStore.userInfo?.permissions as string[]|undefined)?.includes('Revenue.Write'));
const canManageIntegration=computed(()=>isSuper.value || (userStore.userInfo?.permissions as string[]|undefined)?.includes('Integration.Manage'));
const modules=computed(()=>data.value.modules.filter(m=>m.enabled===1));
const filteredModules=computed(()=>modules.value.filter(m=>!search.value||[m.name,m.desc].some(s=>s.includes(search.value))));
const module=computed(()=>modules.value.find(m=>route.path==='/module/'+m.key));
const titles:Record<string,string[]>={wenchuang:['文创产品销售趋势','品类结构占比','各品类销售明细'],naicha:['饮品销售趋势','产品结构占比','各产品线销售明细'],ticket:['门票营收趋势','票种结构占比','各票种售票明细'],hanfu:['汉服服务营收趋势','服务结构占比','各服务项目明细'],guide:['讲解服务营收趋势','服务结构占比','各服务类型明细'],bus:['游览车营收趋势','线路结构占比','各线路运营明细'],hotel:['住宿营收趋势','房型结构占比','各住宿类型明细'],park:['停车营收趋势','场站结构占比','各停车场收费明细'],online:['线上营收趋势','渠道结构占比','各渠道销售明细']};
const ranked=computed(()=>[...filteredModules.value].sort((a,b)=>b.value-a.value));
const period=computed(()=>({day:'本日',month:'本月',year:'本年'}[range.value]||'本月'));
const moduleRows=computed(()=>module.value?.rows.filter(r=>!search.value||r.some(v=>String(v).includes(search.value)))||[]);
function visitorUnit(key?:string){return key==='park'?'车次':key==='online'?'单':key==='wenchuang'?'人':'人次'}
const months=computed(()=>data.value.months||Array.from({length:12},(_,i)=>`${i+1}月`));
const trend=computed(()=>months.value.map((_,i)=>modules.value.reduce((n,m)=>n+(m.trend[i]||0),0)));
const ticket=computed(()=>modules.value.find(m=>m.key==='ticket'));
const kpis=computed(()=>{const m=module.value;if(m){const main=Object.entries(m.struct).filter(([,value])=>value>0).sort((a,b)=>b[1]-a[1])[0];return [
 {icon:m.icon,label:period.value+'营收',value:fmt(m.value),unit:'万元',delta:m.mom,hint:range.value==='day'?'较昨日':range.value==='year'?'较上年':'较上月'},
 {icon:'▲',label:'环比增长',value:m.mom===null?'—':(m.mom>=0?'+':'')+fmt(m.mom),unit:'%',hint:m.mom===null?'暂无对比数据':m.mom>=0?'经营态势向好':'较上一周期下降'},
 {icon:'客',label:'接待量',value:(m.visitors/10000).toFixed(2),unit:'万'+visitorUnit(m.key),hint:'累计统计'},
 {icon:'均',label:'人均消费',value:'¥'+fmt(m.visitors?m.value*10000/m.visitors:0),unit:'/'+visitorUnit(m.key),hint:'客单价水平'},
 {icon:'冠',label:'主力结构',value:main?.[0]||'暂无数据',unit:'',hint:'占比 '+fmt(main?.[1]||0)+'%'}]}
 return [{icon:'◈',label:period.value+'总营收',value:fmt(data.value.total),unit:'万元',delta:data.value.mom?.total??undefined,hint:range.value==='day'?'较昨日':range.value==='year'?'较上年':'较上月'},
 {icon:'票',label:'门票收入',value:fmt(ticket.value?.value||0),unit:'万元',delta:ticket.value?.mom,hint:range.value==='day'?'较昨日':range.value==='year'?'较上年':'较上月'},
 {icon:'缸',label:'二次消费收入',value:fmt(data.value.total-(ticket.value?.value||0)),unit:'万元',delta:data.value.mom?.second??undefined,hint:range.value==='day'?'较昨日':range.value==='year'?'较上年':'较上月'},
 {icon:'客',label:period.value+'接待人次',value:fmt(data.value.visitors/10000),unit:'万人次',delta:data.value.mom?.visitors??undefined,hint:range.value==='day'?'较昨日':range.value==='year'?'较上年':'较上月'},
 {icon:'均',label:'人均消费',value:'¥'+fmt(data.value.visitors?data.value.total*10000/data.value.visitors:0),unit:'/人',delta:data.value.mom?.arpu??undefined,hint:range.value==='day'?'较昨日':range.value==='year'?'较上年':'较上月'}]});
function growthText(n:number|null){return n===null?'暂无对比':(n>=0?'▲ ':'▼ ')+fmt(Math.abs(n))+'%'}
function fmt(n:number|null){return n===null?'—':n.toLocaleString('zh-CN',{minimumFractionDigits:1,maximumFractionDigits:1})}
function query(){return new URLSearchParams({date:date.value,range:range.value}).toString()}
async function api<T>(url:string,init?:RequestInit):Promise<T>{
 return requestJson<T>('/api/'+url,init);
}
let loadSequence=0;
async function load(){if(route.path!==pagePath)return;const sequence=++loadSequence;busy.value=true;error.value='';try{const [summary,config,interfaces]=await Promise.all([api<typeof data.value>('summary?'+query()),api<Settings>('settings'),route.path==='/integrations'&&canManageIntegration.value?api<Integration[]>('integrations'):Promise.resolve(undefined)]);if(sequence!==loadSequence)return;data.value=summary;settings.value=config;if(interfaces)integrations.value=interfaces}catch(e){if(sequence===loadSequence)error.value=(e as Error).message}finally{if(sequence===loadSequence)busy.value=false}}
watch([()=>route.path,date,range],load,{immediate:true});
function newRecord(){form.value={module:module.value?.key||'ticket',date:date.value,category:'',quantity:1,amount:'0.00',visitors:0,source:'手工录入',note:''};formOpen.value=true}
async function saveRecord(){if(saving.value||!recordFormElement.value?.reportValidity())return;saving.value=true;try{await api('records',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...form.value,amount:String(form.value.amount)})});formOpen.value=false;message.success('日报已保存');await load();if(recordListOpen.value)await showRecords()}catch(e){message.error((e as Error).message)}finally{saving.value=false}}
async function showRecords(){try{records.value=await api<RevenueRecord[]>('records?'+query()+(module.value?'&module='+module.value.key:''));recordListOpen.value=true}catch(e){message.error((e as Error).message)}}
async function exportData(format:string,template=false,scope='current'){
 if(exporting.value)return;exporting.value=true;
 try{const blob=await requestBlob('/api/export?'+(scope==='month'?new URLSearchParams({date:date.value,range:'month'}).toString():query())+'&format='+format+(scope==='all'?'&all=1':'')+(template?'&template=1':'')+(module.value?'&module='+module.value.key:''));
 const url=URL.createObjectURL(blob),link=document.createElement('a');link.href=url;link.download=template?'营收导入模板.xlsx':scope==='all'?'营收全部明细.csv':'营收明细.'+format;link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);message.success('导出文件已生成');
 }catch(e){message.error((e as Error).message)}finally{exporting.value=false}
}
async function importFile(event:Event){const input=event.target as HTMLInputElement;const file=input.files?.[0];if(!file)return;const body=new FormData();body.append('file',file);saving.value=true;try{const out=await api<{imported:number}>('import',{method:'POST',body});message.success(`已导入 ${out.imported} 条日报`);await load()}catch(e){message.error((e as Error).message)}finally{saving.value=false;input.value=''}}
async function saveSettings(){if(saving.value||!settingsFormElement.value?.reportValidity())return;saving.value=true;try{await api('settings',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(settingsDraft.value)});settingsEdit.value=false;await load();message.success('设置已保存')}catch(e){message.error((e as Error).message)}finally{saving.value=false}}
async function saveIntegration(){if(saving.value||!integrationFormElement.value?.reportValidity())return;saving.value=true;try{const out=await api<{token:string}>('integrations',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(integration.value)});apiToken.value=out.token;integrationOpen.value=false;await load();message.success('接口配置已保存')}catch(e){message.error((e as Error).message)}finally{saving.value=false}}
async function testInterface(){try{testResult.value=JSON.stringify(await api('integration-test'),null,2)}catch(e){message.error((e as Error).message)}}
function editRecord(r:RevenueRecord){form.value={...r};formOpen.value=true}
</script>

<template>
 <div class="revenue-app">
  <div class="page">
   <div v-if="error" class="note error" role="alert">{{error}} <button class="btn" @click="load">重新加载</button></div>
   <div v-if="busy" class="loading-skeleton" aria-label="正在加载"><div v-for="n in 5" :key="n"></div></div>
   <template v-else-if="!error">
   <section v-if="route.path==='/dashboard'||module">
    <div v-if="module" class="note"><b>{{module.name}}</b> —— {{module.desc}}。{{period}}实现营收 <b>{{fmt(module.value)}} 万元</b>，环比 {{growthText(module.mom)}}，累计接待 {{(module.visitors/10000).toFixed(2)}} 万{{visitorUnit(module.key)}}。</div>
    <div v-else class="note"><b>{{settings.name}}</b> —— 全国重点文物保护单位、国家历史文化名城核心区。本系统汇总景区内 9 大经营性业态的营收数据，覆盖文创、餐饮、门票、体验、服务、住宿、停车及线上渠道，支持按日 / 月 / 年多维度统计。</div>
    <div class="kpis"><div v-for="(k,i) in kpis" :key="k.label" class="kpi" :class="{hero:i===0}"><div class="lab"><span class="ic">{{k.icon}}</span>{{k.label}}</div><div class="val" :class="{'small-value':i===4&&module}" :style="module&&i===1?{color:(module.mom??0)>=0?'var(--up)':'var(--down)'}:{}">{{k.value}}<small>{{k.unit}}</small></div><div class="delta" :class="(k.delta||0)>=0?'up':'down'"><span v-if="k.delta!==undefined&&k.delta!==null" class="tag">{{k.delta>=0?'▲':'▼'}} {{fmt(Math.abs(k.delta??0))}}%</span>{{k.hint}}</div></div></div>
    <div class="grid-2" style="margin-bottom:18px">
     <div class="card"><div class="card-head"><span class="bar"></span><h3>{{module?titles[module.key]?.[0]:'近 12 个月营收趋势'}}</h3><span class="sub">{{data.trendFrom}} 至 {{data.trendTo}} · 万元</span></div><RevenueChart :kind="module?'module':'line'" :label="module?module.name+'营收(万元)':'总营收'" :labels="months" :values="module?.trend||trend" :secondary="module?undefined:ticket?.trend" /></div>
     <div class="card"><div class="card-head"><span class="bar"></span><h3>{{module?titles[module.key]?.[1]:'各业态营收占比'}}</h3><span class="sub">{{period}}</span></div><RevenueChart kind="pie" :unit="module?'%':'万元'" :labels="module?module.structLabels:modules.map(m=>m.name)" :values="module?module.structLabels.map(k=>module?.struct[k]||0):modules.map(m=>m.value)" /></div>
    </div>
    <template v-if="!module">
     <div class="grid-2" style="margin-bottom:22px"><div class="card"><div class="card-head"><span class="bar"></span><h3>各模块营收对比</h3><span class="sub">{{period}} · 万元</span></div><RevenueChart kind="bar" :height="260" :labels="modules.map(m=>m.name.replace('销售','').replace('营收','').replace('数据',''))" :values="modules.map(m=>m.value)" /></div><div class="card"><div class="card-head"><span class="bar"></span><h3>本周客流与营收</h3><span class="sub">人次 / 万元</span></div><RevenueChart kind="week" :height="260" :labels="['周一','周二','周三','周四','周五','周六','周日']" :values="data.week" :secondary="data.weekVisitors.map(n=>n/1000)" /></div></div>
     <div class="sec-title">九大业态经营概况</div><div class="mods"><button v-for="m in filteredModules" :key="m.key" class="mod" @click="router.push('/module/'+m.key)"><div class="top"><div class="mico">{{m.icon}}</div><div><div class="mname">{{m.name}}</div><div class="mdesc">{{m.desc}}</div></div></div><div class="mrow"><div class="mnum">{{fmt(m.value)}}<small>万元</small></div><div class="delta" :class="(m.mom??0)>=0?'up':'down'"><span class="tag">{{growthText(m.mom)}}</span></div></div><div class="progress"><i :style="{width:(data.total?m.value/data.total*100:0)+'%'}"></i></div><div class="mfoot"><span>占比 {{fmt(data.total?m.value/data.total*100:0)}}%</span><span>接待 {{(m.visitors/10000).toFixed(2)}}万{{visitorUnit(m.key)}}</span></div></button></div><p v-if="!filteredModules.length" class="empty">没有匹配的经营模块</p>
    </template>
    <div v-else class="card"><div class="card-head"><span class="bar"></span><h3>{{titles[module.key]?.[2]}}</h3><span class="sub">{{period}}</span></div><div class="toolbar"><button v-if="canWrite" class="btn primary" @click="newRecord">＋ 新增日报数据</button><button class="btn" @click="showRecords">日报记录</button><button class="btn" :disabled="exporting" @click="exportData('xlsx')">导出明细</button></div><div class="tbl-wrap"><table><thead><tr><th v-for="c in module.cols" :key="c">{{c}}</th></tr></thead><tbody><tr v-for="(row,i) in moduleRows" :key="i"><td v-for="(v,j) in row" :key="j" :class="{num:j>0}"><b v-if="j===0">{{v}}</b><template v-else>{{j>0&&j<4?Number(v).toLocaleString('zh-CN',{maximumFractionDigits:2}):v}}</template></td></tr><tr v-if="!moduleRows.length"><td colspan="5" class="empty">当前周期暂无营收数据，可新增日报或通过接口接入。</td></tr></tbody></table></div></div>
   </section>
   <section v-else-if="route.path==='/ranking'">
    <div class="grid-2" style="margin-bottom:18px"><div class="card"><div class="card-head"><span class="bar"></span><h3>业态营收排行榜</h3><span class="sub">{{period}}</span></div><div v-for="(m,i) in ranked" :key="m.key" class="rank-row"><div class="rank-no" :class="'n'+(i+1)">{{i+1}}</div><div class="rank-name">{{m.name}}</div><div class="rank-bar"><i :style="{width:(ranked[0]?.value?m.value/ranked[0].value*100:0)+'%'}"></i></div><div class="rank-val">{{fmt(m.value)}} 万</div></div></div><div class="card"><div class="card-head"><span class="bar"></span><h3>排行榜可视化</h3></div><RevenueChart kind="rank" :height="280" :labels="ranked.map(m=>m.name)" :values="ranked.map(m=>m.value)" /></div></div>
    <div class="card"><div class="card-head"><span class="bar"></span><h3>各业态关键指标明细</h3><span class="sub">{{period}}汇总</span></div><div class="tbl-wrap"><table><thead><tr><th>排名</th><th>经营模块</th><th>{{period}}营收(万元)</th><th>占比</th><th>环比</th><th>接待量</th><th>状态</th></tr></thead><tbody><tr v-for="(m,i) in ranked" :key="m.key"><td class="num">{{i+1}}</td><td><b>{{m.name}}</b></td><td class="num">{{fmt(m.value)}}</td><td class="num">{{fmt(data.total?m.value/data.total*100:0)}}%</td><td class="num">{{growthText(m.mom)}}</td><td>{{(m.visitors/10000).toFixed(2)}}万{{visitorUnit(m.key)}}</td><td><span class="tag-s" :class="(m.mom??0)>=20?'tag-hot':(m.mom??0)>=10?'tag-ok':'tag-warn'">{{(m.mom??0)>=20?'高增长':(m.mom??0)>=10?'表现良好':'平稳'}}</span></td></tr></tbody></table></div></div>
   </section>
   <section v-else-if="route.path==='/settings'">
    <div class="grid-2e settings-grid"><div class="card"><div class="card-head"><span class="bar"></span><h3>景区基础信息</h3><button v-if="isSuper" class="btn" style="margin-left:auto" @click="settingsDraft={...settings};settingsEdit=true">编辑</button></div><table class="settings-table"><tbody><tr><td>景区名称</td><td>{{settings.name}}</td></tr><tr><td>景区等级</td><td>{{settings.level}}</td></tr><tr><td>统计周期</td><td>{{data.year}}-01 至 {{data.year}}-12（自然月）</td></tr><tr><td>币种</td><td>人民币（元）</td></tr><tr><td>数据管理员</td><td>{{settings.department}}</td></tr></tbody></table></div><div class="card"><div class="card-head"><span class="bar"></span><h3>模块启用状态</h3></div><table class="settings-table"><tbody><tr v-for="m in data.modules" :key="m.key"><td style="width:40px"><div class="settings-icon">{{m.icon}}</div></td><td><b>{{m.name}}</b><div class="mdesc">{{m.desc}}</div></td><td style="text-align:right"><span class="tag-s" :class="m.enabled?'tag-ok':'tag-off'">{{m.enabled?'已启用':'已停用'}}</span></td></tr></tbody></table></div></div>
    <div class="card" style="margin-top:18px"><div class="card-head"><span class="bar"></span><h3>数据录入与导出</h3></div><div class="btns"><button v-if="canWrite" class="btn primary" @click="newRecord">＋ 新增日报数据</button><button v-if="canWrite" class="btn" :disabled="saving" @click="fileInput?.click()">批量导入（Excel）</button><button class="btn" :disabled="exporting" @click="exportData('xlsx',false,'month')">导出月报（Excel）</button><button class="btn" :disabled="exporting" @click="exportData('csv',false,'all')">导出全部明细（CSV）</button><button class="btn" :disabled="exporting" @click="exportData('xlsx',true)">下载导入模板</button><button class="btn" @click="showRecords">日报记录</button></div><div class="note" style="margin:16px 0 0">查询与明细导出按顶部日期和周期；月报导出选定日期所在月，全部CSV导出所有日期；导入采用模板字段，全部校验通过后保存。测试记录在来源列标记为“原型测试数据”。</div></div>
   </section>
   <section v-else-if="route.path==='/integrations'">
    <div class="note">其他系统可通过专属接口密钥上报营收数据。每个接口绑定一个经营业态，按来源编号去重。</div>
    <div class="card"><div class="card-head"><span class="bar"></span><h3>系统接口配置</h3></div><div class="btns"><button v-if="canManageIntegration" class="btn primary" @click="integration={name:'',module:'ticket',enabled:1};integrationOpen=true">新增对接系统</button><button class="btn" @click="testInterface">测试接口展示数据</button></div><div v-if="!canManageIntegration" class="note">当前角色可查看测试接口示例。</div><div v-else class="tbl-wrap" style="margin-top:18px"><table><thead><tr><th>对接系统</th><th>经营业态</th><th>状态</th><th>操作</th></tr></thead><tbody><tr v-for="i in integrations" :key="i.id"><td>{{i.name}}</td><td>{{modules.find(m=>m.key===i.module)?.name}}</td><td>{{i.enabled?'启用':'停用'}}</td><td><button class="btn" @click="integration={...i};integrationOpen=true">编辑</button></td></tr><tr v-if="!integrations.length"><td colspan="4" class="empty">暂无对接系统，点击“新增对接系统”创建专属接口。</td></tr></tbody></table></div><div v-if="apiToken" class="note" style="margin-top:16px">新密钥仅本次显示，请保存到对接系统：<code class="secret-token">{{apiToken}}</code><button class="btn" @click="apiToken=''">已保存</button></div></div>
    <div class="card" style="margin-top:18px"><div class="card-head"><span class="bar"></span><h3>接口说明</h3></div><p>POST /api/ingest</p><p>请求头：Authorization: Bearer 接口密钥</p><p>请求内容：sourceId 为来源系统唯一流水号，record 包含日期、品类、数量、金额和接待量。</p><pre>{{testResult||sampleRequest}}</pre></div>
   </section>
   </template>
  </div>
  <div class="foot">寿县古城景区营收统计系统 · 数据来源以日报记录为准</div>
  <input ref="fileInput" hidden type="file" accept=".xlsx" @change="importFile" />
  <Modal v-model:open="formOpen" :z-index="2001" :title="form.id?'编辑日报数据':'新增日报数据'" :confirm-loading="saving" :cancel-button-props="{disabled:saving}" :closable="!saving" :mask-closable="!saving" :keyboard="!saving" @ok="saveRecord" ok-text="保存" cancel-text="取消" wrap-class-name="revenue-modal" :width="640">
   <form ref="recordFormElement" class="revenue-form" @submit.prevent="saveRecord"><label>经营业态<select v-model="form.module" :disabled="saving"><option v-for="m in modules" :key="m.key" :value="m.key">{{m.name}}</option></select></label><label>业务日期<input v-model="form.date" :disabled="saving" type="date" min="2000-01-01" max="2100-12-31" required /></label><label>商品 / 服务品类<input v-model="form.category" :disabled="saving" maxlength="100" required /></label><label>数量<input v-model.number="form.quantity" :disabled="saving" type="number" min="0" max="100000000" step="1" required /></label><label>营收金额（元）<input v-model="form.amount" :disabled="saving" type="number" min="0" step="0.01" required /></label><label>接待量（{{visitorUnit(form.module)}}）<input v-model.number="form.visitors" :disabled="saving" type="number" min="0" max="100000000" step="1" required /></label><label>数据来源<input v-model="form.source" :disabled="saving" maxlength="100" /></label><label>备注<input v-model="form.note" :disabled="saving" maxlength="500" /></label><button type="submit" hidden>保存</button></form>
  </Modal>
  <Modal v-model:open="recordListOpen" title="日报记录" :footer="null" :width="1100" wrap-class-name="revenue-modal"><div class="record-query"><input v-model="recordSearch" placeholder="查询日期、品类、来源或备注" aria-label="查询日报记录" /><button class="btn" @click="recordSearch=''">清空</button></div><div class="record-scroll"><table><thead><tr><th>日期</th><th>业态</th><th>品类</th><th>数量</th><th>营收(元)</th><th>接待量</th><th>来源</th><th>操作</th></tr></thead><tbody><tr v-for="r in visibleRecords" :key="r.id"><td>{{r.date}}</td><td>{{modules.find(m=>m.key===r.module)?.name}}</td><td>{{r.category}}</td><td>{{r.quantity}}</td><td>{{r.amount}}</td><td>{{r.visitors}}</td><td>{{r.source}}</td><td><button v-if="canWrite" class="btn" @click="editRecord(r)">编辑</button></td></tr><tr v-if="!visibleRecords.length"><td colspan="8">当前周期暂无日报记录</td></tr></tbody></table></div></Modal>
  <Modal v-model:open="settingsEdit" title="景区基础信息" :confirm-loading="saving" :cancel-button-props="{disabled:saving}" :closable="!saving" :mask-closable="!saving" :keyboard="!saving" @ok="saveSettings" ok-text="保存" cancel-text="取消" wrap-class-name="revenue-modal"><form ref="settingsFormElement" class="revenue-form single" @submit.prevent="saveSettings"><label>景区名称<input v-model="settingsDraft.name" :disabled="saving" maxlength="200" required /></label><label>景区等级<input v-model="settingsDraft.level" :disabled="saving" maxlength="300" /></label><label>数据管理员<input v-model="settingsDraft.department" :disabled="saving" maxlength="100" /></label><button type="submit" hidden>保存</button></form></Modal>
  <Modal v-model:open="integrationOpen" title="对接系统配置" :confirm-loading="saving" :cancel-button-props="{disabled:saving}" :closable="!saving" :mask-closable="!saving" :keyboard="!saving" @ok="saveIntegration" ok-text="保存" cancel-text="取消" wrap-class-name="revenue-modal"><form ref="integrationFormElement" class="revenue-form single" @submit.prevent="saveIntegration"><label>系统名称<input v-model="integration.name" :disabled="saving" maxlength="100" required /></label><label>经营业态<select v-model="integration.module" :disabled="saving||Boolean(integration.id)"><option v-for="m in modules" :key="m.key" :value="m.key">{{m.name}}</option></select></label><label>接口状态<select v-model="integration.enabled" :disabled="saving"><option :value="1">启用</option><option :value="0">停用</option></select></label><button type="submit" hidden>保存</button></form></Modal>
 </div>
</template>

<style scoped src="./prototype.css">.settings-table .mdesc{margin-top:0}.settings-grid>.card:first-child .settings-table td:first-child{width:130px}.settings-grid>.card:first-child .card-head{position:relative}.settings-grid>.card:first-child .card-head>.btn{position:absolute;right:0;top:-8px}
</style>
<style scoped>
.revenue-form input:disabled,.revenue-form select:disabled{opacity:.65;cursor:default}
.revenue-app{font-family:"PingFang SC","Microsoft YaHei","Hiragino Sans GB",sans-serif;font-size:16px;color:var(--ink);background:var(--paper);line-height:normal;min-height:calc(100vh - 66px);display:flex;flex-direction:column;background-image:radial-gradient(circle at 20% 10%,rgba(184,134,59,.06),transparent 40%),radial-gradient(circle at 85% 80%,rgba(166,58,43,.05),transparent 45%)}
.revenue-topbar{height:66px;background:rgba(251,248,241,.92);border-bottom:1px solid var(--line);display:flex;align-items:center;padding:0 26px;gap:12px}.revenue-topbar h1{font-size:17px;font-weight:700;letter-spacing:1px;font-family:"STKaiti","KaiTi",serif}.date-filter{border:1px solid var(--line);border-radius:8px;background:#fff;padding:7px 9px;color:var(--ink-2);font-size:12px;width:136px}.small-value{font-size:19px!important}.mod{text-align:left}.settings-table{min-width:0}.settings-table td{white-space:normal}.settings-table td:first-child{color:var(--muted)}.mdesc{font-size:11px;color:var(--muted);margin-top:2px}.empty{text-align:center;padding:28px;color:var(--muted)}.error{background:#fce7e3;color:#922c20;display:flex;align-items:center;justify-content:space-between}.loading-skeleton{display:grid;grid-template-columns:repeat(5,1fr);gap:16px}.loading-skeleton div{height:128px;border-radius:14px;background:#e9dfcc}.card{min-width:0}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:#f0e8d8;padding:16px;border-radius:8px;margin-top:16px}.secret-token{display:block;overflow-wrap:anywhere;user-select:all}.btn:disabled{opacity:.5;cursor:wait}.btn:active{transform:translateY(1px)}
@media(max-width:1100px){.top-search{display:none}.revenue-topbar{flex-wrap:wrap;height:auto;min-height:66px;padding:12px 18px}.page{padding:18px}.kpis{grid-template-columns:repeat(2,minmax(0,1fr))}}
@media(max-width:600px){.revenue-topbar{gap:8px}.revenue-topbar>div:first-child{width:100%}.topbar-spacer{display:none}.date-filter{flex:1}.rank-bar{width:60px}.kpis{grid-template-columns:1fr}.loading-skeleton{grid-template-columns:1fr}.mods{grid-template-columns:1fr}}
@media(prefers-reduced-motion:reduce){*{animation:none!important;transition:none!important}}
.settings-table .mdesc{margin-top:0}.settings-grid>.card:first-child .settings-table td:first-child{width:130px}.settings-grid>.card:first-child .card-head{position:relative}.settings-grid>.card:first-child .card-head>.btn{position:absolute;right:0;top:-8px}
</style>
