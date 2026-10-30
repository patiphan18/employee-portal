import type {
  ApiErrorBody,
  HistoryResponse,
  LoginResponse,
  Profile,
  Skill,
  SkillInput,
  SystemSettings,
  TokenInfo,
} from '../types'

const BASE = '/api/v1'

/** ApiError carries both the message and the per-field map from a 422. */
export class ApiError extends Error {
  status: number
  fieldErrors: Record<string, string>

  constructor(status: number, message: string, fieldErrors: Record<string, string> = {}) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.fieldErrors = fieldErrors
  }

  /** The message to show against a specific input, if the server named it. */
  fieldError(field: string): string | undefined {
    return this.fieldErrors[field]
  }
}

type TokenReader = () => string | null
type UnauthorizedHandler = () => void

let readToken: TokenReader = () => null
let onUnauthorized: UnauthorizedHandler = () => {}

/** Wire the client to the auth context. Called once, from AuthProvider. */
export function configureClient(reader: TokenReader, unauthorized: UnauthorizedHandler) {
  readToken = reader
  onUnauthorized = unauthorized
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  const token = readToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  if (init.body && !(init.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }

  const response = await fetch(`${BASE}${path}`, { ...init, headers })

  if (response.status === 401) {
    // The session is gone: drop it and let the app fall back to the login screen.
    onUnauthorized()
    throw new ApiError(401, 'Your session has expired. Please sign in again.')
  }

  if (response.status === 204) return undefined as T

  let body: unknown = null
  const text = await response.text()
  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      body = null
    }
  }

  if (!response.ok) {
    const err = (body ?? {}) as ApiErrorBody
    throw new ApiError(response.status, err.error || 'Something went wrong', err.errors ?? {})
  }

  return body as T
}

export const api = {
  login: (email: string, password: string) =>
    request<LoginResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),

  profile: () => request<Profile>('/me'),

  /** Patch any subset of the contact block; absent keys are left alone. */
  updateContact: (patch: Partial<Record<'mobile' | 'email' | 'address', string>>) =>
    request<Profile>('/me/contact', { method: 'PATCH', body: JSON.stringify(patch) }),

  uploadAvatar: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return request<Profile>('/me/avatar', { method: 'POST', body: form })
  },

  setSkills: (skillIds: string[], customSkills: string[] = []) =>
    request<Profile>('/me/skills', {
      method: 'PUT',
      body: JSON.stringify({ skill_ids: skillIds, custom_skills: customSkills }),
    }),

  skills: () => request<{ skills: Skill[] }>('/skills').then((r) => r.skills),

  // The window is not a client concern: an administrator configures it server-side.
  history: (page = 1, perPage = 5) =>
    request<HistoryResponse>(`/me/history?page=${page}&per_page=${perPage}`),

  /** Works for both roles; the client uses it to route after a page reload. */
  tokenInfo: () => request<TokenInfo>('/auth/me'),

  admin: {
    /** The full catalog, including entries that have been retired. */
    skills: () => request<{ skills: Skill[] }>('/admin/skills').then((r) => r.skills),

    createSkill: (input: SkillInput) =>
      request<Skill>('/admin/skills', { method: 'POST', body: JSON.stringify(input) }),

    updateSkill: (id: string, input: SkillInput) =>
      request<Skill>(`/admin/skills/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),

    settings: () => request<SystemSettings>('/admin/settings'),

    updateSettings: (settings: SystemSettings) =>
      request<SystemSettings>('/admin/settings', {
        method: 'PUT',
        body: JSON.stringify(settings),
      }),
  },
}
