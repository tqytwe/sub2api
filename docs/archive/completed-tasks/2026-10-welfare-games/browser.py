"""Production Vue assets over loopback HTTP fixtures. Never production acceptance.

Baseline dist is #355, whose tree equals c939d570. Prototype DOM edits are visual
previews only; updated captures use the actual rebuilt application without edits.
"""
import argparse
import hashlib
import json
import socket
import threading
import time
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlparse
from playwright.sync_api import sync_playwright, expect

ROOT = Path(__file__).resolve().parents[4]
OUT = ROOT / 'docs/visual-reviews/assets/welfare-games'
OUT.mkdir(parents=True, exist_ok=True)
p = argparse.ArgumentParser()
p.add_argument('--phase', choices=['baseline', 'quiz-repro', 'quiz-balance-repro', 'blindbox-repro', 'blindbox-updated', 'blindbox-identity-repro', 'blindbox-quality-repro', 'updated'], required=True)
p.add_argument('--dist', required=True)
args = p.parse_args()
dist = Path(args.dist).resolve()
assert (dist / "index.html").is_file(), "Pass the actual Vite output directory"
USER = dict(id=50, email='fixture@example.test', username='Fixture', role='user', status='active', balance=1, concurrency=5)
SETTINGS = dict(site_name='极速蹬 · TEST FIXTURE', play_checkin_enabled=True, play_quiz_enabled=True, play_blindbox_enabled=True, registration_enabled=True)
state = dict(code=503, gets=0, posts=0, enabled=True, empty=False, submitted=False, delay=0, reward='balance', lose=False)

def quiz(en=False):
    return dict(enabled=state['enabled'], coupon_pool_ready=True, already_submitted=state['submitted'], server_date='2026-10-11', reward_per_correct=.1,
                questions=[] if state['empty'] or state['submitted'] else [dict(id=i, prompt=f'Fixture question {i}' if en else f'隔离测试题 {i}', options=['A', 'B']) for i in [1, 2]],
                previous_score=2, previous_total=2, previous_reward=.2 if state['reward']=='balance' else 0,
                previous_reward_type=state['reward'], previous_redeem_code=redeem() if state['reward']=='redeem_code' and state['submitted'] else None)

BB=dict(opens=2,reward='redeem_code',known=False,code=0,delay=0,enabled=True,ready=True,energy=False,lose=False,results={},gets=0,posts=0,recents=0)

def blindbox_status():
    pool=dict(version='welfare-fixture',cost=.5,rtp_cap=.9,tiers=[dict(amount=.2,weight=10000)])
    data=dict(enabled=BB['enabled'],coupon_pool_ready=BB['ready'],coupon_prizes=[],cost_amount=.5,pool=pool,current_pool=pool,
                vip_tier=dict(tier=0,label='普通',color_key='neutral'),expected_reward=.2,rtp_cap=.9,pool_version='welfare-fixture',
                daily_limit=2,effective_limit=2,opens_today=BB['opens'],can_open=BB['opens']<2 and USER['balance']>=.5 and not BB['energy'],server_date='2026-10-11',
                growth_eligibility=dict(reward_mode='energy' if BB['energy'] else 'redeemable',primary_reason='email_unverified',progress=dict(account_age_days=4,minimum_account_age_days=3,net_balance_recharge_30d=0,minimum_recharge_cny=1)),growth_governance_available=True)
    if BB['known'] and BB['ready']:
        data.update({k+'_weight_bp':10000 if BB['reward']==k else 0 for k in ['coupon','redeem_code','balance']})
    if BB['energy']:
        for k in ['pool','current_pool','cost_amount','expected_reward','coupon_weight_bp','redeem_code_weight_bp','balance_weight_bp']:data.pop(k,None)
    return data


def redeem():
    return dict(id=1, code='FIXTURE-REWARD', type='balance', value=.3, status='issued', expires_at='2026-10-14T00:00:00Z')

