/** Reused management-shell state labels. Feature pages can override or extend them. */
export default {
  status: {
    unknown: '未知状态',
  },
  admin: {
    accounts: {
      status: {
        active: '正常',
        inactive: '未启用',
        expired: '已过期',
        error: '错误',
        cooldown: '冷却中',
        paused: '已暂停',
        limited: '受限',
        rateLimited: '限流中',
        overloaded: '过载',
        tempUnschedulable: '暂不可调度',
        quotaExceeded: '配额已耗尽',
        unschedulable: '不可调度',
        unknown: '未知状态',
      },
    },
  },
}
