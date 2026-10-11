"""Production Vue assets with loopback HTTP fixtures; never production acceptance.

Baseline uses the unchanged #353 assets (tree identical to the merge baseline).
Prototype DOM changes are only a visual preview, never passing regression proof.
"""
import argparse
import hashlib
import json
import threading
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlparse

from playwright.sync_api import sync_playwright, expect

ROOT = Path(__file__).resolve().parents[4]
OUT = ROOT / 'docs/visual-reviews/assets/checkin-status'
OUT.mkdir(parents=True, exist_ok=True)
parser = argparse.ArgumentParser()
parser.add_argument('--phase', choices=['baseline', 'updated', 'profile-repro', 'hub-session-repro'], required=True)
parser.add_argument('--dist', required=True)
args = parser.parse_args()
dist = Path(args.dist)
USER = dict(id=50, email='fixture@example.test', username='Fixture', role='user', status='active', balance=1, concurrency=5)
SETTINGS = dict(site_name='极速蹬 · TEST FIXTURE', play_checkin_enabled=True, registration_enabled=True)
INITIAL = dict(enabled=True, checked_in_today=False, server_date='2026-10-10', reward_amount=.5,
               redeemable_reward_eligible=True, can_makeup=False, streak_count=6,
               next_milestone_days=7, next_milestone_bonus=1)
state = dict(status=dict(INITIAL), code=503, gets=0, posts=0)


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
        self.wfile.write(raw)

    def do_GET(self):
        path = urlparse(self.path).path
        if path.startswith('/api/'):
            if path.endswith('/checkin/status') or path.endswith('/play/hub'):
                state['gets'] += 1
                if state['code']:
                    return self.reply(state['code'], dict(code=state['code'], reason='FIXTURE_UNAVAILABLE'))
                data = state['status']
                if path.endswith('/play/hub'):
                    data = dict(any_enabled=data['enabled'], pending_actions=int(data['enabled'] and not data['checked_in_today']),
                                growth=dict(balance=USER['balance'], total_recharged=0, payment_enabled=False),
                                checkin=data if data['enabled'] else None)
            elif path.endswith('/auth/me'):
                data = USER
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
        body = self.rfile.read(int(self.headers.get('Content-Length', 0)))
        if self.path.endswith('/auth/login'):
            account_b = json.loads(body).get('email', '').startswith('b@')
            USER.update(id=51 if account_b else 50, email='b@example.test' if account_b else 'a@example.test', balance=2 if account_b else 1)
            return self.reply(200, dict(code=0, data=dict(access_token='fixture-b' if account_b else 'fixture-a', user=USER)))
        if self.path.endswith('/play/checkin'):
            state['posts'] += 1
            state['status'].update(checked_in_today=True, streak_count=7, next_milestone_days=0)
            USER['balance'] = 2.5
            return self.reply(200, dict(code=0, data=dict(reward_type='balance', balance_added=1.5, streak_count=7, milestone_bonus=1)))
        self.reply(200, dict(code=0, data={}))


server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
threading.Thread(target=server.serve_forever, daemon=True).start()
origin = f'http://127.0.0.1:{server.server_port}'
reports = []

def capture(page, **kwargs):
    # Observe newly rendered CSS animations after layout, then let finite
    # entrances finish naturally. Screenshot fast-forward alone can race a
    # Vue render and capture a newly inserted card at near-zero opacity.
    page.evaluate("""async () => {
        await new Promise(requestAnimationFrame);
        await new Promise(requestAnimationFrame);
        await Promise.all(document.getAnimations()
            .filter(a => Number.isFinite(a.effect?.getTiming().iterations))
            .map(a => a.finished.catch(() => {})));
    }""")
    for banner in page.locator('.gw-quest-banner').all():
        assert banner.evaluate('el => Number(getComputedStyle(el).opacity)') >= 0.99
    page.screenshot(**kwargs)

def navigate(page, path, width):
    if width < 1024:
        page.locator('header button').first.click()
    page.locator('aside a[href="' + path + '"]').click()

