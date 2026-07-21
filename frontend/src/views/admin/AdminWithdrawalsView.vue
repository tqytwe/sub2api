<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.withdrawals.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.description') }}</p>
        </div>
        <button type="button" class="btn btn-secondary inline-flex items-center gap-2 self-start" :disabled="loading" @click="refreshAll">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('admin.withdrawals.refresh') }}
        </button>
      </div>

      <div v-if="loadError" class="rounded border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-200">
        {{ t('admin.withdrawals.loadFailed') }}
      </div>

      <div class="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(360px,420px)]">
        <section class="card">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.withdrawals.settingsTitle') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.settingsHint') }}</p>
          </div>
          <form class="grid gap-4 p-5 md:grid-cols-2 xl:grid-cols-4" @submit.prevent="saveSystemSettings">
            <label class="flex items-center gap-3 rounded border border-gray-200 bg-white px-3 py-2 text-sm dark:border-dark-700 dark:bg-dark-900">
              <input v-model="settingsForm.global_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span class="font-medium text-gray-700 dark:text-gray-200">{{ t('admin.withdrawals.globalEnabled') }}</span>
            </label>
            <label class="grid gap-1 text-sm">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.minimumAmount') }}</span>
              <input v-model.trim="settingsForm.minimum_amount" class="input" inputmode="decimal" placeholder="10.00" />
            </label>
            <label class="grid gap-1 text-sm">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.dailyLimit') }}</span>
              <input v-model.trim="settingsForm.daily_limit_amount" class="input" inputmode="decimal" placeholder="500.00" />
            </label>
            <label class="grid gap-1 text-sm">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.dualReview') }}</span>
              <input v-model.trim="settingsForm.double_review_threshold" class="input" inputmode="decimal" placeholder="100.00" />
            </label>
            <div class="text-sm">
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.rewardMaturityHours') }}</p>
              <p class="mt-2 font-semibold text-gray-900 dark:text-white">{{ systemSettings?.reward_maturity_hours ?? '-' }}</p>
            </div>
            <div class="text-sm">
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.updatedAt') }}</p>
              <p class="mt-2 font-semibold text-gray-900 dark:text-white">{{ formatDateTime(systemSettings?.updated_at) }}</p>
            </div>
            <div class="flex items-end md:col-span-2">
              <button type="submit" class="btn btn-primary inline-flex w-full items-center justify-center gap-2" :disabled="settingsSaving">
                <Icon name="save" size="sm" />
                {{ settingsSaving ? t('admin.withdrawals.saving') : t('admin.withdrawals.saveSettings') }}
              </button>
            </div>
          </form>
        </section>

        <section class="card">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.withdrawals.userSettingsTitle') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.userSettingsHint') }}</p>
          </div>
          <div class="space-y-5 p-5">
            <form class="grid gap-3" @submit.prevent="saveUserSettings">
              <div class="flex gap-2">
                <label class="grid flex-1 gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.userId') }}</span>
                  <input v-model.trim="userSettingsForm.user_id" class="input" inputmode="numeric" :placeholder="t('admin.withdrawals.userIdPlaceholder')" />
                </label>
                <button type="button" class="btn btn-secondary self-end" :disabled="userSettingsLoading" @click="loadUserSettings">
                  {{ t('admin.withdrawals.loadUser') }}
                </button>
              </div>
              <label class="flex items-center gap-3 rounded border border-gray-200 bg-white px-3 py-2 text-sm dark:border-dark-700 dark:bg-dark-900">
                <input v-model="userSettingsForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
                <span class="font-medium text-gray-700 dark:text-gray-200">{{ t('admin.withdrawals.enabled') }}</span>
              </label>
              <div class="grid gap-3 sm:grid-cols-2">
                <label class="grid gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.minimumOverride') }}</span>
                  <input v-model.trim="userSettingsForm.minimum_amount_override" class="input" inputmode="decimal" placeholder="10.00" />
                </label>
                <label class="grid gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.dailyLimitOverride') }}</span>
                  <input v-model.trim="userSettingsForm.daily_limit_amount_override" class="input" inputmode="decimal" placeholder="500.00" />
                </label>
              </div>
              <label class="grid gap-1 text-sm">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.disabledReason') }}</span>
                <input v-model.trim="userSettingsForm.disabled_reason" class="input" :placeholder="t('admin.withdrawals.disabledReasonPlaceholder')" />
              </label>
              <div class="flex items-center justify-between gap-3 text-sm">
                <span class="text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.recalcStatus') }}</span>
                <span class="font-medium text-gray-900 dark:text-white">{{ userSettings?.recalc_status || '-' }}</span>
              </div>
              <button type="submit" class="btn btn-primary inline-flex items-center justify-center gap-2" :disabled="userSettingsSaving">
                <Icon name="save" size="sm" />
                {{ userSettingsSaving ? t('admin.withdrawals.saving') : t('admin.withdrawals.saveUser') }}
              </button>
            </form>

            <form class="grid gap-3 border-t border-gray-100 pt-5 dark:border-dark-700" @submit.prevent="saveBatchSettings">
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.withdrawals.batchTitle') }}</h3>
              <label class="grid gap-1 text-sm">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.userIds') }}</span>
                <textarea v-model.trim="batchForm.user_ids" class="input min-h-20 resize-y" :placeholder="t('admin.withdrawals.userIdsPlaceholder')"></textarea>
              </label>
              <label class="grid gap-1 text-sm">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.statusLabel') }}</span>
                <select v-model="batchForm.enabled" class="input">
                  <option value="true">{{ t('admin.withdrawals.enabled') }}</option>
                  <option value="false">{{ t('admin.withdrawals.disabled') }}</option>
                </select>
              </label>
              <div class="grid gap-3 sm:grid-cols-2">
                <label class="grid gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.minimumOverride') }}</span>
                  <input v-model.trim="batchForm.minimum_amount_override" class="input" inputmode="decimal" placeholder="10.00" />
                </label>
                <label class="grid gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.dailyLimitOverride') }}</span>
                  <input v-model.trim="batchForm.daily_limit_amount_override" class="input" inputmode="decimal" placeholder="500.00" />
                </label>
              </div>
              <label class="grid gap-1 text-sm">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.disabledReason') }}</span>
                <input v-model.trim="batchForm.disabled_reason" class="input" :placeholder="t('admin.withdrawals.disabledReasonPlaceholder')" />
              </label>
              <button type="submit" class="btn btn-secondary inline-flex items-center justify-center gap-2" :disabled="batchSaving">
                <Icon name="save" size="sm" />
                {{ batchSaving ? t('admin.withdrawals.saving') : t('admin.withdrawals.batchSave') }}
              </button>
            </form>
          </div>
        </section>
      </div>

      <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_440px]">
        <section class="card">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.withdrawals.queueTitle') }}</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.queueHint') }}</p>
              </div>
              <form class="grid gap-2 sm:grid-cols-[160px_160px_auto_auto]" @submit.prevent="loadQueue">
                <label class="grid gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.statusFilter') }}</span>
                  <select v-model="query.status" class="input">
                    <option v-for="status in statusOptions" :key="status" :value="status">
                      {{ status === 'all' ? t('admin.withdrawals.allStatuses') : statusLabel(status) }}
                    </option>
                  </select>
                </label>
                <label class="grid gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.userIdFilter') }}</span>
                  <input v-model.trim="query.user_id" class="input" inputmode="numeric" :placeholder="t('admin.withdrawals.userIdPlaceholder')" />
                </label>
                <button type="submit" class="btn btn-primary self-end">
                  {{ t('admin.withdrawals.search') }}
                </button>
                <button type="button" class="btn btn-secondary self-end" @click="resetQuery">
                  {{ t('admin.withdrawals.reset') }}
                </button>
              </form>
            </div>
          </div>

          <div class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-100 text-sm dark:divide-dark-700">
              <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                <tr>
                  <th class="px-4 py-3">{{ t('admin.withdrawals.requestNo') }}</th>
                  <th class="px-4 py-3">{{ t('admin.withdrawals.user') }}</th>
                  <th class="px-4 py-3">{{ t('admin.withdrawals.statusLabel') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('admin.withdrawals.amount') }}</th>
                  <th class="px-4 py-3">{{ t('admin.withdrawals.method') }}</th>
                  <th class="px-4 py-3">{{ t('admin.withdrawals.createdAt') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('admin.withdrawals.actions') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="item in withdrawalPage.items" :key="item.id" :class="activeWithdrawal?.id === item.id ? 'bg-primary-50/50 dark:bg-primary-950/20' : ''">
                  <td class="px-4 py-3 align-top">
                    <div class="font-medium text-gray-900 dark:text-white">{{ item.request_no }}</div>
                    <div class="mt-1 text-xs text-gray-500">#{{ item.id }}</div>
                  </td>
                  <td class="px-4 py-3 align-top">
                    <div class="font-medium text-gray-900 dark:text-white">#{{ item.user_id }}</div>
                    <div v-if="item.user_email" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ item.user_email }}</div>
                  </td>
                  <td class="px-4 py-3 align-top">
                    <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="statusClass(item.status)">
                      {{ statusLabel(item.status) }}
                    </span>
                  </td>
                  <td class="px-4 py-3 text-right align-top tabular-nums text-gray-900 dark:text-white">{{ formatMoney(item.amount, item.currency) }}</td>
                  <td class="px-4 py-3 align-top text-gray-700 dark:text-gray-200">{{ methodLabel(item.payout_method) }}</td>
                  <td class="whitespace-nowrap px-4 py-3 align-top text-gray-600 dark:text-gray-300">{{ formatDateTime(item.created_at) }}</td>
                  <td class="px-4 py-3 text-right align-top">
                    <button type="button" class="btn btn-secondary btn-sm" :disabled="detailLoading" @click="selectWithdrawal(item.id)">
                      {{ t('admin.withdrawals.detail') }}
                    </button>
                  </td>
                </tr>
                <tr v-if="!withdrawalPage.items.length">
                  <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">
                    {{ loading ? t('admin.withdrawals.loading') : t('admin.withdrawals.emptyQueue') }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="flex flex-col gap-3 border-t border-gray-100 px-5 py-4 text-sm dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
            <span class="text-gray-500 dark:text-gray-400">
              {{ t('admin.withdrawals.pageInfo', { page: withdrawalPage.page, pages: withdrawalPage.pages, total: withdrawalPage.total }) }}
            </span>
            <div class="flex gap-2">
              <button type="button" class="btn btn-secondary btn-sm" :disabled="withdrawalPage.page <= 1 || loading" @click="changePage(withdrawalPage.page - 1)">
                {{ t('common.previous') }}
              </button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="withdrawalPage.page >= withdrawalPage.pages || loading" @click="changePage(withdrawalPage.page + 1)">
                {{ t('common.next') }}
              </button>
            </div>
          </div>
        </section>

        <aside class="space-y-6">
          <section class="card">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.withdrawals.detailTitle') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ activeWithdrawal ? activeWithdrawal.request_no : t('admin.withdrawals.detailHint') }}</p>
            </div>
            <div v-if="!activeWithdrawal" class="px-5 py-12 text-center text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.withdrawals.noSelection') }}
            </div>
            <div v-else class="space-y-5 p-5">
              <dl class="grid gap-3 text-sm sm:grid-cols-2">
                <div>
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.statusLabel') }}</dt>
                  <dd class="mt-1">
                    <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="statusClass(activeWithdrawal.status)">
                      {{ statusLabel(activeWithdrawal.status) }}
                    </span>
                  </dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.amount') }}</dt>
                  <dd class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatMoney(activeWithdrawal.amount, activeWithdrawal.currency) }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.userId') }}</dt>
                  <dd class="mt-1 font-semibold text-gray-900 dark:text-white">#{{ activeWithdrawal.user_id }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.method') }}</dt>
                  <dd class="mt-1 font-semibold text-gray-900 dark:text-white">{{ methodLabel(activeWithdrawal.payout_method) }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.payoutCurrency') }}</dt>
                  <dd class="mt-1 font-semibold text-gray-900 dark:text-white">{{ activeWithdrawal.payout_currency }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.accountMask') }}</dt>
                  <dd class="mt-1 break-words font-semibold text-gray-900 dark:text-white">{{ activeWithdrawal.payout_account_mask }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.recipientMask') }}</dt>
                  <dd class="mt-1 font-semibold text-gray-900 dark:text-white">{{ activeWithdrawal.payout_recipient_name_mask || '-' }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.paidAt') }}</dt>
                  <dd class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatDateTime(activeWithdrawal.paid_at) }}</dd>
                </div>
              </dl>

              <dl class="grid gap-3 border-t border-gray-100 pt-4 text-sm dark:border-dark-700 sm:grid-cols-2">
                <div>
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.firstApprovedBy') }}</dt>
                  <dd class="mt-1 text-gray-900 dark:text-white">{{ activeWithdrawal.first_approved_by ? `#${activeWithdrawal.first_approved_by}` : '-' }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.secondApprovedBy') }}</dt>
                  <dd class="mt-1 text-gray-900 dark:text-white">{{ activeWithdrawal.second_approved_by ? `#${activeWithdrawal.second_approved_by}` : '-' }}</dd>
                </div>
                <div v-if="activeWithdrawal.rejected_reason" class="sm:col-span-2">
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.rejectedReason') }}</dt>
                  <dd class="mt-1 text-gray-900 dark:text-white">{{ activeWithdrawal.rejected_reason }}</dd>
                </div>
              </dl>
            </div>
          </section>

          <section v-if="activeWithdrawal" class="card">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.withdrawals.reviewTitle') }}</h2>
            </div>
            <div class="space-y-4 p-5">
              <label class="grid gap-1 text-sm">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.reviewNote') }}</span>
                <textarea v-model.trim="reviewNote" class="input min-h-20 resize-y" :placeholder="t('admin.withdrawals.reviewNotePlaceholder')"></textarea>
              </label>
              <div class="grid gap-2 sm:grid-cols-2">
                <button type="button" class="btn btn-primary inline-flex items-center justify-center gap-2" :disabled="reviewActionLoading || !canReview" @click="approveActive">
                  <Icon name="check" size="sm" />
                  {{ reviewActionLoading ? t('admin.withdrawals.approving') : t('admin.withdrawals.approve') }}
                </button>
                <button type="button" class="btn btn-danger inline-flex items-center justify-center gap-2" :disabled="reviewActionLoading || !canReview" @click="rejectActive">
                  <Icon name="x" size="sm" />
                  {{ reviewActionLoading ? t('admin.withdrawals.rejecting') : t('admin.withdrawals.reject') }}
                </button>
              </div>
              <label class="grid gap-1 text-sm">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.rejectReason') }}</span>
                <textarea v-model.trim="rejectReason" class="input min-h-20 resize-y" :placeholder="t('admin.withdrawals.rejectReasonPlaceholder')" maxlength="500"></textarea>
              </label>
            </div>
          </section>

          <section v-if="activeWithdrawal" class="card">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.withdrawals.sensitiveTitle') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.sensitiveHint') }}</p>
            </div>
            <div class="space-y-4 p-5">
              <button type="button" class="btn btn-secondary inline-flex w-full items-center justify-center gap-2" :disabled="sensitiveLoading" @click="loadSensitivePayout">
                <Icon name="eye" size="sm" />
                {{ sensitiveLoading ? t('admin.withdrawals.readingSensitive') : t('admin.withdrawals.readSensitive') }}
              </button>
              <dl v-if="sensitiveEntries.length" class="grid gap-3 text-sm">
                <div v-for="entry in sensitiveEntries" :key="entry.key" class="rounded border border-gray-100 p-3 dark:border-dark-700">
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ sensitiveFieldLabel(entry.key) }}</dt>
                  <dd class="mt-1 break-words font-medium text-gray-900 dark:text-white">{{ entry.value }}</dd>
                </div>
              </dl>
              <div v-else class="rounded border border-dashed border-gray-200 px-4 py-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
                {{ t('admin.withdrawals.sensitiveEmpty') }}
              </div>
            </div>
          </section>

          <section v-if="activeWithdrawal" class="card">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.withdrawals.payoutTitle') }}</h2>
            </div>
            <form class="grid gap-3 p-5" @submit.prevent="markActivePaid">
              <div class="grid gap-3 sm:grid-cols-2">
                <label class="grid gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.paidAmount') }}</span>
                  <input v-model.trim="payoutForm.paid_amount" class="input" inputmode="decimal" placeholder="10.00" />
                </label>
                <label class="grid gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.paidCurrency') }}</span>
                  <select v-model="payoutForm.paid_currency" class="input">
                    <option value="USD">USD</option>
                    <option value="CNY">CNY</option>
                  </select>
                </label>
              </div>
              <div class="grid gap-3 sm:grid-cols-2">
                <label class="grid gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.payoutFxRate') }}</span>
                  <input v-model.trim="payoutForm.payout_fx_rate" class="input" inputmode="decimal" placeholder="1.00" />
                </label>
                <label class="grid gap-1 text-sm">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.paidAtOptional') }}</span>
                  <input v-model="payoutForm.paid_at" type="datetime-local" class="input" />
                </label>
              </div>
              <label class="grid gap-1 text-sm">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.externalTxnId') }}</span>
                <input v-model.trim="payoutForm.external_txn_id" class="input" :placeholder="t('admin.withdrawals.externalTxnIdPlaceholder')" />
              </label>
              <label class="grid gap-1 text-sm">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.withdrawals.payoutNote') }}</span>
                <textarea v-model.trim="payoutForm.note" class="input min-h-20 resize-y" :placeholder="t('admin.withdrawals.payoutNotePlaceholder')"></textarea>
              </label>
              <button type="submit" class="btn btn-primary inline-flex items-center justify-center gap-2" :disabled="paidActionLoading || activeWithdrawal.status !== 'payout_pending'">
                <Icon name="dollar" size="sm" />
                {{ paidActionLoading ? t('admin.withdrawals.markingPaid') : t('admin.withdrawals.markPaid') }}
              </button>
            </form>
          </section>

          <section v-if="activeWithdrawal" class="card">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.withdrawals.historyTitle') }}</h2>
            </div>
            <ol class="space-y-3 p-5">
              <li v-for="event in activeWithdrawal.events || []" :key="event.id" class="rounded border border-gray-100 p-3 text-sm dark:border-dark-700">
                <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                  <span class="font-medium text-gray-900 dark:text-white">{{ statusLabel(event.status) }}</span>
                  <span class="text-gray-500 dark:text-gray-400">{{ formatDateTime(event.created_at) }}</span>
                </div>
                <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.withdrawals.eventActor') }}: {{ actorLabel(event.actor_type, event.actor_user_id) }}
                </div>
                <p v-if="event.note" class="mt-2 text-gray-700 dark:text-gray-200">{{ event.note }}</p>
              </li>
              <li v-if="!(activeWithdrawal.events || []).length" class="rounded border border-dashed border-gray-200 p-4 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
                {{ t('admin.withdrawals.noEvents') }}
              </li>
            </ol>
          </section>
        </aside>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import adminWithdrawalsAPI, {
  type AdminUserWithdrawalSettings,
  type AdminWithdrawalSensitivePayout,
  type AdminWithdrawalSystemSettings,
} from '@/api/admin/withdrawals'
import type {
  WithdrawalCurrency,
  WithdrawalRequest,
  WithdrawalRequestPage,
  WithdrawalStatus,
} from '@/api/wallet'

