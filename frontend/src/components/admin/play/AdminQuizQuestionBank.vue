<template>
  <section class="card" aria-labelledby="admin-quiz-bank-title">
    <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h2 id="admin-quiz-bank-title" class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.playOps.quizBank.title') }}
          </h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.playOps.quizBank.hint') }}
          </p>
        </div>
        <div class="flex flex-col gap-2 sm:flex-row">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="() => loadQuestions()">
            {{ t('admin.playOps.quizBank.refresh') }}
          </button>
          <button type="button" class="btn btn-primary" @click="startCreate">
            {{ t('admin.playOps.quizBank.create') }}
          </button>
        </div>
      </div>
    </div>

    <div class="grid gap-4 border-b border-gray-100 p-5 dark:border-dark-700 sm:grid-cols-2 xl:grid-cols-5">
      <div v-for="card in statCards" :key="card.label" class="rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ card.label }}</p>
        <p class="mt-1 text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ card.value }}</p>
      </div>
    </div>

    <div class="grid gap-3 border-b border-gray-100 p-5 dark:border-dark-700 lg:grid-cols-6">
      <label class="grid gap-1 text-sm">
        <span class="text-gray-600 dark:text-gray-300">{{ t('admin.playOps.quizBank.language') }}</span>
        <select v-model="filters.language" class="input" @change="loadQuestions(1)">
          <option value="">{{ t('admin.playOps.quizBank.allLanguages') }}</option>
          <option value="zh">{{ t('admin.playOps.quizBank.chinese') }}</option>
          <option value="en">{{ t('admin.playOps.quizBank.english') }}</option>
        </select>
      </label>
      <label class="grid gap-1 text-sm">
        <span class="text-gray-600 dark:text-gray-300">{{ t('admin.playOps.quizBank.status') }}</span>
        <select v-model="filters.active" class="input" @change="loadQuestions(1)">
          <option value="">{{ t('admin.playOps.quizBank.allStatuses') }}</option>
          <option value="true">{{ t('admin.playOps.quizBank.enabled') }}</option>
          <option value="false">{{ t('admin.playOps.quizBank.disabled') }}</option>
        </select>
      </label>
      <label class="grid gap-1 text-sm">
        <span class="text-gray-600 dark:text-gray-300">{{ t('admin.playOps.quizBank.category') }}</span>
        <select v-model="filters.category" class="input" @change="loadQuestions(1)">
          <option value="">{{ t('admin.playOps.quizBank.allCategories') }}</option>
          <option v-for="category in categoryOptions" :key="category" :value="category">{{ category }}</option>
        </select>
      </label>
      <label class="grid gap-1 text-sm">
        <span class="text-gray-600 dark:text-gray-300">{{ t('admin.playOps.quizBank.difficulty') }}</span>
        <select v-model="filters.difficulty" class="input" @change="loadQuestions(1)">
          <option value="">{{ t('admin.playOps.quizBank.allDifficulties') }}</option>
          <option value="easy">{{ t('admin.playOps.quizBank.easy') }}</option>
          <option value="normal">{{ t('admin.playOps.quizBank.normal') }}</option>
          <option value="hard">{{ t('admin.playOps.quizBank.hard') }}</option>
        </select>
      </label>
      <label class="grid gap-1 text-sm lg:col-span-2">
        <span class="text-gray-600 dark:text-gray-300">{{ t('admin.playOps.quizBank.keyword') }}</span>
        <input
          v-model="filters.q"
          type="search"
          class="input"
          :placeholder="t('admin.playOps.quizBank.searchPlaceholder')"
          @keyup.enter="loadQuestions(1)"
        />
      </label>
    </div>

    <div class="grid gap-0 xl:grid-cols-[minmax(0,1fr)_420px]">
      <div class="overflow-x-auto border-b border-gray-100 dark:border-dark-700 xl:border-b-0 xl:border-r">
        <table class="min-w-full divide-y divide-gray-100 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
            <tr>
              <th class="px-4 py-3">{{ t('admin.playOps.quizBank.question') }}</th>
              <th class="px-4 py-3">{{ t('admin.playOps.quizBank.category') }}</th>
              <th class="px-4 py-3">{{ t('admin.playOps.quizBank.difficulty') }}</th>
              <th class="px-4 py-3">{{ t('admin.playOps.quizBank.language') }}</th>
              <th class="px-4 py-3">{{ t('admin.playOps.quizBank.status') }}</th>
              <th class="px-4 py-3 text-right">{{ t('admin.playOps.quizBank.actions') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="question in questions" :key="question.id" class="hover:bg-gray-50 dark:hover:bg-dark-800">
              <td class="max-w-xl px-4 py-3 align-top">
                <p class="font-medium text-gray-900 dark:text-white">{{ question.prompt }}</p>
                <p class="mt-1 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.playOps.quizBank.explanation') }}: {{ question.explanation || t('admin.playOps.quizBank.notProvided') }}
                </p>
              </td>
              <td class="px-4 py-3 align-top">{{ question.category || '-' }}</td>
              <td class="px-4 py-3 align-top">{{ difficultyLabel(question.difficulty) }}</td>
              <td class="px-4 py-3 align-top">{{ languageLabel(question.language) }}</td>
              <td class="px-4 py-3 align-top">
                <span
                  class="rounded-full px-2 py-0.5 text-xs font-medium"
                  :class="question.active ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-200' : 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-200'"
                >
                  {{ question.active ? t('admin.playOps.quizBank.enabled') : t('admin.playOps.quizBank.disabled') }}
                </span>
              </td>
              <td class="px-4 py-3 text-right align-top">
                <div class="flex justify-end gap-2">
                  <button type="button" class="text-primary-600 hover:text-primary-700 dark:text-primary-300" @click="editQuestion(question)">
                    {{ t('admin.playOps.quizBank.edit') }}
                  </button>
                  <button type="button" class="text-primary-600 hover:text-primary-700 dark:text-primary-300" @click="toggleQuestion(question)">
                    {{ question.active ? t('admin.playOps.quizBank.disable') : t('admin.playOps.quizBank.enable') }}
                  </button>
                  <button type="button" class="text-red-600 hover:text-red-700 dark:text-red-300" @click="deleteQuestion(question)">
                    {{ t('admin.playOps.quizBank.delete') }}
                  </button>
                </div>
              </td>
            </tr>
            <tr v-if="!questions.length">
              <td colspan="6" class="px-4 py-10 text-center text-gray-500">
                {{ loading ? t('admin.playOps.quizBank.loading') : t('admin.playOps.quizBank.empty') }}
              </td>
            </tr>
          </tbody>
        </table>

        <div class="flex flex-col gap-3 border-t border-gray-100 px-4 py-3 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <p class="text-sm text-gray-500">{{ t('admin.playOps.quizBank.pagination', { total, page }) }}</p>
          <div class="flex gap-2">
            <button type="button" class="btn btn-secondary" :disabled="page <= 1 || loading" @click="loadQuestions(page - 1)">{{ t('admin.playOps.quizBank.previous') }}</button>
            <button type="button" class="btn btn-secondary" :disabled="page * pageSize >= total || loading" @click="loadQuestions(page + 1)">{{ t('admin.playOps.quizBank.next') }}</button>
          </div>
        </div>
      </div>

      <aside class="p-5">
        <form class="space-y-4" @submit.prevent="saveQuestion">
          <div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ editingID ? t('admin.playOps.quizBank.editTitle') : t('admin.playOps.quizBank.createTitle') }}
            </h3>
            <p class="mt-1 text-xs text-gray-500">{{ t('admin.playOps.quizBank.formHint') }}</p>
          </div>

          <div class="grid gap-3 sm:grid-cols-2">
            <label class="grid gap-1 text-sm">
              <span class="text-gray-600 dark:text-gray-300">{{ t('admin.playOps.quizBank.language') }}</span>
              <select v-model="form.language" class="input">
                <option value="zh">{{ t('admin.playOps.quizBank.chinese') }}</option>
                <option value="en">{{ t('admin.playOps.quizBank.english') }}</option>
              </select>
            </label>
            <label class="grid gap-1 text-sm">
              <span class="text-gray-600 dark:text-gray-300">{{ t('admin.playOps.quizBank.difficulty') }}</span>
              <select v-model="form.difficulty" class="input">
                <option value="easy">{{ t('admin.playOps.quizBank.easy') }}</option>
                <option value="normal">{{ t('admin.playOps.quizBank.normal') }}</option>
                <option value="hard">{{ t('admin.playOps.quizBank.hard') }}</option>
              </select>
            </label>
          </div>

          <label class="grid gap-1 text-sm">
            <span class="text-gray-600 dark:text-gray-300">{{ t('admin.playOps.quizBank.category') }}</span>
            <input v-model="form.category" class="input" :placeholder="t('admin.playOps.quizBank.categoryPlaceholder')" />
          </label>

          <label class="grid gap-1 text-sm">
            <span class="text-gray-600 dark:text-gray-300">{{ t('admin.playOps.quizBank.prompt') }}</span>
            <textarea v-model="form.prompt" class="input min-h-[88px]" :placeholder="t('admin.playOps.quizBank.promptPlaceholder')" />
          </label>

          <div class="space-y-2">
            <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('admin.playOps.quizBank.options') }}</p>
            <label v-for="(_, index) in form.options" :key="index" class="grid gap-1 text-sm">
              <span class="text-xs text-gray-500">{{ t('admin.playOps.quizBank.option', { letter: optionLetters[index] }) }}</span>
              <input v-model="form.options[index]" class="input" :placeholder="t('admin.playOps.quizBank.option', { letter: optionLetters[index] })" />
            </label>
          </div>

          <label class="grid gap-1 text-sm">
            <span class="text-gray-600 dark:text-gray-300">{{ t('admin.playOps.quizBank.correctAnswer') }}</span>
            <select v-model.number="form.correct_index" class="input">
              <option v-for="(_, index) in form.options" :key="index" :value="index">
                {{ optionLetters[index] }}
              </option>
            </select>
          </label>

          <label class="grid gap-1 text-sm">
            <span class="text-gray-600 dark:text-gray-300">{{ t('admin.playOps.quizBank.answerExplanation') }}</span>
            <textarea v-model="form.explanation" class="input min-h-[88px]" :placeholder="t('admin.playOps.quizBank.explanationPlaceholder')" />
          </label>

          <div class="grid gap-3 sm:grid-cols-2">
            <label class="grid gap-1 text-sm">
              <span class="text-gray-600 dark:text-gray-300">{{ t('admin.playOps.quizBank.sortOrder') }}</span>
              <input v-model.number="form.sort_order" type="number" class="input" />
            </label>
            <label class="flex items-center gap-2 pt-6 text-sm text-gray-700 dark:text-gray-200">
              <input v-model="form.active" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              {{ t('admin.playOps.quizBank.enableQuestion') }}
            </label>
          </div>

          <div class="flex flex-col gap-2 sm:flex-row">
            <button type="submit" class="btn btn-primary" :disabled="saving">
              {{ saving ? t('admin.playOps.quizBank.saving') : t('admin.playOps.quizBank.save') }}
            </button>
            <button type="button" class="btn btn-secondary" :disabled="saving" @click="resetForm">
              {{ t('admin.playOps.quizBank.cancel') }}
            </button>
          </div>
        </form>
      </aside>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";

