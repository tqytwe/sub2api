<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="mb-3 flex border-b border-gray-200 dark:border-dark-700" role="tablist" :aria-label="t('admin.channelOperations.views')">
          <button type="button" role="tab" :aria-selected="viewMode === 'membership'" :class="viewMode === 'membership' ? 'border-b-2 border-primary-600 text-primary-700 dark:text-primary-300' : 'text-gray-500'" class="px-3 py-2 text-sm font-medium" @click="setViewMode('membership')">{{ t('admin.channelOperations.membershipView') }}</button>
          <button type="button" role="tab" :aria-selected="viewMode === 'statistics'" :class="viewMode === 'statistics' ? 'border-b-2 border-primary-600 text-primary-700 dark:text-primary-300' : 'text-gray-500'" class="px-3 py-2 text-sm font-medium" @click="setViewMode('statistics')">{{ t('admin.channelOperations.statisticsView') }}</button>
        </div>
        <div class="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
          <div class="grid flex-1 grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.channelOperations.channel') }}
              <select v-model.number="channelId" class="input mt-1" @change="resetAndLoad">
                <option :value="0">{{ t('admin.channelOperations.selectChannel') }}</option>
                <option v-for="channel in channels" :key="channel.id" :value="channel.id">
                  {{ channel.name }} #{{ channel.id }}
                </option>
              </select>
            </label>
            <label v-if="viewMode === 'membership'" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.channelOperations.memberAccounts') }}
              <input v-model="search" class="input mt-1" :placeholder="t('admin.channelOperations.searchAccounts')" @keyup.enter="applyFilters" />
            </label>
            <label v-if="viewMode === 'membership'" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.channelOperations.membership') }}
              <select v-model="membership" class="input mt-1" @change="applyFilters">
                <option value="all">{{ t('admin.channelOperations.all') }}</option>
                <option value="assigned">{{ t('admin.channelOperations.assigned') }}</option>
                <option value="unassigned">{{ t('admin.channelOperations.unassigned') }}</option>
              </select>
            </label>
            <label v-if="viewMode === 'membership'" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.channelOperations.upstreamFilter') }}
              <select v-model.number="upstreamFilter" class="input mt-1" @change="applyFilters">
                <option :value="0">{{ t('admin.channelOperations.allUpstreams') }}</option>
                <option :value="-1">{{ t('admin.channelOperations.unassigned') }}</option>
                <option v-for="upstream in upstreams" :key="upstream.id" :value="upstream.id">
                  {{ upstream.name }} #{{ upstream.id }}
                </option>
              </select>
            </label>
          </div>
          <div class="flex items-center gap-2">
            <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="loadCatalog">
              <!-- design-governance-allow: continuous-motion - user-triggered refresh feedback stops when the request settles and honors reduced-motion. -->
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin motion-reduce:animate-none' : ''" />
            </button>
            <button v-if="viewMode === 'membership'" class="btn btn-primary" :disabled="!channelId || busy" @click="openUpstreamDialog()">
              <Icon name="plus" size="md" class="mr-2" />{{ t('admin.channelOperations.createUpstream') }}
            </button>
          </div>
        </div>
        <p class="mt-3 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.channelOperations.scopeNote') }}</p>
        <form v-if="viewMode === 'statistics'" class="mt-4 grid grid-cols-1 items-end gap-3 sm:grid-cols-2 xl:grid-cols-6" @submit.prevent="applySnapshotFilters">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.channelOperations.from') }}<input v-model="from" class="input mt-1" type="datetime-local" required /></label>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.channelOperations.until') }}<input v-model="until" class="input mt-1" type="datetime-local" required /></label>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.channelOperations.group') }}<input v-model.number="groupId" class="input mt-1" type="number" min="0" step="1" /></label>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.channelOperations.modelIdentity') }}<select v-model="modelScope" class="input mt-1"><option value="public">{{ t('admin.channelOperations.publicModel') }}</option><option value="recorded">{{ t('admin.channelOperations.recordedModel') }}</option><option value="upstream">{{ t('admin.channelOperations.upstreamModel') }}</option></select></label>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.channelOperations.exactModel') }}<input v-model.trim="model" class="input mt-1" maxlength="200" /></label>
          <button class="btn btn-primary" :disabled="loading || !channelId">{{ t('common.apply') }}</button>
        </form>
        <div v-if="viewMode === 'statistics' && statsError" class="mt-3 flex items-center justify-between gap-3 border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-900 dark:bg-red-900/20 dark:text-red-200" role="alert"><span>{{ statsError }}</span><button class="btn btn-secondary" :disabled="loading" @click="loadCatalog">{{ t('common.retry') }}</button></div>
        <p v-if="viewMode === 'statistics' && snapshot" class="mt-3 text-xs text-gray-500">{{ t('admin.channelOperations.appliedScope') }} · {{ snapshot.from }} — {{ snapshot.until }} · {{ t('admin.channelOperations.group') }} {{ snapshot.scope.group_id }} · {{ snapshot.scope.model_scope }}: {{ snapshot.scope.model || '*' }} · {{ snapshot.scope.dimension }}</p>
      </template>

      <template #actions>
          <div v-if="channelId && viewMode === 'membership'" class="space-y-3">
          <div class="flex flex-wrap items-center gap-2">
            <span class="text-sm font-medium text-gray-600 dark:text-gray-300">{{ t('admin.channelOperations.upstreams') }}</span>
            <form class="flex items-center gap-2" @submit.prevent="searchUpstreams">
              <input v-model="upstreamSearch" class="input w-52" :placeholder="t('admin.channelOperations.searchUpstreams')" />
              <button class="btn btn-secondary btn-sm" :disabled="loading">{{ t('common.search') }}</button>
            </form>
            <div
              v-for="upstream in upstreams"
              :key="upstream.id"
              class="inline-flex items-center border"
              :class="upstreamFilter === upstream.id ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20' : 'border-gray-300 dark:border-dark-600'"
            >
              <button type="button" class="inline-flex items-center gap-2 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700" :aria-label="`${t('admin.channelOperations.openUpstream')}: ${upstream.name}`" @click="selectUpstream(upstream.id)">
                <span>{{ upstream.name }}</span><span class="text-xs text-gray-500">{{ upstream.members }}</span>
              </button>
              <button type="button" class="rounded p-1 text-gray-500 hover:bg-gray-100 dark:hover:bg-dark-600" :aria-label="t('admin.channelOperations.renameUpstream', { name: upstream.name })" @click="openUpstreamDialog(upstream)">
                <Icon name="edit" size="xs" />
              </button>
              <button type="button" class="rounded p-1 text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20" :aria-label="t('admin.channelOperations.deleteUpstream', { name: upstream.name })" @click="queueDeleteUpstream(upstream)">
                <Icon name="trash" size="xs" />
              </button>
            </div>
            <span v-if="upstreams.length === 0 && !loading" class="text-sm text-gray-500">{{ t('admin.channelOperations.noUpstreams') }}</span>
          </div>
          <div v-if="upstreamTotal > pageSize" class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
            <button class="btn btn-secondary btn-sm" :disabled="loading || upstreamPage <= 1" @click="upstreamPage--; loadCatalog()">{{ t('pagination.previous') }}</button>
            <span>{{ upstreamPage }} / {{ Math.ceil(upstreamTotal / pageSize) }}</span>
            <button class="btn btn-secondary btn-sm" :disabled="loading || upstreamPage * pageSize >= upstreamTotal" @click="upstreamPage++; loadCatalog()">{{ t('pagination.next') }}</button>
          </div>
        </div>
      </template>

      <template #table>
        <div v-if="error && viewMode === 'membership'" class="m-4 flex items-center justify-between gap-3 border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-900 dark:bg-red-900/20 dark:text-red-200" role="alert">
          <span>{{ error }}</span>
          <button class="btn btn-secondary" :disabled="loading" @click="loadCatalog">{{ t('common.retry') }}</button>
        </div>
        <div v-if="notice" class="m-4 border border-green-300 bg-green-50 p-3 text-sm text-green-800 dark:border-green-900 dark:bg-green-900/20 dark:text-green-200" role="status">{{ notice }}</div>
        <div v-if="!channelId && !loading" class="p-12 text-center text-sm text-gray-500">{{ t('admin.channelOperations.chooseChannel') }}</div>
        <DataTable v-else-if="viewMode === 'membership'" :columns="columns" :data="accounts" :loading="loading" row-key="id">
          <template #cell-name="{ row }">
            <div class="font-medium text-gray-900 dark:text-white">{{ row.name || t('admin.channelOperations.unnamedAccount') }}</div>
            <div class="text-xs text-gray-500">#{{ row.id }} · {{ row.platform }}</div>
          </template>
          <template #cell-state="{ row }">
            <div>{{ row.status }}</div>
            <div class="text-xs text-gray-500">{{ t('admin.channelOperations.memberConcurrency', { count: row.concurrency }) }}</div>
          </template>
          <template #cell-upstream="{ row }">
            <span v-if="row.upstream_id">{{ upstreamName(row.upstream_id) }}</span>
            <span v-else class="text-gray-500">{{ t('admin.channelOperations.unassigned') }}</span>
            <span v-if="!row.eligible" class="ml-2 text-xs text-amber-700 dark:text-amber-400">{{ t('admin.channelOperations.ineligible') }}</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex flex-wrap items-center gap-2">
              <button type="button" class="btn btn-secondary btn-sm" :disabled="busy || !row.eligible || upstreams.length === 0" @click="openAssignment(row)">{{ row.upstream_id ? t('admin.channelOperations.reassign') : t('admin.channelOperations.assign') }}</button>
              <button v-if="row.upstream_id" type="button" class="btn btn-ghost btn-sm text-red-600" :disabled="busy" @click="queueDetach(row)">{{ t('admin.channelOperations.detach') }}</button>
            </div>
          </template>
          <template #empty>
            <div class="p-12 text-center">
              <p class="font-medium text-gray-900 dark:text-white">{{ error ? t('admin.channelOperations.queryFailed') : t('admin.channelOperations.empty') }}</p>
              <p class="mt-1 text-sm text-gray-500">{{ error ? t('admin.channelOperations.failedHint') : t('admin.channelOperations.emptyHint') }}</p>
            </div>
          </template>
        </DataTable>
        <div v-else-if="viewMode === 'statistics'" class="space-y-5 p-4" :aria-busy="loading">
          <p v-if="loading && !snapshot" class="py-8 text-center text-sm text-gray-500" role="status">{{ t('admin.channelOperations.loading') }}</p>
          <template v-else-if="snapshot">
            <section class="space-y-3">
              <header><h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.channelOperations.usage') }}</h2><p class="text-xs leading-5 text-gray-500">{{ t('admin.channelOperations.latencyNote') }}</p></header>
              <p v-if="snapshot.usage.status !== 'ready'" class="border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-900/20 dark:text-amber-200" role="status">{{ sectionStatus(snapshot.usage.status, snapshot.usage.error) }}</p>
              <template v-else-if="snapshot.usage.data">
                <dl class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                  <div v-for="metric in usageMetrics" :key="metric.key" class="border border-gray-200 p-3 dark:border-dark-700"><dt class="text-xs text-gray-500">{{ t(`admin.channelOperations.${metric.key}`) }}</dt><dd class="mt-1 text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ metric.value }}</dd></div>
                </dl>
                <p class="text-xs leading-5 text-gray-500">{{ snapshot.usage.data.attribution }} · {{ snapshot.usage.data.latency_measurement }} · {{ t('admin.channelOperations.errorRateUnavailable') }}</p>
                <div v-if="snapshot.usage.data.rows.length" class="overflow-x-auto"><table class="w-full min-w-[520px] text-left text-sm"><thead class="text-xs text-gray-500"><tr><th class="px-3 py-2">{{ t('admin.channelOperations.dimension') }}</th><th class="px-3 py-2">{{ t('admin.channelOperations.requests') }}</th><th class="px-3 py-2">{{ t('admin.channelOperations.known') }}</th><th class="px-3 py-2">{{ t('admin.channelOperations.p90') }}</th><th class="px-3 py-2">{{ t('admin.channelOperations.latest') }}</th></tr></thead><tbody><tr v-for="row in snapshot.usage.data.rows" :key="row.dimension_id" class="border-t border-gray-100 dark:border-dark-700"><td class="px-3 py-2"><button class="text-left font-medium text-primary-700 hover:underline dark:text-primary-300" @click="drillUsage(row.dimension_id)">{{ row.name || (row.dimension_id === -1 ? t('admin.channelOperations.unassigned') : `#${row.dimension_id}`) }} <span class="text-xs text-gray-500">{{ t('admin.channelOperations.drilldown') }}</span></button></td><td class="px-3 py-2 tabular-nums">{{ row.requests }}</td><td class="px-3 py-2 tabular-nums">{{ row.samples }} / {{ row.unknown_samples }}</td><td class="px-3 py-2 tabular-nums">{{ latency(row.p90_ms) }}</td><td class="px-3 py-2">{{ formatDate(row.latest_sample_at) }}</td></tr></tbody></table></div>
                <div v-if="snapshot.usage.data.total > pageSize" class="flex items-center justify-end gap-2"><button class="btn btn-secondary btn-sm" :disabled="loading || statsPage <= 1" @click="changeStatsPage(statsPage - 1)">{{ t('pagination.previous') }}</button><span class="text-sm text-gray-500">{{ statsPage }} / {{ Math.ceil(snapshot.usage.data.total / pageSize) }}</span><button class="btn btn-secondary btn-sm" :disabled="loading || statsPage * pageSize >= snapshot.usage.data.total" @click="changeStatsPage(statsPage + 1)">{{ t('pagination.next') }}</button></div>
                <p v-else class="text-sm text-gray-500">{{ t('admin.channelOperations.noUsage') }}</p>
                <details v-if="snapshot.usage.data.lineage.length" class="border-t border-gray-100 pt-3 text-sm dark:border-dark-700"><summary class="cursor-pointer font-medium">{{ t('admin.channelOperations.lineage') }} ({{ snapshot.usage.data.lineage_total }})</summary><p class="mt-2 text-xs text-gray-500">{{ t('admin.channelOperations.lineageNote') }}</p><div class="mt-2 space-y-2"><div v-for="(line, index) in snapshot.usage.data.lineage" :key="index" class="grid gap-1 border-b border-gray-100 pb-2 text-xs dark:border-dark-700 sm:grid-cols-4"><span>{{ t('admin.channelOperations.publicModel') }}: {{ line.public_model || '—' }}</span><span>{{ t('admin.channelOperations.recordedModel') }}: {{ line.recorded_model || '—' }}</span><span>{{ t('admin.channelOperations.upstreamModel') }}: {{ line.upstream_model || '—' }}</span><span :title="line.model_mapping_chain || ''">{{ t('admin.channelOperations.chain') }}: {{ line.model_mapping_chain || '—' }}</span></div></div></details>
              </template>
            </section>

            <div class="grid gap-5 xl:grid-cols-2">
              <section class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-700"><h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.channelOperations.quality') }}</h2><p v-if="snapshot.quality.status !== 'ready'" class="text-sm text-amber-800 dark:text-amber-300" role="status">{{ sectionStatus(snapshot.quality.status, snapshot.quality.error) }}</p><template v-else-if="snapshot.quality.data"><p class="text-xs text-gray-500">{{ snapshot.quality.data.scope }}</p><dl class="flex flex-wrap gap-4 text-sm"><div><dt class="text-gray-500">{{ t('admin.channelOperations.rounds') }}</dt><dd>{{ snapshot.quality.data.summary.rounds }}</dd></div><div><dt class="text-gray-500">{{ t('admin.channelOperations.passed') }}</dt><dd>{{ snapshot.quality.data.summary.passed }}</dd></div><div><dt class="text-gray-500">{{ t('admin.channelOperations.degraded') }}</dt><dd>{{ snapshot.quality.data.summary.degraded }}</dd></div><div><dt class="text-gray-500">{{ t('admin.channelOperations.inconclusive') }}</dt><dd>{{ snapshot.quality.data.summary.inconclusive }}</dd></div></dl><p v-if="!snapshot.quality.data.rows.length" class="text-sm text-gray-500">{{ t('admin.channelOperations.noQuality') }}</p><div v-else class="space-y-2"> <div v-for="row in snapshot.quality.data.rows" :key="row.id" class="border-b border-gray-100 pb-2 text-sm dark:border-dark-700"><div class="flex flex-wrap items-center justify-between gap-2"><span>{{ row.measured_model || '—' }} · {{ formatDate(row.observed_at) }}</span><span :class="row.outcome === 'passed' ? 'text-green-700 dark:text-green-300' : row.outcome === 'degraded' ? 'text-red-700 dark:text-red-300' : 'text-amber-700 dark:text-amber-300'">{{ t(`admin.channelOperations.outcomes.${row.outcome}`) }}</span></div><p class="text-xs text-gray-500">{{ t('admin.channelOperations.probes') }}: {{ row.passed_probes }} / {{ row.probes }} ({{ row.expected ?? '—' }}) · {{ t('admin.channelOperations.plan') }} #{{ row.plan_id }}</p><p v-if="!row.round_complete" class="text-xs text-amber-700 dark:text-amber-400">{{ t('admin.channelOperations.incompleteRound') }}</p></div></div><button v-if="snapshot.quality.data.has_more" class="btn btn-secondary btn-sm" :disabled="loading" @click="loadOlderQuality">{{ t('admin.channelOperations.older') }}</button></template></section>
              <section class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-700"><h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.channelOperations.errors') }}</h2><p v-if="snapshot.errors.status !== 'ready'" class="text-sm text-amber-800 dark:text-amber-300" role="status">{{ sectionStatus(snapshot.errors.status, snapshot.errors.error) }}</p><template v-else-if="snapshot.errors.data"><p class="text-xs leading-5 text-gray-500">{{ snapshot.errors.data.scope }} {{ t('admin.channelOperations.errorRateUnavailable') }}</p><dl class="flex flex-wrap gap-4 text-sm"><div><dt class="text-gray-500">{{ t('admin.channelOperations.errorRecords') }}</dt><dd>{{ snapshot.errors.data.records }}</dd></div><div><dt class="text-gray-500">{{ t('admin.channelOperations.upstream5xx') }}</dt><dd>{{ snapshot.errors.data.upstream_5xx }}</dd></div><div><dt class="text-gray-500">{{ t('admin.channelOperations.latest') }}</dt><dd>{{ formatDate(snapshot.errors.data.latest_sample_at) }}</dd></div></dl><p v-if="!snapshot.errors.data.rows.length" class="text-sm text-gray-500">{{ t('admin.channelOperations.noErrors') }}</p><div v-else class="space-y-1 text-sm"><div v-for="(row, index) in snapshot.errors.data.rows" :key="index" class="grid grid-cols-4 gap-2 border-b border-gray-100 py-1 text-xs dark:border-dark-700"><span>{{ row.error_type }}</span><span>{{ row.error_phase }}</span><span>{{ row.status_code }}</span><span>{{ row.records }}</span></div></div></template></section>
            </div>

              <section class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-700"><h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.channelOperations.capacity') }}</h2><p v-if="snapshot.capacity.status !== 'ready'" class="text-sm text-amber-800 dark:text-amber-300" role="status">{{ sectionStatus(snapshot.capacity.status, snapshot.capacity.error) }}</p><template v-else-if="snapshot.capacity.data"><p class="text-xs leading-5 text-gray-500">{{ snapshot.capacity.data.scope }}</p><dl class="grid gap-3 sm:grid-cols-3"><div><dt class="text-xs text-gray-500">{{ t('admin.channelOperations.capacityAccounts') }}</dt><dd class="font-semibold tabular-nums">{{ snapshot.capacity.data.accounts }}</dd></div><div><dt class="text-xs text-gray-500">{{ t('admin.channelOperations.schedulable') }}</dt><dd class="font-semibold tabular-nums">{{ snapshot.capacity.data.schedulable }}</dd></div><div><dt class="text-xs text-gray-500">{{ t('admin.channelOperations.capacityConcurrency') }}</dt><dd class="font-semibold tabular-nums">{{ snapshot.capacity.data.configured_concurrency }}</dd></div></dl></template></section>
          </template>
          <p v-else class="py-8 text-center text-sm text-gray-500">{{ t(statsError ? 'admin.channelOperations.statsFailed' : 'admin.channelOperations.noStats') }}</p>
        </div>
      </template>

      <template #pagination>
        <Pagination v-if="channelId && viewMode === 'membership' && total > 0" :page="page" :total="total" :page-size="pageSize" @update:page="setPage" @update:page-size="setPageSize" />
      </template>
    </TablePageLayout>

    <BaseDialog :show="!!upstreamEdit" :title="upstreamEdit?.id ? t('admin.channelOperations.renameTitle') : t('admin.channelOperations.createTitle')" @close="!busy && (upstreamEdit = null)">
      <form class="space-y-4" @submit.prevent="saveUpstream">
        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.channelOperations.upstreamName') }}<input v-model="upstreamEdit!.name" class="input mt-1" maxlength="100" required autofocus /></label>
        <p v-if="dialogError" class="text-sm text-red-600" role="alert">{{ dialogError }}</p>
        <div class="flex justify-end gap-2"><button type="button" class="btn btn-secondary" :disabled="busy" @click="upstreamEdit = null">{{ t('common.cancel') }}</button><button class="btn btn-primary" :disabled="busy || !upstreamEdit?.name.trim()">{{ t('common.save') }}</button></div>
      </form>
    </BaseDialog>

    <BaseDialog :show="!!assignment" :title="assignment?.upstream_id ? t('admin.channelOperations.reassignTitle') : t('admin.channelOperations.assignTitle')" @close="!busy && (assignment = null)">
      <div class="space-y-4">
        <p class="text-sm text-gray-600 dark:text-gray-300">{{ assignment?.name }} #{{ assignment?.id }}</p>
        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.channelOperations.targetUpstream') }}
          <select v-model.number="targetUpstreamId" class="input mt-1">
            <option v-for="upstream in upstreams" :key="upstream.id" :value="upstream.id">{{ upstream.name }} #{{ upstream.id }}</option>
          </select>
        </label>
        <p v-if="dialogError" class="text-sm text-red-600" role="alert">{{ dialogError }}</p>
        <div class="flex justify-end gap-2"><button class="btn btn-secondary" :disabled="busy" @click="assignment = null">{{ t('common.cancel') }}</button><button class="btn btn-primary" :disabled="busy || !targetUpstreamId" @click="saveAssignment">{{ t('common.save') }}</button></div>
      </div>
    </BaseDialog>

    <ConfirmDialog :show="!!pending" :title="t('admin.channelOperations.confirmTitle')" :message="confirmMessage" :confirm-text="t('common.confirm')" :danger="true" @cancel="!busy && (pending = null)" @confirm="confirmPending" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import * as channelsAPI from '@/api/admin/channels'
