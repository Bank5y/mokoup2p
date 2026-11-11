<script setup>
import { onMounted, ref, watch } from 'vue'
import { status } from './backend.js'
import { t, errorText } from './i18n/index.js'
import LoginView from './views/LoginView.vue'
import HomeView from './views/HomeView.vue'
import SettingsView from './views/SettingsView.vue'
import AdminView from './views/AdminView.vue'

// current is the daemon status; null until the first answer arrives.
const current = ref(null)
const error = ref('')
// page is the view shown while logged in: home, settings or admin.
const page = ref('home')

async function refresh() {
  try {
    current.value = await status()
    error.value = ''
  } catch (err) {
    error.value = errorText(err)
  }
}

// Every login starts on the home page; the admin page needs admin rights.
watch(current, (st, prev) => {
  if (st?.state !== 'idle' || prev?.state !== 'idle') page.value = 'home'
  if (page.value === 'admin' && !st?.isAdmin) page.value = 'home'
})

onMounted(refresh)
</script>

<template>
  <main class="app">
    <div v-if="error" class="app-error">
      <p class="notice notice-error">{{ error }}</p>
      <button class="button" type="button" @click="refresh">{{ t('common.retry') }}</button>
    </div>
    <p v-else-if="!current" class="notice">{{ t('common.loading') }}</p>
    <template v-else-if="current.state === 'idle'">
      <nav class="tabs">
        <button class="tab" :class="{ 'tab-active': page === 'home' }" type="button" @click="page = 'home'">
          {{ t('nav.home') }}
        </button>
        <button class="tab" :class="{ 'tab-active': page === 'settings' }" type="button" @click="page = 'settings'">
          {{ t('nav.settings') }}
        </button>
        <button v-if="current.isAdmin" class="tab" :class="{ 'tab-active': page === 'admin' }" type="button"
                @click="page = 'admin'">
          {{ t('nav.admin') }}
        </button>
      </nav>
      <SettingsView v-if="page === 'settings'" :status="current" @status="current = $event" />
      <AdminView v-else-if="page === 'admin' && current.isAdmin" :status="current" @status-lost="refresh" />
      <HomeView v-else :status="current" />
    </template>
    <LoginView v-else :status="current" @logged-in="current = $event" />
  </main>
</template>
