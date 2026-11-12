<script setup>
import { ref } from 'vue'
import { t } from '../i18n/index.js'
import CreateRoomForm from './CreateRoomForm.vue'
import JoinRoomForm from './JoinRoomForm.vue'
import MyRoomsView from './MyRoomsView.vue'
import RoomView from './RoomView.vue'
import SettingsView from './SettingsView.vue'
import AdminView from './AdminView.vue'

defineProps({ status: { type: Object, required: true } })
// status: the daemon status changed; refresh: ask the daemon again.
const emit = defineEmits(['status', 'refresh'])

// page is 'home', 'myRooms', 'createRoom', 'joinRoom', 'settings' or 'admin'
// (only shown to administrators); back is where creating returns to. The
// home page shows the current room when the device is in one.
const page = ref('home')
const back = ref('home')

function startCreate() {
  back.value = page.value
  page.value = 'createRoom'
}

// changed returns home after joining or leaving.
function changed(st) {
  page.value = 'home'
  emit('status', st)
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
    <JoinRoomForm v-else-if="page === 'joinRoom'" @cancel="page = 'home'" @joined="changed" />
    <MyRoomsView v-else-if="page === 'myRooms'" @create="startCreate" />
    <SettingsView v-else-if="page === 'settings'" :status="status" @status="emit('status', $event)" />
    <AdminView v-else-if="page === 'admin' && status.isAdmin" :status="status" @status-lost="emit('refresh')" />
    <RoomView v-else-if="status.room" :room="status.room" :connected="status.connected"
              :revision="status.revision" :is-owner="status.room.owner === status.username" @left="changed" />
    <div v-else-if="status.state === 'joining'" class="panel">
      <p class="notice">{{ t('currentRoom.joining') }}</p>
    </div>
    <div v-else class="panel home-actions">
      <p v-if="status.notice" class="notice notice-error">{{ t(`notices.${status.notice}`) }}</p>
      <button class="button button-primary" type="button" @click="page = 'joinRoom'">{{ t('home.joinRoom') }}</button>
      <button class="button" type="button" @click="startCreate">{{ t('home.createRoom') }}</button>
    </div>
  </section>
</template>
