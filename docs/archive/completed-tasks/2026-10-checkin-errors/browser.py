"""Built Vue assets + loopback HTTP fixtures. No production requests or rewards.

This is UI/fault evidence; the Go contract suite separately reconciles real PG.
Run from the repository root after pnpm --dir frontend build. Baseline mode
requires assets built from the recorded baseline; --dist can select those assets.
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
OUT = ROOT / "docs/visual-reviews/assets/checkin-errors"
parser = argparse.ArgumentParser()
parser.add_argument("--phase", choices=["baseline", "updated"], required=True)
parser.add_argument("--dist", default=str(ROOT / "backend/internal/web/dist"))
args = parser.parse_args()
dist = Path(args.dist)
assets = {str(path.relative_to(dist)): hashlib.sha256(path.read_bytes()).hexdigest()
          for path in [dist / "index.html", *sorted((dist / "assets").glob("CheckInView-*.js"))]}
USER = {"id": 50, "email": "fixture@example.test", "username": "Fixture", "role": "user", "status": "active", "balance": 1, "concurrency": 5}
SETTINGS = {"site_name": "极速蹬 · TEST FIXTURE", "play_checkin_enabled": True, "registration_enabled": True}
INITIAL = {"enabled": True, "checked_in_today": False, "server_date": "2026-10-10", "reward_amount": 0.5, "redeemable_reward_eligible": True, "can_makeup": False, "streak_count": 6, "next_milestone_days": 7, "next_milestone_bonus": 1}
state = {}


class Handler(SimpleHTTPRequestHandler):
    def __init__(self, *a, **kw):
        super().__init__(*a, directory=str(dist), **kw)

    def log_message(self, *a):
        pass

    def reply(self, code, body):
        encoded = json.dumps(body).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def do_GET(self):
        path = urlparse(self.path).path
        if path.startswith("/api/"):
            if path.endswith("/checkin/status"):
                state["gets"] += 1
                if state["mode"] == "load-error":
                    return self.reply(503, {"code": 503, "reason": "FIXTURE_UNAVAILABLE"})
                data = state["status"]
            elif path.endswith("/auth/me"):
                data = USER
            elif path.endswith("/settings/public"):
                data = SETTINGS
            elif "subscriptions" in path:
                data = []
            elif "announcements" in path:
                data = {"items": [], "total": 0}
            else:
                data = {}
            return self.reply(200, {"code": 0, "data": data})
        if not Path(self.translate_path(self.path)).is_file():
            self.path = "/index.html"
        super().do_GET()

    def do_POST(self):
        self.rfile.read(int(self.headers.get("Content-Length", 0)))
        if "/play/checkin" not in self.path:
            return self.reply(200, {"code": 0, "data": {}})
        state["posts"] += 1
        mode = state["mode"]
        if mode.startswith("pending"):
            return self.reply(503, {"code": 503, "reason": "FIXTURE_SUBMIT_FAILURE"})
        if mode == "makeup":
            state["status"]["can_makeup"] = False
            return self.reply(400, {"code": 400, "reason": "PLAY_CHECKIN_MAKEUP_UNAVAILABLE"})
        if mode == "success":
            state["status"].update(checked_in_today=True, streak_count=7)
            return self.reply(200, {"code": 0, "data": {"reward_type": "balance", "balance_added": 1.5, "streak_count": 7, "milestone_bonus": 1}})
        if mode == "qualification":
            return self.reply(503, {"code": 503, "reason": "PLAY_GROWTH_QUALIFICATION_UNAVAILABLE"})
        state["status"]["checked_in_today"] = True
        return self.reply(409, {"code": 409, "reason": "PLAY_CHECKIN_ALREADY_DONE"})


server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
threading.Thread(target=server.serve_forever, daemon=True).start()
origin = f"http://127.0.0.1:{server.server_port}"
report = []
cases = [(1280, "zh", "light", "duplicate")]
if args.phase == "updated":
    cases += [(360, "zh", "dark", "duplicate"), (768, "en", "light", "duplicate"), (1920, "en", "dark", "duplicate")]
    cases += [(360, "zh", "light", mode) for mode in ["loading", "empty", "load-error", "qualification", "makeup", "success", "pending", "pending-makeup"]]
with sync_playwright() as pw:
    browser = pw.chromium.launch(executable_path="/usr/bin/chromium", headless=True, args=["--no-sandbox", "--disable-dev-shm-usage", "--disable-background-networking"])
    for width, lang, theme, mode in cases:
        state.clear()
        state.update(mode=mode, gets=0, posts=0, status=dict(INITIAL))
        if mode == "empty":
            state["status"]["enabled"] = False
        if mode in ["makeup", "pending-makeup"]:
            state["status"].update(can_makeup=True, makeup_date="2026-10-09")
        context = browser.new_context(viewport={"width": width, "height": 900}, reduced_motion="reduce")
        context.add_init_script("window.__APP_CONFIG__=" + json.dumps(SETTINGS) + ";localStorage.setItem('auth_token','fixture-only');localStorage.setItem('auth_user',JSON.stringify(" + json.dumps(USER) + "));localStorage.setItem('theme'," + json.dumps(theme) + ");")
        held = []

        def intercept(route):
            url = urlparse(route.request.url)
            if url.netloc != urlparse(origin).netloc:
                return route.abort()
            if mode == "loading" and url.path.endswith("/checkin/status"):
                held.append(route)
                return
            if mode.startswith("pending") and route.request.method == "POST" and "/play/checkin" in url.path:
                held.append(route)
                return
            return route.continue_()

        context.route("**/*", intercept)
        page = context.new_page()
        errors = []
        page.on("pageerror", lambda error: errors.append(str(error)))
        page.goto(origin + "/check-in?lang=" + lang, wait_until="domcontentloaded")
        if mode != "loading":
            page.wait_for_load_state("networkidle")
        else:
            expect(page.locator(".gw-polling")).to_be_visible()
        stem = f"{args.phase}-{width}-{lang}-{theme}-{mode}"
        if mode in ["duplicate", "makeup", "success", "qualification", "pending", "pending-makeup"]:
            button = page.locator("button.gw-btn-primary").nth(1 if mode in ["makeup", "pending-makeup"] else 0)
            expect(button).to_be_enabled()
            # Keyboard reachability, native focus, and hover on the real action.
            for _ in range(100):
                page.keyboard.press("Tab")
                if button.evaluate("el=>el===document.activeElement"):
                    break
            assert button.evaluate("el=>el===document.activeElement")
            focus = button.evaluate("el=>({outline:getComputedStyle(el).outlineStyle,shadow:getComputedStyle(el).boxShadow})")
            button.hover()
            page.screenshot(path=str(OUT / (stem + "-before.png")), full_page=True, animations="disabled")
            page.keyboard.press("Enter")
            if mode.startswith("pending"):
                expect(button).to_be_disabled()
                bounds = button.bounding_box()
                assert bounds
                # Real pointer and keyboard attempts while the first POST is
                # pending must not create any additional request.
                page.mouse.click(bounds["x"] + 10, bounds["y"] + 10, click_count=2)
                page.keyboard.press("Enter")
                page.wait_for_timeout(200)
                assert len(held) == 1
                assert state["posts"] == 0
                page.screenshot(path=str(OUT / (stem + "-submitting.png")), full_page=True, animations="disabled")
                held.pop().continue_()
                message = "补签失败，请稍后重试" if mode == "pending-makeup" else "签到失败，请稍后重试"
                expect(page.get_by_text(message, exact=True)).to_be_visible()
                expect(button).to_be_enabled()
            elif args.phase == "baseline":
                expect(page.get_by_text("签到失败，请稍后重试", exact=True)).to_be_visible()
            elif mode == "duplicate":
                message = "今日已签到" if lang == "zh" else "You have already checked in today"
                expect(page.locator(".pointer-events-auto[aria-live]")).to_contain_text(message)
                expect(button).to_contain_text("今日已签到" if lang == "zh" else "Checked in today")
                expect(page.locator("button.gw-btn-primary").first).to_be_disabled()
                assert state["gets"] == 2
                assert page.locator('[role="alert"]').count() == 0
            elif mode == "makeup":
                expect(page.get_by_text("当前不可补签", exact=True)).to_be_visible()
                expect(page.locator("button.gw-btn-primary")).to_have_count(1)
            elif mode == "qualification":
                expect(page.get_by_text("签到资格服务暂时不可用，请稍后重试。", exact=True)).to_be_visible()
                expect(button).to_be_enabled()
            else:
                expect(page.get_by_text("已到账 $1.50 · 连续签到 7 天 · 里程碑奖励 $1.00", exact=True)).to_be_visible()
                expect(button).to_be_disabled()
            expect(page.locator(".pointer-events-auto[aria-live]")).to_have_css("opacity", "1")
            expect(page.locator(".pointer-events-auto[aria-live]")).to_have_css("transform", "none")
            assert state["posts"] == 1
        else:
            focus = None
            if mode in ["empty", "load-error"]:
                expect(page.get_by_text("签到功能暂未开启", exact=True)).to_be_visible()
        page.screenshot(path=str(OUT / (stem + ".png")), full_page=True, animations="disabled")
        dims = page.evaluate("({width:innerWidth,scroll:document.documentElement.scrollWidth})")
        assert not errors, errors
        assert dims["scroll"] <= width, dims
        report.append({"mode": mode, "viewport": width, "lang": lang, "theme": theme, "gets": state["gets"], "posts": state["posts"], "focus": focus, "page_errors": errors, "dimensions": dims, "screenshot": stem + ".png"})
        if args.phase == "baseline":
            # Before implementation: use the unchanged page and real shared
            # Toast to preview the already-authorized intended duplicate state.
            page.reload(wait_until="networkidle")
            page.evaluate("document.querySelector('#app').__vue_app__.config.globalProperties.$pinia._s.get('app').showInfo('今日已签到')")
            expect(page.locator(".pointer-events-auto[aria-live]")).to_have_css("opacity", "1")
            expect(page.locator(".pointer-events-auto[aria-live]")).to_have_css("transform", "none")
            page.screenshot(path=str(OUT / "prototype-1280.png"), full_page=True, animations="disabled")
        for request in held:
            request.abort()
        context.close()
    browser.close()
server.shutdown()
Path(__file__).with_name(args.phase + "-browser.json").write_text(json.dumps(report, ensure_ascii=False, indent=2))
Path(__file__).with_name(args.phase + "-assets.json").write_text(json.dumps(assets, indent=2))
print(json.dumps({"phase": args.phase, "cases": len(report), "page_errors": 0, "overflow": 0}))
