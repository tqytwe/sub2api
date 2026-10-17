export default {
  "bps": {
    "title": "Excel / BPS 默认配置",
    "subtitle": "在账号里选择“使用默认配置”开启 BPS 时，填入这里保存的模型和选项。",
    "scopeHint": "这里只保存默认模板。账号内提供“使用默认配置”和“使用初始配置”两种开启方式；新建、导入和同步账号不会自动开启 BPS。",
    "models": "默认使用的模型",
    "ws_sse_accelerationHint": "保存 HTTP 流式 WS 加速的默认勾选状态，默认关闭。",
    "auto_enable_on_degradationHint": "保存降智后自动开启 BPS 的默认勾选状态，默认关闭。",
    "options": "默认勾选的选项",
    "reset": "恢复推荐选项",
    "advanced": "更多选项",
    "advancedHint": "工具、图片、403 恢复与会话代理",
    "ignore_encrypted_contentHint": "跳过 BPS 无法读取的历史加密内容，保留明文与工具调用关系。",
    "auto_disable_on_403Hint": "收到符合条件的上游 403 时关闭 BPS，账号仍可使用原 Codex 通道。",
    "cache_creation_as_inputHint": "缓存创建 token 按普通输入单价计费，上游缓存行为不变。",
    "omit_unsupported_toolsHint": "省略 BPS 不支持的托管工具；实时搜索和图片生成将不可用。",
    "auto_recover_on_403Hint": "需要开启 403 自动关闭。按间隔发送文本探测，成功后重新开启 BPS，会消耗少量额度。",
    "auto_move_on_403Hint": "遇到符合条件的 403 时移入目标分组；可选择退出全部分组。",
    "session_proxyHint": "需要已配置可用的 Mihomo 或 IP 管理代理池；出口不稳定会影响请求。",
    "modelsRequired": "请至少选择一个默认 BPS 模型，或启用全部模型。",
    "applying": "读取默认配置…",
    "loadFailed": "默认配置读取失败，原设置未改动。请再次点击默认配置开关重试。",
    "manage": "管理默认配置",
    "mode": {
      "defaults": "使用默认配置开启",
      "defaultsHint": "打开时填入已保存的默认模型和勾选项，可继续调整。",
      "initial": "使用初始配置开启",
      "initialHint": "打开时使用原始模型选项，附加功能不勾选，再自行配置。",
      "defaultsApplied": "已填入默认配置，保存账号后生效。",
      "initialApplied": "已填入初始配置，可自行勾选，保存账号后生效。"
    },
    "modeHint": "两种方式互斥；切换会重新填入模型和选项，关闭当前开关即关闭 BPS。",
    "adminOnly": "默认模板仅管理员可用，仍可使用初始配置并手动调整。"
  }

}
