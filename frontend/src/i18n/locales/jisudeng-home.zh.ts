/** 极速蹬首页文案（定制主题） */
export const jisudengHomeZh = {
  nav: {
    models: '模型列表',
    docs: '使用文档',
    about: '了解极速蹬',
    contact: '联系我们',
    admin: '管理后台',
    console: '控制台',
    signIn: '登录',
    signUp: '注册'
  },
  hero: {
    eyebrow: '最注重隐私的中转站 · 拒绝数据倒卖 · 拒绝模型掺水',
    titleParts: {
      brand: '极速蹬',
      mid: 'AI',
      tail: '网关'
    },
    tagline: '链接一切 · 让您一念之间 · 极速蹬世界',
    activeOn: 'WORKS ON'
  },
  cta: {
    start: '即刻开始',
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
      '我们对待你的 prompt 像对待密信：在内存里存在的时间不超过一次响应周期，响应完成即从内存释放，不写日志、不入数据库、不参与训练、不与任何第三方共享。',
    integrity: '响应完整性已验证',
    pledges: {
      p1: '私密 · 请求即焚',
      p2: '忠实 · 一字不改',
      p3: '透明 · 原价对账',
      p4: '迅捷 · 毫秒调度',
      d1: '请求体不写日志、不落库；响应结束的瞬间即从内存释放。不参与模型训练，不与第三方分析、监控、广告平台共享。',
      d2: '上游响应字节级转发，不做提示词改写、不蒸馏、不压缩、不静默切换替代模型。每次调用附签名摘要，可在「使用记录」中逐条核对。',
      d3: '每条计费按上游官方原价计算，只乘以你在「模型与价格」页看到的那个倍率。无隐形倍率、无隐形费用、不另外加价。',
      d4: '调度链路 sub-100ms 完成，首字时间与官方直连持平。可在「服务状态」页查看实时延迟、可用率与历史抖动。'
    }
  },
  stats: {
    requests: '累计 API 请求',
    uptime: '服务可用率',
    latency: '平均首字延迟'
  },
  sections: {
    imageTag: 'IMAGE API',
    imageTitle: '一句话，生成一张图。',
    imageLede:
      'gpt-image-2 完整接入，文生图与图像编辑共用同一个 Key。OpenAI 兼容协议，base_url 指过来即用。',
    featuresTag: 'WHY JISU-DENG',
    featuresTitle: '少一些妨碍，多一份思维流速。',
    codeTag: 'ONE-CLICK SETUP',
    codeTitle: '一行代码都不用写。',
    codeLede: '控制台复制一条命令贴进终端——检测环境、写配置、自检，脚本替你全办了。',
    pricingTag: 'FAIR BILLING'
  },
  image: {
    model: 'gpt-image-2',
    badge: 'NEW',
    desc: '两个端点：文生图 (generations) 与图像编辑 (edits)。单次请求最大 30 MiB。',
    caps: ['文生图', '图生图', '多图参考', '风格迁移'],
    docLink: '查看图像 API 文档',
    promptLabel: 'PROMPT',
    promptText: '一只在东京雨夜霓虹下的柴犬，赛博朋克风格',
    statusGen: 'Generating',
    statusMeta: 'gpt-image-2 · 1024×1024',
    statusDone: '完成 · 1.2s',
    demoBadge: 'OpenAI 兼容 · 同一 Key'
  },
  channels: {
    tag: 'PLAY · 趣味功能',
    title: '不止 API，还有得玩。',
    copyTitle: 'Token 不该只是消耗品。',
    copyBody:
      '极速蹬更想把它做成一种趣味——让每一次消耗都多出一点意义：开出的盲盒、签下的日子、流回的佣金、长出的收成。',
    next: '换台',
    enter: '进入频道',
    ch1: { name: '盲盒', hint: '消耗 Token 开盲盒，随机奖励余额' },
    ch2: { name: '签到', hint: '每日签到领测试额度' },
    ch3: { name: 'Agent Team', hint: '组队协作，共享收益' },
    ch4: { name: 'Token 农场', hint: '消耗排行，每日奖励' }
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
    s1t: '复制命令',
    s1d: '控制台「API 密钥」页点一键部署，复制那一条命令',
    s2t: '粘贴进终端',
    s2d: '脚本自动检测环境、写入配置并完成自检',
    s3t: '开始对话',
    s3d: '终端、IDE、桌面客户端，接好即用',
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
    sub: 'GLOBAL · MULTI-MODEL · API GATEWAY'
  },
  footer: {
    tagline: 'AI 中转站',
    docs: '接入文档'
  }
}
