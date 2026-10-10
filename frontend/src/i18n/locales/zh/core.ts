import requestLedger from '../request-ledger.zh'
import { jisudengHomeZh } from '../jisudeng-home.zh'
import { jisudengAuthAsideZh } from '../jisudeng-auth-aside.zh'
import { platformLabels } from '../../platformLabels'
import common from './common'
import landing from './landing'
import { mergeLocaleMessages } from '../merge'

export default mergeLocaleMessages(landing, mergeLocaleMessages(common, {
  requestLedger,
  platform: platformLabels.zh,
  authAside: jisudengAuthAsideZh,
  home: {
    jisudeng: jisudengHomeZh,
  },
}))
