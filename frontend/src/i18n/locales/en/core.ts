import { jisudengHomeEn } from '../jisudeng-home.en'
import common from './common'
import landing from './landing'
import { mergeLocaleMessages } from '../merge'

export default mergeLocaleMessages(landing, mergeLocaleMessages(common, {
  home: {
    jisudeng: jisudengHomeEn,
  },
}))
