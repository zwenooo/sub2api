<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.openAIAuthImportTitle')"
    width="normal"
    close-on-click-outside
    @close="handleClose"
  >
    <form id="import-openai-auth-form" class="space-y-4" @submit.prevent="handleImport">
      <div class="space-y-2 text-sm text-gray-600 dark:text-dark-300">
        <div>{{ t('admin.accounts.openAIAuthImportHint') }}</div>
        <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 font-mono text-xs dark:border-dark-700 dark:bg-dark-800">
          <div>{{ t('admin.accounts.openAIAuthImportFormatHint') }}</div>
          <pre class="mt-2 whitespace-pre-wrap break-all">{{ openAIAuthImportExample }}</pre>
        </div>
      </div>

      <div
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300"
      >
        {{ t('admin.accounts.openAIAuthImportWarning') }}
      </div>

      <div class="flex gap-2">
        <button
          type="button"
          class="btn"
          :class="mode === 'file' ? 'btn-primary' : 'btn-secondary'"
          @click="mode = 'file'"
        >
          {{ t('admin.accounts.openAIAuthImportModeFile') }}
        </button>
        <button
          type="button"
          class="btn"
          :class="mode === 'json' ? 'btn-primary' : 'btn-secondary'"
          @click="mode = 'json'"
        >
          {{ t('admin.accounts.openAIAuthImportModeJson') }}
        </button>
      </div>

      <div v-if="mode === 'file'">
        <label class="input-label">{{ t('admin.accounts.openAIAuthImportFile') }}</label>
        <div
          class="flex items-center justify-between gap-3 rounded-lg border border-dashed border-gray-300 bg-gray-50 px-4 py-3 dark:border-dark-600 dark:bg-dark-800"
        >
          <div class="min-w-0">
            <div class="truncate text-sm text-gray-700 dark:text-dark-200">
              {{ fileName || t('admin.accounts.openAIAuthImportSelectFile') }}
            </div>
            <div class="text-xs text-gray-500 dark:text-dark-400">JSON (.json)</div>
          </div>
          <button type="button" class="btn btn-secondary shrink-0" @click="openFilePicker">
            {{ t('common.chooseFile') }}
          </button>
        </div>
        <input
          ref="fileInput"
          type="file"
          class="hidden"
          accept="application/json,.json"
          @change="handleFileChange"
        />
      </div>

      <div v-else class="space-y-2">
        <label class="input-label">{{ t('admin.accounts.openAIAuthImportJson') }}</label>
        <textarea
          v-model="jsonText"
          rows="10"
          class="input"
          :placeholder="t('admin.accounts.openAIAuthImportJsonPlaceholder')"
        />
      </div>

      <div class="space-y-2">
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.accounts.openAIAuthImportGroupTitle') }}
        </div>
        <div class="text-xs text-gray-500 dark:text-dark-400">
          {{ t('admin.accounts.openAIAuthImportGroupHint') }}
        </div>
        <div
          v-if="groupsLoading"
          class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-300"
        >
          {{ t('common.loading') }}
        </div>
        <GroupSelector
          v-else
          v-model="selectedGroupIds"
          :groups="openAIGroups"
          platform="openai"
        />
      </div>

      <div class="space-y-2">
        <label class="input-label">{{ t('admin.accounts.openAIAuthImportNameTemplate') }}</label>
        <select
          v-model="selectedTemplatePreset"
          class="input"
          @change="applyNameTemplatePreset"
        >
          <option
            v-for="option in nameTemplatePresetOptions"
            :key="option.value"
            :value="option.value"
          >
            {{ option.label }}
          </option>
        </select>
        <input
          v-model="nameTemplate"
          type="text"
          class="input"
          :placeholder="t('admin.accounts.openAIAuthImportNameTemplatePlaceholder')"
        />
        <div class="text-xs text-gray-500 dark:text-dark-400">
          {{ t('admin.accounts.openAIAuthImportNameTemplateHint') }}
        </div>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="token in supportedTemplateTokens"
            :key="token"
            type="button"
            class="rounded-full border border-gray-200 px-2 py-1 text-xs text-gray-600 transition hover:border-primary-300 hover:text-primary-600 dark:border-dark-600 dark:text-dark-300 dark:hover:border-primary-500 dark:hover:text-primary-400"
            @click="appendTemplateToken(token)"
          >
            {{ token }}
          </button>
        </div>
      </div>

      <div
        v-if="result"
        class="space-y-2 rounded-xl border border-gray-200 p-4 dark:border-dark-700"
      >
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.accounts.openAIAuthImportResult') }}
        </div>
        <div class="text-sm text-gray-700 dark:text-dark-300">
          {{ t('admin.accounts.openAIAuthImportResultSummary', result) }}
        </div>

        <div v-if="errorItems.length" class="mt-2">
          <div class="text-sm font-medium text-red-600 dark:text-red-400">
            {{ t('admin.accounts.openAIAuthImportErrors') }}
          </div>
          <div
            class="mt-2 max-h-48 overflow-auto rounded-lg bg-gray-50 p-3 font-mono text-xs dark:bg-dark-800"
          >
            <div v-for="(item, idx) in errorItems" :key="idx" class="whitespace-pre-wrap">
              #{{ item.index }} {{ item.name || '-' }} - {{ item.message }}
            </div>
          </div>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" type="button" :disabled="importing" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          class="btn btn-primary"
          type="submit"
          form="import-openai-auth-form"
          :disabled="importing"
        >
          {{ importing ? t('admin.accounts.openAIAuthImporting') : t('admin.accounts.openAIAuthImportButton') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { AdminGroup, AdminOpenAIAuthImportResult, AdminOpenAIAuthImportSource } from '@/types'

interface Props {
  show: boolean
}

interface Emits {
  (e: 'close'): void
  (e: 'imported'): void
}

type ImportMode = 'file' | 'json'

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()
const appStore = useAppStore()

const openAIAuthImportExample = JSON.stringify(
  [
    {
      tokens: {
        access_token: 'example_access_token',
        refresh_token: 'example_refresh_token',
        id_token: 'example_id_token',
        account_id: 'example_account_id'
      }
    }
  ],
  null,
  2
)

const mode = ref<ImportMode>('file')
const importing = ref(false)
const groupsLoading = ref(false)
const file = ref<File | null>(null)
const jsonText = ref('')
const result = ref<AdminOpenAIAuthImportResult | null>(null)
const openAIGroups = ref<AdminGroup[]>([])
const selectedGroupIds = ref<number[]>([])
const selectedTemplatePreset = ref('')
const nameTemplate = ref('')

const fileInput = ref<HTMLInputElement | null>(null)
const fileName = computed(() => file.value?.name || '')
const errorItems = computed(() => result.value?.errors || [])
const supportedTemplateTokens = [
  '{index}',
  '{email}',
  '{account_id}',
  '{chatgpt_account_id}',
  '{chatgpt_user_id}',
  '{organization_id}',
  '{plan_type}',
  '{client_id}'
]
const nameTemplatePresetOptions = computed(() => [
  {
    value: '',
    label: t('admin.accounts.openAIAuthImportNameTemplatePresetDefault')
  },
  {
    value: '{email}',
    label: t('admin.accounts.openAIAuthImportNameTemplatePresetEmail')
  },
  {
    value: '{plan_type}-{email}',
    label: t('admin.accounts.openAIAuthImportNameTemplatePresetPlanEmail')
  },
  {
    value: '{index}-{email}',
    label: t('admin.accounts.openAIAuthImportNameTemplatePresetIndexEmail')
  },
  {
    value: '{plan_type}-{account_id}',
    label: t('admin.accounts.openAIAuthImportNameTemplatePresetPlanAccount')
  },
  {
    value: '{organization_id}-{email}',
    label: t('admin.accounts.openAIAuthImportNameTemplatePresetOrgEmail')
  }
])

watch(
  () => props.show,
  async (open) => {
    if (!open) {
      return
    }
    mode.value = 'file'
    selectedGroupIds.value = []
    selectedTemplatePreset.value = ''
    nameTemplate.value = ''
    file.value = null
    jsonText.value = ''
    result.value = null
    if (fileInput.value) {
      fileInput.value.value = ''
    }
    await loadOpenAIGroups()
  }
)

const openFilePicker = () => {
  fileInput.value?.click()
}

const handleFileChange = (event: Event) => {
  const target = event.target as HTMLInputElement
  file.value = target.files?.[0] || null
}

const handleClose = () => {
  if (importing.value) return
  emit('close')
}

const loadOpenAIGroups = async () => {
  groupsLoading.value = true
  try {
    openAIGroups.value = await adminAPI.groups.getAll('openai')
  } catch (error: any) {
    openAIGroups.value = []
    appStore.showError(error?.message || t('admin.groups.failedToLoad'))
  } finally {
    groupsLoading.value = false
  }
}

const applyNameTemplatePreset = () => {
  nameTemplate.value = selectedTemplatePreset.value
}

const appendTemplateToken = (token: string) => {
  nameTemplate.value = `${nameTemplate.value}${token}`.trim()
}

const handleImport = async () => {
  importing.value = true
  result.value = null

  try {
    let response: AdminOpenAIAuthImportResult

    if (mode.value === 'file') {
      if (!file.value) {
        appStore.showError(t('admin.accounts.openAIAuthImportSelectFile'))
        return
      }
      response = await adminAPI.accounts.importOpenAIAuthFile(file.value, {
        group_ids: selectedGroupIds.value,
        name_template: nameTemplate.value
      })
    } else {
      if (!jsonText.value.trim()) {
        appStore.showError(t('admin.accounts.openAIAuthImportSelectJson'))
        return
      }

      let payload: unknown
      try {
        payload = JSON.parse(jsonText.value)
      } catch {
        appStore.showError(t('admin.accounts.openAIAuthImportParseFailed'))
        return
      }

      if (!Array.isArray(payload)) {
        appStore.showError(t('admin.accounts.openAIAuthImportMustBeArray'))
        return
      }

      response = await adminAPI.accounts.importOpenAIAuthItems(
        payload as AdminOpenAIAuthImportSource[],
        {
          group_ids: selectedGroupIds.value,
          name_template: nameTemplate.value
        }
      )
    }

    result.value = response

    const msgParams: Record<string, unknown> = {
      account_created: response.account_created,
      account_failed: response.account_failed
    }

    if (response.account_failed > 0) {
      if (response.account_created > 0) {
        emit('imported')
      }
      appStore.showError(t('admin.accounts.openAIAuthImportCompletedWithErrors', msgParams))
      return
    }

    appStore.showSuccess(t('admin.accounts.openAIAuthImportSuccess', msgParams))
    emit('imported')
    emit('close')
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.openAIAuthImportFailed'))
  } finally {
    importing.value = false
  }
}
</script>
