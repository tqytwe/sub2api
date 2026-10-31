<template>
  <AppLayout>
    <div class="min-w-0 space-y-5">
      <header class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.qualityOps.title') }}</h1>
          <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">{{ t('admin.qualityOps.description') }}</p>
        </div>
        <button class="btn btn-secondary" :disabled="loading" @click="load">
          <!-- design-governance-allow: continuous-motion - refresh feedback is user-triggered, stops with the request, and honors reduced motion. -->
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin motion-reduce:animate-none' : ''" />
          {{ t('common.refresh') }}
        </button>
      </header>

      <p v-if="error" role="alert" class="border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:border-red-900 dark:bg-red-900/20 dark:text-red-200">
        {{ error }}
      </p>
      <p v-if="notice" role="status" class="border border-green-300 bg-green-50 p-3 text-sm text-green-800 dark:border-green-900 dark:bg-green-900/20 dark:text-green-200">
        {{ notice }}
      </p>

      <section class="border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <header class="border-b border-gray-100 p-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.qualityOps.globalHistory') }}</h2>
        </header>
        <div v-if="globalHistoryLoading" class="p-5 text-sm text-gray-500">{{ t('common.loading') }}...</div>
        <p v-else-if="globalHistoryError" class="p-5 text-sm text-red-700 dark:text-red-300">{{ globalHistoryError }}</p>
        <p v-else-if="!globalHistory.length" class="p-5 text-sm text-gray-500">{{ t('admin.qualityOps.globalHistoryEmpty') }}</p>
        <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <div v-for="item in globalHistory" :key="item.id" class="flex flex-wrap items-center justify-between gap-3 p-4 text-sm">
            <div>
              <p class="font-medium text-gray-900 dark:text-white">{{ item.account_name }} #{{ item.account_id }} · {{ item.pelican_config?.rule_name || t('admin.qualityOps.unnamed') }}</p>
              <p class="mt-1 text-xs text-gray-500">{{ item.status }} · {{ item.passed_count }}/{{ item.total_count }} {{ t('admin.qualityOps.passedCount') }}<template v-if="item.skipped_count"> · {{ t('admin.qualityOps.skippedCount', { count: item.skipped_count }) }}</template> · {{ formatDate(item.started_at) }}</p>
            </div>
            <span v-if="item.error_message" class="text-xs text-red-700 dark:text-red-300">{{ item.error_message }}</span>
          </div>
          <div v-if="globalHistoryCursor > 0" class="p-4 text-center">
            <button class="btn btn-secondary" :disabled="globalHistoryMoreLoading" @click="loadMoreGlobalHistory">
              {{ globalHistoryMoreLoading ? t('common.loading') : t('admin.qualityOps.loadMoreHistory') }}
            </button>
          </div>
        </div>
      </section>

      <section class="border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800" data-testid="quality-templates">
        <header class="flex flex-wrap items-start justify-between gap-3 border-b border-gray-100 p-4 dark:border-dark-700">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.qualityOps.templates.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.qualityOps.templates.description') }}</p>
          </div>
          <button class="btn btn-secondary" type="button" data-testid="quality-template-new" @click="newTemplate">{{ t('admin.qualityOps.templates.create') }}</button>
        </header>
        <form v-if="templateFormOpen" class="grid gap-3 border-b border-gray-100 p-4 dark:border-dark-700 sm:grid-cols-2" data-testid="quality-template-form" @submit.prevent="saveTemplate">
          <label class="block text-sm"><span class="field-title">{{ t('admin.qualityOps.templates.group') }}</span><input v-model.trim="templateForm.group" class="input w-full" :placeholder="t('admin.qualityOps.templates.groupPlaceholder')" /></label>
          <label class="block text-sm"><span class="field-title">{{ t('admin.qualityOps.templates.type') }}</span><input v-model.trim="templateForm.type" class="input w-full" placeholder="oauth" /></label>
          <label class="block text-sm"><span class="field-title">{{ t('admin.qualityOps.templates.statuses') }}</span><input v-model.trim="templateForm.statuses" class="input w-full" placeholder="active, disabled" /></label>
          <label class="block text-sm"><span class="field-title">{{ t('admin.qualityOps.templates.search') }}</span><input v-model.trim="templateForm.search" class="input w-full" /></label>
          <label class="block text-sm"><span class="field-title">{{ t('admin.qualityOps.model') }}</span><input v-model.trim="templateForm.model_id" class="input w-full" data-testid="quality-template-model-id" required maxlength="100" /></label>
          <label class="block text-sm"><span class="field-title">{{ t('admin.qualityOps.schedule') }}</span><input v-model.trim="templateForm.cron_expression" class="input w-full" required /></label>
          <fieldset class="min-w-0 space-y-2 sm:col-span-2" data-testid="quality-template-extra-models">
            <legend class="field-title">{{ t('admin.qualityOps.additionalModels') }}</legend>
            <div v-for="(_, index) in templateForm.extra_models" :key="index" class="flex min-w-0 gap-2">
              <input v-model.trim="templateForm.extra_models[index]" class="input min-w-0 flex-1" data-testid="quality-template-extra-model" :aria-label="t('admin.qualityOps.modelNumber', { number: index + 2 })" required maxlength="100" />
              <button class="btn btn-secondary" type="button" :aria-label="t('admin.qualityOps.removeModel', { number: index + 2 })" :disabled="templateSaving" @click="templateForm.extra_models.splice(index, 1)"><Icon name="x" size="sm" /></button>
            </div>
            <button class="btn btn-secondary btn-sm" type="button" data-testid="quality-template-add-model" :disabled="templateSaving || templateForm.extra_models.length >= 49" @click="templateForm.extra_models.push('')"><Icon name="plus" size="sm" />{{ t('admin.qualityOps.addModel') }}</button>
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.qualityOps.modelsHelp') }}</p>
          </fieldset>
          <label class="block text-sm"><span class="field-title">{{ t('admin.qualityOps.maxResults') }}</span><input v-model.number="templateForm.max_results" class="input w-full" type="number" min="1" required /></label>
          <label class="block text-sm"><span class="field-title">{{ t('admin.qualityOps.probesPerModel') }}</span><input v-model.number="templateForm.parallel_count" class="input w-full" data-testid="quality-template-parallel-count" type="number" min="1" max="32" step="1" required /><span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.qualityOps.probesHelp') }}</span></label>
          <label class="block text-sm sm:col-span-2"><span class="field-title">{{ t('admin.qualityOps.prompt') }}</span><textarea v-model="templateForm.prompt" class="input min-h-20 w-full font-mono text-sm" required maxlength="32768" /></label>
          <label class="block text-sm"><span class="field-title">{{ t('admin.qualityOps.expectedAnswer') }}</span><input v-model.trim="templateForm.expected_answer" class="input w-full" data-testid="quality-template-expected-answer" required maxlength="4000" /></label>
          <label class="block text-sm"><span class="field-title">{{ t('admin.qualityOps.action') }}</span><select v-model="templateForm.action" class="input w-full"><option value="observe_only">{{ t('admin.qualityOps.observeOnly') }}</option><option value="disable_scheduling">{{ t('admin.qualityOps.disableScheduling') }}</option></select></label>
          <div class="sm:col-span-2">
            <Toggle v-model="templateForm.auto_restore" data-testid="quality-template-auto-restore" :label="t('admin.qualityOps.autoRestore')" :description="t('admin.qualityOps.autoRestoreHelp')" :disabled="templateSaving" />
          </div>
          <label class="flex items-center gap-2 text-sm sm:col-span-2"><input v-model="templateForm.enabled" type="checkbox" />{{ t('admin.qualityOps.enabled') }}</label>
          <div class="flex justify-end gap-2 sm:col-span-2"><button class="btn btn-secondary" type="button" @click="closeTemplateForm">{{ t('common.cancel') }}</button><button class="btn btn-primary" type="submit" :disabled="templateSaving">{{ templateSaving ? t('common.saving') : t('common.save') }}</button></div>
        </form>
        <div v-if="templateLoading" class="p-6 text-sm text-gray-500">{{ t('common.loading') }}...</div>
        <p v-else-if="!templates.length" class="p-6 text-sm text-gray-500">{{ t('admin.qualityOps.templates.empty') }}</p>
        <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <article v-for="template in templates" :key="template.id" class="p-4" data-testid="quality-template-row">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div class="min-w-0">
                <h3 class="font-medium text-gray-900 dark:text-white">{{ templateFilterSummary(template) }}</h3>
                <p class="mt-1 break-words text-xs text-gray-500">{{ modelSummary(template.model_id, template.pelican_config) }} · {{ template.cron_expression }} · {{ t('admin.qualityOps.templates.covered', { count: template.plan_ids.length }) }}</p>
                <p class="mt-1 text-xs text-gray-500">{{ t('admin.qualityOps.templates.createdUpdated', { created: formatDate(template.created_at), updated: formatDate(template.updated_at) }) }}</p>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-xs" :class="template.enabled ? 'text-green-700 dark:text-green-300' : 'text-gray-500'">{{ t(template.enabled ? 'admin.qualityOps.enabled' : 'admin.qualityOps.paused') }}</span>
                <button class="btn btn-secondary btn-sm" type="button" :disabled="templateBusyId === template.id" @click="editTemplate(template)">{{ t('admin.qualityOps.editShort') }}</button>
                <button class="btn btn-secondary btn-sm" type="button" :disabled="templateBusyId === template.id" @click="toggleTemplate(template)">{{ t(template.enabled ? 'admin.qualityOps.templates.pause' : 'admin.qualityOps.templates.enable') }}</button>
                <button class="btn btn-secondary btn-sm" type="button" :disabled="templateBusyId === template.id" @click="deleteTemplateRecord(template, false)">{{ t('admin.qualityOps.templates.deleteKeep') }}</button>
                <button class="btn btn-danger btn-sm" type="button" :disabled="templateBusyId === template.id" @click="deleteTemplateRecord(template, true)">{{ t('admin.qualityOps.templates.deletePlans') }}</button>
              </div>
            </div>
          </article>
        </div>
      </section>

      <section class="border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
        <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ editingId ? t('admin.qualityOps.edit') : t('admin.qualityOps.create') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.qualityOps.scope') }}</p>
          </div>
          <button v-if="editingId" class="btn btn-secondary" type="button" @click="resetForm">{{ t('common.cancel') }}</button>
        </div>
        <form class="grid gap-4 sm:grid-cols-2" @submit.prevent="save">
          <div v-if="!editingId" class="grid gap-2 border border-gray-200 p-3 dark:border-dark-600 sm:col-span-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
            <label class="block text-sm">
              <span class="field-title">{{ t('admin.qualityOps.savedPresets') }}</span>
              <select v-model.number="selectedPresetId" class="input w-full" data-testid="quality-preset-select" :disabled="presetLoading || !presets.length">
                <option :value="0">{{ t(presetLoading ? 'admin.qualityOps.presetsLoading' : 'admin.qualityOps.selectPreset') }}</option>
                <option v-for="preset in presets" :key="preset.id" :value="preset.id">{{ preset.name }}</option>
              </select>
            </label>
            <button class="btn btn-secondary" type="button" data-testid="quality-apply-preset" :disabled="!selectedPresetId" @click="applyPreset">{{ t('admin.qualityOps.applyPreset') }}</button>
            <p v-if="presetError" class="text-xs text-red-700 dark:text-red-300 sm:col-span-2" role="alert">
              {{ presetError }}
              <button type="button" class="underline" @click="loadPresets">{{ t('admin.qualityOps.retryPresets') }}</button>
            </p>
          </div>
          <label class="block text-sm sm:col-span-2">
            <span class="field-title">{{ t('admin.qualityOps.account') }}</span>
            <select v-model.number="form.account_id" class="input w-full" :disabled="!!editingId" required>
              <option :value="0" disabled>{{ t('admin.qualityOps.selectAccount') }}</option>
              <option v-for="account in accounts" :key="account.id" :value="account.id">{{ account.name }} #{{ account.id }}</option>
            </select>
          </label>
          <label class="block text-sm">
            <span class="field-title">{{ t('admin.qualityOps.model') }}</span>
            <input v-model.trim="form.model_id" class="input w-full" data-testid="quality-model-id" required maxlength="100" placeholder="gpt-5.1" />
          </label>
          <label class="block text-sm">
            <span class="field-title">{{ t('admin.qualityOps.schedule') }}</span>
            <input v-model.trim="form.cron_expression" class="input w-full" data-testid="quality-cron-expression" required placeholder="*/30 * * * *" />
          </label>
          <fieldset class="min-w-0 space-y-2 sm:col-span-2" data-testid="quality-extra-models">
            <legend class="field-title">{{ t('admin.qualityOps.additionalModels') }}</legend>
            <div v-for="(_, index) in form.extra_models" :key="index" class="flex min-w-0 gap-2">
              <input v-model.trim="form.extra_models[index]" class="input min-w-0 flex-1" data-testid="quality-extra-model" :aria-label="t('admin.qualityOps.modelNumber', { number: index + 2 })" required maxlength="100" />
              <button class="btn btn-secondary" type="button" :aria-label="t('admin.qualityOps.removeModel', { number: index + 2 })" :disabled="saving || presetSaving" @click="form.extra_models.splice(index, 1)"><Icon name="x" size="sm" /></button>
            </div>
            <button class="btn btn-secondary btn-sm" type="button" data-testid="quality-add-model" :disabled="saving || presetSaving || form.extra_models.length >= 49" @click="form.extra_models.push('')"><Icon name="plus" size="sm" />{{ t('admin.qualityOps.addModel') }}</button>
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.qualityOps.modelsHelp') }}</p>
          </fieldset>
          <label class="block text-sm">
            <span class="field-title">{{ t('admin.qualityOps.maxResults') }}</span>
            <input v-model.number="form.max_results" class="input w-full" data-testid="quality-max-results" type="number" min="1" required />
          </label>
          <label class="block text-sm">
            <span class="field-title">{{ t('admin.qualityOps.probesPerModel') }}</span>
            <input v-model.number="form.parallel_count" class="input w-full" data-testid="quality-parallel-count" type="number" min="1" max="32" step="1" required />
            <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.qualityOps.probesHelp') }}</span>
          </label>
          <label class="block text-sm sm:col-span-2">
            <span class="field-title">{{ t('admin.qualityOps.ruleName') }}</span>
            <input v-model.trim="form.rule_name" class="input w-full" data-testid="quality-rule-name-input" maxlength="120" />
          </label>
          <label class="block text-sm sm:col-span-2">
            <span class="field-title">{{ t('admin.qualityOps.prompt') }}</span>
            <textarea v-model="form.prompt" class="input min-h-24 w-full font-mono text-sm" maxlength="32768" required />
          </label>
          <label class="block text-sm">
            <span class="field-title">{{ t('admin.qualityOps.expectedAnswer') }}</span>
            <input v-model.trim="form.expected_answer" class="input w-full" data-testid="quality-expected-answer" maxlength="4000" required />
          </label>
          <label class="block text-sm">
            <span class="field-title">{{ t('admin.qualityOps.judge') }}</span>
            <select v-model="form.judge_kind" class="input w-full">
              <option value="expected">{{ t('admin.qualityOps.expectedJudge') }}</option>
              <option value="jev">{{ t('admin.qualityOps.jevJudge') }}</option>
            </select>
          </label>
          <label v-if="form.judge_kind === 'jev'" class="block text-sm sm:col-span-2">
            <span class="field-title">{{ t('admin.qualityOps.judgePrompt') }}</span>
            <textarea v-model="form.judge_prompt" class="input min-h-20 w-full font-mono text-sm" maxlength="16000" required />
            <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.qualityOps.jevJudgeHelp') }}</span>
          </label>
          <label class="block text-sm">
            <span class="field-title">{{ t('admin.qualityOps.action') }}</span>
            <select v-model="form.action" class="input w-full">
              <option value="observe_only">{{ t('admin.qualityOps.observeOnly') }}</option>
              <option value="disable_scheduling">{{ t('admin.qualityOps.disableScheduling') }}</option>
            </select>
          </label>
          <div class="sm:col-span-2">
            <Toggle v-model="form.auto_restore" data-testid="quality-auto-restore" :label="t('admin.qualityOps.autoRestore')" :description="t('admin.qualityOps.autoRestoreHelp')" :disabled="saving || presetSaving" />
          </div>
          <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 sm:col-span-2">
            <input v-model="form.enabled" type="checkbox" />
            {{ t('admin.qualityOps.enabled') }}
          </label>
          <div class="border border-gray-200 p-3 dark:border-dark-600 sm:col-span-2">
            <button class="text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400" type="button" data-testid="quality-toggle-save-preset" @click="showPresetSave = !showPresetSave">
              {{ t('admin.qualityOps.saveAsPreset') }}
            </button>
            <div v-if="showPresetSave" class="mt-3 flex flex-wrap items-end gap-2">
              <label class="block min-w-48 flex-1 text-sm">
                <span class="field-title">{{ t('admin.qualityOps.presetName') }}</span>
                <input v-model.trim="presetName" class="input w-full" maxlength="100" data-testid="quality-preset-name" />
              </label>
              <button class="btn btn-secondary" type="button" data-testid="quality-save-preset" :disabled="presetSaving || !presetName.trim() || !form.model_id.trim() || !form.cron_expression.trim()" @click="savePreset">
                {{ presetSaving ? t('common.saving') : t('admin.qualityOps.savePreset') }}
              </button>
            </div>
            <p v-if="presetSaveError" class="mt-2 text-xs text-red-700 dark:text-red-300" role="alert">{{ presetSaveError }}</p>
          </div>
          <div class="flex justify-end sm:col-span-2">
            <button class="btn btn-primary" type="submit" :disabled="saving || !form.account_id">
              {{ saving ? t('common.saving') : t('common.save') }}
            </button>
          </div>
        </form>
      </section>

      <section class="border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <header class="border-b border-gray-100 p-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.qualityOps.rules') }}</h2>
        </header>
        <div v-if="loading" class="p-8 text-center text-sm text-gray-500">{{ t('common.loading') }}...</div>
        <div v-else-if="!plans.length" class="p-8 text-center text-sm text-gray-500">{{ t('admin.qualityOps.empty') }}</div>
        <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <article v-for="plan in plans" :key="plan.id" class="p-4">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div class="min-w-0">
                <h3 class="font-medium text-gray-900 dark:text-white">{{ accountName(plan.account_id) }} #{{ plan.account_id }}</h3>
                <p class="mt-1 text-sm text-gray-600 dark:text-gray-300">{{ plan.pelican_config?.rule_name || t('admin.qualityOps.unnamed') }}</p>
                <p class="mt-1 break-words text-xs text-gray-500">{{ modelSummary(plan.model_id, plan.pelican_config) }} · {{ plan.cron_expression }} · {{ plan.enabled ? t('admin.qualityOps.enabled') : t('admin.qualityOps.paused') }}</p>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <button class="btn btn-secondary btn-sm" :disabled="busyPlan === plan.id" @click="edit(plan)">{{ t('admin.qualityOps.editShort') }}</button>
                <button class="btn btn-secondary btn-sm" :disabled="busyPlan === plan.id || !plan.enabled" @click="run(plan)">{{ busyPlan === plan.id ? t('admin.qualityOps.running') : t('admin.qualityOps.run') }}</button>
                <button class="btn btn-secondary btn-sm" :disabled="busyPlan === plan.id" @click="toggleHistory(plan)">{{ t('admin.qualityOps.history') }}</button>
                <button class="btn btn-secondary btn-sm" :disabled="busyPlan === plan.id" @click="preview(plan)">{{ t('admin.qualityOps.ownership') }}</button>
              </div>
            </div>
            <div v-if="ownershipPlanId === plan.id" class="mt-3 border border-emerald-200 bg-emerald-50/50 p-3 text-sm dark:border-emerald-900/50 dark:bg-emerald-900/10">
              <p v-if="ownershipLoading" class="text-gray-500">{{ t('common.loading') }}...</p>
              <template v-else-if="ownership">
                <div class="flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-700 dark:text-gray-300">
                  <span>{{ t('admin.qualityOps.action') }}: {{ ownership.action || '-' }}</span>
                  <span>{{ t('admin.qualityOps.ownedState') }}: {{ ownership.state_present ? t('admin.qualityOps.present') : t('admin.qualityOps.emptyState') }}</span>
                  <code>{{ ownership.rule_version }}</code>
                </div>
                <div class="mt-3 flex flex-wrap gap-2">
                  <button v-if="ownership.reconcile_ready" class="btn btn-secondary btn-sm" @click="applyOwnership('reconcile', plan)">{{ t('admin.qualityOps.reconcile') }}</button>
                  <button v-if="ownership.restore_ready" class="btn btn-secondary btn-sm" @click="applyOwnership('restore', plan)">{{ t('admin.qualityOps.restore') }}</button>
                  <button v-if="!ownership.state_present" class="btn btn-danger btn-sm" @click="applyOwnership('delete', plan)">{{ t('admin.qualityOps.delete') }}</button>
                </div>
              </template>
              <p v-if="ownershipError" class="mt-2 text-xs text-red-700 dark:text-red-300">{{ ownershipError }}</p>
            </div>
            <div v-if="historyPlanId === plan.id" class="mt-3 border-t border-gray-100 pt-3 dark:border-dark-700">
              <p v-if="historyLoading" class="text-sm text-gray-500">{{ t('common.loading') }}...</p>
              <p v-else-if="!history.length" class="text-sm text-gray-500">{{ t('admin.qualityOps.noHistory') }}</p>
              <div v-else class="space-y-2">
                <div v-for="result in history" :key="result.id" data-testid="quality-probe-result" class="min-w-0 border border-gray-100 p-3 text-sm dark:border-dark-700">
                  <div class="flex flex-wrap justify-between gap-2"><span>{{ result.status }}</span><span>{{ result.quality_action || '-' }}</span><time>{{ formatDate(result.started_at) }}</time></div>
                  <p class="mt-1 break-words text-xs text-gray-500">{{ t('admin.qualityOps.model') }}: {{ result.pelican_config?.model_id || plan.model_id }}<template v-if="result.quality_round_id"> · {{ t('admin.qualityOps.round') }}: {{ result.quality_round_id }}</template></p>
                  <p v-if="result.error_message" class="mt-1 text-xs text-red-700 dark:text-red-300">{{ result.error_message }}</p>
                  <p v-if="result.quality_judgment" class="mt-1 text-xs text-gray-500">{{ JSON.stringify(result.quality_judgment) }}</p>
                </div>
              </div>
            </div>
          </article>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import * as accountQualityAPI from '@/api/admin/accountQuality'
