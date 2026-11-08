import { chromium } from '/tmp/v2193-browser-tools/node_modules/playwright-core/index.mjs'
import { resolve } from 'node:path'
const browser = await chromium.launch({ headless: true, args: ['--no-sandbox'] })
try {
  for (const width of [360, 768, 1280, 1920]) {
    for (const theme of ['light', 'dark']) {
      const page = await browser.newPage({ viewport: { width, height: 1000 } })
      await page.setContent(`<html lang="zh"><meta charset="UTF-8"><style>
        body{margin:0;padding:24px;background:${theme==='dark'?'#111827':'#f8fafc'};color:${theme==='dark'?'#f3f4f6':'#111827'};font:15px/1.65 sans-serif}h1{font-size:24px}h2{font-size:18px;margin:0 0 12px}section{margin:24px 0;padding:16px;border:1px solid #94a3b8;border-radius:8px}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:12px}strong{display:block;font-size:23px}.muted{font-size:12px;color:${theme==='dark'?'#cbd5e1':'#475569'}}.toolbar{display:flex;flex-wrap:wrap;gap:8px}button,input,select{font:inherit;color:inherit;background:transparent;border:1px solid #94a3b8;border-radius:6px;padding:8px}button:focus-visible,input:focus-visible,select:focus-visible{outline:2px solid #2563eb;outline-offset:2px}.scroll{overflow-x:auto}table{width:100%;border-collapse:collapse;min-width:520px}th,td{text-align:left;padding:12px;border-bottom:1px solid #94a3b8}.state{border-left:4px solid #dc2626;padding:8px}a{color:${theme==='dark'?'#93c5fd':'#1d4ed8'}}
        </style><h1>邀请返利 / 管理员实施审查板</h1><p class="muted">静态实现审查板 · 非运行中的 Vue 页面 · 非生产验收 · ${width}px / ${theme}</p>
        <nav>邀请返利：运营概览 ｜ 邀请记录 ｜ 返利记录 ｜ 提现记录</nav>
        <section><h2>运营概览</h2><div class="toolbar"><label>上海时区统计月 <input type="month" value="2026-11"></label><button>刷新</button></div><p class="muted">新增邀请按绑定月；有效人数按首次有效付费原月；转化率只比较同月受邀 cohort。</p><div class="grid"><div>新增邀请<strong>120</strong></div><div>有效首付人数<strong>32</strong></div><div>同期转化率<strong>20.0%</strong></div><div>待核验（全部）<strong>7</strong></div></div><p>账户额度：可用 17.5 ｜ 冻结 3.25 ｜ 转入余额 11 ｜ 线下登记 8</p><p class="muted">账户额度不表示现金兑换；入账与撤销按来源及币种分组。</p><div class="scroll"><table><tr><th>来源</th><th>币种</th><th>入账</th><th>撤销</th><th>净入账</th></tr><tr><td>常规返利</td><td>USD</td><td>20</td><td>2</td><td>18</td></tr><tr><td>邀请活动</td><td>CNY</td><td>100</td><td>0</td><td>100</td></tr></table></div><p>每日趋势：复用 Chart.js 折线 + 可展开数据表。</p></section>
        <section><h2>有效付费邀请月榜 · 前10名</h2><p class="muted">仅荣誉无奖励；每5秒更新，页面隐藏暂停，失败退避并保留旧榜。</p><div class="scroll"><table><tr><th>名次</th><th>推广用户</th><th>有效付费邀请</th></tr><tr><td>1</td><td><a href="#detail">example-user</a></td><td>25</td></tr><tr><td>2</td><td><a href="#detail">example-second</a></td><td>18</td></tr></table></div><p class="state">示例错误状态：榜单加载失败，其他统计可继续使用。 <button>重试</button></p></section>
        <section><h2>原记录页面 · 同一筛选上下文</h2><div class="toolbar"><button>邀请关系</button><button>推广用户</button><input aria-label="精确用户" placeholder="推广用户ID（精确）"><input aria-label="精确受邀人" placeholder="受邀用户ID（精确）"><select aria-label="来源"><option>全部来源</option><option>常规返利</option><option>邀请活动</option><option>其他来源</option></select><button>导出当前条件</button></div><p class="muted">汇总与 CSV 使用相同过滤；上限10,000条，超限提示缩小范围。历史数据缺失显示空值而非伪造0。</p></section>
        <section id="detail"><h2>用户详情 / 复用 BaseDialog</h2><nav>邀请与资格 ｜ 资金流水 ｜ 操作审计 ｜ 专属配置</nav><p>归属 → 首付与资格原因 → 关联订单 → 来源/币种流水 → 操作审计</p><p>风险排除 → 精确案件ID → 现有风险详情；活动 → 现有玩法运营，保留原审批。</p><p>全局比例继承时输入留空；清除配置解释会更换邀请码；所有配置入口与线下登记统一二次验证。</p></section>
        <section><h2>线下提现登记 / 已站外打款</h2><div class="toolbar"><input aria-label="金额" placeholder="账户额度"><input aria-label="流水号" placeholder="站外付款流水号"><input aria-label="付款时间" type="datetime-local"><input aria-label="备注" placeholder="付款备注"><button disabled>填写三项凭据后登记</button></div><p class="muted">金额与凭据随幂等键固定；结果未知时锁定输入并重试原请求，保留操作快照。</p></section>
        <footer class="muted">审查覆盖 default / error / empty / disabled / focus / loading 设计及真实组件单测。该板不能证明最终页面布局；仍需本地浏览器检查浅深色、中英文、200%缩放、游客/用户/管理员与真实账本。</footer></html>`)
      await page.screenshot({ path: resolve(`docs/visual-reviews/assets/2026-11-08-affiliate-admin-${width}-${theme}.png`), fullPage: true })
      await page.close()
    }
  }
} finally { await browser.close() }
