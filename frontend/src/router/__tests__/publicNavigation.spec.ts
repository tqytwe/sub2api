import { describe, expect, it } from 'vitest'

import {
  PUBLIC_ROUTE_NAMES,
  aiCreationSpaceEntryRoute,
  buildHomePrimaryNav,
  dashboardEntryRoute,
} from '@/router/publicNavigation'

describe('public navigation contract', () => {
  it('routes the models entry to the model plaza', () => {
    const models = buildHomePrimaryNav(false).find((item) => item.key === 'models')

    expect(models?.to).toEqual({ name: PUBLIC_ROUTE_NAMES.models })
  })

  it('keeps public navigation stable for guests and authenticated users', () => {
    const guest = buildHomePrimaryNav(false).map((item) => item.key)
    const user = buildHomePrimaryNav(true).map((item) => item.key)

    expect(user).toEqual(guest)
    expect(guest).toEqual(['models', 'docs', 'creation', 'keyUsage', 'about', 'contact'])
  })

  it('preserves auth-aware destinations only for protected actions', () => {
    expect(aiCreationSpaceEntryRoute(false)).toEqual({
      name: PUBLIC_ROUTE_NAMES.register,
      query: { redirect: '/ai-creation-space' },
    })
    expect(aiCreationSpaceEntryRoute(true)).toEqual({ name: PUBLIC_ROUTE_NAMES.aiCreationSpace })
    expect(dashboardEntryRoute(false)).toEqual({ name: PUBLIC_ROUTE_NAMES.dashboard })
    expect(dashboardEntryRoute(true)).toEqual({ name: PUBLIC_ROUTE_NAMES.adminDashboard })
  })

  it('keeps English home links on English public routes', () => {
    const nav = buildHomePrimaryNav(false, 'en')
    expect(nav.find((item) => item.key === 'models')?.to).toEqual({ name: PUBLIC_ROUTE_NAMES.englishModels })
    expect(nav.find((item) => item.key === 'docs')?.to).toEqual({ name: PUBLIC_ROUTE_NAMES.englishDocs })
    expect(nav.find((item) => item.key === 'about')?.to).toEqual({
      name: PUBLIC_ROUTE_NAMES.englishAbout,
    })
    expect(nav.find((item) => item.key === 'contact')?.to).toEqual({
      name: PUBLIC_ROUTE_NAMES.englishContact,
    })
  })
})
