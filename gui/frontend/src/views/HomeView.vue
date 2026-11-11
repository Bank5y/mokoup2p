<script setup>
import { ref } from 'vue'
import { t } from '../i18n/index.js'
import CreateRoomForm from './CreateRoomForm.vue'
import MyRoomsView from './MyRoomsView.vue'
import SettingsView from './SettingsView.vue'
import AdminView from './AdminView.vue'

defineProps({ status: { type: Object, required: true } })
// status: the daemon status changed; refresh: ask the daemon again.
defineEmits(['status', 'refresh'])

// page is 'home', 'myRooms', 'createRoom', 'settings' or 'admin' (only shown
// to administrators); back is where creating returns to.
const page = ref('home')
const back = ref('home')

function startCreate() {
  back.value = page.value
  page.value = 'createRoom'
}
</script>

<template>
  <section class="home">
    <header class="topbar">
      <span class="topbar-title">{{ t('app.name') }}</span>
      <nav class="nav">
        <button class="nav-item" :class="{ active: page === 'home' }" type="button" @click="page = 'home'">
          {{ t('nav.home') }}
        </button>
        <button class="nav-item" :class="{ active: page === 'myRooms' }" type="button" @click="page = 'myRooms'">
          {{ t('nav.myRooms') }}
        </button>
        <button class="nav-item" :class="{ active: page === 'settings' }" type="button" @click="page = 'settings'">
          {{ t('nav.settings') }}
        </button>
        <button v-if="status.isAdmin" class="nav-item" :class="{ active: page === 'admin' }" type="button"
                @click="page = 'admin'">
          {{ t('nav.admin') }}
        </button>
      </nav>
      <span class="topbar-meta">{{ t('home.loggedInAs', { username: status.username }) }}</span>
      <span class="topbar-meta">{{ t('home.device', { deviceName: status.deviceName }) }}</span>
    </header>
    <CreateRoomForm v-if="page === 'createRoom'" @cancel="page = back" @done="page = 'myRooms'" />
    <MyRoomsView v-else-if="page === 'myRooms'" @create="startCreate" />
    <SettingsView v-else-if="page === 'settings'" :status="status" @status="$emit('status', $event)" />
    <AdminView v-else-if="page === 'admin' && status.isAdmin" :status="status" @status-lost="$emit('refresh')" />
    <div v-else class="panel home-actions">
      <button class="button button-primary" type="button" @click="startCreate">{{ t('home.createRoom') }}</button>
    </div>
  </section>
</template>
