<template>
  <div :class="{ 'chat-message-container': true, 'bubble-mode': bubble, 'user-message': message.role === 'user', 'assistant-message': message.role === 'assistant', 'system-message': message.role === 'system', 'clickable': clickable }" @click="handleClick">
    <div class="message-wrapper">
      <v-avatar v-if="bubble" size="32" :color="avatarColor" class="message-avatar">
        <v-icon v-if="message.role === 'user'" icon="$account" size="20"></v-icon>
        <v-icon v-else-if="message.role === 'system'" icon="$robot-industrial" size="20"></v-icon>
        <v-icon v-else icon="$robot" size="20"></v-icon>
      </v-avatar>

      <div class="message-bubble-content">
        <div v-if="!bubble" class="list-layout d-flex align-start w-100">
          <request-type :request-type="requestType" size="20" />
          <div class="flex-grow-1 min-width-0">
            <div class="d-flex align-center justify-space-between mb-1">
              <div>
                <span class="text-medium-emphasis text-caption">{{ formattedDate }} </span>
                <v-chip class="ml-2" size="small" variant="flat">{{ message.role }}</v-chip>
                <v-chip v-if="message.model" class="ml-1" size="small" variant="outlined" color="secondary">{{ message.model }}</v-chip>
                <slot name="append-info"></slot>
              </div>
            </div>
            <div class="message-text" :class="{ 'full-size': fullSize }" v-html="renderedContent"></div>
          </div>
        </div>

        <div v-else class="bubble-layout">
          <div class="bubble-header d-flex align-center mb-1">
            <span class="text-medium-emphasis text-caption">{{ formattedDate }} </span>
            <request-type :request-type="requestType" size="20" />
            <v-chip class="ml-2" size="small" variant="flat">{{ message.role }}</v-chip>
            <v-chip v-if="message.model" class="ml-1" size="small" variant="outlined" color="secondary">{{ message.model }}</v-chip>
          </div>

          <div class="bubble-body">
            <div class="message-text" :class="{ 'full-size': fullSize }" v-html="renderedContent"></div>

            <div v-if="hasMetadata" class="bubble-footer mt-2 pt-1 d-flex flex-wrap align-center text-caption text-grey">
              <v-icon start icon="$information-outline" size="12" class="mr-1"></v-icon>
              {{ formattedMetrics }}
            </div>
          </div>

          <div class="copy-button-container">
            <v-btn
              icon="$content-copy"
              size="x-small"
              variant="tonal"
              color="grey"
              class="copy-btn"
              @click.stop="copyToClipboard"
              title="Copy raw message"
            ></v-btn>
          </div>
        </div>

        <div v-if="!bubble" class="copy-button-container">
          <v-btn
            icon="$content-copy"
            size="x-small"
            variant="tonal"
            color="grey"
            class="copy-btn"
            @click.stop="copyToClipboard"
            title="Copy raw message"
          ></v-btn>
        </div>
        <div v-if="$slots.append" class="append-slot">
          <slot name="append"></slot>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Message } from '../services/api'
import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js'
import RequestType from './RequestType.vue'

const md = new MarkdownIt({
  breaks: true,
  highlight(code, lang) {
    try {
      if (lang && hljs.getLanguage(lang)) {
        const { value } = hljs.highlight(code, { language: lang, ignoreIllegals: true })
        return `<pre><code class="hljs language-${lang}">${value}</code></pre>`
      }
      const { value } = hljs.highlightAuto(code)
      return `<pre><code class="hljs">${value}</code></pre>`
    } catch (_) {
      // Fallback: escape HTML safely when highlighting fails
      const esc = MarkdownIt().utils.escapeHtml
      return `<pre><code class="hljs">${esc(code)}</code></pre>`
    }
  }
})

const props = defineProps<{
  message: Message
  requestType?: string
  clickable?: boolean
  fullSize?: boolean
  bubble?: boolean
}>()

const emit = defineEmits<{
  (e: 'click', message: Message): void
}>()

const avatarColor = computed(() => {
  if (props.message.role === 'user') return 'primary'
  if (props.message.role === 'assistant') return 'teal-lighten-1'
  if (props.message.role === 'system') return 'deep-orange-lighten-1'
  return 'grey'
})

const hasMetadata = computed(() => {
  return props.message.prompt_tokens || props.message.completion_tokens || props.message.prompt_eval_duration || props.message.eval_duration
})

const formattedDate = computed(() => {
  if (!props.message.created_at) return ''
  const d = new Date(props.message.created_at)
  return d.toLocaleString()
})

const renderedContent = computed(() => {
  return md.render(props.message.content || '')
})

function formatDuration(ns?: number): string | null {
  if (!ns || ns <= 0) return null
  const ms = ns / 1e6
  if (ms < 1000) return `${Math.round(ms)} ms`
  const s = ms / 1000
  if (s < 60) return `${s.toFixed(2)} s`
  const m = Math.floor(s / 60)
  const rs = (s % 60).toFixed(1)
  return `${m}m ${rs}s`
}

