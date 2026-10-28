import requestLedger from '../request-ledger.zh'
import { jisudengHomeZh } from '../jisudeng-home.zh'
import { jisudengAuthAsideZh } from '../jisudeng-auth-aside.zh'
import { platformLabels } from '../../platformLabels'
import common from './common'
import landing from './landing'
import { mergeLocaleMessages } from '../merge'
import tokenGuardV2 from '../zh/tokenGuardV2'

export default mergeLocaleMessages(landing, mergeLocaleMessages(common, {
  requestLedger,
  accountOps: { smartTitle: '账号运营' },
  tokenGuardV2,
  platform: platformLabels.zh,
  authAside: jisudengAuthAsideZh,
  home: {
    jisudeng: jisudengHomeZh,
  },
}))
