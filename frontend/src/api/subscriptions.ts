/**
 * User Subscription API
 * API for regular users to view their own subscriptions and progress
 */

import { apiClient } from './client'
import type { SubscriptionProgress, SubscriptionProgressEntry, SubscriptionUsageWindow, UserSubscription } from '@/types'

type UnknownRecord = Record<string, unknown>

function asRecord(value: unknown): UnknownRecord | null {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as UnknownRecord
    : null
}

function numberOrNull(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function stringOrNull(value: unknown): string | null {
  return typeof value === 'string' && value.trim() ? value : null
}

function normalizeUsageWindow(value: unknown): SubscriptionUsageWindow | null {
  const wire = asRecord(value)
  if (!wire) return null

  const limitUsd = numberOrNull(wire.limit_usd ?? wire.limit)
  const usedUsd = numberOrNull(wire.used_usd ?? wire.used) ?? 0
  const remainingUsd = numberOrNull(wire.remaining_usd) ?? (limitUsd === null ? null : Math.max(limitUsd - usedUsd, 0))
  const percentage = numberOrNull(wire.percentage) ?? (limitUsd && limitUsd > 0 ? (usedUsd / limitUsd) * 100 : 0)

  return {
    limitUsd,
    usedUsd,
    remainingUsd,
    percentage: Math.min(Math.max(percentage, 0), 100),
    windowStart: stringOrNull(wire.window_start),
    resetsAt: stringOrNull(wire.resets_at),
    resetsInSeconds: numberOrNull(wire.resets_in_seconds ?? wire.reset_in_seconds),
  }
}

/**
 * The backend owns its response shape. Normalize current and retired field
 * names once at the API boundary so views only consume this stable model.
 */
export function normalizeSubscriptionProgress(value: unknown): SubscriptionProgress {
  const wire = asRecord(value) ?? {}
  return {
    id: numberOrNull(wire.id ?? wire.subscription_id) ?? 0,
    groupName: stringOrNull(wire.group_name) ?? '',
    expiresAt: stringOrNull(wire.expires_at),
    expiresInDays: numberOrNull(wire.expires_in_days ?? wire.days_remaining),
    daily: normalizeUsageWindow(wire.daily),
    weekly: normalizeUsageWindow(wire.weekly),
    monthly: normalizeUsageWindow(wire.monthly),
  }
}

/**
 * The progress endpoint only returns active subscriptions. Keep inactive
 * records visible in subscription management without falling back to retired
 * raw usage fields.
 */
export function createSubscriptionProgressFallback(subscription: UserSubscription): SubscriptionProgress {
  return {
    id: subscription.id,
    groupName: subscription.group?.name ?? '',
    expiresAt: subscription.expires_at,
    expiresInDays: null,
    daily: null,
    weekly: null,
    monthly: null,
  }
}

function normalizeSubscriptionProgressEntry(value: unknown): SubscriptionProgressEntry {
  const wire = asRecord(value) ?? {}
  const subscription = asRecord(wire.subscription) as UserSubscription | null
  const progress = normalizeSubscriptionProgress(wire.progress ?? wire)
  return { subscription, progress }
}

/**
 * Subscription summary for user dashboard
 */
export interface SubscriptionSummary {
  active_count: number
  subscriptions: Array<{
    id: number
    group_name: string
    status: string
    daily_progress: number | null
    weekly_progress: number | null
    monthly_progress: number | null
    expires_at: string | null
    days_remaining: number | null
  }>
}

/**
 * Get list of current user's subscriptions
 */
export async function getMySubscriptions(): Promise<UserSubscription[]> {
  const response = await apiClient.get<UserSubscription[]>('/subscriptions')
  return response.data
}

/**
 * Get current user's active subscriptions
 */
export async function getActiveSubscriptions(): Promise<UserSubscription[]> {
  const response = await apiClient.get<UserSubscription[]>('/subscriptions/active')
  return response.data
}

/**
 * Get progress for all user's active subscriptions
 */
export async function getSubscriptionsProgress(): Promise<SubscriptionProgressEntry[]> {
  const response = await apiClient.get<unknown>('/subscriptions/progress')
  const rows = Array.isArray(response.data) ? response.data : []
  return rows.map(normalizeSubscriptionProgressEntry)
}

/**
 * Get subscription summary for dashboard display
 */
export async function getSubscriptionSummary(): Promise<SubscriptionSummary> {
  const response = await apiClient.get<SubscriptionSummary>('/subscriptions/summary')
  return response.data
}

/**
 * Get progress for a specific subscription
 */
export async function getSubscriptionProgress(
  subscriptionId: number
): Promise<SubscriptionProgress> {
  const response = await apiClient.get<unknown>(`/subscriptions/${subscriptionId}/progress`)
  return normalizeSubscriptionProgress(response.data)
}

export default {
  getMySubscriptions,
  getActiveSubscriptions,
  getSubscriptionsProgress,
  getSubscriptionSummary,
  getSubscriptionProgress
}
