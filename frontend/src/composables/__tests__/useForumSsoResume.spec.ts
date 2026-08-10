import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { ref, nextTick } from 'vue'

import {
  parseForumSsoResume,
  useForumSsoResume,
  FORUM_SSO_RESUME_PARAM
} from '../useForumSsoResume'

const completeForumSsoAuthorize = vi.fn()

vi.mock('@/api/forumSso', () => ({
  completeForumSsoAuthorize: (...args: unknown[]) => completeForumSsoAuthorize(...args)
}))

describe('parseForumSsoResume', () => {
  it('extracts the OAuth parameters from a relative authorize URI', () => {
    const parsed = parseForumSsoResume(
      '/api/v1/sso/oauth/authorize?client_id=sub2api_forum&redirect_uri=https%3A%2F%2Fforum.example%2Fcb&state=xyz&scope=profile'
    )

    expect(parsed).toEqual({
      clientId: 'sub2api_forum',
      redirectUri: 'https://forum.example/cb',
      state: 'xyz',
      scope: 'profile'
    })
  })

  it('defaults state and scope to empty strings when absent', () => {
    const parsed = parseForumSsoResume('?client_id=a&redirect_uri=https%3A%2F%2Ff.example%2Fcb')
    expect(parsed).toMatchObject({ state: '', scope: '' })
  })

  it('returns null when required parameters are missing', () => {
    expect(parseForumSsoResume('/authorize?client_id=only')).toBeNull()
    expect(parseForumSsoResume('/authorize?redirect_uri=https%3A%2F%2Ff.example')).toBeNull()
    expect(parseForumSsoResume('/authorize')).toBeNull()
  })

  it('returns null for absent, empty or non-string values', () => {
    expect(parseForumSsoResume(undefined)).toBeNull()
    expect(parseForumSsoResume(null)).toBeNull()
    expect(parseForumSsoResume('')).toBeNull()
    expect(parseForumSsoResume('   ')).toBeNull()
    expect(parseForumSsoResume(42)).toBeNull()
  })

  it('takes the first entry when the query parameter is repeated', () => {
    const parsed = parseForumSsoResume([
      '?client_id=first&redirect_uri=https%3A%2F%2Ff.example%2Fcb',
      '?client_id=second&redirect_uri=https%3A%2F%2Fevil.example%2Fcb'
    ])
    expect(parsed?.clientId).toBe('first')
  })

  it('ignores the host and path of an absolute value, keeping only the query', () => {
    // An attacker-supplied absolute URL must not become a navigation target.
    // Only the query is read; the origin is discarded.
    const parsed = parseForumSsoResume(
      'https://evil.example/anything?client_id=c&redirect_uri=https%3A%2F%2Fforum.example%2Fcb'
    )
    expect(parsed).toEqual({
      clientId: 'c',
      redirectUri: 'https://forum.example/cb',
      state: '',
      scope: ''
    })
  })
})

