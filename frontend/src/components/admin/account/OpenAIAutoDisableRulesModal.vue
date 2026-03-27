<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.openAIAutoDisableRulesTitle')"
    width="wide"
    close-on-click-outside
    @close="handleClose"
  >
    <div class="space-y-4">
      <label class="flex items-start gap-3 rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800">
        <input
          v-model="enabled"
          type="checkbox"
          class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
        />
        <div class="space-y-1">
          <div class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t('admin.accounts.openAIAutoDisableRulesEnabled') }}
          </div>
          <div class="text-xs text-gray-500 dark:text-dark-400">
            {{
              enabled
                ? t('admin.accounts.openAIAutoDisableRulesHint')
                : t('admin.accounts.openAIAutoDisableRulesDisabledHint')
            }}
          </div>
        </div>
      </label>

      <div class="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-200">
        {{ t('admin.accounts.openAIAutoDisableRulesHint') }}
      </div>

      <div class="flex items-center justify-between gap-3">
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.accounts.openAIAutoDisableRulesRules') }}
        </div>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="loading || saving" @click="addRule">
          {{ t('admin.accounts.openAIAutoDisableRulesAdd') }}
        </button>
      </div>

      <div v-if="loading" class="rounded-xl border border-gray-200 p-6 text-sm text-gray-500 dark:border-dark-700 dark:text-dark-300">
        {{ t('common.loading') }}
      </div>

      <div
        v-else-if="rules.length === 0"
        class="rounded-xl border border-dashed border-gray-300 p-6 text-sm text-gray-500 dark:border-dark-600 dark:text-dark-300"
      >
        {{ t('admin.accounts.openAIAutoDisableRulesEmpty') }}
      </div>

      <div v-else class="space-y-4">
        <div
          v-for="(rule, index) in rules"
          :key="index"
          class="rounded-xl border border-gray-200 p-4 dark:border-dark-700"
        >
          <div class="flex items-center justify-between gap-3">
            <div class="text-sm font-medium text-gray-900 dark:text-white">
              {{ t('admin.accounts.tempUnschedulable.ruleIndex', { index: index + 1 }) }}
            </div>
            <button
              type="button"
              class="btn btn-danger btn-sm"
              :disabled="loading || saving"
              @click="removeRule(index)"
            >
              {{ t('admin.accounts.openAIAutoDisableRulesRemove') }}
            </button>
          </div>

          <div class="mt-4 grid gap-4 md:grid-cols-2">
            <div class="space-y-2">
              <label class="input-label">{{ t('admin.accounts.openAIAutoDisableRulesStatusCode') }}</label>
              <input
                v-model="rule.statusCode"
                type="number"
                min="100"
                max="599"
                class="input"
                :placeholder="t('admin.accounts.openAIAutoDisableRulesStatusCodePlaceholder')"
              />
            </div>

            <div class="space-y-2">
              <label class="input-label">{{ t('admin.accounts.openAIAutoDisableRulesKeywords') }}</label>
              <textarea
                v-model="rule.keywordsText"
                rows="3"
                class="input"
                :placeholder="t('admin.accounts.openAIAutoDisableRulesKeywordsPlaceholder')"
              />
              <div class="text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.accounts.openAIAutoDisableRulesKeywordsHint') }}
              </div>
            </div>
          </div>

          <div class="mt-4 space-y-2">
            <label class="input-label">{{ t('admin.accounts.openAIAutoDisableRulesDescription') }}</label>
            <input
              v-model="rule.description"
              type="text"
              class="input"
              :placeholder="t('admin.accounts.openAIAutoDisableRulesDescriptionPlaceholder')"
            />
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex items-center justify-between gap-3">
        <button type="button" class="btn btn-secondary" :disabled="loading || saving" @click="addRule">
          {{ t('admin.accounts.openAIAutoDisableRulesAdd') }}
        </button>
        <div class="flex items-center gap-3">
          <button type="button" class="btn btn-secondary" :disabled="saving" @click="handleClose">
            {{ t('common.cancel') }}
          </button>
          <button type="button" class="btn btn-primary" :disabled="loading || saving" @click="handleSave">
            {{
              saving
                ? t('admin.accounts.openAIAutoDisableRulesSaving')
                : t('admin.accounts.openAIAutoDisableRulesSave')
            }}
          </button>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { OpenAIAutoDisableRule, OpenAIAutoDisableSettings } from '@/types'