const { t, locale } = useI18n()
const appStore = useAppStore()

const statusOptions: Array<WithdrawalStatus | 'all'> = [
  'all',
  'pending_review',
  'second_review',
  'payout_pending',
  'paid',
  'rejected',
  'canceled',
]

const loading = ref(false)
const loadError = ref(false)
const detailLoading = ref(false)
const settingsSaving = ref(false)
const userSettingsLoading = ref(false)
const userSettingsSaving = ref(false)
const batchSaving = ref(false)
const reviewActionLoading = ref(false)
const sensitiveLoading = ref(false)
const paidActionLoading = ref(false)

const systemSettings = ref<AdminWithdrawalSystemSettings | null>(null)
const userSettings = ref<AdminUserWithdrawalSettings | null>(null)
const activeWithdrawal = ref<WithdrawalRequest | null>(null)
const sensitivePayout = ref<AdminWithdrawalSensitivePayout | null>(null)

const query = reactive({
  status: 'pending_review' as WithdrawalStatus | 'all',
  user_id: '',
})

const settingsForm = reactive({
  global_enabled: false,
  minimum_amount: '10.00',
  daily_limit_amount: '500.00',
  double_review_threshold: '100.00',
})

const userSettingsForm = reactive({
  user_id: '',
  enabled: false,
  minimum_amount_override: '',
  daily_limit_amount_override: '',
  disabled_reason: '',
})

