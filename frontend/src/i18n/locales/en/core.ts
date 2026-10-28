import requestLedger from '../request-ledger.en'
import { jisudengHomeEn } from '../jisudeng-home.en'
import { jisudengAuthAsideEn } from '../jisudeng-auth-aside.en'
import { platformLabels } from '../../platformLabels'
import common from './common'
import landing from './landing'
import { mergeLocaleMessages } from '../merge'
import tokenGuardV2 from '../en/tokenGuardV2'

export default mergeLocaleMessages(landing, mergeLocaleMessages(common, {
  requestLedger,
  accountOps: { smartTitle: 'Account operations' },
  tokenGuardV2,
  platform: platformLabels.en,
  authAside: jisudengAuthAsideEn,
  home: {
    jisudeng: jisudengHomeEn,
  },
}))