import adminPlayAPI, {
	type AdminQuizQuestion,
	type AdminQuizQuestionDifficulty,
	type AdminQuizQuestionInput,
	type AdminQuizQuestionLanguage,
	type AdminQuizQuestionStats,
} from "@/api/admin/play";
import { useAppStore } from "@/stores";

const appStore = useAppStore();
const { t } = useI18n();
const optionLetters = ["A", "B", "C", "D"];
const pageSize = 20;

const questions = ref<AdminQuizQuestion[]>([]);
const total = ref(0);
const page = ref(1);
const loading = ref(false);
const saving = ref(false);
const editingID = ref<number | null>(null);
const stats = ref<AdminQuizQuestionStats>({
  total: 0,
  active: 0,
  inactive: 0,
  zh_active: 0,
  en_active: 0,
  categories: [],
  difficulties: [],
});

const filters = reactive({
  language: "" as AdminQuizQuestionLanguage | "",
  active: "" as "" | "true" | "false",
  category: "",
  difficulty: "" as AdminQuizQuestionDifficulty | "",
  q: "",
});

const emptyForm = (): AdminQuizQuestionInput => ({
  language: "zh",
  prompt: "",
  options: ["", "", "", ""],
  correct_index: 0,
  category: "",
  difficulty: "normal",
  explanation: "",
  sort_order: 0,
  active: true,
});

