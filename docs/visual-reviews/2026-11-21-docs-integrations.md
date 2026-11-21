# 工具集成目录和配置教程

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/content/public-docs-additional-guides.ts",
    "frontend/src/content/public-docs-data.en.ts",
    "frontend/src/content/public-docs-data.zh.ts",
    "frontend/src/content/public-docs-integrations.ts",
    "frontend/src/content/public-docs-tool-guides.ts",
    "frontend/src/content/public-docs-tree.ts",
    "frontend/src/styles/public-docs.css",
    "frontend/src/views/public/DocsView.vue"
  ],
  "routes_or_surfaces": ["/docs", "/en/docs", "/docs?cat=tools", "/en/docs?cat=tools", "/docs?cat=tools&page=dify"],
  "languages_and_themes": ["zh-CN/light", "en-US/dark"],
  "states": ["default", "hover", "active", "focus-visible", "expanded", "collapsed", "loading", "error", "search-empty"],
  "viewports": ["360x800", "768x860", "1280x900", "1440x1000", "1920x1080"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/docs-integrations/prototype.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/docs-integrations/before.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/docs-integrations/updated-tools-360.png", "docs/visual-reviews/assets/docs-integrations/updated-tools-1440.png", "docs/visual-reviews/assets/docs-integrations/updated-guide-1920.png"],
  "commands": ["Playwright Chromium against Vite production build preview", "pnpm design:check", "pnpm exec vitest run src/content/__tests__/publicDocsReader.spec.ts"],
  "checks": {
    "keyboard": { "status": "passed", "notes": "Verified Control+K, search input and Enter navigation." },
    "reduced_motion": { "status": "passed", "notes": "Navigation flows checked with reduced motion enabled." }
  },
  "residual_risks": ["Third-party client releases and real metered calls require separate validation.", "Production acceptance on the user's computer is pending."]
}
-->

## Scope

- `/docs` 和 `/en/docs` 共用文档阅读框架。游客可浏览全部教程，访问密钥页面时进入登录流程。
- 工具集成分类收录参考站总览的 19 个条目，并保留本站 6 个附加工具。参考 [MoleAPI 工具集成](https://docs.moleapi.com/zh-CN/docs/apps)。

## Baseline

- 分类页只有提示文字。侧栏分类标题只能折叠。对标清单中的 14 项没有对应文章。[生产分类页基线](assets/docs-integrations/before.png)

## Prototype

- 根据用户提供的对标范围，在开发前用现有文档布局呈现分类、通用配置、协议和教程列表。[原型图](assets/docs-integrations/prototype.png)

## Reuse Decision

- 复用文档阅读布局、站点 API 地址、表格和代码复制组件。新增教程的参数示意采用语义标签，密钥控制台教程链接到现有脱敏图解。
- 工具注册表统一生成分类首页、侧栏文章列表及中英文内容树。分类标题打开总览，独立箭头折叠子项。

## State Coverage

- 预览验证分类展开、收起、锚点定位、键盘搜索、空搜索结果、回到分类与浏览器返回。保留原有文档加载、重试和未找到状态。列表链接没有提交态、成功提示或禁用态。

## Viewport Coverage

- 浏览器自动化遍历 69 篇文章及 8 个分类，覆盖中英文和 360、768、1440 像素视口。检查正文宽度、横向溢出、图片、可见图标、文章锚点和站内链接。1280 和 1920 像素补测工具集成入口及返回路径。

## Evidence

- [中文桌面分类](assets/docs-integrations/updated-tools-1440.png)、[英文移动分类](assets/docs-integrations/updated-tools-360.png)、[英文桌面教程](assets/docs-integrations/updated-guide-1920.png)。这些画面来自构建预览。

## Residual Risk

- 第三方教程中的参数示意与实际客户端界面可能因版本而异。第三方客户端的现场安装和真实模型计费请求仍需单独验证。
- 最终生产验收由用户在本地浏览器完成。
