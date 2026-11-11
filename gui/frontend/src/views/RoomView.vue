<script setup>
import { ref } from 'vue'
import { leave } from '../backend.js'
import { t, errorText } from '../i18n/index.js'
import RoomCode from '../components/RoomCode.vue'

// room is status.room: the room this device is in.
defineProps({ room: { type: Object, required: true } })
const emit = defineEmits(['left'])

const busy = ref(false)
const error = ref('')

async function leaveRoom() {
  busy.value = true
  error.value = ''
  try {
    emit('left', await leave())
  } catch (err) {
    error.value = errorText(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="panel">
    <h2 class="panel-title">{{ room.name }}</h2>
    <dl class="room-facts">
      <dt>{{ t('room.code') }}</dt>
      <dd><RoomCode :code="room.code" /></dd>
      <dt>{{ t('currentRoom.virtualIp') }}</dt>
      <dd class="room-facts-ip">{{ room.virtualIp }}</dd>
      <dt>{{ t('currentRoom.subnet') }}</dt>
      <dd>{{ room.subnet }}</dd>
      <dt>{{ t('currentRoom.owner') }}</dt>
      <dd>{{ room.owner }}</dd>
    </dl>
    <p v-if="error" class="notice notice-error">{{ error }}</p>
    <div class="actions">
      <button class="button" type="button" :disabled="busy" @click="leaveRoom">
        {{ busy ? t('currentRoom.leaving') : t('currentRoom.leave') }}
      </button>
    </div>
  </section>
</template>
