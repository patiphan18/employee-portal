/** A catalog entry an administrator maintains. */
export interface Skill {
  id: string
  name: string
  active: boolean
  sort_order: number
}

/**
 * One skill on a profile: either a catalog entry or free text the employee added
 * for themselves. A custom skill has no id, because it exists only on that profile.
 */
export interface ProfileSkill {
  id: string
  name: string
  custom: boolean
}

export interface Completeness {
  percent: number
  missing: string[]
}

export interface Profile {
  id: string
  first_name: string
  last_name: string
  email: string
  mobile: string | null
  address: string | null
  avatar_url: string | null
  role: Role
  skills: ProfileSkill[]
  completeness: Completeness
  updated_at: string
}

export type ChangeField = 'avatar' | 'mobile' | 'email' | 'address' | 'skills'

export interface ProfileChange {
  id: string
  user_id: string
  field: ChangeField
  old_value: string | null
  new_value: string | null
  changed_by: string
  changed_at: string
}

export interface HistoryResponse {
  days: number
  page: number
  per_page: number
  total: number
  total_pages: number
  events: ProfileChange[]
}

export type Role = 'employee' | 'admin'

export interface LoginResponse {
  token: string
  expires_at: string
  role: Role
  email: string
  /** Null for administrators, who have no employee profile. */
  profile: Profile | null
}

/** What GET /auth/me reports about the current token. */
export interface TokenInfo {
  user_id: string
  email: string
  role: Role
}

export interface SystemSettings {
  history_retention_days: number
}

/** Fields an admin may change on a catalog entry; omitted keys are left alone. */
export interface SkillInput {
  name?: string
  active?: boolean
  sort_order?: number
}

/** The error envelope every endpoint uses. */
export interface ApiErrorBody {
  error: string
  errors?: Record<string, string>
}
