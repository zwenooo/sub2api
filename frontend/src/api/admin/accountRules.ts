import { apiClient } from '../client'

export interface AccountRuleErrorRule {
  id: number
  scope_id: number
  name: string
  enabled: boolean
  priority: number
  status_codes: number[]
  keywords: string[]
  match_mode: 'any' | 'all'
  action_disable: boolean
  action_failover: boolean
  action_delete: boolean
  action_override: boolean
  passthrough_code: boolean
  response_code?: number | null
  passthrough_body: boolean
  custom_message?: string | null
  skip_monitoring: boolean
  description: string
  sample_response: string
  created_at?: string
  updated_at?: string
}

export interface AccountRuleScope {
  id: number
  platform: string
  account_type: string
  enabled: boolean
  model_set: string[]
  description: string
  created_at?: string
  updated_at?: string
  rules: AccountRuleErrorRule[]
}

export interface AccountRuleObservedScope {
  platform: string
  account_type: string
  account_count: number
}

export interface AccountRuleSettings {
  forward_max_attempts: number
}

export interface AccountRuleCatalog {
  scopes: AccountRuleScope[]
  observed_scopes: AccountRuleObservedScope[]
  settings: AccountRuleSettings
}

export interface AccountRuleDraft {
  platform: string
  account_type: string
  matched_scope_id?: number | null
  account_id?: number | null
  account_name?: string | null
  rule: AccountRuleErrorRule
}

export interface UpsertAccountRuleScopeRequest {
  platform: string
  account_type: string
  enabled?: boolean
  model_set: string[]
  description: string
}

export interface UpsertAccountRuleRequest {
  name: string
  enabled?: boolean
  priority?: number
  status_codes: number[]
  keywords: string[]
  match_mode: 'any' | 'all'
  action_disable?: boolean
  action_failover?: boolean
  action_delete?: boolean
  action_override?: boolean
  passthrough_code?: boolean
  response_code?: number | null
  passthrough_body?: boolean
  custom_message?: string | null
  skip_monitoring?: boolean
  description: string
  sample_response: string
}

export async function getCatalog(): Promise<AccountRuleCatalog> {
  const { data } = await apiClient.get<AccountRuleCatalog>('/admin/accounts/rules')
  return data
}

export async function updateSettings(payload: AccountRuleSettings): Promise<AccountRuleSettings> {
  const { data } = await apiClient.put<AccountRuleSettings>('/admin/accounts/rules/settings', payload)
  return data
}

export async function createScope(payload: UpsertAccountRuleScopeRequest): Promise<AccountRuleScope> {
  const { data } = await apiClient.post<AccountRuleScope>('/admin/accounts/rules/scopes', payload)
  return data
}

export async function updateScope(id: number, payload: UpsertAccountRuleScopeRequest): Promise<AccountRuleScope> {
  const { data } = await apiClient.put<AccountRuleScope>(`/admin/accounts/rules/scopes/${id}`, payload)
  return data
}

export async function deleteScope(id: number): Promise<void> {
  await apiClient.delete(`/admin/accounts/rules/scopes/${id}`)
}

export async function createRule(scopeId: number, payload: UpsertAccountRuleRequest): Promise<AccountRuleErrorRule> {
  const { data } = await apiClient.post<AccountRuleErrorRule>(`/admin/accounts/rules/scopes/${scopeId}/rules`, payload)
  return data
}

export async function updateRule(id: number, payload: UpsertAccountRuleRequest): Promise<AccountRuleErrorRule> {
  const { data } = await apiClient.put<AccountRuleErrorRule>(`/admin/accounts/rules/rules/${id}`, payload)
  return data
}

export async function deleteRule(id: number): Promise<void> {
  await apiClient.delete(`/admin/accounts/rules/rules/${id}`)
}

export async function getOpsDraft(source: 'request-error' | 'upstream-error', id: number): Promise<AccountRuleDraft> {
  const { data } = await apiClient.get<AccountRuleDraft>('/admin/accounts/rules/drafts/from-ops', {
    params: { source, id }
  })
  return data
}

export const accountRulesAPI = {
  getCatalog,
  updateSettings,
  createScope,
  updateScope,
  deleteScope,
  createRule,
  updateRule,
  deleteRule,
  getOpsDraft
}

export default accountRulesAPI