const batchForm = reactive({
  user_ids: '',
  enabled: 'false',
  minimum_amount_override: '',
  daily_limit_amount_override: '',
  disabled_reason: '',
})

const reviewNote = ref('')
const rejectReason = ref('')

const payoutForm = reactive({
  paid_amount: '',
  paid_currency: 'USD' as WithdrawalCurrency,
  payout_fx_rate: '1.00',
  external_txn_id: '',
  paid_at: '',
  note: '',
})

const withdrawalPage = ref<WithdrawalRequestPage>({
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 1,
})

const canReview = computed(() => {
  return activeWithdrawal.value?.status === 'pending_review' || activeWithdrawal.value?.status === 'second_review'
})

const sensitiveEntries = computed(() => {
  if (!sensitivePayout.value) return []
  return Object.entries(sensitivePayout.value)
    .filter(([, value]) => value !== undefined && value !== null && value !== '')
    .map(([key, value]) => ({ key, value: formatSensitiveValue(value) }))
})

function formatMoney(value: string | number | undefined, currency = 'USD') {
  return new Intl.NumberFormat(locale.value, {
    style: 'currency',
    currency: currency || 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(Number(value ?? 0))
}

function formatDateTime(value: string | undefined) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString(locale.value)
}

function toDateTimeInputValue(value: string | undefined) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const offset = date.getTimezoneOffset()
  const local = new Date(date.getTime() - offset * 60_000)
  return local.toISOString().slice(0, 16)
}

