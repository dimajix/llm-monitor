<template>
  <div
    :class="[
      'chat-bubble-container',
      `role-${message.role}`,
      { 'has-tool-calls': message.tool_calls?.length }
    ]"
  >
    <div class="bubble-wrapper">
      <v-avatar
        size="36"
        :color="avatarColor"
        class="bubble-avatar elevation-1"
      >
        <v-icon :icon="avatarIcon" size="20"></v-icon>
      </v-avatar>

      <div class="bubble-content-outer">
        <div class="bubble-header d-flex align-center mb-1">
          <span class="role-label font-weight-bold text-uppercase mr-2">{{ message.role }}</span>
          <span class="text-caption text-medium-emphasis">{{ formattedDate }}</span>
          <v-chip
            v-if="message.model"
            size="x-small"
            variant="outlined"
            color="secondary"
            class="ml-2"
          >
            {{ message.model }}
          </v-chip>
          <v-spacer />
          <div class="bubble-actions">
            <v-btn
              icon="$content-copy"
              size="x-small"
              variant="text"
              color="grey"
              density="comfortable"
              @click.stop="copyToClipboard"
              title="Copy content"
            ></v-btn>
            <slot name="actions"></slot>
          </div>
        </div>

        <div class="bubble-card elevation-1">
          <div class="message-text pa-3" v-html="renderedContent"></div>

          <div v-if="message.tool_calls?.length" class="tool-calls px-3 pb-3">
            <v-divider class="mb-3" />
            <tool-call
              v-for="tc in message.tool_calls"
              :key="tc.id"
              :tool-call="tc"
              :tools="message.tools"
            />
          </div>

          <div v-if="hasMetrics" class="bubble-footer px-3 py-1 text-caption d-flex align-center">
            <v-icon icon="$information-outline" size="14" class="mr-1 opacity-60"></v-icon>
            <span class="opacity-70">{{ formattedMetrics }}</span>
          </div>
        </div>
        <div class="bubble-append mt-1">
          <slot name="append"></slot>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Message } from '../services/api'
import ToolCall from './ToolCall.vue'
import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js'

const props = defineProps<{
  message: Message
}>()

const md = new MarkdownIt({
  breaks: true,
  highlight(code, lang) {
    if (lang && hljs.getLanguage(lang)) {
      try {
        return `<pre><code class="hljs language-${lang}">${hljs.highlight(code, { language: lang, ignoreIllegals: true }).value}</code></pre>`
      } catch (_) {}
    }
    return `<pre><code class="hljs">${md.utils.escapeHtml(code)}</code></pre>`
  }
})

const renderedContent = computed(() => md.render(props.message.content || ''))

const avatarColor = computed(() => {
  switch (props.message.role) {
    case 'user': return 'primary'
    case 'assistant': return 'teal-darken-1'
    case 'system': return 'deep-orange-darken-1'
    case 'tool': return 'blue-grey-darken-1'
    default: return 'grey'
  }
})

const avatarIcon = computed(() => {
  switch (props.message.role) {
    case 'user': return '$account'
    case 'assistant': return '$robot'
    case 'system': return '$robot-industrial'
    case 'tool': return '$wrench'
    default: return '$help'
  }
})

const formattedDate = computed(() => {
  if (!props.message.created_at) return ''
  return new Date(props.message.created_at).toLocaleString()
})

const hasMetrics = computed(() =>
  props.message.prompt_tokens || props.message.completion_tokens ||
  props.message.prompt_eval_duration || props.message.eval_duration
)

function formatDuration(ns?: number): string {
  if (!ns || ns <= 0) return '0ms'
  const ms = ns / 1e6
  if (ms < 1000) return `${Math.round(ms)}ms`
  return `${(ms / 1000).toFixed(2)}s`
}

function calculateTps(tokens?: number, ns?: number): string {
  if (!tokens || !ns || ns <= 0) return '0 t/s'
  return `${(tokens / (ns / 1e9)).toFixed(1)} t/s`
}

const formattedMetrics = computed(() => {
  const parts = []
  if (props.message.prompt_tokens || props.message.prompt_eval_duration) {
    parts.push(`Prompt: ${props.message.prompt_tokens || 0} tokens / ${formatDuration(props.message.prompt_eval_duration)} / ${calculateTps(props.message.prompt_tokens, props.message.prompt_eval_duration)}`)
  }
  if (props.message.completion_tokens || props.message.eval_duration) {
    parts.push(`Response: ${props.message.completion_tokens || 0} tokens / ${formatDuration(props.message.eval_duration)} / ${calculateTps(props.message.completion_tokens, props.message.eval_duration)}`)
  }
  return parts.join(' • ')
})

async function copyToClipboard() {
  await navigator.clipboard.writeText(props.message.content || '')
}
</script>

<style scoped>
.chat-bubble-container {
  display: flex;
  width: 100%;
  margin-bottom: 24px;
}

.bubble-wrapper {
  display: flex;
  max-width: 85%;
  width: auto;
}

.bubble-avatar {
  flex-shrink: 0;
  margin-top: 28px; /* Align with the card top */
}

.bubble-content-outer {
  flex-grow: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.bubble-header {
  font-size: 0.75rem;
  padding: 0 4px;
}

.role-label {
  letter-spacing: 0.05em;
}

.bubble-card {
  border-radius: 12px;
  background-color: rgb(var(--v-theme-surface));
  overflow: hidden;
  border: 1px solid rgba(var(--v-theme-on-surface), 0.08);
}

.bubble-footer {
  background-color: rgba(var(--v-theme-on-surface), 0.02);
  border-top: 1px solid rgba(var(--v-theme-on-surface), 0.04);
}

/* Role Specific Alignments */
.role-user {
  justify-content: flex-end;
}
.role-user .bubble-wrapper {
  flex-direction: row-reverse;
}
.role-user .bubble-avatar {
  margin-left: 12px;
}
.role-user .bubble-content-outer {
  align-items: flex-end;
}
.role-user .bubble-card {
  background-color: rgb(var(--v-theme-primary), 0.04);
  border-color: rgb(var(--v-theme-primary), 0.15);
  border-top-right-radius: 4px;
}
.role-user .role-label {
  color: rgb(var(--v-theme-primary));
}

.role-assistant {
  justify-content: flex-start;
}
.role-assistant .bubble-avatar {
  margin-right: 12px;
}
.role-assistant .bubble-card {
  border-top-left-radius: 4px;
}
.role-assistant .role-label {
  color: #00796B; /* teal darken-2 */
}

.role-system {
  justify-content: center;
  max-width: 100%;
}
.role-system .bubble-wrapper {
  max-width: 95%;
}
.role-system .bubble-card {
  background-color: rgba(var(--v-theme-on-surface), 0.04);
  border-style: dashed;
}

/* Content Styles */
.message-text {
  line-height: 1.6;
  word-break: break-word;
}
.message-text :deep(p) { margin-bottom: 12px; }
.message-text :deep(p:last-child) { margin-bottom: 0; }
.message-text :deep(pre) {
  background-color: #f5f5f5;
  padding: 12px;
  border-radius: 8px;
  margin: 12px 0;
  overflow-x: auto;
}
.message-text :deep(code) {
  font-family: 'Fira Code', monospace;
  font-size: 0.9em;
  padding: 2px 4px;
  border-radius: 4px;
  background-color: rgba(0,0,0,0.05);
}

.bubble-actions {
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.2s;
}
.bubble-content-outer:hover .bubble-actions {
  opacity: 1;
}

.opacity-60 { opacity: 0.6; }
.opacity-70 { opacity: 0.7; }
</style>
