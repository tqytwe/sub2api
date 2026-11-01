<template>
  <section
    class="space-y-4 rounded-lg border border-blue-200 bg-blue-50/70 p-4 dark:border-blue-900/60 dark:bg-blue-950/20"
    data-testid="excel-oauth-binding"
  >
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.accounts.openai.excelOAuth.title') }}
        </h3>
        <p class="mt-1 text-xs text-gray-600 dark:text-gray-400">
          {{ t('admin.accounts.openai.excelOAuth.description') }}
        </p>
      </div>
      <span
        v-if="status"
        class="rounded-full px-2.5 py-1 text-xs font-medium"
        :class="status.authorized
          ? 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-200'
          : 'bg-gray-200 text-gray-700 dark:bg-dark-600 dark:text-gray-200'"
        data-testid="excel-oauth-status"
      >
        {{ status.authorized
          ? t('admin.accounts.openai.excelOAuth.connected')
          : t('admin.accounts.openai.excelOAuth.notConnected') }}
      </span>
    </div>

    <div v-if="loadingStatus" class="text-xs text-gray-500 dark:text-gray-400" role="status">
      {{ t('admin.accounts.openai.excelOAuth.loading') }}
    </div>
    <div v-else-if="status?.authorized" class="space-y-1 text-xs text-gray-600 dark:text-gray-300">
      <p v-if="status.expires_at">
        {{ t('admin.accounts.openai.excelOAuth.expiresAt', { date: formatDate(status.expires_at) }) }}
      </p>
      <p v-if="status.updated_at">
        {{ t('admin.accounts.openai.excelOAuth.updatedAt', { date: formatDate(status.updated_at) }) }}
      </p>
      <p>{{ t('admin.accounts.openai.excelOAuth.modelsHint') }}</p>
    </div>

    <div class="flex flex-wrap gap-2">
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="loadingStatus || generating"
        data-testid="excel-oauth-refresh-status"
        @click="loadStatus"
      >
        {{ loadingStatus
          ? t('admin.accounts.openai.excelOAuth.loading')
          : t('admin.accounts.openai.excelOAuth.refreshStatus') }}
      </button>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="generating || loadingStatus || !status"
        data-testid="excel-oauth-start"
        @click="startAuthorization"
      >
        {{ generating
          ? t('admin.accounts.openai.excelOAuth.generating')
          : status?.authorized
            ? t('admin.accounts.openai.excelOAuth.reconnect')
            : t('admin.accounts.openai.excelOAuth.connect') }}
      </button>
      <button
        v-if="status?.authorized"
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="discovering || loadingStatus"
        data-testid="excel-oauth-discover"
        @click="discoverCapabilities"
      >
        {{ discovering
          ? t('admin.accounts.openai.excelOAuth.discovering')
          : t('admin.accounts.openai.excelOAuth.discover') }}
      </button>
    </div>

    <div v-if="authUrl" class="space-y-3 rounded-md border border-blue-200 bg-white p-3 dark:border-blue-900/60 dark:bg-dark-800">
      <p class="text-xs text-gray-600 dark:text-gray-300">
        {{ t('admin.accounts.openai.excelOAuth.authorizationHint') }}
      </p>
      <a
        :href="authUrl"
        target="_blank"
        rel="noopener noreferrer"
        class="inline-flex text-sm font-medium text-primary-600 underline hover:text-primary-700 dark:text-primary-400"
        data-testid="excel-oauth-open-link"
      >
        {{ t('admin.accounts.openai.excelOAuth.openAuthorization') }}
      </a>
      <label class="block">
        <span class="input-label">{{ t('admin.accounts.openai.excelOAuth.callbackLabel') }}</span>
        <textarea
          v-model="callbackInput"
          rows="3"
          class="input font-mono text-xs"
          :placeholder="t('admin.accounts.openai.excelOAuth.callbackPlaceholder')"
          autocomplete="off"
          spellcheck="false"
          data-testid="excel-oauth-callback-input"
        />
      </label>
      <button
        type="button"
        class="btn btn-primary btn-sm"
        :disabled="binding || !callbackInput.trim() || !sessionId || !expectedState"
        data-testid="excel-oauth-bind"
        @click="bindAuthorization"
      >
        {{ binding
          ? t('admin.accounts.openai.excelOAuth.binding')
          : t('admin.accounts.openai.excelOAuth.completeAuthorization') }}
      </button>
    </div>

    <div v-if="capabilities" class="space-y-2" data-testid="excel-oauth-capabilities">
      <p class="text-xs font-medium text-gray-700 dark:text-gray-200">
        {{ capabilities.allowed
          ? t('admin.accounts.openai.excelOAuth.allowedModels', { count: capabilities.models.length })
          : t('admin.accounts.openai.excelOAuth.notAllowed') }}
      </p>
      <ul v-if="capabilities.allowed && capabilities.models.length" class="flex flex-wrap gap-2">
        <li
          v-for="model in capabilities.models"
          :key="model.id"
          class="rounded bg-white px-2 py-1 font-mono text-xs text-gray-700 ring-1 ring-gray-200 dark:bg-dark-800 dark:text-gray-200 dark:ring-dark-600"
        >
          {{ model.display_name || model.id }}
          <span v-if="model.reasoning_efforts?.length" class="ml-1 text-gray-500 dark:text-gray-400">
            ({{ model.reasoning_efforts.join(', ') }})
          </span>
        </li>
      </ul>
      <p v-else-if="capabilities.allowed" class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.accounts.openai.excelOAuth.noModels') }}
      </p>
    </div>

    <p v-if="error" class="text-xs text-red-700 dark:text-red-300" role="alert" data-testid="excel-oauth-error">
      {{ error }}
    </p>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  ExcelBPSCapabilities,
  OpenAIExcelOAuthAuthURL,
  OpenAIExcelOAuthStatus
} from '@/api/admin/accounts'

