const fs = require('node:fs');
const changes = {
 'frontend/src/views/user/CheckInView.vue': s => s
  .replace("status.value?.growth_energy_enabled || growthEligibility.value?.reward_mode === 'energy'", "status.value?.growth_energy_enabled ?? growthEligibility.value?.reward_mode === 'energy'")
  .replace('const governanceBlocked = computed(', "const participationOnlyMode = computed(\n  () => !growthEnergyMode.value && growthEligibility.value?.reward_mode === 'energy',\n)\nconst governanceBlocked = computed(")
  .replace('!growthEnergyMode.value && status.value?.growth_governance_available', '!growthEnergyMode.value && !participationOnlyMode.value && status.value?.growth_governance_available')
  .replace("  if (result.reward_type === 'coupon'", "  if (result.reward_type === 'none') return t('checkin.participationOnlySuccess')\n  if (result.reward_type === 'coupon'")
  .replace("growthEnergyMode ? t('checkin.energyHint') : t('checkin.randomRewardHint')", "participationOnlyMode ? t('checkin.participationOnlyHint') : growthEnergyMode ? t('checkin.energyHint') : t('checkin.randomRewardHint')")
  .replace('<div v-else-if="growthEnergyMode"', '<div v-else-if="participationOnlyMode || growthEnergyMode"')
  .replace("{{ t('checkin.energyTitle') }}", "{{ participationOnlyMode ? t('checkin.participationOnlyTitle') : t('checkin.energyTitle') }}")
  .replace('v-if="remainingMilestoneDays"', 'v-if="remainingMilestoneDays && !participationOnlyMode"'),
 'frontend/src/views/user/PlayHubView.vue': s => s
  .replace("return status?.growth_eligibility?.reward_mode === 'energy' || status?.growth_energy_enabled === true", "return status?.growth_energy_enabled ?? status?.growth_eligibility?.reward_mode === 'energy'")
  .replace(': isGrowthEnergyMode(c)', ": c?.growth_energy_enabled === false && c?.growth_eligibility?.reward_mode === 'energy'\n          ? t('checkin.participationOnlyHint')\n          : isGrowthEnergyMode(c)"),
 'frontend/src/components/admin/play/AdminCheckinRecords.vue': s => s
  .replace("text('奖励模式', 'Reward mode')", "text('领奖资格', 'Reward eligibility')")
  .replace("<option value=\"energy\">{{ text('成长能量', 'Growth energy') }}</option>", "<option value=\"energy\">{{ text('未达领奖资格', 'Not reward eligible') }}</option>")
  .replace('modeLabel(row.reward_mode)', 'modeLabel(row.reward_mode, row.growth_energy)')
  .replace("function modeLabel(mode: string) { return mode === 'energy' ? text('成长能量', 'Growth energy')", "function modeLabel(mode: string, energy = 0) { return mode === 'energy' ? energy > 0 ? text('历史成长能量', 'Historical growth energy') : text('仅签到（未达领奖资格）', 'Check-in only (not reward eligible)')"),
};
const messages = {
 zh: {participationOnlyHint:'当前未达领奖资格，本次仅记录签到和连续天数，不发放奖励；达标后的签到按已发布奖池发奖。',participationOnlyTitle:'仅记录签到，暂不发奖',participationOnlySuccess:'签到已记录，连续天数已更新；本次不发放奖励'},
 en: {participationOnlyHint:'You are not reward eligible yet. This check-in records participation and your streak without a reward. Future eligible check-ins use the published pool.',participationOnlyTitle:'Check-in only; no reward',participationOnlySuccess:'Check-in and streak recorded. No reward was issued.'},
};
function updateLocale(source, language) {const prefix='    randomRewardHint:';const entries=Object.entries(messages[language]).map(([k,v])=>'    '+k+': '+JSON.stringify(v)+',').join('\n');return source.replace(prefix,entries+'\n'+prefix);}
module.exports={changes,messages,updateLocale};
if(require.main===module){for(const [file,change] of Object.entries(changes))fs.writeFileSync(file,change(fs.readFileSync(file,'utf8')));for(const language of ['zh','en']){const p=`frontend/src/i18n/locales/jisudeng-pages.${language}.ts`;fs.writeFileSync(p,updateLocale(fs.readFileSync(p,'utf8'),language));}}
