# RED 摘要

原始日志仅保留本机 /tmp；以下为无凭据断言摘要。首轮状态夹具 keep-alive 清理造成的额外网络失败已修正；只以实际 503/null 与6/7剩余天数断言佐证业务缺陷。refresh RED的卸载后未清理异步错误是失败断言提前退出造成，GREEN无未处理拒绝。

## status

     → expected false to be true // Object.is equality
     → expected false to be true // Object.is equality
     → expected false to be true // Object.is equality
     → expected false to be true // Object.is equality
     → expected false to be true // Object.is equality
     → expected 1 to be 2 // Object.is equality
     → expected 'playHub.loadForbidden' to match /[\u4e00-\u9fff]/
     → expected 'playHub.loadNetworkFailed' to match /[\u4e00-\u9fff]/
     → expected 'playHub.loadNetworkFailed' to match /[\u4e00-\u9fff]/
     → expected 3 to be 2 // Object.is equality
     → expected undefined to be defined
     → expected '每日签到每日签到完成签到后随机开奖，结果即时发放当前余额$1.00连续签到…' to contain '再签 1 天可额外获得 $1.00'
⎯⎯⎯⎯⎯⎯ Failed Tests 14 ⎯⎯⎯⎯⎯⎯⎯
 FAIL  src/views/user/__tests__/WelfareStatus.http.spec.ts > checkin status through real Axios > distinguishes 503 from disabled and recovers via retry
 FAIL  src/views/user/__tests__/WelfareStatus.http.spec.ts > checkin status through real Axios > distinguishes 403 from disabled and recovers via retry
 FAIL  src/views/user/__tests__/WelfareStatus.http.spec.ts > checkin status through real Axios > distinguishes 0 from disabled and recovers via retry
 FAIL  src/views/user/__tests__/WelfareStatus.http.spec.ts > checkin status through real Axios > distinguishes timeout from disabled and recovers via retry
 FAIL  src/views/user/__tests__/WelfareStatus.http.spec.ts > checkin status through real Axios > allows one retry in flight, shows loading and hides stale actions
 FAIL  src/views/user/__tests__/WelfareStatus.http.spec.ts > checkin status through real Axios > refreshes a closed feature, reopens it and discards stale displayed actions during refresh
 FAIL  src/views/user/__tests__/WelfareStatus.http.spec.ts > playHub status through real Axios > distinguishes 503 from disabled and recovers via retry
 FAIL  src/views/user/__tests__/WelfareStatus.http.spec.ts > playHub status through real Axios > distinguishes 403 from disabled and recovers via retry
- Expected:
+ Received:
 FAIL  src/views/user/__tests__/WelfareStatus.http.spec.ts > playHub status through real Axios > distinguishes 0 from disabled and recovers via retry
 FAIL  src/views/user/__tests__/WelfareStatus.http.spec.ts > playHub status through real Axios > distinguishes timeout from disabled and recovers via retry
- Expected:
+ Received:
 FAIL  src/views/user/__tests__/WelfareStatus.http.spec.ts > playHub status through real Axios > allows one retry in flight, shows loading and hides stale actions
 FAIL  src/views/user/__tests__/WelfareStatus.http.spec.ts > playHub status through real Axios > refreshes a closed feature, reopens it and discards stale displayed actions during refresh
 FAIL  src/views/user/__tests__/WelfareStatus.http.spec.ts > playHub status through real Axios > does not let an old response overwrite a reopened page
 FAIL  src/views/user/__tests__/WelfareStatus.http.spec.ts > renders remaining days from the absolute next milestone day without changing reward amounts
Expected: "再签 1 天可额外获得 $1.00"
Received: "每日签到每日签到完成签到后随机开奖，结果即时发放当前余额$1.00连续签到 6 天服务器日期：2026-10-10立即签到可兑换福利已解锁本次奖励以服务端当前发布的奖池和风控规则为准。再签 7 天可额外获得 $1.00昨日（2026-10-09）未签到，充值后 24h 内可补签补签昨日去充值"
      Tests  14 failed | 1 passed (15)

## refresh

     → expected undefined to be defined
     → expected undefined to be defined
