<template>
  <div>
    <v-btn variant="text" prepend-icon="$arrow-left" @click="$router.back()">Back</v-btn>

    <div class="mt-2" v-if="!loading || allMessages.length > 0">
      <div class="d-flex align-center px-4 py-2">
        <div>
          <h2 class="text-h6">
            Conversation {{ id }}
            <v-tooltip v-if="conversation?.request_type === 'chat' || conversation?.request_type === 'generate'" :text="'Request Type: ' + conversation?.request_type">
              <template #activator="{ props }">
                <v-icon v-bind="props" class="ml-2" :icon="'$' + conversation?.request_type" size="20" color="info"></v-icon>
              </template>
            </v-tooltip>
            <v-tooltip v-else-if="conversation?.request_type" :text="'Request Type: ' + conversation?.request_type">
              <template #activator="{ props }">
                <v-chip v-bind="props" class="ml-2" size="small" variant="flat" color="info">
                  {{ conversation?.request_type }}
                </v-chip>
              </template>
            </v-tooltip>
          </h2>
          <div class="text-subtitle-2 opacity-70">Branch: {{ currentBranchId || mainBranchId || 'unknown' }}</div>
        </div>
        <v-spacer />
        <v-progress-circular v-if="loading" indeterminate size="24" color="primary"></v-progress-circular>
      </div>
      <v-divider />

      <div class="chat-messages-list py-4">
        <template v-for="m in visibleMessages" :key="m.id">
          <chat-message :message="m" full-size bubble>
            <template #append>
              <div class="d-flex align-center">
                <v-tooltip v-if="(m.child_branch_ids?.length || 0) > 0" text="Switch branch from here">
                  <template #activator="{ props }">
                    <v-btn v-bind="props" size="x-small" icon="$source-branch" color="secondary" variant="elevated" @click="openBranches(m)"></v-btn>
                  </template>
                </v-tooltip>
              </div>
            </template>
          </chat-message>
        </template>
      </div>

      <v-alert v-if="!loading && visibleMessages.length === 0" type="info" variant="tonal" class="ma-4">
        No messages in this conversation yet.
      </v-alert>
    </div>

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
import { getConversationMessages, getBranchHistory, type Message, type ConversationMessages } from '../services/api'
import ChatMessage from '../components/ChatMessage.vue'

const props = defineProps<{
  id: string
  initialBranchId?: string
}>()

const loading = ref(false)
const allMessages = ref<Message[]>([])
const conversation = ref<ConversationMessages['conversation'] | null>(null)
const mainBranchId = ref<string | null>(null)
const currentBranchId = ref<string | null>(null)

const branchesDialog = ref(false)
const selectedMessage = ref<Message | null>(null)

async function load() {
  loading.value = true
  try {
    const data = await getConversationMessages(props.id)
    allMessages.value = data.messages
    conversation.value = data.conversation
    // Pick main branch as the one with most messages
    const counts = new Map<string, number>()
    for (const m of data.messages) counts.set(m.branch_id, (counts.get(m.branch_id) || 0) + 1)
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

function openBranches(m: Message) {
  selectedMessage.value = m
  branchesDialog.value = true
}
function formatDate(dt:number | string | Date) : string|null {
  if (!dt) return ''
  const d = new Date(dt)
  return d.toLocaleString()
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
.chat-messages-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.opacity-70 {
  opacity: 0.7;
}
</style>
