<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue';
import Chart from 'chart.js/auto';
const props=defineProps<{kind:string;labels:string[];values:number[];secondary?:number[];unit?:string;label?:string;height?:number}>();
const canvas=ref<HTMLCanvasElement>();let chart:Chart|undefined;
const palette=['#a63a2b','#b8863b','#3f7d6e','#8a6d3b','#c98b4b','#6a8caf','#9c6b8e','#7a8b5a','#b5786b'];
function render(){
 if(!canvas.value)return;chart?.destroy();
 const pie=props.kind==='pie',line=props.kind==='line'||props.kind==='module',week=props.kind==='week',rank=props.kind==='rank';
 const sets:any[]=[{type:pie?'doughnut':line?'line':'bar',label:props.label||(props.unit==='%'?'占比':'营收(万元)'),data:props.values,borderColor:line?'#a63a2b':pie?'#fbf8f1':undefined,borderWidth:pie?2:line?2.5:0,backgroundColor:line?(ctx:any)=>{const g=ctx.chart.ctx.createLinearGradient(0,0,0,260);g.addColorStop(0,'rgba(166,58,43,.26)');g.addColorStop(1,'rgba(166,58,43,0)');return g}:week?'rgba(184,134,59,.75)':palette,fill:line,tension:.38,pointRadius:3,pointBackgroundColor:'#a63a2b',pointBorderColor:'#fff',pointBorderWidth:1.5,borderRadius:pie?0:week?5:6,...(rank?{barThickness:18}:{})}];
 if(props.kind==='module')sets.push({type:'bar',label:'月度',data:props.values,backgroundColor:'rgba(184,134,59,.28)',borderRadius:4});
 if(props.secondary)sets.push({type:'line',label:week?'客流(千人次)':'门票收入',data:props.secondary,borderColor:week?'#3f7d6e':'#b8863b',pointBackgroundColor:week?'#3f7d6e':'#b8863b',borderDash:week?[]:[5,4],borderWidth:week?2.2:1.8,tension:week?.35:.38,pointRadius:week?3:2,yAxisID:week?'y1':'y',fill:false});
 Chart.defaults.font.family='"PingFang SC","Microsoft YaHei",sans-serif';Chart.defaults.color='#8a7a63';
 chart=new Chart(canvas.value,{type:pie?'doughnut':line?'line':'bar',data:{labels:props.labels,datasets:sets},options:{responsive:true,maintainAspectRatio:false,animation:false,indexAxis:rank?'y':'x',...(pie?{cutout:'58%'}:{}),plugins:{legend:{display:pie||line||week,position:pie?'right':'top',labels:{boxWidth:pie?9:12,font:{size:pie&&props.unit!=='%'?10.5:11},padding:pie&&props.unit!=='%'?7:pie?8:10}},tooltip:{mode:line?'index':'nearest',intersect:!line,callbacks:{label:(c:any)=>` ${pie?c.label:c.dataset.label||''}：${Number(c.raw).toLocaleString('zh-CN',{maximumFractionDigits:2})}${pie?props.unit||'万元':week&&c.datasetIndex===1?'千人次':'万元'}`}}},scales:pie?{}:{x:{grid:{display:rank,color:'#ece3d1'},ticks:rank?{callback:(v:any)=>v+'万'}:props.kind==='bar'?{font:{size:10},maxRotation:35}:{}},y:{grid:{display:!rank,color:'#ece3d1'},ticks:rank?{font:{size:11}}:{callback:(v:any)=>v+'万'}},...(week?{y1:{position:'right',grid:{display:false},ticks:{callback:(v:any)=>v+'千'}}}:{})}}} as any);
}
onMounted(render);watch(()=>[props.kind,props.values,props.secondary,props.labels],render,{deep:true});onBeforeUnmount(()=>chart?.destroy());
</script>
<template><div class="chart-box" :style="{height:(height||280)+'px'}"><canvas ref="canvas"></canvas></div></template>
<style scoped>.chart-box{position:relative;width:100%;min-width:0}</style>
