<template>
  <v-list-item :class="{ 'chat-message': true, 'clickable': clickable }" @click="handleClick">
    <template #prepend>
      <v-avatar size="28" color="grey-lighten-2">
        <span class="text-caption">{{ roleInitial }}</span>
      </v-avatar>
    </template>

    <v-list-item-title class="d-flex align-center justify-space-between">
      <div>
        <span class="text-medium-emphasis">{{ formattedDate }}</span>
        <v-chip class="ml-2" size="x-small" variant="flat">{{ message.role }}</v-chip>
        <v-chip v-if="message.model" class="ml-1" size="x-small" variant="outlined" color="secondary">{{ message.model }}</v-chip>
        <v-chip v-if="message.prompt_tokens || message.completion_tokens" class="ml-1" size="x-small" variant="text" color="grey">
          <v-icon start icon="mdi-memory" size="12"></v-icon>
          {{ message.prompt_tokens || 0 }} prompt tokens / {{ message.completion_tokens || 0 }} completion tokens
        </v-chip>
        <v-chip v-if="message.prompt_eval_duration || message.eval_duration" class="ml-1" size="x-small" variant="text" color="grey">
          <v-icon start icon="mdi-timer-outline" size="12"></v-icon>
          {{ formattedDurations }}
        </v-chip>
      </div>
    </v-list-item-title>

  <v-list-item-subtitle class="py-2 message-content" :class="{ 'full-size': fullSize }" style="opacity: 1">
      <div class="message-text" v-html="renderedContent"></div>
    </v-list-item-subtitle>

    <template v-if="$slots.append" #append>
      <slot name="append"></slot>
    </template>
  </v-list-item>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Message } from '../services/api'
import MarkdownIt from 'markdown-it'

const md = new MarkdownIt({
  breaks: true
})

const props = defineProps<{
  message: Message
  clickable?: boolean
  fullSize?: boolean
}>()

const emit = defineEmits<{
  (e: 'click', message: Message): void
}>()

const roleInitial = computed(() => {
  return props.message.role ? props.message.role[0].toUpperCase() : '?'
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

const formattedDurations = computed(() => {
  const prompt = formatDuration(props.message.prompt_eval_duration)
  const gen = formatDuration(props.message.eval_duration)
  if (prompt && gen) return `prompt ${prompt} / output ${gen}`
  return prompt ? `prompt ${prompt}` : gen ? `output ${gen}` : ''
})

function handleClick() {
  if (props.clickable) {
    emit('click', props.message)
  }
}
</script>

<style scoped>
.message-content.full-size {
  white-space: normal !important;
  display: block !important;
  -webkit-line-clamp: initial !important;
}
.message-text {
  font-family: inherit;
  opacity: 1;
}
.message-text :deep(p) {
  margin-bottom: 1rem;
}
.message-text :deep(p:last-child) {
  margin-bottom: 0;
}
.message-text :deep(pre) {
  background-color: #f5f5f5;
  padding: 0.5rem;
  border-radius: 4px;
  overflow-x: auto;
  margin-bottom: 1rem;
}
.message-text :deep(code) {
  background-color: #f5f5f5;
  padding: 0.1rem 0.3rem;
  border-radius: 3px;
  font-size: 0.9em;
}
.message-text :deep(pre code) {
  padding: 0;
  background-color: transparent;
}
.chat-message.clickable {
  cursor: pointer;
}
</style>
