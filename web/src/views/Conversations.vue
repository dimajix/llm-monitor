<template>
  <div>
    <v-text-field
      v-model="search"
      label="Search all messages"
      prepend-inner-icon="$magnify"
      clearable
      class="mb-4"
      :loading="loading"
      @click:clear="clearSearch"
    />

    <v-card>
      <v-list lines="three">
        <template v-if="!search">
          <template v-for="c in conversations" :key="c.id">
            <chat-message
              v-if="c.first_message"
              :message="c.first_message"
              :request-type="c.request_type"
              clickable
              @click="goDetail(c.id)"
            >
              <template #append-info>
                <v-tooltip v-if="c.system_prompt" text="System prompt present">
                  <template #activator="{ props }">
                    <v-icon v-bind="props" size="small" color="grey" class="ml-2" icon="$robot-industrial"></v-icon>
                  </template>
                </v-tooltip>
              </template>
            </chat-message>
          </template>
        </template>
        <template v-else>
          <chat-message
            v-for="m in foundMessages"
            :key="m.id"
            :message="m"
            clickable
            @click="goMessageDetail(m)"
          />
          <v-list-item v-if="!loading && foundMessages.length === 0">
            <v-list-item-title>No messages found matching "{{ search }}"</v-list-item-title>
          </v-list-item>
        </template>
      </v-list>

      <v-divider />

      <div class="d-flex justify-space-between pa-4">
        <v-btn :disabled="offset === 0 || loading" @click="prevPage" variant="tonal">Prev</v-btn>
        <div>Page {{ page }}</div>
        <v-btn :disabled="!hasMore || loading" @click="nextPage" variant="tonal">Next</v-btn>
      </div>
    </v-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { listConversations, searchMessages, type ConversationOverview, type Message } from '../services/api'
import ChatMessage from '../components/ChatMessage.vue'

const router = useRouter()

const conversations = ref<ConversationOverview[]>([])
const foundMessages = ref<Message[]>([])
const page = ref(1)
const limit = 20
const offset = ref(0)
const hasMore = ref(false)
const loading = ref(false)
const search = ref('')

let debounceTimeout: any = null

async function load() {
  loading.value = true
  try {
    if (search.value) {
      const data = await searchMessages(search.value, limit, offset.value)
      foundMessages.value = data
      hasMore.value = data.length === limit
    } else {
      const data = await listConversations(limit, offset.value)
      conversations.value = data
      hasMore.value = data.length === limit
    }
  } finally {
    loading.value = false
  }
}

function clearSearch() {
  search.value = ''
  offset.value = 0
  page.value = 1
  load()
}

function prevPage() {
  if (offset.value === 0) return
  offset.value = Math.max(0, offset.value - limit)
  page.value = Math.max(1, page.value - 1)
  load()
}

function nextPage() {
  if (!hasMore.value) return
  offset.value += limit
  page.value += 1
  load()
}

function goDetail(id: string) {
  router.push({ name: 'conversation', params: { id } })
}

function goMessageDetail(m: Message) {
  router.push({
    name: 'conversation',
    params: { id: m.conversation_id },
    query: { branchId: m.branch_id }
  })
}

onMounted(load)

watch(search, () => {
  offset.value = 0
  page.value = 1
  if (debounceTimeout) clearTimeout(debounceTimeout)
  debounceTimeout = setTimeout(() => {
    load()
  }, 300)
})
</script>

<style scoped>
</style>
