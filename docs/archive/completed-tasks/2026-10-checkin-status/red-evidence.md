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
