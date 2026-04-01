<template>
  <BaseDialog
    :show="show"
    title="账号规则管理"
    width="extra-wide"
    @close="emit('close')"
  >
    <div class="space-y-5">
      <section class="rounded-2xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="space-y-1.5">
            <div class="text-sm font-semibold text-gray-900 dark:text-white">先定运行策略，再选生效范围，最后维护模型与错误规则资源</div>
            <div class="max-w-3xl text-xs leading-6 text-gray-500 dark:text-gray-400">
              绑定优先级为“平台 + 业务类型”高于“平台”。模型集合和错误集合都是可复用资源库，选中某个绑定后再决定它挂哪套策略，阅读和维护成本会更低。
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

        <div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <div class="rounded-xl border border-gray-200 bg-white px-4 py-3 dark:border-dark-700 dark:bg-dark-800">
            <div class="text-[11px] uppercase tracking-[0.18em] text-gray-400 dark:text-gray-500">Binding</div>
            <div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ catalog?.bindings.length ?? 0 }}</div>
            <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">已启用 {{ enabledBindingCount }} 个绑定</div>
          </div>
          <div class="rounded-xl border border-gray-200 bg-white px-4 py-3 dark:border-dark-700 dark:bg-dark-800">
            <div class="text-[11px] uppercase tracking-[0.18em] text-gray-400 dark:text-gray-500">Coverage</div>
            <div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ unconfiguredObservedBindings.length }}</div>
            <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">个已观测范围仍未建绑定</div>
          </div>
          <div class="rounded-xl border border-gray-200 bg-white px-4 py-3 dark:border-dark-700 dark:bg-dark-800">
            <div class="text-[11px] uppercase tracking-[0.18em] text-gray-400 dark:text-gray-500">Model Sets</div>
            <div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ catalog?.model_collections.length ?? 0 }}</div>
            <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">模型白名单资源库</div>
          </div>
          <div class="rounded-xl border border-gray-200 bg-white px-4 py-3 dark:border-dark-700 dark:bg-dark-800">
            <div class="text-[11px] uppercase tracking-[0.18em] text-gray-400 dark:text-gray-500">Error Rules</div>
            <div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ totalRuleCount }}</div>
            <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">分布在 {{ catalog?.error_collections.length ?? 0 }} 个错误集合里</div>
          </div>
        </div>
      </section>

      <section class="rounded-2xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="space-y-1">
            <div class="text-sm font-semibold text-gray-900 dark:text-white">运行设置</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">
              这里只放全局运行策略。错误规则里的“转发请求”和这里的 429 自动转发，都共享同一个“统一转发次数上限”。
            </div>
          </div>
          <div class="rounded-full bg-gray-100 px-3 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">
            当前 429 自动转发：{{ settingsForm.failover_on_429 ? '开启' : '关闭' }}
          </div>
        </div>

        <div class="mt-4 grid gap-4 xl:grid-cols-[minmax(0,1.25fr),minmax(280px,0.75fr)]">
          <div class="rounded-2xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900">
            <div class="flex items-start justify-between gap-4">
              <div class="space-y-1">
                <div class="text-sm font-semibold text-gray-900 dark:text-white">429 自动转发</div>
                <div class="text-xs leading-6 text-gray-500 dark:text-gray-400">
                  开启后，遇到 429 会先保留限流状态和自动恢复时间，再按统一转发次数上限切换其他正常账号；关闭后只记录限流并直接把 429 返回给当前请求。
                </div>
              </div>
              <button
                type="button"
                :aria-pressed="settingsForm.failover_on_429"
                class="relative inline-flex h-6 w-11 flex-shrink-0 rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:focus:ring-offset-dark-800"
                :class="settingsForm.failover_on_429 ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'"
                @click="settingsForm.failover_on_429 = !settingsForm.failover_on_429"
              >
                <span
                  class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow transition duration-200 ease-in-out"
                  :class="settingsForm.failover_on_429 ? 'translate-x-5' : 'translate-x-0'"
                />
              </button>
            </div>
            <div class="mt-4 rounded-xl border border-dashed border-gray-200 bg-white px-3 py-3 text-xs leading-6 text-gray-500 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-400">
              建议：429 场景优先用这里的自动转发，不要在错误规则里用“踢出号池”替代。前者会保留限流自动恢复，后者会把账号直接打成错误态。
            </div>
          </div>

          <div class="rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
            <label class="input-label">统一转发次数上限</label>
            <input
              v-model.number="settingsForm.forward_max_attempts"
              type="number"
              min="1"
              class="input"
              :disabled="loading"
            />
            <p class="mt-2 text-xs leading-6 text-gray-500 dark:text-gray-400">
              命中“转发请求”动作或 429 自动转发时，单个请求最多还能切换多少次账号。
            </p>
            <button
              type="button"
              class="btn btn-primary mt-4 w-full"
              :disabled="savingSettings || loading"
              @click="saveSettings"
            >
              {{ savingSettings ? '保存中...' : '保存运行设置' }}
            </button>
          </div>
        </div>
      </section>

      <div v-if="loading" class="flex items-center justify-center py-12">
        <Icon name="refresh" size="lg" class="animate-spin text-gray-400" />
      </div>

      <div v-else class="space-y-5">
        <section class="grid gap-4 xl:grid-cols-[320px,minmax(0,1fr)]">
          <aside class="space-y-4">
            <section class="rounded-2xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
              <div class="mb-3 flex items-center justify-between gap-2">
                <div>
                  <div class="text-sm font-semibold text-gray-900 dark:text-white">步骤 1：选择生效范围</div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">平台 / 业务类型绑定共 {{ catalog?.bindings.length ?? 0 }} 个</div>
                </div>
                <button type="button" class="btn btn-primary btn-sm" @click="openCreateBinding()">
                  <Icon name="plus" size="sm" class="mr-1" />
                  新建
                </button>
              </div>

              <div v-if="!catalog?.bindings.length" class="rounded-lg border border-dashed border-gray-200 px-3 py-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
                还没有任何绑定关系。
              </div>

              <div v-else class="space-y-2">
                <button
                  v-for="binding in catalog.bindings"
                  :key="binding.id"
                  type="button"
                  :class="[
                    'w-full rounded-xl border px-3 py-3 text-left transition-colors',
                    binding.id === selectedBindingId
                      ? 'border-primary-300 bg-primary-50 dark:border-primary-700 dark:bg-primary-900/20'
                      : 'border-gray-200 hover:bg-gray-50 dark:border-dark-600 dark:hover:bg-dark-700'
                  ]"
                  @click="selectedBindingId = binding.id"
                >
                  <div class="flex items-start justify-between gap-3">
                    <div class="min-w-0">
                      <div class="flex flex-wrap items-center gap-2">
                        <span class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-200">
                          <PlatformIcon :platform="binding.platform" size="xs" />
                          <span>{{ formatPlatformLabel(binding.platform) }}</span>
                        </span>
                        <span
                          :class="[
                            'inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium',
                            businessTypeBadgeClass(binding.business_type)
                          ]"
                        >
                          {{ formatBusinessTypeLabel(binding.business_type) }}
                        </span>
                        <span
                          :class="[
                            'inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium',
                            binding.enabled
                              ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
                              : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
                          ]"
                        >
                          {{ binding.enabled ? '启用' : '停用' }}
                        </span>
                      </div>
                      <div class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                        模型集合：{{ resolveModelCollectionName(binding.model_collection_id) || '未绑定' }}
                      </div>
                      <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                        错误集合：{{ resolveErrorCollectionName(binding.error_collection_id) || '未绑定' }}
                      </div>
                      <div v-if="binding.description" class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">
                        {{ binding.description }}
                      </div>
                    </div>
                    <Icon name="chevronRight" size="sm" class="mt-0.5 text-gray-400" />
                  </div>
                </button>
              </div>

              <div class="mt-4 border-t border-gray-200 pt-4 dark:border-dark-700">
                <div class="mb-3">
                  <div class="text-sm font-semibold text-gray-900 dark:text-white">从现有账号快速建绑定</div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">
                    已在账号管理里出现过、但还没建绑定的范围，可以直接一键补齐。
                  </div>
                </div>

                <div v-if="!unconfiguredObservedBindings.length" class="rounded-lg border border-dashed border-gray-200 px-3 py-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
                  当前已覆盖所有已观测到的平台 / 业务类型组合。
                </div>

                <div v-else class="space-y-2">
                  <button
                    v-for="binding in unconfiguredObservedBindings"
                    :key="observedBindingKey(binding)"
                    type="button"
                    class="flex w-full items-center justify-between rounded-xl border border-dashed border-gray-200 px-3 py-2.5 text-left hover:bg-gray-50 dark:border-dark-600 dark:hover:bg-dark-700"
                    @click="openCreateBinding(binding)"
                  >
                    <div class="flex min-w-0 items-center gap-2">
                      <span class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-200">
                        <PlatformIcon :platform="binding.platform" size="xs" />
                        <span>{{ formatPlatformLabel(binding.platform) }}</span>
                      </span>
                      <span
                        :class="[
                          'inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium',
                          businessTypeBadgeClass(binding.business_type)
                        ]"
                      >
                        {{ formatBusinessTypeLabel(binding.business_type) }}
                      </span>
                      <span class="text-xs text-gray-500 dark:text-gray-400">{{ binding.account_count }} 个账号</span>
                    </div>
                    <span class="text-xs font-medium text-primary-600 dark:text-primary-300">创建</span>
                  </button>
                </div>
              </div>
            </section>
          </aside>

          <div class="space-y-4">
            <section class="rounded-2xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
              <div v-if="selectedBinding" class="space-y-4">
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div class="space-y-2">
                    <div class="text-sm font-semibold text-gray-900 dark:text-white">步骤 2：为当前范围绑定策略资源</div>
                    <div class="flex flex-wrap items-center gap-2">
                      <span class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-200">
                        <PlatformIcon :platform="selectedBinding.platform" size="xs" />
                        <span>{{ formatPlatformLabel(selectedBinding.platform) }}</span>
                      </span>
                      <span
                        :class="[
                          'inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium',
                          businessTypeBadgeClass(selectedBinding.business_type)
                        ]"
                      >
                        {{ formatBusinessTypeLabel(selectedBinding.business_type) }}
                      </span>
                      <span
                        :class="[
                          'inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium',
                          selectedBinding.enabled
                            ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
                            : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
                        ]"
                      >
                        {{ selectedBinding.enabled ? '启用' : '停用' }}
                      </span>
                    </div>
                    <div class="text-xs leading-6 text-gray-500 dark:text-gray-400">
                      绑定只决定这个范围最终使用哪套模型集合和错误集合。集合内容本身在下方资源库里维护。
                    </div>
                    <div v-if="selectedBinding.description" class="text-sm text-gray-600 dark:text-gray-300">
                      {{ selectedBinding.description }}
                    </div>
                  </div>
                  <div class="flex flex-wrap gap-2">
                    <button type="button" class="btn btn-secondary btn-sm" @click="openEditBinding(selectedBinding)">
                      编辑绑定
                    </button>
                    <button type="button" class="btn btn-danger btn-sm" @click="removeBinding(selectedBinding)">
                      删除绑定
                    </button>
                  </div>
                </div>

                <div class="grid gap-4 lg:grid-cols-2">
                  <div class="rounded-2xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900">
                    <div class="flex items-start justify-between gap-3">
                      <div>
                        <div class="text-sm font-semibold text-gray-900 dark:text-white">模型策略</div>
                        <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">控制这个范围允许调度哪些模型。</div>
                      </div>
                      <button type="button" class="btn btn-secondary btn-sm" @click="openEditBinding(selectedBinding)">
                        改绑定
                      </button>
                    </div>

                    <div v-if="selectedBindingModelCollection" class="mt-4 space-y-3">
                      <div>
                        <div class="text-sm font-semibold text-gray-900 dark:text-white">{{ selectedBindingModelCollection.name }}</div>
                        <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                          模型 {{ selectedBindingModelCollection.models.length }} 个，已被 {{ modelCollectionBindingCounts[selectedBindingModelCollection.id] || 0 }} 个绑定使用。
                        </div>
                      </div>
                      <div v-if="selectedBindingModelCollection.description" class="text-sm text-gray-600 dark:text-gray-300">
                        {{ selectedBindingModelCollection.description }}
                      </div>
                      <div class="flex flex-wrap gap-2">
                        <button type="button" class="btn btn-secondary btn-sm" @click="openEditModelCollection(selectedBindingModelCollection)">
                          编辑集合
                        </button>
                        <button type="button" class="btn btn-danger btn-sm" @click="removeModelCollection(selectedBindingModelCollection)">
                          删除集合
                        </button>
                      </div>
                    </div>
                    <div v-else class="mt-4 rounded-xl border border-dashed border-gray-200 bg-white px-3 py-5 text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-400">
                      当前绑定还没有挂模型集合。先新建模型集合，再回到“编辑绑定”把它挂上。
                    </div>
                  </div>

                  <div class="rounded-2xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900">
                    <div class="flex items-start justify-between gap-3">
                      <div>
                        <div class="text-sm font-semibold text-gray-900 dark:text-white">错误策略</div>
                        <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">控制命中错误时是转发、摘除、删除，还是改写响应。</div>
                      </div>
                      <button type="button" class="btn btn-secondary btn-sm" @click="openEditBinding(selectedBinding)">
                        改绑定
                      </button>
                    </div>

                    <div v-if="selectedBindingErrorCollection" class="mt-4 space-y-3">
                      <div>
                        <div class="text-sm font-semibold text-gray-900 dark:text-white">{{ selectedBindingErrorCollection.name }}</div>
                        <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                          规则 {{ selectedBindingErrorCollection.rules.length }} 条，已被 {{ errorCollectionBindingCounts[selectedBindingErrorCollection.id] || 0 }} 个绑定使用。
                        </div>
                      </div>
                      <div v-if="selectedBindingErrorCollection.description" class="text-sm text-gray-600 dark:text-gray-300">
                        {{ selectedBindingErrorCollection.description }}
                      </div>
                      <div class="flex flex-wrap gap-2">
                        <button type="button" class="btn btn-secondary btn-sm" @click="openEditErrorCollection(selectedBindingErrorCollection)">
                          编辑集合
                        </button>
                        <button type="button" class="btn btn-secondary btn-sm" @click="openCreateRule(undefined, selectedBindingErrorCollection.id)">
                          新建规则
                        </button>
                      </div>
                    </div>
                    <div v-else class="mt-4 rounded-xl border border-dashed border-gray-200 bg-white px-3 py-5 text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-400">
                      当前绑定还没有挂错误集合。先建错误集合，再回到“编辑绑定”把它挂上。
                    </div>

                    <div class="mt-4 rounded-xl border border-dashed border-amber-200 bg-amber-50 px-3 py-3 text-xs leading-6 text-amber-800 dark:border-amber-700/40 dark:bg-amber-900/20 dark:text-amber-200">
                      429 自动转发{{ settingsForm.failover_on_429 ? '已开启' : '已关闭' }}。{{ settingsForm.failover_on_429 ? '当前范围遇到 429 会优先保留限流状态并按统一上限切换账号。' : '当前范围遇到 429 只会记录限流，不会自动切到其他账号。' }}
                    </div>
                  </div>
                </div>
              </div>

              <div v-else class="rounded-xl border border-dashed border-gray-200 px-4 py-10 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
                先在左侧选择一个绑定，右侧才会显示这个范围当前挂载的模型策略和错误策略。
              </div>
            </section>
        </div>
        </section>

        <section class="grid gap-4 2xl:grid-cols-[0.85fr,1.15fr]">
          <section class="rounded-2xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <div class="mb-3 flex items-center justify-between gap-2">
              <div>
                <div class="text-sm font-semibold text-gray-900 dark:text-white">步骤 3A：模型集合资源库</div>
                <div class="text-xs text-gray-500 dark:text-gray-400">共 {{ catalog?.model_collections.length ?? 0 }} 个</div>
              </div>
              <button type="button" class="btn btn-primary btn-sm" @click="openCreateModelCollection()">
                <Icon name="plus" size="sm" class="mr-1" />
                新建
              </button>
            </div>

            <div v-if="!catalog?.model_collections.length" class="rounded-lg border border-dashed border-gray-200 px-3 py-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
              还没有任何模型集合。
            </div>

            <div v-else class="space-y-4">
              <div class="grid gap-3 lg:grid-cols-2 2xl:grid-cols-3">
                <button
                  v-for="collection in catalog.model_collections"
                  :key="collection.id"
                  type="button"
                  :class="[
                    'rounded-2xl border px-4 py-4 text-left transition-colors',
                    collection.id === selectedModelCollectionId
                      ? 'border-primary-300 bg-primary-50 dark:border-primary-700 dark:bg-primary-900/20'
                      : 'border-gray-200 hover:bg-gray-50 dark:border-dark-600 dark:hover:bg-dark-700'
                  ]"
                  @click="selectedModelCollectionId = collection.id"
                >
                  <div class="flex flex-wrap items-start justify-between gap-2">
                    <div class="min-w-0">
                      <div class="text-sm font-semibold text-gray-900 dark:text-white">{{ collection.name }}</div>
                      <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                        模型 {{ collection.models.length }} 个，绑定 {{ modelCollectionBindingCounts[collection.id] || 0 }} 个
                      </div>
                    </div>
                    <span
                      v-if="selectedBinding?.model_collection_id === collection.id"
                      class="inline-flex rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300"
                    >
                      当前绑定使用中
                    </span>
                  </div>
                  <div
                    v-if="collection.description"
                    class="mt-3 line-clamp-2 text-xs leading-6 text-gray-500 dark:text-gray-400"
                  >
                    {{ collection.description }}
                  </div>
                </button>
              </div>

              <div
                v-if="selectedModelCollection"
                class="rounded-2xl border border-gray-200 p-4 dark:border-dark-700"
              >
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div class="space-y-2">
                    <div class="flex flex-wrap items-center gap-2">
                      <div class="text-sm font-semibold text-gray-900 dark:text-white">{{ selectedModelCollection.name }}</div>
                      <span
                        v-if="selectedBinding?.model_collection_id === selectedModelCollection.id"
                        class="inline-flex rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300"
                      >
                        当前绑定正在使用
                      </span>
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">
                      当前 {{ selectedModelCollection.models.length }} 个模型，已被 {{ modelCollectionBindingCounts[selectedModelCollection.id] || 0 }} 个绑定使用。
                    </div>
                    <div v-if="selectedModelCollection.description" class="text-sm text-gray-600 dark:text-gray-300">
                      {{ selectedModelCollection.description }}
                    </div>
                  </div>
                  <div class="flex flex-wrap gap-2">
                    <button type="button" class="btn btn-secondary btn-sm" @click="openEditModelCollection(selectedModelCollection)">
                      编辑集合
                    </button>
                    <button type="button" class="btn btn-danger btn-sm" @click="removeModelCollection(selectedModelCollection)">
                      删除集合
                    </button>
                  </div>
                </div>

                <div v-if="selectedModelCollection.models.length" class="mt-4 flex flex-wrap gap-2">
                  <span
                    v-for="model in selectedModelCollection.models"
                    :key="model"
                    class="inline-flex max-w-full items-center rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-300"
                  >
                    <span class="truncate">{{ model }}</span>
                  </span>
                </div>
                <div v-else class="mt-4 rounded-lg border border-dashed border-gray-200 px-3 py-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
                  这个模型集合里还没有任何模型。
                </div>
              </div>
            </div>
          </section>

          <section class="rounded-2xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <div class="mb-3 flex items-center justify-between gap-2">
              <div>
                <div class="text-sm font-semibold text-gray-900 dark:text-white">步骤 3B：错误规则资源库</div>
                <div class="text-xs text-gray-500 dark:text-gray-400">共 {{ catalog?.error_collections.length ?? 0 }} 个错误集合</div>
              </div>
              <div class="flex flex-wrap gap-2">
                <button type="button" class="btn btn-secondary btn-sm" :disabled="!selectedErrorCollection" @click="openCreateRule()">
                  新建规则
                </button>
                <button type="button" class="btn btn-primary btn-sm" @click="openCreateErrorCollection()">
                  <Icon name="plus" size="sm" class="mr-1" />
                  新建集合
                </button>
              </div>
            </div>

            <div v-if="!catalog?.error_collections.length" class="rounded-lg border border-dashed border-gray-200 px-3 py-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
              还没有任何错误规则集合。
            </div>

            <div v-else class="space-y-4">
              <div class="grid gap-3 lg:grid-cols-2 2xl:grid-cols-3">
                <button
                  v-for="collection in catalog.error_collections"
                  :key="collection.id"
                  type="button"
                  :class="[
                    'rounded-2xl border px-4 py-4 text-left transition-colors',
                    collection.id === selectedErrorCollectionId
                      ? 'border-primary-300 bg-primary-50 dark:border-primary-700 dark:bg-primary-900/20'
                      : 'border-gray-200 hover:bg-gray-50 dark:border-dark-600 dark:hover:bg-dark-700'
                  ]"
                  @click="selectedErrorCollectionId = collection.id"
                >
                  <div class="flex flex-wrap items-start justify-between gap-2">
                    <div class="min-w-0">
                      <div class="text-sm font-semibold text-gray-900 dark:text-white">{{ collection.name }}</div>
                      <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                        规则 {{ collection.rules.length }} 条，绑定 {{ errorCollectionBindingCounts[collection.id] || 0 }} 个
                      </div>
                    </div>
                    <span
                      v-if="selectedBinding?.error_collection_id === collection.id"
                      class="inline-flex rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300"
                    >
                      当前绑定使用中
                    </span>
                  </div>
                  <div
                    v-if="collection.description"
                    class="mt-3 line-clamp-2 text-xs leading-6 text-gray-500 dark:text-gray-400"
                  >
                    {{ collection.description }}
                  </div>
                </button>
              </div>

              <div
                v-if="selectedErrorCollection"
                class="space-y-4 rounded-2xl border border-gray-200 p-4 dark:border-dark-700"
              >
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div class="space-y-2">
                    <div class="flex flex-wrap items-center gap-2">
                      <div class="text-sm font-semibold text-gray-900 dark:text-white">{{ selectedErrorCollection.name }}</div>
                      <span
                        v-if="selectedBinding?.error_collection_id === selectedErrorCollection.id"
                        class="inline-flex rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300"
                      >
                        当前绑定正在使用
                      </span>
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">
                      当前 {{ selectedErrorCollection.rules.length }} 条规则，已被 {{ errorCollectionBindingCounts[selectedErrorCollection.id] || 0 }} 个绑定使用。
                    </div>
                    <div v-if="selectedErrorCollection.description" class="text-sm text-gray-600 dark:text-gray-300">
                      {{ selectedErrorCollection.description }}
                    </div>
                  </div>
                  <div class="flex flex-wrap gap-2">
                    <button type="button" class="btn btn-secondary btn-sm" @click="openEditErrorCollection(selectedErrorCollection)">
                      编辑集合
                    </button>
                    <button type="button" class="btn btn-danger btn-sm" @click="removeErrorCollection(selectedErrorCollection)">
                      删除集合
                    </button>
                  </div>
                </div>

                <div v-if="!selectedErrorCollection.rules.length" class="rounded-lg border border-dashed border-gray-200 px-3 py-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
                  这个错误集合还没有任何规则。
                </div>

                <div v-else class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-700">
                  <div
                    v-for="rule in selectedErrorCollection.rules"
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
              </div>
            </div>
          </section>
        </section>
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
      :show="showBindingEditor"
      :title="editingBindingId ? '编辑绑定' : '新建绑定'"
      width="wide"
      @close="closeBindingEditor"
    >
      <form class="space-y-4" @submit.prevent="saveBinding">
        <div class="grid gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">平台</label>
            <input v-model.trim="bindingForm.platform" type="text" class="input" placeholder="例如 openai / gemini" />
          </div>
          <div>
            <label class="input-label">业务类型</label>
            <input v-model.trim="bindingForm.business_type" type="text" class="input" placeholder="例如 team / google_ai_pro；留空表示平台级绑定" />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">OpenAI / Sora 优先取 plan_type；Gemini 取 tier_id 或 oauth_type；Antigravity 取订阅 tier。</p>
          </div>
        </div>

        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input v-model="bindingForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
          <span>启用这个绑定</span>
        </label>

        <div class="grid gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">模型集合</label>
            <select v-model="bindingForm.model_collection_id" class="input">
              <option value="">未绑定</option>
              <option v-for="collection in catalog?.model_collections ?? []" :key="collection.id" :value="String(collection.id)">
                {{ collection.name }}
              </option>
            </select>
            <button type="button" class="mt-2 text-xs font-medium text-primary-600 dark:text-primary-300" @click="openCreateModelCollection(true)">
              在这里新建模型集合
            </button>
          </div>
          <div>
            <label class="input-label">错误集合</label>
            <select v-model="bindingForm.error_collection_id" class="input">
              <option value="">未绑定</option>
              <option v-for="collection in catalog?.error_collections ?? []" :key="collection.id" :value="String(collection.id)">
                {{ collection.name }}
              </option>
            </select>
            <button type="button" class="mt-2 text-xs font-medium text-primary-600 dark:text-primary-300" @click="openCreateErrorCollection(true)">
              在这里新建错误集合
            </button>
          </div>
        </div>

        <div>
          <label class="input-label">绑定说明</label>
          <input v-model.trim="bindingForm.description" type="text" class="input" placeholder="例如：OpenAI Team 统一绑定到团队模型集合和限流规则" />
        </div>

        <div v-if="pendingDraft" class="rounded-xl bg-primary-50 px-3 py-2 text-xs text-primary-700 dark:bg-primary-900/20 dark:text-primary-300">
          当前有一条来自运维页面的错误草稿。只要这个绑定最终关联了错误集合，保存后会自动继续创建这条规则。
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="closeBindingEditor">取消</button>
          <button type="button" class="btn btn-primary" :disabled="savingBinding" @click="saveBinding">
            {{ savingBinding ? '保存中...' : '保存绑定' }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="showModelCollectionEditor"
      :title="editingModelCollectionId ? '编辑模型集合' : '新建模型集合'"
      width="wide"
      @close="closeModelCollectionEditor"
    >
      <form class="space-y-4" @submit.prevent="saveModelCollection">
        <div>
          <label class="input-label">集合名称</label>
          <input v-model.trim="modelCollectionForm.name" type="text" class="input" placeholder="例如：OpenAI Team 模型集合" />
        </div>

        <div>
          <label class="input-label">集合说明</label>
          <input v-model.trim="modelCollectionForm.description" type="text" class="input" placeholder="例如：团队账号允许调度的模型列表" />
        </div>

        <div>
          <label class="input-label">模型列表</label>
          <ModelWhitelistSelector v-model="modelCollectionForm.models" />
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="closeModelCollectionEditor">取消</button>
          <button type="button" class="btn btn-primary" :disabled="savingModelCollection" @click="saveModelCollection">
            {{ savingModelCollection ? '保存中...' : '保存模型集合' }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="showErrorCollectionEditor"
      :title="editingErrorCollectionId ? '编辑错误集合' : '新建错误集合'"
      width="wide"
      @close="closeErrorCollectionEditor"
    >
      <form class="space-y-4" @submit.prevent="saveErrorCollection">
        <div>
          <label class="input-label">集合名称</label>
          <input v-model.trim="errorCollectionForm.name" type="text" class="input" placeholder="例如：OpenAI Team 错误集合" />
        </div>

        <div>
          <label class="input-label">集合说明</label>
          <textarea v-model.trim="errorCollectionForm.description" rows="4" class="input" placeholder="例如：429 转发 + 失效账号踢出；400 模型不支持则篡改响应" />
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="closeErrorCollectionEditor">取消</button>
          <button type="button" class="btn btn-primary" :disabled="savingErrorCollection" @click="saveErrorCollection">
            {{ savingErrorCollection ? '保存中...' : '保存错误集合' }}
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
      <form class="space-y-5" @submit.prevent="saveRule">
        <section class="rounded-2xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="space-y-1">
              <div class="text-sm font-semibold text-gray-900 dark:text-white">当前错误集合：{{ selectedRuleCollectionName || '未选择' }}</div>
              <div class="text-xs leading-6 text-gray-500 dark:text-gray-400">
                先定义匹配条件，再决定命中后的处置方式。429 自动切号属于全局运行策略，建议在这里补充特例动作，而不是用“踢出号池”替代限流恢复。
              </div>
            </div>
            <div
              v-if="ruleTargets429"
              class="inline-flex rounded-full bg-amber-50 px-3 py-1 text-xs font-medium text-amber-700 dark:bg-amber-900/20 dark:text-amber-200"
            >
              当前规则命中 429
            </div>
          </div>
        </section>

        <section class="rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
          <div class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">1. 基础信息</div>
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

          <div class="mt-4 grid gap-4 md:grid-cols-[auto,minmax(0,1fr)]">
            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="ruleForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span>启用这条规则</span>
            </label>
            <div>
              <label class="input-label">规则说明</label>
              <input v-model.trim="ruleForm.description" type="text" class="input" placeholder="例如：429 时切换到新的正常账号" />
            </div>
          </div>
        </section>

        <section class="rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
          <div class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">2. 匹配条件</div>
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

          <div class="mt-4">
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
        </section>

        <section class="rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
          <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
            <div>
              <div class="text-sm font-semibold text-gray-900 dark:text-white">3. 命中后的动作</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                这里决定规则命中后如何处理账号与响应。
              </div>
            </div>
          </div>

          <div
            v-if="ruleTargets429"
            class="mb-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs leading-6 text-amber-800 dark:border-amber-700/40 dark:bg-amber-900/20 dark:text-amber-200"
          >
            这条规则包含 429。自动切号受“运行设置 > 429 自动转发”控制，转发次数与统一转发次数上限一致。
            <span v-if="ruleForm.action_disable" class="mt-2 block">
              当前同时勾选了“踢出号池”，命中后会把账号直接打成错误态，不会按普通限流那样自动恢复。
            </span>
          </div>

          <div class="grid gap-3 md:grid-cols-2">
            <label
              :class="[
                'flex items-start gap-3 rounded-xl border px-4 py-3 text-sm transition-colors',
                ruleForm.action_disable
                  ? 'border-red-200 bg-red-50 dark:border-red-900/40 dark:bg-red-900/20'
                  : 'border-gray-200 dark:border-dark-700'
              ]"
            >
              <input v-model="ruleForm.action_disable" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span class="space-y-1">
                <span class="block font-medium text-gray-900 dark:text-white">踢出号池</span>
                <span class="block text-xs leading-6 text-gray-500 dark:text-gray-400">把账号标记为错误态并停止调度，这不是限流态，后续不会自动恢复。</span>
              </span>
            </label>
            <label
              :class="[
                'flex items-start gap-3 rounded-xl border px-4 py-3 text-sm transition-colors',
                ruleForm.action_failover
                  ? 'border-primary-200 bg-primary-50 dark:border-primary-900/40 dark:bg-primary-900/20'
                  : 'border-gray-200 dark:border-dark-700'
              ]"
            >
              <input v-model="ruleForm.action_failover" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span class="space-y-1">
                <span class="block font-medium text-gray-900 dark:text-white">转发请求</span>
                <span class="block text-xs leading-6 text-gray-500 dark:text-gray-400">把当前请求继续切到其他正常账号，切换次数沿用统一转发次数上限。</span>
              </span>
            </label>
            <label
              :class="[
                'flex items-start gap-3 rounded-xl border px-4 py-3 text-sm transition-colors',
                ruleForm.action_delete
                  ? 'border-red-200 bg-red-50 dark:border-red-900/40 dark:bg-red-900/20'
                  : 'border-gray-200 dark:border-dark-700'
              ]"
            >
              <input v-model="ruleForm.action_delete" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span class="space-y-1">
                <span class="block font-medium text-gray-900 dark:text-white">删除账号</span>
                <span class="block text-xs leading-6 text-gray-500 dark:text-gray-400">直接删除命中的账号，只适合明确失效且不需要保留的场景。</span>
              </span>
            </label>
            <label
              :class="[
                'flex items-start gap-3 rounded-xl border px-4 py-3 text-sm transition-colors',
                ruleForm.action_override
                  ? 'border-amber-200 bg-amber-50 dark:border-amber-900/40 dark:bg-amber-900/20'
                  : 'border-gray-200 dark:border-dark-700'
              ]"
            >
              <input v-model="ruleForm.action_override" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span class="space-y-1">
                <span class="block font-medium text-gray-900 dark:text-white">篡改响应</span>
                <span class="block text-xs leading-6 text-gray-500 dark:text-gray-400">把返回给用户的状态码或消息改写掉，适合兼容上游的已知异常格式。</span>
              </span>
            </label>
          </div>
        </section>

        <section v-if="ruleForm.action_override" class="rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
          <div class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">4. 响应改写细节</div>
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
              placeholder="返回给用户的错误消息"
            />
          </div>
        </section>

        <section class="rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
          <div class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">5. 排查辅助</div>
          <div>
            <label class="input-label">样例响应</label>
            <textarea
              v-model="ruleForm.sample_response"
              rows="5"
              class="input font-mono text-xs"
              placeholder="可粘贴完整错误响应，方便后续排查"
            />
          </div>
        </section>
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
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import ModelWhitelistSelector from '@/components/account/ModelWhitelistSelector.vue'
import {
  accountRulesAPI,
  type AccountRuleBinding,
  type AccountRuleCatalog,
  type AccountRuleDraft,
  type AccountRuleErrorCollection,
  type AccountRuleErrorRule,
  type AccountRuleModelCollection,
  type AccountRuleObservedBinding,
  type UpsertAccountRuleBindingRequest,
  type UpsertAccountRuleErrorCollectionRequest,
  type UpsertAccountRuleModelCollectionRequest,
  type UpsertAccountRuleRequest
} from '@/api/admin/accountRules'
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
const savingBinding = ref(false)
const savingModelCollection = ref(false)
const savingErrorCollection = ref(false)
const savingRule = ref(false)

