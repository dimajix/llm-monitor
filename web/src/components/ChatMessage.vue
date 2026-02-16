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
      </div>
    </v-list-item-title>

    <v-list-item-subtitle class="py-2">
      <pre class="mb-0 message-text">{{ message.content }}</pre>
    </v-list-item-subtitle>

    <template v-if="$slots.append" #append>
      <slot name="append"></slot>
    </template>
  </v-list-item>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Message } from '../services/api'

const props = defineProps<{
  message: Message
  clickable?: boolean
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

function handleClick() {
  if (props.clickable) {
    emit('click', props.message)
  }
}
</script>

<style scoped>
.message-text {
  white-space: pre-wrap;
  font-family: inherit;
}
.chat-message.clickable {
  cursor: pointer;
}
</style>
