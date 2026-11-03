import tokenGuardV2 from './tokenGuardV2'
import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import pelicanShowcase from './pelicanShowcase'
import pelicanTests from './pelicanTests'
import channelMonitorV2 from './channelMonitorV2'
import channelMonitorV3 from './channelMonitorV3'
import batchImage from './batchImage'
import admin from './admin'
import misc from './misc'

export default {
  tokenGuardV2,
  ...landing,
  ...common,
  ...dashboard,
  ...pelicanShowcase,
  pelicanTests,
  ...channelMonitorV2,
  ...channelMonitorV3,
  ...batchImage,
  admin,
  ...misc,
}
