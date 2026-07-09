import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { fetchPublicGrowthTeaser, type PublicGrowthTeaser } from '@/api/publicGrowthTeaser'

export function usePublicGrowthTeaser() {
  const { t } = useI18n()
  const teaser = ref<PublicGrowthTeaser | null>(null)
  const loaded = ref(false)

  async function load() {
    teaser.value = await fetchPublicGrowthTeaser()
    loaded.value = true
  }

  onMounted(() => {
    void load()
  })

  const perkLines = computed(() => {
    const g = teaser.value
    if (!g) return [] as string[]

    const lines: string[] = []
    if (g.signup_grant_enabled && g.signup_balance_usd > 0) {
      lines.push(
        t('home.jisudeng.hero.perks.signupCredit', {
          amount: g.signup_balance_usd.toFixed(2),
        }),
      )
    }
    if (g.checkin_enabled && (g.checkin_daily_reward ?? 0) > 0) {
      lines.push(
        t('home.jisudeng.hero.perks.dailyCheckin', {
          amount: (g.checkin_daily_reward ?? 0).toFixed(2),
        }),
      )
    }
    if (g.public_models_enabled && g.public_model_count > 0) {
      lines.push(t('home.jisudeng.hero.perks.modelCount', { count: g.public_model_count }))
    }
    if (g.affiliate_enabled && (g.affiliate_rebate_pct ?? 0) > 0) {
      lines.push(
        t('home.jisudeng.hero.perks.referral', {
          pct: Math.round(g.affiliate_rebate_pct ?? 0),
        }),
      )
    }
    if (g.payment_enabled) {
      lines.push(t('home.jisudeng.hero.perks.payPerToken'))
    }
    return lines
  })

  return { teaser, loaded, perkLines, reload: load }
}
