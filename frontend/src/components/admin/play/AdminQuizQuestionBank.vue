<template>
  <section class="card" aria-labelledby="admin-quiz-bank-title">
    <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h2 id="admin-quiz-bank-title" class="text-lg font-semibold text-gray-900 dark:text-white">
            答题题库管理
          </h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            管理每日答题的题目内容、分类、难度和解析；停用后不会再进入用户每日题目。
          </p>
        </div>
        <div class="flex flex-col gap-2 sm:flex-row">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="() => loadQuestions()">
            刷新题库
          </button>
          <button type="button" class="btn btn-primary" @click="startCreate">
            新增题目
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
        <span class="text-gray-600 dark:text-gray-300">语言</span>
        <select v-model="filters.language" class="input" @change="loadQuestions(1)">
          <option value="">全部语言</option>
          <option value="zh">中文</option>
          <option value="en">英文</option>
        </select>
      </label>
      <label class="grid gap-1 text-sm">
        <span class="text-gray-600 dark:text-gray-300">状态</span>
        <select v-model="filters.active" class="input" @change="loadQuestions(1)">
          <option value="">全部状态</option>
          <option value="true">启用</option>
          <option value="false">停用</option>
        </select>
      </label>
      <label class="grid gap-1 text-sm">
        <span class="text-gray-600 dark:text-gray-300">分类</span>
        <select v-model="filters.category" class="input" @change="loadQuestions(1)">
          <option value="">全部分类</option>
          <option v-for="category in categoryOptions" :key="category" :value="category">{{ category }}</option>
        </select>
      </label>
      <label class="grid gap-1 text-sm">
        <span class="text-gray-600 dark:text-gray-300">难度</span>
        <select v-model="filters.difficulty" class="input" @change="loadQuestions(1)">
          <option value="">全部难度</option>
          <option value="easy">简单</option>
          <option value="normal">普通</option>
          <option value="hard">进阶</option>
        </select>
      </label>
      <label class="grid gap-1 text-sm lg:col-span-2">
        <span class="text-gray-600 dark:text-gray-300">关键词</span>
        <input
          v-model="filters.q"
          type="search"
          class="input"
          placeholder="搜索题干或解析"
          @keyup.enter="loadQuestions(1)"
        />
      </label>
    </div>

    <div class="grid gap-0 xl:grid-cols-[minmax(0,1fr)_420px]">
      <div class="overflow-x-auto border-b border-gray-100 dark:border-dark-700 xl:border-b-0 xl:border-r">
        <table class="min-w-full divide-y divide-gray-100 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
            <tr>
              <th class="px-4 py-3">题目</th>
              <th class="px-4 py-3">分类</th>
              <th class="px-4 py-3">难度</th>
              <th class="px-4 py-3">语言</th>
              <th class="px-4 py-3">状态</th>
              <th class="px-4 py-3 text-right">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="question in questions" :key="question.id" class="hover:bg-gray-50 dark:hover:bg-dark-800">
              <td class="max-w-xl px-4 py-3 align-top">
                <p class="font-medium text-gray-900 dark:text-white">{{ question.prompt }}</p>
                <p class="mt-1 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">
                  解析：{{ question.explanation || '暂未填写' }}
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
                  {{ question.active ? '启用' : '停用' }}
                </span>
              </td>
              <td class="px-4 py-3 text-right align-top">
                <div class="flex justify-end gap-2">
                  <button type="button" class="text-primary-600 hover:text-primary-700 dark:text-primary-300" @click="editQuestion(question)">
                    编辑
                  </button>
                  <button type="button" class="text-primary-600 hover:text-primary-700 dark:text-primary-300" @click="toggleQuestion(question)">
                    {{ question.active ? '停用' : '启用' }}
                  </button>
                  <button type="button" class="text-red-600 hover:text-red-700 dark:text-red-300" @click="deleteQuestion(question)">
                    删除
                  </button>
                </div>
              </td>
            </tr>
            <tr v-if="!questions.length">
              <td colspan="6" class="px-4 py-10 text-center text-gray-500">
                {{ loading ? '题库加载中' : '暂无符合条件的题目' }}
              </td>
            </tr>
          </tbody>
        </table>

        <div class="flex flex-col gap-3 border-t border-gray-100 px-4 py-3 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <p class="text-sm text-gray-500">共 {{ total }} 题，第 {{ page }} 页</p>
          <div class="flex gap-2">
            <button type="button" class="btn btn-secondary" :disabled="page <= 1 || loading" @click="loadQuestions(page - 1)">上一页</button>
            <button type="button" class="btn btn-secondary" :disabled="page * pageSize >= total || loading" @click="loadQuestions(page + 1)">下一页</button>
          </div>
        </div>
      </div>

      <aside class="p-5">
        <form class="space-y-4" @submit.prevent="saveQuestion">
          <div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ editingID ? '编辑题目' : '新增题目' }}
            </h3>
            <p class="mt-1 text-xs text-gray-500">保存后启用的题目会进入用户每日答题候选池。</p>
          </div>

          <div class="grid gap-3 sm:grid-cols-2">
            <label class="grid gap-1 text-sm">
              <span class="text-gray-600 dark:text-gray-300">语言</span>
              <select v-model="form.language" class="input">
                <option value="zh">中文</option>
                <option value="en">英文</option>
              </select>
            </label>
            <label class="grid gap-1 text-sm">
              <span class="text-gray-600 dark:text-gray-300">难度</span>
              <select v-model="form.difficulty" class="input">
                <option value="easy">简单</option>
                <option value="normal">普通</option>
                <option value="hard">进阶</option>
              </select>
            </label>
          </div>

          <label class="grid gap-1 text-sm">
            <span class="text-gray-600 dark:text-gray-300">分类</span>
            <input v-model="form.category" class="input" placeholder="例如：优惠券、计费知识、API 基础" />
          </label>

          <label class="grid gap-1 text-sm">
            <span class="text-gray-600 dark:text-gray-300">题干</span>
            <textarea v-model="form.prompt" class="input min-h-[88px]" placeholder="请输入题目内容" />
          </label>

          <div class="space-y-2">
            <p class="text-sm text-gray-600 dark:text-gray-300">选项</p>
            <label v-for="(_, index) in form.options" :key="index" class="grid gap-1 text-sm">
              <span class="text-xs text-gray-500">选项 {{ optionLetters[index] }}</span>
              <input v-model="form.options[index]" class="input" :placeholder="`选项 ${optionLetters[index]}`" />
            </label>
          </div>

          <label class="grid gap-1 text-sm">
            <span class="text-gray-600 dark:text-gray-300">正确答案</span>
            <select v-model.number="form.correct_index" class="input">
              <option v-for="(_, index) in form.options" :key="index" :value="index">
                {{ optionLetters[index] }}
              </option>
            </select>
          </label>

          <label class="grid gap-1 text-sm">
            <span class="text-gray-600 dark:text-gray-300">答案解析</span>
            <textarea v-model="form.explanation" class="input min-h-[88px]" placeholder="用户答完后可用于解释规则或平台知识" />
          </label>

          <div class="grid gap-3 sm:grid-cols-2">
            <label class="grid gap-1 text-sm">
              <span class="text-gray-600 dark:text-gray-300">排序</span>
              <input v-model.number="form.sort_order" type="number" class="input" />
            </label>
            <label class="flex items-center gap-2 pt-6 text-sm text-gray-700 dark:text-gray-200">
              <input v-model="form.active" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              启用题目
            </label>
          </div>

          <div class="flex flex-col gap-2 sm:flex-row">
            <button type="submit" class="btn btn-primary" :disabled="saving">
              {{ saving ? '保存中' : '保存题目' }}
            </button>
            <button type="button" class="btn btn-secondary" :disabled="saving" @click="resetForm">
              取消编辑
            </button>
          </div>
        </form>
      </aside>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";