class Handler(SimpleHTTPRequestHandler):
    def __init__(self, *a, **kw):
        super().__init__(*a, directory=str(dist), **kw)
    def log_message(self, *a):
        pass
    def reply(self, code, data):
        raw = json.dumps(data).encode()
        self.send_response(code)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(raw)))
        self.end_headers()
        try:
            self.wfile.write(raw)
        except BrokenPipeError:
            pass
    def do_GET(self):
        path = urlparse(self.path).path
        if path.startswith('/api/'):
            if path.endswith('/quiz/today'):
                state['gets'] += 1
                time.sleep(state['delay'])
                if state['code']:
                    return self.reply(state['code'], dict(code=state['code'], reason='FIXTURE_READ_FAILURE'))
                data = quiz(self.headers.get('Accept-Language', '').startswith('en'))
            elif path.endswith('/blindbox/status'):
                BB['gets']+=1;time.sleep(BB['delay'])
                if BB['code']:return self.reply(BB['code'],dict(code=BB['code'],reason='FIXTURE_READ_FAILURE'))
                data = blindbox_status()
            elif path.endswith('/blindbox/pool'):
                data = dict(enabled=True,coupon_pool_ready=True)
            elif path.endswith('/blindbox/recent'):
                BB['recents']+=1
                data = [dict(user='fixture-recent-user',reward=.2,when='2026-10-11T00:00:00Z')]
            elif path.endswith('/auth/me'):
                data = dict(USER, id=51, username='Fixture B') if self.headers.get('Authorization') == 'Bearer fixture-b' else USER
            elif path.endswith('/settings/public'):
                data = SETTINGS
            elif 'subscriptions' in path:
                data = []
            elif 'announcements' in path:
                data = dict(items=[], total=0)
            else:
                data = {}
            return self.reply(200, dict(code=0, data=data))
        if not Path(self.translate_path(self.path)).is_file():
            self.path = '/index.html'
        super().do_GET()
    def do_POST(self):
        self.rfile.read(int(self.headers.get('Content-Length', 0)))
        if self.path.endswith('/blindbox/open'):
            BB['posts']+=1
            key=self.headers.get('Idempotency-Key','')
            if key in BB['results']:return self.reply(200,dict(code=0,data=BB['results'][key]))
            BB['opens']+=1
            cash=.2 if BB['reward']=='balance' else 0
            USER['balance']+=cash-.5
            data=dict(cost_amount=.5,reward_amount=cash,net_amount=cash-.5,reward_type=BB['reward'],opens_today=BB['opens'],pool_version='welfare-fixture',server_date='2026-10-11')
            if BB['reward']=='redeem_code':data['redeem_code']=redeem()
            if BB['reward']=='coupon':data['coupon']=dict(user_coupon_id=1,template_id=1,name='隔离福利券',benefit_type='fixed_amount',benefit_value=1,currency='USD',applicable_scopes=['balance'],minimum_order_amount=0,valid_from='2026-10-10T00:00:00Z',expires_at='2026-10-14T00:00:00Z')
            BB['results'][key]=data
            if BB['lose']:
                BB['lose']=False;self.connection.shutdown(socket.SHUT_RDWR);self.connection.close();return
            return self.reply(200,dict(code=0,data=data))
        if self.path.endswith('/quiz/submit'):
            state['posts'] += 1
            if state['submitted']:
                return self.reply(409, dict(code=409, reason='PLAY_QUIZ_ALREADY_DONE'))
            state['submitted'] = True
            if state['reward']=='balance': USER['balance'] += .2
            if state['lose']:
                state['lose']=False
                self.connection.shutdown(socket.SHUT_RDWR);self.connection.close();return
            data = dict(score=2, total=2, reward_amount=.2 if state['reward']=='balance' else 0, reward_type=state['reward'])
            if state['reward']=='redeem_code':
                data['redeem_code'] = redeem()
            return self.reply(200, dict(code=0, data=data))
        return self.reply(404, dict(code=404))

server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
threading.Thread(target=server.serve_forever, daemon=True).start()
origin = f'http://127.0.0.1:{server.server_port}'
reports = []

def capture(page, name):
    page.evaluate('''async () => {
      await new Promise(requestAnimationFrame);await new Promise(requestAnimationFrame);
      await Promise.all(document.getAnimations().filter(a=>Number.isFinite(a.effect?.getTiming().iterations)).map(a=>a.finished.catch(()=>{})));
    }''')
    page.screenshot(path=str(OUT / (name+'.png')), full_page=True, animations='disabled')