function parsePositiveID(raw: string) {
  const value = Number(raw)
  return Number.isInteger(value) && value > 0 ? value : 0
}

function parseBatchUserIDs(raw: string) {
  const chunks = raw.split(/[\s,;，；]+/).filter(Boolean)
  if (!chunks.length) return []
  const ids = chunks.map((chunk) => Number(chunk))
  if (ids.some((id) => !Number.isInteger(id) || id <= 0)) return null
  return Array.from(new Set(ids))
}

function statusLabel(status: string) {
  const key = `admin.withdrawals.status.${status}`
  const translated = t(key)
  return translated === key ? t('admin.withdrawals.statusLabel') : translated
}

function methodLabel(method: string) {
  const key = `admin.withdrawals.methods.${method}`
  const translated = t(key)
  return translated === key ? t('admin.withdrawals.method') : translated
}

function statusClass(status: WithdrawalStatus) {
  if (status === 'paid') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-200'
  if (status === 'rejected' || status === 'canceled') return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  if (status === 'payout_pending') return 'bg-blue-50 text-blue-700 dark:bg-blue-950/40 dark:text-blue-200'
  return 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-200'
}

function actorLabel(actorType: string, actorUserID?: number) {
  const translated = t(`admin.withdrawals.actors.${actorType}`)
  const label = translated === `admin.withdrawals.actors.${actorType}` ? t('admin.withdrawals.actors.system') : translated
  return actorUserID ? `${label} #${actorUserID}` : label
}