const props = defineProps<{ accountId: number }>()
const { t } = useI18n()

const status = ref<OpenAIExcelOAuthStatus | null>(null)
const capabilities = ref<ExcelBPSCapabilities | null>(null)
const authUrl = ref('')
const sessionId = ref('')
const expectedState = ref('')
const redirectURI = ref('')
const callbackInput = ref('')
const loadingStatus = ref(false)
const generating = ref(false)
const binding = ref(false)
const discovering = ref(false)
const error = ref('')

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function clearAuthorization(): void {
  authUrl.value = ''
  sessionId.value = ''
  expectedState.value = ''
  redirectURI.value = ''
  callbackInput.value = ''
}

function clearError(): void {
  error.value = ''
}

function apiError(err: unknown, fallback: string): string {
  // Provider and proxy error bodies can contain credential-bearing data.
  // Show the local safe summary instead of echoing a server message.
  void err
  return fallback
}

async function loadStatus(): Promise<void> {
  loadingStatus.value = true
  clearError()
  try {
    status.value = await adminAPI.accounts.getOpenAIExcelOAuthStatus(props.accountId)
    capabilities.value = null
  } catch (err) {
    error.value = apiError(err, t('admin.accounts.openai.excelOAuth.statusFailed'))
  } finally {
    loadingStatus.value = false
  }
}

function parseAuthURL(auth: OpenAIExcelOAuthAuthURL): { state: string; redirectUri: string } {
  const parsed = new URL(auth.auth_url)
  return {
    state: parsed.searchParams.get('state') || '',
    redirectUri: parsed.searchParams.get('redirect_uri') || ''
  }
}

