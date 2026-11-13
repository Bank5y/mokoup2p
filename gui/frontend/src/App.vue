<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { status, waitStatus } from './backend.js'
import { t, errorText } from './i18n/index.js'
import LoginView from './views/LoginView.vue'
import HomeView from './views/HomeView.vue'
import UpdateBanner from './components/UpdateBanner.vue'
import DependencyNotice from './components/DependencyNotice.vue'
import { checkAtStartup } from './update.js'

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

// follow keeps current up to date: the daemon answers waitStatus as soon as
// anything changes (members joining or leaving, the room ending, the
// connection to the server dropping), so the UI never needs polling.
let following = true
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

async function follow() {
  while (following) {
    if (!current.value) {
      await sleep(1000)
      continue
    }
    try {
      current.value = await waitStatus(current.value.revision)
      error.value = ''
    } catch {
      await sleep(2000)
    }
  }
}

onMounted(async () => {
  await refresh()
  follow()
  checkAtStartup()
})
onUnmounted(() => {
  following = false
})
</script>

<template>
  <main class="app">
    <DependencyNotice />
    <UpdateBanner :edition="current?.edition" />
    <div v-if="error" class="app-error">
      <p class="notice notice-error">{{ error }}</p>
      <button class="button" type="button" @click="refresh">{{ t('common.retry') }}</button>
    </div>
    <p v-else-if="!current" class="notice">{{ t('common.loading') }}</p>
    <HomeView v-else-if="current.state !== 'logged_out'" :status="current"
              @status="current = $event" @refresh="refresh" />
    <LoginView v-else :status="current" @logged-in="current = $event" />
  </main>
</template>
