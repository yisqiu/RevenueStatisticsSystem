<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue';
import { Modal, Select, message } from 'ant-design-vue';
import { systemApi as api } from './system-service';
import { requestBlob } from './request';

type Role = { code: string; name: string; description: string; permissions: string[]; enabled: boolean; builtIn: boolean; isSuperAdmin: boolean; version: number; members: number };
type User = { subject: string; username: string; displayName: string; phone: string; lastLogin: string; roles: string[]; enabled: boolean; version: number; isSuperAdmin: boolean; centralAllowed: boolean };
type Log = { id: string; time: string; actor: string; module: string; action: string; recordId: string; detail: string };
type Bootstrap = { users: User[]; roles: Role[]; directory: User[]; permissions: { key: string; name: string }[] };
const tabs = [{ key: 'users', title: '用户管理' }, { key: 'roles', title: '角色管理' }, { key: 'logs', title: '操作日志' }];
const active = ref('users');
const data = ref<Bootstrap>({ users: [], roles: [], directory: [], permissions: [] });
const loading = ref(false); const syncing = ref(false); const error = ref(''); const busyUser = ref('');
const userSearch = ref(''); const userStatus = ref(''); const userRole = ref(''); const userPage = ref(1);
const roleSearch = ref(''); const roleOpen = ref(false); const roleCreating = ref(false); const roleSaving = ref(false); const roleError = ref('');
const roleForm = reactive({ code: '', name: '', description: '', enabled: true, permissions: [] as string[], version: 0, isSuperAdmin: false });
const bindingOpen = ref(false); const bindingSaving = ref(false); const bindingError = ref(''); const bindingSubject = ref(''); const bindingRoles = ref<string[]>([]);
const logs = ref<Log[]>([]); const logTotal = ref(0); const logPage = ref(1); const logSize = ref(15); const logLoading = ref(false); const logExporting = ref(false); const logError = ref('');
const logFilter = reactive({ keyword: '', module: '', from: '', to: '' });
const modules = ['文创馆销售','豆乳奶茶销售','票务数据','汉服体验馆销售','讲解员接待销售','游览车营收','酒店民宿销售','停车场营收','线上文创点铺营收','营收日报','景区设置','系统接口','用户管理','角色管理'];
let logRequest = 0;
const roleOptions = computed(() => data.value.roles.map(r => ({ value: r.code, label: r.name + (r.isSuperAdmin ? '（统一认证指定）' : r.enabled ? '' : '（停用）'), disabled: r.isSuperAdmin || !r.enabled })));
const bindingOptions = computed(() => data.value.directory.filter(u => !u.isSuperAdmin && u.centralAllowed).map(u => ({ value: u.subject, label: `${u.displayName}（${u.username}）` })));
const bindingRoleOptions = computed(() => roleOptions.value.filter(r => !r.disabled));
function userRoleOptions(user: User) { return roleOptions.value.filter(r => r.value !== 'super_admin' && (!r.disabled || user.roles.includes(r.value))).map(r => ({ ...r, disabled: false })); }
const visibleUsers = computed(() => data.value.users.filter(u => `${u.username} ${u.displayName}`.toLowerCase().includes(userSearch.value.trim().toLowerCase()) && (!userStatus.value || String(u.enabled) === userStatus.value) && (!userRole.value || u.roles.includes(userRole.value))));
const userPages = computed(() => Math.max(1, Math.ceil(visibleUsers.value.length / 10)));
const pagedUsers = computed(() => visibleUsers.value.slice((userPage.value - 1) * 10, userPage.value * 10));
const visibleRoles = computed(() => data.value.roles.filter(r => `${r.name} ${r.code}`.toLowerCase().includes(roleSearch.value.trim().toLowerCase())));
const logPages = computed(() => Math.max(1, Math.ceil(logTotal.value / logSize.value)));
async function load() { loading.value = true; error.value = ''; try { data.value = await api('/system/bootstrap'); userPage.value = Math.min(userPage.value, userPages.value); } catch (e) { error.value = (e as Error).message; } finally { loading.value = false; } }
async function syncUsers() { if (syncing.value) return; syncing.value = true; error.value = ''; try { const result = await api('/system/sync-users', 'POST'); await load(); message.success(`已同步 ${result.count} 个统一账号`); } catch (e) { error.value = (e as Error).message; } finally { syncing.value = false; } }
function selectTab(key: string) { active.value = key; }
function tabKey(event: KeyboardEvent, index: number) {
  let next = index;
  if (event.key === 'ArrowRight') next = (index + 1) % tabs.length;
  else if (event.key === 'ArrowLeft') next = (index + tabs.length - 1) % tabs.length;
  else if (event.key === 'Home') next = 0;
  else if (event.key === 'End') next = tabs.length - 1;
  else return;
  event.preventDefault(); void selectTab(tabs[next]!.key); document.getElementById(`system-tab-${tabs[next]!.key}`)?.focus();
}
async function saveUser(user: User) {
  if (busyUser.value) return;
  busyUser.value = user.subject;
  try { await api(`/system/users/${encodeURIComponent(user.subject)}`, 'PUT', { roles: user.roles, version: user.version }); await load(); message.success('用户角色已保存'); }
  catch (e) { message.error((e as Error).message); }
  finally { busyUser.value = ''; }
}
function toggleUser(user: User) {
  Modal.confirm({ title: `${user.enabled ? '停用' : '启用'}用户“${user.username}”？`, content: user.enabled ? '停用后该用户的本系统会话立即失效。' : '启用后，已分配有效角色的用户可重新登录本系统。', okText: '确认', cancelText: '取消', onOk: async () => {
    busyUser.value = user.subject;
    try { await api(`/system/users/${encodeURIComponent(user.subject)}/status`, 'PUT', { enabled: !user.enabled, version: user.version }); await load(); message.success('用户状态已更新'); }
    catch (e) { message.error((e as Error).message); throw e; }
    finally { busyUser.value = ''; }
  } });
}
function openBinding() { bindingSubject.value = ''; bindingRoles.value = []; bindingError.value = ''; bindingOpen.value = true; }
function selectBinding() { bindingRoles.value = [...(data.value.users.find(u => u.subject === bindingSubject.value)?.roles || [])]; }
async function bindUser() {
  if (bindingSaving.value) return;
  if (!bindingSubject.value || !bindingRoles.value.length) { bindingError.value = '请选择统一账号和至少一个本系统角色'; return; }
  bindingSaving.value = true; bindingError.value = '';
  try { const existing = data.value.users.find(u => u.subject === bindingSubject.value); await api('/system/users', 'POST', { subject: bindingSubject.value, roles: bindingRoles.value, version: existing?.version || 0 }); bindingOpen.value = false; await load(); message.success('系统用户已绑定角色'); }
  catch (e) { bindingError.value = (e as Error).message; }
  finally { bindingSaving.value = false; }
}
async function editRole(role?: Role) {
  roleCreating.value = !role; roleError.value = '';
  Object.assign(roleForm, role ? { ...role, permissions: [...role.permissions] } : { code: '', name: '', description: '', enabled: true, permissions: ['Revenue.Read'], version: 0, isSuperAdmin: false });
  roleOpen.value = true; await nextTick();
}
function togglePermission(key: string, checked: boolean) {
  const keys = new Set(roleForm.permissions); checked ? keys.add(key) : keys.delete(key);
  roleForm.permissions = [...keys];
}
async function saveRole() {
  if (roleForm.isSuperAdmin || roleSaving.value) return;
  roleError.value = '';
  if (!roleForm.name.trim() || !roleForm.code.trim() || !roleForm.description.trim()) { roleError.value = '请填写角色名称、编码和描述'; return; }
  roleSaving.value = true;
  try {
    const { code, name, description, enabled, permissions, version } = roleForm;
    await api(`/system/roles${roleCreating.value ? '' : `/${encodeURIComponent(code)}`}`, roleCreating.value ? 'POST' : 'PUT', { code, name, description, enabled, permissions, version });
    roleOpen.value = false; await load(); message.success(roleCreating.value ? '角色已新增' : '角色已保存');
  } catch (e) { roleError.value = (e as Error).message; }
  finally { roleSaving.value = false; }
}
function deleteRole(role: Role) {
  Modal.confirm({ title: `删除角色“${role.name}”？`, content: '已有绑定用户的角色须先解除绑定，删除操作将保留日志。', okText: '删除', cancelText: '取消', okButtonProps: { danger: true }, onOk: async () => {
    try { await api(`/system/roles/${encodeURIComponent(role.code)}?version=${role.version}`, 'DELETE'); await load(); message.success('角色已删除'); }
    catch (e) { message.error((e as Error).message); throw e; }
  } });
}
function logQuery() { return new URLSearchParams({ ...logFilter, page: String(logPage.value), size: String(logSize.value) }).toString(); }
async function loadLogs() {
  const current = ++logRequest; logLoading.value = true; logError.value = '';
  try { const result = await api(`/system/logs?${logQuery()}`); if (current === logRequest) { logs.value = result.items; logTotal.value = result.total; } }
  catch (e) { if (current === logRequest) logError.value = (e as Error).message; }
  finally { if (current === logRequest) logLoading.value = false; }
}
function searchLogs() { logPage.value = 1; void loadLogs(); }
function resetLogs() { Object.assign(logFilter, { keyword: '', module: '', from: '', to: '' }); searchLogs(); }
function changeLogPage(delta: number) { logPage.value += delta; void loadLogs(); }
async function exportLogs() {
  if (logExporting.value) return;
  logExporting.value = true;
  try { const blob = await requestBlob(`/api/system/logs/export?${logQuery()}`); const url = URL.createObjectURL(blob); const link = document.createElement('a'); link.href = url; link.download = '营收统计系统操作日志.csv'; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000); message.success('日志已导出'); }
  catch (e) { message.error((e as Error).message); }
  finally { logExporting.value = false; }
}
watch([userSearch, userRole, userStatus], () => { userPage.value = 1; });
watch(active, tab => { if (tab === 'logs') void loadLogs(); });
onMounted(() => { void load(); if (active.value === 'logs') void loadLogs(); });
</script>

