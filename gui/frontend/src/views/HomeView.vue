<script setup>
import { ref } from 'vue'
import { t } from '../i18n/index.js'
import CreateRoomForm from './CreateRoomForm.vue'
import MyRoomsView from './MyRoomsView.vue'

defineProps({ status: { type: Object, required: true } })

// page is 'home', 'myRooms' or 'createRoom'; back is where creating returns to.
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
      </nav>
      <span class="topbar-meta">{{ t('home.loggedInAs', { username: status.username }) }}</span>
      <span class="topbar-meta">{{ t('home.device', { deviceName: status.deviceName }) }}</span>
    </header>
    <CreateRoomForm v-if="page === 'createRoom'" @cancel="page = back" @done="page = 'myRooms'" />
    <MyRoomsView v-else-if="page === 'myRooms'" @create="startCreate" />
    <div v-else class="panel home-actions">
      <button class="button button-primary" type="button" @click="startCreate">{{ t('home.createRoom') }}</button>
    </div>
  </section>
</template>
