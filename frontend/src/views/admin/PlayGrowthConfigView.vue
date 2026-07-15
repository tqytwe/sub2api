<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { adminAPI } from '@/api'
import type { PlayGrowthConfig } from '@/api/admin/play'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
const loading = ref(true)
const saving = ref(false)

const form = reactive<PlayGrowthConfig>({
  blindbox_pool: { version: '', cost: 0.5, rtp_cap: 0.9, tiers: [] },
  blindbox_paid_enabled: false,
  blindbox_region_enabled: false,
  team_max_members: 8,
  team_weekly_token_target: 100000,
  team_weekly_request_target: 20,
  public_activity_min_count: 1,
  founder_season_json: '{"name":"Founding Season","duration_weeks":6,"enabled":true}',
  growth_experiment_json: '{"holdout_pct":5,"enabled":false}',
})

const totalWeight = computed(() => form.blindbox_pool.tiers.reduce((sum, tier) => sum + Number(tier.weight || 0), 0))
const expectedReward = computed(() => form.blindbox_pool.tiers.reduce(
  (sum, tier) => sum + Number(tier.amount || 0) * Number(tier.weight || 0) / 10000,
  0,
))
const rtp = computed(() => form.blindbox_pool.cost > 0 ? expectedReward.value / form.blindbox_pool.cost : 0)
const valid = computed(() => totalWeight.value === 10000 && rtp.value <= form.blindbox_pool.rtp_cap + 1e-9)

async function load() {
  loading.value = true
  try {
    Object.assign(form, await adminAPI.play.getGrowthConfig())
  } catch {
    appStore.showError('加载增长配置失败')
  } finally {
    loading.value = false
  }
}

function addTier() {
  form.blindbox_pool.tiers.push({ amount: 0, weight: 1 })
}

function removeTier(index: number) {
  form.blindbox_pool.tiers.splice(index, 1)
}

async function save() {
  if (!valid.value || saving.value) return
  saving.value = true
  try {
    await adminAPI.play.updateGrowthConfig(JSON.parse(JSON.stringify(form)))
    appStore.showSuccess('增长配置已保存')
    await load()
  } catch (error: unknown) {
    const message = (error as { message?: string })?.message || '保存增长配置失败'
    appStore.showError(message)
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-6">
    <header class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">增长世界运营配置</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">奖池、团队目标、创始赛季和实验规则均采用版本化配置。</p>
      </div>
      <button type="button" class="btn btn-primary" :disabled="loading || saving || !valid" @click="save">
        {{ saving ? '保存中' : '保存配置' }}
      </button>
    </header>

    <div v-if="loading" class="card p-6 text-sm text-gray-500">加载中...</div>
    <template v-else>
      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
          <h2 class="font-semibold text-gray-900 dark:text-white">盲盒奖池</h2>
          <p class="mt-1 text-sm text-gray-500">权重必须合计 10000，实际 RTP 不得超过上限。</p>
        </div>
        <div class="space-y-5 p-6">
          <div class="grid gap-4 md:grid-cols-3">
            <label class="space-y-1 text-sm"><span>版本</span><input v-model="form.blindbox_pool.version" class="input" /></label>
            <label class="space-y-1 text-sm"><span>单次成本</span><input v-model.number="form.blindbox_pool.cost" type="number" min="0.01" step="0.01" class="input" /></label>
            <label class="space-y-1 text-sm"><span>RTP 上限</span><input v-model.number="form.blindbox_pool.rtp_cap" type="number" min="0.01" max="1" step="0.01" class="input" /></label>
          </div>
          <div class="overflow-x-auto border-y border-gray-200 dark:border-dark-600">
            <table class="min-w-full text-sm">
              <thead><tr class="text-left"><th class="px-3 py-2">奖励金额</th><th class="px-3 py-2">权重</th><th class="px-3 py-2">概率</th><th class="w-16"></th></tr></thead>
              <tbody>
                <tr v-for="(tier, index) in form.blindbox_pool.tiers" :key="index" class="border-t border-gray-100 dark:border-dark-700">
                  <td class="px-3 py-2"><input v-model.number="tier.amount" type="number" min="0" step="0.01" class="input" /></td>
                  <td class="px-3 py-2"><input v-model.number="tier.weight" type="number" min="1" step="1" class="input" /></td>
                  <td class="px-3 py-2 tabular-nums">{{ (tier.weight / 100).toFixed(2) }}%</td>
                  <td class="px-3 py-2"><button type="button" class="text-sm text-red-600" @click="removeTier(index)">删除</button></td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="flex flex-wrap items-center justify-between gap-3 text-sm">
            <button type="button" class="btn btn-secondary" @click="addTier">添加档位</button>
            <div :class="valid ? 'text-green-600' : 'text-red-600'">权重 {{ totalWeight }}/10000 · 期望 ${{ expectedReward.toFixed(4) }} · RTP {{ (rtp * 100).toFixed(2) }}%</div>
          </div>
          <div class="grid gap-4 md:grid-cols-2">
            <div class="flex items-center justify-between border-t pt-4 dark:border-dark-700"><span class="text-sm">允许付费开箱</span><Toggle v-model="form.blindbox_paid_enabled" /></div>
            <div class="flex items-center justify-between border-t pt-4 dark:border-dark-700"><span class="text-sm">地区合规已启用</span><Toggle v-model="form.blindbox_region_enabled" /></div>
          </div>
        </div>
      </section>

      <section class="card p-6">
        <h2 class="font-semibold text-gray-900 dark:text-white">Agent Team 与真实活动</h2>
        <div class="mt-4 grid gap-4 md:grid-cols-2">
          <label class="space-y-1 text-sm"><span>小队人数上限</span><input v-model.number="form.team_max_members" type="number" min="2" max="100" class="input" /></label>
          <label class="space-y-1 text-sm"><span>活动流最低请求数</span><input v-model.number="form.public_activity_min_count" type="number" min="1" class="input" /></label>
          <label class="space-y-1 text-sm"><span>团队周 Token 目标</span><input v-model.number="form.team_weekly_token_target" type="number" min="1" class="input" /></label>
          <label class="space-y-1 text-sm"><span>团队周请求目标</span><input v-model.number="form.team_weekly_request_target" type="number" min="1" class="input" /></label>
        </div>
      </section>

      <section class="card p-6">
        <h2 class="font-semibold text-gray-900 dark:text-white">赛季与实验</h2>
        <div class="mt-4 grid gap-4 lg:grid-cols-2">
          <label class="space-y-1 text-sm"><span>创始赛季 JSON</span><textarea v-model="form.founder_season_json" rows="8" class="input font-mono" /></label>
          <label class="space-y-1 text-sm"><span>A/B 实验 JSON</span><textarea v-model="form.growth_experiment_json" rows="8" class="input font-mono" /></label>
        </div>
      </section>
    </template>
  </div>
</template>