import * as operationsAPI from '@/api/admin/channelOperations'
import type { ChannelOperationAccount, ChannelOperationsMutation, ChannelOperationsSnapshot, ChannelOperationsSnapshotQuery, ChannelOperationUpstream } from '@/api/admin/channelOperations'

const { t } = useI18n()
const pageSize = ref(20)
const page = ref(1)
const total = ref(0)
const upstreamTotal = ref(0)
const viewMode = ref<'membership' | 'statistics'>('membership')
const channels = ref<channelsAPI.Channel[]>([])
const channelId = ref(0)
const catalog = ref<operationsAPI.ChannelOperationsCatalog | null>(null)
const loading = ref(false)
const busy = ref(false)
const error = ref('')
const statsError = ref('')
const dialogError = ref('')
const notice = ref('')
const search = ref('')
const membership = ref<'all' | 'assigned' | 'unassigned'>('all')
const upstreamFilter = ref(0)
const upstreamId = ref(0)
const accountId = ref(0)
const upstreamSearch = ref('')
const upstreamPage = ref(1)
const snapshot = ref<ChannelOperationsSnapshot | null>(null)
const from = ref(toLocalDateTime(new Date(Date.now() - 24 * 60 * 60_000)))
const until = ref(toLocalDateTime(new Date()))
const groupId = ref(0)
const model = ref('')
const modelScope = ref<'public' | 'recorded' | 'upstream'>('public')
const statsPage = ref(1)
const qualityBefore = ref(0)
const upstreams = computed(() => catalog.value?.upstreams ?? [])
const accounts = computed(() => catalog.value?.accounts ?? [])
const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.channelOperations.account'), sortable: false },
  { key: 'state', label: t('admin.channelOperations.status'), sortable: false },
  { key: 'upstream', label: t('admin.channelOperations.upstream'), sortable: false },
  { key: 'actions', label: t('common.actions'), sortable: false },
])
const usageMetrics = computed(() => {
  const metrics = snapshot.value?.usage.data?.summary
  return [
    { key: 'requests', value: metrics?.requests ?? '—' },
    { key: 'known', value: metrics?.samples ?? '—' },
    { key: 'unknown', value: metrics?.unknown_samples ?? '—' },
    { key: 'p50', value: latency(metrics?.p50_ms ?? null) },
    { key: 'p90', value: latency(metrics?.p90_ms ?? null) },
    { key: 'p95', value: latency(metrics?.p95_ms ?? null) },
    { key: 'durationSamples', value: metrics?.duration_samples ?? '—' },
    { key: 'durationP90', value: latency(metrics?.duration_p90_ms ?? null) },
  ]
})
const upstreamEdit = ref<{ id: number; name: string } | null>(null)
const assignment = ref<ChannelOperationAccount | null>(null)
const targetUpstreamId = ref(0)
type Pending = { kind: 'mutation'; body: ChannelOperationsMutation; label: string }
const pending = ref<Pending | null>(null)
const confirmMessage = computed(() => pending.value?.label ?? '')
let activeRequest = 0
let alive = true
let abortController: AbortController | null = null

