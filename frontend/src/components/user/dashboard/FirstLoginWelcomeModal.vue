<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores'
import keysAPI from '@/api/keys'
import userGroupsAPI from '@/api/groups'
import playAPI, { type PlayHubGrowth, type PlayHubImageStudio } from '@/api/play'
import { isFeatureFlagEnabled, FeatureFlags } from '@/utils/featureFlags'
import { buildGatewayUrl } from '@/api/url'
import {
  consumeFirstLoginWelcomePending,
  isFirstLoginWelcomeDone,
  markFirstLoginWelcomeDone,
} from '@/utils/firstLoginWelcome'
import type { ApiKey, Group } from '@/types'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const show = ref(false)
const step = ref<1 | 2 | 3>(1)
const keyName = ref('')
const groups = ref<Group[]>([])
const selectedGroupId = ref<number | null>(null)
const createdKey = ref<ApiKey | null>(null)
const creating = ref(false)
const growth = ref<PlayHubGrowth | null>(null)
const imageStudio = ref<PlayHubImageStudio | null>(null)

const showStudioFirst = computed(
  () =>
    isFeatureFlagEnabled(FeatureFlags.imageStudio) &&
    imageStudio.value?.enabled &&
    !imageStudio.value.has_completed_job,
)

const curlExample = computed(() => {
  const apiKey = createdKey.value?.key || 'YOUR_API_KEY'
  const endpoint = buildGatewayUrl('/v1/chat/completions')
  return `curl ${endpoint} \\
  -H "Authorization: Bearer ${apiKey}" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}'`
})

const rechargeHint = computed(() => {
  const g = growth.value
  if (!g?.payment_enabled || !g.first_recharge_eligible) {
    return t('dashboard.firstLoginWelcome.rechargeGeneric')
  }
  const mult = g.recharge_multiplier > 0 ? g.recharge_multiplier : 1
  const lines = [t('dashboard.firstLoginWelcome.rechargeBonus', {
    example: '10',
    credited: (10 * mult).toFixed(2),
  })]
  const bonusPct = g.campaign_recharge_bonus_pct ?? 0
  if (bonusPct > 0) {
    lines.push(t('dashboard.firstLoginWelcome.rechargeBonusCampaign', { bonusPct }))
  }
  return lines.join(' ')
})

const stepTitle = computed(() => {
  if (step.value === 1) {
    return showStudioFirst.value
      ? t('dashboard.firstLoginWelcome.step1TitleStudio')
      : t('dashboard.firstLoginWelcome.step1Title')
  }
  if (step.value === 2) return t('dashboard.firstLoginWelcome.step2Title')
  return t('dashboard.firstLoginWelcome.step3Title')
})

async function loadContext() {
  const [groupList, hub] = await Promise.all([
    userGroupsAPI.getAvailable().catch(() => [] as Group[]),
    playAPI.getPlayHub().catch(() => null),
  ])
  groups.value = groupList
  selectedGroupId.value = groupList[0]?.id ?? null
  growth.value = hub?.growth ?? null
  imageStudio.value = hub?.image_studio ?? null
  keyName.value = t('dashboard.firstLoginWelcome.defaultKeyName')
}

function openIfNeeded() {
  const userId = authStore.user?.id
  if (!userId || authStore.user?.role === 'admin' || authStore.isSimpleMode) return
  if (isFirstLoginWelcomeDone(userId)) return
  if (!consumeFirstLoginWelcomePending()) return
  show.value = true
  step.value = 1
  void loadContext()
}

function finish() {
  const userId = authStore.user?.id
  if (userId) markFirstLoginWelcomeDone(userId)
  show.value = false
}

function dismiss() {
  finish()
}

