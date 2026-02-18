import axios from 'axios'

const apiBase = import.meta.env.VITE_API_BASE || '' // empty uses same origin or dev proxy

export type ConversationOverview = {
  id: string
  created_at: string
  metadata?: Record<string, any>
  first_message?: Message
}

export type ConversationMessages = {
    id: string
    created_at: string
    metadata?: Record<string, any>
    messages: Message[]
}

export type Message = {
  id: string
  conversation_id: string
  branch_id: string
  sequence_number: number
  created_at: string
  role: string
  content: string
  model?: string
  prompt_tokens?: number
  completion_tokens?: number
  prompt_eval_duration?: number
  eval_duration?: number
  upstream_status_code?: number
  upstream_error?: string | null
  parent_message_id?: string | null
  child_branch_ids?: string[]
}

export async function listConversations(limit = 20, offset = 0) {
  const { data } = await axios.get<ConversationOverview[]>(`${apiBase}/api/v1/conversations`, {
    params: { limit, offset },
  })
  return data
}

export async function getConversationMessages(id: string) {
  const { data } = await axios.get<ConversationMessages>(`${apiBase}/api/v1/conversations/${id}`)
  return data
}

export async function searchMessages(q: string, limit = 50, offset = 0) {
  const { data } = await axios.get<Message[]>(`${apiBase}/api/v1/search`, {
    params: { q, limit, offset },
  })
  return data
}

export async function getBranchHistory(branchId: string) {
  const { data } = await axios.get<{ branch: { id: string; conversation_id: string }; messages: Message[] }>(
    `${apiBase}/api/v1/branches/${branchId}`,
  )
  return data
}