function apiStatus(value: unknown): number | undefined {
  return (value as { status?: number } | null)?.status
}

function toLocalDateTime(date: Date) {
  return new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 16)
}

function latency(value: number | null) {
  return value == null ? '—' : `${Math.round(value).toLocaleString()} ms`
}

function formatDate(value: string | null) {
  return value ? new Date(value).toLocaleString() : '—'
}

function sectionStatus(status: string, detail?: string) {
  const key = status === 'query_failed' ? 'statsQueryFailed' : 'unsupportedScope'
  return `${t(`admin.channelOperations.${key}`)}${detail ? `: ${detail}` : ''}`
}

function currentDimension(): ChannelOperationsSnapshotQuery['dimension'] {
  if (accountId.value) return 'account'
  if (upstreamId.value !== 0) return 'upstream'
  return 'channel'
}

function snapshotQuery(start: Date, end: Date): ChannelOperationsSnapshotQuery {
  return {
    channel_id: channelId.value,
    upstream_id: upstreamId.value,
    account_id: accountId.value,
    group_id: groupId.value || 0,
    from: start.toISOString(),
    until: end.toISOString(),
    model: model.value.trim(),
    model_scope: modelScope.value,
    dimension: currentDimension(),
    page: statsPage.value,
    page_size: pageSize.value,
    quality_before: qualityBefore.value,
  }
}