<template>
  <div class="revenue-surface revenue-content revenue-system">
    <div class="page-head"><div><div class="page-title"><span class="bar"></span>系统管理</div><div class="page-desc">统一维护系统用户、业务角色与操作记录</div></div><div class="page-head-sp"></div><button class="btn btn-ghost btn-sm" :disabled="loading || syncing" @click="active === 'logs' ? loadLogs() : load()">刷新</button></div>
    <div v-if="error" class="revenue-error" role="alert">{{ error }}<button class="btn btn-ghost btn-sm" @click="load">重试</button></div>
    <div class="card system-shell">
      <div class="system-tabs" role="tablist" aria-label="系统管理栏目"><button v-for="(tab, index) in tabs" :id="`system-tab-${tab.key}`" :key="tab.key" role="tab" :aria-selected="active === tab.key" :aria-controls="`system-panel-${tab.key}`" :tabindex="active === tab.key ? 0 : -1" :class="{ active: active === tab.key }" @click="selectTab(tab.key)" @keydown="tabKey($event, index)">{{ tab.title }}</button></div>
      <section v-show="active === 'users'" id="system-panel-users" role="tabpanel" aria-labelledby="system-tab-users" class="system-panel" :aria-busy="loading">
        <div class="system-toolbar"><div><h2>用户管理</h2><p>同步统一账号，分配本系统角色并管理启用状态。</p></div><div class="system-actions"><button class="btn btn-ghost" :disabled="syncing || loading" @click="syncUsers">{{ syncing ? '同步中…' : '同步用户' }}</button><button class="btn btn-primary" :disabled="loading" @click="openBinding">新增系统用户</button></div></div>
        <div class="system-filters"><div class="f-search"><span aria-hidden="true">⌕</span><input v-model="userSearch" placeholder="搜索账号 / 姓名" aria-label="搜索账号或姓名"></div><select v-model="userStatus" class="sel" aria-label="用户状态"><option value="">全部状态</option><option value="true">启用</option><option value="false">停用</option></select><select v-model="userRole" class="sel" aria-label="角色筛选"><option value="">全部角色</option><option v-for="role in data.roles" :key="role.code" :value="role.code">{{ role.name }}</option></select><span class="system-count">共 {{ visibleUsers.length }} 人</span></div>
        <div class="t-wrap"><table class="system-users-table"><thead><tr><th>用户名</th><th>显示名称</th><th>本系统角色</th><th>联系电话</th><th>最近登录</th><th>本系统状态</th><th>操作</th></tr></thead><tbody><tr v-for="user in pagedUsers" :key="user.subject" :data-subject="user.subject"><td>{{ user.username }}</td><td>{{ user.displayName }}</td><td class="system-user-roles"><span v-if="user.isSuperAdmin" class="tag tag-national">超级管理员（统一认证指定）</span><Select v-else v-model:value="user.roles" mode="multiple" :options="userRoleOptions(user)" :disabled="!!busyUser || loading" placeholder="请选择本系统角色" :aria-label="`${user.username} 的角色`" style="width:100%" /></td><td>{{ user.phone || '—' }}</td><td class="system-nowrap">{{ user.lastLogin || '—' }}</td><td><span class="tag" :class="user.enabled ? 'tag-pub' : 'tag-draft'">{{ user.enabled ? '启用' : '停用' }}</span><small v-if="!user.centralAllowed" class="system-account-note">中央许可未生效</small></td><td><div class="system-actions system-nowrap"><button class="btn btn-ghost btn-sm" :disabled="user.isSuperAdmin || !!busyUser" @click="toggleUser(user)">{{ user.enabled ? '停用' : '启用' }}</button><button class="btn btn-primary btn-sm" :disabled="user.isSuperAdmin || !!busyUser" @click="saveUser(user)">{{ busyUser === user.subject ? '保存中…' : '保存' }}</button></div></td></tr></tbody></table></div>
        <div v-if="loading" class="revenue-loading">正在读取用户…</div><div v-else-if="!visibleUsers.length" class="empty"><h4>{{ data.users.length ? '未找到匹配用户' : '暂无系统用户' }}</h4><p>{{ data.users.length ? '请调整搜索条件。' : '点击同步用户，获取具有本应用许可的统一账号。' }}</p></div>
        <div class="pager"><span class="info">共 {{ visibleUsers.length }} 人 · 第 {{ userPage }}/{{ userPages }} 页</span><div class="pages"><button class="pg" :disabled="userPage <= 1" @click="userPage--">‹ 上一页</button><button class="pg" :disabled="userPage >= userPages" @click="userPage++">下一页 ›</button></div></div>
      </section>
      <section v-show="active === 'roles'" id="system-panel-roles" role="tabpanel" aria-labelledby="system-tab-roles" class="system-panel" :aria-busy="loading">
        <div class="system-toolbar"><div class="f-search"><span aria-hidden="true">⌕</span><input v-model="roleSearch" placeholder="搜索角色名称 / 编码" aria-label="搜索角色"></div><button class="btn btn-primary" :disabled="loading" @click="editRole()">新增角色</button></div>
        <div class="system-role-grid"><article v-for="role in visibleRoles" :key="role.code" class="system-role-card" :data-role-code="role.code"><h3>{{ role.name }}</h3><code>{{ role.code }}</code><p class="system-role-description">{{ role.description }}</p><div class="system-role-tags"><span class="tag" :class="role.enabled ? 'tag-pub' : 'tag-draft'">{{ role.enabled ? '启用' : '停用' }}</span><span v-if="role.isSuperAdmin" class="tag tag-national">统一认证指定</span><span v-else-if="role.builtIn" class="tag tag-cat">内置角色</span></div><p class="system-role-summary">用户 {{ role.members }} 人 · {{ role.isSuperAdmin ? '全部权限' : `${role.permissions.length} 项权限` }}</p><div class="system-role-actions"><button class="btn btn-ghost btn-sm" @click="editRole(role)">{{ role.isSuperAdmin ? '查看权限' : '配置权限' }}</button><button class="btn btn-ghost btn-sm system-delete" :disabled="role.isSuperAdmin || role.builtIn" :title="role.builtIn ? '内置角色保留使用' : ''" @click="deleteRole(role)">删除</button></div></article></div>
        <div v-if="loading" class="revenue-loading">正在读取角色…</div><div v-else-if="!visibleRoles.length" class="empty"><h4>未找到匹配角色</h4><p>请调整搜索条件或新增角色。</p></div>
      </section>
      <section v-show="active === 'logs'" id="system-panel-logs" role="tabpanel" aria-labelledby="system-tab-logs" class="system-panel" :aria-busy="logLoading">
        <form class="system-filters system-log-filters" @submit.prevent="searchLogs"><div class="f-search"><span aria-hidden="true">⌕</span><input v-model="logFilter.keyword" maxlength="200" placeholder="搜索操作人 / 操作 / 记录编号" aria-label="搜索操作日志"></div><select v-model="logFilter.module" class="sel" aria-label="日志模块"><option value="">全部模块</option><option v-for="module in modules" :key="module">{{ module }}</option></select><label class="system-date">开始日期<input v-model="logFilter.from" type="date" aria-label="开始日期"></label><label class="system-date">结束日期<input v-model="logFilter.to" type="date" aria-label="结束日期"></label><button class="btn btn-primary btn-sm" :disabled="logLoading" type="submit">查询</button><button class="btn btn-ghost btn-sm" type="button" @click="resetLogs">重置</button><button class="btn btn-ghost btn-sm" type="button" :disabled="logExporting || logLoading" @click="exportLogs">{{ logExporting ? '导出中…' : '导出日志' }}</button></form>
        <div v-if="logError" class="revenue-error" role="alert">{{ logError }}</div><div class="t-wrap"><table class="system-logs-table"><thead><tr><th>操作时间</th><th>操作人</th><th>模块</th><th>操作</th><th>记录编号</th><th>详情</th></tr></thead><tbody><tr v-for="log in logs" :key="log.id"><td class="system-nowrap">{{ log.time }}</td><td>{{ log.actor }}</td><td class="system-nowrap">{{ log.module }}</td><td class="system-nowrap">{{ log.action }}</td><td class="system-record">{{ log.recordId }}</td><td class="system-log-detail">{{ log.detail }}</td></tr></tbody></table></div>
        <div v-if="logLoading" class="revenue-loading">正在查询日志…</div><div v-else-if="!logs.length && !logError" class="empty"><h4>暂无匹配日志</h4><p>请调整查询条件。</p></div><div class="pager"><span class="info">共 {{ logTotal }} 条 · 第 {{ logPage }}/{{ logPages }} 页</span><div class="pages"><select v-model="logSize" class="sel" aria-label="每页日志数量" @change="searchLogs"><option :value="15">15 条/页</option><option :value="30">30 条/页</option><option :value="50">50 条/页</option></select><button class="pg" :disabled="logLoading || logPage <= 1" @click="changeLogPage(-1)">‹ 上一页</button><button class="pg" :disabled="logLoading || logPage >= logPages" @click="changeLogPage(1)">下一页 ›</button></div></div>
      </section>
    </div>
    <Modal v-model:open="bindingOpen" title="新增系统用户" :footer="null" :width="520" centered :mask-closable="!bindingSaving" :closable="!bindingSaving" :keyboard="!bindingSaving" wrap-class-name="revenue-surface revenue-system-dialog">
      <form class="system-dialog-form" @submit.prevent="bindUser"><p class="system-dialog-hint">选择统一账号，并绑定本系统业务角色。</p><label class="system-form-label">统一账号 <em>*</em><Select v-model:value="bindingSubject" show-search option-filter-prop="label" :options="bindingOptions" :disabled="bindingSaving" :placeholder="bindingOptions.length ? '请选择统一账号' : '暂无可绑定的统一账号'" aria-label="统一账号" @change="selectBinding" /></label><label class="system-form-label">本系统角色 <em>*</em><Select v-model:value="bindingRoles" mode="multiple" :options="bindingRoleOptions" :disabled="bindingSaving" placeholder="请选择本系统角色" aria-label="新增用户的角色" /></label><p v-if="!bindingOptions.length" class="system-dialog-hint">具有本应用访问许可的普通统一账号会显示在此处。</p><p v-if="bindingError" class="revenue-error" role="alert">{{ bindingError }}</p><div class="system-dialog-footer"><button type="button" class="btn btn-ghost" :disabled="bindingSaving" @click="bindingOpen = false">取消</button><button class="btn btn-primary" :disabled="bindingSaving || !bindingOptions.length" type="submit">{{ bindingSaving ? '保存中…' : '保存' }}</button></div></form>
    </Modal>
    <Modal v-model:open="roleOpen" :title="roleForm.isSuperAdmin ? '超级管理员权限' : roleCreating ? '新增角色' : '配置角色权限'" :footer="null" :width="680" centered :mask-closable="!roleSaving" :closable="!roleSaving" :keyboard="!roleSaving" wrap-class-name="revenue-surface revenue-system-dialog">
      <form class="system-dialog-form" @submit.prevent="saveRole"><div class="system-identity-grid"><label class="system-form-label">角色名称 <em>*</em><input v-model="roleForm.name" :disabled="roleForm.isSuperAdmin || roleSaving" maxlength="50" required autofocus></label><label class="system-form-label">角色编码 <em>*</em><input v-model="roleForm.code" :disabled="!roleCreating || roleSaving" maxlength="50" pattern="[a-z][a-z0-9_]{0,49}" title="小写字母开头，可含数字和下划线" required></label></div><label class="system-form-label">角色描述 <em>*</em><textarea v-model="roleForm.description" :disabled="roleForm.isSuperAdmin || roleSaving" rows="2" maxlength="300" required></textarea></label><label class="system-enabled"><input v-model="roleForm.enabled" type="checkbox" :disabled="roleForm.isSuperAdmin || roleSaving">启用角色</label><fieldset class="system-permissions"><legend>权限配置</legend><div class="system-permission-group"><strong>营收与接口</strong><div class="system-permission-items"><label v-for="option in data.permissions" :key="option.key"><input type="checkbox" :checked="roleForm.isSuperAdmin || roleForm.permissions.includes(option.key)" :disabled="roleForm.isSuperAdmin || roleSaving || option.key === 'Revenue.Read'" @change="togglePermission(option.key, ($event.target as HTMLInputElement).checked)">{{ option.name }}</label></div></div><div class="system-permission-group"><strong>系统管理 <small>统一认证指定</small></strong><div class="system-permission-items"><label v-for="name in ['用户管理', '角色管理', '操作日志']" :key="name"><input type="checkbox" :checked="roleForm.isSuperAdmin" disabled>{{ name }}</label></div></div></fieldset><p v-if="roleError" class="revenue-error" role="alert">{{ roleError }}</p><div class="system-dialog-footer"><button type="button" class="btn btn-ghost" :disabled="roleSaving" @click="roleOpen = false">{{ roleForm.isSuperAdmin ? '关闭' : '取消' }}</button><button v-if="!roleForm.isSuperAdmin" class="btn btn-primary" :disabled="roleSaving" type="submit">{{ roleSaving ? '保存中…' : roleCreating ? '新增' : '保存' }}</button></div></form>
    </Modal>
  </div>
