import { jisudengHomeZh } from '../jisudeng-home.zh'
import common from './common'
import landing from './landing'
import { mergeLocaleMessages } from '../merge'

export default mergeLocaleMessages(landing, mergeLocaleMessages(common, {
  home: {
    jisudeng: jisudengHomeZh,
  },
}))