async function createKey() {
  const name = keyName.value.trim()
  if (!name) {
    appStore.showWarning(t('dashboard.firstLoginWelcome.keyNameRequired'))
    return
  }
  creating.value = true
  try {
    createdKey.value = await keysAPI.create(name, selectedGroupId.value ?? undefined)
    step.value = 2
  } catch (error: unknown) {
    const detail =
      (error as { response?: { data?: { message?: string; detail?: string } } })?.response?.data
        ?.message ||
      (error as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    appStore.showError(detail || t('keys.failedToSave'))
  } finally {
    creating.value = false
  }
}

function skipKeyStep() {
  step.value = 2
}

function goRecharge() {
  finish()
  router.push('/purchase')
}

function goImageStudio() {
  finish()
  router.push('/image-studio')
}

async function copyCurl() {
  await copyToClipboard(curlExample.value, t('dashboard.firstLoginWelcome.curlCopied'))
}

watch(
  () => authStore.user?.id,
  () => openIfNeeded(),
)

onMounted(() => {
  openIfNeeded()
})
</script>

<template>
  <BaseDialog
    :show="show"
    :title="stepTitle"
    width="wide"
    :close-on-click-outside="false"
    @close="dismiss"
  >
    <div class="space-y-5">
      <div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wide text-gray-400">
        <span
          v-for="n in 3"
          :key="n"
          class="h-1.5 flex-1 rounded-full"
          :class="n <= step ? 'bg-primary-500' : 'bg-gray-200 dark:bg-dark-700'"
        />
      </div>

      <template v-if="step === 1">
        <p v-if="showStudioFirst" class="rounded-xl border border-[var(--gw-line)] bg-[var(--gw-paper)] px-4 py-3 text-sm text-gray-700 dark:text-dark-200">
          {{ t('dashboard.firstLoginWelcome.studioHint') }}
        </p>
        <p class="text-sm text-gray-600 dark:text-dark-300">
          {{ showStudioFirst ? t('dashboard.firstLoginWelcome.step1DescStudio') : t('dashboard.firstLoginWelcome.step1Desc') }}
        </p>
        <div>
          <label class="input-label">{{ t('keys.nameLabel') }}</label>
          <input
            v-model="keyName"
            type="text"
            class="input mt-1"
            :placeholder="t('dashboard.firstLoginWelcome.defaultKeyName')"
          />
        </div>
        <div v-if="groups.length > 0">
          <label class="input-label">{{ t('keys.group') }}</label>
          <select v-model="selectedGroupId" class="input mt-1">
            <option v-for="group in groups" :key="group.id" :value="group.id">
              {{ group.name }}
            </option>
          </select>
        </div>
      </template>

      <template v-else-if="step === 2">
        <p class="text-sm text-gray-600 dark:text-dark-300">
          {{ t('dashboard.firstLoginWelcome.step2Desc') }}
        </p>
        <pre
          class="overflow-x-auto rounded-xl bg-gray-950 p-4 text-xs leading-relaxed text-emerald-100"
        >{{ curlExample }}</pre>
        <button
          type="button"
          class="text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"
          @click="copyCurl"
        >
          {{ t('common.copy') }}
        </button>
      </template>

      <template v-else>
        <p class="text-sm text-gray-600 dark:text-dark-300">
          {{ rechargeHint }}
        </p>
        <p class="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200">
          {{ t('dashboard.firstLoginWelcome.step3Desc') }}
        </p>
      </template>
    </div>

    <template #footer>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <button
          type="button"
          class="text-sm text-gray-500 hover:text-gray-700 dark:text-dark-400 dark:hover:text-dark-200"
          @click="dismiss"
        >
          {{ t('dashboard.firstLoginWelcome.later') }}
        </button>
        <div class="flex flex-wrap gap-2">
          <template v-if="step === 1">
            <button v-if="showStudioFirst" type="button" class="btn btn-primary" @click="goImageStudio">
              {{ t('dashboard.firstLoginWelcome.studioCta') }}
            </button>
            <button type="button" class="btn btn-secondary" @click="skipKeyStep">
              {{ t('dashboard.firstLoginWelcome.skipKey') }}
            </button>
            <button type="button" class="btn btn-primary" :disabled="creating" @click="createKey">
              {{ creating ? t('common.loading') : t('dashboard.firstLoginWelcome.createKey') }}
            </button>
          </template>
          <template v-else-if="step === 2">
            <button type="button" class="btn btn-primary" @click="step = 3">
              {{ t('dashboard.firstLoginWelcome.next') }}
            </button>
          </template>
          <template v-else>
            <button
              v-if="growth?.payment_enabled && growth?.first_recharge_eligible"
              type="button"
              class="btn btn-primary"
              @click="goRecharge"
            >
              {{ t('dashboard.growth.rechargeCta') }}
            </button>
            <button type="button" class="btn btn-secondary" @click="finish">
              {{ t('dashboard.firstLoginWelcome.done') }}
            </button>
          </template>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>
