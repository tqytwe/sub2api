import { jisudengHomeEn } from '../jisudeng-home.en'
import { jisudengAuthAsideEn } from '../jisudeng-auth-aside.en'
import { platformLabels } from '../../platformLabels'
import common from './common'
import landing from './landing'
import { mergeLocaleMessages } from '../merge'

export default mergeLocaleMessages(landing, mergeLocaleMessages(common, {
  platform: platformLabels.en,
  authAside: jisudengAuthAsideEn,
  home: {
    jisudeng: jisudengHomeEn,
  },
}))
