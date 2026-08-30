/**
 * Subscription Store
 * Global state management for user subscriptions with caching and deduplication
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import subscriptionsAPI from '@/api/subscriptions'
import type { SubscriptionProgressEntry, UserSubscription } from '@/types'

// Cache TTL: 60 seconds
const CACHE_TTL_MS = 60_000

// Request generation counter to invalidate stale in-flight responses
let requestGeneration = 0

export const useSubscriptionStore = defineStore('subscriptions', () => {
  // State
  const activeSubscriptions = ref<UserSubscription[]>([])
  const activeSubscriptionProgress = ref<SubscriptionProgressEntry[]>([])
  const loading = ref(false)
  const progressLoading = ref(false)
  const loaded = ref(false)
  const progressLoaded = ref(false)
  const lastFetchedAt = ref<number | null>(null)
  const progressLastFetchedAt = ref<number | null>(null)

  // In-flight request deduplication
  let activePromise: Promise<UserSubscription[]> | null = null
  let progressPromise: Promise<SubscriptionProgressEntry[]> | null = null
  let progressRequestGeneration = 0

  // Auto-refresh interval
  let pollerInterval: ReturnType<typeof setInterval> | null = null

  // Computed
  const hasActiveSubscriptions = computed(() => activeSubscriptions.value.length > 0)
  const hasActiveSubscriptionProgress = computed(() => activeSubscriptionProgress.value.length > 0)

  /**
   * Fetch active subscriptions with caching and deduplication
   * @param force - Force refresh even if cache is valid
   */
  async function fetchActiveSubscriptions(force = false): Promise<UserSubscription[]> {
    const now = Date.now()

    // Return cached data if valid
    if (
      !force &&
      loaded.value &&
      lastFetchedAt.value &&
      now - lastFetchedAt.value < CACHE_TTL_MS
    ) {
      return activeSubscriptions.value
    }

    // Return in-flight request if exists (deduplication)
    if (activePromise && !force) {
      return activePromise
    }

    const currentGeneration = ++requestGeneration

    // Start new request
    loading.value = true
    const requestPromise = subscriptionsAPI
      .getActiveSubscriptions()
      .then((data) => {
        if (currentGeneration === requestGeneration) {
          activeSubscriptions.value = data
          loaded.value = true
          lastFetchedAt.value = Date.now()
        }
        return data
      })
      .catch((error) => {
        console.error('Failed to fetch active subscriptions:', error)
        throw error
      })
      .finally(() => {
        if (activePromise === requestPromise) {
          loading.value = false
          activePromise = null
        }
      })

    activePromise = requestPromise

    return activePromise
  }

  /**
   * Fetch normalized progress for active subscriptions. This is intentionally
   * separate from the legacy subscription list because the progress endpoint
   * owns current quota and reset-window values.
   */
  async function fetchSubscriptionProgress(force = false): Promise<SubscriptionProgressEntry[]> {
    const now = Date.now()

    if (
      !force &&
      progressLoaded.value &&
      progressLastFetchedAt.value &&
      now - progressLastFetchedAt.value < CACHE_TTL_MS
    ) {
      return activeSubscriptionProgress.value
    }

    if (progressPromise && !force) {
      return progressPromise
    }

    const currentGeneration = ++progressRequestGeneration
    progressLoading.value = true
    const requestPromise = subscriptionsAPI
      .getSubscriptionsProgress()
      .then((data) => {
        if (currentGeneration === progressRequestGeneration) {
          activeSubscriptionProgress.value = data
          progressLoaded.value = true
          progressLastFetchedAt.value = Date.now()
        }
        return data
      })
      .catch((error) => {
        console.error('Failed to fetch subscription progress:', error)
        throw error
      })
      .finally(() => {
        if (progressPromise === requestPromise) {
          progressLoading.value = false
          progressPromise = null
        }
      })

    progressPromise = requestPromise
    return progressPromise
  }

  /**
   * Refresh the two active-subscription representations together. Callers
   * that need an immediate post-purchase update must use this instead of the
   * legacy metadata-only fetch.
   */
  async function refreshActiveSubscriptionState(force = false): Promise<void> {
    await Promise.all([
      fetchActiveSubscriptions(force),
      fetchSubscriptionProgress(force),
    ])
  }

  /**
   * Start auto-refresh polling 
   */
  function startPolling() {
    if (pollerInterval) return

    pollerInterval = setInterval(() => {
      refreshActiveSubscriptionState(true).catch((error) => {
        console.error('Subscription polling failed:', error)
      })
    }, 5 * 60 * 1000)
  }

  /**
   * Stop auto-refresh polling
   */
  function stopPolling() {
    if (pollerInterval) {
      clearInterval(pollerInterval)
      pollerInterval = null
    }
  }

  /**
   * Clear all subscription data and stop polling
   */
  function clear() {
    requestGeneration++
    progressRequestGeneration++
    activePromise = null
    progressPromise = null
    activeSubscriptions.value = []
    activeSubscriptionProgress.value = []
    loading.value = false
    progressLoading.value = false
    loaded.value = false
    progressLoaded.value = false
    lastFetchedAt.value = null
    progressLastFetchedAt.value = null
    stopPolling()
  }

  /**
   * Invalidate cache (force next fetch to reload)
   */
  function invalidateCache() {
    lastFetchedAt.value = null
    progressLastFetchedAt.value = null
  }

  return {
    // State
    activeSubscriptions,
    activeSubscriptionProgress,
    loading,
    progressLoading,
    hasActiveSubscriptions,
    hasActiveSubscriptionProgress,

    // Actions
    fetchActiveSubscriptions,
    fetchSubscriptionProgress,
    refreshActiveSubscriptionState,
    startPolling,
    stopPolling,
    clear,
    invalidateCache
  }
})