with sync_playwright() as pw:
    browser = pw.chromium.launch(executable_path='/usr/bin/chromium', headless=True, args=['--no-sandbox', '--disable-dev-shm-usage'])
    context = browser.new_context(viewport=dict(width=1280, height=900), reduced_motion='reduce')
    context.add_init_script('window.__APP_CONFIG__=' + json.dumps(SETTINGS) + ';localStorage.setItem("auth_token","fixture-only");localStorage.setItem("auth_user",JSON.stringify(' + json.dumps(USER) + '));localStorage.setItem("theme","light");')
    context.route('**/*', lambda route: route.continue_() if urlparse(route.request.url).netloc == urlparse(origin).netloc else route.abort())
    page = context.new_page()
    page.goto(origin + '/check-in?lang=zh', wait_until='networkidle')
    if args.phase == 'baseline':
        expect(page.get_by_text('签到功能暂未开启', exact=True)).to_be_visible()
        capture(page, path=str(OUT / 'baseline-503.png'), full_page=True, animations='disabled')
        page.evaluate("""() => {
          const panel = document.querySelector('.gw-detail-grid > .gw-panel');
          panel.innerHTML = '<div class="gw-subtitle" role="alert"><p>签到状态加载失败，请检查网络后重试。</p><button type="button" class="gw-btn gw-btn-secondary mt-3">重试</button></div>';
          const title = document.querySelector('.gw-hero-grid > div');
          const button = document.createElement('button');button.type='button';button.className='gw-btn gw-btn-secondary mt-3';button.textContent='刷新';title.append(button);
          document.querySelector('.gw-balance-value').textContent='—';
        }""")
        capture(page, path=str(OUT / 'prototype-error.png'), full_page=True, animations='disabled')
        state['code'] = 0
        page.reload(wait_until='networkidle')
        expect(page.get_by_text('再签 7 天可额外获得 $1.00', exact=True)).to_be_visible()
        capture(page, path=str(OUT / 'baseline-milestone.png'), full_page=True, animations='disabled')
        page.evaluate("""() => {
          document.querySelectorAll('.gw-quest-banner').forEach(el => {
            if (el.textContent.includes('再签 7 天')) el.textContent = '再签 1 天可额外获得 $1.00';
          });
        }""")
        capture(page, path=str(OUT / 'prototype-milestone.png'), full_page=True, animations='disabled')
        state['code'] = 503
        page.goto(origin + '/play?lang=zh', wait_until='networkidle')
        expect(page.get_by_text('福利信息加载失败，请重试。', exact=True)).to_be_visible()
        capture(page, path=str(OUT / 'baseline-hub.png'), full_page=True, animations='disabled')
        page.evaluate("""() => {
          const title = document.querySelector('[data-testid="play-hub-shell"] h1').parentElement;
          const button = document.createElement('button');button.type='button';button.className='gw-btn gw-btn-secondary mt-3';button.textContent='刷新';title.append(button);
          document.querySelector('.gw-balance-value').textContent='—';
        }""")
        capture(page, path=str(OUT / 'prototype-hub.png'), full_page=True, animations='disabled')
        reports.append(dict(case='503-and-milestone', source='unchanged build; prototypes are DOM previews only', gets=state['gets'], posts=state['posts']))
    context.close()
    if args.phase in ['updated', 'profile-repro']:
        state.update(status=dict(INITIAL), code=0, gets=0, posts=0)
        USER['balance'] = 1
        context = browser.new_context(viewport=dict(width=1280, height=900), reduced_motion='reduce')
        context.add_init_script('window.__APP_CONFIG__=' + json.dumps(SETTINGS) + ';localStorage.setItem("auth_token","fixture-only");localStorage.setItem("auth_user",JSON.stringify(' + json.dumps(USER) + '));')
        pending_profiles = []
        profile_gate = dict(hold=False)
        def intercept_profile(route):
            url = urlparse(route.request.url)
            if url.netloc != urlparse(origin).netloc:
                return route.abort()
            if profile_gate['hold'] and url.path.endswith('/auth/me'):
                pending_profiles.append(route)
                return
            route.continue_()
        context.route('**/*', intercept_profile)
        page = context.new_page()
        page.goto(origin + '/play?lang=zh', wait_until='networkidle')
        profile_gate['hold'] = True
        page.locator('.gw-hub-card[href="/check-in"]').click()
        page.wait_for_timeout(200)
        assert len(pending_profiles) == 1
        old_profile = pending_profiles.pop()
        profile_gate['hold'] = False
        navigate(page, '/play', 1280)
        expect(page.get_by_test_id('refresh-status')).to_be_enabled()
        assert context.request.post(origin + '/api/v1/play/checkin').status == 200
        # A real shared store refresh represents another current consumer.
        page.evaluate("async () => { await document.querySelector('#app').__vue_app__.config.globalProperties.$pinia._s.get('auth').refreshUser() }")
        page.locator('.gw-hub-card[href="/check-in"]').click()
        expect(page.locator('.gw-balance-value')).to_have_text('$2.50')
        old_profile.fulfill(status=200, content_type='application/json', body=json.dumps(dict(code=0, data={**USER, 'balance': 1})))
        page.wait_for_timeout(200)
        expect(page.locator('.gw-balance-value')).to_have_text('$2.50')
        assert page.evaluate("document.querySelector('#app').__vue_app__.config.globalProperties.$pinia._s.get('auth').user.balance") == 2.5
        capture(page, path=str(OUT / (args.phase + '-late-profile.png')), full_page=True, animations='disabled')
        reports.append(dict(case='late-profile-after-router-reopen', balance=2.5, shared_balance=2.5, posts=state['posts']))
        context.close()
    if args.phase in ['updated', 'hub-session-repro']:
        state.update(status=dict(INITIAL), code=0, gets=0, posts=0)
        USER.update(id=50, email='a@example.test', balance=1)
        context = browser.new_context(viewport=dict(width=1280, height=900), reduced_motion='reduce')
        context.add_init_script('window.__APP_CONFIG__=' + json.dumps(SETTINGS) + ';localStorage.setItem("auth_token","fixture-a");localStorage.setItem("auth_user",JSON.stringify(' + json.dumps(USER) + '));')
        held_hub = []
        session_gate = dict(hold=False)
        def intercept_session(route):
            url = urlparse(route.request.url)
            if url.netloc != urlparse(origin).netloc:
                return route.abort()
            if url.path.endswith('/play/hub') and session_gate['hold']:
                held_hub.append(route)
                return
            route.continue_()
        context.route('**/*', intercept_session)
        page = context.new_page()
        page.goto(origin + '/play?lang=zh', wait_until='networkidle')
        expect(page.locator('.gw-balance-value')).to_have_text('$1.00')
        page.evaluate("window.__sessionHubElement = document.querySelector('[data-testid=play-hub-shell]')")
        session_gate['hold'] = True
        page.get_by_test_id('refresh-status').click()
        page.wait_for_timeout(100)
        assert len(held_hub) == 1
        old_a = held_hub.pop()
        session_gate['hold'] = False
        page.evaluate("async () => await document.querySelector('#app').__vue_app__.config.globalProperties.$pinia._s.get('auth').login({email:'b@example.test',password:'fixture-only'})")
        # The same actual component stays mounted while a new session loads.
        assert page.evaluate("window.__sessionHubElement === document.querySelector('[data-testid=play-hub-shell]')")
        if args.phase == 'updated':
            expect(page.locator('.gw-balance-value')).to_have_text('$2.00')
        else:
            page.wait_for_timeout(150)
        old_hub = dict(any_enabled=True, pending_actions=1, growth=dict(balance=1, total_recharged=0, payment_enabled=False), checkin=dict(INITIAL))
        old_a.fulfill(status=200, content_type='application/json', body=json.dumps(dict(code=0, data=old_hub)))
        page.wait_for_timeout(100)
        capture(page, path=str(OUT / (args.phase + '-hub-session.png')), full_page=True, animations='disabled')
        expect(page.locator('.gw-balance-value')).to_have_text('$2.00')
        reports.append(dict(case='same-mounted-hub-account-switch-late-A', balance=2, same_component=True))
        # Token adoption during a pending read starts a new read; the old token
        # response cannot restore a stale balance or finish the new loading state.
        session_gate['hold'] = True
        page.get_by_test_id('refresh-status').click()
        page.wait_for_timeout(100)
        assert len(held_hub) == 1
        old_token = held_hub.pop()
        session_gate['hold'] = False
        USER['balance'] = 3
        page.evaluate("() => { const auth=document.querySelector('#app').__vue_app__.config.globalProperties.$pinia._s.get('auth'); localStorage.setItem('auth_token','fixture-renewed'); auth.token='fixture-renewed'; }")
        expect(page.locator('.gw-balance-value')).to_have_text('$3.00')
        old_hub['growth']['balance'] = 2
        old_token.fulfill(status=200, content_type='application/json', body=json.dumps(dict(code=0, data=old_hub)))
        page.wait_for_timeout(100)
        expect(page.locator('.gw-balance-value')).to_have_text('$3.00')
        reports.append(dict(case='same-mounted-hub-token-adoption-late-read', balance=3, actual_automatic_refresh='covered by Pinia HTTP tests'))
        session_gate['hold'] = True
        page.get_by_test_id('refresh-status').click()
        page.wait_for_timeout(100)
        assert len(held_hub) == 1
        old_logout = held_hub.pop()
        page.evaluate("async () => await document.querySelector('#app').__vue_app__.config.globalProperties.$pinia._s.get('auth').logout()")
        expect(page.locator('.gw-balance-value')).to_have_text('—')
        old_hub['growth']['balance'] = 3
        old_logout.fulfill(status=200, content_type='application/json', body=json.dumps(dict(code=0, data=old_hub)))
        page.wait_for_timeout(100)
        expect(page.locator('.gw-balance-value')).to_have_text('—')
        expect(page.locator('.gw-hub-card')).to_have_count(0)
        capture(page, path=str(OUT / 'updated-hub-logout.png'), full_page=True, animations='disabled')
        reports.append(dict(case='same-mounted-hub-logout-late-read', balance=None, private_cards=0))
        context.close()
    if args.phase == 'updated':
        for width, lang, theme in [(360, 'zh', 'dark'), (768, 'en', 'light'), (1280, 'zh', 'light'), (1920, 'en', 'dark')]:
            for hub in [False, True]:
                state.update(status=dict(INITIAL), code=503, gets=0, posts=0)
                USER['balance'] = 1
                context = browser.new_context(viewport=dict(width=width, height=900), reduced_motion='reduce')
                context.add_init_script('window.__APP_CONFIG__=' + json.dumps(SETTINGS) + ';localStorage.setItem("auth_token","fixture-only");localStorage.setItem("auth_user",JSON.stringify(' + json.dumps(USER) + '));localStorage.setItem("theme",' + json.dumps(theme) + ');')
                held = []
                gate = dict(hold=False, network=False)
                def intercept(route):
                    url = urlparse(route.request.url)
                    if url.netloc != urlparse(origin).netloc:
                        return route.abort()
                    if url.path.endswith('/checkin/status') or url.path.endswith('/play/hub'):
                        if gate['network']:
                            return route.abort()
                        if gate['hold']:
                            held.append(route)
                            return
                    route.continue_()
                context.route('**/*', intercept)
                page = context.new_page()
                errors = []
                page.on('pageerror', lambda error: errors.append(str(error)))
                path = '/play' if hub else '/check-in'
                page.goto(origin + path + '?lang=' + lang, wait_until='networkidle')
                stem = f'updated-{width}-{lang}-{theme}-{"hub" if hub else "checkin"}'
                alert = page.locator('[role="alert"]')
                expect(alert).to_be_visible()
                expect(alert).to_contain_text(('福利信息加载失败，请重试。' if hub else '签到状态加载失败，请稍后重试。') if lang == 'zh' else ('Unable to load welfare information. Please retry.' if hub else 'Unable to load check-in status. Please retry.'))
                capture(page, path=str(OUT / (stem + '-error.png')), full_page=True, animations='disabled')
                state['code'] = 403
                alert.locator('button').click()
                expect(alert).to_contain_text(('暂无权限查看福利信息。' if hub else '暂无权限查看签到状态。') if lang == 'zh' else ('You do not have permission to view welfare information.' if hub else 'You do not have permission to view check-in status.'))
                gate['network'] = True
                alert.locator('button').click()
                expect(alert).to_contain_text(('网络异常，请检查连接后重试。') if lang == 'zh' else 'Network error. Check your connection and retry.')
                capture(page, path=str(OUT / (stem + '-network.png')), full_page=True, animations='disabled')
                gate['network'] = False
                state.update(code=0, status={**INITIAL, 'enabled': False})
                alert.locator('button').click()
                expect(alert).to_have_count(0)
                expect(page.locator('.gw-polling')).to_have_count(0)
                expect(page.locator('.gw-detail-grid button.w-full')).to_have_count(0)
                capture(page, path=str(OUT / (stem + '-disabled.png')), full_page=True, animations='disabled')
                state['status']['enabled'] = True
                refresh = page.get_by_test_id('refresh-status')
                # Keyboard reachability, focus-visible, hover and native activation.
                for _ in range(100):
                    page.keyboard.press('Tab')
                    if refresh.evaluate('el => el === document.activeElement'):
                        break
                assert refresh.evaluate('el => el === document.activeElement')
                focus = refresh.evaluate('el => ({outline: getComputedStyle(el).outlineStyle, shadow: getComputedStyle(el).boxShadow})')
                refresh.hover()
                gate['hold'] = True
                page.keyboard.press('Enter')
                expect(refresh).to_be_disabled()
                expect(page.locator('[aria-busy="true"]')).to_be_visible()
                box = refresh.bounding_box()
                page.mouse.click(box['x'] + 10, box['y'] + 10, click_count=2)
                page.keyboard.press('Enter')
                page.wait_for_timeout(100)
                assert len(held) == 1
                capture(page, path=str(OUT / (stem + '-loading.png')), full_page=True, animations='disabled')
                gate['hold'] = False
                held.pop().continue_()
                expect(refresh).to_be_enabled()
                if not hub:
                    expect(page.get_by_text('再签 1 天可额外获得 $1.00' if lang == 'zh' else '1 more days for an extra $1.00', exact=True)).to_be_visible()
                else:
                    expect(page.locator('.gw-hub-card[href="/check-in"]')).to_be_visible()
                capture(page, path=str(OUT / (stem + '-ready.png')), full_page=True, animations='disabled')
                # Leave while a refresh is pending, reopen fresh, then release the
                # old response as disabled. It cannot overwrite the current view.
                gate['hold'] = True
                refresh.click()
                expect(page.locator('[aria-busy="true"]')).to_be_visible()
                page.wait_for_timeout(100)
                assert len(held) == 1
                old = held.pop()
                gate['hold'] = False
                navigate(page, '/check-in' if hub else '/play', width)
                expect(page.get_by_test_id('refresh-status')).to_be_enabled()
                navigate(page, path, width)
                expect(page.get_by_test_id('refresh-status')).to_be_enabled()
                old.fulfill(status=200, content_type='application/json', body=json.dumps(dict(code=0, data=dict(enabled=False) if not hub else dict(any_enabled=False, growth=dict(balance=1)))))
                expect(page.get_by_test_id('refresh-status')).to_be_enabled()
                expect(alert).to_have_count(0)
                if not hub:
                    expect(page.locator('.gw-detail-grid button.w-full')).to_be_enabled()
                else:
                    expect(page.locator('.gw-hub-card[href="/check-in"]')).to_be_visible()
                dims = page.evaluate('({width:innerWidth,scroll:document.documentElement.scrollWidth})')
                assert dims['scroll'] <= width, dims
                assert not errors, errors
                reports.append(dict(case=stem, gets=state['gets'], posts=state['posts'], focus=focus, dimensions=dims, page_errors=errors, states=['503', '403', 'network', 'disabled', 'reopen', 'retry', 'loading', 'rapid-click', 'late-response']))
                context.close()
        # Actual router navigation and auth refresh on the built assets.
        state.update(status=dict(INITIAL), code=0, gets=0, posts=0)
        USER['balance'] = 1
        context = browser.new_context(viewport=dict(width=1280, height=900), reduced_motion='reduce')
        context.add_init_script('window.__APP_CONFIG__=' + json.dumps(SETTINGS) + ';localStorage.setItem("auth_token","fixture-only");localStorage.setItem("auth_user",JSON.stringify(' + json.dumps(USER) + '));')
        context.route('**/*', lambda route: route.continue_() if urlparse(route.request.url).netloc == urlparse(origin).netloc else route.abort())
        page = context.new_page()
        page.goto(origin + '/play?lang=zh', wait_until='networkidle')
        # A different tab/API client commits the grant while this page retains
        # its old auth store. Refresh hub, then enter check-in through the router.
        response = context.request.post(origin + '/api/v1/play/checkin')
        assert response.status == 200
        page.get_by_test_id('refresh-status').click()
        expect(page.locator('.gw-balance-value')).to_have_text('$2.50')
        page.locator('.gw-hub-card[href="/check-in"]').click()
        expect(page.locator('.gw-balance-value')).to_have_text('$2.50')
        expect(page.locator('.gw-detail-grid button.w-full')).to_be_disabled()
        page.get_by_test_id('refresh-status').click()
        expect(page.locator('.gw-balance-value')).to_have_text('$2.50')
        reports.append(dict(case='other-tab-grant-hub-checkin-balance', posts=state['posts'], balance=USER['balance']))
        # Reset only the isolated fixture for the normal same-tab interaction.
        state.update(status=dict(INITIAL), code=0, gets=0, posts=0)
        USER['balance'] = 1
        page.get_by_test_id('refresh-status').click()
        expect(page.locator('.gw-detail-grid button.w-full')).to_be_enabled()
        page.locator('.gw-detail-grid button.w-full').click()
        expect(page.locator('.gw-detail-grid button.w-full')).to_be_disabled()
        expect(page.locator('.gw-balance-value')).to_have_text('$2.50')
        expect(page.locator('.gw-detail-grid button.w-full')).to_contain_text('今日已签到')
        page.locator('aside a[href="/play"]').click()
        expect(page.locator('.gw-balance-value')).to_have_text('$2.50')
        expect(page.locator('.gw-hub-card[href="/check-in"]')).to_contain_text('已签到')
        capture(page, path=str(OUT / 'updated-navigation-success.png'), full_page=True, animations='disabled')
        page.locator('.gw-hub-card[href="/check-in"]').click()
        expect(page.locator('.gw-detail-grid button.w-full')).to_be_disabled()
        assert state['posts'] == 1
        reports.append(dict(case='router-checkin-hub-refresh', gets=state['gets'], posts=state['posts'], balance=USER['balance']))
        context.close()
    browser.close()
server.shutdown()
assets = {str(p.relative_to(dist)): hashlib.sha256(p.read_bytes()).hexdigest()
          for p in [dist / 'index.html', *sorted((dist / 'assets').glob('CheckInView-*.js')), *sorted((dist / 'assets').glob('PlayHubView-*.js'))]}
Path(__file__).with_name(args.phase + '-assets.json').write_text(json.dumps(assets, indent=2) + '\n')
Path(__file__).with_name(args.phase + '-browser.json').write_text(json.dumps(reports, ensure_ascii=False, indent=2) + '\n')
print(json.dumps(dict(phase=args.phase, cases=len(reports))))