function failureText(value: unknown): string {
  return apiStatus(value) === 409 ? t('admin.channelOperations.conflict') : t('admin.channelOperations.failed')
}

function upstreamName(id: number): string {
  return upstreams.value.find((upstream) => upstream.id === id)?.name ?? `#${id}`
}

async function loadChannels() {
  try {
    const result = await channelsAPI.list(1, 100, { sort_by: 'name', sort_order: 'asc' })
    channels.value = result.items
    if (!channelId.value && result.items.length) channelId.value = result.items[0].id
  } catch {
    channels.value = []
    error.value = t('admin.channelOperations.failed')
  }
}

async function loadCatalog() {
  if (!channelId.value) {
    catalog.value = null
    snapshot.value = null
    return
  }
  const requestId = ++activeRequest
  abortController?.abort()
  abortController = new AbortController()
  loading.value = true
  error.value = ''
  statsError.value = ''
  if (viewMode.value === 'statistics') snapshot.value = null
  try {
    const result = await operationsAPI.getCatalog(channelId.value, {
      page: page.value,
      page_size: pageSize.value,
      search: search.value.trim(),
      upstream_page: upstreamPage.value,
      upstream_search: upstreamSearch.value.trim(),
      upstream_id: upstreamFilter.value,
      membership: membership.value,
    }, abortController.signal)
    if (!alive || requestId !== activeRequest) return
    catalog.value = result
    total.value = result.total
    upstreamTotal.value = result.upstream_total
    if (viewMode.value === 'statistics') {
      const start = new Date(from.value)
      const end = new Date(until.value)
      const maxWindow = 31 * 24 * 60 * 60_000
      if (!Number.isFinite(start.getTime()) || !Number.isFinite(end.getTime()) || end <= start || end.getTime() - start.getTime() > maxWindow) {
        snapshot.value = null
        statsError.value = t('admin.channelOperations.invalidWindow')
      } else {
        try {
          snapshot.value = await operationsAPI.getSnapshot(snapshotQuery(start, end), abortController.signal)
          if (!alive || requestId !== activeRequest) return
        } catch {
          if (!alive || requestId !== activeRequest) return
          snapshot.value = null
          statsError.value = t('admin.channelOperations.statsFailed')
        }
      }
    }
  } catch (failure) {
    if (!alive || requestId !== activeRequest || apiStatus(failure) === 0) return
    catalog.value = null
    total.value = 0
    error.value = failureText(failure)
  } finally {
    if (alive && requestId === activeRequest) loading.value = false
  }
}

