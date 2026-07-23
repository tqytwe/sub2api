import { jisudengHomeZh } from '../jisudeng-home.zh'
import { jisudengAuthAsideZh } from '../jisudeng-auth-aside.zh'
import common from './common'
import landing from './landing'
import { mergeLocaleMessages } from '../merge'

export default mergeLocaleMessages(landing, mergeLocaleMessages(common, {
  authAside: jisudengAuthAsideZh,
  home: {
    jisudeng: jisudengHomeZh,
  },
}))
