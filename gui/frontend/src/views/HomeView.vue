<script setup>
import { ref } from 'vue'
import { t } from '../i18n/index.js'
import CreateRoomForm from './CreateRoomForm.vue'
import JoinRoomForm from './JoinRoomForm.vue'
import MyRoomsView from './MyRoomsView.vue'
import RoomView from './RoomView.vue'

defineProps({ status: { type: Object, required: true } })
// status carries a new daemon status up after joining or leaving.
const emit = defineEmits(['status'])

// page is 'home', 'myRooms', 'createRoom' or 'joinRoom'; back is where
// creating returns to.
const page = ref('home')
const back = ref('home')

function startCreate() {
  back.value = page.value
  page.value = 'createRoom'
}

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
      </nav>
      <span class="topbar-meta">{{ t('home.loggedInAs', { username: status.username }) }}</span>
      <span class="topbar-meta">{{ t('home.device', { deviceName: status.deviceName }) }}</span>
    </header>
    <CreateRoomForm v-if="page === 'createRoom'" @cancel="page = back" @done="page = 'myRooms'" />
    <JoinRoomForm v-else-if="page === 'joinRoom'" @cancel="page = 'home'" @joined="changed" />
    <MyRoomsView v-else-if="page === 'myRooms'" @create="startCreate" />
    <RoomView v-else-if="status.room" :room="status.room" @left="changed" />
    <div v-else-if="status.state === 'joining'" class="panel">
      <p class="notice">{{ t('currentRoom.joining') }}</p>
    </div>
    <div v-else class="panel home-actions">
      <button class="button button-primary" type="button" @click="page = 'joinRoom'">{{ t('home.joinRoom') }}</button>
      <button class="button" type="button" @click="startCreate">{{ t('home.createRoom') }}</button>
    </div>
  </section>
</template>