function sensitiveFieldLabel(key: string) {
  const translated = t(`admin.withdrawals.sensitiveFields.${key}`)
  return translated === `admin.withdrawals.sensitiveFields.${key}` ? t('admin.withdrawals.sensitiveFields.other') : translated
}

function formatSensitiveValue(value: unknown) {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  return JSON.stringify(value)
}

function showWithdrawalError(err: unknown, fallbackKey = 'admin.withdrawals.loadFailed') {
  appStore.showError(extractI18nErrorMessage(err, t, 'admin.withdrawals.errors', t(fallbackKey)))
}

function applySystemSettings(settings: AdminWithdrawalSystemSettings) {
  systemSettings.value = settings
  settingsForm.global_enabled = settings.global_enabled
  settingsForm.minimum_amount = String(settings.minimum_amount)
  settingsForm.daily_limit_amount = String(settings.daily_limit_amount)
  settingsForm.double_review_threshold = String(settings.double_review_threshold)
}

function applyUserSettings(settings: AdminUserWithdrawalSettings) {
  userSettings.value = settings
  userSettingsForm.user_id = String(settings.user_id)
  userSettingsForm.enabled = settings.enabled
  userSettingsForm.minimum_amount_override = settings.minimum_amount_override ? String(settings.minimum_amount_override) : ''
  userSettingsForm.daily_limit_amount_override = settings.daily_limit_amount_override ? String(settings.daily_limit_amount_override) : ''
  userSettingsForm.disabled_reason = settings.disabled_reason || ''
}

