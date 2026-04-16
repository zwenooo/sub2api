<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  BarElement,
  BarController,
  CategoryScale,
  Legend,
  LineController,
  LineElement,
  LinearScale,
  PointElement,
  Tooltip
} from 'chart.js'
import type { ChartData } from 'chart.js'
import { Bar } from 'vue-chartjs'
import EmptyState from '@/components/common/EmptyState.vue'
import type { AdminAccountRiskOverview } from '@/types'

ChartJS.register(BarController, BarElement, CategoryScale, Legend, LineController, LineElement, LinearScale, PointElement, Tooltip)

const props = withDefaults(defineProps<{
  overview?: AdminAccountRiskOverview | null
  loading?: boolean
}>(), {
  overview: null,
  loading: false
})

const { t } = useI18n()

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const chartTheme = computed(() => ({
  grid: isDarkMode.value ? 'rgba(71, 85, 105, 0.35)' : 'rgba(148, 163, 184, 0.18)',
  text: isDarkMode.value ? '#94a3b8' : '#64748b',
  line: '#0f172a',
  lineDark: '#e2e8f0'
}))

const riskBucketMeta = computed(() => [
  { key: 'below_50', label: t('admin.accounts.riskOverview.buckets.below_50'), color: '#0f766e' },
  { key: '50_60', label: t('admin.accounts.riskOverview.buckets.50_60'), color: '#0ea5a5' },
  { key: '60_70', label: t('admin.accounts.riskOverview.buckets.60_70'), color: '#22c55e' },
  { key: '70_80', label: t('admin.accounts.riskOverview.buckets.70_80'), color: '#84cc16' },
  { key: '80_90', label: t('admin.accounts.riskOverview.buckets.80_90'), color: '#f59e0b' },
  { key: '90_100', label: t('admin.accounts.riskOverview.buckets.90_100'), color: '#f97316' },
  { key: 'rate_limited', label: t('admin.accounts.riskOverview.buckets.rate_limited'), color: '#dc2626' }
])

const recoveryBucketMeta = computed(() => [
  { key: 'under_30m', label: t('admin.accounts.riskOverview.recoveryBuckets.under_30m') },
  { key: 'under_1h', label: t('admin.accounts.riskOverview.recoveryBuckets.under_1h') },
  { key: 'under_3h', label: t('admin.accounts.riskOverview.recoveryBuckets.under_3h') },
  { key: 'under_6h', label: t('admin.accounts.riskOverview.recoveryBuckets.under_6h') },
  { key: 'under_12h', label: t('admin.accounts.riskOverview.recoveryBuckets.under_12h') },
  { key: 'under_24h', label: t('admin.accounts.riskOverview.recoveryBuckets.under_24h') },
  { key: 'under_3d', label: t('admin.accounts.riskOverview.recoveryBuckets.under_3d') },
  { key: 'under_7d', label: t('admin.accounts.riskOverview.recoveryBuckets.under_7d') },
  { key: 'over_7d', label: t('admin.accounts.riskOverview.recoveryBuckets.over_7d') }
])

const summary = computed(() => props.overview?.summary ?? {
  total_accounts: 0,
  supported_accounts: 0,
  charted_accounts: 0,
  excluded_accounts: 0,
  unknown_accounts: 0,
  high_risk_accounts: 0,
  rate_limited_accounts: 0,
  recovery_tracked_accounts: 0
})

const hasSupportedAccounts = computed(() => summary.value.supported_accounts > 0)
const hasChartedAccounts = computed(() => summary.value.charted_accounts > 0)

const riskBucketValues = computed(() =>
  riskBucketMeta.value.map((meta) => {
    const count = props.overview?.risk_buckets.find((bucket) => bucket.bucket_key === meta.key)?.count ?? 0
    return { ...meta, count }
  })
)

const recoveryBucketValues = computed(() =>
  recoveryBucketMeta.value.map((meta) => {
    const count = props.overview?.recovery_buckets.find((bucket) => bucket.bucket_key === meta.key)?.count ?? 0
    return { ...meta, count }
  })
)

