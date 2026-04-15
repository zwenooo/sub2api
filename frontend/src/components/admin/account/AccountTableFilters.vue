<template>
  <div class="flex flex-col gap-3">
    <div class="flex flex-wrap items-center gap-3">
      <SearchInput
        :model-value="searchQuery"
        :placeholder="t('admin.accounts.searchAccounts')"
        class="w-full sm:w-64"
        @update:model-value="$emit('update:searchQuery', $event)"
        @search="$emit('change')"
      />
      <Select :model-value="filters.platform" class="w-40" :options="pOpts" @update:model-value="updatePlatform" @change="$emit('change')" />
      <Select :model-value="filters.type" class="w-40" :options="tOpts" @update:model-value="updateType" @change="$emit('change')" />
      <Select :model-value="filters.privacy_mode" class="w-40" :options="privacyOpts" @update:model-value="updatePrivacyMode" @change="$emit('change')" />
      <Select :model-value="filters.group" class="w-40" :options="gOpts" @update:model-value="updateGroup" @change="$emit('change')" />
    </div>
    <div class="flex flex-wrap items-center gap-2">
      <button
        v-for="item in statusFilterItems"
        :key="item.value || 'all'"
        type="button"
        class="inline-flex min-w-[104px] items-center justify-between gap-3 rounded-full border px-3 py-2 text-sm transition-colors"
        :class="item.active
          ? 'border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-400 dark:bg-primary-500/10 dark:text-primary-200'
          : 'border-gray-200 bg-white text-gray-700 hover:border-primary-200 hover:text-primary-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:border-primary-500/40 dark:hover:text-primary-300'"
        :aria-pressed="item.active"
        :disabled="statusSummaryLoading"
        @click="updateStatus(item.value)"
      >
        <span class="text-xs font-medium">{{ item.label }}</span>
        <span class="font-semibold tabular-nums">{{ item.count }}</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import type { AdminAccountStatusSummary, AdminGroup } from '@/types'

const props = defineProps<{
  searchQuery: string
  filters: Record<string, any>
  groups?: AdminGroup[]
  statusSummary?: AdminAccountStatusSummary
  statusSummaryLoading?: boolean
}>()

const emit = defineEmits(['update:searchQuery', 'update:filters', 'change'])

const { t } = useI18n()

const updatePlatform = (value: string | number | boolean | null) => {
  emit('update:filters', { ...props.filters, platform: value })
}

const updateType = (value: string | number | boolean | null) => {
  emit('update:filters', { ...props.filters, type: value })
}

const updateStatus = (value: string | number | boolean | null) => {
  const nextStatus = String(value || '')
  if (String(props.filters.status || '') === nextStatus) return
  emit('update:filters', { ...props.filters, status: nextStatus })
  emit('change')
}

const updatePrivacyMode = (value: string | number | boolean | null) => {
  emit('update:filters', { ...props.filters, privacy_mode: value })
}

const updateGroup = (value: string | number | boolean | null) => {
  emit('update:filters', { ...props.filters, group: value })
}

const pOpts = computed(() => [
  { value: '', label: t('admin.accounts.allPlatforms') },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'antigravity', label: 'Antigravity' }
])

const tOpts = computed(() => [
  { value: '', label: t('admin.accounts.allTypes') },
  { value: 'oauth', label: t('admin.accounts.oauthType') },
  { value: 'setup-token', label: t('admin.accounts.setupToken') },
  { value: 'apikey', label: t('admin.accounts.apiKey') },
  { value: 'bedrock', label: 'AWS Bedrock' }
])
const privacyOpts = computed(() => [
  { value: '', label: t('admin.accounts.allPrivacyModes') },
  { value: '__unset__', label: t('admin.accounts.privacyUnset') },
  { value: 'training_off', label: 'Privacy' },
  { value: 'training_set_cf_blocked', label: 'CF' },
  { value: 'training_set_failed', label: 'Fail' }
])

const gOpts = computed(() => [
  { value: '', label: t('admin.accounts.allGroups') },
  { value: 'ungrouped', label: t('admin.accounts.ungroupedGroup') },
  ...(props.groups || []).map(g => ({ value: String(g.id), label: g.name }))
])

const statusFilterItems = computed(() => {
  const summary = props.statusSummary ?? {
    total: 0,
    active: 0,
    rate_limited: 0,
    error: 0,
    inactive: 0,
    temp_unschedulable: 0
  }
  const currentStatus = String(props.filters.status || '')
  return [
    { value: '', label: t('common.all'), count: summary.total, active: currentStatus === '' },
    { value: 'active', label: t('admin.accounts.status.active'), count: summary.active, active: currentStatus === 'active' },
    { value: 'rate_limited', label: t('admin.accounts.status.rateLimited'), count: summary.rate_limited, active: currentStatus === 'rate_limited' },
    { value: 'error', label: t('admin.accounts.status.error'), count: summary.error, active: currentStatus === 'error' },
    { value: 'inactive', label: t('admin.accounts.status.inactive'), count: summary.inactive, active: currentStatus === 'inactive' },
    {
      value: 'temp_unschedulable',
      label: t('admin.accounts.status.tempUnschedulable'),
      count: summary.temp_unschedulable,
      active: currentStatus === 'temp_unschedulable'
    }
  ]
})
</script>
