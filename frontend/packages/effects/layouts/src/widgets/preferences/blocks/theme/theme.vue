<script setup lang="ts">
import type { Component } from 'vue';

import type { ThemeModeType } from '@vben/types';

import { watch } from 'vue';

import { MoonStar, Sun, SunMoon } from '@vben/icons';
import { $t } from '@vben/locales';
import { usePreferences } from '@vben/preferences';

import SwitchItem from '../switch-item.vue';

defineOptions({
  name: 'PreferenceTheme',
});

const modelValue = defineModel<string>({ default: 'auto' });
const themeSemiDarkSidebar = defineModel<boolean>('themeSemiDarkSidebar');
const themeSemiDarkSidebarColor = defineModel<string>(
  'themeSemiDarkSidebarColor',
  { default: '#1c1e23' },
);
const themeSemiDarkSidebarSub = defineModel<boolean>('themeSemiDarkSidebarSub');
const themeSemiDarkHeader = defineModel<boolean>('themeSemiDarkHeader');
const themeSemiDarkHeaderColor = defineModel<string>(
  'themeSemiDarkHeaderColor',
  { default: '#1c1e23' },
);

const { layout } = usePreferences();

watch(
  () => themeSemiDarkSidebar.value,
  () => {
    if (!themeSemiDarkSidebar.value) {
      themeSemiDarkSidebarSub.value = themeSemiDarkSidebar.value;
    }
  },
);

const THEME_PRESET: Array<{ icon: Component; name: ThemeModeType }> = [
  {
    icon: Sun,
    name: 'light',
  },
  {
    icon: MoonStar,
    name: 'dark',
  },
  {
    icon: SunMoon,
    name: 'auto',
  },
];

const DARK_COLOR_PRESETS = [
  '#5955a0',
  '#181f23',
  '#222d32',
  '#1a2332',
  '#304156',
  '#001529',
  '#343a40',
  '#26355e',
];

function resetDarkColor(target: 'header' | 'sidebar') {
  if (target === 'sidebar') {
    themeSemiDarkSidebarColor.value = '#1c1e23';
  } else {
    themeSemiDarkHeaderColor.value = '#1c1e23';
  }
}

function activeClass(theme: string): string[] {
  return theme === modelValue.value ? ['outline-box-active'] : [];
}

function nameView(name: string) {
  switch (name) {
    case 'auto': {
      return $t('preferences.followSystem');
    }
    case 'dark': {
      return $t('preferences.theme.dark');
    }
    case 'light': {
      return $t('preferences.theme.light');
    }
  }
}
</script>

<template>
  <div class="flex w-full flex-wrap justify-between">
    <template v-for="theme in THEME_PRESET" :key="theme.name">
      <div
        class="flex cursor-pointer flex-col"
        @click="modelValue = theme.name"
      >
        <div
          :class="activeClass(theme.name)"
          class="outline-box flex-center py-4"
        >
          <component :is="theme.icon" class="mx-9 size-5" />
        </div>
        <div class="mt-2 text-center text-xs text-muted-foreground">
          {{ nameView(theme.name) }}
        </div>
      </div>
    </template>

    <SwitchItem
      v-model="themeSemiDarkSidebar"
      :disabled="
        modelValue === 'dark' ||
        layout === 'header-nav' ||
        layout === 'full-content'
      "
      :tip="$t('preferences.theme.darkSidebarTip')"
      class="mt-6"
    >
      {{ $t('preferences.theme.darkSidebar') }}
    </SwitchItem>
    <div v-if="themeSemiDarkSidebar" class="mb-3 w-full rounded-md border p-3">
      <div class="mb-2 flex items-center justify-between text-xs">
        <span>{{ $t('preferences.theme.darkSidebarColor') }}</span>
        <button
          class="text-muted-foreground hover:text-foreground"
          type="button"
          @click="resetDarkColor('sidebar')"
        >
          {{ $t('preferences.theme.resetColor') }}
        </button>
      </div>
      <div class="grid grid-cols-4 gap-2">
        <button
          v-for="color in DARK_COLOR_PRESETS"
          :key="color"
          :aria-label="color"
          :class="{
            'ring-2 ring-primary ring-offset-2 ring-offset-background':
              themeSemiDarkSidebarColor.toLowerCase() === color,
          }"
          :style="{ backgroundColor: color }"
          class="size-6 rounded-md border border-white/20"
          type="button"
          @click="themeSemiDarkSidebarColor = color"
        ></button>
      </div>
      <label class="mt-3 flex cursor-pointer items-center gap-2 text-xs">
        <span
          :style="{ backgroundColor: themeSemiDarkSidebarColor }"
          class="size-6 rounded-md border"
        ></span>
        <span>{{ $t('preferences.theme.customColor') }}</span>
        <input
          v-model="themeSemiDarkSidebarColor"
          class="h-6 w-12 cursor-pointer"
          type="color"
        />
      </label>
    </div>
    <SwitchItem
      v-model="themeSemiDarkSidebarSub"
      :disabled="
        modelValue === 'dark' ||
        (layout !== 'header-mixed-nav' && layout !== 'sidebar-mixed-nav') ||
        !themeSemiDarkSidebar
      "
      :tip="$t('preferences.theme.darkSidebarSubTip')"
    >
      {{ $t('preferences.theme.darkSidebarSub') }}
    </SwitchItem>
    <SwitchItem v-model="themeSemiDarkHeader" :disabled="modelValue === 'dark'">
      {{ $t('preferences.theme.darkHeader') }}
    </SwitchItem>
    <div v-if="themeSemiDarkHeader" class="w-full rounded-md border p-3">
      <div class="mb-2 flex items-center justify-between text-xs">
        <span>{{ $t('preferences.theme.darkHeaderColor') }}</span>
        <button
          class="text-muted-foreground hover:text-foreground"
          type="button"
          @click="resetDarkColor('header')"
        >
          {{ $t('preferences.theme.resetColor') }}
        </button>
      </div>
      <div class="grid grid-cols-4 gap-2">
        <button
          v-for="color in DARK_COLOR_PRESETS"
          :key="color"
          :aria-label="color"
          :class="{
            'ring-2 ring-primary ring-offset-2 ring-offset-background':
              themeSemiDarkHeaderColor.toLowerCase() === color,
          }"
          :style="{ backgroundColor: color }"
          class="size-6 rounded-md border border-white/20"
          type="button"
          @click="themeSemiDarkHeaderColor = color"
        ></button>
      </div>
      <label class="mt-3 flex cursor-pointer items-center gap-2 text-xs">
        <span
          :style="{ backgroundColor: themeSemiDarkHeaderColor }"
          class="size-6 rounded-md border"
        ></span>
        <span>{{ $t('preferences.theme.customColor') }}</span>
        <input
          v-model="themeSemiDarkHeaderColor"
          class="h-6 w-12 cursor-pointer"
          type="color"
        />
      </label>
    </div>
  </div>
</template>
