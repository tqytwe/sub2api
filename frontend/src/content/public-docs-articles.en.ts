import type { PublicDocPageContent } from './public-docs-data.zh'

type Article = Omit<PublicDocPageContent, 'id' | 'title'>

export const ENGLISH_ARTICLES: Partial<Record<string, Article>> = {
  'tutorial:api-key': {
    summary: 'Create, copy, edit, and rotate an API key from the console.',
    html: `<h2>Create a key</h2>
<ol><li>Open <a href="/keys">API Keys</a> and choose <strong>Create API Key</strong>.</li><li>Enter a name and select a group that supports your model and protocol. Select additional groups only if your setup needs them.</li><li>Save, then use the copy control in the key's row.</li></ol>
<figure class="docs-figure"><img src="/doc-assets/integrations/create-api-key-en.png" alt="Create API Key form with the name and group filled in" width="512" height="856" loading="lazy" /><figcaption>Give the key a name, select a group, and choose Create. The group pictured here is for demonstration.</figcaption></figure>
<h2>Set limits and revoke access</h2><p>Edit the key to set its concurrency cap and spending limits for 5 hours, 1 day, or 7 days. Setting an item to zero removes that extra key-level limit. To rotate a key, test a replacement before disabling the old one. Keep keys out of source control and browser code.</p>`,
  },
  'tutorial:concurrency': {
    summary: 'Check active requests, waiting, and spending limits.',
    html: `<h2>Check which limit applies</h2><p>Concurrency counts requests in flight. RPM counts requests per minute. Spending limits are monetary limits for a given window. Use <a href="/keys">API Keys</a> to view concurrency and waiting for the key and to check its extra limits. Account, group, and upstream limits may also apply.</p><h2>Handle HTTP 429</h2><p>Read the error body first. If the condition is temporary, reduce simultaneous requests and honor <code>Retry-After</code> where supplied. For a spending limit, check the window and remaining allowance before retrying.</p>`,
  },
  'recharge-vip:how-to-recharge': {
    summary: 'Make a recharge and verify your order and balance.',
    html: `<h2>Place an order</h2><ol><li>Open <a href="/purchase">Recharge</a> and enter an amount within the range shown.</li><li>Select an available payment method. Review the currency, fees, and expected credit before confirming.</li><li>Complete payment and return to the platform.</li></ol><h2>Verify credit</h2><p>Check <a href="/orders">My Orders</a> for the payment status and <a href="/wallet">Wallet</a> for the credit. If they disagree after refreshing, contact <a href="/contact">support</a> with the order number.</p>`,
  },
  'recharge-vip:discount-examples': {
    summary: 'Distinguish recharge bonuses from API usage charges.',
    html: `<h2>Read the checkout preview</h2><p>Compare the payment amount, base credit, VIP bonus, any active promotional bonus, and expected total credit on <a href="/purchase">Recharge</a>. For illustration, base credit of 100 plus bonuses of 6 and 5 gives total credit of 111.</p><h2>Compare the actual charge</h2><p>To estimate API cost, check your assigned group and model in <a href="/catalog">Models &amp; Pricing</a>. After a test request, compare the actual charge in <a href="/usage">Usage</a>.</p>`,
  },
  'recharge-vip:faq': {
    summary: 'Find account, payment, and API troubleshooting entry points.',
    html: `<h2>Where is my payment?</h2><p>Check <a href="/orders">My Orders</a> and <a href="/wallet">Wallet</a>. For a pending order, note the order number before contacting <a href="/contact">support</a>.</p><h2>Why is a model missing?</h2><p>Sign in, open <a href="/catalog">Models &amp; Pricing</a>, and compare the group to the one attached to your <a href="/keys">API key</a>. Verify the interface before retrying.</p><h2>Why is an activity missing?</h2><p>Check the activities currently shown in <a href="/play">Play</a>. Availability and rewards depend on account eligibility and published event rules.</p>`,
  },
  'recharge-vip:blindbox-rewards': {
    summary: 'Check eligibility, active reward pools, and reward history.',
    html: `<h2>Open the activity</h2><p>Open <a href="/blindbox">Blind Box</a>. Read the rules and available draw count before confirming a draw. If the event is unavailable, check the status in <a href="/play">Play</a>.</p><h2>Find your reward</h2><p>Use the result screen and history to identify the reward type. Check <a href="/wallet">Wallet</a> for credited balance or an awarded coupon.</p>`,
  },
  'recharge-vip:token-farm': {
    summary: 'Find active challenges, ranking periods, and settlement status.',
    html: `<h2>Check the current challenge</h2><p>Open <a href="/play">Play</a> and follow the ranking or task card if it is available. Check the current round, qualifying activity, and reward rules before participating.</p><h2>Verify your progress</h2><p>Use <a href="/usage">Usage</a> to verify your API activity. Consult the ranking or task page for the activity's own progress and settlement. The displayed round rules determine which requests qualify.</p>`,
  },
  'about-us:about-us-overview': {
    summary: 'Find the gateway, catalog, usage records, and service status.',
    html: `<h2>Connect to the gateway</h2><p>Use an <a href="/keys">API key</a> and select a model and interface from <a href="/catalog">Models &amp; Pricing</a>. Match the endpoint to the protocol expected by your client.</p><h2>Verify a request</h2><p>Start with a short test and check <a href="/usage">Usage</a> for the returned model and charge. For service incidents, open <a href="/status">System Status</a>. When contacting <a href="/contact">support</a>, share the request ID, time, and error code without sharing credentials.</p>`,
  },
  'about-us:about-us-privacy': {
    summary: 'Understand request routing and how to share diagnostic information.',
    html: `<h2>Request data and diagnostics</h2><p>The gateway routes requests to the selected service. Usage records contain billing and request metadata. Feature-specific processing can differ, including asynchronous job storage and configured security auditing.</p><h2>Share only what is needed</h2><p>Keep credentials, private prompts, and personal data out of support screenshots. If you need to confirm what is processed or retained for a specific feature, contact <a href="/contact">support</a> before sending sensitive material.</p>`,
  },
  'model-learning:model-list': {
    summary: 'Find the exact model ID and protocol available for your key.',
    html: `<h2>Read the live catalog</h2><ol><li>Sign in and open <a href="/catalog">Models &amp; Pricing</a>.</li><li>Check the same group used by your <a href="/keys">API key</a>.</li><li>Copy the model ID and verify the interface and current pricing before using it.</li></ol><h2>Resolve a missing model</h2><p>Check the group, exact ID, and protocol. A model display name or a published sample may differ from the ID accepted for your account.</p>`,
  },
  'model-learning:choose-model': {
    summary: 'Compare capability, protocol, latency, and cost with a small test.',
    html: `<h2>Start with required capabilities</h2><p>Use <a href="/catalog">Models &amp; Pricing</a> to identify models available to your key. Check tool calling for coding agents, image interfaces for image jobs, and context size for long inputs.</p><h2>Compare results</h2><p>Run the same short, representative request on suitable models. Compare accuracy and latency, then check <a href="/usage">Usage</a> before increasing volume.</p>`,
  },
  'model-learning:best-practices': {
    summary: 'Validate options and keep retries and costs under control.',
    html: `<h2>Choose options for your endpoint</h2><p>Use the output limit, streaming, and caching options supported by the specific model and API interface. Start with a small output limit and compare the actual usage before relying on caching savings.</p><h2>Retry safely</h2><p>Retry transient errors with bounded backoff. For tasks that accept an idempotency key, reuse the same key only when retrying the same request. Fix authentication, invalid parameters, and exhausted allowances before retrying.</p>`,
  },
  'deploy:sdk-quick': {
    summary: 'Pick the matching SDK and set its gateway URL, key, and model.',
    html: `<h2>OpenAI-compatible clients</h2><p>Use the base URL <code>https://your-host/v1</code> and the <a href="/keys">API key</a>. Set the exact model ID from <a href="/catalog">Models &amp; Pricing</a>. Use Chat Completions or Responses only when your group supports the selected endpoint. See the <a href="/docs?cat=tutorial&amp;page=quick-start">Node.js, Python, and curl examples</a>.</p><h2>Anthropic Messages clients</h2><p>Use <code>https://your-host</code> for the SDK base URL so the SDK can append <code>/v1/messages</code>. Use a Messages-compatible key and model. Verify the first response in <a href="/usage">Usage</a>.</p>`,
  },
  'environment:nodejs-windows': {
    summary: 'Install Node.js with the Windows installer and verify PATH.',
    html: `<h2>Install Node.js</h2><ol><li>Open the <a href="https://nodejs.org/en/download">official Node.js download page</a> and choose a supported LTS Windows installer for your architecture.</li><li>Run the installer and retain the PATH option.</li><li>Open a fresh PowerShell window.</li></ol><h2>Verify the installation</h2><pre><code>node --version
npm --version</code></pre><p>If the commands are missing, check PATH and reopen the terminal before installing a client.</p>`,
  },
  'environment:nodejs-macos': {
    summary: 'Install a supported Node.js release and confirm your shell sees it.',
    html: `<h2>Install an LTS release</h2><p>Choose the macOS installer or a version manager from the <a href="https://nodejs.org/en/download">Node.js download page</a>. Use the architecture of your Mac and check the client’s minimum Node.js version.</p><h2>Verify in a new terminal</h2><pre><code>node --version
npm --version
command -v node</code></pre><p>If a version manager is in use, load its initialization from your shell startup file.</p>`,
  },
  'environment:nodejs-linux': {
    summary: 'Use a supported Node.js release without changing the system runtime.',
    html: `<h2>Choose an installer</h2><p>Use your distribution’s supported package or a user-level version manager listed on the <a href="https://nodejs.org/en/download">Node.js download page</a>. Match the client’s Node.js requirement.</p><h2>Check the active binary</h2><pre><code>node --version
npm --version
command -v node</code></pre><p>Run these from the shell where the client will run. For WSL, install inside the Linux distribution if you plan to run the client there.</p>`,
  },
  'environment:network': {
    summary: 'Diagnose connection, endpoint, and authentication errors separately.',
    html: `<h2>Find where the request stops</h2><ol><li>Copy the gateway URL from the documentation index. Check for a missing or duplicated <code>/v1</code>.</li><li>Check <a href="/status">System Status</a> if requests time out or fail before authentication.</li><li>For HTTP 401 or 403, check <a href="/keys">API Keys</a> and the selected group. For 404, compare the full request path to the expected protocol.</li></ol><h2>Report an unresolved error</h2><p>Share the timestamp, endpoint path, error code, and request ID with <a href="/contact">support</a>. Redact authorization headers and the request body.</p>`,
  },
  'vibe-coding:start-here': {
    summary: 'Run a small coding task and inspect the change before committing.',
    html: `<h2>Prepare a practice repository</h2><p>Choose a configured client from Tool integrations. Start from a clean Git state and choose a small task with an observable result.</p><h2>Run a full change cycle</h2><ol><li>Describe the expected behavior and ask for a brief plan.</li><li>Review the proposed files and commands.</li><li>Run a focused test, inspect the diff, and commit once the result is correct.</li></ol>`,
  },
  'vibe-coding:learn-ai': {
    summary: 'Choose a focused task and verify what the assistant changed.',
    html: `<h2>Pick a verifiable exercise</h2><p>Start with a failing test, an input-validation issue, or a small documentation change. Give the relevant files and the exact behavior that should change.</p><h2>Review the outcome</h2><p>Ask for the changed files and the relevant test command. Read the diff and run the test in your own environment. Keep unrelated fixes for a separate task.</p>`,
  },
  'vibe-coding:instruct-ai': {
    summary: 'Provide context, constraints, and a measurable definition of done.',
    html: `<h2>Write a concrete request</h2><p>Give the observed behavior, expected behavior, affected files, and how to verify the change. Include a short reproduction case and redact secrets.</p><h2>Agree on the scope</h2><p>Ask for a diagnosis first when the cause is unclear. State what other behavior must be preserved, and review the patch against those conditions.</p>`,
  },
  'vibe-coding:habits-ai': {
    summary: 'Keep reviews small, reproducible, and easy to recover.',
    html: `<h2>Make small changes</h2><p>Start on a branch with a clean status. Limit each task to one outcome and run focused tests after the patch.</p><h2>Keep evidence</h2><p>Record the command and result, review configuration changes, and keep credentials out of diffs and logs. Commit independent changes separately.</p>`,
  },
  'vibe-coding:constrain-ai': {
    summary: 'Set boundaries for files, commands, and acceptance criteria.',
    html: `<h2>Define the boundaries</h2><p>Name the directory the assistant may modify and any commands that require review. Provide representative inputs including an edge case.</p><h2>Verify the boundaries</h2><p>Inspect the resulting diff, check for out-of-scope files, and confirm error and loading behavior where applicable. Use a new task if the scope expands.</p>`,
  },
  'vibe-coding:context7': {
    summary: 'Look up version-matched library documentation in your coding client.',
    html: `<h2>Connect the service</h2><p>Follow the <a href="https://github.com/upstash/context7">Context7 setup guide</a> for the MCP transport supported by your client.</p><h2>Check the result</h2><p>Ask for the docs for a single package and version from your lockfile. Compare the returned source and version before applying a snippet.</p>`,
  },
  'vibe-coding:serena': {
    summary: 'Use symbol-aware navigation in a local repository.',
    html: `<h2>Set up a project</h2><p>Use the <a href="https://github.com/oraios/serena">Serena setup instructions</a> for your editor or agent. Select the intended repository and make sure the project's language is supported.</p><h2>Start with a read-only query</h2><p>Find a known function and list its references. Compare the results with the editor before relying on symbol searches for edits.</p>`,
  },
  'vibe-coding:playwright-mcp': {
    summary: 'Exercise a known browser flow with a test profile.',
    html: `<h2>Configure the browser tool</h2><p>Use the <a href="https://github.com/microsoft/playwright-mcp">Playwright MCP documentation</a> to add the server to your client. Start with a test profile and sample data.</p><h2>Verify navigation</h2><p>On a page you control, visit the start URL, click a known link, and confirm the expected title and Back behavior. Save a screenshot if layout is part of the task.</p>`,
  },
  'vibe-coding:exa': {
    summary: 'Use search results as pointers to primary sources.',
    html: `<h2>Configure search</h2><p>Use the <a href="https://github.com/exa-labs/exa-mcp-server">Exa MCP project</a> for the supported connection options. Follow its authentication instructions and store any service credential securely.</p><h2>Inspect returned evidence</h2><p>Ask for a precise topic and date range. Open the returned links and check the original source before using a technical claim.</p>`,
  },
  'vibe-coding:deepwiki': {
    summary: 'Orient yourself in a repository, then verify against source.',
    html: `<h2>Locate a repository</h2><p>Open <a href="https://deepwiki.com">DeepWiki</a> and search for the relevant public project. Use the architecture overview to identify the components to inspect.</p><h2>Check the source</h2><p>Compare the listed files to the repository branch you actually use. Verify important behavior in the source and its tests.</p>`,
  },
  'vibe-coding:codegraph': {
    summary: 'Trace callers and dependencies before changing shared code.',
    html: `<h2>Index the intended project</h2><p>Follow the <a href="https://github.com/colbymchenry/codegraph">CodeGraph project instructions</a> and point the index at your active checkout.</p><h2>Trace a symbol</h2><p>Start with a known exported function. Inspect callers and dependencies, then validate the graph against the source and refresh it after large changes.</p>`,
  },
  'vibe-coding:codex-duo': {
    summary: 'Use one session to implement and another to independently review.',
    html: `<h2>Prepare the implementation</h2><p>Configure <a href="/docs?cat=tools&amp;page=codex">Codex CLI</a>, make a small change in a practice repo, and run the focused test.</p><h2>Ask for an independent review</h2><p>Give a separate session the acceptance criteria and diff. Ask it to flag regressions and missing tests. Review its findings before making another edit.</p>`,
  },
  'vibe-coding:official-plugins': {
    summary: 'Choose a plugin supported by your client and inspect its permissions.',
    html: `<h2>Use the client’s plugin browser</h2><p>Open the plugin browser in your installed client and verify the publisher. Claude Code users can consult the <a href="https://github.com/anthropics/claude-plugins-official">official plugin collection</a>.</p><h2>Test one plugin at a time</h2><p>Inspect permissions and required service accounts before enabling it. Test on a practice repo and verify which tools it adds.</p>`,
  },
  'vibe-coding:understand-anything': {
    summary: 'Generate a codebase map and follow it back to source.',
    html: `<h2>Run on a practice checkout</h2><p>Use the <a href="https://github.com/Egonex-AI/Understand-Anything">project setup instructions</a> for your supported client. Confirm which folders the tool will read.</p><h2>Review a known module</h2><p>Generate a map and start with a module you recognize. Verify the links and inferred relationships against the checked-out source.</p>`,
  },
  'vibe-coding:skills-intro': {
    summary: 'Package a repeated workflow in a reviewable skill.',
    html: `<h2>Choose a workflow</h2><p>Start with a repeated task such as reviewing a change or running a release checklist. Consult the <a href="https://github.com/anthropics/skills">skills examples</a> and use the skill directory format supported by your client.</p><h2>Verify discovery</h2><p>Check the name, description, and instructions, then ask the client to run the skill on a sample task. Inspect any included scripts before execution.</p>`,
  },
  'vibe-coding:superpowers': {
    summary: 'Use a planning and verification workflow on one small change.',
    html: `<h2>Install for your client</h2><p>Follow the <a href="https://github.com/obra/superpowers">Superpowers project</a> instructions for the client and version you use.</p><h2>Try the workflow</h2><p>Choose a small feature, review the proposed design and plan, then ask the assistant to implement one step with tests. Inspect the diff before continuing.</p>`,
  },
  'vibe-coding:trellis': {
    summary: 'Keep project conventions and task context in a shared workflow.',
    html: `<h2>Initialize a practice project</h2><p>Use the <a href="https://www.npmjs.com/package/@mindfoldhq/trellis">Trellis package instructions</a>. Initialize inside the intended repository and review the generated files before committing.</p><h2>Walk one task through</h2><p>Record a task and its acceptance criteria, apply the relevant project conventions, and run the review step. Check the task notes against the resulting diff.</p>`,
  },
  'vibe-coding:gsd': {
    summary: 'Turn a larger change into verifiable milestones.',
    html: `<h2>Set up the workflow</h2><p>Use the <a href="https://github.com/open-gsd/gsd-core">GSD project instructions</a> for the client you use. Start a small project and review any files generated by the initializer.</p><h2>Validate each milestone</h2><p>Write one observable outcome per milestone. Review the plan, execute a small phase, and record the verification result before starting the next one.</p>`,
  },
  'vibe-coding:task-master': {
    summary: 'Convert a product brief into a reviewable task list.',
    html: `<h2>Prepare a brief</h2><p>Use the <a href="https://github.com/eyaltoledano/claude-task-master">Task Master setup instructions</a>. Configure a supported model provider and keep its API key out of your repository.</p><h2>Review generated tasks</h2><p>Parse a short brief with expected behavior and acceptance tests. Check dependencies and task boundaries before running or assigning the first task.</p>`,
  },
  'vibe-coding:open-design': {
    summary: 'Create a small design draft using a supported provider.',
    html: `<h2>Install and connect</h2><p>Follow the <a href="https://github.com/nexu-io/open-design">Open Design project instructions</a>. If your installed release offers a custom OpenAI-compatible provider, enter the matching base URL, a <a href="/keys">key</a>, and the supported model ID.</p><h2>Generate a draft</h2><p>Choose an output format, describe a small design, and review typography, layout, and the export in the target screen size.</p>`,
  },
}
