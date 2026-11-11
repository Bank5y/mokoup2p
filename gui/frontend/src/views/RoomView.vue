<script setup>
import { onMounted, ref, watch } from 'vue'
import { leave, members as listMembers } from '../backend.js'
import { t, errorText } from '../i18n/index.js'
import RoomCode from '../components/RoomCode.vue'

// room is status.room: the room this device is in. connected and revision
// come from the status too; a new revision may mean the members changed.
const props = defineProps({
  room: { type: Object, required: true },
  connected: { type: Boolean, default: true },
  revision: { type: Number, default: 0 },
})
const emit = defineEmits(['left'])

const busy = ref(false)
const error = ref('')
const members = ref([])
const membersError = ref('')

async function loadMembers() {
  try {
    members.value = await listMembers()
    membersError.value = ''
  } catch (err) {
    membersError.value = errorText(err)
  }
}

onMounted(loadMembers)
watch(() => props.revision, loadMembers)

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
    <p v-if="!connected" class="notice notice-error">{{ t('currentRoom.disconnected') }}</p>
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
    <h3 class="section-title">{{ t('currentRoom.membersTitle', { count: members.length }) }}</h3>
    <p v-if="membersError" class="notice notice-error">{{ membersError }}</p>
    <table class="table members">
      <thead>
        <tr>
          <th>{{ t('currentRoom.colMember') }}</th>
          <th>{{ t('currentRoom.colVirtualIp') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="m in members" :key="m.id">
          <td>
            {{ m.username }} / {{ m.deviceName }}
            <span v-if="m.self" class="member-self">{{ t('currentRoom.self') }}</span>
          </td>
          <td class="room-facts-ip">{{ m.virtualIp }}</td>
        </tr>
      </tbody>
    </table>
    <p v-if="error" class="notice notice-error">{{ error }}</p>
    <div class="actions">
      <button class="button" type="button" :disabled="busy" @click="leaveRoom">
        {{ busy ? t('currentRoom.leaving') : t('currentRoom.leave') }}
      </button>
    </div>
  </section>
</template>