function setViewMode(next: 'membership' | 'statistics') {
  if (viewMode.value === next) return
  viewMode.value = next
  if (next === 'statistics') {
    upstreamId.value = 0
    accountId.value = 0
    qualityBefore.value = 0
  }
  void loadCatalog()
}

function applySnapshotFilters() {
  statsPage.value = 1
  qualityBefore.value = 0
  void loadCatalog()
}

function changeStatsPage(next: number) {
  statsPage.value = next
  void loadCatalog()
}

function drillUsage(id: number) {
  statsPage.value = 1
  if (currentDimension() === 'channel') {
    upstreamId.value = id
    accountId.value = 0
  } else if (currentDimension() === 'upstream') {
    accountId.value = id
  }
  qualityBefore.value = 0
  void loadCatalog()
}

function loadOlderQuality() {
  const rows = snapshot.value?.quality.data?.rows
  if (!rows?.length) return
  qualityBefore.value = rows[rows.length - 1].id
  void loadCatalog()
}

async function resetAndLoad() {
  page.value = 1
  upstreamPage.value = 1
  upstreamFilter.value = 0
  catalog.value = null
  await loadCatalog()
}

function applyFilters() {
  page.value = 1
  void loadCatalog()
}

function searchUpstreams() {
  upstreamPage.value = 1
  void loadCatalog()
}

