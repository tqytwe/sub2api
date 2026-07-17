<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useAppStore } from '@/stores/app'
import {
  approveImportItem,
  approvePrompt,
  createAdminPrompt,
  createPromptImportJob,
  getAdminPrompt,
  listAdminPrompts,
  listImportItems,
  listImportJobs,
  listReports,
  rejectImportItem,
  resolvePromptReport,
  rollbackPrompt,
  submitPromptReview,
  unpublishPrompt,
  updateAdminPrompt,
  type AdminPrompt,
  type AdminPromptDraft,
  type AdminPromptPagination,
  type AdminPromptStatus,
  type PromptImportItem,
  type PromptImportJob,
  type PromptReport,
} from '@/api/admin/prompts'
import type { PromptSourceAttribution, PromptVariableDefinition } from '@/api/prompts'
import {
  createDefaultAdminPromptDraft,
  PROMPT_SOURCE_LABELS,
  PROMPT_STATUS_LABELS,
  promptSourceLabel,
  promptStatusLabel,
} from '@/utils/promptLibrary'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import '@/components/prompt/prompt-admin.css'

type AdminTab = 'prompts' | 'imports' | 'reports'
type ConfirmAction = 'unpublish'

const appStore = useAppStore()
const activeTab = ref<AdminTab>('prompts')
const loading = ref(true)
const loadFailed = ref(false)
const actionBusy = ref(false)
const search = ref('')
const statusFilter = ref<AdminPromptStatus | ''>('')
const prompts = reactive<AdminPromptPagination>({
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 0,
})
const imports = reactive({
  items: [] as PromptImportItem[],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 0,
})
const importJobs = ref<PromptImportJob[]>([])
const reports = reactive({
  items: [] as PromptReport[],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 0,
})
let searchTimer: ReturnType<typeof setTimeout> | null = null

const editorOpen = ref(false)
const editingPrompt = ref<AdminPrompt | null>(null)
const form = reactive<AdminPromptDraft>(createDefaultAdminPromptDraft())
const variablesText = ref('[]')
const modelsText = ref('')
const sizesText = ref('')

const confirmOpen = ref(false)
const confirmAction = ref<ConfirmAction>('unpublish')
const confirmTarget = ref<AdminPrompt | null>(null)

const rollbackOpen = ref(false)
const rollbackTarget = ref<AdminPrompt | null>(null)
const rollbackVersion = ref(1)

const rejectOpen = ref(false)
const rejectTarget = ref<PromptImportItem | null>(null)
const rejectReason = ref('')

const reportOpen = ref(false)
const reportTarget = ref<PromptReport | null>(null)
const reportNote = ref('')

const importDialogOpen = ref(false)
const importSourceKey = ref('人工整理')
const importPayloadText = ref('[]')

const sourceOptions: Array<{ value: PromptSourceAttribution; label: string }> = [
  { value: 'curated', label: PROMPT_SOURCE_LABELS.curated },
  { value: 'original', label: PROMPT_SOURCE_LABELS.original },
  { value: 'authorized', label: PROMPT_SOURCE_LABELS.authorized },
  { value: 'community', label: PROMPT_SOURCE_LABELS.community },
]

