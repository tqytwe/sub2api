export default {
  "bps": {
    "title": "Excel / BPS defaults",
    "subtitle": "Models and options to fill when enabling BPS with defaults in an account.",
    "scopeHint": "This page only saves the template. Accounts offer default and initial activation modes. Creating, importing or syncing an account never enables BPS automatically.",
    "models": "Default models",
    "ws_sse_accelerationHint": "Save the default checkbox state for HTTP streaming WS acceleration. Off by default.",
    "auto_enable_on_degradationHint": "Save the default checkbox state for enabling BPS after degradation. Off by default.",
    "options": "Preselected options",
    "reset": "Restore recommended options",
    "advanced": "More options",
    "advancedHint": "Tools, images, 403 recovery and session proxy",
    "ignore_encrypted_contentHint": "Skip historical encrypted content BPS cannot read while keeping text and tool relationships.",
    "auto_disable_on_403Hint": "Disable BPS on a matching upstream 403. The account can still use its original Codex route.",
    "cache_creation_as_inputHint": "Bill cache creation tokens at the input rate without changing upstream caching.",
    "omit_unsupported_toolsHint": "Omit unsupported hosted tools. Live search and image generation will be unavailable.",
    "auto_recover_on_403Hint": "Requires automatic disabling on 403. Periodic text probes consume some quota and re-enable BPS on success.",
    "auto_move_on_403Hint": "Move to the selected group on a matching 403, or choose to leave all groups.",
    "session_proxyHint": "Requires a configured Mihomo or IP management pool. Unstable egress can affect requests.",
    "modelsRequired": "Select at least one default BPS model or enable all models.",
    "applying": "Loading defaults…",
    "loadFailed": "Could not load defaults. Existing settings are unchanged. Click the defaults switch to retry.",
    "manage": "Manage defaults",
    "mode": {
      "defaults": "Enable with defaults",
      "defaultsHint": "Fill saved models and options when enabled, then adjust as needed.",
      "initial": "Enable with initial settings",
      "initialHint": "Start with the original models and optional features unchecked, then configure manually.",
      "defaultsApplied": "Defaults filled. Save the account to apply.",
      "initialApplied": "Initial settings filled. Adjust options and save the account to apply."
    },
    "modeHint": "Only one mode can be active. Switching refills models and options; turning off the active switch disables BPS.",
    "adminOnly": "Default templates are available to administrators. Initial settings remain available for manual configuration."
  }

}
