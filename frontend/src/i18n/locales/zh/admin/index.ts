import overview from './overview'
import channels from './channels'
import scheduling from './scheduling'
import accounts from './accounts'
import resources from './resources'
import ops from './ops'
import settings from './settings'
import audit from './audit'
import playOps from './playOps'
import promptAudit from './promptAudit'
import plugins from './plugins'

export default {
  ...overview,
  ...channels,
  ...scheduling,
  ...accounts,
  ...resources,
  ...ops,
  ...settings,
  ...audit,
  ...playOps,
  ...promptAudit,
  ...plugins,
}
