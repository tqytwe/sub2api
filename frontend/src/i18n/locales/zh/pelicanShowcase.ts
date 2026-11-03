export default {
  pelicanShowcase: {
    title: '鹈鹕测智', description: '各分组模型定时完成同一道绘图题，查看作品并比较生成质量。',
    notice: '测试会限制模型输出长度，作品细节可能不完整。生成效果会受模型和渠道状态影响。',
    allGroups: '全部分组', keepRule: '每组保留最近 {count} 张', retentionRule: '超过 {days} 天自动清理',
    itemCount: '{count} 张', latestAt: '最近更新 {time}', groupEmpty: '该分组还没有作品。',
    scrollLabel: '{group}：查看历史作品', loadError: '加载鹈鹕测智失败', itemLoading: '作品加载中…',
    itemLoadError: '作品加载失败', invalidHtml: '这张作品无法显示', duration: '耗时 {seconds} 秒',
    reasoning: '思考强度 {effort}', efforts: { minimal: '最低', low: '低', medium: '中', high: '高', xhigh: '极高' },
    preview: '查看作品', previewTitle: '{group} · {model}', fitArtwork: '适应窗口', actualSize: '100%',
    previewSizing: '预览缩放', sandboxNote: '作品在隔离沙箱中运行，不能联网，也读取不到你的账号信息。',
    remove: '从展示中移除', removeConfirm: '确定移除此作品？所有用户都将无法查看。', removed: '作品已移除', removeFailed: '移除失败',
    api: { title: 'API 调用', available: 'API 已开放', unavailable: 'API 未开放', unavailableHint: '管理员尚未开放 API Key 读取作品。', manageKeys: '管理 API Key', readOnly: '只读接口不会运行测试、调用模型或扣除余额。', manifestEndpoint: '作品清单', itemEndpoint: '作品正文', copyUrl: '复制接口地址', authentication: '使用本站 API Key，通过 Authorization: Bearer YOUR_API_KEY 鉴权。', examples: '调用示例', manifestExample: '读取清单', itemExample: '读取正文', cacheExample: '条件请求', copyExample: '复制调用示例', manifestHint: '清单仅包含作品摘要，完整内容按作品地址获取。', itemHint: '删除或过期的作品返回 404。', cacheHint: '使用 ETag 和 If-None-Match 减少重复下载。', polling: '建议每 60 秒或更久轮询；遵守 Retry-After。', keySafety: '请将 API Key 保存在调用端后端，作品通过隔离 iframe 显示。' },
    disabled: { title: '鹈鹕测智暂未开放', description: '管理员开启后，这里会展示各分组生成的作品。' },
    empty: { title: '暂无作品', description: '当前没有可展示的分组作品。' },
    history: { title: '降智检测 · {group}', model: '检测模型', hint: '按模型查看最近生成结果。', runs: '检测记录', empty: '该分组暂无生成记录。', loadFailed: '加载检测记录失败。', selected: '检测模型：{model}', duration: '耗时 {duration}', previewLoading: '正在加载作品…', previewFailed: '作品加载失败。', invalidArtwork: '该记录没有可预览的 HTML/SVG 作品。', previewTitle: '{model} 生成的作品', chooseRun: '选择一次检测记录预览作品。' },
  },
}
