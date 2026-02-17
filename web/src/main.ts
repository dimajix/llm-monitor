import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'

// Vuetify
import 'vuetify/styles'
import { createVuetify } from 'vuetify'
import { aliases, mdi } from 'vuetify/iconsets/mdi-svg'
import { mdiMagnify, mdiMessageTextOutline, mdiArrowLeft, mdiHistory, mdiAccount, mdiRobot, mdiSourceBranch, mdiThemeLightDark, mdiMemory, mdiTimerOutline, mdiContentCopy, mdiCog } from '@mdi/js'

import Conversations from './views/Conversations.vue'
import ConversationDetail from './views/ConversationDetail.vue'

// Syntax highlighting theme for code blocks rendered from Markdown
import 'highlight.js/styles/github.css'

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
      cog: mdiCog,
      'source-branch': mdiSourceBranch,
      'theme-light-dark': mdiThemeLightDark,
      'memory': mdiMemory,
      'timer-outline': mdiTimerOutline,
      'content-copy': mdiContentCopy,
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