function calculateTps(tokens?: number, ns?: number): string | null {
  if (!tokens || !ns || ns <= 0) return null
  const s = ns / 1e9
  const tps = tokens / s
  return `${tps.toFixed(1)} t/s`
}

const formattedMetrics = computed(() => {
  const parts: string[] = []

  if (props.message.prompt_tokens || props.message.prompt_eval_duration) {
    const tokens = props.message.prompt_tokens || 0
    const duration = formatDuration(props.message.prompt_eval_duration) || '0.0sec'
    const tps = calculateTps(props.message.prompt_tokens, props.message.prompt_eval_duration) || '0.0 t/s'
    parts.push(`Prompt ${tokens} toks / ${duration} / ${tps}`)
  }

  if (props.message.completion_tokens || props.message.eval_duration) {
    const tokens = props.message.completion_tokens || 0
    const duration = formatDuration(props.message.eval_duration) || '0.0sec'
    const tps = calculateTps(props.message.completion_tokens, props.message.eval_duration) || '0.0 t/s'
    parts.push(`Response ${tokens} toks / ${duration} / ${tps}`)
  }

  return parts.join(' — ')
})

function handleClick() {
  if (props.clickable) {
    emit('click', props.message)
  }
}

async function copyToClipboard() {
  try {
    await navigator.clipboard.writeText(props.message.content || '')
  } catch (err) {
    console.error('Failed to copy text: ', err)
  }
}
</script>

<style scoped>
.chat-message-container {
  padding: 8px 16px;
  position: relative;
  width: 100%;
}
.chat-message-container.clickable {
  cursor: pointer;
}
.chat-message-container.clickable:hover:not(.bubble-mode) {
  background-color: rgba(var(--v-theme-on-surface), 0.04);
}

.message-wrapper {
  display: flex;
  max-width: 100%;
}

.message-bubble-content {
  position: relative;
  flex: 1;
  min-width: 0;
}

/* Bubble Mode Styles */
.bubble-mode .message-wrapper {
  flex-direction: row;
}
.bubble-mode.user-message .message-wrapper {
  flex-direction: row-reverse;
}

.bubble-mode .message-avatar {
  margin-top: 4px;
  flex-shrink: 0;
}
.bubble-mode.assistant-message .message-avatar {
  margin-right: 12px;
}
.bubble-mode.user-message .message-avatar {
  margin-left: 12px;
}

.bubble-mode .bubble-layout {
  padding: 12px 16px;
  border-radius: 16px;
  max-width: 85%;
  position: relative;
  background-color: rgb(var(--v-theme-surface));
  border: 1px solid rgba(var(--v-theme-on-surface), 0.08);
  box-shadow: 0 1px 2px rgba(0,0,0,0.05);
}

.bubble-mode.user-message .bubble-layout {
  margin-left: auto;
  border-top-right-radius: 4px;
  background-color: rgb(var(--v-theme-primary), 0.05);
  border-color: rgb(var(--v-theme-primary), 0.2);
}

.bubble-mode.assistant-message .bubble-layout {
  margin-right: auto;
  border-top-left-radius: 4px;
}

.bubble-mode.system-message .bubble-layout {
  margin-left: auto;
  margin-right: auto;
  background-color: rgb(var(--v-theme-surface-variant), 0.3);
  border-style: dashed;
  border-color: rgba(var(--v-theme-on-surface), 0.2);
  width: 95%;
  max-width: 95%;
}

.bubble-header {
  opacity: 0.8;
}

.bubble-footer {
  border-top: 1px dashed rgba(var(--v-theme-on-surface), 0.1);
}

.opacity-70 { opacity: 0.7; }
.opacity-60 { opacity: 0.6; }

.message-text {
  font-family: inherit;
  line-height: 1.5;
  word-break: break-word;
}
.message-text:not(.full-size) {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.message-text :deep(p) {
  margin-bottom: 0.75rem;
}
.message-text :deep(p:last-child) {
  margin-bottom: 0;
}
.message-text :deep(pre) {
  background-color: rgb(var(--v-theme-surface-variant));
  padding: 0.75rem;
  border-radius: 8px;
  overflow-x: auto;
  margin: 0.75rem 0;
}
.message-text :deep(code) {
  background-color: rgb(var(--v-theme-surface-variant));
  padding: 0.15rem 0.35rem;
  border-radius: 4px;
  font-size: 0.85em;
}
.message-text :deep(pre code) {
  padding: 0;
  background-color: transparent;
}

.copy-button-container {
  position: absolute;
  top: 8px;
  right: 8px;
  z-index: 10;
}

.copy-btn {
  opacity: 0;
  transition: opacity 0.2s;
}
.chat-message-container:hover .copy-btn {
  opacity: 0.6;
}
.copy-btn:hover {
  opacity: 1 !important;
}

.append-slot {
  position: absolute;
  bottom: 0;
  right: 0;
  transform: translateY(50%);
  z-index: 11;
}

.min-width-0 { min-width: 0; }
</style>