const recoveryCumulativeValues = computed(() => {
  let running = 0
  return recoveryBucketValues.value.map((bucket) => {
    running += bucket.count
    return running
  })
})

const hasRecoveryData = computed(() => recoveryBucketValues.value.some((bucket) => bucket.count > 0))

const statCards = computed(() => [
  {
    label: t('admin.accounts.riskOverview.summary.charted'),
    value: summary.value.charted_accounts,
    tone: 'text-slate-900 dark:text-white',
    accent: 'from-slate-100 to-slate-50 dark:from-dark-700 dark:to-dark-800'
  },
  {
    label: t('admin.accounts.riskOverview.summary.highRisk'),
    value: summary.value.high_risk_accounts,
    tone: 'text-amber-700 dark:text-amber-300',
    accent: 'from-amber-100 to-orange-50 dark:from-amber-900/30 dark:to-orange-900/10'
  },
  {
    label: t('admin.accounts.riskOverview.summary.rateLimited'),
    value: summary.value.rate_limited_accounts,
    tone: 'text-red-700 dark:text-red-300',
    accent: 'from-red-100 to-rose-50 dark:from-red-900/30 dark:to-rose-900/10'
  },
  {
    label: t('admin.accounts.riskOverview.summary.notIncluded'),
    value: summary.value.excluded_accounts + summary.value.unknown_accounts,
    tone: 'text-sky-700 dark:text-sky-300',
    accent: 'from-sky-100 to-cyan-50 dark:from-sky-900/30 dark:to-cyan-900/10'
  }
])

const riskChartData = computed(() => {
  if (!hasChartedAccounts.value) return null
  return {
    labels: riskBucketValues.value.map((bucket) => bucket.label),
    datasets: [
      {
        label: t('admin.accounts.riskOverview.series.accounts'),
        data: riskBucketValues.value.map((bucket) => bucket.count),
        backgroundColor: riskBucketValues.value.map((bucket) => bucket.color),
        borderRadius: 8,
        borderSkipped: false,
        barThickness: 16
      }
    ]
  }
})

const riskChartOptions = computed(() => {
  const theme = chartTheme.value
  return {
    responsive: true,
    maintainAspectRatio: false,
    indexAxis: 'y' as const,
    plugins: {
      legend: { display: false },
      tooltip: {
        callbacks: {
          label: (context: any) => `${t('admin.accounts.riskOverview.series.accounts')}: ${formatNumber(context.raw as number)}`
        }
      }
    },
    scales: {
      x: {
        beginAtZero: true,
        grid: { color: theme.grid, borderDash: [4, 4] },
        ticks: { color: theme.text, precision: 0, font: { size: 11 } }
      },
      y: {
        grid: { display: false },
        ticks: { color: theme.text, font: { size: 11 } }
      }
    }
  }
})

// 注意：这是一个 mixed chart（bar + line 叠加），chart.js v4 运行时原生支持，
// 但 vue-chartjs 的 <Bar> 组件类型签名只接受 ChartData<'bar'>，不允许 dataset 里出现
// type: 'line'。这里通过 `as unknown as ChartData<'bar'>` 显式告诉 TS：我们知道
// 类型上不严丝合缝，但运行时是合法的 mixed chart 数据，由 chart.js 自己分派每个
// dataset 的渲染器。
const recoveryChartData = computed<ChartData<'bar'> | null>(() => {
  if (!hasRecoveryData.value) return null
  return {
    labels: recoveryBucketValues.value.map((bucket) => bucket.label),
    datasets: [
      {
        type: 'bar' as const,
        label: t('admin.accounts.riskOverview.series.recovery'),
        data: recoveryBucketValues.value.map((bucket) => bucket.count),
        backgroundColor: '#fb923c',
        borderRadius: 8,
        borderSkipped: false,
        barPercentage: 0.72,
        yAxisID: 'y'
      },
      {
        type: 'line' as const,
        label: t('admin.accounts.riskOverview.series.cumulative'),
        data: recoveryCumulativeValues.value,
        borderColor: isDarkMode.value ? chartTheme.value.lineDark : chartTheme.value.line,
        backgroundColor: 'transparent',
        borderWidth: 2,
        tension: 0.32,
        pointRadius: 3,
        pointHoverRadius: 4,
        pointBackgroundColor: '#f8fafc',
        pointBorderColor: isDarkMode.value ? chartTheme.value.lineDark : chartTheme.value.line,
        yAxisID: 'y1'
      }
    ]
  } as unknown as ChartData<'bar'>
})