function applyActiveWithdrawal(next: WithdrawalRequest) {
  activeWithdrawal.value = next
  payoutForm.paid_amount = next.paid_amount || next.amount || ''
  payoutForm.paid_currency = (next.paid_currency || next.payout_currency || 'USD') as WithdrawalCurrency
  payoutForm.payout_fx_rate = next.payout_fx_rate || '1.00'
  payoutForm.external_txn_id = next.external_txn_id || ''
  payoutForm.paid_at = toDateTimeInputValue(next.paid_at)
  payoutForm.note = next.payout_note || ''
  replaceQueueItem(next)
}

function replaceQueueItem(next: WithdrawalRequest) {
  withdrawalPage.value = {
    ...withdrawalPage.value,
    items: withdrawalPage.value.items.map((item) => (item.id === next.id ? next : item)),
  }
}

async function refreshAll() {
  await Promise.all([loadSettings(), loadQueue()])
}

async function loadSettings() {
  try {
    applySystemSettings(await adminWithdrawalsAPI.getSettings())
  } catch (err) {
    showWithdrawalError(err)
  }
}

async function saveSystemSettings() {
  settingsSaving.value = true
  try {
    applySystemSettings(await adminWithdrawalsAPI.updateSettings({
      global_enabled: settingsForm.global_enabled,
      minimum_amount: settingsForm.minimum_amount,
      daily_limit_amount: settingsForm.daily_limit_amount,
      double_review_threshold: settingsForm.double_review_threshold,
    }))
    appStore.showSuccess(t('admin.withdrawals.settingsSaved'))
  } catch (err) {
    showWithdrawalError(err, 'admin.withdrawals.settingsSaveFailed')
  } finally {
    settingsSaving.value = false
  }
}

