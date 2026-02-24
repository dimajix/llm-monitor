<template>
  <v-app :theme="theme.global.name.value">
    <v-app-bar app color="primary" elevation="1">
      <v-app-bar-title>LLM Monitor</v-app-bar-title>
      <v-spacer />
      <v-btn
        icon="$theme-light-dark"
        @click="toggleTheme"
        title="Toggle dark mode"
      ></v-btn>
    </v-app-bar>
    <v-main>
      <v-container class="py-6">
        <router-view />
      </v-container>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useTheme } from 'vuetify'

const theme = useTheme()

function toggleTheme() {
  theme.global.name.value = theme.global.current.value.dark ? 'light' : 'dark'
  localStorage.setItem('theme', theme.global.name.value)
}

onMounted(() => {
  const savedTheme = localStorage.getItem('theme')
  if (savedTheme) {
    theme.global.name.value = savedTheme
  } else if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
    theme.global.name.value = 'dark'
  }
})
</script>

<style>
html, body {
  height: 100%;
}
</style>
