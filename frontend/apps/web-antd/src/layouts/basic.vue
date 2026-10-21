<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import RevenueFilters from '#/views/revenue/filters.vue';
import { useWatermark } from '@vben/hooks';
import { BasicLayout, LockScreen, Notification, UserDropdown } from '@vben/layouts';
import { preferences, updatePreferences, usePreferences } from '@vben/preferences';
import { useUserStore } from '@vben/stores';
import { message } from 'ant-design-vue';
import { useAuthStore } from '#/store';

const router = useRouter();
const route = useRoute();
const userStore = useUserStore();
const authStore = useAuthStore();
const authError = ref('');
const { isDark, isMobile } = usePreferences();
watch(() => route.path, () => { if (isMobile.value) updatePreferences({ sidebar: { collapsed: true } }); });
const pageTitle=computed(()=>route.meta.headerTitle||route.meta.title);
const { destroyWatermark, updateWatermark } = useWatermark();
const avatar = computed(() => userStore.userInfo?.avatar || '/revenue-avatar.svg');
const menus = [{ icon: 'lucide:user-round', text: '个人中心', handler: () => router.push('/profile') }];
async function logout() {
  try { await authStore.logout(); } catch (error) { message.error((error as Error).message); }
}
function handleAuthError(event: Event) { authError.value = (event as CustomEvent<string>).detail; }
onMounted(() => window.addEventListener('team-auth-error', handleAuthError));
onUnmounted(() => window.removeEventListener('team-auth-error', handleAuthError));
watch(() => [preferences.app.watermark, preferences.app.watermarkContent, isDark.value], async () => {
  if (preferences.app.watermark) await updateWatermark({ content: preferences.app.watermarkContent || userStore.userInfo?.username || '' });
  else destroyWatermark();
}, { immediate: true });
</script>

<template>
  <BasicLayout @clear-preferences-and-logout="logout">
    <template #logo-text>
      <div class="revenue-brand"><strong>寿县古城景区</strong><span>SHOUXIAN ANCIENT CITY</span><small>营收统计系统</small></div>
    </template>
    <template #header-left-1><div class="revenue-heading"><h1>{{pageTitle}}</h1><div>首页 · {{route.path.startsWith('/module/')?'经营模块 · '+pageTitle:route.path==='/settings'||route.path==='/integrations'?'系统':'经营总览'}}</div></div></template>
    <template #header-right-1><RevenueFilters /></template>
    <template #user-dropdown>
      <UserDropdown :avatar :menus :text="userStore.userInfo?.realName || userStore.userInfo?.username" :tag-text="String(userStore.userInfo?.role || '业务人员')" :description="`统一身份认证 · ${userStore.userInfo?.username}`" @logout="logout" @clear-preferences-and-logout="logout" />
    </template>
    <template #notification><Notification :dot="false" :notifications="[]" /></template>
    <template #lock-screen><LockScreen :avatar @to-login="logout" /></template>
    <template #extra>
      <div v-if="authError" class="team-session-error" role="alert"><span>{{ authError }}</span><a href="/api/auth/login">重新登录</a></div>
    </template>
  </BasicLayout>
</template>

<style scoped>
.team-brand { text-align: left; white-space: nowrap; line-height: 20px; }
.team-brand strong { display: block; font-size: 14px; font-weight: 600; color: #252a31; }
.team-brand span { display: block; font-size: 12px; color: #6d7b8a; }
.team-session-error { position: fixed; top: 96px; right: 24px; z-index: 1000; display: flex; gap: 24px; padding: 16px 24px; background: #fff2f3; color: #9c1d2d; border: 1px solid #efb7bd; border-radius: 8px; }
.team-session-error a { text-decoration: underline; }
</style>
