import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import PluginsView from '../PluginsView.vue'

const {
  listPlugins,
  uploadPlugin,
  enablePlugin,
  savePluginConfig,
  createUISession,
  testPlugin,
  stepUpRun,
  activeLocale,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  listPlugins: vi.fn(),
  uploadPlugin: vi.fn(),
  enablePlugin: vi.fn(),
  savePluginConfig: vi.fn(),
  createUISession: vi.fn(),
  testPlugin: vi.fn(),
  stepUpRun: vi.fn((action: () => Promise<unknown>) => action()),
  activeLocale: { value: 'zh-CN' },
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    plugins: {
      list: listPlugins,
      upload: uploadPlugin,
      enable: enablePlugin,
      disable: vi.fn(),
      remove: vi.fn(),
      getConfig: vi.fn().mockResolvedValue({}),
      saveConfig: savePluginConfig,
      test: testPlugin,
      createUISession,
    },
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    showInfo: vi.fn(),
  }),
}))

vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({ run: stepUpRun }),
  isStepUpBlocked: () => false,
  isStepUpCancelled: () => false,
  stepUpBlockReason: () => '',
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key, locale: activeLocale }),
}))

const plugin = {
  id: 7,
  plugin_key: 'local.test.transport',
  name: 'Test Transport',
  version: '1.0.0',
  description: '',
  author: 'test',
  manifest: {
    schema_version: 1,
    id: 'local.test.transport',
    name: 'Test Transport',
    version: '1.0.0',
    requires: {
      sub2api: '>=0.1.0',
      plugin_protocol: 1,
      transport_api: 1,
      ui_bridge: 1,
    },
    capabilities: [],
    ui: { entrypoint: 'ui/index.html' },
  },
  binary_sha256: 'a'.repeat(64),
  signature_status: 'trusted' as const,
  state: 'disabled' as const,
  last_error: '',
  installed_at: '2026-08-22T00:00:00Z',
  updated_at: '2026-08-22T00:00:00Z',
  bindings: [
    {
      id: 1,
      plugin_id: 7,
      capability: 'openai.oauth.outbound_transport.v1',
      platform: 'openai',
      account_type: 'oauth',
      enabled: false,
      rollout_percent: 100,
    },
  ],
  compatibility: {
    compatible: true,
    tested: true,
    status: 'compatible' as const,
    message: '当前 Sub2API 版本已由插件声明测试',
    current_sub2api_version: '0.1.0',
    required_sub2api_version: '>=0.1.0',
    recommended_sub2api_version: '0.1.0',
    plugin_protocol: 1,
    transport_api: 1,
    ui_bridge: 1,
  },
  runtime_healthy: false,
  runtime_message: '插件进程运行中',
}

function mountView() {
  return mount(PluginsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        BaseDialog: { template: '<div><slot /></div>' },
        Icon: true,
        TotpStepUpDialog: true,
      },
    },
  })
}

describe('管理员插件页二次验证', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    activeLocale.value = 'zh-CN'
    stepUpRun.mockImplementation((action: () => Promise<unknown>) => action())
    listPlugins.mockResolvedValue([plugin])
    uploadPlugin.mockResolvedValue(plugin)
    enablePlugin.mockResolvedValue(plugin)
    savePluginConfig.mockResolvedValue({ enabled: true })
    testPlugin.mockResolvedValue({ success: true, message: '检查通过', latency_ms: 1 })
    createUISession.mockResolvedValue({
      url: '/api/v1/plugin-ui/token/index.html#bridge_token=bridge',
      bridge_token: 'bridge',
      ui_bridge_version: 1,
      expires_at: '2026-08-22T01:00:00Z',
    })
  })

  it('启用插件通过 step-up 控制器执行', async () => {
    const wrapper = mountView()
    await flushPromises()

    const button = wrapper.findAll('button').find((item) => item.text().includes('admin.plugins.enable'))
    expect(button).toBeDefined()
    await button!.trigger('click')
    await flushPromises()

    expect(stepUpRun).toHaveBeenCalledTimes(1)
    expect(enablePlugin).toHaveBeenCalledWith(7, 100, false)
  })

  it('上传插件通过 step-up 控制器执行', async () => {
    const wrapper = mountView()
    await flushPromises()
    const input = wrapper.get('input[type="file"]')
    Object.defineProperty(input.element, 'files', {
      configurable: true,
      value: [new File(['plugin'], 'transport.s2plugin', { type: 'application/zip' })],
    })

    await input.trigger('change')
    await flushPromises()

    expect(stepUpRun).toHaveBeenCalledTimes(1)
    expect(uploadPlugin).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['zh-CN', 'zh'],
    ['en-US', 'en'],
    ['fr-FR', 'zh'],
  ])('插件配置 iframe 为 %s 传递规范化语言 %s', async (locale, expected) => {
    activeLocale.value = locale
    const wrapper = mountView()
    await flushPromises()

    const button = wrapper.findAll('button').find((item) => item.text().includes('admin.plugins.configure'))
    expect(button).toBeDefined()
    await button!.trigger('click')
    await flushPromises()

    const src = wrapper.get('iframe').attributes('src')
    const fragment = new URLSearchParams(src.split('#')[1])
    expect(fragment.get('bridge_token')).toBe('bridge')
    expect(fragment.get('locale')).toBe(expected)
  })

  it('英文路径不直接渲染后端中文兼容性和运行状态', async () => {
    activeLocale.value = 'en-US'
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).not.toContain('当前 Sub2API 版本已由插件声明测试')
    expect(wrapper.text()).not.toContain('插件进程运行中')
    expect(wrapper.text()).toContain('admin.plugins.compatibleMessage')
  })

  it('英文路径测试结果不直接显示插件中文消息', async () => {
    activeLocale.value = 'en-US'
    const wrapper = mountView()
    await flushPromises()

    const button = wrapper.findAll('button').find((item) => item.text() === 'admin.plugins.test')
    expect(button).toBeDefined()
    await button!.trigger('click')
    await flushPromises()

    expect(showSuccess).toHaveBeenCalledWith('admin.plugins.testSuccess')
    expect(showSuccess).not.toHaveBeenCalledWith('检查通过')

    testPlugin.mockResolvedValueOnce({ success: false, message: '配置检查失败', latency_ms: 1 })
    await button!.trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('admin.plugins.testFailed')
    expect(showError).not.toHaveBeenCalledWith('配置检查失败')
  })
})
