<script setup lang="ts">
import { date,range,search } from './filters';
</script>
<template><div class="revenue-filters"><label class="search"><span>⌕</span><input v-model="search" aria-label="搜索经营模块" placeholder="搜索模块 / 报表 / 日期" /></label><input v-model="date" class="date" type="date" aria-label="统计日期" min="2000-01-01" max="2100-12-31" /><button v-for="(label,key) in {day:'本日',month:'本月',year:'本年'}" :key="key" :class="{on:range===key}" @click="range=key">{{label}}</button></div></template>
<style scoped>
.revenue-filters{display:flex;align-items:center;gap:10px;margin-right:12px;font-family:"Microsoft YaHei",sans-serif}.search{display:flex;align-items:center;gap:8px;background:#fff;border:1px solid #e3d8c3;border-radius:20px;padding:8px 16px;width:230px;font-size:13px;color:#8a7a63}.search input{border:0;outline:0;background:transparent;width:100%;color:#2b2118}.date{width:132px;font-size:12px}.date,button{background:#fff;border:1px solid #e3d8c3;border-radius:8px;padding:7px 12px;color:#4a3b2c}button{font-size:12px;white-space:nowrap}button.on{background:#a63a2b;color:#fff;border-color:#a63a2b}button:hover{border-color:#b8863b}button:active{transform:translateY(1px)}@media(max-width:1250px){.search{display:none}}@media(max-width:800px){.date{width:115px}.revenue-filters{gap:4px;margin:0}button{padding:7px 8px}}
</style>
