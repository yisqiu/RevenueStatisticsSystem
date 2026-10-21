<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue';
import Chart from 'chart.js/auto';
const props=defineProps<{ kind: string; labels: string[]; values: number[]; secondary?: number[] }>();
const canvas=ref<HTMLCanvasElement>();let chart:Chart|undefined;
const palette=['#a63a2b','#b8863b','#3f7d6e','#8a6d3b','#c98b4b','#6a8caf','#9c6b8e','#7a8b5a','#b5786b'];
function render(){if(!canvas.value)return;chart?.destroy();const pie=props.kind==='pie';const line=props.kind==='line';const week=props.kind==='week';
const sets:any[]=[{type:pie?'doughnut':line?'line':'bar',label:'营收(万元)',data:props.values,borderColor:line?'#a63a2b':pie?'#fbf8f1':undefined,borderWidth:pie?2:line?2.5:0,backgroundColor:line?(ctx:any)=>{const g=ctx.chart.ctx.createLinearGradient(0,0,0,280);g.addColorStop(0,'rgba(166,58,43,.28)');g.addColorStop(1,'rgba(166,58,43,0)');return g}:week?'rgba(184,134,59,.75)':palette,fill:line,tension:.38,pointRadius:3,borderRadius:pie?0:6}];
if(props.secondary)sets.push({type:'line',label:week?'客流(千人次)':'门票收入',data:props.secondary,borderColor:week?'#3f7d6e':'#b8863b',borderDash:week?[]:[5,4],borderWidth:2,tension:.35,pointRadius:2,yAxisID:week?'y1':'y'});
Chart.defaults.font.family='"PingFang SC","Microsoft YaHei",sans-serif';Chart.defaults.color='#8a7a63';
chart=new Chart(canvas.value,{type:pie?'doughnut':line?'line':'bar',data:{labels:props.labels,datasets:sets},options:{responsive:true,maintainAspectRatio:false,animation:false,indexAxis:props.kind==='rank'?'y':'x',...(pie?{cutout:'58%'}:{}),plugins:{legend:{display:pie||line||week,position:pie?'right':'top',labels:{boxWidth:10,font:{size:11},padding:8}}},scales:pie?{}:{x:{grid:{display:props.kind==='rank'},ticks:{font:{size:11}}},y:{grid:{color:'#ece3d1'},ticks:{font:{size:11}}},...(week?{y1:{position:'right',grid:{display:false}}}:{})}}} as any)}
onMounted(render);watch(()=>[props.kind,props.values,props.secondary,props.labels],render,{deep:true});onBeforeUnmount(()=>chart?.destroy());
</script>
<template><div class="chart-box"><canvas ref="canvas"></canvas></div></template>
<style scoped>.chart-box{height:280px;position:relative;width:100%;min-width:0}</style>
