import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { resetAllStores, useAccessStore, useUserStore } from '@vben/stores';
import { defineStore } from 'pinia';
import { fetchOidcUserInfo, startOidcLogin, startOidcLogout } from '#/auth/oidc';

export const useAuthStore = defineStore('auth', () => {
  const accessStore = useAccessStore();
  const userStore = useUserStore();
  const router = useRouter();
  const loginLoading = ref(false);
  function authLogin() {
    loginLoading.value = true;
    const redirect = router.currentRoute.value.query.redirect;
    startOidcLogin(typeof redirect === 'string' ? decodeURIComponent(redirect) : '/dashboard');
  }
  async function fetchUserInfo() {
    const user = await fetchOidcUserInfo();
    userStore.setUserInfo(user);
    accessStore.setAccessCodes(user.permissions);
    return user;
  }
  async function logout() {
    await startOidcLogout();
    resetAllStores();
  }
  return { authLogin, fetchUserInfo, loginLoading, logout, $reset: () => { loginLoading.value = false; } };
});
