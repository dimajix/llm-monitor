<template>
  <div>
    <v-text-field
      v-model="search"
      label="Search all messages"
      prepend-inner-icon="mdi-magnify"
      clearable
      class="mb-4"
      :loading="loading"
      @click:clear="clearSearch"
    />

    <v-card>
      <v-list lines="three">
        <template v-if="!search">
          <v-list-item
            v-for="c in conversations"
            :key="c.id"
            :title="formatDate(c.created_at)"
            :subtitle="c.first_message?.content || 'No messages yet'"
            @click="goDetail(c.id)"
            class="conversation-item"
          >
            <template #prepend>
              <v-icon color="primary">mdi-message-text-outline</v-icon>
            </template>
            <template #append>
              <v-chip size="small" variant="flat">{{ c.first_message?.role || '-' }}</v-chip>
            </template>
          </v-list-item>
        </template>
        <template v-else>
          <v-list-item
            v-for="m in foundMessages"
            :key="m.id"
            :title="formatDate(m.created_at)"
            @click="goMessageDetail(m)"
            class="conversation-item"
          >
            <template #prepend>
              <v-icon color="secondary">mdi-message-text-outline</v-icon>
            </template>
            <v-list-item-subtitle class="text-body-2 text-high-emphasis">
              <span class="font-weight-bold">{{ m.role }}:</span> {{ m.content }}
            </v-list-item-subtitle>
            <template #append>
              <v-chip size="small" variant="flat">{{ m.role }}</v-chip>
            </template>
          </v-list-item>
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

function formatDate(iso: string) {
  const d = new Date(iso)
  return d.toLocaleString()
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
.conversation-item {
  cursor: pointer;
}
</style>
