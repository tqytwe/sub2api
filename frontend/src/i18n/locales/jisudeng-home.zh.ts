/** 极速蹬首页文案（定制主题）— 中文 GTM 同步 */
export const jisudengHomeZh = {
  nav: {
    models: '模型与价格',
    docs: '使用文档',
    about: '了解极速蹬',
    contact: '联系我们',
    admin: '管理后台',
    console: '控制台',
    signIn: '登录',
    signUp: '免费注册'
  },
  hero: {
    eyebrow: '最注重隐私的中转站 · 拒绝数据倒卖 · 拒绝模型掺水',
    titleParts: {
      brand: '极速蹬',
      mid: 'AI',
      tail: '网关'
    },
    subtitle: '一个 API Key · Claude / GPT / Gemini · 按 Token 计费 · 无月租',
    tagline: '注册即用 · 官方倍率 · 提示词不落盘',
    activeOn: 'WORKS ON',
    perks: {
      signupCredit: '注册即送 ${amount} 测试额度',
      dailyCheckin: '每日签到 · ${amount} 余额',
      modelCount: '公开定价 {count}+ 款模型',
      referral: '邀请返利 {pct}%',
      payPerToken: '自助充值 · 余额永不过期'
    }
  },
  cta: {
    start: '免费领 API Key',
    register: '免费注册',
    console: '进入控制台',
    docs: '看接入文档',
    viewPrice: '查看模型与价格'
  },
  manifesto: {
    tag: 'OUR PROMISE · 我们的承诺',
    title: '一字不改的 Token，一分不掺的账单',
    body1:
      '极速蹬 API 网关只做一件事——把你的请求按官方协议原样转发给上游模型，响应再原样回到你手里。不蒸馏、不压缩、不替换模型，你收到的每一个 token 都来自你声明的那个模型本身。',
    body2:
      '同步请求正文不写日志，响应完成即释放；需要恢复的异步与 Batch 任务只加密或受控保留必要数据，并按保留期清理。不参与训练，不与分析或广告平台共享。',
    verifyLink: '在使用记录中查看验证方式 →',
    pledges: {
      p1: '私密 · 最小留存',
      p2: '忠实 · 一字不改',
      p3: '透明 · 原价对账',
      p4: '迅捷 · 毫秒调度',
      d1: '同步请求体不写日志、不持久化；异步任务只保留恢复所需的加密或受控数据并按期清理。不参与模型训练，不与第三方分析、监控、广告平台共享。',
      d2: '上游响应字节级转发，不做提示词改写、不蒸馏、不压缩、不静默切换替代模型。',
      d3: '每条计费按上游官方原价计算，只乘以你在「模型与价格」页看到的那个倍率。无隐形倍率、无隐形费用、不另外加价。',
      d4: '调度链路 sub-100ms 完成，首字时间与官方直连持平。可在「服务状态」页查看实时延迟、可用率与历史抖动。'
    }
  },
  stats: {
    requests: '累计已记录 API 请求',
    uptime: '30 天可用率',
    latency: '24 小时平均首字延迟',
    models: '可用模型数',
    through: '最近运营样本结束于 {time}',
    computed: '快照生成于 {time}',
    stale: '当前显示缓存或滞后数据'
  },
  sections: {
    imageTag: 'IMAGE API',
    imageTitle: '一句话，生成一张图。',
    imageLede:
      '同步 Images 支持生成、编辑与一次多张；单请求异步避开长连接超时，Batch 负责多个 prompt 的持久任务。统一使用 API Key 接入。',
    featuresTag: 'WHY JISU-DENG',
    featuresTitle: '少一些妨碍，多一份思维流速。',
    codeTag: '3 分钟接入',
    codeTitle: '复制一条命令，立刻开调。',
    codeLede: '控制台一键部署——检测环境、写配置、自检，脚本替你全办了。',
    pricingTag: 'FAIR BILLING',
    faqTag: '常见问题',
    faqTitle: '开发者最常问的几件事'
  },
  faq: {
    items: [
      {
        q: '你们会存储我的 prompt 吗？',
        a: '同步请求正文不写日志、不持久化，响应完成即释放。Gateway 异步与 Batch 为任务恢复会加密或受控暂存必要请求数据，并按保留期清理。'
      },
      {
        q: '怎么确认调用的是我要的模型？',
        a: '上游响应字节级转发，不做静默替换。使用记录里可查模型名、Token 数与响应 ID，逐条核对。'
      },
      {
        q: '注册需要绑卡吗？',
        a: '不需要。邮箱或 GitHub/Google 注册即可。若开启注册赠送与每日签到，可先试后充。'
      },
      {
        q: '计费怎么算？',
        a: '按 Token 与公开倍率结算，无月租、无席位费、无最低消费，余额不过期。'
      },
      {
        q: '哪些工具能直接用？',
        a: '任意 OpenAI 兼容客户端：Claude Code、Cursor、Codex CLI、Cline、Gemini CLI、Continue 等，改 base_url 和 Key 即可。'
      },
      {
        q: '「用量回馈」是什么？',
        a: '可选玩法：每日签到、排行榜、邀请返利、VIP 等级等，在公平 API 计费之上叠加。不用这些也不影响正常调用。'
      }
    ]
  },
  image: {
    model: 'gpt-image-2',
    badge: 'NEW',
    desc: '同步 n=1-10、单请求异步与 Batch 多 prompt 均有明确接口；每个 multipart 字段最大 20 MiB。',
    caps: ['文生图', '图生图', '多图参考', '风格迁移'],
    docLink: '查看图像 API 文档',
    studioCta: '免费试做一张',
    promptLabel: 'PROMPT',
    promptText: '一只在东京雨夜霓虹下的柴犬，赛博朋克风格',
    statusGen: 'Generating',
    statusMeta: 'gpt-image-2 · 1024×1024',
    statusDone: '完成 · 1.2s',
    demoBadge: 'OpenAI 兼容 · 同一 Key'
  },
  channels: {
    tag: '用量回馈',
    title: '不止 API，还有得玩。',
    copyTitle: 'Token 不该只是消耗品。',
    copyBody:
      '每日签到、排行榜、邀请返利、VIP 等级——在公平 API 计费之上，让每一次消耗多一点回馈。',
    next: '换台',
    enter: '进入频道',
    joinCta: '注册参与',
    ch1: { name: '盲盒', hint: '消耗 Token 开盲盒，随机奖励余额' },
    ch2: { name: '签到', hint: '每日签到领测试额度' },
    ch3: { name: 'Agent Team', hint: '组队协作，共享收益' },
    ch4: { name: 'Token 农场', hint: '每日任务有奖 · 每月赛季大奖' }
  },
  features: {
    multiModel: {
      title: '多模型聚合',
      desc: 'Claude / GPT / Gemini 等主流模型一站接入，统一 OpenAI 兼容协议，免在多个平台间切换。'
    },
    stable: {
      title: '稳定可靠',
      desc: '多线路冗余、跨区域容灾、自动故障切换，长链路 SSE 不中断。99.9% 可用性，关键调用从不掉队。'
    },
    privacy: {
      title: '隐私至上',
      desc: '请求体不落盘、不用于模型训练、不与第三方共享。全链路 TLS，日志仅保留必要计费摘要。你的 prompt 只属于你。'
    },
    instant: {
      title: '极速接入',
      desc: '注册即开 API，无需审批，无需绑卡。第一次充值即可立即开始调用，凭据自助管理。'
    },
    transparent: {
      title: '透明计费',
      desc: '按官方 Token 倍率计价。每一次调用都有用量日志、余额变动与原始响应，可逐条回溯。'
    },
    selfService: {
      title: '自助充值',
      desc: '支持主流支付方式，余额可视化，接近阈值自动提醒。卡密兑换、邀请返利同步可用。'
    }
  },
  onboard: {
    s1t: '注册并创建密钥',
    s1d: '邮箱或 OAuth 注册——在控制台创建第一个 API Key',
    s2t: '复制部署命令',
    s2d: '一键脚本为 Claude Code、Cursor 或 Codex 写入配置',
    s3t: '发出第一次调用',
    s3d: 'OpenAI 兼容 SDK，改 base_url 即可开调',
    docLink: '想直接调 API?',
    docLinkCta: '看接入文档'
  },
  pricing: {
    lineA: '按 Token 计费，',
    lineB: '不按月、不按席、',
    lineC: '见账见量。',
    blurb:
      '没有月租，没有最低消费，也没有"为你打包却用不完的额度"。所有调用按官方 Token 倍率结算，实时可查。',
    tags: ['官方倍率结算 · 可逐条对账', '无月费 · 无最低消费', '余额永不过期 · 充值无门槛']
  },
  closer: {
    title: '一念既起 · 模型即至',
    sub: 'ONE KEY · MULTI-MODEL · VERIFIABLE BILLING'
  },
  footer: {
    tagline: 'AI 中转站',
    docs: '接入文档'
  },
  registerBanner: {
    signupCredit: '现在注册 — 送 ${amount} 测试额度，全模型可试。',
    checkin: '另享每日签到 ${amount} 余额。',
    cta: '创建免费账号'
  },
  anchors: {
    manifesto: '承诺',
    stats: '数据',
    image: '图像',
    channels: '渠道',
    features: '优势',
    onboard: '接入',
    pricing: '计费',
    faq: '问答',
    closer: '开始',
    backToTop: '回到顶部'
  }
}
