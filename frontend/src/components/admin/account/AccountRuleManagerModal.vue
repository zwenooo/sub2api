<template>
  <BaseDialog
    :show="show"
    title="账号规则管理"
    width="extra-wide"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <div class="rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="space-y-1">
            <div class="text-sm font-semibold text-gray-900 dark:text-white">统一管理平台 / 平台+类型的模型集合与错误规则</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">
              作用域优先级为“平台+类型”高于“平台”。同一作用域下同时管理模型集合、错误匹配条件和命中后的动作。
            </div>
            <div
              v-if="draftHint"
              class="inline-flex items-center rounded-full bg-primary-50 px-2.5 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/20 dark:text-primary-300"
            >
              {{ draftHint }}
            </div>
          </div>
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="loading"
            @click="loadCatalog()"
          >
            <Icon name="refresh" size="sm" class="mr-1.5" :class="loading ? 'animate-spin' : ''" />
            刷新
          </button>
        </div>
      </div>

      <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
        <div class="flex flex-wrap items-end gap-3">
          <div class="min-w-[180px] flex-1">
            <label class="input-label">统一转发次数上限</label>
            <input
              v-model.number="settingsForm.forward_max_attempts"
              type="number"
              min="1"
              class="input"
            />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              命中“转发请求”动作时，单个请求最多还能切换多少次账号。
            </p>
          </div>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="savingSettings"
            @click="saveSettings"
          >
            {{ savingSettings ? '保存中...' : '保存设置' }}
          </button>
        </div>
      </div>

      <div v-if="loading" class="flex items-center justify-center py-12">
        <Icon name="refresh" size="lg" class="animate-spin text-gray-400" />
      </div>

      <div v-else class="grid gap-4 lg:grid-cols-[320px,minmax(0,1fr)]">
        <div class="space-y-4">
          <section class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <div class="mb-3 flex items-center justify-between gap-2">
              <div>
                <div class="text-sm font-semibold text-gray-900 dark:text-white">已配置作用域</div>
                <div class="text-xs text-gray-500 dark:text-gray-400">共 {{ catalog?.scopes.length ?? 0 }} 个</div>
              </div>
              <button type="button" class="btn btn-primary btn-sm" @click="openCreateScope()">
                <Icon name="plus" size="sm" class="mr-1" />
                新建
              </button>
            </div>

            <div v-if="!catalog?.scopes.length" class="rounded-lg border border-dashed border-gray-200 px-3 py-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
              还没有任何规则作用域。
            </div>

            <div v-else class="space-y-2">
              <button
                v-for="scope in catalog.scopes"
                :key="scope.id"
                type="button"
                :class="[
                  'w-full rounded-xl border px-3 py-3 text-left transition-colors',
                  scope.id === selectedScopeId
                    ? 'border-primary-300 bg-primary-50 dark:border-primary-700 dark:bg-primary-900/20'
                    : 'border-gray-200 hover:bg-gray-50 dark:border-dark-600 dark:hover:bg-dark-700'
                ]"
                @click="selectedScopeId = scope.id"
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <div class="flex flex-wrap items-center gap-2">
                      <PlatformTypeBadge :platform="scope.platform" :type="scope.account_type" />
                      <span
                        :class="[
                          'inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium',
                          scope.enabled
                            ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
                            : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
                        ]"
                      >
                        {{ scope.enabled ? '启用' : '停用' }}
                      </span>
                    </div>
                    <div class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                      模型 {{ scope.model_set.length }} 个 · 规则 {{ scope.rules.length }} 条
                    </div>
                    <div v-if="scope.description" class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">
                      {{ scope.description }}
                    </div>
                  </div>
                  <Icon name="chevronRight" size="sm" class="mt-0.5 text-gray-400" />
                </div>
              </button>
            </div>
          </section>

          <section class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <div class="mb-3">
              <div class="text-sm font-semibold text-gray-900 dark:text-white">从现有账号快速建作用域</div>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                账号管理里出现过的平台 / 类型组合，都可以直接一键生成作用域。
              </div>
            </div>

            <div v-if="!unconfiguredObservedScopes.length" class="rounded-lg border border-dashed border-gray-200 px-3 py-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
              当前已覆盖所有已观测到的平台 / 类型组合。
            </div>

            <div v-else class="space-y-2">
              <button
                v-for="scope in unconfiguredObservedScopes"
                :key="observedScopeKey(scope)"
                type="button"
                class="flex w-full items-center justify-between rounded-xl border border-dashed border-gray-200 px-3 py-2.5 text-left hover:bg-gray-50 dark:border-dark-600 dark:hover:bg-dark-700"
                @click="openCreateScope(scope)"
              >
                <div class="flex min-w-0 items-center gap-2">
                  <PlatformTypeBadge :platform="scope.platform" :type="scope.account_type" />
                  <span class="text-xs text-gray-500 dark:text-gray-400">{{ scope.account_count }} 个账号</span>
                </div>
                <span class="text-xs font-medium text-primary-600 dark:text-primary-300">创建</span>
              </button>
            </div>
          </section>
        </div>

        <div class="space-y-4">
          <section
            v-if="selectedScope"
            class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800"
          >
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div class="space-y-2">
                <div class="flex flex-wrap items-center gap-2">
                  <PlatformTypeBadge :platform="selectedScope.platform" :type="selectedScope.account_type" />
                  <span
                    :class="[
                      'inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium',
                      selectedScope.enabled
                        ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
                        : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
                    ]"
                  >
                    {{ selectedScope.enabled ? '启用' : '停用' }}
                  </span>
                </div>
                <div class="text-xs text-gray-500 dark:text-gray-400">
                  这个作用域下的模型集合会在没有账号级显式模型映射时生效；错误规则按优先级从小到大匹配。
                </div>
                <div v-if="selectedScope.description" class="text-sm text-gray-600 dark:text-gray-300">
                  {{ selectedScope.description }}
                </div>
              </div>
              <div class="flex flex-wrap gap-2">
                <button type="button" class="btn btn-secondary btn-sm" @click="openEditScope(selectedScope)">
                  编辑作用域
                </button>
                <button type="button" class="btn btn-danger btn-sm" @click="removeScope(selectedScope)">
                  删除作用域
                </button>
              </div>
            </div>
          </section>

          <section
            v-if="selectedScope"
            class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800"
          >
            <div class="mb-3 flex items-center justify-between gap-2">
              <div>
                <div class="text-sm font-semibold text-gray-900 dark:text-white">模型集合</div>
                <div class="text-xs text-gray-500 dark:text-gray-400">当前 {{ selectedScope.model_set.length }} 个模型</div>
              </div>
              <button type="button" class="btn btn-secondary btn-sm" @click="openEditScope(selectedScope)">
                编辑模型集合
              </button>
            </div>

            <div v-if="selectedScope.model_set.length" class="flex flex-wrap gap-2">
              <span
                v-for="model in selectedScope.model_set"
                :key="model"
                class="inline-flex max-w-full items-center rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-300"
              >
                <span class="truncate">{{ model }}</span>
              </span>
            </div>
            <div v-else class="rounded-lg border border-dashed border-gray-200 px-3 py-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
              还没有配置模型集合，此作用域目前只会影响错误规则。
            </div>
          </section>

          <section
            v-if="selectedScope"
            class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800"
          >
            <div class="mb-3 flex items-center justify-between gap-2">
              <div>
                <div class="text-sm font-semibold text-gray-900 dark:text-white">错误规则</div>
                <div class="text-xs text-gray-500 dark:text-gray-400">
                  命中后可执行踢出号池、转发请求、删除账号、篡改响应。
                </div>
              </div>
              <button type="button" class="btn btn-primary btn-sm" @click="openCreateRule()">
                <Icon name="plus" size="sm" class="mr-1" />
                新建规则
              </button>
            </div>

            <div v-if="!selectedScope.rules.length" class="rounded-lg border border-dashed border-gray-200 px-3 py-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
              这个作用域还没有任何错误规则。
            </div>

            <div v-else class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-700">
              <div
                v-for="rule in selectedScope.rules"
                :key="rule.id"
                class="border-b border-gray-200 px-4 py-4 last:border-b-0 dark:border-dark-700"
              >
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div class="min-w-0 flex-1 space-y-2">
                    <div class="flex flex-wrap items-center gap-2">
                      <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ rule.name }}</span>
                      <span class="inline-flex rounded-full bg-gray-100 px-2 py-0.5 text-[11px] text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                        优先级 {{ rule.priority }}
                      </span>
                      <span
                        :class="[
                          'inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium',
                          rule.enabled
                            ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
                            : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
                        ]"
                      >
                        {{ rule.enabled ? '启用' : '停用' }}
                      </span>
                    </div>

                    <div class="flex flex-wrap gap-2 text-xs">
                      <span
                        v-for="code in rule.status_codes"
                        :key="`${rule.id}-code-${code}`"
                        class="inline-flex rounded-full bg-red-50 px-2 py-0.5 font-medium text-red-700 dark:bg-red-900/20 dark:text-red-300"
                      >
                        {{ code }}
                      </span>
                      <span
                        v-for="keyword in rule.keywords"
                        :key="`${rule.id}-kw-${keyword}`"
                        class="inline-flex rounded-full bg-gray-100 px-2 py-0.5 font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-300"
                      >
                        {{ keyword }}
                      </span>
                    </div>

                    <div class="flex flex-wrap gap-2 text-xs text-gray-600 dark:text-gray-300">
                      <span class="rounded-full bg-primary-50 px-2 py-0.5 dark:bg-primary-900/20">
                        {{ rule.match_mode === 'all' ? '状态码 + 关键词都要命中' : '状态码 / 关键词任一命中' }}
                      </span>
                      <span
                        v-for="action in formatRuleActions(rule)"
                        :key="`${rule.id}-${action}`"
                        class="rounded-full bg-amber-50 px-2 py-0.5 dark:bg-amber-900/20"
                      >
                        {{ action }}
                      </span>
                    </div>

                    <div v-if="rule.description" class="text-sm text-gray-600 dark:text-gray-300">
                      {{ rule.description }}
                    </div>
                  </div>

                  <div class="flex flex-wrap gap-2">
                    <button type="button" class="btn btn-secondary btn-sm" @click="openEditRule(rule)">
                      编辑
                    </button>
                    <button type="button" class="btn btn-danger btn-sm" @click="removeRule(rule)">
                      删除
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section
            v-else
            class="rounded-xl border border-dashed border-gray-200 bg-white px-4 py-16 text-center text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-400"
          >
            先在左侧选择或创建一个作用域，再配置模型集合和错误规则。
          </section>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button type="button" class="btn btn-secondary" @click="emit('close')">
          关闭
        </button>
      </div>
    </template>

    <BaseDialog
      :show="showScopeEditor"
      :title="editingScopeId ? '编辑作用域' : '新建作用域'"
      width="wide"
      @close="closeScopeEditor"
    >
      <form class="space-y-4" @submit.prevent="saveScope">
        <div class="grid gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">平台</label>
            <input v-model.trim="scopeForm.platform" type="text" class="input" placeholder="例如 openai / gemini" :disabled="!!editingScopeId" />
          </div>
          <div>
            <label class="input-label">类型</label>
            <input v-model.trim="scopeForm.account_type" type="text" class="input" placeholder="留空表示平台级作用域" :disabled="!!editingScopeId" />
          </div>
        </div>

        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input v-model="scopeForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
          <span>启用这个作用域</span>
        </label>

        <div>
          <label class="input-label">作用域说明</label>
          <input v-model.trim="scopeForm.description" type="text" class="input" placeholder="例如：OpenAI API Key 账号统一规则" />
        </div>

        <div>
          <label class="input-label">模型集合</label>
          <ModelWhitelistSelector
            v-model="scopeForm.model_set"
            :platform="scopeForm.platform"
          />
        </div>

        <div v-if="pendingDraft && !editingScopeId" class="rounded-xl bg-primary-50 px-3 py-2 text-xs text-primary-700 dark:bg-primary-900/20 dark:text-primary-300">
          当前有一条来自运维页面的错误草稿。保存作用域后，会自动继续创建这条规则。
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="closeScopeEditor">取消</button>
          <button type="button" class="btn btn-primary" :disabled="savingScope" @click="saveScope">
            {{ savingScope ? '保存中...' : '保存作用域' }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="showRuleEditor"
      :title="editingRuleId ? '编辑错误规则' : '新建错误规则'"
      width="wide"
      @close="closeRuleEditor"
    >
      <form class="space-y-4" @submit.prevent="saveRule">
        <div class="grid gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">规则名称</label>
            <input v-model.trim="ruleForm.name" type="text" class="input" placeholder="例如：429 自动切号" />
          </div>
          <div>
            <label class="input-label">优先级</label>
            <input v-model.number="ruleForm.priority" type="number" min="0" class="input" />
          </div>
        </div>

        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input v-model="ruleForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
          <span>启用这条规则</span>
        </label>

        <div class="grid gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">状态码</label>
            <input v-model="ruleForm.statusCodesText" type="text" class="input" placeholder="例如：429, 400" />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">多个状态码用逗号或空格分隔。</p>
          </div>
          <div>
            <label class="input-label">匹配模式</label>
            <select v-model="ruleForm.match_mode" class="input">
              <option value="any">状态码 / 关键词任一命中</option>
              <option value="all">状态码 + 关键词都命中</option>
            </select>
          </div>
        </div>

        <div>
          <label class="input-label">关键词</label>
          <textarea
            v-model="ruleForm.keywordsText"
            rows="4"
            class="input font-mono text-xs"
            placeholder="每行一个关键词，例如：
rate limit
model is not supported"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">用于匹配错误响应内容，不区分大小写。</p>
        </div>

        <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
          <div class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">命中后的动作</div>
          <div class="grid gap-3 md:grid-cols-2">
            <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="ruleForm.action_disable" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span>踢出号池：把账号状态设置为不正常并停止调度</span>
            </label>
            <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="ruleForm.action_failover" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span>转发请求：把当前请求继续切换到其他正常账号</span>
            </label>
            <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="ruleForm.action_delete" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span>删除账号：直接删除命中的账号</span>
            </label>
            <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="ruleForm.action_override" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span>篡改响应：把返回给用户的状态码或消息改写掉</span>
            </label>
          </div>
        </div>

        <div v-if="ruleForm.action_override" class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
          <div class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">篡改响应细节</div>
          <div class="grid gap-4 md:grid-cols-2">
            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="ruleForm.passthrough_code" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span>透传上游状态码</span>
            </label>
            <div>
              <label class="input-label">自定义状态码</label>
              <input
                v-model.number="ruleForm.response_code"
                type="number"
                min="100"
                max="599"
                class="input"
                :disabled="ruleForm.passthrough_code"
              />
            </div>
          </div>

          <div class="mt-4 grid gap-4 md:grid-cols-2">
            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="ruleForm.passthrough_body" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span>透传上游错误消息</span>
            </label>
            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="ruleForm.skip_monitoring" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span>命中时跳过运维监控记录</span>
            </label>
          </div>

          <div class="mt-4">
            <label class="input-label">自定义错误消息</label>
            <textarea
              v-model="ruleForm.custom_message"
              rows="3"
              class="input"
              :disabled="ruleForm.passthrough_body"
              placeholder="如果不透传上游消息，这里填写返回给用户的内容"
            />
          </div>
        </div>

        <div>
          <label class="input-label">规则说明</label>
          <input v-model.trim="ruleForm.description" type="text" class="input" placeholder="例如：429 命中时切号并保持用户无感" />
        </div>

        <div>
          <label class="input-label">样例响应</label>
          <textarea
            v-model="ruleForm.sample_response"
            rows="6"
            class="input font-mono text-xs"
            placeholder="建议直接粘贴运维页面里看到的响应详情，便于后续确认这条规则为什么存在。"
          />
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="closeRuleEditor">取消</button>
          <button type="button" class="btn btn-primary" :disabled="savingRule" @click="saveRule">
            {{ savingRule ? '保存中...' : '保存规则' }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import Icon from '@/components/icons/Icon.vue'
import ModelWhitelistSelector from '@/components/account/ModelWhitelistSelector.vue'
import { accountRulesAPI, type AccountRuleCatalog, type AccountRuleDraft, type AccountRuleErrorRule, type AccountRuleObservedScope, type AccountRuleScope, type UpsertAccountRuleRequest, type UpsertAccountRuleScopeRequest } from '@/api/admin/accountRules'
import { useAppStore } from '@/stores'

type DraftSource = 'request-error' | 'upstream-error'

interface Props {
  show: boolean
  draftSource?: DraftSource | null
  draftId?: number | null
}

interface Emits {
  (e: 'close'): void
  (e: 'updated'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()
const appStore = useAppStore()

const loading = ref(false)
const savingSettings = ref(false)
const savingScope = ref(false)
const savingRule = ref(false)
const showScopeEditor = ref(false)
const showRuleEditor = ref(false)
const selectedScopeId = ref<number | null>(null)
const editingScopeId = ref<number | null>(null)
const editingRuleId = ref<number | null>(null)
const editingRuleScopeId = ref<number | null>(null)
const pendingDraft = ref<AccountRuleDraft | null>(null)
const appliedDraftKey = ref('')
const catalog = ref<AccountRuleCatalog | null>(null)

const settingsForm = reactive({
  forward_max_attempts: 3
})

const scopeForm = reactive({
  platform: '',
  account_type: '',
  enabled: true,
  model_set: [] as string[],
  description: ''
})

const ruleForm = reactive({
  name: '',
  enabled: true,
  priority: 100,
  statusCodesText: '',
  keywordsText: '',
  match_mode: 'any' as 'any' | 'all',
  action_disable: false,
  action_failover: true,
  action_delete: false,
  action_override: false,
  passthrough_code: true,
  response_code: null as number | null,
  passthrough_body: true,
  custom_message: '',
  skip_monitoring: false,
  description: '',
  sample_response: ''
})

const selectedScope = computed(() => {
  return catalog.value?.scopes.find(scope => scope.id === selectedScopeId.value) ?? null
})

const draftKey = computed(() => {
  if (!props.draftSource || !props.draftId) return ''
  return `${props.draftSource}:${props.draftId}`
})

const draftHint = computed(() => {
  if (!props.draftSource || !props.draftId) return ''
  return `正在处理来自 /admin/ops 的错误草稿：${props.draftSource} #${props.draftId}`
})

const configuredScopeKeys = computed(() => {
  return new Set((catalog.value?.scopes ?? []).map(scopeKey))
})

const unconfiguredObservedScopes = computed(() => {
  return (catalog.value?.observed_scopes ?? []).filter(scope => !configuredScopeKeys.value.has(observedScopeKey(scope)))
})

function scopeKey(scope: Pick<AccountRuleScope, 'platform' | 'account_type'>): string {
  return `${scope.platform.trim().toLowerCase()}::${scope.account_type.trim().toLowerCase()}`
}

function observedScopeKey(scope: Pick<AccountRuleObservedScope, 'platform' | 'account_type'>): string {
  return `${scope.platform.trim().toLowerCase()}::${scope.account_type.trim().toLowerCase()}`
}

function resetScopeForm() {
  scopeForm.platform = ''
  scopeForm.account_type = ''
  scopeForm.enabled = true
  scopeForm.model_set = []
  scopeForm.description = ''
}

function resetRuleForm() {
  ruleForm.name = ''
  ruleForm.enabled = true
  ruleForm.priority = 100
  ruleForm.statusCodesText = ''
  ruleForm.keywordsText = ''
  ruleForm.match_mode = 'any'
  ruleForm.action_disable = false
  ruleForm.action_failover = true
  ruleForm.action_delete = false
  ruleForm.action_override = false
  ruleForm.passthrough_code = true
  ruleForm.response_code = null
  ruleForm.passthrough_body = true
  ruleForm.custom_message = ''
  ruleForm.skip_monitoring = false
  ruleForm.description = ''
  ruleForm.sample_response = ''
}

function syncSelection() {
  const scopes = catalog.value?.scopes ?? []
  if (!scopes.length) {
    selectedScopeId.value = null
    return
  }
  if (!selectedScopeId.value || !scopes.some(scope => scope.id === selectedScopeId.value)) {
    selectedScopeId.value = scopes[0].id
  }
}

async function loadCatalog(preferredScopeId?: number | null) {
  loading.value = true
  try {
    const data = await accountRulesAPI.getCatalog()
    catalog.value = data
    settingsForm.forward_max_attempts = data.settings.forward_max_attempts || 3
    if (preferredScopeId) {
      selectedScopeId.value = preferredScopeId
    }
    syncSelection()
    await maybeApplyDraft()
  } catch (error: any) {
    console.error('[AccountRuleManagerModal] Failed to load catalog', error)
    appStore.showError(error?.response?.data?.message || error?.response?.data?.detail || '加载账号规则失败')
  } finally {
    loading.value = false
  }
}

async function maybeApplyDraft() {
  if (!props.show || !props.draftSource || !props.draftId) return
  if (draftKey.value && draftKey.value === appliedDraftKey.value) return

  try {
    const draft = await accountRulesAPI.getOpsDraft(props.draftSource, props.draftId)
    pendingDraft.value = draft
    if (draft.matched_scope_id) {
      selectedScopeId.value = draft.matched_scope_id
      openCreateRule(draft.rule, draft.matched_scope_id)
      pendingDraft.value = null
    } else {
      openCreateScope({
        platform: draft.platform,
        account_type: draft.account_type,
        account_count: 0
      })
      appStore.showInfo('没有找到匹配的作用域，请先创建作用域，然后继续保存这条规则。')
    }
    appliedDraftKey.value = draftKey.value
  } catch (error: any) {
    console.error('[AccountRuleManagerModal] Failed to load ops draft', error)
    appStore.showError(error?.response?.data?.message || error?.response?.data?.detail || '加载运维错误草稿失败')
  }
}

async function saveSettings() {
  if (!Number.isFinite(settingsForm.forward_max_attempts) || settingsForm.forward_max_attempts <= 0) {
    appStore.showError('统一转发次数上限必须大于 0')
    return
  }

  savingSettings.value = true
  try {
    const saved = await accountRulesAPI.updateSettings({
      forward_max_attempts: settingsForm.forward_max_attempts
    })
    settingsForm.forward_max_attempts = saved.forward_max_attempts
    appStore.showSuccess('规则设置已保存')
    emit('updated')
  } catch (error: any) {
    console.error('[AccountRuleManagerModal] Failed to save settings', error)
    appStore.showError(error?.response?.data?.message || error?.response?.data?.detail || '保存规则设置失败')
  } finally {
    savingSettings.value = false
  }
}

function openCreateScope(observed?: Partial<AccountRuleObservedScope>) {
  editingScopeId.value = null
  resetScopeForm()
  scopeForm.platform = String(observed?.platform || '').trim()
  scopeForm.account_type = String(observed?.account_type || '').trim()
  showScopeEditor.value = true
}

function openEditScope(scope: AccountRuleScope) {
  editingScopeId.value = scope.id
  scopeForm.platform = scope.platform
  scopeForm.account_type = scope.account_type
  scopeForm.enabled = scope.enabled
  scopeForm.model_set = [...(scope.model_set || [])]
  scopeForm.description = scope.description || ''
  showScopeEditor.value = true
}

function closeScopeEditor() {
  showScopeEditor.value = false
  editingScopeId.value = null
  resetScopeForm()
}

async function saveScope() {
  if (!scopeForm.platform.trim()) {
    appStore.showError('平台不能为空')
    return
  }

  const payload: UpsertAccountRuleScopeRequest = {
    platform: scopeForm.platform.trim(),
    account_type: scopeForm.account_type.trim(),
    enabled: scopeForm.enabled,
    model_set: [...scopeForm.model_set],
    description: scopeForm.description.trim()
  }

  savingScope.value = true
  try {
    const scope = editingScopeId.value
      ? await accountRulesAPI.updateScope(editingScopeId.value, payload)
      : await accountRulesAPI.createScope(payload)

    await loadCatalog(scope.id)
    appStore.showSuccess(editingScopeId.value ? '作用域已更新' : '作用域已创建')
    emit('updated')
    closeScopeEditor()

    if (pendingDraft.value && scopeKey(scope) === scopeKey(pendingDraft.value)) {
      openCreateRule(pendingDraft.value.rule, scope.id)
      pendingDraft.value = null
    }
  } catch (error: any) {
    console.error('[AccountRuleManagerModal] Failed to save scope', error)
    appStore.showError(error?.response?.data?.message || error?.response?.data?.detail || '保存作用域失败')
  } finally {
    savingScope.value = false
  }
}

async function removeScope(scope: AccountRuleScope) {
  if (!window.confirm(`确定删除作用域 ${scope.platform}${scope.account_type ? ` / ${scope.account_type}` : ''} 吗？`)) {
    return
  }

  try {
    await accountRulesAPI.deleteScope(scope.id)
    await loadCatalog()
    appStore.showSuccess('作用域已删除')
    emit('updated')
  } catch (error: any) {
    console.error('[AccountRuleManagerModal] Failed to delete scope', error)
    appStore.showError(error?.response?.data?.message || error?.response?.data?.detail || '删除作用域失败')
  }
}

function openCreateRule(prefill?: Partial<AccountRuleErrorRule> | null, scopeId?: number | null) {
  editingRuleId.value = null
  editingRuleScopeId.value = scopeId ?? selectedScopeId.value
  resetRuleForm()
  if (prefill) {
    ruleForm.name = prefill.name || ''
    ruleForm.enabled = prefill.enabled ?? true
    ruleForm.priority = prefill.priority ?? 100
    ruleForm.statusCodesText = (prefill.status_codes || []).join(', ')
    ruleForm.keywordsText = (prefill.keywords || []).join('\n')
    ruleForm.match_mode = prefill.match_mode || 'any'
    ruleForm.action_disable = prefill.action_disable ?? false
    ruleForm.action_failover = prefill.action_failover ?? true
    ruleForm.action_delete = prefill.action_delete ?? false
    ruleForm.action_override = prefill.action_override ?? false
    ruleForm.passthrough_code = prefill.passthrough_code ?? true
    ruleForm.response_code = prefill.response_code ?? null
    ruleForm.passthrough_body = prefill.passthrough_body ?? true
    ruleForm.custom_message = prefill.custom_message || ''
    ruleForm.skip_monitoring = prefill.skip_monitoring ?? false
    ruleForm.description = prefill.description || ''
    ruleForm.sample_response = prefill.sample_response || ''
  }
  showRuleEditor.value = true
}

function openEditRule(rule: AccountRuleErrorRule) {
  editingRuleId.value = rule.id
  editingRuleScopeId.value = rule.scope_id
  openCreateRule(rule, rule.scope_id)
  editingRuleId.value = rule.id
}

function closeRuleEditor() {
  showRuleEditor.value = false
  editingRuleId.value = null
  editingRuleScopeId.value = null
  resetRuleForm()
}

function parseStatusCodes(input: string): number[] {
  return Array.from(
    new Set(
      input
        .split(/[\s,]+/)
        .map(item => Number.parseInt(item.trim(), 10))
        .filter(code => Number.isFinite(code) && code >= 0 && code <= 999)
    )
  )
}

function parseKeywords(input: string): string[] {
  return Array.from(
    new Set(
      input
        .split('\n')
        .map(item => item.trim())
        .filter(Boolean)
    )
  )
}

async function saveRule() {
  if (!editingRuleScopeId.value) {
    appStore.showError('请先选择一个作用域')
    return
  }
  if (!ruleForm.name.trim()) {
    appStore.showError('规则名称不能为空')
    return
  }

  const status_codes = parseStatusCodes(ruleForm.statusCodesText)
  const keywords = parseKeywords(ruleForm.keywordsText)
  if (!status_codes.length && !keywords.length) {
    appStore.showError('至少要填写一个状态码或关键词')
    return
  }
  if (!ruleForm.action_disable && !ruleForm.action_failover && !ruleForm.action_delete && !ruleForm.action_override) {
    appStore.showError('至少要勾选一个动作')
    return
  }
  if (ruleForm.action_override && !ruleForm.passthrough_code && (!ruleForm.response_code || ruleForm.response_code <= 0)) {
    appStore.showError('关闭状态码透传时，必须填写自定义状态码')
    return
  }
  if (ruleForm.action_override && !ruleForm.passthrough_body && !ruleForm.custom_message.trim()) {
    appStore.showError('关闭消息透传时，必须填写自定义错误消息')
    return
  }

  const payload: UpsertAccountRuleRequest = {
    name: ruleForm.name.trim(),
    enabled: ruleForm.enabled,
    priority: ruleForm.priority,
    status_codes,
    keywords,
    match_mode: ruleForm.match_mode,
    action_disable: ruleForm.action_disable,
    action_failover: ruleForm.action_failover,
    action_delete: ruleForm.action_delete,
    action_override: ruleForm.action_override,
    passthrough_code: ruleForm.passthrough_code,
    response_code: ruleForm.passthrough_code ? null : ruleForm.response_code,
    passthrough_body: ruleForm.passthrough_body,
    custom_message: ruleForm.passthrough_body ? null : ruleForm.custom_message.trim(),
    skip_monitoring: ruleForm.skip_monitoring,
    description: ruleForm.description.trim(),
    sample_response: ruleForm.sample_response.trim()
  }

  savingRule.value = true
  try {
    if (editingRuleId.value) {
      await accountRulesAPI.updateRule(editingRuleId.value, payload)
    } else {
      await accountRulesAPI.createRule(editingRuleScopeId.value, payload)
    }
    await loadCatalog(editingRuleScopeId.value)
    appStore.showSuccess(editingRuleId.value ? '规则已更新' : '规则已创建')
    emit('updated')
    closeRuleEditor()
  } catch (error: any) {
    console.error('[AccountRuleManagerModal] Failed to save rule', error)
    appStore.showError(error?.response?.data?.message || error?.response?.data?.detail || '保存规则失败')
  } finally {
    savingRule.value = false
  }
}

async function removeRule(rule: AccountRuleErrorRule) {
  if (!window.confirm(`确定删除规则「${rule.name}」吗？`)) {
    return
  }

  try {
    await accountRulesAPI.deleteRule(rule.id)
    await loadCatalog(rule.scope_id)
    appStore.showSuccess('规则已删除')
    emit('updated')
  } catch (error: any) {
    console.error('[AccountRuleManagerModal] Failed to delete rule', error)
    appStore.showError(error?.response?.data?.message || error?.response?.data?.detail || '删除规则失败')
  }
}

function formatRuleActions(rule: AccountRuleErrorRule): string[] {
  const actions: string[] = []
  if (rule.action_disable) actions.push('踢出号池')
  if (rule.action_failover) actions.push('转发请求')
  if (rule.action_delete) actions.push('删除账号')
  if (rule.action_override) actions.push('篡改响应')
  return actions
}

watch(
  () => props.show,
  async (show) => {
    if (!show) {
      showScopeEditor.value = false
      showRuleEditor.value = false
      editingScopeId.value = null
      editingRuleId.value = null
      editingRuleScopeId.value = null
      resetScopeForm()
      resetRuleForm()
      return
    }
    await loadCatalog(selectedScopeId.value)
  },
  { immediate: true }
)

watch(
  () => catalog.value?.scopes,
  () => {
    syncSelection()
  }
)

watch(
  () => [props.draftSource, props.draftId],
  ([source, id]) => {
    if (!source || !id) {
      appliedDraftKey.value = ''
      pendingDraft.value = null
    }
  }
)
</script>
