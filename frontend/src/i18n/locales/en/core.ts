import { jisudengHomeEn } from '../jisudeng-home.en'
import { jisudengAuthAsideEn } from '../jisudeng-auth-aside.en'
import common from './common'
import landing from './landing'
import { mergeLocaleMessages } from '../merge'

export default mergeLocaleMessages(landing, mergeLocaleMessages(common, {
  authAside: jisudengAuthAsideEn,
  home: {
    jisudeng: jisudengHomeEn,
  },
}))