</template>

<style scoped>
.revenue-system{padding:24px 26px;min-height:calc(100vh - 66px);background:var(--paper);color:var(--ink);font-family:"Microsoft YaHei",sans-serif;font-size:13px;line-height:1.5;--ink-3:var(--muted);--cinnabar:var(--brick)}
.page-head{display:flex;align-items:center;gap:16px;margin-bottom:20px}.page-title{display:flex;align-items:center;gap:10px;font-family:"STKaiti","KaiTi",serif;font-size:22px;font-weight:700}.bar{width:4px;height:20px;background:var(--brick);border-radius:2px}.page-desc{margin-top:6px;color:var(--muted)}.page-head-sp{flex:1}
.card{background:var(--paper-2);border:1px solid var(--line);border-radius:14px;box-shadow:var(--shadow);overflow:hidden}.btn{padding:8px 17px;border:1px solid var(--line);border-radius:8px;background:#fff;color:var(--ink-2);font-size:13px;white-space:nowrap;cursor:pointer}.btn:hover{border-color:var(--gold)}.btn-primary{background:var(--brick);border-color:var(--brick);color:#fff}.btn-sm{padding:5px 12px;font-size:12px}.btn:disabled,.pg:disabled{opacity:.45;cursor:default}
.f-search{display:flex;align-items:center;gap:8px;background:#fff;border:1px solid var(--line);border-radius:8px;padding:8px 12px;max-width:100%;color:var(--muted)}.f-search input{width:100%;min-width:0;border:0;outline:0;background:transparent;color:var(--ink)}.sel{border:1px solid var(--line);border-radius:7px;background:#fff;padding:7px 10px;color:var(--ink-2);font-size:12px}.t-wrap{overflow-x:auto}table{width:100%;border-collapse:collapse;font-size:13px}th,td{padding:12px 14px;text-align:left;border-bottom:1px solid var(--line)}th{background:#f0e8d8;font-size:12px;color:var(--muted);font-weight:600}.tag{display:inline-block;padding:2px 8px;border-radius:5px;font-size:11px;white-space:nowrap}.tag-pub{background:#3f7d6e20;color:var(--jade)}.tag-draft{background:#8a7a6320;color:var(--muted)}.tag-national,.tag-cat{background:#b8863b20;color:#8a6220}.pager{display:flex;align-items:center;justify-content:space-between;gap:12px}.pg{border:1px solid var(--line);border-radius:7px;background:#fff;padding:6px 10px;color:var(--ink-2);white-space:nowrap}.empty{padding:32px;text-align:center;color:var(--muted)}.empty h4{font-size:15px;margin-bottom:8px}.revenue-error{padding:12px 16px;margin-bottom:14px;border:1px solid #e8c0b2;border-radius:8px;background:#fae9e2;color:#8c2a20}.revenue-loading{padding:24px;color:var(--muted)}
button:focus-visible,select:focus-visible,input:focus-visible{outline:2px solid var(--brick);outline-offset:2px}
@media(max-width:650px){.revenue-system{padding:16px}.page-head{gap:8px}.page-title{font-size:20px}}
.system-shell { min-height:480px; }
.system-tabs { display:flex;gap:30px;padding:0 24px;border-bottom:1px solid var(--line); }
.system-tabs button { position:relative;padding:17px 0;font-size:15px;font-weight:600;color:var(--ink-3);background:transparent;border:0;cursor:pointer; }
.system-tabs button.active { color:var(--cinnabar); }
.system-tabs button.active::after { content:'';position:absolute;bottom:-1px;left:0;right:0;height:3px;background:var(--cinnabar);border-radius:3px 3px 0 0; }
.system-panel { padding:24px; }
.system-toolbar { display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:20px; }
.system-toolbar h2 { font-size:18px;font-weight:700;margin:0 0 5px; }
.system-toolbar p,.system-dialog-hint { color:var(--ink-3);font-size:13px;margin:0; }
.system-actions,.system-filters { display:flex;align-items:center;gap:8px; }
.system-filters { flex-wrap:wrap;margin-bottom:18px;gap:12px; }
.system-filters .f-search,.system-toolbar .f-search { width:250px; }
.system-count { margin-left:auto;color:var(--ink-3);font-size:12px; }
.system-user-roles { width:280px;min-width:230px; }
.system-users-table { min-width:980px; }
.system-nowrap { white-space:nowrap; }
.system-account-note { display:block;font-size:11px;color:var(--ink-3);margin-top:4px; }
.system-role-grid { display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px; }
.system-role-card { padding:20px;border:1px solid var(--line);border-radius:10px;background:var(--paper); }
.system-role-card h3 { font-size:16px;font-weight:700;margin:0 0 6px; }
.system-role-card code { font-size:12px;color:var(--ink-3);overflow-wrap:anywhere; }
.system-role-description { font-size:13px;color:var(--ink-2);line-height:1.7;min-height:44px;margin:12px 0;overflow-wrap:anywhere; }
.system-role-tags,.system-role-actions { display:flex;flex-wrap:wrap;gap:8px; }
.system-role-summary { margin:16px 0;font-size:12px;color:var(--ink-3); }
.system-delete { color:var(--cinnabar); }
.system-date { display:flex;align-items:center;gap:6px;color:var(--ink-3);font-size:12px; }
.system-date input { padding:6px 8px;border:1px solid var(--line);border-radius:6px;background:var(--paper);color:var(--ink);font:inherit;height:32px; }
.system-logs-table { min-width:900px; }
.system-record { max-width:210px;overflow-wrap:anywhere;font-size:12px; }
.system-log-detail { min-width:180px;max-width:400px;overflow-wrap:anywhere; }
.system-panel .pager { padding:16px 0 0; }
.system-panel .pager .info { font-size:12.5px;color:var(--ink-3); }
.system-panel .pages { display:flex;align-items:center;gap:8px;margin-left:auto; }
@media(max-width:1200px) { .system-role-grid { grid-template-columns:repeat(2,minmax(0,1fr)); } }
@media(max-width:650px) { .system-panel { padding:16px; }.system-tabs { padding:0 16px;gap:24px; }.system-toolbar { align-items:flex-start;flex-direction:column; }.system-role-grid { grid-template-columns:1fr; }.system-count { margin-left:0; }.system-panel .pager { align-items:flex-start;gap:12px;flex-wrap:wrap; } }
</style>
<style>
.revenue-system-dialog { background:transparent;--ink-3:var(--muted);--cinnabar:var(--brick); }
.revenue-system .ant-select-selector { background:var(--paper)!important;border-color:var(--line)!important;font-size:12px!important; }
.revenue-system-dialog .ant-modal-content { padding:24px;border:1px solid var(--line);border-radius:10px;background:var(--paper);color:var(--ink);max-height:90dvh;overflow:auto; }
.revenue-system-dialog .ant-modal-header { background:transparent;margin-bottom:20px; }
.revenue-system-dialog .ant-modal-title { color:var(--ink);font-size:18px;font-weight:700; }
.revenue-system-dialog .system-dialog-form { display:grid;gap:16px; }
.revenue-system-dialog .system-dialog-hint { color:var(--ink-3);font-size:13px;margin:0; }
.revenue-system-dialog .system-identity-grid { display:grid;grid-template-columns:1fr 1fr;gap:16px; }
.revenue-system-dialog .system-form-label { display:flex;flex-wrap:wrap;gap:8px 4px;font-size:13px;color:var(--ink-2); }
.revenue-system-dialog .system-form-label em { color:var(--cinnabar);font-style:normal; }
.revenue-system-dialog .system-form-label > input,.revenue-system-dialog .system-form-label > textarea,.revenue-system-dialog .system-form-label > .ant-select { flex-basis:100%;width:100%;min-width:0; }
.revenue-system-dialog .system-form-label > input,.revenue-system-dialog .system-form-label > textarea { border:1px solid var(--line);border-radius:6px;padding:8px 10px;background:var(--paper-2);color:var(--ink);font:inherit; }
.revenue-system-dialog input:focus-visible,.revenue-system-dialog textarea:focus-visible { outline:2px solid var(--cinnabar);outline-offset:1px; }
.revenue-system-dialog input:disabled,.revenue-system-dialog textarea:disabled { opacity:.6;cursor:not-allowed; }
.revenue-system-dialog .system-enabled { display:flex;align-items:center;gap:8px;font-size:13px; }
.revenue-system-dialog input[type=checkbox] { width:15px;height:15px;accent-color:var(--cinnabar); }
.revenue-system-dialog .system-permissions { border:1px solid var(--line);border-radius:8px;padding:16px;margin:0; }
.revenue-system-dialog .system-permissions legend { padding:0 6px;font-size:13px;font-weight:600; }
.revenue-system-dialog .system-permission-group + .system-permission-group { margin-top:20px;padding-top:16px;border-top:1px solid var(--line); }
.revenue-system-dialog .system-permission-group strong { font-size:13px; }
.revenue-system-dialog .system-permission-group small { margin-left:8px;font-weight:400;color:var(--ink-3);font-size:11px; }
.revenue-system-dialog .system-permission-items { display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-top:12px; }
.revenue-system-dialog .system-permission-items label { display:flex;align-items:center;gap:8px;font-size:12px; }
.revenue-system-dialog .system-dialog-footer { display:flex;justify-content:flex-end;gap:10px;margin-top:4px; }
@media(max-width:500px) { .revenue-system-dialog .system-identity-grid,.revenue-system-dialog .system-permission-items { grid-template-columns:1fr; }.revenue-system-dialog .ant-modal-content { padding:20px; } }
</style>
