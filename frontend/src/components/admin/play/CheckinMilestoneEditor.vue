<template>
  <section class="card">
    <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ text('签到里程碑奖励', 'Check-in streak milestones') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ text('设置连续签到达到指定天数时的额外奖励', 'Set bonus rewards for reaching specific consecutive check-in days') }}</p>
      </div>
    </div>

    <!-- design-governance-allow: continuous-motion - this transient loading indicator is removed when the settings request settles. -->
    <div v-if="loading && !settings" class="flex min-h-36 items-center justify-center"><Icon name="refresh" size="lg" class="animate-spin text-gray-400" /></div>

    <div v-else-if="settings" class="space-y-4 p-5">
      <!-- 补签功能开关 -->
      <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-sm font-medium text-gray-900 dark:text-white">{{ text('补签功能', 'Make-up check-in') }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ text('允许用户补签昨日遗漏的签到（需前日已签到且24小时内有充值）', 'Allow users to make up yesterday\'s missed check-in (requires prior day check-in and recharge within 24 hours)') }}</p>
          </div>
          <Toggle v-model="settings.makeup_enabled" :disabled="saving" />
        </div>
      </div>

      <!-- 里程碑表格 -->
      <div class="overflow-x-auto rounded border border-gray-200 dark:border-dark-700">
        <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800">
            <tr>
              <th class="px-4 py-3">{{ text('连签天数', 'Streak days') }}</th>
              <th class="px-4 py-3">{{ text('额外奖励（美元）', 'Bonus reward (USD)') }}</th>
              <th class="w-16 px-4 py-3">
                <span class="sr-only">{{ text('操作', 'Actions') }}</span>
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="(milestone, index) in settings.milestones" :key="index">
              <td class="px-4 py-3">
                <input
                  v-model.number="milestone.days"
                  type="number"
                  min="1"
                  step="1"
                  class="input"
                  :aria-label="text(`里程碑 ${index + 1} 天数`, `Milestone ${index + 1} days`)"
                />
              </td>
              <td class="px-4 py-3">
                <input
                  v-model.number="milestone.bonus"
                  type="number"
                  min="0.01"
                  step="0.01"
                  class="input"
                  :aria-label="text(`里程碑 ${index + 1} 奖励`, `Milestone ${index + 1} bonus`)"
                />
              </td>
              <td class="px-4 py-3 text-right">
                <button
                  type="button"
                  class="inline-flex h-9 w-9 items-center justify-center rounded text-gray-400 hover:bg-red-50 hover:text-red-600 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-red-950/30 dark:hover:text-red-400"
                  :disabled="settings.milestones.length <= 1 || saving"
                  :title="text('删除里程碑', 'Remove milestone')"
                  @click="settings.milestones.splice(index, 1)"
                >
                  <Icon name="trash" size="sm" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 验证错误提示 -->
      <p v-if="validationMessage" role="alert" class="rounded border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300">
        {{ validationMessage }}
      </p>

      <!-- 操作按钮 -->
      <div class="flex justify-between gap-3">
        <button
          type="button"
          class="btn btn-secondary inline-flex items-center gap-2"
          :disabled="settings.milestones.length >= 32 || saving"
          @click="addMilestone"
        >
          <Icon name="plus" size="sm" />
          {{ text('新增里程碑', 'Add milestone') }}
        </button>
        <!-- design-governance-allow: continuous-motion - save progress spinner is temporary and stops after the request resolves. -->
        <button
          type="button"
          class="btn btn-primary inline-flex items-center gap-2"
          :disabled="Boolean(validationMessage) || saving"
          @click="save"
        >
          <!-- design-governance-allow: continuous-motion - save progress spinner is temporary and stops after the request resolves. -->
          <Icon :name="saving ? 'refresh' : 'check'" size="sm" :class="{ 'animate-spin': saving }" />
          {{ text('保存配置', 'Save settings') }}
        </button>
      </div>
    </div>

    <TotpStepUpDialog :controller="stepUp" />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import adminPlayAPI, { type AdminCheckinMilestoneSettings, type AdminCheckinMilestone } from '@/api/admin/play'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import { isStepUpBlocked, isStepUpCancelled, stepUpBlockReason, useStepUp } from '@/composables/useStepUp'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'

const { locale } = useI18n()
const appStore = useAppStore()
const settings = ref<AdminCheckinMilestoneSettings | null>(null)
const loading = ref(false)
const saving = ref(false)
const stepUp = useStepUp()

const isZh = computed(() => locale.value.startsWith('zh'))
const text = (zh: string, en: string) => isZh.value ? zh : en

const validationMessage = computed(() => validate())

function validate(): string {
  if (!settings.value) return ''

  const { milestones } = settings.value

  if (!milestones.length || milestones.length > 32) {
    return text('里程碑必须为 1 到 32 个', 'Milestones must contain 1 to 32 entries')
  }

  let previousDays = 0
  for (const milestone of milestones) {
    if (!Number.isInteger(milestone.days) || milestone.days <= previousDays) {
      return text('天数必须是正整数且严格递增', 'Days must be positive integers and strictly increasing')
    }
    if (!Number.isFinite(Number(milestone.bonus)) || Number(milestone.bonus) <= 0) {
      return text('奖励必须是正数', 'Bonus must be positive')
    }
    previousDays = milestone.days
  }

  return ''
}

function addMilestone() {
  if (!settings.value || settings.value.milestones.length >= 32) return

  const last = settings.value.milestones.at(-1)
  settings.value.milestones.push({
    days: (last?.days ?? 0) + 7,
    bonus: last?.bonus ?? 1.0
  } as AdminCheckinMilestone)
}

function clone(value: AdminCheckinMilestoneSettings): AdminCheckinMilestoneSettings {
  return {
    milestones: value.milestones.map((m) => ({ ...m })),
    makeup_enabled: value.makeup_enabled
  }
}

async function load() {
  loading.value = true
  try {
    settings.value = clone(await adminPlayAPI.getCheckinMilestoneSettings())
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, text('加载签到里程碑配置失败', 'Failed to load check-in milestone settings')))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!settings.value || validationMessage.value || saving.value) return

  saving.value = true
  try {
    settings.value = clone(await stepUp.run(() => adminPlayAPI.updateCheckinMilestoneSettings(clone(settings.value!))))
    appStore.showSuccess(text('签到里程碑配置已保存', 'Check-in milestone settings saved'))
  } catch (error) {
    if (isStepUpCancelled(error)) return
    if (isStepUpBlocked(error)) {
      appStore.showError(
        stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
          ? text('管理员 API Key 不能完成此验证，请使用管理员会话。', 'An administrator API key cannot complete this verification. Use an administrator session.')
          : text('当前管理员尚未启用验证器，请先在账号安全设置中启用。', 'This administrator has not enabled an authenticator. Enable it in account security first.')
      )
      return
    }
    appStore.showError(extractApiErrorMessage(error, text('保存签到里程碑配置失败', 'Failed to save check-in milestone settings')))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
