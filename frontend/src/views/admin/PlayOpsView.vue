<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.playOps.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.playOps.description') }}</p>
        </div>
        <button type="button" class="btn btn-secondary inline-flex items-center gap-2 self-start" :disabled="loading" @click="load">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('admin.playOps.refresh') }}
        </button>
      </div>

      <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-6">
        <div v-for="card in statCards" :key="card.label" class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ card.label }}</p>
          <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ card.value }}</p>
        </div>
      </div>

      <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_420px]">
        <div class="space-y-6">
          <section class="card">
            <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.playOps.campaignsTitle') }}</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.playOps.campaignsHint') }}</p>
              </div>
              <button type="button" class="btn btn-primary inline-flex items-center gap-2 self-start" data-testid="new-campaign" @click="startCreateCampaign">
                <Icon name="plus" size="sm" />
                {{ t('admin.playOps.newCampaign') }}
              </button>
            </div>

            <form v-if="campaignFormOpen" class="border-b border-gray-100 bg-gray-50/60 px-5 py-4 dark:border-dark-700 dark:bg-dark-800/40" data-testid="campaign-form" @submit.prevent="submitCampaign">
              <div class="grid gap-4 lg:grid-cols-2">
                <label class="space-y-1">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.campaignName') }}</span>
                  <input v-model="campaignForm.name" class="input" data-testid="campaign-name" required maxlength="128" />
                </label>
                <label class="flex items-center gap-3 rounded border border-gray-200 bg-white px-3 py-2 text-sm dark:border-dark-700 dark:bg-dark-900">
                  <input v-model="campaignForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
                  <span class="font-medium text-gray-700 dark:text-gray-200">{{ t('admin.playOps.campaignEnabled') }}</span>
                </label>
                <label class="space-y-1">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.campaignNameZh') }}</span>
                  <input v-model="campaignForm.nameZh" class="input" maxlength="128" />
                </label>
                <label class="space-y-1">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.campaignNameEn') }}</span>
                  <input v-model="campaignForm.nameEn" class="input" maxlength="128" />
                </label>
                <label class="space-y-1">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.campaignStartAt') }}</span>
                  <input v-model="campaignForm.startAt" type="datetime-local" class="input" data-testid="campaign-start" required />
                </label>
                <label class="space-y-1">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.campaignEndAt') }}</span>
                  <input v-model="campaignForm.endAt" type="datetime-local" class="input" data-testid="campaign-end" required />
                </label>
                <label class="space-y-1">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.rechargeBonusPct') }}</span>
                  <input v-model="campaignForm.rechargeBonusPct" type="number" min="0" max="1000" step="0.01" class="input" data-testid="campaign-recharge-bonus" />
                </label>
                <label class="space-y-1">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.blindboxExtraOpens') }}</span>
                  <input v-model="campaignForm.blindboxExtraOpens" type="number" min="0" max="100" step="1" class="input" data-testid="campaign-blindbox-extra" />
                </label>
                <label class="space-y-1">
                  <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.playOps.arenaScoreMultiplier') }}</span>
                  <input v-model="campaignForm.arenaScoreMultiplier" type="number" min="0" max="100" step="0.01" class="input" data-testid="campaign-arena-multiplier" :placeholder="t('admin.playOps.arenaMultiplierPlaceholder')" />
                </label>
              </div>
              <div class="mt-4 flex flex-col gap-2 sm:flex-row sm:justify-end">
                <button type="button" class="btn btn-secondary inline-flex items-center justify-center gap-2" :disabled="campaignSaving" @click="closeCampaignForm">
                  <Icon name="x" size="sm" />
                  {{ t('admin.playOps.cancel') }}
                </button>
                <button type="submit" class="btn btn-primary inline-flex items-center justify-center gap-2" data-testid="save-campaign" :disabled="campaignSaving">
                  <Icon name="save" size="sm" />
                  {{ campaignSaving ? t('admin.playOps.saving') : t('admin.playOps.saveCampaign') }}
                </button>
              </div>
            </form>

            <div class="overflow-x-auto">
              <table class="min-w-full divide-y divide-gray-100 text-sm dark:divide-dark-700">
                <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                  <tr>
                    <th class="px-4 py-3">{{ t('admin.playOps.campaign') }}</th>
                    <th class="px-4 py-3">{{ t('admin.playOps.campaignWindow') }}</th>
                    <th class="px-4 py-3">{{ t('admin.playOps.campaignRules') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.playOps.actions') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                  <tr v-for="campaign in campaigns" :key="campaign.id">
                    <td class="px-4 py-3 align-top">
                      <div class="flex flex-wrap items-center gap-2">
                        <span class="font-medium text-gray-900 dark:text-white">{{ campaign.name }}</span>
                        <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="campaignStatusClass(campaign)">
                          {{ t(`admin.playOps.campaignStatus.${campaignStatus(campaign)}`) }}
                        </span>
                      </div>
                      <div class="mt-1 text-xs text-gray-500">#{{ campaign.id }}</div>
                    </td>
                    <td class="px-4 py-3 align-top">
                      <div class="tabular-nums">{{ formatDateTime(campaign.start_at) }}</div>
                      <div class="mt-1 text-xs text-gray-500">{{ formatDateTime(campaign.end_at) }}</div>
                    </td>
                    <td class="px-4 py-3 align-top">
                      <div v-if="campaignRuleLines(campaign).length" class="space-y-1">
                        <div v-for="line in campaignRuleLines(campaign)" :key="line" class="text-gray-700 dark:text-gray-200">{{ line }}</div>
                      </div>
                      <span v-else class="text-gray-500">{{ t('admin.playOps.noCampaignRules') }}</span>
                    </td>
                    <td class="px-4 py-3 text-right align-top">
                      <div class="inline-flex items-center gap-2">
                        <button type="button" class="btn btn-secondary btn-sm inline-flex items-center gap-1" @click="startEditCampaign(campaign)">
                          <Icon name="edit" size="xs" />
                          {{ t('admin.playOps.edit') }}
                        </button>
                        <button type="button" class="btn btn-danger btn-sm inline-flex items-center gap-1" :disabled="campaignDeletingId === campaign.id" @click="deleteCampaign(campaign)">
                          <Icon name="trash" size="xs" />
                          {{ t('admin.playOps.delete') }}
                        </button>
                      </div>
                    </td>
                  </tr>
                  <tr v-if="!campaigns.length">
                    <td colspan="4" class="px-4 py-8 text-center text-gray-500">{{ loading ? t('admin.playOps.loading') : t('admin.playOps.noCampaigns') }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <section class="card">
            <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.playOps.arenaTitle') }}</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.playOps.arenaHint') }}</p>
              </div>
              <div class="inline-flex rounded border border-gray-200 p-1 text-sm dark:border-dark-700">
                <button type="button" class="rounded px-3 py-1" :class="arenaPeriodType === 'daily' ? activeTabClass : idleTabClass" @click="switchArena('daily')">
                  {{ t('admin.playOps.daily') }}
                </button>
                <button type="button" class="rounded px-3 py-1" :class="arenaPeriodType === 'monthly' ? activeTabClass : idleTabClass" @click="switchArena('monthly')">
                  {{ t('admin.playOps.monthly') }}
                </button>
              </div>
            </div>
            <div class="overflow-x-auto">
              <table class="min-w-full divide-y divide-gray-100 text-sm dark:divide-dark-700">
                <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                  <tr>
                    <th class="px-4 py-3">{{ t('admin.playOps.rank') }}</th>
                    <th class="px-4 py-3">{{ t('admin.playOps.user') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.playOps.tokens') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.playOps.estimatedReward') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                  <tr v-for="row in arena?.rows || []" :key="row.user_id">
                    <td class="px-4 py-3 font-medium">#{{ row.rank }}</td>
                    <td class="px-4 py-3">
                      <div>{{ row.display_name }}</div>
                      <div v-if="row.email" class="text-xs text-gray-500">{{ row.email }}</div>
                    </td>
                    <td class="px-4 py-3 text-right tabular-nums">{{ formatNumber(row.token_sum) }}</td>
                    <td class="px-4 py-3 text-right tabular-nums">{{ formatMoney(row.estimated_reward) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <section class="card">
            <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.playOps.teamsTitle') }}</h2>
              <div class="flex flex-col gap-2 sm:flex-row">
                <input v-model="query" type="search" class="input sm:w-72" :placeholder="t('admin.playOps.searchPlaceholder')" @keyup.enter="loadTeams" />
                <select v-model="status" class="input sm:w-32" @change="loadTeams">
                  <option value="active">{{ t('admin.playOps.statusActive') }}</option>
                  <option value="archived">{{ t('admin.playOps.statusArchived') }}</option>
                  <option value="all">{{ t('admin.playOps.statusAll') }}</option>
                </select>
              </div>
            </div>
            <div class="overflow-x-auto">
              <table class="min-w-full divide-y divide-gray-100 text-sm dark:divide-dark-700">
                <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                  <tr>
                    <th class="px-4 py-3">{{ t('admin.playOps.team') }}</th>
                    <th class="px-4 py-3">{{ t('admin.playOps.captain') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.playOps.members') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.playOps.spend') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.playOps.pool') }}</th>
                    <th class="px-4 py-3"></th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                  <tr v-for="team in teams.items" :key="team.id">
                    <td class="px-4 py-3">
                      <div class="font-medium text-gray-900 dark:text-white">{{ team.name }}</div>
                      <div class="text-xs text-gray-500">{{ team.invite_code }}</div>
                    </td>
                    <td class="px-4 py-3">
                      <div>{{ team.captain_display_name }}</div>
                      <div class="text-xs text-gray-500">{{ team.captain_email }}</div>
                    </td>
                    <td class="px-4 py-3 text-right tabular-nums">{{ team.member_count }}</td>
                    <td class="px-4 py-3 text-right tabular-nums">{{ formatMoney(team.team_spend) }}</td>
                    <td class="px-4 py-3 text-right tabular-nums">{{ formatMoney(team.estimated_pool) }}</td>
                    <td class="px-4 py-3 text-right">
                      <button type="button" class="btn btn-secondary btn-sm" @click="selectTeam(team.id)">
                        {{ t('admin.playOps.details') }}
                      </button>
                    </td>
                  </tr>
                  <tr v-if="!teams.items.length">
                    <td colspan="6" class="px-4 py-8 text-center text-gray-500">{{ loading ? t('admin.playOps.loading') : t('admin.playOps.noTeams') }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>

        <aside class="card self-start">
          <div class="flex items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.playOps.detailTitle') }}</h2>
            <button
              v-if="selectedTeam && !selectedTeam.archived_at"
              type="button"
              class="btn btn-primary btn-sm inline-flex items-center gap-1"
              data-testid="add-team-member"
              @click="openMemberRepair"
            >
              <Icon name="userPlus" size="xs" />
              {{ t('admin.playOps.addMember') }}
            </button>
          </div>
          <div v-if="selectedTeam" class="space-y-5 p-5">
            <div>
              <h3 class="text-base font-semibold">{{ selectedTeam.team.name }}</h3>
              <p class="text-sm text-gray-500">{{ t('admin.playOps.inviteCode') }}: {{ selectedTeam.team.invite_code }}</p>
            </div>
            <div>
              <h4 class="mb-2 text-sm font-semibold">{{ t('admin.playOps.memberContributions') }}</h4>
              <div class="space-y-2">
                <div v-for="member in selectedTeam.team.members" :key="member.user_id" class="rounded border border-gray-200 p-3 text-sm dark:border-dark-700">
                  <div class="flex justify-between gap-3">
                    <span class="min-w-0">
                      <span class="block truncate font-medium">{{ member.display_name }}</span>
                      <span v-if="member.email" class="block truncate text-xs text-gray-500">{{ member.email }}</span>
                    </span>
                    <span>{{ formatMoney(member.spend) }}</span>
                  </div>
                  <div class="mt-1 flex justify-between gap-3 text-xs text-gray-500">
                    <span>{{ t('admin.playOps.memberTokensLine', { tokens: formatNumber(member.token_sum), pct: member.spend_pct }) }}</span>
                    <span>{{ t('admin.playOps.memberEstimated') }} {{ formatMoney(member.estimated_reward) }}</span>
                  </div>
                </div>
              </div>
            </div>
            <div>
              <h4 class="mb-2 text-sm font-semibold">{{ t('admin.playOps.settlements') }}</h4>
              <div v-if="!selectedTeam.settlements.length" class="text-sm text-gray-500">{{ t('admin.playOps.noSettlements') }}</div>
              <div v-for="record in selectedTeam.settlements" :key="record.settlement.id" class="mb-3 rounded border border-gray-200 p-3 text-sm dark:border-dark-700">
                <div class="flex justify-between gap-3 font-medium">
                  <span>{{ record.settlement.period_start.slice(0, 7) }}</span>
                  <span>{{ formatMoney(record.settlement.pool_amount) }} · {{ settlementStatusLabel(record.settlement.status) }}</span>
                </div>
                <div v-for="allocation in record.allocations" :key="allocation.id" class="mt-2 flex justify-between gap-3 text-xs text-gray-500">
                  <span class="min-w-0">
                    <span class="block truncate">{{ allocation.display_name || `#${allocation.user_id}` }}</span>
                    <span v-if="allocation.email" class="block truncate">{{ allocation.email }}</span>
                  </span>
                  <span>{{ formatMoney(allocation.reward_amount) }} · {{ payoutStatusLabel(allocation.payout_status) }}</span>
                </div>
              </div>
            </div>
            <div>
              <h4 class="mb-2 text-sm font-semibold">{{ t('admin.playOps.eventsTitle') }}</h4>
              <div v-if="!teamEvents.length" class="text-sm text-gray-500">{{ t('admin.playOps.noEvents') }}</div>
              <ol v-else class="space-y-3">
                <li v-for="event in teamEvents" :key="event.id" class="border-l-2 border-gray-200 pl-3 text-sm dark:border-dark-600">
                  <div class="flex items-start justify-between gap-3">
                    <span class="font-medium text-gray-800 dark:text-gray-100">{{ eventLabel(event.event_type) }}</span>
                    <time class="shrink-0 text-xs text-gray-500">{{ formatDateTime(event.created_at) }}</time>
                  </div>
                  <p class="mt-1 text-xs text-gray-500">
                    {{ t('admin.playOps.eventActorSubject', {
                      actor: event.actor_display_name,
                      subject: event.subject_display_name || `#${event.subject_user_id || '-'}`,
                    }) }}
                  </p>
                  <p v-if="eventReason(event)" class="mt-1 text-xs text-gray-500">
                    {{ t('admin.playOps.eventReason', { reason: eventReason(event) }) }}
                  </p>
                  <p v-if="eventEffectiveAtLabel(event)" class="mt-1 text-xs text-gray-500">
                    {{ eventEffectiveAtLabel(event) }}
                  </p>
                  <p v-if="eventTeamTransitionLabel(event)" class="mt-1 text-xs text-gray-500">
                    {{ eventTeamTransitionLabel(event) }}
                  </p>
                </li>
              </ol>
            </div>
          </div>
          <div v-else class="p-5 text-sm text-gray-500">{{ t('admin.playOps.noTeams') }}</div>
        </aside>
      </div>

      <BaseDialog
        :show="memberRepairOpen"
        :title="t('admin.playOps.memberRepair.title')"
        :close-label="t('admin.playOps.memberRepair.closeDialog')"
        width="wide"
        @close="closeMemberRepair"
      >
        <div class="space-y-5">
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.playOps.memberRepair.description') }}</p>

          <div class="grid gap-4 md:grid-cols-2">
            <div>
              <span class="input-label">{{ t('admin.playOps.memberRepair.operation') }}</span>
              <div
                class="inline-flex w-full rounded border border-gray-200 p-1 dark:border-dark-700"
                role="group"
                :aria-label="t('admin.playOps.memberRepair.operationSelection')"
              >
                <button
                  type="button"
                  class="flex-1 rounded px-3 py-2 text-sm"
                  :class="memberRepairOperation === 'add' ? activeTabClass : idleTabClass"
                  data-testid="member-repair-operation-add"
                  :aria-pressed="memberRepairOperation === 'add'"
                  @click="changeMemberRepairOperation('add')"
                >
                  {{ t('admin.playOps.memberRepair.operationAdd') }}
                </button>
                <button
                  type="button"
                  class="flex-1 rounded px-3 py-2 text-sm"
                  :class="memberRepairOperation === 'move' ? activeTabClass : idleTabClass"
                  data-testid="member-repair-operation-move"
                  :aria-pressed="memberRepairOperation === 'move'"
                  @click="changeMemberRepairOperation('move')"
                >
                  {{ t('admin.playOps.memberRepair.operationMove') }}
                </button>
              </div>
            </div>
            <label>
              <span class="input-label">{{ t('admin.playOps.memberRepair.effectiveAt') }}</span>
              <input
                v-model="memberRepairEffectiveAt"
                type="datetime-local"
                class="input"
                :min="memberRepairMonthStart"
                :max="memberRepairNow"
                @change="clearMemberRepairPreview"
              />
              <span class="input-hint">{{ t('admin.playOps.memberRepair.effectiveNow') }}</span>
            </label>
          </div>

          <div>
            <label class="input-label" for="member-candidate-query">{{ t('admin.playOps.memberRepair.search') }}</label>
            <div class="flex flex-col gap-2 sm:flex-row">
              <input
                id="member-candidate-query"
                v-model="memberCandidateQuery"
                type="search"
                class="input flex-1"
                data-testid="member-candidate-query"
                :placeholder="t('admin.playOps.memberRepair.searchPlaceholder')"
                @input="clearMemberRepairPreview"
                @keyup.enter="searchMemberCandidates"
              />
              <button
                type="button"
                class="btn btn-secondary inline-flex items-center justify-center gap-2"
                data-testid="search-member-candidates"
                :disabled="memberCandidatesLoading || !memberCandidateQuery.trim()"
                @click="searchMemberCandidates"
              >
                <Icon name="search" size="sm" />
                {{ t('admin.playOps.memberRepair.searchAction') }}
              </button>
            </div>
          </div>

          <div>
            <h4 class="mb-2 text-sm font-semibold">{{ t('admin.playOps.memberRepair.selectCandidate') }}</h4>
            <div v-if="memberCandidatesLoading" class="py-6 text-center text-sm text-gray-500">{{ t('admin.playOps.loading') }}</div>
            <div v-else-if="memberCandidateSearched && !memberCandidates.length" class="py-6 text-center text-sm text-gray-500">
              {{ t('admin.playOps.memberRepair.noCandidates') }}
            </div>
            <div v-else-if="!memberCandidateSearched" class="py-6 text-center text-sm text-gray-500">
              {{ t('admin.playOps.memberRepair.searchFirst') }}
            </div>
            <div
              v-else
              class="grid gap-3"
              role="listbox"
              :aria-label="t('admin.playOps.memberRepair.candidateSelection')"
            >
              <button
                v-for="candidate in memberCandidates"
                :key="candidate.user_id"
                type="button"
                class="rounded border p-4 text-left transition-colors"
                :class="selectedMemberCandidate?.user_id === candidate.user_id
                  ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20'
                  : 'border-gray-200 hover:border-primary-300 dark:border-dark-700'"
                :data-testid="`member-candidate-${candidate.user_id}`"
                role="option"
                :aria-selected="selectedMemberCandidate?.user_id === candidate.user_id"
                @click="selectedMemberCandidate = candidate"
              >
                <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                  <span class="min-w-0">
                    <span class="block truncate font-medium text-gray-900 dark:text-white">{{ candidate.display_name }}</span>
                    <span class="block truncate text-xs text-gray-500">{{ candidate.email }} · #{{ candidate.user_id }}</span>
                  </span>
                  <span class="text-xs font-medium text-gray-600 dark:text-gray-300">
                    {{ memberUserStatusLabel(candidate.status) }} ·
                    {{ candidate.is_captain ? t('admin.playOps.memberRepair.captain') : t('admin.playOps.memberRepair.regularMember') }}
                  </span>
                </div>
                <div class="mt-3 grid gap-2 text-xs text-gray-500 sm:grid-cols-2">
                  <span>
                    {{ t('admin.playOps.memberRepair.currentTeam') }}:
                    {{ candidate.current_team?.name || t('admin.playOps.memberRepair.noCurrentTeam') }}
                    <template v-if="candidate.current_team?.archived_at"> · {{ t('admin.playOps.memberRepair.archivedTeam') }}</template>
                  </span>
                  <span>
                    {{ t('admin.playOps.memberRepair.affiliate') }}:
                    {{ candidate.affiliate?.inviter_display_name || t('admin.playOps.memberRepair.noAffiliate') }}
                  </span>
                </div>
                <div class="mt-3 grid gap-2 text-xs sm:grid-cols-2">
                  <span class="font-medium text-gray-700 dark:text-gray-200 sm:col-span-2">
                    {{ t('admin.playOps.memberRepair.impactTitle') }}
                  </span>
                  <span>
                    {{ t('admin.playOps.memberRepair.userSpend') }}:
                    {{ formatMoney(candidate.impact.user_spend) }}
                  </span>
                  <span>{{ t('admin.playOps.memberRepair.targetSpend') }}: {{ impactLine(candidate.impact.target_spend_before, candidate.impact.target_spend_after) }}</span>
                  <span>{{ t('admin.playOps.memberRepair.targetPool') }}: {{ impactLine(candidate.impact.target_pool_before, candidate.impact.target_pool_after) }}</span>
                  <span v-if="candidate.current_team && candidate.current_team.id !== memberRepairTarget?.id">
                    {{ t('admin.playOps.memberRepair.sourceSpend') }}: {{ impactLine(candidate.impact.source_spend_before, candidate.impact.source_spend_after) }}
                  </span>
                  <span v-if="candidate.current_team && candidate.current_team.id !== memberRepairTarget?.id">
                    {{ t('admin.playOps.memberRepair.sourcePool') }}: {{ impactLine(candidate.impact.source_pool_before, candidate.impact.source_pool_after) }}
                  </span>
                </div>
                <div v-if="candidate.blockers?.length" class="mt-3 space-y-1 text-xs text-red-600 dark:text-red-300">
                  <p v-for="code in candidate.blockers" :key="code">{{ memberRepairBlockerLabel(code) }}</p>
                </div>
                <div v-if="candidate.warnings?.length" class="mt-3 space-y-1 text-xs text-amber-700 dark:text-amber-300">
                  <p v-for="code in candidate.warnings" :key="code">{{ memberRepairWarningLabel(code) }}</p>
                </div>
              </button>
            </div>
          </div>

          <label>
            <span class="input-label">{{ t('admin.playOps.memberRepair.reason') }}</span>
            <textarea
              v-model="memberRepairReason"
              rows="4"
              maxlength="500"
              class="input"
              data-testid="member-repair-reason"
              :placeholder="t('admin.playOps.memberRepair.reasonPlaceholder')"
            ></textarea>
            <span class="input-hint">{{ t('admin.playOps.memberRepair.reasonCount', { count: memberRepairReasonLength }) }}</span>
          </label>
        </div>

        <template #footer>
          <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <button type="button" class="btn btn-secondary" :disabled="memberRepairSubmitting" @click="closeMemberRepair">
              {{ t('admin.playOps.memberRepair.cancel') }}
            </button>
            <button
              type="button"
              class="btn btn-primary inline-flex items-center justify-center gap-2"
              data-testid="confirm-member-repair"
              :disabled="!canSubmitMemberRepair"
              @click="prepareMemberRepair"
            >
              <Icon name="check" size="sm" />
              {{ t('admin.playOps.memberRepair.review') }}
            </button>
          </div>
        </template>
      </BaseDialog>

      <BaseDialog
        :show="memberRepairConfirmOpen"
        :title="t('admin.playOps.memberRepair.confirmTitle')"
        width="narrow"
        :show-close-button="false"
        :close-on-escape="!memberRepairSubmitting"
        :z-index="60"
        @close="cancelMemberRepairConfirmation"
      >
        <p class="text-sm text-gray-700 dark:text-gray-200">{{ memberRepairConfirmationMessage }}</p>

        <template #footer>
          <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <button
              type="button"
              class="btn btn-secondary"
              data-testid="cancel-member-repair-confirm"
              :disabled="memberRepairSubmitting"
              @click="cancelMemberRepairConfirmation"
            >
              {{ t('admin.playOps.memberRepair.cancel') }}
            </button>
            <button
              type="button"
              class="btn btn-primary inline-flex items-center justify-center gap-2"
              data-testid="execute-member-repair"
              :disabled="memberRepairSubmitting"
              @click="executeMemberRepair"
            >
              <Icon name="check" size="sm" />
              {{ memberRepairSubmitting ? t('admin.playOps.memberRepair.submitting') : t('admin.playOps.memberRepair.confirm') }}
            </button>
          </div>
        </template>
      </BaseDialog>
      <TotpStepUpDialog :controller="memberRepairStepUp" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import adminPlayAPI, {
  type AdminArenaLeaderboard,
  type AdminPlayCampaign,
  type AdminPlayCampaignInput,
  type AdminPlayOpsSummary,
  type AdminPlayTeamDetail,
  type AdminPlayTeamList,
  type AdminTeamEvent,
  type AdminTeamMemberCandidate,
  type AdminTeamMemberOperation,
  type AdminTeamMemberRepairInput,
} from '@/api/admin/play'
import { useAppStore } from '@/stores'
import { extractApiErrorCode } from '@/utils/apiError'
import { isStepUpCancelled, useStepUp } from '@/composables/useStepUp'

const { t, locale } = useI18n()
const appStore = useAppStore()

interface CampaignFormState {
  id?: number
  name: string
  nameZh: string
  nameEn: string
  startAt: string
  endAt: string
  enabled: boolean
  rechargeBonusPct: string
  blindboxExtraOpens: string
  arenaScoreMultiplier: string
}

const loading = ref(false)
const campaignSaving = ref(false)
const campaignDeletingId = ref<number | null>(null)
const campaignFormOpen = ref(false)
const arenaPeriodType = ref<'daily' | 'monthly'>('monthly')
const arena = ref<AdminArenaLeaderboard | null>(null)
const campaigns = ref<AdminPlayCampaign[]>([])
const teams = ref<AdminPlayTeamList>({ items: [], total: 0, page: 1, page_size: 20 })
const summary = ref<AdminPlayOpsSummary | null>(null)
const selectedTeam = ref<AdminPlayTeamDetail | null>(null)
const teamEvents = ref<AdminTeamEvent[]>([])
const query = ref('')
const status = ref<'active' | 'archived' | 'all'>('active')
const campaignForm = ref<CampaignFormState>(blankCampaignForm())
const memberRepairOpen = ref(false)
const memberRepairOperation = ref<AdminTeamMemberOperation>('add')
const memberRepairEffectiveAt = ref('')
const memberCandidateQuery = ref('')
const memberCandidateSearched = ref(false)
const memberCandidatesLoading = ref(false)
const memberCandidates = ref<AdminTeamMemberCandidate[]>([])
const selectedMemberCandidate = ref<AdminTeamMemberCandidate | null>(null)
const memberRepairReason = ref('')
const memberRepairSubmitting = ref(false)
const memberRepairConfirmOpen = ref(false)
const memberRepairPendingInput = ref<AdminTeamMemberRepairInput | null>(null)
const memberRepairTarget = ref<{ id: number; name: string } | null>(null)
const memberRepairPreviewEffectiveAt = ref<string | null>(null)
const memberRepairStepUp = useStepUp()
const memberRepairReferenceNow = ref(new Date())
let memberCandidateRequestVersion = 0
let teamSelectionRequestVersion = 0

const activeTabClass = 'bg-primary-600 text-white'
const idleTabClass = 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'
const memberRepairNow = computed(() => toShanghaiDateTimeLocal(memberRepairReferenceNow.value))
const memberRepairMonthStart = computed(() => `${memberRepairNow.value.slice(0, 7)}-01T00:00`)
const memberRepairReasonLength = computed(() => Array.from(memberRepairReason.value.trim()).length)
const canSubmitMemberRepair = computed(() => {
  const candidate = selectedMemberCandidate.value
  return Boolean(
    candidate
    && !candidate.blockers?.length
    && memberRepairReasonLength.value >= 10
    && memberRepairReasonLength.value <= 500
    && !memberRepairSubmitting.value,
  )
})
const memberRepairConfirmationMessage = computed(() => {
  const input = memberRepairPendingInput.value
  const candidate = selectedMemberCandidate.value
  const team = memberRepairTarget.value
  if (!input || !candidate || !team) return ''
  return t(
    `admin.playOps.memberRepair.${input.operation === 'move' ? 'confirmMove' : 'confirmAdd'}`,
    {
      user: candidate.display_name,
      team: team.name,
      effectiveAt: input.effective_at
        ? formatDateTime(input.effective_at)
        : t('admin.playOps.memberRepair.effectiveImmediately'),
    },
  )
})

const statCards = computed(() => {
  const arenaBudget = arenaPeriodType.value === 'daily'
    ? summary.value?.daily_arena_reward_budget
    : summary.value?.monthly_arena_reward_budget
  return [
    { label: t('admin.playOps.totalTeams'), value: formatNumber(summary.value?.total_teams) },
    { label: t('admin.playOps.activeTeams'), value: formatNumber(summary.value?.active_teams) },
    { label: t('admin.playOps.monthSpend'), value: formatMoney(summary.value?.month_spend) },
    { label: t('admin.playOps.estimatedPool'), value: formatMoney(summary.value?.estimated_shared_pool) },
    { label: t('admin.playOps.arenaBudget'), value: formatMoney(arenaBudget) },
    { label: t('admin.playOps.pendingFailedSettlements'), value: formatNumber(summary.value?.pending_failed_settlements) },
  ]
})

async function load() {
  loading.value = true
  try {
    const [summaryData, arenaData, campaignData, teamData] = await Promise.all([
      adminPlayAPI.getSummary(),
      adminPlayAPI.getArenaLeaderboard({ period_type: arenaPeriodType.value, limit: 20 }),
      adminPlayAPI.listCampaigns(),
      adminPlayAPI.listTeams({ status: status.value, q: query.value, page: 1, page_size: 50 }),
    ])
    summary.value = summaryData
    arena.value = arenaData
    campaigns.value = campaignData
    teams.value = teamData
    if (!selectedTeam.value && teamData.items[0]) {
      await selectTeam(teamData.items[0].id)
    }
  } catch (error) {
    appStore.showError(localizedPlayOpsError(error, t('admin.playOps.loadFailed')))
  } finally {
    loading.value = false
  }
}

function blankCampaignForm(): CampaignFormState {
  const start = new Date()
  start.setMinutes(0, 0, 0)
  const end = new Date(start)
  end.setDate(end.getDate() + 7)
  return {
    name: '',
    nameZh: '',
    nameEn: '',
    startAt: toDateTimeLocal(start.toISOString()),
    endAt: toDateTimeLocal(end.toISOString()),
    enabled: true,
    rechargeBonusPct: '',
    blindboxExtraOpens: '',
    arenaScoreMultiplier: '',
  }
}

function startCreateCampaign() {
  campaignForm.value = blankCampaignForm()
  campaignFormOpen.value = true
}

function startEditCampaign(campaign: AdminPlayCampaign) {
  campaignForm.value = {
    id: campaign.id,
    name: campaign.name,
    nameZh: campaign.rules.name_i18n?.zh || '',
    nameEn: campaign.rules.name_i18n?.en || '',
    startAt: toDateTimeLocal(campaign.start_at),
    endAt: toDateTimeLocal(campaign.end_at),
    enabled: campaign.enabled,
    rechargeBonusPct: campaign.rules.recharge_bonus_pct ? String(campaign.rules.recharge_bonus_pct) : '',
    blindboxExtraOpens: campaign.rules.blindbox_extra_opens ? String(campaign.rules.blindbox_extra_opens) : '',
    arenaScoreMultiplier: campaign.rules.arena_score_multiplier ? String(campaign.rules.arena_score_multiplier) : '',
  }
  campaignFormOpen.value = true
}

function closeCampaignForm() {
  campaignFormOpen.value = false
}

async function submitCampaign() {
  let input: AdminPlayCampaignInput
  try {
    input = buildCampaignInput(campaignForm.value)
  } catch (error) {
    appStore.showError((error as Error).message)
    return
  }

  campaignSaving.value = true
  try {
    if (campaignForm.value.id) {
      await adminPlayAPI.updateCampaign(campaignForm.value.id, input)
      appStore.showSuccess(t('admin.playOps.campaignUpdated'))
    } else {
      await adminPlayAPI.createCampaign(input)
      appStore.showSuccess(t('admin.playOps.campaignCreated'))
    }
    campaignFormOpen.value = false
    campaigns.value = await adminPlayAPI.listCampaigns()
  } catch (error) {
    appStore.showError(localizedPlayOpsError(error, t('admin.playOps.campaignSaveFailed')))
  } finally {
    campaignSaving.value = false
  }
}

async function deleteCampaign(campaign: AdminPlayCampaign) {
  if (!window.confirm(t('admin.playOps.deleteCampaignConfirm', { name: campaign.name }))) {
    return
  }
  campaignDeletingId.value = campaign.id
  try {
    await adminPlayAPI.deleteCampaign(campaign.id)
    campaigns.value = campaigns.value.filter((item) => item.id !== campaign.id)
    appStore.showSuccess(t('admin.playOps.campaignDeleted'))
  } catch (error) {
    appStore.showError(localizedPlayOpsError(error, t('admin.playOps.campaignDeleteFailed')))
  } finally {
    campaignDeletingId.value = null
  }
}

function buildCampaignInput(form: CampaignFormState): AdminPlayCampaignInput {
  const start = new Date(form.startAt)
  const end = new Date(form.endAt)
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) {
    throw new Error(t('admin.playOps.campaignTimeRequired'))
  }
  if (end <= start) {
    throw new Error(t('admin.playOps.campaignTimeInvalid'))
  }

  const nameI18n: Record<string, string> = {}
  if (form.nameZh.trim()) nameI18n.zh = form.nameZh.trim()
  if (form.nameEn.trim()) nameI18n.en = form.nameEn.trim()

  const rules = {
    recharge_bonus_pct: parseOptionalNumber(form.rechargeBonusPct),
    blindbox_extra_opens: parseOptionalInteger(form.blindboxExtraOpens),
    arena_score_multiplier: parseOptionalNumber(form.arenaScoreMultiplier),
    name_i18n: Object.keys(nameI18n).length ? nameI18n : undefined,
  }

  return {
    name: form.name.trim(),
    start_at: start.toISOString(),
    end_at: end.toISOString(),
    enabled: form.enabled,
    rules,
  }
}

function parseOptionalNumber(value: string | number | undefined): number | undefined {
  const raw = String(value ?? '').trim()
  if (!raw) return undefined
  const numberValue = Number(raw)
  return Number.isFinite(numberValue) ? numberValue : undefined
}

function parseOptionalInteger(value: string | number | undefined): number | undefined {
  const numberValue = parseOptionalNumber(value)
  return numberValue === undefined ? undefined : Math.trunc(numberValue)
}

function campaignStatus(campaign: AdminPlayCampaign): 'active' | 'upcoming' | 'ended' | 'disabled' {
  if (!campaign.enabled) return 'disabled'
  const now = Date.now()
  const start = new Date(campaign.start_at).getTime()
  const end = new Date(campaign.end_at).getTime()
  if (now < start) return 'upcoming'
  if (now >= end) return 'ended'
  return 'active'
}

function campaignStatusClass(campaign: AdminPlayCampaign) {
  const status = campaignStatus(campaign)
  if (status === 'active') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
  if (status === 'upcoming') return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200'
  if (status === 'ended') return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
}

function campaignRuleLines(campaign: AdminPlayCampaign) {
  const rules = campaign.rules || {}
  const lines: string[] = []
  if (rules.recharge_bonus_pct) lines.push(t('admin.playOps.ruleRechargeBonus', { pct: rules.recharge_bonus_pct }))
  if (rules.blindbox_extra_opens) lines.push(t('admin.playOps.ruleBlindboxExtra', { count: rules.blindbox_extra_opens }))
  if (rules.arena_score_multiplier) lines.push(t('admin.playOps.ruleArenaMultiplier', { mult: rules.arena_score_multiplier }))
  return lines
}

async function loadTeams() {
  try {
    teams.value = await adminPlayAPI.listTeams({ status: status.value, q: query.value, page: 1, page_size: 50 })
    if (selectedTeam.value && !teams.value.items.some((team) => team.id === selectedTeam.value?.team.id)) {
      selectedTeam.value = null
    }
  } catch (error) {
    appStore.showError(localizedPlayOpsError(error, t('admin.playOps.loadFailed')))
  }
}

async function switchArena(periodType: 'daily' | 'monthly') {
  const previousPeriodType = arenaPeriodType.value
  arenaPeriodType.value = periodType
  try {
    arena.value = await adminPlayAPI.getArenaLeaderboard({ period_type: periodType, limit: 20 })
  } catch (error) {
    arenaPeriodType.value = previousPeriodType
    appStore.showError(localizedPlayOpsError(error, t('admin.playOps.loadFailed')))
  }
}

async function selectTeam(id: number) {
  const requestVersion = ++teamSelectionRequestVersion
  let detail: AdminPlayTeamDetail
  try {
    detail = await adminPlayAPI.getTeam(id)
  } catch (error) {
    if (requestVersion === teamSelectionRequestVersion) {
      appStore.showError(localizedPlayOpsError(error, t('admin.playOps.loadFailed')))
    }
    return
  }
  if (requestVersion !== teamSelectionRequestVersion) return
  selectedTeam.value = detail
  teamEvents.value = []
  try {
    const events = await adminPlayAPI.listTeamEvents(id)
    if (requestVersion !== teamSelectionRequestVersion) return
    teamEvents.value = events
  } catch (error) {
    if (requestVersion !== teamSelectionRequestVersion) return
    appStore.showError(localizedPlayOpsError(error, t('admin.playOps.eventsLoadFailed')))
  }
}

function formatNumber(value: number | string | undefined) {
  return Number(value || 0).toLocaleString(locale.value)
}

function formatMoney(value: number | string | undefined) {
  return `$${Number(value || 0).toFixed(2)}`
}

function formatDateTime(value: string | undefined) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString(locale.value, { timeZone: 'Asia/Shanghai' })
}

function settlementStatusLabel(statusValue: string) {
  const known = ['pending', 'processing', 'completed', 'partial', 'failed']
  return known.includes(statusValue)
    ? t(`admin.playOps.settlementStatus.${statusValue}`)
    : t('admin.playOps.statusUnknown')
}

function payoutStatusLabel(statusValue: string) {
  const known = ['pending', 'processing', 'paid', 'failed']
  return known.includes(statusValue)
    ? t(`admin.playOps.payoutStatus.${statusValue}`)
    : t('admin.playOps.statusUnknown')
}

function eventLabel(eventType: string) {
  const known = [
    'team_created',
    'member_joined',
    'member_left',
    'captain_transferred',
    'member_removed',
    'team_archived',
    'admin_member_added',
    'admin_member_moved',
  ]
  return known.includes(eventType)
    ? t(`admin.playOps.events.${eventType}`)
    : t('admin.playOps.eventUnknown')
}

function eventReason(event: AdminTeamEvent) {
  const parts: string[] = []
  const reasonCode = typeof event.detail?.reason_code === 'string'
    ? event.detail.reason_code
    : ''
  if (['last_member_left', 'admin_moved_last_captain', 'admin_manual_membership_repair'].includes(reasonCode)) {
    parts.push(t(`admin.playOps.eventReasons.${reasonCode}`))
  }
  const repairReason = typeof event.detail?.reason === 'string'
    ? event.detail.reason.trim()
    : ''
  if (repairReason) parts.push(repairReason)
  return parts.join(' · ')
}

function eventEffectiveAtLabel(event: AdminTeamEvent) {
  const effectiveAt = event.detail?.effective_at
  if (typeof effectiveAt !== 'string' || !effectiveAt) return ''
  return t('admin.playOps.eventEffectiveAt', { time: formatDateTime(effectiveAt) })
}

function eventTeamTransitionLabel(event: AdminTeamEvent) {
  const source = event.detail?.source_team_id
  const target = event.detail?.target_team_id
  if (
    typeof source !== 'number'
    || !Number.isSafeInteger(source)
    || source <= 0
    || typeof target !== 'number'
    || !Number.isSafeInteger(target)
    || target <= 0
  ) {
    return ''
  }
  return t('admin.playOps.eventTeamTransition', { source, target })
}

function memberUserStatusLabel(statusValue: string) {
  return ['active', 'disabled', 'deleted'].includes(statusValue)
    ? t(`admin.playOps.memberRepair.userStatuses.${statusValue}`)
    : t('admin.playOps.memberRepair.userStatuses.unknown')
}

function memberRepairBlockerLabel(code: string) {
  const key = `admin.playOps.memberRepair.blockers.${code}`
  const value = t(key)
  return value === key ? t('admin.playOps.memberRepair.unknownBlocker') : value
}

function memberRepairWarningLabel(code: string) {
  const key = `admin.playOps.memberRepair.warnings.${code}`
  const value = t(key)
  return value === key ? t('admin.playOps.memberRepair.unknownWarning') : value
}

function impactLine(before: string | number | undefined, after: string | number | undefined) {
  return `${formatMoney(before)} → ${formatMoney(after)}`
}

function openMemberRepair() {
  const team = selectedTeam.value?.team
  if (!team) return
  memberRepairTarget.value = { id: team.id, name: team.name }
  memberRepairReferenceNow.value = new Date()
  memberRepairOperation.value = 'add'
  memberRepairEffectiveAt.value = ''
  memberCandidateQuery.value = ''
  memberRepairReason.value = ''
  memberRepairConfirmOpen.value = false
  memberRepairPendingInput.value = null
  clearMemberRepairPreview()
  memberRepairOpen.value = true
}

function closeMemberRepair() {
  if (memberRepairSubmitting.value) return
  memberRepairOpen.value = false
  memberRepairConfirmOpen.value = false
  memberRepairPendingInput.value = null
  clearMemberRepairPreview()
  memberRepairTarget.value = null
}

function changeMemberRepairOperation(operation: AdminTeamMemberOperation) {
  if (memberRepairOperation.value === operation) return
  memberRepairOperation.value = operation
  clearMemberRepairPreview()
}

function clearMemberRepairPreview() {
  memberCandidateRequestVersion += 1
  memberCandidatesLoading.value = false
  memberCandidateSearched.value = false
  memberCandidates.value = []
  selectedMemberCandidate.value = null
  memberRepairPreviewEffectiveAt.value = null
}

async function searchMemberCandidates() {
  const target = memberRepairTarget.value
  if (!target || !memberCandidateQuery.value.trim()) return
  clearMemberRepairPreview()
  const requestVersion = memberCandidateRequestVersion
  memberCandidatesLoading.value = true
  try {
    const data = await adminPlayAPI.listTeamMemberCandidates(target.id, {
      q: memberCandidateQuery.value.trim(),
      operation: memberRepairOperation.value,
      effective_at: memberRepairEffectiveAt.value
        ? shanghaiInputToISO(memberRepairEffectiveAt.value)
        : undefined,
    })
    if (requestVersion !== memberCandidateRequestVersion) return
    if (!data.effective_at || Number.isNaN(new Date(data.effective_at).getTime())) {
      throw new Error('invalid member repair preview effective time')
    }
    memberCandidates.value = data.items || []
    memberRepairPreviewEffectiveAt.value = data.effective_at
    memberCandidateSearched.value = true
  } catch (error) {
    if (requestVersion !== memberCandidateRequestVersion) return
    appStore.showError(localizedPlayOpsError(
      error,
      t('admin.playOps.memberRepair.loadFailed'),
    ))
  } finally {
    if (requestVersion === memberCandidateRequestVersion) {
      memberCandidatesLoading.value = false
    }
  }
}

function prepareMemberRepair() {
  if (!memberRepairTarget.value || !selectedMemberCandidate.value || !memberRepairPreviewEffectiveAt.value) {
    appStore.showError(t('admin.playOps.memberRepair.candidateRequired'))
    return
  }
  if (memberRepairReasonLength.value < 10 || memberRepairReasonLength.value > 500) {
    appStore.showError(t('admin.playOps.memberRepair.reasonInvalid'))
    return
  }
  const candidate = selectedMemberCandidate.value
  const input: AdminTeamMemberRepairInput = {
    user_id: candidate.user_id,
    operation: memberRepairOperation.value,
    ...(memberRepairEffectiveAt.value
      ? { effective_at: memberRepairPreviewEffectiveAt.value }
      : {}),
    reason: memberRepairReason.value.trim(),
    expected_source_team_id: memberRepairOperation.value === 'move'
      ? candidate.current_team?.id
      : undefined,
  }
  memberRepairPendingInput.value = input
  memberRepairOpen.value = false
  memberRepairConfirmOpen.value = true
}

function cancelMemberRepairConfirmation() {
  if (memberRepairSubmitting.value) return
  memberRepairConfirmOpen.value = false
  memberRepairPendingInput.value = null
  memberRepairOpen.value = true
}

async function executeMemberRepair() {
  if (!memberRepairTarget.value || !memberRepairPendingInput.value) return
  const teamID = memberRepairTarget.value.id
  const input = memberRepairPendingInput.value
  memberRepairConfirmOpen.value = false
  memberRepairSubmitting.value = true
  let succeeded = false
  try {
    const result = await memberRepairStepUp.run(
      () => adminPlayAPI.repairTeamMember(teamID, input),
    )
    const successKey = result.status === 'moved'
      ? 'successMoved'
      : result.status === 'no_op'
        ? 'successNoOp'
        : 'successAdded'
    appStore.showSuccess(t(`admin.playOps.memberRepair.${successKey}`))
    memberRepairOpen.value = false
    succeeded = true
  } catch (error) {
    memberRepairOpen.value = true
    if (!isStepUpCancelled(error)) {
      appStore.showError(localizedPlayOpsError(
        error,
        t('admin.playOps.memberRepair.submitFailed'),
      ))
    }
  } finally {
    memberRepairSubmitting.value = false
    memberRepairPendingInput.value = null
  }
  if (!succeeded) return
  memberRepairTarget.value = null
  try {
    await Promise.all([selectTeam(teamID), loadTeams()])
  } catch (error) {
    appStore.showError(localizedPlayOpsError(error, t('admin.playOps.loadFailed')))
  }
}

function shanghaiInputToISO(value: string) {
  return new Date(`${value}:00+08:00`).toISOString()
}

function localizedPlayOpsError(error: unknown, fallback: string) {
  const code = extractApiErrorCode(error)
  if (!code) return fallback
  const key = `admin.playOps.errors.${code}`
  const translated = t(key)
  return translated === key ? fallback : translated
}

function toShanghaiDateTimeLocal(value: Date) {
  return new Date(value.getTime() + 8 * 60 * 60 * 1000).toISOString().slice(0, 16)
}

function toDateTimeLocal(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const offsetMs = date.getTimezoneOffset() * 60 * 1000
  return new Date(date.getTime() - offsetMs).toISOString().slice(0, 16)
}

onMounted(load)
</script>