const form = reactive<AdminQuizQuestionInput>(emptyForm());

const categoryOptions = computed(() => stats.value.categories ?? []);
const statCards = computed(() => [
  { label: t("admin.playOps.quizBank.total"), value: stats.value.total },
  { label: t("admin.playOps.quizBank.active"), value: stats.value.active },
  { label: t("admin.playOps.quizBank.inactive"), value: stats.value.inactive },
  { label: t("admin.playOps.quizBank.zhActive"), value: stats.value.zh_active },
  { label: t("admin.playOps.quizBank.enActive"), value: stats.value.en_active },
]);

function difficultyLabel(value: AdminQuizQuestionDifficulty | string): string {
  if (value === "easy") return t("admin.playOps.quizBank.easy");
  if (value === "hard") return t("admin.playOps.quizBank.hard");
  return t("admin.playOps.quizBank.normal");
}

function languageLabel(value: AdminQuizQuestionLanguage | string): string {
  return value === "zh" ? t("admin.playOps.quizBank.chinese") : t("admin.playOps.quizBank.english");
}

function assignForm(input: AdminQuizQuestionInput) {
  Object.assign(form, {
    ...input,
    options: [...input.options.slice(0, 4), "", "", "", ""].slice(0, 4),
  });
}

function resetForm() {
  editingID.value = null;
  assignForm(emptyForm());
}

