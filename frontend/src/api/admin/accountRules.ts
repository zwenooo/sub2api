import { apiClient } from '../client'

export interface AccountRuleBinding {
  id: number
  platform: string
  business_type: string
  enabled: boolean
  model_collection_id?: number | null
  error_collection_id?: number | null
  description: string
  created_at?: string
  updated_at?: string
}

export interface AccountRuleModelCollection {
  id: number
  name: string
  models: string[]
  description: string
  created_at?: string
  updated_at?: string
}

export interface AccountRuleErrorRule {
  id: number
  error_collection_id: number
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

export interface AccountRuleErrorCollection {
  id: number
  name: string
  description: string
  created_at?: string
  updated_at?: string
  rules: AccountRuleErrorRule[]
}

export interface AccountRuleObservedBinding {
  platform: string
  business_type: string
  account_count: number
}

export interface AccountRuleSettings {
  failover_on_429: boolean
  forward_max_attempts: number
}

export interface AccountRuleCatalog {
  bindings: AccountRuleBinding[]
  model_collections: AccountRuleModelCollection[]
  error_collections: AccountRuleErrorCollection[]
  observed_bindings: AccountRuleObservedBinding[]
  settings: AccountRuleSettings
}

export interface AccountRuleDraft {
  platform: string
  business_type: string
  matched_binding_id?: number | null
  matched_error_collection_id?: number | null
  account_id?: number | null
  account_name?: string | null
  rule: AccountRuleErrorRule
}

export interface UpsertAccountRuleBindingRequest {
  platform: string
  business_type: string
  enabled?: boolean
  model_collection_id?: number | null
  error_collection_id?: number | null
  description: string
}

export interface UpsertAccountRuleModelCollectionRequest {
  name: string
  models: string[]
  description: string
}

export interface UpsertAccountRuleErrorCollectionRequest {
  name: string
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

export async function createBinding(payload: UpsertAccountRuleBindingRequest): Promise<AccountRuleBinding> {
  const { data } = await apiClient.post<AccountRuleBinding>('/admin/accounts/rules/bindings', payload)
  return data
}

export async function updateBinding(id: number, payload: UpsertAccountRuleBindingRequest): Promise<AccountRuleBinding> {
  const { data } = await apiClient.put<AccountRuleBinding>(`/admin/accounts/rules/bindings/${id}`, payload)
  return data
}

export async function deleteBinding(id: number): Promise<void> {
  await apiClient.delete(`/admin/accounts/rules/bindings/${id}`)
}

export async function createModelCollection(payload: UpsertAccountRuleModelCollectionRequest): Promise<AccountRuleModelCollection> {
  const { data } = await apiClient.post<AccountRuleModelCollection>('/admin/accounts/rules/model-collections', payload)
  return data
}

export async function updateModelCollection(id: number, payload: UpsertAccountRuleModelCollectionRequest): Promise<AccountRuleModelCollection> {
  const { data } = await apiClient.put<AccountRuleModelCollection>(`/admin/accounts/rules/model-collections/${id}`, payload)
  return data
}

export async function deleteModelCollection(id: number): Promise<void> {
  await apiClient.delete(`/admin/accounts/rules/model-collections/${id}`)
}

export async function createErrorCollection(payload: UpsertAccountRuleErrorCollectionRequest): Promise<AccountRuleErrorCollection> {
  const { data } = await apiClient.post<AccountRuleErrorCollection>('/admin/accounts/rules/error-collections', payload)
  return data
}

export async function updateErrorCollection(id: number, payload: UpsertAccountRuleErrorCollectionRequest): Promise<AccountRuleErrorCollection> {
  const { data } = await apiClient.put<AccountRuleErrorCollection>(`/admin/accounts/rules/error-collections/${id}`, payload)
  return data
}

export async function deleteErrorCollection(id: number): Promise<void> {
  await apiClient.delete(`/admin/accounts/rules/error-collections/${id}`)
}

export async function createRule(errorCollectionId: number, payload: UpsertAccountRuleRequest): Promise<AccountRuleErrorRule> {
  const { data } = await apiClient.post<AccountRuleErrorRule>(`/admin/accounts/rules/error-collections/${errorCollectionId}/rules`, payload)
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
  createBinding,
  updateBinding,
  deleteBinding,
  createModelCollection,
  updateModelCollection,
  deleteModelCollection,
  createErrorCollection,
  updateErrorCollection,
  deleteErrorCollection,
  createRule,
  updateRule,
  deleteRule,
  getOpsDraft
}

export default accountRulesAPI
