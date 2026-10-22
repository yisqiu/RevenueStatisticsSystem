<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import { useUserStore } from '@vben/stores';
import { date,range,search } from './filters';
const router=useRouter(),open=ref(false),userStore=useUserStore();
const routes=[['综合数据看板','/dashboard'],['营收排行榜','/ranking'],['文创馆销售','/module/wenchuang'],['豆乳奶茶销售','/module/naicha'],['票务数据','/module/ticket'],['汉服体验馆销售','/module/hanfu'],['讲解员接待销售','/module/guide'],['游览车营收','/module/bus'],['酒店民宿销售','/module/hotel'],['停车场营收','/module/park'],['线上文创点铺营收','/module/online'],['系统设置 / 报表导出','/settings'],['接口对接','/integrations']];
const matches=computed(()=>search.value?routes.filter(r=>r[0]?.includes(search.value) && (r[1]!=='/system'||userStore.userInfo?.super) && (r[1]!=='/integrations'||userStore.userInfo?.super||(userStore.userInfo?.permissions as string[]|undefined)?.includes('Integration.Manage'))):[]);
routes.push(['系统管理','/system']);
const dateMatch=computed(()=>{const d=new Date(search.value+'T00:00:00Z');return /^(20\d{2}|2100)-\d{2}-\d{2}$/.test(search.value)&&Number.isFinite(d.getTime())&&d.toISOString().slice(0,10)===search.value});
function go(path:string){open.value=false;search.value='';router.push(path)}
function applyDate(){date.value=search.value;search.value='';open.value=false}
function submit(){if(dateMatch.value)applyDate();else if(matches.value[0]?.[1])go(matches.value[0][1])}
</script>
<template><div class="revenue-filters"><div class="search-wrap"><label class="search"><span>⌕</span><input v-model="search" aria-label="搜索经营模块" placeholder="搜索模块 / 报表 / 日期" @focus="open=true" @input="open=true" @blur="open=false" @keydown.enter="submit" @keydown.esc="open=false" /></label><div v-if="open&&search&&(matches.length||dateMatch)" class="search-results"><button v-for="r in matches" :key="r[1]" @mousedown.prevent="go(r[1]!)">{{r[0]}}</button><button v-if="dateMatch" @mousedown.prevent="applyDate">查询 {{search}} 的营收</button></div></div><input v-model="date" class="date" type="date" aria-label="统计日期" min="2000-01-01" max="2100-12-31" /><button v-for="(label,key) in {day:'本日',month:'本月',year:'本年'}" :key="key" :class="{on:range===key}" @click="range=key">{{label}}</button></div></template>
<style scoped>
.revenue-filters{display:flex;align-items:center;gap:10px;margin-right:12px;font-family:"Microsoft YaHei",sans-serif}.search-wrap{position:relative}.search{display:flex;align-items:center;gap:8px;background:#fff;border:1px solid #e3d8c3;border-radius:20px;padding:8px 16px;width:230px;font-size:13px;color:#8a7a63}.search input{border:0;outline:0;background:transparent;width:100%;color:#2b2118}.search-results{position:absolute;top:100%;left:0;right:0;background:#fbf8f1;border:1px solid #e3d8c3;border-radius:8px;box-shadow:0 4px 20px #3c2d1920;z-index:30;max-height:260px;overflow:auto}.search-results button{display:block;width:100%;border:0;border-radius:0;text-align:left}.search-results button:hover{background:#f0e8d8}.date{width:132px;font-size:12px}.date,button{background:#fff;border:1px solid #e3d8c3;border-radius:8px;padding:7px 12px;color:#4a3b2c}button{font-size:12px;white-space:nowrap}button.on{background:#a63a2b;color:#fff;border-color:#a63a2b}button:hover{border-color:#b8863b}button:active{transform:translateY(1px)}@media(max-width:1250px){.search-wrap{display:none}}@media(max-width:800px){.date{width:115px}.revenue-filters{gap:4px;margin:0}button{padding:7px 8px}}
</style>