const recoveryChartOptions = computed(() => {
  const theme = chartTheme.value
  return {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: { display: false },
      tooltip: {
        callbacks: {
          label: (context: any) => {
            const label = context.dataset.label as string
            return `${label}: ${formatNumber(context.raw as number)}`
          }
        }
      }
    },
    scales: {
      x: {
        grid: { display: false },
        ticks: {
          color: theme.text,
          font: { size: 10 },
          maxRotation: 0,
          autoSkip: false
        }
      },
      y: {
        beginAtZero: true,
        grid: { color: theme.grid, borderDash: [4, 4] },
        ticks: { color: theme.text, precision: 0, font: { size: 10 } }
      },
      y1: {
        beginAtZero: true,
        position: 'right' as const,
        grid: { display: false },
        ticks: { color: theme.text, precision: 0, font: { size: 10 } }
      }
    }
  }
})

const formatNumber = (value: number) => value.toLocaleString()
</script>

<template>
  <section
    class="overflow-hidden rounded-[28px] border border-slate-200/80 bg-[radial-gradient(circle_at_top_left,_rgba(251,191,36,0.14),_transparent_34%),radial-gradient(circle_at_bottom_right,_rgba(59,130,246,0.10),_transparent_28%),linear-gradient(135deg,rgba(255,255,255,0.98),rgba(248,250,252,0.96))] p-5 shadow-sm dark:border-dark-700 dark:bg-[radial-gradient(circle_at_top_left,_rgba(245,158,11,0.16),_transparent_34%),radial-gradient(circle_at_bottom_right,_rgba(37,99,235,0.12),_transparent_28%),linear-gradient(135deg,rgba(15,23,42,0.98),rgba(17,24,39,0.96))]"
  >
    <div class="grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-start">
      <div class="max-w-2xl">
        <p class="text-[11px] font-semibold uppercase tracking-[0.26em] text-amber-600 dark:text-amber-300">
          {{ t('admin.accounts.riskOverview.eyebrow') }}
        </p>
        <h3 class="mt-1.5 text-lg font-semibold text-slate-900 dark:text-white">
          {{ t('admin.accounts.riskOverview.title') }}
        </h3>
        <p class="mt-1 text-sm leading-5 text-slate-600 dark:text-slate-300">
          {{ t('admin.accounts.riskOverview.description') }}
        </p>
        <div class="mt-2 flex flex-wrap items-center gap-2 text-[11px] text-slate-500 dark:text-slate-400">
          <span class="rounded-full bg-white/80 px-3 py-1 ring-1 ring-slate-200/80 dark:bg-dark-800/80 dark:ring-dark-600">
            {{ t('admin.accounts.riskOverview.coverage', { charted: formatNumber(summary.charted_accounts), supported: formatNumber(summary.supported_accounts) }) }}
          </span>
          <span class="rounded-full bg-white/80 px-3 py-1 ring-1 ring-slate-200/80 dark:bg-dark-800/80 dark:ring-dark-600">
            {{ t('admin.accounts.riskOverview.excluded', { count: formatNumber(summary.excluded_accounts) }) }}
          </span>
          <span class="rounded-full bg-white/80 px-3 py-1 ring-1 ring-slate-200/80 dark:bg-dark-800/80 dark:ring-dark-600">
            {{ t('admin.accounts.riskOverview.unknown', { count: formatNumber(summary.unknown_accounts) }) }}
          </span>
        </div>
      </div>

      <div class="grid grid-cols-2 gap-2 sm:grid-cols-4 lg:min-w-[440px]">
        <div
          v-for="card in statCards"
          :key="card.label"
          :class="[
            'flex min-w-0 items-center justify-between gap-2 rounded-2xl border border-white/60 bg-gradient-to-br px-3 py-2 shadow-sm ring-1 ring-black/5 dark:border-white/5 dark:ring-white/5',
            card.accent
          ]"
        >
          <p class="truncate text-[10px] font-medium uppercase tracking-[0.14em] text-slate-500 dark:text-slate-400">
            {{ card.label }}
          </p>
          <p :class="['shrink-0 text-base font-semibold', card.tone]">
            {{ formatNumber(card.value) }}
          </p>
        </div>
      </div>
    </div>

    <div v-if="loading" class="mt-4 grid grid-cols-1 gap-3 lg:grid-cols-2">
      <div class="h-[228px] animate-pulse rounded-3xl bg-slate-100/90 dark:bg-dark-700/60"></div>
      <div class="h-[228px] animate-pulse rounded-3xl bg-slate-100/90 dark:bg-dark-700/60"></div>
    </div>

    <div v-else-if="!hasSupportedAccounts" class="mt-4 rounded-3xl border border-dashed border-slate-300/80 bg-white/70 p-6 dark:border-dark-600 dark:bg-dark-800/70">
      <EmptyState
        :title="t('admin.accounts.riskOverview.noSupportedTitle')"
        :description="t('admin.accounts.riskOverview.noSupportedDescription')"
      />
    </div>

    <div v-else-if="!hasChartedAccounts" class="mt-4 rounded-3xl border border-dashed border-slate-300/80 bg-white/70 p-6 dark:border-dark-600 dark:bg-dark-800/70">
      <EmptyState
        :title="t('admin.accounts.riskOverview.noChartedTitle')"
        :description="t('admin.accounts.riskOverview.noChartedDescription')"
      />
    </div>

    <div v-else class="mt-4 grid grid-cols-1 gap-3 lg:grid-cols-2">
      <div class="rounded-3xl border border-slate-200/80 bg-white/85 p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800/80">
        <div class="mb-2">
          <h4 class="text-sm font-semibold text-slate-900 dark:text-white">
            {{ t('admin.accounts.riskOverview.riskChartTitle') }}
          </h4>
          <p class="mt-1 text-[11px] leading-4 text-slate-500 dark:text-slate-400">
            {{ t('admin.accounts.riskOverview.riskChartDescription') }}
          </p>
        </div>
        <div class="h-[192px] lg:h-[200px]">
          <Bar v-if="riskChartData" :data="riskChartData" :options="riskChartOptions" />
        </div>
      </div>

      <div class="rounded-3xl border border-slate-200/80 bg-white/85 p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800/80">
        <div class="mb-2">
          <h4 class="text-sm font-semibold text-slate-900 dark:text-white">
            {{ t('admin.accounts.riskOverview.recoveryChartTitle') }}
          </h4>
          <p class="mt-1 text-[11px] leading-4 text-slate-500 dark:text-slate-400">
            {{ t('admin.accounts.riskOverview.recoveryChartDescription') }}
          </p>
        </div>
        <div v-if="recoveryChartData" class="h-[192px] lg:h-[200px]">
          <Bar :data="recoveryChartData" :options="recoveryChartOptions" />
        </div>
        <div v-else class="flex h-[192px] lg:h-[200px] items-center justify-center">
          <EmptyState
            :title="t('admin.accounts.riskOverview.noRecoveryTitle')"
            :description="t('admin.accounts.riskOverview.noRecoveryDescription')"
          />
        </div>
      </div>
    </div>

    <p class="mt-3 text-[11px] leading-4 text-slate-500 dark:text-slate-400">
      {{ t('admin.accounts.riskOverview.footnote') }}
    </p>
  </section>
</template>
