<template>
  <v-list-item
    :active="false"
    class="conversation-list-item py-3 px-4"
    @click="$emit('click')"
  >
    <template v-slot:prepend>
      <div class="d-flex flex-column align-center mr-4">
        <request-type :request-type="requestType" size="24" class="ml-0" />
      </div>
    </template>

    <v-list-item-subtitle class="text-caption">
      <div class="d-flex align-center flex-wrap">
        <span class="text-medium-emphasis">{{ formattedDate }}</span>
        <v-chip
          v-if="message.role"
          size="x-small"
          variant="flat"
          class="ml-2 px-2"
          :color="roleColor"
        >
          {{ message.role }}
        </v-chip>
        <v-chip
          v-if="message.model"
          size="x-small"
          variant="outlined"
          color="secondary"
          class="ml-1 px-2"
        >
          {{ message.model }}
        </v-chip>
        <slot name="append-info"></slot>
      </div>
    </v-list-item-subtitle>

    <template v-slot:append>
      <v-icon icon="$chevron-right" color="grey-lighten-1"></v-icon>
    </template>

    <v-list-item-title class="text-subtitle-1 font-weight-medium mb-1">
      <div class="message-preview text-truncate">
        {{ message.content }}
      </div>
    </v-list-item-title>
  </v-list-item>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Message } from '../services/api'
import RequestType from './RequestType.vue'

const props = defineProps<{
  message: Message
  requestType?: string
}>()

defineEmits<{
  (e: 'click'): void
}>()

const formattedDate = computed(() => {
  if (!props.message.created_at) return ''
  const d = new Date(props.message.created_at)
  return d.toLocaleString()
})

const roleColor = computed(() => {
  switch (props.message.role) {
    case 'user': return 'primary'
    case 'assistant': return 'teal-lighten-1'
    case 'system': return 'deep-orange-lighten-1'
    default: return 'grey'
  }
})
</script>

<style scoped>
.conversation-list-item {
  transition: background-color 0.2s;
  cursor: pointer;
}

.conversation-list-item:hover {
  background-color: rgba(var(--v-theme-on-surface), 0.04);
}

.message-preview {
  max-width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