async function loadUserSettings() {
  const userID = parsePositiveID(userSettingsForm.user_id)
  if (!userID) {
    appStore.showError(t('admin.withdrawals.validation.userIdRequired'))
    return
  }
  userSettingsLoading.value = true
  try {
    applyUserSettings(await adminWithdrawalsAPI.getUserSettings(userID))
  } catch (err) {
    showWithdrawalError(err)
  } finally {
    userSettingsLoading.value = false
  }
}

async function saveUserSettings() {
  const userID = parsePositiveID(userSettingsForm.user_id)
  if (!userID) {
    appStore.showError(t('admin.withdrawals.validation.userIdRequired'))
    return
  }
  userSettingsSaving.value = true
  try {
    applyUserSettings(await adminWithdrawalsAPI.updateUserSettings(userID, {
      enabled: userSettingsForm.enabled,
      minimum_amount_override: userSettingsForm.minimum_amount_override || undefined,
      daily_limit_amount_override: userSettingsForm.daily_limit_amount_override || undefined,
      disabled_reason: userSettingsForm.disabled_reason,
    }))
    appStore.showSuccess(t('admin.withdrawals.userSettingsSaved'))
  } catch (err) {
    showWithdrawalError(err, 'admin.withdrawals.userSettingsSaveFailed')
  } finally {
    userSettingsSaving.value = false
  }
}

async function saveBatchSettings() {
  const ids = parseBatchUserIDs(batchForm.user_ids)
  if (ids === null) {
    appStore.showError(t('admin.withdrawals.validation.invalidUserIds'))
    return
  }
  if (!ids.length) {
    appStore.showError(t('admin.withdrawals.validation.batchUserIdsRequired'))
    return
  }
  batchSaving.value = true
  try {
    const result = await adminWithdrawalsAPI.batchUpdateUserSettings({
      user_ids: ids,
      enabled: batchForm.enabled === 'true',
      minimum_amount_override: batchForm.minimum_amount_override || undefined,
      daily_limit_amount_override: batchForm.daily_limit_amount_override || undefined,
      disabled_reason: batchForm.disabled_reason,
    })
    appStore.showSuccess(t('admin.withdrawals.batchSettingsSaved', { count: result.affected }))
  } catch (err) {
    showWithdrawalError(err, 'admin.withdrawals.batchSettingsSaveFailed')
  } finally {
    batchSaving.value = false
  }
}