import adminPlayAPI, {
	type AdminQuizQuestion,
	type AdminQuizQuestionDifficulty,
	type AdminQuizQuestionInput,
	type AdminQuizQuestionLanguage,
	type AdminQuizQuestionStats,
} from "@/api/admin/play";
import { useAppStore } from "@/stores";

const appStore = useAppStore();
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
  category: "平台知识",
  difficulty: "normal",
  explanation: "",
  sort_order: 0,
  active: true,
});

const form = reactive<AdminQuizQuestionInput>(emptyForm());

const categoryOptions = computed(() => stats.value.categories ?? []);
const statCards = computed(() => [
  { label: "题库总量", value: stats.value.total },
  { label: "启用题目", value: stats.value.active },
  { label: "停用题目", value: stats.value.inactive },
  { label: "中文启用", value: stats.value.zh_active },
  { label: "英文启用", value: stats.value.en_active },
]);

function difficultyLabel(value: AdminQuizQuestionDifficulty | string): string {
  if (value === "easy") return "简单";
  if (value === "hard") return "进阶";
  return "普通";
}

function languageLabel(value: AdminQuizQuestionLanguage | string): string {
  return value === "zh" ? "中文" : "英文";
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
    appStore.showError(errorMessage(error, "题库加载失败"));
  } finally {
    loading.value = false;
  }
}

function validateForm(): string | null {
  if (!form.prompt.trim()) return "题干不能为空";
  if (form.options.length !== 4 || form.options.some((option) => !option.trim())) return "必须填写 4 个选项";
  const normalized = form.options.map((option) => option.trim().toLowerCase());
  if (new Set(normalized).size !== normalized.length) return "选项不能重复";
  if (form.correct_index < 0 || form.correct_index > 3) return "正确答案无效";
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
      category: form.category.trim() || "平台知识",
      explanation: form.explanation.trim(),
      options: form.options.map((option) => option.trim()),
    };
    if (editingID.value) {
      await adminPlayAPI.updateQuizQuestion(editingID.value, payload);
      appStore.showSuccess("题目已保存");
    } else {
      await adminPlayAPI.createQuizQuestion(payload);
      appStore.showSuccess("题目已新增");
    }
    resetForm();
    await loadQuestions(1);
  } catch (error) {
    appStore.showError(errorMessage(error, "题目保存失败"));
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
    appStore.showSuccess(question.active ? "题目已停用" : "题目已启用");
    await loadQuestions();
  } catch (error) {
    appStore.showError(errorMessage(error, "状态修改失败"));
  }
}

async function deleteQuestion(question: AdminQuizQuestion) {
  if (!window.confirm(`确定删除题目「${question.prompt}」吗？删除后不会再进入题库。`)) return;
  try {
    await adminPlayAPI.deleteQuizQuestion(question.id);
    appStore.showSuccess("题目已删除");
    await loadQuestions(page.value);
  } catch (error) {
    appStore.showError(errorMessage(error, "题目删除失败"));
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
