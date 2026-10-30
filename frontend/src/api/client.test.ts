import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, api, configureClient } from './client'

function mockFetch(status: number, body: unknown, capture?: (init: RequestInit & { url: string }) => void) {
  const fn = vi.fn(async (url: string, init: RequestInit = {}) => {
    capture?.({ ...init, url })
    return {
      status,
      ok: status >= 200 && status < 300,
      text: async () => (body === undefined ? '' : JSON.stringify(body)),
    } as Response
  })
  vi.stubGlobal('fetch', fn)
  return fn
}

describe('api client', () => {
  beforeEach(() => {
    configureClient(
      () => null,
      () => {},
    )
  })

  it('prefixes requests with the API base path', async () => {
    let seen: { url: string } | undefined
    mockFetch(200, { id: 'user-1' }, (init) => (seen = init))
    await api.profile()
    expect(seen?.url).toBe('/api/v1/me')
  })

  it('attaches the bearer token when there is one', async () => {
    let seen: RequestInit | undefined
    mockFetch(200, { id: 'user-1' }, (init) => (seen = init))
    configureClient(
      () => 'token-abc',
      () => {},
    )

    await api.profile()
    const headers = seen?.headers as Headers
    expect(headers.get('Authorization')).toBe('Bearer token-abc')
  })

  it('omits the Authorization header when signed out', async () => {
    let seen: RequestInit | undefined
    mockFetch(200, {}, (init) => (seen = init))
    await api.profile()
    expect((seen?.headers as Headers).has('Authorization')).toBe(false)
  })

  it('sets a JSON content type for JSON bodies', async () => {
    let seen: RequestInit | undefined
    mockFetch(200, {}, (init) => (seen = init))
    await api.updateContact({ mobile: '0812345678' })
    expect((seen?.headers as Headers).get('Content-Type')).toBe('application/json')
  })

  it('lets the browser set the boundary for a multipart upload', async () => {
    let seen: RequestInit | undefined
    mockFetch(200, {}, (init) => (seen = init))
    await api.uploadAvatar(new File(['x'], 'a.png', { type: 'image/png' }))
    expect((seen?.headers as Headers).has('Content-Type')).toBe(false)
    expect(seen?.body).toBeInstanceOf(FormData)
  })

  it('calls the unauthorized handler and throws on a 401', async () => {
    const onUnauthorized = vi.fn()
    configureClient(() => 'stale-token', onUnauthorized)
    mockFetch(401, { error: 'invalid or expired token' })

    await expect(api.profile()).rejects.toThrow(ApiError)
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
  })

  it('exposes per-field errors from a 422', async () => {
    mockFetch(422, { error: 'validation failed', errors: { mobile: 'invalid phone number' } })

    const err = await api.updateContact({ mobile: 'nope' }).catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err.status).toBe(422)
    expect(err.fieldError('mobile')).toBe('invalid phone number')
    expect(err.fieldError('email')).toBeUndefined()
  })

  it('reports a conflict with the server message', async () => {
    mockFetch(409, { error: 'email already in use' })
    const err = await api.updateContact({ email: 'taken@example.com' }).catch((e) => e)
    expect(err.status).toBe(409)
    expect(err.message).toBe('email already in use')
  })

  it('survives an error response with no JSON body', async () => {
    mockFetch(500, undefined)
    const err = await api.profile().catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err.message).toBe('Something went wrong')
  })

  it('unwraps the skills envelope', async () => {
    mockFetch(200, { skills: [{ id: 's-go', name: 'Go' }] })
    const skills = await api.skills()
    expect(skills).toHaveLength(1)
    expect(skills[0].name).toBe('Go')
  })

  it('passes only paging to the history endpoint', async () => {
    let seen: { url: string } | undefined
    mockFetch(200, { days: 7, page: 2, per_page: 5, total: 0, total_pages: 1, events: [] },
      (init) => (seen = init))
    await api.history(2, 5)
    expect(seen?.url).toBe('/api/v1/me/history?page=2&per_page=5')
  })

  // The retention window is the administrator's setting, so the client never asks
  // for one.
  it('never sends a days parameter', async () => {
    let seen: { url: string } | undefined
    mockFetch(200, { days: 7, page: 1, per_page: 5, total: 0, total_pages: 1, events: [] },
      (init) => (seen = init))
    await api.history()
    expect(seen?.url).toBe('/api/v1/me/history?page=1&per_page=5')
    expect(seen?.url).not.toContain('days')
  })

  it('sends catalog ids and free-text skills in separate fields', async () => {
    let seen: RequestInit | undefined
    mockFetch(200, {}, (init) => (seen = init))
    await api.setSkills(['s-go', 's-react'], ['Elixir'])
    expect(JSON.parse(seen?.body as string)).toEqual({
      skill_ids: ['s-go', 's-react'],
      custom_skills: ['Elixir'],
    })
  })

  it('defaults custom skills to an empty list', async () => {
    let seen: RequestInit | undefined
    mockFetch(200, {}, (init) => (seen = init))
    await api.setSkills(['s-go'])
    expect(JSON.parse(seen?.body as string)).toEqual({
      skill_ids: ['s-go'],
      custom_skills: [],
    })
  })
})
