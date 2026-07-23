/** 极速蹬 home page copy (custom theme) — global English GTM */
export const jisudengHomeEn = {
  nav: {
    models: 'Models & Pricing',
    docs: 'Docs',
    creation: 'AI Studio',
    prompts: 'Prompts',
    keyUsage: 'API Audit',
    about: 'About',
    contact: 'Contact',
    admin: 'Admin',
    console: 'Console',
    signIn: 'Sign in',
    signUp: 'Register free'
  },
  hero: {
    eyebrow: 'Privacy-first · No data resale · No model dilution',
    titleParts: {
      brand: 'Jisudeng',
      mid: 'One API',
      tail: 'for AI models'
    },
    subtitle: 'Access DeepSeek, Qwen, Kimi, GLM, GPT, Claude, Gemini and more through one OpenAI-compatible API.',
    tagline: 'Register in minutes. Official token rates. Prompts never stored.',
    activeOn: 'WORKS WITH',
    perks: {
      signupCredit: '${amount} free credits on signup',
      dailyCheckin: 'Daily check-in · ${amount} balance',
      modelCount: '{count}+ models on public pricing',
      referral: '{pct}% referral rewards',
      payPerToken: 'Stripe top-up · Balance never expires'
    }
  },
  cta: {
    start: 'Get API key — free',
    register: 'Register free',
    console: 'Open console',
    docs: 'Read docs',
    viewPrice: 'View models & pricing'
  },
  manifesto: {
    tag: 'OUR PROMISE',
    title: 'Unchanged tokens, unmixed bills',
    body1:
      'We forward your requests to upstream models exactly as specified — no distillation, compression, or silent model swaps.',
    body2:
      'Synchronous request bodies are not logged and are released after the response. Async and Batch jobs keep only the encrypted or access-controlled data required for recovery until retention cleanup. No training or analytics-ad sharing.',
    verifyLink: 'See how to verify in usage logs →',
    pledges: {
      p1: 'Private · Minimal retention',
      p2: 'Faithful · Byte-for-byte',
      p3: 'Transparent · Official rates',
      p4: 'Fast · Sub-100ms routing',
      d1: 'Synchronous request bodies are not logged or persisted. Async jobs retain only recovery data under encryption or access controls until scheduled cleanup.',
      d2: 'Upstream responses forwarded byte-for-byte. No prompt rewriting or silent model substitution.',
      d3: 'Billing uses official upstream rates multiplied only by the published multiplier on the pricing page.',
      d4: 'Routing completes in sub-100ms. First-token latency matches direct upstream access.'
    }
  },
  stats: {
    requests: 'Recorded API requests',
    uptime: '30-day availability',
    latency: '24-hour avg. first-token latency',
    models: 'Models available',
    through: 'Latest operations sample ended {time}',
    computed: 'Snapshot generated {time}',
    stale: 'Showing cached or delayed data'
  },
  sections: {
    imageTag: 'IMAGE API',
    imageTitle: 'One sentence, one image.',
    imageLede:
      'Synchronous Images handles generation, edits, and multiple outputs; single-request async avoids long connections, while Batch runs durable multi-prompt jobs.',
    featuresTag: 'WHY US',
    featuresTitle: 'Less friction, more flow.',
    codeTag: '3-MINUTE SETUP',
    codeTitle: 'Copy one command. Start calling.',
    codeLede: 'One-click deploy from the console — detect env, write config, self-check.',
    pricingTag: 'FAIR BILLING',
    faqTag: 'FAQ',
    faqTitle: 'Questions developers ask first'
  },
  faq: {
    items: [
      {
        q: 'Do you store my prompts?',
        a: 'Synchronous request bodies are not logged or persisted and are released after the response. Async and Batch jobs keep the minimum required request data encrypted or access-controlled until retention cleanup.'
      },
      {
        q: 'How do I know I get the model I asked for?',
        a: 'We byte-forward upstream responses without silent substitution. Usage logs show model name, token counts, and response IDs you can audit line by line.'
      },
      {
        q: 'Is a credit card required to start?',
        a: 'No. Register with email or GitHub/Google OAuth. Free signup credits (when enabled) and daily check-in rewards let you test before you top up via Stripe.'
      },
      {
        q: 'How is billing calculated?',
        a: 'Pay per token at published official multipliers. No monthly fee, no seat license, no minimum spend. Balance does not expire.'
      },
      {
        q: 'Which tools work out of the box?',
        a: 'Any OpenAI-compatible client: Claude Code, Cursor, Codex CLI, Cline, Gemini CLI, Continue, and custom SDKs — change base_url and API key only.'
      },
      {
        q: 'What are Usage Rewards?',
        a: 'Optional perks for active users: daily check-in credits, leaderboards, referral rebates, and VIP tiers from lifetime top-ups. All optional — the API works without them.'
      }
    ]
  },
  image: {
    model: 'gpt-image-2',
    badge: 'NEW',
    desc: 'Synchronous n=1-10, single-request async, and multi-prompt Batch have distinct APIs. Up to 20 MiB per multipart field.',
    caps: ['Text-to-image', 'Image-to-image', 'Multi-reference', 'Style transfer'],
    docLink: 'Image API docs',
    studioCta: 'Try a free image',
    promptLabel: 'PROMPT',
    promptText: 'A shiba inu in cyberpunk Tokyo rain, neon lights',
    statusGen: 'Generating',
    statusMeta: 'gpt-image-2 · 1024×1024',
    statusDone: 'Done · 1.2s',
    demoBadge: 'OpenAI compatible · Same key'
  },
  channels: {
    tag: 'USAGE REWARDS',
    title: 'More than a meter.',
    copyTitle: 'Usage should pay you back.',
    copyBody:
      'Daily check-ins, leaderboards, referral rebates, and VIP tiers — optional perks on top of fair API billing.',
    next: 'Next',
    enter: 'Explore',
    joinCta: 'Register to join',
    ch1: { name: 'Rewards box', hint: 'Spend tokens for random balance boosts' },
    ch2: { name: 'Daily check-in', hint: 'Free balance every day' },
    ch3: { name: 'Agent Team', hint: 'Squad up and share rewards' },
    ch4: { name: 'Token Farm', hint: 'Daily quests · Monthly season prizes' }
  },
  features: {
    multiModel: {
      title: 'Multi-model hub',
      desc: 'DeepSeek, Qwen, Kimi, GLM, GPT, Claude, Gemini and more — one OpenAI-compatible endpoint. Switch models in one line.'
    },
    stable: {
      title: 'Reliable routing',
      desc: 'Multi-line redundancy, automatic failover, stable long SSE streams. 99.9% uptime target.'
    },
    privacy: {
      title: 'Privacy first',
      desc: 'No disk storage, no training, no third-party sharing. TLS end-to-end. Your prompt stays yours.'
    },
    instant: {
      title: 'Instant access',
      desc: 'Register and create an API key immediately. No approval queue, no card required to start.'
    },
    transparent: {
      title: 'Transparent billing',
      desc: 'Official token multipliers published on the models page. Every call auditable in usage logs.'
    },
    selfService: {
      title: 'Self-service billing',
      desc: 'Stripe top-up, balance dashboard, low-balance alerts, promo codes, and referral program.'
    }
  },
  onboard: {
    s1t: 'Register & create key',
    s1d: 'Sign up with email or OAuth — create your first API key in the console',
    s2t: 'Copy deploy command',
    s2d: 'One-click script writes config for Claude Code, Cursor, or Codex',
    s3t: 'First API call',
    s3d: 'OpenAI-compatible SDK — change base_url and start calling',
    docLink: 'Prefer curl or raw HTTP?',
    docLinkCta: 'Quick start docs',
    terminalAria: 'One-command setup terminal demo',
    terminalTitle: 'bash — Jisudeng AI · one-command setup',
    terminalEnv: 'Environment detected - Node 22 · OpenClaw installed',
    terminalPasteKey: 'Paste API key › sk-••••••••••••',
    terminalModel: 'Primary model › claude-opus-4-7',
    terminalConfig: 'Config written to ~/.openclaw/openclaw.json',
    terminalSelfCheck: 'Self-check passed - setup complete',
    terminalIntroPrompt: 'openclaw "Introduce yourself"',
    terminalIntroReply: 'Hi, I am your local AI assistant connected through Jisudeng. Ready to work.'
  },
  pricing: {
    lineA: 'Pay per token,',
    lineB: 'not per seat,',
    lineC: 'pay what you use.',
    blurb: 'No monthly fee, no minimum spend. All calls billed at published official multipliers — line by line.',
    tags: ['Official multiplier · Line-item audit', 'No monthly fee · No minimum', 'Balance never expires']
  },
  closer: {
    title: 'Ready when you are.',
    sub: 'ONE KEY · MULTI-MODEL · VERIFIABLE BILLING'
  },
  footer: {
    tagline: 'AI API gateway',
    docs: 'Documentation',
    lmspeedBadgeAlt: 'Jisudeng is listed on LMSpeed.net'
  },
  registerBanner: {
    signupCredit: 'Register now — get ${amount} free credits to test every model.',
    checkin: 'Plus ${amount}/day from daily check-in.',
    cta: 'Create free account'
  },
  anchors: {
    manifesto: 'Promise',
    stats: 'Stats',
    image: 'Image',
    channels: 'Channels',
    features: 'Why us',
    onboard: 'Setup',
    pricing: 'Pricing',
    faq: 'FAQ',
    closer: 'Start',
    scrollToContent: 'Scroll to content',
    backToTop: 'Back to top'
  }
}