function startCreate() {
  resetForm();
}

function editQuestion(question: AdminQuizQuestion) {
  editingID.value = question.id;
  assignForm({
    language: question.language,
    prompt: question.prompt,
    options: [...question.options],
    correct_index: question.correct_index,
    category: question.category,
    difficulty: question.difficulty,
    explanation: question.explanation,
    sort_order: question.sort_order,
    active: question.active,
  });
}

function buildParams(targetPage = page.value) {
  const active =
    filters.active === "" ? undefined : filters.active === "true";
  return {
    language: filters.language,
    active,
    category: filters.category || undefined,
    difficulty: filters.difficulty,
    q: filters.q.trim() || undefined,
    page: targetPage,
    page_size: pageSize,
  };
}

async function loadQuestions(targetPage = page.value) {
  loading.value = true;
  try {
    const result = await adminPlayAPI.listQuizQuestions(buildParams(targetPage));
    questions.value = result.items ?? [];
    total.value = result.total ?? 0;
    page.value = result.page || targetPage;
    stats.value = result.stats ?? stats.value;
  } catch (error) {
    appStore.showError(errorMessage(error, t("admin.playOps.quizBank.loadFailed")));
  } finally {
    loading.value = false;
  }
}

function validateForm(): string | null {
  if (!form.prompt.trim()) return t("admin.playOps.quizBank.promptRequired");
  if (form.options.length !== 4 || form.options.some((option) => !option.trim())) return t("admin.playOps.quizBank.optionsRequired");
  const normalized = form.options.map((option) => option.trim().toLowerCase());
  if (new Set(normalized).size !== normalized.length) return t("admin.playOps.quizBank.optionsUnique");
  if (form.correct_index < 0 || form.correct_index > 3) return t("admin.playOps.quizBank.correctInvalid");
  return null;
}

async function saveQuestion() {
  const invalid = validateForm();
  if (invalid) {
    appStore.showError(invalid);
    return;
  }
  saving.value = true;
  try {
    const payload: AdminQuizQuestionInput = {
      ...form,
      prompt: form.prompt.trim(),
      category: form.category.trim() || (form.language === "zh" ? "平台知识" : "Platform knowledge"),
      explanation: form.explanation.trim(),
      options: form.options.map((option) => option.trim()),
    };
    if (editingID.value) {
      await adminPlayAPI.updateQuizQuestion(editingID.value, payload);
      appStore.showSuccess(t("admin.playOps.quizBank.updated"));
    } else {
      await adminPlayAPI.createQuizQuestion(payload);
      appStore.showSuccess(t("admin.playOps.quizBank.created"));
    }
    resetForm();
    await loadQuestions(1);
  } catch (error) {
    appStore.showError(errorMessage(error, t("admin.playOps.quizBank.saveFailed")));
  } finally {
    saving.value = false;
  }
}

async function toggleQuestion(question: AdminQuizQuestion) {
  try {
    await adminPlayAPI.updateQuizQuestion(question.id, {
      language: question.language,
      prompt: question.prompt,
      options: [...question.options],
      correct_index: question.correct_index,
      category: question.category,
      difficulty: question.difficulty,
      explanation: question.explanation,
      sort_order: question.sort_order,
      active: !question.active,
    });
    appStore.showSuccess(question.active ? t("admin.playOps.quizBank.disabledSuccess") : t("admin.playOps.quizBank.enabledSuccess"));
    await loadQuestions();
  } catch (error) {
    appStore.showError(errorMessage(error, t("admin.playOps.quizBank.statusFailed")));
  }
}

async function deleteQuestion(question: AdminQuizQuestion) {
  if (!window.confirm(t("admin.playOps.quizBank.deleteConfirm", { prompt: question.prompt }))) return;
  try {
    await adminPlayAPI.deleteQuizQuestion(question.id);
    appStore.showSuccess(t("admin.playOps.quizBank.deleted"));
    await loadQuestions(page.value);
  } catch (error) {
    appStore.showError(errorMessage(error, t("admin.playOps.quizBank.deleteFailed")));
  }
}

function errorMessage(error: unknown, fallback: string): string {
  const response = (error as { response?: { data?: { message?: string; error?: string } } })?.response;
  return response?.data?.message || response?.data?.error || fallback;
}

onMounted(() => {
  void loadQuestions(1);
});
</script>