function selectUpstream(id: number) {
  upstreamFilter.value = upstreamFilter.value === id ? 0 : id
  membership.value = 'all'
  applyFilters()
}

function setPage(next: number) {
  page.value = next
  void loadCatalog()
}

function setPageSize(next: number) {
  pageSize.value = next
  page.value = 1
  void loadCatalog()
}

function openUpstreamDialog(upstream?: ChannelOperationUpstream) {
  dialogError.value = ''
  upstreamEdit.value = { id: upstream?.id ?? 0, name: upstream?.name ?? '' }
}

function openAssignment(account: ChannelOperationAccount) {
  dialogError.value = ''
  assignment.value = account
  targetUpstreamId.value = upstreams.value.find((upstream) => upstream.id !== account.upstream_id)?.id ?? 0
}

function queue(body: Omit<ChannelOperationsMutation, 'revision'>, label: string) {
  if (!catalog.value) return
  pending.value = { kind: 'mutation', body: { ...body, revision: catalog.value.revision }, label }
}

function queueDeleteUpstream(upstream: ChannelOperationUpstream) {
  queue(
    { action: 'delete_upstream', upstream_id: upstream.id, confirm: true },
    t('admin.channelOperations.deleteConfirm', { name: upstream.name, count: upstream.members })
  )
}

