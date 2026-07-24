function isSetupRoute(path: string): boolean {
  return path === '/setup' || path.startsWith('/setup/')
}

export function shouldProbeSetupStatusOnStartup(path: string, hasInjectedPublicSettings: boolean): boolean {
  if (isSetupRoute(path)) return false
  return !hasInjectedPublicSettings
}