async function loadQueue() {
  loading.value = true
  loadError.value = false
  try {
    const userID = query.user_id ? parsePositiveID(query.user_id) : undefined
    if (query.user_id && !userID) {
      appStore.showError(t('admin.withdrawals.validation.userIdRequired'))
      return
    }
    withdrawalPage.value = await adminWithdrawalsAPI.list({
      status: query.status,
      user_id: userID,
      page: withdrawalPage.value.page,
      page_size: withdrawalPage.value.page_size,
    })
  } catch (err) {
    loadError.value = true
    showWithdrawalError(err)
  } finally {
    loading.value = false
  }
}

async function selectWithdrawal(id: number) {
  detailLoading.value = true
  sensitivePayout.value = null
  reviewNote.value = ''
  rejectReason.value = ''
  try {
    applyActiveWithdrawal(await adminWithdrawalsAPI.get(id))
  } catch (err) {
    showWithdrawalError(err)
  } finally {
    detailLoading.value = false
  }
}

async function approveActive() {
  if (!activeWithdrawal.value) {
    appStore.showError(t('admin.withdrawals.validation.selectRequest'))
    return
  }
  reviewActionLoading.value = true
  try {
    applyActiveWithdrawal(await adminWithdrawalsAPI.approve(activeWithdrawal.value.id, { note: reviewNote.value }))
    appStore.showSuccess(t('admin.withdrawals.success.approved'))
    await loadQueue()
  } catch (err) {
    showWithdrawalError(err)
  } finally {
    reviewActionLoading.value = false
  }
}

async function rejectActive() {
  if (!activeWithdrawal.value) {
    appStore.showError(t('admin.withdrawals.validation.selectRequest'))
    return
  }
  if (!rejectReason.value.trim()) {
    appStore.showError(t('admin.withdrawals.validation.rejectReasonRequired'))
    return
  }
  reviewActionLoading.value = true
  try {
    applyActiveWithdrawal(await adminWithdrawalsAPI.reject(activeWithdrawal.value.id, {
      reason: rejectReason.value,
      note: reviewNote.value,
    }))
    appStore.showSuccess(t('admin.withdrawals.success.rejected'))
    await loadQueue()
  } catch (err) {
    showWithdrawalError(err)
  } finally {
    reviewActionLoading.value = false
  }
}

async function loadSensitivePayout() {
  if (!activeWithdrawal.value) {
    appStore.showError(t('admin.withdrawals.validation.selectRequest'))
    return
  }
  sensitiveLoading.value = true
  try {
    sensitivePayout.value = await adminWithdrawalsAPI.getSensitivePayout(activeWithdrawal.value.id)
    appStore.showSuccess(t('admin.withdrawals.success.sensitiveLoaded'))
  } catch (err) {
    showWithdrawalError(err)
  } finally {
    sensitiveLoading.value = false
  }
}

async function markActivePaid() {
  if (!activeWithdrawal.value) {
    appStore.showError(t('admin.withdrawals.validation.selectRequest'))
    return
  }
  if (!payoutForm.paid_amount || !payoutForm.paid_currency || !payoutForm.external_txn_id) {
    appStore.showError(t('admin.withdrawals.validation.paidFieldsRequired'))
    return
  }
  paidActionLoading.value = true
  try {
    const paidAt = payoutForm.paid_at ? new Date(payoutForm.paid_at).toISOString() : undefined
    applyActiveWithdrawal(await adminWithdrawalsAPI.markPaid(activeWithdrawal.value.id, {
      paid_amount: payoutForm.paid_amount,
      paid_currency: payoutForm.paid_currency,
      payout_fx_rate: payoutForm.payout_fx_rate || '1.00',
      external_txn_id: payoutForm.external_txn_id,
      paid_at: paidAt,
      note: payoutForm.note,
    }))
    appStore.showSuccess(t('admin.withdrawals.success.paid'))
    await loadQueue()
  } catch (err) {
    showWithdrawalError(err)
  } finally {
    paidActionLoading.value = false
  }
}

async function changePage(page: number) {
  if (page < 1 || page > withdrawalPage.value.pages) return
  withdrawalPage.value = { ...withdrawalPage.value, page }
  await loadQueue()
}

async function resetQuery() {
  query.status = 'pending_review'
  query.user_id = ''
  withdrawalPage.value = { ...withdrawalPage.value, page: 1 }
  await loadQueue()
}

onMounted(refreshAll)
</script>