import type { QualityRulePreset, QualityRulePresetRequest, QualityRuleTemplate, QualityRuleTemplateRequest } from '@/api/admin/accountQuality'
import type { AccountListItem, PelicanTestConfig, QualityHistoryResult, QualityOwnershipPreview, ScheduledTestPlan, ScheduledTestResult } from '@/types'

const { t } = useI18n()
const plans = ref<ScheduledTestPlan[]>([])
const accounts = ref<AccountListItem[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')
const editingId = ref<number | null>(null)
const busyPlan = ref<number | null>(null)
const historyPlanId = ref<number | null>(null)
const history = ref<ScheduledTestResult[]>([])
const historyLoading = ref(false)
const ownershipPlanId = ref<number | null>(null)
const ownership = ref<QualityOwnershipPreview | null>(null)
const ownershipLoading = ref(false)
const ownershipError = ref('')
const globalHistory = ref<QualityHistoryResult[]>([])
const globalHistoryLoading = ref(false)
const globalHistoryError = ref('')
const globalHistoryCursor = ref(0)
const globalHistoryMoreLoading = ref(false)
const presets = ref<QualityRulePreset[]>([])
const presetLoading = ref(false)
const presetError = ref('')
const presetSaveError = ref('')
const presetSaving = ref(false)
const presetName = ref('')
const selectedPresetId = ref(0)
const showPresetSave = ref(false)
const templates = ref<QualityRuleTemplate[]>([])
const templateLoading = ref(false)
const templateSaving = ref(false)
const templateBusyId = ref<number | null>(null)
const templateFormOpen = ref(false)
const editingTemplateId = ref<number | null>(null)
const templateForm = reactive({ group: '', type: '', statuses: '', search: '', model_id: '', extra_models: [] as string[], parallel_count: 1, cron_expression: '*/30 * * * *', max_results: 100, prompt: '', expected_answer: '', action: 'observe_only', auto_restore: false, enabled: true })

const form = reactive({ account_id: 0, model_id: '', extra_models: [] as string[], parallel_count: 1, cron_expression: '*/30 * * * *', max_results: 100, rule_name: '', prompt: '', expected_answer: '', judge_kind: 'expected' as 'expected' | 'jev', judge_prompt: '', action: 'observe_only', auto_restore: false, enabled: true })

function resetForm() {
  editingId.value = null
  selectedPresetId.value = 0
  presetName.value = ''
  showPresetSave.value = false
  presetSaveError.value = ''
  Object.assign(form, { account_id: 0, model_id: '', extra_models: [] as string[], parallel_count: 1, cron_expression: '*/30 * * * *', max_results: 100, rule_name: '', prompt: '', expected_answer: '', judge_kind: 'expected', judge_prompt: '', action: 'observe_only', auto_restore: false, enabled: true })
}

function configuredModels(primary: string, config?: PelicanTestConfig | null): string[] {
  const models = [...new Set((config?.model_ids || []).map((model) => model.trim()).filter(Boolean))]
  return models.length ? models : [primary.trim()]
}

function probeConfig(primary: string, extra: string[], count: number): Pick<PelicanTestConfig, 'model_ids' | 'parallel_count'> {
  const models = [...new Set([primary, ...extra].map((model) => model.trim()))]
  if (models.some((model) => !model || model.length > 100) || models.length > 50) {
    throw new Error(t('admin.qualityOps.probeModelsInvalid'))
  }
  if (!Number.isInteger(count) || count < 1 || count > 32) {
    throw new Error(t('admin.qualityOps.probeCountInvalid'))
  }
  if (models.length * count > 100) throw new Error(t('admin.qualityOps.probeSamplesInvalid'))
  return { model_ids: models, parallel_count: count }
}

function modelSummary(primary: string, config?: PelicanTestConfig | null): string {
  return configuredModels(primary, config).join(' / ')
}

function accountName(id: number) {
  return accounts.value.find((account) => account.id === id)?.name || `#${id}`
}

function formatDate(value?: string | null) {
  return value ? new Date(value).toLocaleString() : '-'
}

function resetTemplateForm() {
  editingTemplateId.value = null
  Object.assign(templateForm, { group: '', type: '', statuses: '', search: '', model_id: '', extra_models: [] as string[], parallel_count: 1, cron_expression: '*/30 * * * *', max_results: 100, prompt: '', expected_answer: '', action: 'observe_only', auto_restore: false, enabled: true })
}

function newTemplate() {
  resetTemplateForm()
  templateFormOpen.value = true
}

function closeTemplateForm() {
  templateFormOpen.value = false
  resetTemplateForm()
}

function templatePelicanConfig(): PelicanTestConfig {
  return { ...probeConfig(templateForm.model_id, templateForm.extra_models, templateForm.parallel_count), question_kind: 'text', prompt: templateForm.prompt, quality: { action: templateForm.action, expected_answer: templateForm.expected_answer, auto_restore: templateForm.auto_restore, judge: { kind: 'expected' } } }
}

function templateRequest(): QualityRuleTemplateRequest {
  const statuses = templateForm.statuses.split(',').map((item) => item.trim()).filter(Boolean)
  return {
    account_filter: { group: templateForm.group || undefined, type: templateForm.type || undefined, search: templateForm.search || undefined, statuses: statuses.length ? statuses : undefined },
    model_id: templateForm.model_id.trim(), cron_expression: templateForm.cron_expression.trim(), max_results: templateForm.max_results, enabled: templateForm.enabled, pelican_config: templatePelicanConfig()
  }
}

function templateFilterSummary(template: QualityRuleTemplate) {
  const filter = template.account_filter || {}
  return [filter.group || t('admin.qualityOps.templates.allGroups'), filter.type || t('admin.qualityOps.templates.allTypes'), filter.statuses?.join(', ') || t('admin.qualityOps.templates.allStatuses'), filter.search || ''].filter(Boolean).join(' · ')
}

function editTemplate(template: QualityRuleTemplate) {
  editingTemplateId.value = template.id
  const filter = template.account_filter || {}
  const [model_id, ...extra_models] = configuredModels(template.model_id, template.pelican_config)
  Object.assign(templateForm, { group: filter.group || '', type: filter.type || '', statuses: filter.statuses?.join(', ') || '', search: filter.search || '', model_id, extra_models, parallel_count: template.pelican_config?.parallel_count || 1, cron_expression: template.cron_expression, max_results: template.max_results, prompt: template.pelican_config?.prompt || '', expected_answer: template.pelican_config?.quality?.expected_answer || '', action: template.pelican_config?.quality?.action || 'observe_only', auto_restore: template.pelican_config?.quality?.auto_restore === true, enabled: template.enabled })
  templateFormOpen.value = true
}

async function loadTemplates() {
  templateLoading.value = true
  try { templates.value = await accountQualityAPI.listTemplates() } catch (err: any) { error.value = err?.message || t('admin.qualityOps.templates.loadFailed') } finally { templateLoading.value = false }
}

async function saveTemplate() {
  templateSaving.value = true
  error.value = ''
  try {
    if (editingTemplateId.value) {
      const result = await accountQualityAPI.updateTemplate(editingTemplateId.value, templateRequest())
      notice.value = t('admin.qualityOps.templates.updated', { updated: result.updated, created: result.created, failed: result.failed })
    } else {
      const result = await accountQualityAPI.createTemplate(templateRequest())
      notice.value = t('admin.qualityOps.templates.created', { created: result.created })
    }
    closeTemplateForm()
    await loadTemplates()
  } catch (err: any) { error.value = err?.message || t('admin.qualityOps.templates.saveFailed') } finally { templateSaving.value = false }
}

async function toggleTemplate(template: QualityRuleTemplate) {
  templateBusyId.value = template.id
  try { const result = await accountQualityAPI.updateTemplate(template.id, { enabled: !template.enabled }); notice.value = t('admin.qualityOps.templates.updated', { updated: result.updated, created: result.created, failed: result.failed }); await loadTemplates() } catch (err: any) { error.value = err?.message || t('admin.qualityOps.templates.saveFailed') } finally { templateBusyId.value = null }
}

async function deleteTemplateRecord(template: QualityRuleTemplate, deletePlans: boolean) {
  const promptKey = deletePlans ? 'admin.qualityOps.templates.confirmDeletePlans' : 'admin.qualityOps.templates.confirmDeleteKeep'
  if (!window.confirm(t(promptKey))) return
  templateBusyId.value = template.id
  try { const result = await accountQualityAPI.deleteTemplate(template.id, deletePlans); notice.value = t('admin.qualityOps.templates.deleted', { deleted: result.deleted }); await loadTemplates() } catch (err: any) { error.value = err?.message || t('admin.qualityOps.templates.deleteFailed') } finally { templateBusyId.value = null }
}

async function load() {
  loading.value = true
  error.value = ''
  globalHistoryLoading.value = true
  globalHistoryError.value = ''
  try {
    const [nextPlans, accountPage] = await Promise.all([
      accountQualityAPI.listPlans(),
      adminAPI.accounts.list(1, 1000, { lite: 'true' })
    ])
    plans.value = nextPlans
    accounts.value = accountPage.items
    await Promise.all([loadPresets(), loadTemplates()])
    try {
      const page = await accountQualityAPI.listHistory()
      globalHistory.value = page.items
      globalHistoryCursor.value = page.next_cursor
    } catch (err: any) {
      globalHistoryError.value = err?.message || t('admin.qualityOps.globalHistoryFailed')
      globalHistory.value = []
      globalHistoryCursor.value = 0
    }
  } catch (err: any) {
    error.value = err?.message || t('admin.qualityOps.loadFailed')
  } finally {
    loading.value = false
    globalHistoryLoading.value = false
  }
}

async function loadPresets() {
  presetLoading.value = true
  presetError.value = ''
  try {
    presets.value = await accountQualityAPI.listPresets()
  } catch (err: any) {
    presetError.value = err?.message || t('admin.qualityOps.presetsLoadFailed')
    presets.value = []
  } finally {
    presetLoading.value = false
  }
}

function applyPreset() {
  const preset = presets.value.find((item) => item.id === selectedPresetId.value)
  if (!preset) return
  const config = preset.pelican_config
  const [primary, ...extra] = configuredModels(preset.model_id, config)
  form.model_id = primary
  form.extra_models = extra
  form.parallel_count = config.parallel_count || 1
  form.cron_expression = preset.cron_expression
  form.max_results = preset.max_results
  form.rule_name = config.rule_name || preset.name
  form.prompt = config.prompt || ''
  form.expected_answer = config.quality?.expected_answer || ''
  form.judge_kind = config.quality?.judge?.kind === 'jev' ? 'jev' : 'expected'
  form.judge_prompt = config.quality?.judge?.prompt || ''
  form.action = config.quality?.action || 'observe_only'
  form.auto_restore = config.quality?.auto_restore === true
  form.enabled = preset.enabled
  notice.value = t('admin.qualityOps.presetApplied')
}

function pelicanConfig(): PelicanTestConfig {
  return {
    ...probeConfig(form.model_id, form.extra_models, form.parallel_count),
    rule_name: form.rule_name || undefined,
    question_kind: 'text',
    prompt: form.prompt,
    quality: {
      action: form.action,
      auto_restore: form.auto_restore,
      expected_answer: form.expected_answer,
      judge: { kind: form.judge_kind, prompt: form.judge_kind === 'jev' ? form.judge_prompt || undefined : undefined }
    }
  }
}

async function savePreset() {
  if (presetSaving.value) return
  const name = presetName.value.trim()
  if (!name || Array.from(name).length > 100) {
    presetSaveError.value = t('admin.qualityOps.presetNameInvalid')
    return
  }
  presetSaving.value = true
  presetSaveError.value = ''
  try {
    const request: QualityRulePresetRequest = {
      name,
      model_id: form.model_id.trim(),
      cron_expression: form.cron_expression.trim(),
      enabled: form.enabled,
      max_results: form.max_results,
      pelican_config: pelicanConfig()
    }
    const created = await accountQualityAPI.createPreset(request)
    presets.value = [created, ...presets.value.filter((item) => item.id !== created.id)]
    selectedPresetId.value = created.id
    presetName.value = ''
    showPresetSave.value = false
    notice.value = t('admin.qualityOps.presetSaved')
  } catch (err: any) {
    presetSaveError.value = err?.message || t('admin.qualityOps.presetSaveFailed')
  } finally {
    presetSaving.value = false
  }
}

async function loadMoreGlobalHistory() {
  if (globalHistoryMoreLoading.value || globalHistoryCursor.value <= 0) return
  globalHistoryMoreLoading.value = true
  globalHistoryError.value = ''
  try {
    const page = await accountQualityAPI.listHistory(globalHistoryCursor.value)
    const knownIds = new Set(globalHistory.value.map((item) => item.id))
    globalHistory.value.push(...page.items.filter((item) => !knownIds.has(item.id)))
    globalHistoryCursor.value = page.next_cursor
  } catch (err: any) {
    globalHistoryError.value = err?.message || t('admin.qualityOps.globalHistoryFailed')
  } finally {
    globalHistoryMoreLoading.value = false
  }
}

function edit(plan: ScheduledTestPlan) {
  editingId.value = plan.id
  form.account_id = plan.account_id
  const [primary, ...extra] = configuredModels(plan.model_id, plan.pelican_config)
  form.model_id = primary
  form.extra_models = extra
  form.parallel_count = plan.pelican_config?.parallel_count || 1
  form.cron_expression = plan.cron_expression
  form.max_results = plan.max_results
  form.rule_name = plan.pelican_config?.rule_name || ''
  form.prompt = plan.pelican_config?.prompt || ''
  form.expected_answer = plan.pelican_config?.quality?.expected_answer || ''
  form.judge_kind = plan.pelican_config?.quality?.judge?.kind === 'jev' ? 'jev' : 'expected'
  form.judge_prompt = plan.pelican_config?.quality?.judge?.prompt || ''
  form.action = plan.pelican_config?.quality?.action || 'observe_only'
  form.auto_restore = plan.pelican_config?.quality?.auto_restore === true
  form.enabled = plan.enabled
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function save() {
  if (!form.account_id || saving.value) return
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    const pelican_config = pelicanConfig()
    if (editingId.value) {
      const current = plans.value.find((plan) => plan.id === editingId.value)
      await adminAPI.scheduledTests.update(editingId.value, { model_id: form.model_id.trim(), cron_expression: form.cron_expression, max_results: form.max_results, enabled: form.enabled, pelican_config, expected_updated_at: current?.updated_at })
      notice.value = t('admin.qualityOps.saved')
    } else {
      await adminAPI.scheduledTests.create({ account_id: form.account_id, model_id: form.model_id.trim(), cron_expression: form.cron_expression, enabled: form.enabled, max_results: form.max_results, pelican_config })
      notice.value = t('admin.qualityOps.created')
    }
    resetForm()
    await load()
  } catch (err: any) {
    error.value = err?.message || t('admin.qualityOps.saveFailed')
  } finally {
    saving.value = false
  }
}

async function run(plan: ScheduledTestPlan) {
  busyPlan.value = plan.id
  error.value = ''
  try {
    await accountQualityAPI.runPlan(plan.id)
    notice.value = t('admin.qualityOps.queued')
    await load()
  } catch (err: any) {
    error.value = err?.message || t('admin.qualityOps.runFailed')
  } finally {
    busyPlan.value = null
  }
}

async function toggleHistory(plan: ScheduledTestPlan) {
  if (historyPlanId.value === plan.id) { historyPlanId.value = null; return }
  historyPlanId.value = plan.id
  historyLoading.value = true
  try { history.value = await accountQualityAPI.listResults(plan.id) } catch (err: any) { error.value = err?.message || t('admin.qualityOps.historyFailed'); history.value = [] } finally { historyLoading.value = false }
}

async function preview(plan: ScheduledTestPlan) {
  ownershipPlanId.value = plan.id
  ownership.value = null
  ownershipError.value = ''
  ownershipLoading.value = true
  try { ownership.value = await adminAPI.scheduledTests.ownershipPreview(plan.id) } catch (err: any) { ownershipError.value = err?.message || t('admin.qualityOps.ownershipFailed') } finally { ownershipLoading.value = false }
}

async function applyOwnership(action: 'reconcile' | 'restore' | 'delete', plan: ScheduledTestPlan) {
  if (!ownership.value?.fence || !window.confirm(t(`admin.qualityOps.confirm.${action}`))) return
  busyPlan.value = plan.id
  try {
    if (action === 'reconcile') await adminAPI.scheduledTests.reconcileOwnership(plan.id, ownership.value.fence)
    else if (action === 'restore') await adminAPI.scheduledTests.restoreOwnership(plan.id, ownership.value.fence)
    else await adminAPI.scheduledTests.deleteOwnedPlan(plan.id, ownership.value.fence)
    notice.value = t('admin.qualityOps.operationDone')
    await load()
    ownershipPlanId.value = null
    ownership.value = null
  } catch (err: any) { ownershipError.value = err?.message || t('admin.qualityOps.operationFailed') } finally { busyPlan.value = null }
}

onMounted(load)
</script>