const showBindingEditor = ref(false)
const showModelCollectionEditor = ref(false)
const showErrorCollectionEditor = ref(false)
const showRuleEditor = ref(false)

const selectedBindingId = ref<number | null>(null)
const selectedModelCollectionId = ref<number | null>(null)
const selectedErrorCollectionId = ref<number | null>(null)

const editingBindingId = ref<number | null>(null)
const editingModelCollectionId = ref<number | null>(null)
const editingErrorCollectionId = ref<number | null>(null)
const editingRuleId = ref<number | null>(null)
const editingRuleCollectionId = ref<number | null>(null)

const pendingDraft = ref<AccountRuleDraft | null>(null)
const appliedDraftKey = ref('')
const catalog = ref<AccountRuleCatalog | null>(null)
const autoAssignModelCollectionToBinding = ref(false)
const autoAssignErrorCollectionToBinding = ref(false)

const settingsForm = reactive({
  forward_max_attempts: 3,
  failover_on_429: true
})

const bindingForm = reactive({
  platform: '',
  business_type: '',
  enabled: true,
  model_collection_id: '',
  error_collection_id: '',
  description: ''
})

const modelCollectionForm = reactive({
  name: '',
  models: [] as string[],
  description: ''
})

const errorCollectionForm = reactive({
  name: '',
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

const selectedBinding = computed(() => {
  return catalog.value?.bindings.find(binding => binding.id === selectedBindingId.value) ?? null
})

const selectedModelCollection = computed(() => {
  return catalog.value?.model_collections.find(collection => collection.id === selectedModelCollectionId.value) ?? null
})

const selectedErrorCollection = computed(() => {
  return catalog.value?.error_collections.find(collection => collection.id === selectedErrorCollectionId.value) ?? null
})

const selectedRuleCollectionName = computed(() => {
  return resolveErrorCollectionName(editingRuleCollectionId.value)
})

const selectedBindingModelCollection = computed(() => {
  if (!selectedBinding.value?.model_collection_id) return null
  return catalog.value?.model_collections.find(collection => collection.id === selectedBinding.value?.model_collection_id) ?? null
})

const selectedBindingErrorCollection = computed(() => {
  if (!selectedBinding.value?.error_collection_id) return null
  return catalog.value?.error_collections.find(collection => collection.id === selectedBinding.value?.error_collection_id) ?? null
})

const draftKey = computed(() => {
  if (!props.draftSource || !props.draftId) return ''
  return `${props.draftSource}:${props.draftId}`
})

const draftHint = computed(() => {
  if (!props.draftSource || !props.draftId) return ''
  return `正在处理来自 /admin/ops 的错误草稿：${props.draftSource} #${props.draftId}`
})

const configuredBindingKeys = computed(() => {
  return new Set((catalog.value?.bindings ?? []).map(bindingKey))
})

const unconfiguredObservedBindings = computed(() => {
  return (catalog.value?.observed_bindings ?? []).filter(binding => !configuredBindingKeys.value.has(observedBindingKey(binding)))
})

const modelCollectionBindingCounts = computed<Record<number, number>>(() => {
  const counts: Record<number, number> = {}
  for (const binding of catalog.value?.bindings ?? []) {
    if (!binding.model_collection_id) continue
    counts[binding.model_collection_id] = (counts[binding.model_collection_id] || 0) + 1
  }
  return counts
})

const errorCollectionBindingCounts = computed<Record<number, number>>(() => {
  const counts: Record<number, number> = {}
  for (const binding of catalog.value?.bindings ?? []) {
    if (!binding.error_collection_id) continue
    counts[binding.error_collection_id] = (counts[binding.error_collection_id] || 0) + 1
  }
  return counts
})

const enabledBindingCount = computed(() => {
  return (catalog.value?.bindings ?? []).filter(binding => binding.enabled).length
})

const totalRuleCount = computed(() => {
  return (catalog.value?.error_collections ?? []).reduce((count, collection) => count + collection.rules.length, 0)
})

const ruleTargets429 = computed(() => {
  return parseStatusCodes(ruleForm.statusCodesText).includes(429)
})

function bindingKey(binding: Pick<AccountRuleBinding, 'platform' | 'business_type'>): string {
  return `${binding.platform.trim().toLowerCase()}::${binding.business_type.trim().toLowerCase()}`
}

function observedBindingKey(binding: Pick<AccountRuleObservedBinding, 'platform' | 'business_type'>): string {
  return `${binding.platform.trim().toLowerCase()}::${binding.business_type.trim().toLowerCase()}`
}

function draftTargetKey(draft: Pick<AccountRuleDraft, 'platform' | 'business_type'>): string {
  return `${draft.platform.trim().toLowerCase()}::${draft.business_type.trim().toLowerCase()}`
}

function formatPlatformLabel(platform: string): string {
  const normalized = platform.trim().toLowerCase()
  switch (normalized) {
    case 'anthropic':
      return 'Anthropic'
    case 'openai':
      return 'OpenAI'
    case 'gemini':
      return 'Gemini'
    case 'antigravity':
      return 'Antigravity'
    case 'sora':
      return 'Sora'
    default:
      return platform || '-'
  }
}

function formatBusinessTypeLabel(businessType: string): string {
  const normalized = businessType.trim().toLowerCase()
  if (!normalized) return '平台级'

  switch (normalized) {
    case 'plus':
      return 'Plus'
    case 'team':
      return 'Team'
    case 'chatgptpro':
    case 'pro':
      return 'Pro'
    case 'free':
      return 'Free'
    case 'google_one':
      return 'Google One'
    case 'google_one_free':
      return 'Google One Free'
    case 'google_ai_pro':
      return 'Google AI Pro'
    case 'google_ai_ultra':
      return 'Google AI Ultra'
    case 'gcp_standard':
      return 'GCP Standard'
    case 'gcp_enterprise':
      return 'GCP Enterprise'
    case 'aistudio_free':
      return 'AI Studio Free'
    case 'aistudio_paid':
      return 'AI Studio Paid'
    case 'ai_studio':
      return 'AI Studio'
    case 'code_assist':
      return 'Code Assist'
    case 'free-tier':
      return 'Free'
    case 'g1-pro-tier':
      return 'Pro'
    case 'g1-ultra-tier':
      return 'Ultra'
    case 'bedrock':
      return 'Bedrock'
    default:
      return businessType
  }
}

function businessTypeBadgeClass(businessType: string): string {
  if (!businessType.trim()) {
    return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  }
  return 'bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
}

function resolveModelCollectionName(id?: number | null): string {
  if (!id) return ''
  return catalog.value?.model_collections.find(collection => collection.id === id)?.name || ''
}

function resolveErrorCollectionName(id?: number | null): string {
  if (!id) return ''
  return catalog.value?.error_collections.find(collection => collection.id === id)?.name || ''
}

function resetBindingForm() {
  bindingForm.platform = ''
  bindingForm.business_type = ''
  bindingForm.enabled = true
  bindingForm.model_collection_id = ''
  bindingForm.error_collection_id = ''
  bindingForm.description = ''
}

function resetModelCollectionForm() {
  modelCollectionForm.name = ''
  modelCollectionForm.models = []
  modelCollectionForm.description = ''
}

function resetErrorCollectionForm() {
  errorCollectionForm.name = ''
  errorCollectionForm.description = ''
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

function syncSelections() {
  const bindings = catalog.value?.bindings ?? []
  if (bindings.length > 0) {
    if (!selectedBindingId.value || !bindings.some(binding => binding.id === selectedBindingId.value)) {
      selectedBindingId.value = bindings[0].id
    }
  } else {
    selectedBindingId.value = null
  }

  const modelCollections = catalog.value?.model_collections ?? []
  if (modelCollections.length > 0) {
    if (!selectedModelCollectionId.value || !modelCollections.some(collection => collection.id === selectedModelCollectionId.value)) {
      selectedModelCollectionId.value = modelCollections[0].id
    }
  } else {
    selectedModelCollectionId.value = null
  }

  const errorCollections = catalog.value?.error_collections ?? []
  if (errorCollections.length > 0) {
    if (!selectedErrorCollectionId.value || !errorCollections.some(collection => collection.id === selectedErrorCollectionId.value)) {
      selectedErrorCollectionId.value = errorCollections[0].id
    }
  } else {
    selectedErrorCollectionId.value = null
  }
}

async function loadCatalog(preferred?: {
  bindingId?: number | null
  modelCollectionId?: number | null
  errorCollectionId?: number | null
}) {
  loading.value = true
  try {
    const data = await accountRulesAPI.getCatalog()
    catalog.value = data
    settingsForm.forward_max_attempts = data.settings.forward_max_attempts || 3
    settingsForm.failover_on_429 = data.settings.failover_on_429 ?? true
    if (preferred?.bindingId) selectedBindingId.value = preferred.bindingId
    if (preferred?.modelCollectionId) selectedModelCollectionId.value = preferred.modelCollectionId
    if (preferred?.errorCollectionId) selectedErrorCollectionId.value = preferred.errorCollectionId
    syncSelections()
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

    if (draft.matched_error_collection_id) {
      selectedErrorCollectionId.value = draft.matched_error_collection_id
      openCreateRule(draft.rule, draft.matched_error_collection_id)
      pendingDraft.value = null
    } else if (draft.matched_binding_id) {
      selectedBindingId.value = draft.matched_binding_id
      const binding = catalog.value?.bindings.find(item => item.id === draft.matched_binding_id) ?? null
      if (binding) {
        openEditBinding(binding)
      } else {
        openCreateBinding({
          platform: draft.platform,
          business_type: draft.business_type
        })
      }
      appStore.showInfo('已匹配到绑定，但它还没有绑定错误集合，请先绑定错误集合，再继续创建规则。')
    } else {
      openCreateBinding({
        platform: draft.platform,
        business_type: draft.business_type
      })
      appStore.showInfo('没有找到匹配绑定，请先创建绑定并关联错误集合。')
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
      forward_max_attempts: settingsForm.forward_max_attempts,
      failover_on_429: settingsForm.failover_on_429
    })
    settingsForm.forward_max_attempts = saved.forward_max_attempts
    settingsForm.failover_on_429 = saved.failover_on_429
    appStore.showSuccess('规则设置已保存')
    emit('updated')
  } catch (error: any) {
    console.error('[AccountRuleManagerModal] Failed to save settings', error)
    appStore.showError(error?.response?.data?.message || error?.response?.data?.detail || '保存规则设置失败')
  } finally {
    savingSettings.value = false
  }
}

function openCreateBinding(prefill?: Partial<AccountRuleObservedBinding | AccountRuleDraft>) {
  editingBindingId.value = null
  resetBindingForm()
  bindingForm.platform = String(prefill?.platform || '').trim()
  bindingForm.business_type = String(prefill?.business_type || '').trim()
  showBindingEditor.value = true
}

function openEditBinding(binding: AccountRuleBinding) {
  editingBindingId.value = binding.id
  bindingForm.platform = binding.platform
  bindingForm.business_type = binding.business_type
  bindingForm.enabled = binding.enabled
  bindingForm.model_collection_id = binding.model_collection_id ? String(binding.model_collection_id) : ''
  bindingForm.error_collection_id = binding.error_collection_id ? String(binding.error_collection_id) : ''
  bindingForm.description = binding.description || ''
  showBindingEditor.value = true
}

function closeBindingEditor() {
  showBindingEditor.value = false
  editingBindingId.value = null
  resetBindingForm()
}

function parseOptionalId(value: string): number | null {
  const parsed = Number.parseInt(value, 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null
}

async function saveBinding() {
  if (!bindingForm.platform.trim()) {
    appStore.showError('平台不能为空')
    return
  }

  const payload: UpsertAccountRuleBindingRequest = {
    platform: bindingForm.platform.trim(),
    business_type: bindingForm.business_type.trim(),
    enabled: bindingForm.enabled,
    model_collection_id: parseOptionalId(bindingForm.model_collection_id),
    error_collection_id: parseOptionalId(bindingForm.error_collection_id),
    description: bindingForm.description.trim()
  }

  savingBinding.value = true
  try {
    const binding = editingBindingId.value
      ? await accountRulesAPI.updateBinding(editingBindingId.value, payload)
      : await accountRulesAPI.createBinding(payload)

    await loadCatalog({
      bindingId: binding.id,
      modelCollectionId: binding.model_collection_id ?? null,
      errorCollectionId: binding.error_collection_id ?? null
    })
    appStore.showSuccess(editingBindingId.value ? '绑定已更新' : '绑定已创建')
    emit('updated')
    closeBindingEditor()

    if (
      pendingDraft.value &&
      bindingKey(binding) === draftTargetKey(pendingDraft.value) &&
      binding.error_collection_id
    ) {
      openCreateRule(pendingDraft.value.rule, binding.error_collection_id)
      pendingDraft.value = null
    }
  } catch (error: any) {
    console.error('[AccountRuleManagerModal] Failed to save binding', error)
    appStore.showError(error?.response?.data?.message || error?.response?.data?.detail || '保存绑定失败')
  } finally {
    savingBinding.value = false
  }
}

async function removeBinding(binding: AccountRuleBinding) {
  if (!window.confirm(`确定删除绑定 ${formatPlatformLabel(binding.platform)} / ${formatBusinessTypeLabel(binding.business_type)} 吗？`)) {
    return
  }

  try {
    await accountRulesAPI.deleteBinding(binding.id)
    await loadCatalog()
    appStore.showSuccess('绑定已删除')
    emit('updated')
  } catch (error: any) {
    console.error('[AccountRuleManagerModal] Failed to delete binding', error)
    appStore.showError(error?.response?.data?.message || error?.response?.data?.detail || '删除绑定失败')
  }
}

function openCreateModelCollection(assignToBinding = false) {
  autoAssignModelCollectionToBinding.value = assignToBinding
  editingModelCollectionId.value = null
  resetModelCollectionForm()
  showModelCollectionEditor.value = true
}

function openEditModelCollection(collection: AccountRuleModelCollection) {
  editingModelCollectionId.value = collection.id
  modelCollectionForm.name = collection.name
  modelCollectionForm.models = [...(collection.models || [])]
  modelCollectionForm.description = collection.description || ''
  showModelCollectionEditor.value = true
}

function closeModelCollectionEditor() {
  showModelCollectionEditor.value = false
  editingModelCollectionId.value = null
  autoAssignModelCollectionToBinding.value = false
  resetModelCollectionForm()
}

async function saveModelCollection() {
  if (!modelCollectionForm.name.trim()) {
    appStore.showError('模型集合名称不能为空')
    return
  }

  const payload: UpsertAccountRuleModelCollectionRequest = {
    name: modelCollectionForm.name.trim(),
    models: [...modelCollectionForm.models],
    description: modelCollectionForm.description.trim()
  }

  savingModelCollection.value = true
  try {
    const collection = editingModelCollectionId.value
      ? await accountRulesAPI.updateModelCollection(editingModelCollectionId.value, payload)
      : await accountRulesAPI.createModelCollection(payload)

    await loadCatalog({
      modelCollectionId: collection.id,
      bindingId: selectedBindingId.value,
      errorCollectionId: selectedErrorCollectionId.value
    })
    if (autoAssignModelCollectionToBinding.value) {
      bindingForm.model_collection_id = String(collection.id)
    }
    appStore.showSuccess(editingModelCollectionId.value ? '模型集合已更新' : '模型集合已创建')
    emit('updated')
    closeModelCollectionEditor()
  } catch (error: any) {
    console.error('[AccountRuleManagerModal] Failed to save model collection', error)
    appStore.showError(error?.response?.data?.message || error?.response?.data?.detail || '保存模型集合失败')
  } finally {
    savingModelCollection.value = false
  }
}

async function removeModelCollection(collection: AccountRuleModelCollection) {
  if (!window.confirm(`确定删除模型集合「${collection.name}」吗？绑定到它的关系会失去模型集合配置。`)) {
    return
  }

  try {
    await accountRulesAPI.deleteModelCollection(collection.id)
    await loadCatalog({
      bindingId: selectedBindingId.value,
      errorCollectionId: selectedErrorCollectionId.value
    })
    appStore.showSuccess('模型集合已删除')
    emit('updated')
  } catch (error: any) {
    console.error('[AccountRuleManagerModal] Failed to delete model collection', error)
    appStore.showError(error?.response?.data?.message || error?.response?.data?.detail || '删除模型集合失败')
  }
}

function openCreateErrorCollection(assignToBinding = false) {
  autoAssignErrorCollectionToBinding.value = assignToBinding
  editingErrorCollectionId.value = null
  resetErrorCollectionForm()
  showErrorCollectionEditor.value = true
}

function openEditErrorCollection(collection: AccountRuleErrorCollection) {
  editingErrorCollectionId.value = collection.id
  errorCollectionForm.name = collection.name
  errorCollectionForm.description = collection.description || ''
  showErrorCollectionEditor.value = true
}

function closeErrorCollectionEditor() {
  showErrorCollectionEditor.value = false
  editingErrorCollectionId.value = null
  autoAssignErrorCollectionToBinding.value = false
  resetErrorCollectionForm()
}

async function saveErrorCollection() {
  if (!errorCollectionForm.name.trim()) {
    appStore.showError('错误集合名称不能为空')
    return
  }

  const payload: UpsertAccountRuleErrorCollectionRequest = {
    name: errorCollectionForm.name.trim(),
    description: errorCollectionForm.description.trim()
  }

  savingErrorCollection.value = true
  try {
    const collection = editingErrorCollectionId.value
      ? await accountRulesAPI.updateErrorCollection(editingErrorCollectionId.value, payload)
      : await accountRulesAPI.createErrorCollection(payload)

    await loadCatalog({
      errorCollectionId: collection.id,
      bindingId: selectedBindingId.value,
      modelCollectionId: selectedModelCollectionId.value
    })
    if (autoAssignErrorCollectionToBinding.value) {
      bindingForm.error_collection_id = String(collection.id)
    }
    appStore.showSuccess(editingErrorCollectionId.value ? '错误集合已更新' : '错误集合已创建')
    emit('updated')
    closeErrorCollectionEditor()
  } catch (error: any) {
    console.error('[AccountRuleManagerModal] Failed to save error collection', error)
    appStore.showError(error?.response?.data?.message || error?.response?.data?.detail || '保存错误集合失败')
  } finally {
    savingErrorCollection.value = false
  }
}

async function removeErrorCollection(collection: AccountRuleErrorCollection) {
  if (!window.confirm(`确定删除错误集合「${collection.name}」吗？绑定到它的关系会失去错误规则配置。`)) {
    return
  }

  try {
    await accountRulesAPI.deleteErrorCollection(collection.id)
    await loadCatalog({
      bindingId: selectedBindingId.value,
      modelCollectionId: selectedModelCollectionId.value
    })
    appStore.showSuccess('错误集合已删除')
    emit('updated')
  } catch (error: any) {
    console.error('[AccountRuleManagerModal] Failed to delete error collection', error)
    appStore.showError(error?.response?.data?.message || error?.response?.data?.detail || '删除错误集合失败')
  }
}

function openCreateRule(prefill?: Partial<AccountRuleErrorRule> | null, errorCollectionId?: number | null) {
  const resolvedCollectionId = errorCollectionId ?? selectedErrorCollectionId.value
  if (!resolvedCollectionId) {
    appStore.showError('请先选择一个错误集合')
    return
  }
  editingRuleId.value = null
  editingRuleCollectionId.value = resolvedCollectionId
  selectedErrorCollectionId.value = resolvedCollectionId
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
  openCreateRule(rule, rule.error_collection_id)
  editingRuleId.value = rule.id
}

function closeRuleEditor() {
  showRuleEditor.value = false
  editingRuleId.value = null
  editingRuleCollectionId.value = null
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
  if (!editingRuleCollectionId.value) {
    appStore.showError('请先选择一个错误集合')
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
      await accountRulesAPI.createRule(editingRuleCollectionId.value, payload)
    }
    await loadCatalog({
      errorCollectionId: editingRuleCollectionId.value,
      bindingId: selectedBindingId.value,
      modelCollectionId: selectedModelCollectionId.value
    })
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
    await loadCatalog({
      errorCollectionId: rule.error_collection_id,
      bindingId: selectedBindingId.value,
      modelCollectionId: selectedModelCollectionId.value
    })
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
  async show => {
    if (!show) {
      showBindingEditor.value = false
      showModelCollectionEditor.value = false
      showErrorCollectionEditor.value = false
      showRuleEditor.value = false
      editingBindingId.value = null
      editingModelCollectionId.value = null
      editingErrorCollectionId.value = null
      editingRuleId.value = null
      editingRuleCollectionId.value = null
      resetBindingForm()
      resetModelCollectionForm()
      resetErrorCollectionForm()
      resetRuleForm()
      appliedDraftKey.value = ''
      pendingDraft.value = null
      return
    }
    await loadCatalog({
      bindingId: selectedBindingId.value,
      modelCollectionId: selectedModelCollectionId.value,
      errorCollectionId: selectedErrorCollectionId.value
    })
  },
  { immediate: true }
)

watch(
  () => [catalog.value?.bindings, catalog.value?.model_collections, catalog.value?.error_collections],
  () => {
    syncSelections()
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