async function startAuthorization(): Promise<void> {
  generating.value = true
  clearError()
  clearAuthorization()
  capabilities.value = null
  let popup: Window | null = null
  try {
    // Open synchronously from the user's click so browser popup blockers do
    // not suppress the provider page while the API request is in flight.
    popup = window.open('about:blank', '_blank')
  } catch {
    // The authorization link below remains available if popups are blocked.
  }
  try {
    const result = await adminAPI.accounts.generateOpenAIExcelOAuthAuthURL(props.accountId)
    const parsed = parseAuthURL(result)
    if (!result.session_id || !parsed.state) {
      throw new Error(t('admin.accounts.openai.excelOAuth.invalidAuthResponse'))
    }
    authUrl.value = result.auth_url
    sessionId.value = result.session_id
    expectedState.value = parsed.state
    redirectURI.value = parsed.redirectUri
    if (popup) {
      popup.opener = null
      popup.location.href = result.auth_url
    }
  } catch (err) {
    if (popup && !popup.closed) popup.close()
    error.value = apiError(err, t('admin.accounts.openai.excelOAuth.authorizationFailed'))
  } finally {
    generating.value = false
  }
}

function parseCallback(value: string): { code: string; state: string } {
  const trimmed = value.trim()
  if (!trimmed) return { code: '', state: '' }
  const looksLikeCallback = trimmed.includes('://') || /(?:^|[?&])(?:code|state)=/.test(trimmed)
  if (!looksLikeCallback) {
    return { code: trimmed, state: expectedState.value }
  }
  try {
    const parsed = trimmed.includes('://')
      ? new URL(trimmed)
      : new URL(`http://localhost/callback?${trimmed.replace(/^\?/, '')}`)
    const code = parsed.searchParams.get('code') || ''
    const callbackState = parsed.searchParams.get('state') || ''
    const providerError = parsed.searchParams.get('error')
    if (providerError) throw new Error(providerError)
    // Pasting the raw authorization code is supported; its state is the one
    // held in memory from the generated URL. Full callback URLs carry state.
    return { code, state: callbackState || expectedState.value }
  } catch (err) {
    if (err instanceof Error && err.message !== 'Invalid URL') throw err
    return { code: trimmed, state: expectedState.value }
  }
}

async function bindAuthorization(): Promise<void> {
  if (!status.value || !sessionId.value || !expectedState.value) return
  clearError()
  let callback: { code: string; state: string }
  try {
    callback = parseCallback(callbackInput.value)
  } catch {
    error.value = t('admin.accounts.openai.excelOAuth.callbackInvalid')
    return
  }
  if (!callback.code) {
    error.value = t('admin.accounts.openai.excelOAuth.callbackInvalid')
    return
  }
  if (!callback.state || callback.state !== expectedState.value) {
    error.value = t('admin.accounts.openai.excelOAuth.stateMismatch')
    return
  }

  binding.value = true
  try {
    status.value = await adminAPI.accounts.bindOpenAIExcelOAuth(props.accountId, {
      session_id: sessionId.value,
      code: callback.code,
      state: callback.state,
      ...(redirectURI.value ? { redirect_uri: redirectURI.value } : {}),
      expected_revision: status.value.revision
    })
    clearAuthorization()
    capabilities.value = null
  } catch (err) {
    const code = typeof err === 'object' && err !== null && 'code' in err
      ? String((err as { code?: unknown }).code)
      : ''
    if (code === 'OPENAI_EXCEL_OAUTH_REVISION_CONFLICT') {
      clearAuthorization()
      await loadStatus()
      error.value = t('admin.accounts.openai.excelOAuth.revisionConflict')
    } else {
      error.value = apiError(err, t('admin.accounts.openai.excelOAuth.authorizationFailed'))
    }
  } finally {
    binding.value = false
  }
}

async function discoverCapabilities(): Promise<void> {
  discovering.value = true
  clearError()
  try {
    capabilities.value = await adminAPI.accounts.discoverOpenAIExcelOAuthCapabilities(props.accountId)
  } catch (err) {
    error.value = apiError(err, t('admin.accounts.openai.excelOAuth.discoveryFailed'))
  } finally {
    discovering.value = false
  }
}

onMounted(() => void loadStatus())
</script>