describe('useForumSsoResume', () => {
  let assign: ReturnType<typeof vi.fn>
  let originalLocation: Location

  beforeEach(() => {
    completeForumSsoAuthorize.mockReset()
    assign = vi.fn()
    originalLocation = window.location
    Object.defineProperty(window, 'location', {
      configurable: true,
      writable: true,
      value: { ...originalLocation, assign }
    })
  })

  afterEach(() => {
    Object.defineProperty(window, 'location', {
      configurable: true,
      writable: true,
      value: originalLocation
    })
  })

  function makeRoute(resume?: string) {
    return {
      path: '/login',
      query: resume === undefined ? {} : { [FORUM_SSO_RESUME_PARAM]: resume }
    } as never
  }

  const validResume = '?client_id=sub2api_forum&redirect_uri=https%3A%2F%2Fforum.example%2Fcb&state=s1'

  it('navigates to the redirect target returned by the backend once authenticated', async () => {
    completeForumSsoAuthorize.mockResolvedValue({
      redirect_uri: 'https://forum.example/cb?code=abc&state=s1'
    })
    const router = { replace: vi.fn(), push: vi.fn() }

    useForumSsoResume({
      router: router as never,
      route: makeRoute(validResume),
      isAuthenticated: ref(true)
    })
    await nextTick()
    await vi.waitFor(() => expect(assign).toHaveBeenCalled())

    expect(completeForumSsoAuthorize).toHaveBeenCalledWith({
      client_id: 'sub2api_forum',
      redirect_uri: 'https://forum.example/cb',
      state: 's1',
      scope: ''
    })
    expect(assign).toHaveBeenCalledWith('https://forum.example/cb?code=abc&state=s1')
  })

  it('does not call the backend while unauthenticated, then resumes on login', async () => {
    completeForumSsoAuthorize.mockResolvedValue({ redirect_uri: 'https://forum.example/cb?code=x' })
    const isAuthenticated = ref(false)
    const router = { replace: vi.fn(), push: vi.fn() }

    useForumSsoResume({
      router: router as never,
      route: makeRoute(validResume),
      isAuthenticated
    })
    await nextTick()
    expect(completeForumSsoAuthorize).not.toHaveBeenCalled()

    isAuthenticated.value = true
    await nextTick()
    await vi.waitFor(() => expect(assign).toHaveBeenCalled())
    expect(completeForumSsoAuthorize).toHaveBeenCalledTimes(1)
  })

  it('reports hasPendingResume so callers can suppress their own redirect', async () => {
    completeForumSsoAuthorize.mockResolvedValue({ redirect_uri: 'https://forum.example/cb?code=x' })
    const router = { replace: vi.fn(), push: vi.fn() }

    const withResume = useForumSsoResume({
      router: router as never,
      route: makeRoute(validResume),
      isAuthenticated: ref(false)
    })
    expect(withResume.hasPendingResume.value).toBe(true)

    const without = useForumSsoResume({
      router: router as never,
      route: makeRoute(),
      isAuthenticated: ref(false)
    })
    expect(without.hasPendingResume.value).toBe(false)
  })

  it('does nothing when the resume parameter is unusable', async () => {
    const router = { replace: vi.fn(), push: vi.fn() }
    const { hasPendingResume } = useForumSsoResume({
      router: router as never,
      route: makeRoute('?client_id=missing_redirect'),
      isAuthenticated: ref(true)
    })
    await nextTick()

    expect(hasPendingResume.value).toBe(false)
    expect(completeForumSsoAuthorize).not.toHaveBeenCalled()
    expect(assign).not.toHaveBeenCalled()
  })

  it('reports the error and strips the parameter when authorize fails', async () => {
    completeForumSsoAuthorize.mockRejectedValue(new Error('redirect_uri not allowed'))
    const router = { replace: vi.fn(), push: vi.fn() }
    const onError = vi.fn()

    useForumSsoResume({
      router: router as never,
      route: makeRoute(validResume),
      isAuthenticated: ref(true),
      onError
    })
    await nextTick()
    await vi.waitFor(() => expect(onError).toHaveBeenCalled())

    expect(onError).toHaveBeenCalledWith('redirect_uri not allowed')
    expect(assign).not.toHaveBeenCalled()
    // Parameter dropped so a failed handoff cannot retrigger on every navigation.
    expect(router.replace).toHaveBeenCalledWith({ path: '/login', query: {} })
  })

  it('treats a response without a redirect target as a failure', async () => {
    completeForumSsoAuthorize.mockResolvedValue({ redirect_uri: '' })
    const router = { replace: vi.fn(), push: vi.fn() }
    const onError = vi.fn()

    useForumSsoResume({
      router: router as never,
      route: makeRoute(validResume),
      isAuthenticated: ref(true),
      onError
    })
    await nextTick()
    await vi.waitFor(() => expect(onError).toHaveBeenCalled())

    expect(assign).not.toHaveBeenCalled()
  })

  it('resumes only once even if resume() is also called explicitly', async () => {
    completeForumSsoAuthorize.mockResolvedValue({ redirect_uri: 'https://forum.example/cb?code=x' })
    const router = { replace: vi.fn(), push: vi.fn() }

    const { resume } = useForumSsoResume({
      router: router as never,
      route: makeRoute(validResume),
      isAuthenticated: ref(true)
    })
    await nextTick()
    await resume()
    await vi.waitFor(() => expect(assign).toHaveBeenCalled())

    expect(completeForumSsoAuthorize).toHaveBeenCalledTimes(1)
    expect(assign).toHaveBeenCalledTimes(1)
  })
})