interface Props {
  show: boolean
}

interface Emits {
  (e: 'close'): void
  (e: 'saved', settings: OpenAIAutoDisableSettings): void
}

interface EditableRule {
  statusCode: string
  keywordsText: string
  description: string
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()
const appStore = useAppStore()

const enabled = ref(false)
const rules = ref<EditableRule[]>([])
const loading = ref(false)
const saving = ref(false)

const createEditableRule = (): EditableRule => ({
  statusCode: '',
  keywordsText: '',
  description: ''
})

const applySettings = (settings: OpenAIAutoDisableSettings) => {
  enabled.value = settings.enabled === true
  rules.value = Array.isArray(settings.rules)
    ? settings.rules.map((rule) => ({
        statusCode:
          typeof rule.status_code === 'number' && Number.isFinite(rule.status_code)
            ? String(rule.status_code)
            : '',
        keywordsText: Array.isArray(rule.message_keywords) ? rule.message_keywords.join('\n') : '',
        description: String(rule.description || '')
      }))
    : []
}

const loadSettings = async () => {
  loading.value = true
  try {
    const settings = await adminAPI.accounts.getOpenAIAutoDisableRules()
    applySettings(settings)
  } catch (error: any) {
    console.error('Failed to load OpenAI auto disable rules:', error)
    appStore.showError(error?.message || t('admin.accounts.openAIAutoDisableRulesLoadFailed'))
    emit('close')
  } finally {
    loading.value = false
  }
}

watch(
  () => props.show,
  (open) => {
    if (!open) {
      return
    }
    void loadSettings()
  }
)

const addRule = () => {
  rules.value.push(createEditableRule())
}

const removeRule = (index: number) => {
  rules.value.splice(index, 1)
}

const serializeRule = (rule: EditableRule): OpenAIAutoDisableRule | null => {
  const statusText = rule.statusCode.trim()
  const statusValue = statusText ? Number.parseInt(statusText, 10) : NaN
  const statusCode =
    Number.isInteger(statusValue) && statusValue >= 100 && statusValue <= 599
      ? statusValue
      : null

  const messageKeywords = rule.keywordsText
    .split(/[\n,，]+/)
    .map((item) => item.trim())
    .filter((item) => item.length > 0)

  if (statusCode == null && messageKeywords.length === 0) {
    return null
  }

  return {
    status_code: statusCode,
    message_keywords: messageKeywords,
    description: rule.description.trim()
  }
}

const handleClose = () => {
  if (saving.value) return
  emit('close')
}

const handleSave = async () => {
  if (loading.value || saving.value) return

  const nextRules = rules.value
    .map((rule) => serializeRule(rule))
    .filter((rule): rule is OpenAIAutoDisableRule => rule !== null)

  if (enabled.value && nextRules.length === 0) {
    appStore.showError(t('admin.accounts.openAIAutoDisableRulesInvalid'))
    return
  }

  saving.value = true
  try {
    const settings = await adminAPI.accounts.updateOpenAIAutoDisableRules({
      enabled: enabled.value,
      rules: nextRules
    })
    applySettings(settings)
    appStore.showSuccess(t('admin.accounts.openAIAutoDisableRulesSaved'))
    emit('saved', settings)
    emit('close')
  } catch (error: any) {
    console.error('Failed to save OpenAI auto disable rules:', error)
    appStore.showError(error?.message || t('admin.accounts.openAIAutoDisableRulesSaveFailed'))
  } finally {
    saving.value = false
  }
}
</script>
