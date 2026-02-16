import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'

// Vuetify
import 'vuetify/styles'
import { createVuetify } from 'vuetify'
import { aliases, mdi } from 'vuetify/iconsets/mdi-svg'
import { mdiMagnify, mdiMessageTextOutline, mdiArrowLeft, mdiHistory, mdiAccount, mdiRobot, mdiSourceBranch } from '@mdi/js'

import Conversations from './views/Conversations.vue'
import ConversationDetail from './views/ConversationDetail.vue'

const vuetify = createVuetify({
  icons: {
    defaultSet: 'mdi',
    aliases: {
      ...aliases,
      magnify: mdiMagnify,
      'message-text-outline': mdiMessageTextOutline,
      'arrow-left': mdiArrowLeft,
      history: mdiHistory,
      account: mdiAccount,
      robot: mdiRobot,
      'source-branch': mdiSourceBranch,
    },
    sets: { mdi },
  },
})

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'conversations', component: Conversations },
    { path: '/conversations/:id', name: 'conversation', component: ConversationDetail, props: (route) => ({ id: route.params.id, initialBranchId: route.query.branchId }) },
  ],
})

createApp(App).use(router).use(vuetify).mount('#app')
