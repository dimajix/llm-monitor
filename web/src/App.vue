<template>
  <v-app>
    <v-app-bar app color="primary" dark>
      <v-app-bar-title>LLM Monitor</v-app-bar-title>
    </v-app-bar>
    <v-main>
      <v-container class="py-6">
        <router-view />
      </v-container>
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
</script>

<style>
html, body, #app {
  height: 100%;
}
</style>
