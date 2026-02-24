<template>
  <v-card variant="flat" border class="tool-call-card mb-2">
    <v-card-item density="compact" class="py-1">
      <template v-slot:prepend>
        <v-icon icon="$wrench" size="x-small" color="primary" class="mr-2"></v-icon>
      </template>
      <v-card-title class="text-caption font-weight-bold">
        {{ toolCall.function.name }}
        <div v-if="toolDescription" class="text-caption font-weight-regular opacity-70 mt-1">
          {{ toolDescription }}
        </div>
      </v-card-title>
    </v-card-item>
    <v-card-text class="py-1 px-3">
      <pre class="tool-args">{{ formattedArguments }}</pre>
    </v-card-text>
  </v-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { ToolCall, Tool } from '../services/api'

const props = defineProps<{
  toolCall: ToolCall
  tools?: Tool[]
}>()

const toolDescription = computed(() => {
  return props.tools?.find(t => t.name === props.toolCall.function.name)?.description
})

const formattedArguments = computed(() => {
  try {
    return JSON.stringify(JSON.parse(props.toolCall.function.arguments), null, 2)
  } catch (_) {
    return props.toolCall.function.arguments
  }
})
</script>

<style scoped>
.tool-call-card {
  background-color: rgba(var(--v-theme-on-surface), 0.02) !important;
}

.tool-args {
  font-family: 'Fira Code', 'Courier New', Courier, monospace;
  font-size: 0.7rem;
  white-space: pre-wrap;
  word-break: break-all;
  opacity: 0.8;
}

.opacity-70 {
  opacity: 0.7;
}
</style>
