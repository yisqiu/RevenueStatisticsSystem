<script setup lang="ts">
import { watch } from 'vue';
import { useRoute } from 'vue-router';
import { Button, Card, Spin } from 'ant-design-vue';
import { useAuthStore } from '#/store';

const route = useRoute();
const authStore = useAuthStore();
watch(() => route.fullPath, () => {
  if (route.path === '/auth/login' && !route.query.signedOut && !route.query.error) authStore.authLogin();
}, { immediate: true });
</script>

<template>
  <div class="flex min-h-dvh items-center justify-center bg-background p-6">
    <Card class="w-96 text-center" title="寿县统一身份认证">
      <Spin :spinning="authStore.loginLoading">
        <img src="/revenue-logo.png" alt="寿县古城景区" class="mx-auto mb-6 h-16 object-contain" />
        <p class="mb-6 text-muted-foreground">{{ route.query.error ? '认证回调失败，请重新登录' : route.query.signedOut ? '已退出营收统计系统' : '正在前往统一身份认证平台' }}</p>
        <Button type="primary" block @click="authStore.authLogin">统一身份认证登录</Button>
      </Spin>
    </Card>
  </div>
</template>