def context_for(browser, width=1280, theme='light', guest=False):
    context = browser.new_context(viewport=dict(width=width, height=900), reduced_motion='reduce')
    script = 'window.__APP_CONFIG__='+json.dumps(SETTINGS)+';localStorage.setItem("theme",'+json.dumps(theme)+');'
    if not guest:
        script += 'localStorage.setItem("auth_token","fixture-only");localStorage.setItem("auth_user",JSON.stringify('+json.dumps(USER)+'));'
    context.add_init_script(script)
    context.route('**/*', lambda route: route.continue_() if urlparse(route.request.url).netloc == urlparse(origin).netloc else route.abort())
    return context

with sync_playwright() as pw:
    browser = pw.chromium.launch(executable_path='/usr/bin/chromium', headless=True, args=['--no-sandbox', '--disable-dev-shm-usage'])
    if args.phase == 'baseline':
        context=context_for(browser);page=context.new_page()
        page.goto(origin+'/quiz-quest?lang=zh',wait_until='networkidle')
        expect(page.locator('.play-workspace').get_by_text('答题闯关暂未开启',exact=True).last).to_be_visible()
        capture(page,'baseline-quiz-503')
        page.evaluate('''() => {
          const note=document.querySelector('.play-workspace > .play-note');note.setAttribute('role','alert');
          note.innerHTML='<p>题目加载失败，请稍后重试。</p><button class="play-btn play-btn-secondary mt-3" type="button">重试</button>';
          document.querySelector('.play-action-panel > .play-note').textContent='题目加载失败，请稍后重试。';
          const button=document.createElement('button');button.type='button';button.className='play-btn play-btn-secondary mt-3';button.textContent='刷新';document.querySelector('.play-hero-grid > div').append(button);
        }''')
        capture(page,'prototype-quiz-error')
        page.evaluate('''() => {
          const note=document.querySelector('.play-workspace > .play-note');note.removeAttribute('role');note.innerHTML='<p>登录后即可查看今日题目和福利资格。</p><a href="/login?redirect=/quiz-quest" class="play-btn play-btn-primary mt-3">登录</a>';
          document.querySelector('.play-action-panel > .play-note').textContent='登录后即可查看今日题目和福利资格。';document.querySelector('.play-hero-grid > div > button').remove();
        }''')
        capture(page,'prototype-quiz-guest');context.close()
        reports.append(dict(case='quiz-503-and-DOM-prototypes',http_status=503,private_gets=state['gets']))
    if args.phase == 'quiz-repro':
        state.update(code=0,reward='redeem_code')
        context=context_for(browser);page=context.new_page();page.goto(origin+'/quiz-quest?lang=zh',wait_until='networkidle')
        for question in page.locator('.quiz-question-card').all():
            question.locator('label').first.click()
        page.locator('button.play-btn-primary').click()
        expect(page.get_by_text('得分 2/2 · 到账 $0.00',exact=True)).to_be_visible()
        capture(page,'baseline-quiz-redeem-toast')
        page.get_by_text('得分 2/2 · 到账 $0.00',exact=True).evaluate("el=>el.textContent='今日得分 2/2 · 兑换码 FIXTURE-REWARD 已发放'")
        capture(page,'prototype-quiz-redeem-toast');context.close()
        reports.append(dict(case='redeem-shown-as-zero-cash',posts=state['posts'],prototype='DOM preview only'))
    if args.phase == 'quiz-balance-repro':
        state.update(code=0,enabled=True,empty=False,submitted=False,reward='balance');USER['balance']=1
        context=context_for(browser);page=context.new_page();page.goto(origin+'/quiz-quest?lang=zh',wait_until='networkidle')
        for question in page.locator('.quiz-question-card').all():question.locator('label').first.click()
        assert context.request.post(origin+'/api/v1/play/quiz/submit').status==200
        page.locator('button.play-btn-primary').click()
        expect(page.locator('.quiz-question-card')).to_have_count(0)
        expect(page.get_by_role('banner')).to_contain_text('$1.00')
        page.get_by_role('button', name='关闭', exact=True).click()
        capture(page,'baseline-quiz-stale-balance')
        page.get_by_role('banner').get_by_text('$1.00',exact=True).evaluate_all("els=>els.forEach(el=>el.textContent='$1.20')")
        capture(page,'prototype-quiz-recovered-balance');context.close()
        reports.append(dict(case='quiz-409-stale-balance',profile_balance=1.2,header_balance=1.0,prototype='DOM preview only'))
    if args.phase == 'blindbox-repro':
        USER['balance']=0
        context=context_for(browser);context.add_init_script("sessionStorage.setItem('blindbox.pending-open:50','fixture-last-unknown')")
        page=context.new_page();page.goto(origin+'/blindbox?lang=zh',wait_until='networkidle')
        expect(page.locator('.play-action-panel button.play-btn-primary')).to_be_disabled()
        expect(page.get_by_test_id('blindbox-branches').get_by_text('概率未公开',exact=True)).to_have_count(3)
        capture(page,'baseline-blindbox-unknown-and-weights')
        page.evaluate('''() => {
          const panel=document.querySelector('.play-action-panel > .space-y-4');
          const note=document.createElement('div');note.className='play-note';note.innerHTML='<p>上次开箱结果尚未确认。重试会继续上次开箱，不会重复扣费。</p><button class="play-btn play-btn-secondary mt-3">重试上次开箱</button>';panel.append(note);
          const odds=document.querySelectorAll('[data-testid="blindbox-branches"] dd');[...odds].forEach((el,i)=>el.textContent=i===1?'100%':'0%');
        }''')
        capture(page,'prototype-blindbox-recovery-and-weights');context.close()
        reports.append(dict(case='blindbox-pending-recovery-and-known-weights',prototype='DOM preview only',reward='redeem_code',opens_today=2))
    if args.phase=='blindbox-quality-repro':
        BB.update(opens=0,known=True,code=503,energy=False);USER['balance']=1
        context=context_for(browser);page=context.new_page();page.goto(origin+'/blindbox?lang=zh',wait_until='networkidle')
        expect(page.get_by_test_id('blindbox-status-retry')).to_be_visible();expect(page.get_by_text('fixture-recent-user')).to_be_visible()
        capture(page,'baseline-blindbox-failed-qualification-feed')
        page.locator('.play-wins-list').evaluate("el=>el.innerHTML='<li>暂无公开中奖记录，开启后的奖励会即时入账。</li>'")
        capture(page,'prototype-blindbox-failed-qualification-hidden');context.close()
    if args.phase in ['blindbox-identity-repro', 'blindbox-updated']:
        BB.update(opens=0,reward='redeem_code',known=True,code=0,delay=0,enabled=True,ready=True,energy=False,lose=False,results={});USER['balance']=1
        context=context_for(browser);page=context.new_page();page.goto(origin+'/blindbox?lang=zh',wait_until='networkidle')
        page.locator('.play-action-panel button.play-btn-primary').click();expect(page.get_by_role('dialog')).to_contain_text('FIXTURE-REWARD')
        page.wait_for_function("sessionStorage.getItem('blindbox.pending-open:50') === null")
        page.evaluate("async () => { await document.querySelector('#app').__vue_app__.config.globalProperties.$pinia._s.get('auth').logout(); }")
        page.wait_for_timeout(150)
        if args.phase=='blindbox-identity-repro':
            expect(page.get_by_role('dialog')).to_contain_text('FIXTURE-REWARD');capture(page,'baseline-blindbox-logout-reward')
            page.get_by_role('dialog').evaluate("el=>el.remove()");capture(page,'prototype-blindbox-logout-cleared')
        else:
            expect(page.get_by_role('dialog')).to_have_count(0);expect(page.get_by_text('FIXTURE-REWARD')).to_have_count(0);capture(page,'updated-blindbox-logout-cleared')
        context.close()
        BB.update(opens=0,energy=False);USER['balance']=1
        context=context_for(browser);page=context.new_page();page.goto(origin+'/blindbox?lang=zh',wait_until='networkidle')
        expect(page.locator('.play-prize-amount')).to_be_visible();expect(page.get_by_text('fixture-recent-user')).to_be_visible();before=BB['gets'];recent_before=BB['recents'];BB['delay']=2;BB['energy']=True
        page.evaluate("() => { const auth=document.querySelector('#app').__vue_app__.config.globalProperties.$pinia._s.get('auth');localStorage.setItem('auth_token','fixture-b');localStorage.setItem('auth_user',JSON.stringify({...auth.user,id:51}));auth.checkAuth(); }")
        page.wait_for_timeout(200);assert BB['gets']==before+1
        if args.phase=='blindbox-identity-repro':
            expect(page.locator('.play-prize-amount')).to_be_visible();capture(page,'baseline-blindbox-switch-prize')
            page.locator('.play-prize-pool').evaluate_all("els=>els.forEach(el=>el.remove())")
            page.locator('.play-prize-amount').evaluate_all("els=>els.forEach(el=>el.parentElement.remove())")
            capture(page,'prototype-blindbox-switch-cleared')
        else:
            expect(page.locator('.play-prize-amount')).to_have_count(0);expect(page.get_by_test_id('blindbox-branches')).to_have_count(0);capture(page,'updated-blindbox-switch-cleared')
        page.wait_for_timeout(2200);BB['delay']=0
        if args.phase=='blindbox-updated':
            assert BB['recents']==recent_before
            expect(page.get_by_text('fixture-recent-user')).to_have_count(0)
            BB['energy']=False
            page.evaluate("() => { const auth=document.querySelector('#app').__vue_app__.config.globalProperties.$pinia._s.get('auth');localStorage.setItem('auth_token','fixture-only');localStorage.setItem('auth_user',JSON.stringify({...auth.user,id:50}));auth.checkAuth(); }")
            expect(page.get_by_text('fixture-recent-user')).to_be_visible();assert BB['recents']==recent_before+1
            capture(page,'updated-blindbox-qualified-recent-restored')
            recent_before=BB['recents'];BB['code']=503
            page.evaluate("() => { const auth=document.querySelector('#app').__vue_app__.config.globalProperties.$pinia._s.get('auth');localStorage.setItem('auth_token','fixture-b');localStorage.setItem('auth_user',JSON.stringify({...auth.user,id:51}));auth.checkAuth(); }")
            expect(page.get_by_test_id('blindbox-status-retry')).to_be_visible();expect(page.get_by_text('fixture-recent-user')).to_have_count(0);assert BB['recents']==recent_before
            capture(page,'updated-blindbox-switch-failed-qualification')
            BB['code']=0;page.get_by_test_id('blindbox-status-retry').click();expect(page.get_by_text('fixture-recent-user')).to_be_visible();assert BB['recents']==recent_before+1
            capture(page,'updated-blindbox-switch-qualification-retried')
        context.close()
        reports.append(dict(case='blindbox-identity-isolation',logout_clears_reward=args.phase=='blindbox-updated',switch_clears_private_pool=args.phase=='blindbox-updated',failed_qualification_retry=args.phase=='blindbox-updated',qualified_recent_restored=args.phase=='blindbox-updated',explorer_recent_requests=0 if args.phase=='blindbox-updated' else None,auth_actions='production Pinia logout/checkAuth; synthetic HTTP identity'))
    if args.phase == 'blindbox-updated':
        for width,lang,theme in [(360,'zh','dark'),(768,'en','light'),(1280,'zh','light'),(1920,'en','dark')]:
            BB.update(opens=0,reward='redeem_code',known=True,code=503,delay=0,enabled=True,ready=True,energy=False,lose=False,results={});USER['balance']=1
            context=context_for(browser,width,theme);page=context.new_page();errors=[];page.on('pageerror',lambda err:errors.append(str(err)))
            page.goto(origin+'/blindbox?lang='+lang,wait_until='networkidle')
            expect(page.get_by_test_id('blindbox-status-retry')).to_be_visible();expect(page.get_by_text('fixture-recent-user')).to_have_count(0);capture(page,f'updated-blindbox-{width}-{lang}-{theme}-error')
            BB['code']=0;BB['delay']=.4;page.get_by_test_id('blindbox-status-retry').click()
            capture(page,f'updated-blindbox-{width}-{lang}-{theme}-loading')
            button=page.locator('.play-action-panel button.play-btn-primary');expect(button).to_be_enabled();BB['delay']=0
            expect(page.get_by_test_id('blindbox-branches').locator('dd')).to_have_text(['0%','100%','0%']);expect(page.get_by_text('fixture-recent-user')).to_be_visible()
            button.hover();button.focus();expect(button).to_be_focused();capture(page,f'updated-blindbox-{width}-{lang}-{theme}-ready')
            before=BB['posts'];button.press('Enter');expect(page.get_by_role('dialog')).to_contain_text('FIXTURE-REWARD')
            expect(page.get_by_role('banner')).to_contain_text('$0.50');capture(page,f'updated-blindbox-{width}-{lang}-{theme}-success')
            assert BB['posts']==before+1 and not errors,errors
            assert page.evaluate('document.documentElement.scrollWidth <= innerWidth')
            reports.append(dict(case='blindbox-states',viewport=width,language=lang,theme=theme,known_probabilities=[0,100,0],js_errors=errors,overflow=False))
            context.close()
        BB.update(opens=1,reward='redeem_code',known=True,code=0,delay=0,enabled=True,ready=True,energy=False,lose=True,results={});USER['balance']=.5
        context=context_for(browser);page=context.new_page();page.goto(origin+'/blindbox?lang=zh',wait_until='networkidle')
        before=BB['posts'];page.locator('.play-action-panel button.play-btn-primary').click()
        expect(page.get_by_text('开箱失败，请稍后重试',exact=True)).to_be_visible();page.reload(wait_until='networkidle')
        expect(page.get_by_test_id('blindbox-resume')).to_be_enabled();expect(page.locator('.play-action-panel button.play-btn-primary')).to_be_disabled()
        capture(page,'updated-blindbox-unknown-recovery');page.get_by_test_id('blindbox-resume').click()
        expect(page.get_by_role('dialog')).to_contain_text('FIXTURE-REWARD');expect(page.get_by_role('banner')).to_contain_text('$0.00')
        capture(page,'updated-blindbox-recovered-reward');assert BB['posts']==before+2 and BB['opens']==2 and USER['balance']==0
        reports.append(dict(case='blindbox-unknown-remount-replay',opens=2,posts=2,balance=0));context.close()
        for state_name in ['disabled','unpublished','explorer','guest']:
            BB.update(opens=0,enabled=state_name!='disabled',ready=state_name!='unpublished',energy=state_name=='explorer',known=True,results={})
            context=context_for(browser,360,'light',state_name=='guest');page=context.new_page();before=BB['gets'];page.goto(origin+'/blindbox?lang=zh',wait_until='networkidle')
            if state_name=='guest':assert BB['gets']==before
            if state_name=='explorer':expect(page.get_by_test_id('blindbox-branches')).to_have_count(0)
            capture(page,'updated-blindbox-'+state_name);context.close()
        reports.append(dict(case='blindbox-disabled-unpublished-explorer-guest',guest_private_gets=0,explorer_reward_details=False))
    if args.phase == 'updated':
        for width,lang,theme in [(360,'zh','dark'),(768,'en','light'),(1280,'zh','light'),(1920,'en','dark')]:
            state.update(code=503,enabled=True,empty=False,submitted=False,delay=0,reward='balance')
            USER['balance']=1
            context=context_for(browser,width,theme);page=context.new_page();errors=[];page.on('pageerror',lambda err:errors.append(str(err)))
            page.goto(origin+'/quiz-quest?lang='+lang,wait_until='networkidle')
            expect(page.locator('[role="alert"] button')).to_be_visible()
            capture(page,f'updated-quiz-{width}-{lang}-{theme}-error')
            state['code']=0;page.locator('[role="alert"] button').click()
            expect(page.locator('.quiz-question-card')).to_have_count(2)
            refresh=page.get_by_test_id('refresh-quiz');refresh.focus();expect(refresh).to_be_focused()
            refresh.hover();state['delay']=.4;before=state['gets'];refresh.evaluate('el=>{el.click();el.click()}')
            expect(page.locator('.play-page')).to_have_attribute('aria-busy','true');expect(refresh).to_be_disabled()
            expect(page.locator('input[type="radio"]')).to_have_count(0)
            capture(page,f'updated-quiz-{width}-{lang}-{theme}-loading')
            expect(page.locator('.quiz-question-card')).to_have_count(2)
            assert state['gets']==before+1
            state['delay']=0
            for question in page.locator('.quiz-question-card').all():question.locator('label').first.click()
            capture(page,f'updated-quiz-{width}-{lang}-{theme}-ready')
            button=page.locator('button.play-btn-primary');button.focus();button.press('Enter')
            expect(page.locator('.quiz-question-card')).to_have_count(0)
            page.reload(wait_until='networkidle');expect(page.locator('.play-content-panel .play-intro')).to_contain_text('2/2')
            capture(page,f'updated-quiz-{width}-{lang}-{theme}-success')
            assert page.evaluate('document.documentElement.scrollWidth <= innerWidth')
            assert not errors,errors
            reports.append(dict(case='quiz-states',viewport=width,language=lang,theme=theme,js_errors=errors,overflow=False))
            context.close()
        state.update(code=0,enabled=True,empty=True,submitted=False,delay=0)
        context=context_for(browser);page=context.new_page();page.goto(origin+'/quiz-quest?lang=zh',wait_until='networkidle')
        expect(page.get_by_text('今日暂无可用题目，请稍后刷新。',exact=True)).to_be_visible();capture(page,'updated-quiz-empty')
        state['enabled']=False;page.get_by_test_id('refresh-quiz').click();expect(page.get_by_text('答题闯关暂未开启',exact=True).last).to_be_visible()
        capture(page,'updated-quiz-disabled');context.close()
        before=state['gets'];context=context_for(browser,360,'light',True);page=context.new_page();page.goto(origin+'/quiz-quest?lang=zh',wait_until='networkidle')
        expect(page.locator('a[href="/login?redirect=/quiz-quest"]')).to_have_text('登录')
        assert state['gets']==before
        capture(page,'updated-quiz-guest');context.close()
        reports.append(dict(case='quiz-empty-disabled-guest',guest_private_gets=0))
        state.update(code=0,enabled=True,empty=False,submitted=False,reward='redeem_code')
        USER['balance']=1
        context=context_for(browser);page=context.new_page();page.goto(origin+'/quiz-quest?lang=zh',wait_until='networkidle')
        for question in page.locator('.quiz-question-card').all():question.locator('label').first.click()
        page.locator('button.play-btn-primary').click()
        expect(page.get_by_text('今日得分 2/2 · 兑换码 FIXTURE-REWARD 已发放',exact=True).first).to_be_visible()
        expect(page.get_by_text('得分 2/2 · 到账 $0.00',exact=True)).to_have_count(0)
        capture(page,'updated-quiz-redeem-toast');context.close()
        reports.append(dict(case='quiz-redeem-toast',cash_toast=False))
        state.update(code=0,enabled=True,empty=False,submitted=False,reward='balance',lose=False);USER['balance']=1
        context=context_for(browser);page=context.new_page();page.goto(origin+'/quiz-quest?lang=zh',wait_until='networkidle')
        for question in page.locator('.quiz-question-card').all():question.locator('label').first.click()
        before_gets=state['gets'];before_posts=state['posts']
        page.evaluate('''() => { document.querySelector('button.play-btn-primary').click(); document.querySelector('[data-testid="refresh-quiz"]').click(); }''')
        expect(page.locator('.play-page')).to_have_attribute('aria-busy','false')
        expect(page.locator('.quiz-question-card')).to_have_count(0)
        expect(page.get_by_role('banner')).to_contain_text('$1.20')
        assert state['posts']==before_posts+1 and state['gets']==before_gets+1
        reports.append(dict(case='quiz-submit-then-refresh',posts=1,authoritative_reads=1,completed=True))
        context.close()
        for recovery in ['duplicate', 'unknown']:
            state.update(code=0,enabled=True,empty=False,submitted=False,reward='balance',lose=False);USER['balance']=1
            context=context_for(browser);page=context.new_page();page.goto(origin+'/quiz-quest?lang=zh',wait_until='networkidle')
            for question in page.locator('.quiz-question-card').all():question.locator('label').first.click()
            before=state['posts']
            if recovery=='duplicate':
                assert context.request.post(origin+'/api/v1/play/quiz/submit').status==200
            else:
                state['lose']=True
                page.locator('button.play-btn-primary').click()
                expect(page.get_by_text('提交失败，请稍后重试',exact=True)).to_be_visible()
                expect(page.locator('button.play-btn-primary')).to_be_enabled()
            page.locator('button.play-btn-primary').click()
            expect(page.locator('.quiz-question-card')).to_have_count(0)
            expect(page.get_by_role('banner')).to_contain_text('$1.20')
            for close in page.get_by_role('button',name='关闭',exact=True).all():close.click()
            capture(page,'updated-quiz-'+recovery+'-balance')
            assert state['posts']==before+2
            assert USER['balance']==1.2
            reports.append(dict(case='quiz-'+recovery+'-balance',posts=2,profile_balance=1.2,header_balance=1.2,reload=False))
            context.close()
    browser.close()
server.shutdown()
(OUT/(args.phase+'-browser.json')).write_text(json.dumps(dict(reports=reports,index_sha256=hashlib.sha256((dist/'index.html').read_bytes()).hexdigest()),indent=2)+'\n')
