<script setup>
import { onMounted, ref } from 'vue'
import { status } from './backend.js'
import { t, errorText } from './i18n/index.js'
import LoginView from './views/LoginView.vue'
import HomeView from './views/HomeView.vue'

// current is the daemon status; null until the first answer arrives.
const current = ref(null)
const error = ref('')

async function refresh() {
  try {
    current.value = await status()
    error.value = ''
  } catch (err) {
    error.value = errorText(err)
  }
}

onMounted(refresh)
</script>

<template>
  <main class="app">
    <div v-if="error" class="app-error">
      <p class="notice notice-error">{{ error }}</p>
      <button class="button" type="button" @click="refresh">{{ t('common.retry') }}</button>
    </div>
    <p v-else-if="!current" class="notice">{{ t('common.loading') }}</p>
    <HomeView v-else-if="current.state !== 'logged_out'" :status="current" @status="current = $event" />
    <LoginView v-else :status="current" @logged-in="current = $event" />
  </main>
</template>