⎯⎯⎯⎯⎯⎯⎯ Failed Tests 2 ⎯⎯⎯⎯⎯⎯⎯
 FAIL  src/views/user/__tests__/CheckInView.errors.spec.ts > CheckInView errors through the real Axios HTTP client > recovers from an actual checkin HTTP timeout and suppresses rapid duplicate clicks
 FAIL  src/views/user/__tests__/CheckInView.errors.spec.ts > CheckInView errors through the real Axios HTTP client > recovers from an actual makeup HTTP timeout and suppresses rapid duplicate clicks
      Tests  2 failed | 12 passed (14)

## balance

             → expected '$1.00' to be '$1.50' // Object.is equality
             → expected '$1.00' to be '$2.50' // Object.is equality
             → expected "wrappedAction" to be called 2 times, but got 1 times
        ⎯⎯⎯⎯⎯⎯⎯ Failed Tests 3 ⎯⎯⎯⎯⎯⎯⎯
         FAIL  src/views/user/__tests__/CheckInView.http.spec.ts > CheckInView with real check-in HTTP and PostgreSQL > renders the persisted cash grant and reloads checked-in state
        Expected: "$1.50"
        Received: "$1.00"
         FAIL  src/views/user/__tests__/CheckInView.http.spec.ts > CheckInView with real check-in HTTP and PostgreSQL > renders the persisted milestone grant and reloads checked-in state
        Expected: "$2.50"
        Received: "$1.00"
         FAIL  src/views/user/__tests__/CheckInView.http.spec.ts > CheckInView with real check-in HTTP and PostgreSQL > renders a persisted makeup milestone and refreshes a stale second tab after rejection
              Tests  3 failed | 2 passed (5)

## 旧账户响应（中间构建）

真实 production build、Pinia、HTTP 与 RouterLink；旧资料请求在新页读到2.50后才释放：

AssertionError: Locator expected to have text '$2.50'
Actual value: $1.00
       - unexpected value "$1.00"

## Token 续期与奖励提交并发

     → expected 2 to be 3 // Object.is equality
     → expected 2 to be 3 // Object.is equality
⎯⎯⎯⎯⎯⎯⎯ Failed Tests 2 ⎯⎯⎯⎯⎯⎯⎯
 FAIL  src/views/user/__tests__/CheckInView.balance.http.spec.ts > refreshes after checkin even when token renewal started an older status read
 FAIL  src/views/user/__tests__/CheckInView.balance.http.spec.ts > refreshes after makeup even when token renewal started an older status read
AssertionError: expected 2 to be 3 // Object.is equality
      Tests  2 failed | 3 passed (5)

## 同一中枢组件的会话隔离（父线程复审补充）

原组合 head `56e58ed83707ebcc159f0e9f545f6f4ab454cbe9` 虽通过此前用例及 7/7 CI，父线程复审指出 PlayHub 仅检查 disposed，遗漏切账号/登出时的隔离。先以实际 Pinia、auth login/logout、Axios 拦截器和回环 HTTP 复现，再修改页面；没有构造 Axios 错误对象。

有效 RED：8 例中 7 失败、1 个真实 Axios401刷新对照通过，无未处理错误。失败覆盖 A→B 迟到200/403、已有 A 数据清空、旧503不能结束B加载、登出和再登录、实际自动续期后 token 被 store 采用时恢复读取。最早夹具误用含时区 query 的 raw URL 匹配路径，修正为 pathname 后重新取得上述有效 RED；早期夹具错误不计业务证据。

旧 production build Chromium，真实 auth.login 切换账户且断言 DOM 组件未重建；释放 A 的迟到响应后：

```text
AssertionError: Locator expected to have text '$2.00'
Actual value: $1.00
```

截图：`docs/visual-reviews/assets/checkin-status/hub-session-repro-hub-session.png`；旧资源哈希：`hub-session-red-assets.json`（同 56e58ed8 的 pre-session-fix-assets.json）。新增第9例 A→B→A 同 id/token 返回，验证请求代次而非仅比较身份；该例在首次 GREEN 后追加，不冒记首次 RED。

自动续期证据边界：实际 authStore 定时器调用真实 refresh HTTP 并持久化新 token，测试再调用生产 checkAuth 采用该 token；没有宣称既有公共 store 自动同步策略已修复。另以实际 Axios401→refresh→重试验证成功读取不会被丢弃。