function queueDetach(account: ChannelOperationAccount) {
  queue(
    { action: 'detach', account_id: account.id, expected_upstream_id: account.upstream_id, confirm: true },
    t('admin.channelOperations.detachConfirm', { account: account.name || `#${account.id}`, upstream: upstreamName(account.upstream_id) })
  )
}

async function saveUpstream() {
  if (!upstreamEdit.value || !catalog.value) return
  const edit = upstreamEdit.value
  await performMutation({
    action: edit.id ? 'update_upstream' : 'create_upstream',
    revision: catalog.value.revision,
    ...(edit.id ? { upstream_id: edit.id } : {}),
    name: edit.name.trim(),
  }, true)
}

async function saveAssignment() {
  if (!assignment.value || !catalog.value || !targetUpstreamId.value) return
  const account = assignment.value
  const body: ChannelOperationsMutation = {
    action: 'assign',
    revision: catalog.value.revision,
    upstream_id: targetUpstreamId.value,
    account_id: account.id,
    expected_upstream_id: account.upstream_id,
    ...(account.upstream_id ? { confirm: true } : {}),
  }
  if (account.upstream_id) {
    queue(body, t('admin.channelOperations.reassignConfirm', { account: account.name || `#${account.id}`, from: upstreamName(account.upstream_id), to: upstreamName(targetUpstreamId.value) }))
  } else {
    await performMutation(body, true)
  }
}

async function performMutation(body: ChannelOperationsMutation, closeDialog = false) {
  if (busy.value || !channelId.value) return
  busy.value = true
  error.value = ''
  dialogError.value = ''
  notice.value = ''
  try {
    await operationsAPI.mutate(channelId.value, body)
    pending.value = null
    if (closeDialog) {
      upstreamEdit.value = null
      assignment.value = null
    }
    notice.value = t('admin.channelOperations.saved')
    await loadCatalog()
  } catch (failure) {
    const message = failureText(failure)
    pending.value = null
    await loadCatalog()
    if (upstreamEdit.value || assignment.value) dialogError.value = message
    else error.value = message
  } finally {
    busy.value = false
  }
}

async function confirmPending() {
  const action = pending.value
  if (action) await performMutation(action.body)
}

onMounted(async () => {
  await loadChannels()
  await loadCatalog()
})

onBeforeUnmount(() => {
  alive = false
  activeRequest++
  abortController?.abort()
})
</script>
