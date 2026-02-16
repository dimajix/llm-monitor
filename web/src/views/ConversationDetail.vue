<template>
  <div>
    <v-btn variant="text" prepend-icon="mdi-arrow-left" @click="$router.back()">Back</v-btn>

    <v-card class="mt-2" :loading="loading">
      <v-card-title>Conversation {{ id }}</v-card-title>
      <v-card-subtitle>Branch: {{ currentBranchId || mainBranchId || 'unknown' }}</v-card-subtitle>
      <v-divider />

      <v-list lines="three">
        <template v-for="m in visibleMessages" :key="m.id">
          <v-list-item>
            <template #prepend>
              <v-avatar size="28" color="grey-lighten-2">
                <span class="text-caption">{{ m.role[0]?.toUpperCase() }}</span>
              </v-avatar>
            </template>
            <v-list-item-title class="d-flex align-center justify-space-between">
              <div>
                <span class="text-medium-emphasis">{{ formatDate(m.created_at) }}</span>
                <v-chip class="ml-2" size="x-small" variant="flat">{{ m.role }}</v-chip>
                <v-chip v-if="m.model" class="ml-1" size="x-small" variant="text">{{ m.model }}</v-chip>
              </div>
            </v-list-item-title>
            <v-list-item-subtitle class="py-2">
              <pre class="mb-0 message-text">{{ m.content }}</pre>
            </v-list-item-subtitle>

            <template #append>
              <div class="d-flex align-center">
                <v-tooltip v-if="(m.child_branch_ids?.length || 0) > 0" text="Switch branch from here">
                  <template #activator="{ props }">
                    <v-btn v-bind="props" size="small" icon="mdi-source-branch" @click="openBranches(m)"></v-btn>
                  </template>
                </v-tooltip>
              </div>
            </template>
          </v-list-item>
          <v-divider />
        </template>
      </v-list>

      <v-alert v-if="!loading && visibleMessages.length === 0" type="info" variant="tonal" class="ma-4">
        No messages in this conversation yet.
      </v-alert>
    </v-card>

    <v-dialog v-model="branchesDialog" max-width="480">
      <v-card>
        <v-card-title>Available branches</v-card-title>
        <v-card-text>
          <div v-if="selectedMessage">
            <div class="text-body-2 mb-2">From message at {{ formatDate(selectedMessage.created_at) }}</div>
            <v-list density="compact">
              <v-list-item
                v-for="bid in selectedMessage.child_branch_ids || []"
                :key="bid"
                :title="bid"
                @click="switchBranch(bid)"
              />
            </v-list>
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="branchesDialog = false">Close</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { getConversationMessages, getBranchHistory, type Message } from '../services/api'

const props = defineProps<{
  id: string
  initialBranchId?: string
}>()

const loading = ref(false)
const allMessages = ref<Message[]>([])
const mainBranchId = ref<string | null>(null)
const currentBranchId = ref<string | null>(null)

const branchesDialog = ref(false)
const selectedMessage = ref<Message | null>(null)

async function load() {
  loading.value = true
  try {
    const data = await getConversationMessages(props.id)
    allMessages.value = data
    // Pick main branch as the one with most messages
    const counts = new Map<string, number>()
    for (const m of data) counts.set(m.branch_id, (counts.get(m.branch_id) || 0) + 1)
    let max = 0
    let maxId: string | null = null
    counts.forEach((v, k) => {
      if (v > max) {
        max = v
        maxId = k
      }
    })
    mainBranchId.value = maxId

    // If initialBranchId is provided, load its full history.
    if (props.initialBranchId) {
      await switchBranch(props.initialBranchId)
    } else if (!currentBranchId.value) {
      currentBranchId.value = maxId
    }
  } finally {
    loading.value = false
  }
}

const visibleMessages = computed(() => {
  if (!currentBranchId.value) return []
  return allMessages.value.filter((m) => m.branch_id === currentBranchId.value).sort((a, b) => a.sequence_number - b.sequence_number)
})

function formatDate(iso: string) {
  const d = new Date(iso)
  return d.toLocaleString()
}

function openBranches(m: Message) {
  selectedMessage.value = m
  branchesDialog.value = true
}

async function switchBranch(branchId: string) {
  branchesDialog.value = false
  selectedMessage.value = null
  loading.value = true
  try {
    const { messages } = await getBranchHistory(branchId)
    // Replace/merge messages for this branch
    allMessages.value = allMessages.value.filter((m) => m.branch_id !== branchId).concat(messages)
    currentBranchId.value = branchId
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => props.id, load)
watch(() => props.initialBranchId, (newId) => {
  if (newId) switchBranch(newId)
})
</script>

<style scoped>
.message-text {
  white-space: pre-wrap;
}
</style>
