<template>
  <v-tooltip v-if="requestType === 'chat' || requestType === 'generate'" :text="'Request Type: ' + requestType">
    <template #activator="{ props: tooltipProps }">
      <v-icon
        v-bind="tooltipProps"
        :class="['ml-2', className]"
        :icon="'$' + requestType"
        :size="size"
        color="info"
      ></v-icon>
    </template>
  </v-tooltip>
  <v-tooltip v-else-if="requestType" :text="'Request Type: ' + requestType">
    <template #activator="{ props: tooltipProps }">
      <v-chip
        v-bind="tooltipProps"
        :class="['ml-2', className]"
        size="small"
        variant="flat"
        color="info"
      >
        {{ requestType }}
      </v-chip>
    </template>
  </v-tooltip>
</template>

<script setup lang="ts">
defineProps<{
  requestType?: string | null
  className?: string
  size?: string | number
}>()
</script>
