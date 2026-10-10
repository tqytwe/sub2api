export default {
  walletNotice: '这里引用已存在的钱包流水。预留金额不代表已经完成最终结算。', walletOperations: { video_balance_hold: '视频预留', video_balance_capture: '视频结算', video_balance_release: '视频释放', image_balance_hold: '图像预留', image_balance_capture: '图像结算', image_balance_release: '图像释放' },
  phase: '阶段', phases: { external_search: '外部搜索', ws_input: 'WS 持续输入', ws_observed: '已观察到的自动轮次', request: '请求发送', ws_connect: 'WS 建连', ws_control: 'WS 控制' },
  walletTransaction: '钱包流水',
  inputTokens: '输入 Token', outputTokens: '输出 Token',
  title: '请求台账', description: '分别查看执行、用量和结算状态。',
  notice: '未知用量不等于零费用。上线前缺失的历史记录无法在这里补全。',
  back: '使用记录', privateId: '私有请求 ID', request: '请求 / 时间', identity: '身份', userId: '用户 ID', keyId: '密钥 ID', accountId: '调度或母账户 ID',
  execution: '执行状态', usage: '用量状态', settlement: '结算状态', attempts: '尝试', all: '全部状态',
  start: '开始时间（本地）', end: '结束时间（不含）', filter: '筛选', reset: '重置', refresh: '刷新', detail: '详情',
  loadError: '暂时无法读取请求台账。已保留筛选条件，请稍后重试。',
  invalidDates: '结束时间必须晚于开始时间。', retry: '重试', empty: '没有符合筛选条件的请求。',
  anonymous: '匿名请求', parent: '父请求', began: '开始时间', ended: '终止时间', unfinished: '尚未结束',
  output: '已观察到上游响应字节', yes: '是', no: '暂无证据', http: 'HTTP 状态', errorCode: '安全错误枚举',
  account: '调度账户', credential: '凭据母账户', billing: '计费关联', noBilling: '尚无可验证的用量关联。',
  usageLog: '用量记录', pendingCost: '待核对', applied: '本请求完成结算', deduplicated: '引用已有结算，未重复扣费',
  subscription: '订阅', package: '套餐权益', amount: '已验证扣费金额', operations: '操作',
  states: { inflight: '处理中', succeeded: '已成功', failed: '失败', cancelled: '已取消', timeout: '已超时', interrupted: '已中断', not_applicable: '不适用', pending: '等待用量', known: '已知', usage_unknown: '用量未知', not_required: '无需结算', settlement_pending: '待结算', settled: '已结算' }
}