async function loadPromptList() {
  loading.value = true
  loadFailed.value = false
  try {
    Object.assign(prompts, await listAdminPrompts({
      q: search.value.trim() || undefined,
      status: statusFilter.value || undefined,
      page: prompts.page,
      page_size: prompts.page_size,
    }))
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

async function loadImports() {
  loading.value = true
  loadFailed.value = false
  try {
    const [itemsResponse, jobsResponse] = await Promise.all([
      listImportItems({ status: 'pending_review', page: imports.page, page_size: imports.page_size }),
      listImportJobs({ page: 1, page_size: 10 }),
    ])
    Object.assign(imports, itemsResponse)
    importJobs.value = jobsResponse.items
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

function openImportDialog() {
  importSourceKey.value = '人工整理'
  importPayloadText.value = '[]'
  importDialogOpen.value = true
}

async function createImportJob() {
  let parsed: unknown
  try {
    parsed = JSON.parse(importPayloadText.value)
  } catch {
    appStore.showError('导入内容不是有效的 JSON')
    return
  }
  const items = Array.isArray(parsed)
    ? parsed
    : parsed && typeof parsed === 'object' && Array.isArray((parsed as { items?: unknown }).items)
      ? (parsed as { items: unknown[] }).items
      : null
  if (!importSourceKey.value.trim() || !items?.length) {
    appStore.showError('请填写来源标识并提供至少一条内容')
    return
  }
  actionBusy.value = true
  try {
    await createPromptImportJob({
      source_key: importSourceKey.value.trim(),
      items: items.filter((item): item is Record<string, unknown> =>
        !!item && typeof item === 'object' && !Array.isArray(item)),
      raw_payload: { imported_at: new Date().toISOString() },
    })
    appStore.showSuccess('导入任务已进入待审核队列')
    importDialogOpen.value = false
    await loadImports()
  } catch {
    appStore.showError('导入失败，请检查内容格式后重试')
  } finally {
    actionBusy.value = false
  }
}

async function loadReportsList() {
  loading.value = true
  loadFailed.value = false
  try {
    Object.assign(reports, await listReports({
      status: 'open',
      page: reports.page,
      page_size: reports.page_size,
    }))
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

function switchTab(tab: AdminTab) {
  activeTab.value = tab
  if (tab === 'prompts') void loadPromptList()
  if (tab === 'imports') void loadImports()
  if (tab === 'reports') void loadReportsList()
}

function scheduleSearch() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    prompts.page = 1
    void loadPromptList()
  }, 260)
}

function resetForm() {
  Object.assign(form, createDefaultAdminPromptDraft())
  variablesText.value = '[]'
  modelsText.value = ''
  sizesText.value = ''
}

function openCreate() {
  editingPrompt.value = null
  resetForm()
  editorOpen.value = true
}

function fillEditor(prompt: AdminPrompt) {
  editingPrompt.value = prompt
  Object.assign(form, {
    title: prompt.title,
    purpose_description: prompt.purpose_description,
    prompt_template: prompt.prompt_template,
    variables: prompt.variables || [],
    preview_image_url: prompt.preview_image_url || '',
    recommended_models: prompt.recommended_models || [],
    recommended_sizes: prompt.recommended_sizes || [],
    reference_requirement: prompt.reference_requirement,
    reference_instructions: prompt.reference_instructions || '',
    source_attribution: prompt.source_attribution,
    source_evidence_summary: prompt.source_evidence_summary || '',
    source_evidence_verified: prompt.source_evidence_verified || false,
    source_evidence_captured_at: prompt.source_evidence_captured_at || '',
    source_author: prompt.source_author || '',
    source_url: prompt.source_url || '',
    featured: prompt.featured,
    purpose: prompt.purpose || '',
    style: prompt.style || '',
    subject: prompt.subject || '',
    content_notice: prompt.content_notice || '',
  })
  variablesText.value = JSON.stringify(prompt.variables || [], null, 2)
  modelsText.value = (prompt.recommended_models || []).join('\n')
  sizesText.value = (prompt.recommended_sizes || []).join('\n')
  editorOpen.value = true
}

async function openEdit(prompt: AdminPrompt) {
  actionBusy.value = true
  try {
    fillEditor(await getAdminPrompt(prompt.id))
  } catch {
    appStore.showError('加载提示词详情失败，请稍后重试')
  } finally {
    actionBusy.value = false
  }
}

function lines(value: string): string[] {
  return Array.from(new Set(value.split(/\r?\n|,/).map((item) => item.trim()).filter(Boolean)))
}

function parseVariables(): PromptVariableDefinition[] | null {
  try {
    const parsed = JSON.parse(variablesText.value || '[]')
    if (!Array.isArray(parsed)) return null
    return parsed as PromptVariableDefinition[]
  } catch {
    return null
  }
}

async function savePrompt() {
  const variables = parseVariables()
  if (!variables) {
    appStore.showError('变量定义格式不正确')
    return
  }
  if (
    form.source_attribution === 'original'
    && (!form.source_evidence_verified || !form.source_evidence_summary.trim())
  ) {
    appStore.showError('标记为极速蹬原创前，必须填写来源证据并确认已经核验')
    return
  }
  actionBusy.value = true
  try {
    const payload: AdminPromptDraft = {
      ...form,
      variables,
      recommended_models: lines(modelsText.value),
      recommended_sizes: lines(sizesText.value),
    }
    if (editingPrompt.value) {
      await updateAdminPrompt(editingPrompt.value.id, payload, editingPrompt.value.version)
      appStore.showSuccess('提示词已更新')
    } else {
      await createAdminPrompt(payload)
      appStore.showSuccess('提示词已创建')
    }
    editorOpen.value = false
    await loadPromptList()
  } catch {
    appStore.showError('保存失败，请检查填写内容后重试')
  } finally {
    actionBusy.value = false
  }
}

async function runPromptAction(
  prompt: AdminPrompt,
  action: 'submit' | 'approve',
) {
  actionBusy.value = true
  try {
    if (action === 'submit') await submitPromptReview(prompt.id)
    if (action === 'approve') await approvePrompt(prompt.id)
    appStore.showSuccess({
      submit: '已提交审核',
      approve: '已批准并发布提示词',
    }[action])
    await loadPromptList()
  } catch {
    appStore.showError('操作失败，请稍后重试')
  } finally {
    actionBusy.value = false
  }
}

function openConfirm(prompt: AdminPrompt, action: ConfirmAction) {
  confirmTarget.value = prompt
  confirmAction.value = action
  confirmOpen.value = true
}

async function confirmPromptAction() {
  if (!confirmTarget.value) return
  actionBusy.value = true
  try {
    await unpublishPrompt(confirmTarget.value.id)
    appStore.showSuccess('提示词已下线')
    confirmOpen.value = false
    await loadPromptList()
  } catch {
    appStore.showError('操作失败，请稍后重试')
  } finally {
    actionBusy.value = false
  }
}

function openRollback(prompt: AdminPrompt) {
  rollbackTarget.value = prompt
  rollbackVersion.value = Math.max(1, prompt.version - 1)
  rollbackOpen.value = true
}

async function confirmRollback() {
  if (!rollbackTarget.value || rollbackVersion.value >= rollbackTarget.value.version) {
    appStore.showError('请选择早于当前版本的版本号')
    return
  }
  actionBusy.value = true
  try {
    await rollbackPrompt(rollbackTarget.value.id, rollbackVersion.value)
    appStore.showSuccess('提示词已回滚')
    rollbackOpen.value = false
    await loadPromptList()
  } catch {
    appStore.showError('回滚失败，请确认版本号后重试')
  } finally {
    actionBusy.value = false
  }
}

async function approveImported(item: PromptImportItem) {
  actionBusy.value = true
  try {
    await approveImportItem(item.id)
    appStore.showSuccess('导入内容已批准')
    await loadImports()
  } catch {
    appStore.showError('批准失败，请稍后重试')
  } finally {
    actionBusy.value = false
  }
}

function openReject(item: PromptImportItem) {
  rejectTarget.value = item
  rejectReason.value = ''
  rejectOpen.value = true
}

async function confirmReject() {
  if (!rejectTarget.value || !rejectReason.value.trim()) {
    appStore.showError('请填写拒绝原因')
    return
  }
  actionBusy.value = true
  try {
    await rejectImportItem(rejectTarget.value.id, rejectReason.value.trim())
    appStore.showSuccess('导入内容已拒绝')
    rejectOpen.value = false
    await loadImports()
  } catch {
    appStore.showError('拒绝操作失败，请稍后重试')
  } finally {
    actionBusy.value = false
  }
}

function openReport(report: PromptReport) {
  reportTarget.value = report
  reportNote.value = ''
  reportOpen.value = true
}

async function finishReport(resolution: 'resolved' | 'dismissed') {
  if (!reportTarget.value || !reportNote.value.trim()) {
    appStore.showError('请填写处理说明')
    return
  }
  actionBusy.value = true
  try {
    await resolvePromptReport(reportTarget.value.id, resolution, reportNote.value.trim())
    appStore.showSuccess(resolution === 'resolved' ? '投诉已处理' : '投诉已驳回')
    reportOpen.value = false
    await loadReportsList()
  } catch {
    appStore.showError('投诉处理失败，请稍后重试')
  } finally {
    actionBusy.value = false
  }
}

function statusClass(status: AdminPromptStatus): string {
  if (status === 'published') return 'is-live'
  if (status === 'pending_review') return 'is-pending'
  if (status === 'offline') return 'is-offline'
  return ''
}

function formatTime(value?: string): string {
  if (!value) return '暂无'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

onMounted(loadPromptList)

onBeforeUnmount(() => {
  if (searchTimer) clearTimeout(searchTimer)
})
</script>

<template>
  <AppLayout>
    <div class="prompt-admin-page">
      <header class="prompt-admin-heading">
        <div>
          <h1>提示词管理</h1>
          <p>管理内容、审核版本、导入证据与用户投诉。</p>
        </div>
        <button
          v-if="activeTab === 'prompts'"
          type="button"
          class="btn btn-primary"
          data-testid="create-prompt"
          @click="openCreate"
        >
          <Icon name="plus" size="sm" class="mr-1" />
          创建提示词
        </button>
        <button
          v-else-if="activeTab === 'imports'"
          type="button"
          class="btn btn-primary"
          data-testid="create-import-job"
          @click="openImportDialog"
        >
          <Icon name="plus" size="sm" class="mr-1" />
          导入标准化内容
        </button>
      </header>

      <nav class="prompt-admin-tabs" aria-label="提示词管理视图">
        <button
          type="button"
          class="prompt-admin-tab"
          :class="{ 'is-active': activeTab === 'prompts' }"
          @click="switchTab('prompts')"
        >
          提示词列表
        </button>
        <button
          type="button"
          class="prompt-admin-tab"
          :class="{ 'is-active': activeTab === 'imports' }"
          @click="switchTab('imports')"
        >
          导入待审
        </button>
        <button
          type="button"
          class="prompt-admin-tab"
          :class="{ 'is-active': activeTab === 'reports' }"
          @click="switchTab('reports')"
        >
          投诉处理
        </button>
      </nav>

      <template v-if="activeTab === 'prompts'">
        <div class="prompt-admin-toolbar">
          <label class="prompt-admin-search">
            <span class="sr-only">搜索提示词</span>
            <Icon name="search" size="sm" />
            <input
              v-model="search"
              type="search"
              class="input"
              placeholder="搜索标题或用途"
              @input="scheduleSearch"
            />
          </label>
          <label>
            <span class="sr-only">状态筛选</span>
            <select
              v-model="statusFilter"
              class="input"
              aria-label="状态筛选"
              @change="prompts.page = 1; loadPromptList()"
            >
              <option value="">全部状态</option>
              <option v-for="(label, value) in PROMPT_STATUS_LABELS" :key="value" :value="value">
                {{ label }}
              </option>
            </select>
          </label>
          <button type="button" class="btn btn-secondary" :disabled="loading" aria-label="刷新列表" @click="loadPromptList">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          </button>
        </div>

        <div class="prompt-admin-table-shell">
          <div v-if="loading" class="prompt-admin-state">正在加载提示词</div>
          <div v-else-if="loadFailed" class="prompt-admin-state">
            <p>提示词列表加载失败</p>
            <button type="button" class="btn btn-secondary" @click="loadPromptList">重新加载</button>
          </div>
          <div v-else-if="prompts.items.length === 0" class="prompt-admin-state">
            暂无符合条件的提示词
          </div>
          <table v-else class="prompt-admin-table">
            <thead>
              <tr>
                <th>提示词</th>
                <th>来源归属</th>
                <th>状态</th>
                <th>版本</th>
                <th>使用情况</th>
                <th>更新时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="prompt in prompts.items" :key="prompt.id">
                <td class="prompt-admin-title-cell">
                  <strong>{{ prompt.title }}</strong>
                  <span>{{ prompt.purpose_description }}</span>
                </td>
                <td><span class="prompt-admin-badge">{{ promptSourceLabel(prompt.source_attribution) }}</span></td>
                <td>
                  <span class="prompt-admin-badge" :class="statusClass(prompt.status)">
                    {{ promptStatusLabel(prompt.status) }}
                  </span>
                </td>
                <td>第 {{ prompt.version }} 版</td>
                <td>{{ prompt.use_count || 0 }} 次使用<br />{{ prompt.favorite_count || 0 }} 次收藏</td>
                <td>{{ formatTime(prompt.updated_at) }}</td>
                <td>
                  <div class="prompt-admin-actions">
                    <button type="button" class="prompt-admin-action" @click="openEdit(prompt)">编辑</button>
                    <button
                      v-if="prompt.status === 'draft' || prompt.status === 'offline'"
                      type="button"
                      class="prompt-admin-action"
                      :disabled="actionBusy"
                      @click="runPromptAction(prompt, 'submit')"
                    >
                      提交审核
                    </button>
                    <button
                      v-if="prompt.status === 'pending_review'"
                      type="button"
                      class="prompt-admin-action"
                      :disabled="actionBusy"
                      @click="runPromptAction(prompt, 'approve')"
                    >
                      批准并发布
                    </button>
                    <button
                      v-if="prompt.status === 'published'"
                      type="button"
                      class="prompt-admin-action is-danger"
                      :disabled="actionBusy"
                      @click="openConfirm(prompt, 'unpublish')"
                    >
                      下线
                    </button>
                    <button
                      v-if="prompt.version > 1"
                      type="button"
                      class="prompt-admin-action"
                      :disabled="actionBusy"
                      @click="openRollback(prompt)"
                    >
                      回滚
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <Pagination
          v-if="prompts.total > 0"
          :total="prompts.total"
          :page="prompts.page"
          :page-size="prompts.page_size"
          :show-page-size-selector="false"
          @update:page="prompts.page = $event; loadPromptList()"
        />
      </template>

      <template v-else-if="activeTab === 'imports'">
        <div v-if="importJobs.length" class="prompt-admin-job-summary" aria-label="导入任务摘要">
          <span v-for="job in importJobs" :key="job.id">
            {{ job.source_name || '导入任务' }}：待审 {{ job.pending_items || 0 }} 条
          </span>
        </div>
        <div class="prompt-admin-table-shell">
          <div v-if="loading" class="prompt-admin-state">正在加载导入待审内容</div>
          <div v-else-if="loadFailed" class="prompt-admin-state">导入待审内容加载失败</div>
          <div v-else-if="imports.items.length === 0" class="prompt-admin-state">暂无待审导入内容</div>
          <table v-else class="prompt-admin-table">
            <thead>
              <tr>
                <th>标题</th>
                <th>来源归属</th>
                <th>来源证据摘要</th>
                <th>提交时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in imports.items" :key="item.id">
                <td class="prompt-admin-title-cell">
                  <strong>{{ item.title }}</strong>
                  <span>{{ item.prompt_template || '未提供生成提示词' }}</span>
                </td>
                <td><span class="prompt-admin-badge">{{ promptSourceLabel(item.source_attribution) }}</span></td>
                <td class="prompt-admin-evidence">
                  <strong>{{ item.source_author || '未注明作者' }}</strong>
                  {{ item.source_evidence_summary || '未提供来源证据摘要' }}
                </td>
                <td>{{ formatTime(item.created_at) }}</td>
                <td>
                  <div class="prompt-admin-actions">
                    <button type="button" class="prompt-admin-action" :disabled="actionBusy" @click="approveImported(item)">
                      批准
                    </button>
                    <button type="button" class="prompt-admin-action is-danger" :disabled="actionBusy" @click="openReject(item)">
                      拒绝
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <template v-else>
        <div class="prompt-admin-table-shell">
          <div v-if="loading" class="prompt-admin-state">正在加载投诉</div>
          <div v-else-if="loadFailed" class="prompt-admin-state">投诉列表加载失败</div>
          <div v-else-if="reports.items.length === 0" class="prompt-admin-state">暂无待处理投诉</div>
          <table v-else class="prompt-admin-table">
            <thead>
              <tr>
                <th>提示词</th>
                <th>投诉原因</th>
                <th>补充说明</th>
                <th>提交人</th>
                <th>提交时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="report in reports.items" :key="report.id">
                <td>{{ report.prompt_title || `提示词 ${report.prompt_id}` }}</td>
                <td>{{ report.reason }}</td>
                <td class="prompt-admin-evidence">{{ report.description || '无补充说明' }}</td>
                <td>{{ report.reporter_name || '匿名用户' }}</td>
                <td>{{ formatTime(report.created_at) }}</td>
                <td>
                  <button type="button" class="prompt-admin-action" @click="openReport(report)">处理投诉</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </div>

    <BaseDialog
      :show="editorOpen"
      :title="editingPrompt ? `编辑提示词 · 第 ${editingPrompt.version} 版` : '创建提示词'"
      width="extra-wide"
      @close="editorOpen = false"
    >
      <form id="prompt-editor-form" class="prompt-admin-form" @submit.prevent="savePrompt">
        <section class="prompt-admin-form-section">
          <h3>基础内容</h3>
          <div class="prompt-admin-form-grid">
            <label class="prompt-admin-field">
              <span>中文标题</span>
              <input v-model="form.title" class="input" required />
            </label>
            <label class="prompt-admin-field">
              <span>示例效果图片地址</span>
              <input v-model="form.preview_image_url" class="input" type="url" />
            </label>
            <label class="prompt-admin-field is-full">
              <span>用途说明</span>
              <textarea v-model="form.purpose_description" class="input" rows="3" required></textarea>
            </label>
            <label class="prompt-admin-field is-full">
              <span>生成提示词（英文）</span>
              <textarea v-model="form.prompt_template" class="input font-mono" rows="8" required></textarea>
            </label>
            <label class="prompt-admin-field is-full">
              <span>变量定义</span>
              <textarea v-model="variablesText" class="input font-mono" rows="7"></textarea>
              <small>使用数组格式，每项可包含名称、中文标签、说明与是否必填。</small>
            </label>
          </div>
        </section>

        <section class="prompt-admin-form-section">
          <h3>分类与创作建议</h3>
          <div class="prompt-admin-form-grid is-three">
            <label class="prompt-admin-field">
              <span>用途</span>
              <input v-model="form.purpose" class="input" />
            </label>
            <label class="prompt-admin-field">
              <span>风格</span>
              <input v-model="form.style" class="input" />
            </label>
            <label class="prompt-admin-field">
              <span>主体</span>
              <input v-model="form.subject" class="input" />
            </label>
            <label class="prompt-admin-field">
              <span>推荐模型</span>
              <textarea v-model="modelsText" class="input" rows="4"></textarea>
              <small>每行填写一个模型名。</small>
            </label>
            <label class="prompt-admin-field">
              <span>推荐尺寸</span>
              <textarea v-model="sizesText" class="input" rows="4"></textarea>
              <small>每行填写一个尺寸。</small>
            </label>
            <label class="prompt-admin-field">
              <span>参考图要求</span>
              <select v-model="form.reference_requirement" class="input">
                <option value="none">无需参考图</option>
                <option value="optional">可选参考图</option>
                <option value="required">需要参考图</option>
              </select>
            </label>
            <label class="prompt-admin-field is-full">
              <span>参考图说明</span>
              <textarea v-model="form.reference_instructions" class="input" rows="3"></textarea>
            </label>
          </div>
        </section>

        <section class="prompt-admin-form-section">
          <h3>来源与展示</h3>
          <div class="prompt-admin-form-grid">
            <label class="prompt-admin-field">
              <span>来源归属</span>
              <select
                v-model="form.source_attribution"
                class="input"
                data-testid="prompt-source-attribution"
              >
                <option v-for="option in sourceOptions" :key="option.value" :value="option.value">
                  {{ option.label }}
                </option>
              </select>
            </label>
            <label class="prompt-admin-field">
              <span>作者或权利人</span>
              <input v-model="form.source_author" class="input" />
            </label>
            <label class="prompt-admin-field is-full">
              <span>来源证据摘要</span>
              <textarea v-model="form.source_evidence_summary" class="input" rows="4"></textarea>
            </label>
            <label class="prompt-admin-field">
              <span>取证时间</span>
              <input v-model="form.source_evidence_captured_at" class="input" type="datetime-local" />
            </label>
            <label class="prompt-admin-check">
              <input v-model="form.source_evidence_verified" type="checkbox" />
              已核验作者、链接与授权状态
            </label>
            <label class="prompt-admin-field is-full">
              <span>来源证据地址</span>
              <input v-model="form.source_url" class="input" type="url" />
            </label>
            <label class="prompt-admin-field is-full">
              <span>内容说明</span>
              <textarea v-model="form.content_notice" class="input" rows="3"></textarea>
            </label>
            <label class="prompt-admin-check is-full">
              <input v-model="form.featured" type="checkbox" />
              设为极速蹬精选
            </label>
          </div>
        </section>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="editorOpen = false">取消</button>
          <button type="submit" form="prompt-editor-form" class="btn btn-primary" :disabled="actionBusy">
            {{ actionBusy ? '正在保存' : '保存提示词' }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="importDialogOpen"
      title="导入标准化内容"
      width="extra-wide"
      @close="importDialogOpen = false"
    >
      <div class="prompt-admin-form">
        <label class="prompt-admin-field">
          <span>来源标识</span>
          <input v-model="importSourceKey" class="input" />
        </label>
        <label class="prompt-admin-field">
          <span>待审核内容（JSON）</span>
          <textarea v-model="importPayloadText" class="input font-mono" rows="18"></textarea>
        </label>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="importDialogOpen = false">取消</button>
          <button type="button" class="btn btn-primary" :disabled="actionBusy" @click="createImportJob">
            {{ actionBusy ? '正在导入' : '进入待审核队列' }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="confirmOpen"
      title="下线提示词"
      :message="`确认下线“${confirmTarget?.title || ''}”？公共页面将不再展示。`"
      confirm-text="确认下线"
      cancel-text="取消"
      danger
      @confirm="confirmPromptAction"
      @cancel="confirmOpen = false"
    />

    <BaseDialog :show="rollbackOpen" title="回滚提示词版本" width="narrow" @close="rollbackOpen = false">
      <label class="prompt-admin-field">
        <span>目标版本</span>
        <input
          v-model.number="rollbackVersion"
          class="input"
          type="number"
          min="1"
          :max="Math.max(1, (rollbackTarget?.version || 1) - 1)"
        />
        <small>当前为第 {{ rollbackTarget?.version || 1 }} 版，只能回滚到更早版本。</small>
      </label>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="rollbackOpen = false">取消</button>
          <button type="button" class="btn btn-primary" :disabled="actionBusy" @click="confirmRollback">确认回滚</button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="rejectOpen" title="拒绝导入内容" width="normal" @close="rejectOpen = false">
      <label class="prompt-admin-field">
        <span>拒绝原因</span>
        <textarea v-model="rejectReason" class="input" rows="5" placeholder="说明证据、授权或内容方面的问题"></textarea>
      </label>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="rejectOpen = false">取消</button>
          <button type="button" class="btn btn-danger" :disabled="actionBusy" @click="confirmReject">确认拒绝</button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="reportOpen" title="处理投诉" width="normal" @close="reportOpen = false">
      <div class="space-y-4">
        <p class="text-sm text-gray-600 dark:text-gray-300">
          {{ reportTarget?.reason }}：{{ reportTarget?.description || '无补充说明' }}
        </p>
        <label class="prompt-admin-field">
          <span>处理说明</span>
          <textarea v-model="reportNote" class="input" rows="5" placeholder="记录核查结果与后续处理"></textarea>
        </label>
      </div>
      <template #footer>
        <div class="flex flex-wrap justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="reportOpen = false">取消</button>
          <button type="button" class="btn btn-secondary" :disabled="actionBusy" @click="finishReport('dismissed')">
            驳回投诉
          </button>
          <button type="button" class="btn btn-primary" :disabled="actionBusy" @click="finishReport('resolved')">
            完成处理
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>
